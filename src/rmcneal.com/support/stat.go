// stat.go
package support

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"time"
)

const (
	_        = iota
	StatRead = iota + 1
	StatWrite
	StatDisplay
	StatClear
	StatStop
	StatHoldDisplay
	StatRelDisplay
	StatSetHistogram
	StatFlush
)

type StatsRecord struct {
	OpType     int
	opSize     int64
	opBlk      int64
	opDuration time.Duration
	opStr      string
	opIdx      int
}

type StatsState struct {
	// Make sure these fields remain lower case for the
	// first character to keep them private to the structure.
	// If not ClearStruct() will zero them.
	fp          *os.File
	incoming    chan StatsRecord
	statusChans chan string
	gcfg        *JobData
	runtime     time.Duration
	holdDisplay bool
	printer     *Printer
	latency     *DistroGraph

	// From here to the end of the structure field names
	// will start with an upper case character so that
	// ClearStruct() can do it's job.
	StartTime time.Time

	SampleSpeed map[int]int64
	SampleIdx   int
	Iops        int64
	ReadBW      int64
	ReadIOPS    int64
	ReadLatAvg  time.Duration
	ReadLatHigh time.Duration
	ReadLatLow  time.Duration

	WriteBW      int64
	WriteIOPS    int64
	WriteLatAvg  time.Duration
	WriteLatHigh time.Duration
	WriteLatLow  time.Duration

	HistogramSize  [64]int64
	HistoBitmap    [64][]byte // For per second display of activity
	HistoNextAvail int
	MarkerSeconds  int
	LastIOPS       int64
	LastBW         int64
}

func (s *StatsState) Send(record StatsRecord) {
	s.incoming <- record
}

func StatsInit(global *JobData, printer *Printer) (*StatsState, error) {
	s := &StatsState{}
	s.incoming = make(chan StatsRecord, 10000)
	s.gcfg = global
	s.statusChans = make(chan string)
	s.StartTime = time.Now()
	s.SampleSpeed = map[int]int64{}
	s.runtime = s.gcfg.runtime
	s.SampleIdx = 0
	s.holdDisplay = true
	s.printer = printer
	s.latency = DistroInit(printer, "Latency Distribution")
	if global.doLinear {
		s.latency.CreateLinear(global.linearParams[0], global.linearParams[1], global.linearParams[2])
	}

	// Set the low latency statistic to a high value to start with.
	s.ReadLatLow = time.Duration(^uint64(0) >> 1)
	s.WriteLatLow = time.Duration(^uint64(0) >> 1)

	if fp, err := os.OpenFile(global.Record_File, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666); err != nil {
		return nil, err
	} else {
		s.fp = fp
		_, _ = fmt.Fprintln(fp, "# Time, IOPS, Read B/W, Write B/W")
	}

	go s.StatsWorker()
	return s, nil
}

func (s *StatsState) NextHistogramIdx() int {
	idx := s.HistoNextAvail
	s.HistoNextAvail++
	return idx
}

func (s *StatsState) Flush() string {
	s.Send(StatsRecord{OpType: StatFlush})
	return <-s.statusChans
}

func (s *StatsState) StatsWorker() {
	keepRunning := true
	recordMarkers := time.Tick(s.gcfg.recordTime)
	var recordIOPS, recordRead, recordWrite int64 = 0, 0, 0

	for keepRunning {
		select {
		case r := <-s.incoming:
			switch r.OpType {
			case StatRead:
				s.Iops++
				s.ReadIOPS++
				s.ReadBW += r.opSize
				s.ReadLatAvg += r.opDuration
				if s.ReadLatLow > r.opDuration {
					s.ReadLatLow = r.opDuration
				}
				if s.ReadLatHigh < r.opDuration {
					s.ReadLatHigh = r.opDuration
				}
				if s.HistogramSize[r.opIdx] != 0 {
					idx := r.opBlk / (s.HistogramSize[r.opIdx] / int64(len(s.HistoBitmap[r.opIdx])))
					if r.opDuration.Nanoseconds() > int64(100*time.Millisecond) {
						s.HistoBitmap[r.opIdx][idx] = '0'
					} else if s.HistoBitmap[r.opIdx][idx] != '0' {
						s.HistoBitmap[r.opIdx][idx] = 'r'
					}
				}
				s.latency.Aggregate(r.opDuration)

			case StatWrite:
				s.Iops++
				s.WriteIOPS++
				s.WriteBW += r.opSize
				s.WriteLatAvg += r.opDuration
				if s.WriteLatLow > r.opDuration {
					s.WriteLatLow = r.opDuration
				}
				if s.WriteLatHigh < r.opDuration {
					s.WriteLatHigh = r.opDuration
				}
				if s.HistogramSize[r.opIdx] != 0 {
					idx := r.opBlk / (s.HistogramSize[r.opIdx] / int64(len(s.HistoBitmap[r.opIdx])))
					if r.opDuration.Nanoseconds() > int64(100*time.Millisecond) {
						s.HistoBitmap[r.opIdx][idx] = '0'
					} else if s.HistoBitmap[r.opIdx][idx] != '0' {
						s.HistoBitmap[r.opIdx][idx] = 'w'
					}
				}
				s.latency.Aggregate(r.opDuration)

			case StatClear:
				ClearStruct(s)
				s.ReadLatLow = time.Duration(^uint64(0) >> 1)
				s.WriteLatLow = time.Duration(^uint64(0) >> 1)
				s.StartTime = time.Now()
				s.SampleSpeed = map[int]int64{}
				s.runtime = s.gcfg.runtime
				_, _ = fmt.Fprintln(s.fp, "# ---- Barrier request ----")
				recordIOPS, recordRead, recordWrite = 0, 0, 0

			case StatSetHistogram:
				var width int
				if ws, err1 := GetWinsize(os.Stdout.Fd()); err1 != nil {
					width = 80 - 11
				} else {
					width = int(ws.Width) - 11
				}
				s.HistogramSize[r.opIdx] = r.opSize
				s.HistoBitmap[r.opIdx] = make([]byte, width)
				for i := range s.HistoBitmap {
					s.HistoBitmap[r.opIdx][i] = ' '
				}

			case StatFlush:
				/*
				 * The fact that we're dealing with this operation means all previous
				 * ops have been dealt with so send an ack back.
				 */
				s.statusChans <- "stats flushed"
			case StatDisplay:
				s.StatsDump()
			case StatStop:
				keepRunning = false
			case StatHoldDisplay:
				s.holdDisplay = true
			case StatRelDisplay:
				s.holdDisplay = false
			default:
				s.printer.Send("Bad stat op request: op_type=%d\n", r.OpType)
			}

		case t := <-recordMarkers:
			if s.holdDisplay {
				break
			}

			_, _ = fmt.Fprintf(s.fp, "%02d:%02d:%02d, %d, %d, %d\n",
				t.Hour(), t.Minute(), t.Second(), s.Iops-recordIOPS,
				s.ReadBW-recordRead, s.WriteBW-recordWrite)
			recordIOPS = s.Iops
			recordRead = s.ReadBW
			recordWrite = s.WriteBW
		}
	}

	s.statusChans <- "stat channel"
}

func (s *StatsState) String() string {
	var buffer bytes.Buffer

	if s.holdDisplay {
		return ""
	}
	_, _ = fmt.Fprintf(&buffer, "[%s]", SecsToHMSstr(s.MarkerSeconds))
	s.MarkerSeconds++
	if s.HistogramSize[0] == 0 {
		_, _ = fmt.Fprintf(&buffer, "iops: %6s, BW: %6s\r", Humanize(s.Iops-s.LastIOPS, 1),
			Humanize((s.ReadBW+s.WriteBW)-s.LastBW, 1))
		s.LastIOPS = s.Iops
		s.LastBW = s.ReadBW + s.WriteBW
	} else {
		for avail := range s.HistogramSize {
			if s.HistogramSize[avail] != 0 {
				for idx, v := range s.HistoBitmap[avail] {
					_, _ = fmt.Fprintf(&buffer, "%c", v)
					s.HistoBitmap[avail][idx] = ' '
				}
				_, _ = fmt.Fprintf(&buffer, "\n%*s", 11, "")
			}
		}
		fmt.Fprintf(&buffer, "\r")
	}
	return buffer.String()
}

/*
 * findColWidth -- Reflect upon the StatsState and find the maximum width for stats
 *
 * We're interested in finding out the maximum width needed for "Low"/ReadLatLow/WriteLatLow as strings.
 * The same holds true for "Avg" and "High". Luckily we can use reflection to find the fields by their
 * names.
 */
func (s *StatsState) findColWidth(label string) (colSize int) {
	/* ---- Use the label as the minimum size ---- */
	colSize = len(label)

	/* ---- Don't bother testing for nil, this call requires a pointer ---- */
	sPtr := reflect.ValueOf(s).Elem()

	for _, latencyType := range []string{"Read", "Write"} {
		field := sPtr.FieldByName(fmt.Sprintf("%sLat%s", latencyType, label))
		size := len(fmt.Sprintf("%s", time.Duration(field.Int())))
		if colSize < size {
			colSize = size
		}
	}
	return
}

func (s *StatsState) StatsDump() {
	forceRaw := false
	runTime := time.Now().Sub(s.StartTime)

	s.latency.Graph()

	if s.ReadIOPS != 0 {
		s.ReadLatAvg = s.ReadLatAvg / time.Duration(s.ReadIOPS)
	} else {
		s.ReadLatAvg = 0
	}
	if s.WriteIOPS != 0 {
		s.WriteLatAvg = s.WriteLatAvg / time.Duration(s.WriteIOPS)
	} else {
		s.WriteLatAvg = 0
	}
	typeCol := len("Read")
	if typeCol < len("Write") {
		typeCol = len("Write")
	}
	lowCol := s.findColWidth("Low")
	avgCol := s.findColWidth("Avg")
	highCol := s.findColWidth("High")

	if int64(runTime.Seconds()) > 0 {
		s.printer.Send("%*sSummary\n", typeCol+(((lowCol+avgCol+highCol+4)-len("Summary"))/2), "")
		s.printer.Send("%*s %s\n", typeCol, "", DashLine(lowCol, avgCol, highCol))
		s.printer.Send("%*s |%*s|%*s|%*s|\n", typeCol, "", lowCol, "Low", avgCol, "Avg", highCol, "High")
		s.printer.Send("%*s %s\n", typeCol, "", DashLine(lowCol, avgCol, highCol))
		if s.ReadIOPS != 0 {
			s.printer.Send("%*s |%*s|%*s|%*s|\n",
				typeCol, "Read",
				lowCol, s.ReadLatLow,
				avgCol, s.ReadLatAvg,
				highCol, s.ReadLatHigh)
		}
		if s.WriteIOPS != 0 {
			s.printer.Send("%*s |%*s|%*s|%*s|\n",
				typeCol, "Write",
				lowCol, s.WriteLatLow,
				avgCol, s.WriteLatAvg,
				highCol, s.WriteLatHigh)
		}
		s.printer.Send("%*s %s\n", typeCol, "", DashLine(lowCol, avgCol, highCol))
		s.printer.Send("IOPS: %s, Time: %s, Bandwidth: %s (r:%s,w:%s)\n",
			Humanize(s.Iops/int64(runTime.Seconds()), 1),
			runTime,
			Humanize((s.ReadBW+s.WriteBW)/int64(runTime.Seconds()), 1),
			Humanize(s.ReadBW/int64(runTime.Seconds()), 1),
			Humanize(s.WriteBW/int64(runTime.Seconds()), 1))
	} else {
		forceRaw = true
	}

	if s.gcfg.Verbose || forceRaw {
		s.printer.Send("IO's(read=%d,write=%d), Bytes xfer'd(read=%d,write=%d)\n", s.ReadIOPS, s.WriteIOPS,
			s.ReadBW, s.WriteBW)
	}
}
