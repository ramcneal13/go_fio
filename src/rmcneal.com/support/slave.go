package support

import (
	"container/list"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type WorkerStat struct {
	Reads        int64
	Writes       int64
	BytesRead    int64
	BytesWritten int64
	ReadErrors   int
	WriteErrors  int
	Elapsed      time.Duration
	LowResponse  time.Duration
	HighResponse time.Duration
	AvgResponse  time.Duration
	Histogram    *DistroGraph
}

type SlaveController struct {
	JobConfig *JobData
	Name      string
	Printer   *Printer
	SlaveConn net.Conn
	encode    *json.Encoder
	decode    *json.Decoder
	Stats     WorkerStat
	StatChan  chan WorkerStat
}

type SlaveState struct {
	printer       *Printer
	SlaveConn     net.Conn
	params        *SlaveParams
	encode        *json.Encoder
	decode        *json.Decoder
	threadRunning bool
	workerCmpt    chan WorkerStat
	statChan      chan *WorkerStat
	targetDev     *os.File
	removeOnClose bool
	totalStats    WorkerStat
}

/*
 * The member names must all start with a capital letter else JSON
 * will not encode or decode the data correctly.
 */
type SlaveParams struct {
	JobName       string
	FileName      string
	IODepth       int
	FileSize      int64
	Runtime       time.Duration
	AccessPattern string
	accessPattern *list.List
	Verbose       bool
}

const (
	_             = iota
	SlaveOpWarmup = iota + 1
	SlaveOpStart
	SlaveOpStop
	SlaveFinished
	SlaveFinishedStats
	SlaveIntermediateStats
	StatusOkay  = 1
	StatusError = 2
)

type SlaveOp struct {
	OpType int
}

type SlaveResponse struct {
	RespStatus int
	RespInfo   string
}

type SlaveStatReply struct {
	SlaveName  string
	SlaveStats WorkerStat
}

func (sc *SlaveController) InitDial() error {
	var err error

	if sc.SlaveConn, err = net.Dial("tcp", sc.JobConfig.Slave_Host+":6969"); err != nil {
		return fmt.Errorf("failed to connect to %s, err=%s", sc.JobConfig.Slave_Host, err)
	}
	sc.encode = json.NewEncoder(sc.SlaveConn)
	sc.decode = json.NewDecoder(sc.SlaveConn)

	sapi := SlaveParams{JobName: sc.Name, FileName: sc.JobConfig.Name, IODepth: sc.JobConfig.IODepth,
		FileSize: sc.JobConfig.fileSize, Runtime: sc.JobConfig.runtime, AccessPattern: sc.JobConfig.Access_Pattern,
		Verbose: sc.JobConfig.Verbose}
	if err = sc.encode.Encode(&sapi); err != nil {
		return fmt.Errorf("JSON Encode failed on params: %s", err)
	}

	var resp SlaveResponse
	if err = sc.decode.Decode(&resp); err != nil {
		return fmt.Errorf("JSON decode error on response: %s", err)
	}
	if resp.RespStatus != StatusOkay {
		return fmt.Errorf(resp.RespInfo)
	}
	return nil
}

func (sc *SlaveController) ClientStart() error {
	slaveOp := SlaveOp{OpType: SlaveOpStart}
	if err := sc.encode.Encode(&slaveOp); err != nil {
		return fmt.Errorf("Failed to start slave: %s\n", err)
	}
	var resp SlaveResponse
	if err := sc.decode.Decode(&resp); err != nil {
		return fmt.Errorf("JSON decode error on response: %s", err)
	}
	if resp.RespStatus != StatusOkay {
		return fmt.Errorf(resp.RespInfo)
	}
	return nil
}

func (sc *SlaveController) ClientWait() error {
	defer func() {
		sc.SlaveConn.Close()
	}()
	var slaveOp SlaveOp
	for {
		if err := sc.decode.Decode(&slaveOp); err != nil {
			return fmt.Errorf("JSON decode error: %s", err)
		}
		switch slaveOp.OpType {
		case SlaveFinished:
			return nil
		case SlaveFinishedStats:
			var stats SlaveStatReply
			if err := sc.decode.Decode(&stats); err != nil {
				return fmt.Errorf("JSON decode error on final stats: %s", err)
			}
			sc.Stats = stats.SlaveStats
		case SlaveIntermediateStats:
			var stats SlaveStatReply
			if err := sc.decode.Decode(&stats); err == nil {
				sc.StatChan <- stats.SlaveStats
			}
		}
	}
}

func (sc *SlaveController) ClientStop() {
	slaveOp := SlaveOp{OpType: SlaveOpStop}
	sc.encode.Encode(&slaveOp)
}

func (s *SlaveState) SlaveExecute(p *Printer) {
	var err error

	defer func() {
		s.SlaveConn.Close()
	}()

	s.encode = json.NewEncoder(s.SlaveConn)
	s.decode = json.NewDecoder(s.SlaveConn)
	s.printer = p
	s.workerCmpt = make(chan WorkerStat, 10)
	s.statChan = make(chan *WorkerStat, 10)

	var params SlaveParams
	if err = s.decode.Decode(&params); err != nil {
		p.Send("Failed to decode first params: %s\n", err)
		return
	}
	s.params = &params

	if s.prepTarget() == false {
		p.Send("Failed to prep target: %s\n", s.params.FileName)
		return
	}
	s.sendReply(&SlaveResponse{StatusOkay, "okay"})

	go s.intermediateStats()
	var slaveOp SlaveOp
	s.threadRunning = true
	for s.threadRunning {
		if err = s.decode.Decode(&slaveOp); err != nil {
			/*
			 * Look for a means to test the type of error. If the error is EOF
			 * that's expected with the master side closes the connection. However,
			 * other errors which might be a decoding error should have some
			 * status sent to the master.
			 */
			return
		}
		switch slaveOp.OpType {
		case SlaveOpWarmup:
			s.sendReply(&SlaveResponse{StatusOkay, "okay"})
		case SlaveOpStart:
			go s.slaveRun()
			s.sendReply(&SlaveResponse{StatusOkay, "okay"})
		case SlaveOpStop:
			p.Send("STOP requested\n")
			s.threadRunning = false
			s.sendReply(&SlaveResponse{StatusOkay, "okay"})
		default:
			p.Send("Invalid slave operation: %d\n", slaveOp.OpType)
			s.sendReply(&SlaveResponse{StatusError, fmt.Sprintf("invalid op %d", slaveOp.OpType)})
		}
	}
}

func (s *SlaveState) prepTarget() bool {
	var err error
	var fileInfo os.FileInfo
	openFLags := os.O_RDWR

	if _, err := os.Stat(s.params.FileName); err == nil {
		s.removeOnClose = false
	} else {
		s.removeOnClose = true
		openFLags |= os.O_CREATE
	}
	if s.targetDev, err = os.OpenFile(s.params.FileName, openFLags, 0666); err != nil {
		s.sendReply(&SlaveResponse{RespStatus: StatusError, RespInfo: fmt.Sprintf("%s", err)})
		return false
	}
	if fileInfo, err = s.targetDev.Stat(); err == nil {
		if fileInfo.Mode().IsRegular() {
			if fileInfo.Size() < s.params.FileSize {
				var bufSize int
				if s.params.FileSize < (1024 * 1024) {
					bufSize = int(s.params.FileSize)
				} else {
					bufSize = 1024 * 1024
				}
				bp := make([]byte, bufSize)
				for fillSize := s.params.FileSize; fillSize > 0; fillSize -= int64(bufSize) {
					if _, err := s.targetDev.Write(bp); err != nil {
						s.sendReply(&SlaveResponse{StatusError,
							fmt.Sprintf("failed to fill '%s'", s.params.FileName)})
						s.targetDev.Close()
						return false
					}
				}
			}
			if s.params.FileSize == 0 {
				s.params.FileSize = fileInfo.Size()
			}
			if s.params.FileSize == 0 {
				s.sendReply(&SlaveResponse{StatusError,
					fmt.Sprintf("no file size for '%s'", s.params.FileName)})
				s.targetDev.Close()
				return false
			}
			s.targetDev.Seek(0, 0)
			s.targetDev.Sync()
		} else {
			if pos, err := s.targetDev.Seek(0, 2); err != nil {
				s.sendReply(&SlaveResponse{StatusError,
					fmt.Sprintf("Unable to find size of device: '%s'", s.params.FileName)})
				s.targetDev.Close()
				return false
			} else {
				s.params.FileSize = pos
				s.targetDev.Seek(0, 0)
			}
		}
	} else {
		s.sendReply(&SlaveResponse{StatusError,
			fmt.Sprintf("stat failed after open on '%s'", s.params.FileName)})
	}

	if err := s.parseAccessPattern(); err != nil {
		s.sendReply(&SlaveResponse{RespStatus: StatusError,
			RespInfo: fmt.Sprintf("access Pattern err: %s", err)})
	}
	return true
}

/*
 * This code is copied directly from config.go. Need to look into using
 * interface{} so I only have one copy of the code.
 */
func (s *SlaveState) parseAccessPattern() error {
	var ok bool
	total := 0
	l := list.New()
	sections := strings.Split(s.params.AccessPattern, ",")
	if len(sections) == 0 {
		sections = []string{s.params.AccessPattern}
	}
	for _, sec := range sections {
		params := strings.Split(sec, ":")
		if len(params) != 3 {
			return fmt.Errorf("should be tuple of 3 '%s'", sec)
		} else {
			e := AccessPattern{}
			e.sectionPercent, _ = strconv.Atoi(params[0])
			total += e.sectionPercent
			rwPercentage := strings.Split(params[1], "|")
			if len(rwPercentage) == 1 {
				if e.opType, ok = accessType[params[1]]; !ok {
					return fmt.Errorf("invalid op %s", params[1])
				}
				e.readPercent = 50
			} else {
				if e.opType, ok = accessType[rwPercentage[0]]; !ok {
					return fmt.Errorf("invalid op %s", rwPercentage[0])
				}
				e.readPercent, _ = strconv.Atoi(rwPercentage[1])
			}
			if e.blkSize, ok = BlkStringToInt64(params[2]); !ok {
				return fmt.Errorf("invalid blksize: %s", params[2])
			}
			l.PushBack(e)
		}
	}
	if total > 100 {
		//noinspection GoPlaceholderCount
		return fmt.Errorf("more than 100%")
	}
	if total < 100 {
		e := AccessPattern{sectionPercent: 100 - total, opType: NoneType, blkSize: 0, readPercent: 0,
			lastBlk: 0, sectionStart: 0, sectionEnd: 0, buf: nil}
		l.PushBack(e)
	}
	s.params.accessPattern = l
	currentBlk := int64(0)
	for e := s.params.accessPattern.Front(); e != nil; e = e.Next() {
		access := e.Value.(AccessPattern)
		access.sectionStart = currentBlk
		currentBlk += s.params.FileSize * int64(access.sectionPercent) / 100
		access.sectionEnd = currentBlk - access.blkSize
		access.buf = make([]byte, access.blkSize)
		for i := range access.buf {
			access.buf[i] = byte(rand.Intn(256))
		}
		e.Value = access
	}
	return nil
}

func (s *SlaveState) slaveRun() {
	workersComplete := 0
	var avgResponse time.Duration = 0
	avgCount := 0

	defer func() {
		s.SlaveConn.Close()
		if s.removeOnClose {
			os.Remove(s.params.FileName)
		}
		s.targetDev.Close()
	}()

	for i := 0; i < s.params.IODepth; i++ {
		go s.slaveWorker(i)
	}
	s.totalStats.LowResponse = time.Hour * 24
	s.totalStats.Histogram = DistroInit(nil, "")
	if s.params.Verbose {
		DebugEnable()
	}
	start := time.Now()
	for {
		select {
		case worker := <-s.workerCmpt:
			s.addStats(&s.totalStats, &worker)
			avgResponse += worker.AvgResponse
			avgCount++

			workersComplete++
			if workersComplete < s.params.IODepth {
				continue
			}
			s.totalStats.AvgResponse = avgResponse / time.Duration(avgCount)
			s.totalStats.Elapsed = time.Now().Sub(start)

			s.sendReply(&SlaveOp{OpType: SlaveFinishedStats})
			s.sendReply(&SlaveStatReply{SlaveName: s.params.JobName, SlaveStats: s.totalStats})
			s.sendReply(&SlaveOp{OpType: SlaveFinished})
			s.threadRunning = false
			if s.params.Verbose {
				DebugDisable()
			}
			return
		}
	}
}

func (s *SlaveState) intermediateStats() {
	thrStats := &WorkerStat{}
	thrStats.Histogram = DistroInit(nil, "")
	checkin := 0
	for {
		select {
		case ws := <-s.statChan:
			s.addStats(thrStats, ws)
			checkin++
			if checkin < s.params.IODepth {
				continue
			}
			checkin = 0
			s.sendReply(&SlaveOp{OpType: SlaveIntermediateStats})
			s.sendReply(&SlaveStatReply{SlaveName: s.params.JobName, SlaveStats: *thrStats})
			thrStats = &WorkerStat{}
			thrStats.Histogram = DistroInit(nil, "")
		}
	}
}

func (s *SlaveState) addStats(parent, w *WorkerStat) {
	DebugLog("[]---- addStats ----[]\n")
	DebugIncrease()
	classPtr := reflect.ValueOf(parent).Elem()
	wPtr := reflect.ValueOf(w).Elem()
	for i := 0; i < classPtr.NumField(); i++ {
		classField := classPtr.Field(i)
		wField := wPtr.FieldByName(classPtr.Type().Field(i).Name)
		switch classField.Kind() {
		case reflect.Int64:
			switch classPtr.Type().Field(i).Name {
			case "LowResponse":
				DebugLog("Low: class(%d) field(%d)\n", classField.Int(), wField.Int())
				if classField.Int() > wField.Int() {
					classField.SetInt(wField.Int())
				}
			case "HighResponse":
				DebugLog("High: class(%d) field(%d)\n", classField.Int(), wField.Int())
				if classField.Int() < wField.Int() {
					classField.SetInt(wField.Int())
				}
			default:
				classField.SetInt(wField.Int() + classField.Int())
			}
		case reflect.Ptr:
			DebugLog("Name(%s), CanSet(%t), Kind(%s)\n", classPtr.Type().Field(i).Name, classField.CanSet(),
				classField.Kind())
			h := classPtr.Field(i).Addr().Elem().Interface().(*DistroGraph)
			lh := wPtr.FieldByName(classPtr.Type().Field(i).Name).Addr().Elem().Interface().(*DistroGraph)
			h.addBins(lh)
		}
	}
	DebugDecrase()
}

func (s *SlaveState) sendReply(v interface{}) {
	if err := s.encode.Encode(v); err != nil {
		s.printer.Send("sendReply error: err=%s\n", err)
	}
}

func (s *SlaveState) slaveWorker(id int) {
	var stats WorkerStat
	var totalLatency time.Duration = 0
	var totalCount = 0
	stats.Histogram = DistroInit(nil, "")
	tick := time.Tick(time.Second)
	boom := time.After(s.params.Runtime)
	stats.LowResponse = time.Hour * 24
	for s.threadRunning {
		select {
		case <-tick:
			s.statChan <- &stats
		case <-boom:
			stats.AvgResponse = totalLatency / time.Duration(totalCount)
			s.workerCmpt <- stats
			return
		default:
			ad := s.oneAD()
			start := time.Now()
			switch ad.op {
			case ReadBaseType:
				stats.Reads++
				stats.BytesRead += int64(len(ad.buf))
				if _, err := s.targetDev.ReadAt(ad.buf, ad.blk); err != nil {
					stats.ReadErrors++
				}
			case WriteBaseType:
				stats.Writes++
				stats.BytesWritten += int64(len(ad.buf))
				if _, err := s.targetDev.WriteAt(ad.buf, ad.blk); err != nil {
					stats.WriteErrors++
				}
			}
			latency := time.Now().Sub(start)
			if latency < stats.LowResponse {
				stats.LowResponse = latency
			}
			if latency > stats.HighResponse {
				stats.HighResponse = latency
			}
			stats.Histogram.Aggregate(latency)
			totalCount++
			totalLatency += latency
		}
	}
	stats.AvgResponse = totalLatency / time.Duration(totalCount)
	s.workerCmpt <- stats
	s.printer.Send("[%d] thread halted\n", id)
}

func (s *SlaveState) oneAD() AccessData {
	ad := AccessData{}
	section := rand.Intn(100)
	/*
	 * Looping through a linked list in a high call method isn't a good
	 * idea normally. Luckily accessPattern normally is a small list (<3)
	 * that will not add much to the overhead.
	 */
	for e := s.params.accessPattern.Front(); e != nil; e = e.Next() {
		access := e.Value.(AccessPattern)
		// If the current requested section is less than the percentage
		// of the section being worked on we've found range to work with.
		// Otherwise, subtract the current range from the section and
		// go to the next one.
		if access.sectionPercent > section {
			ad.buf = access.buf

			// Generate the block number for the next request.
			switch access.opType {
			case ReadSeqType, WriteSeqType, RwseqType:
				access.lastBlk += access.blkSize
				if access.lastBlk >= access.sectionEnd {
					access.lastBlk = access.sectionStart
				}
				ad.blk = access.lastBlk
			case ReadRandType, WriteRandType, RwrandType:
				randBlk := rand.Int63n((access.sectionEnd - access.sectionStart - int64(len(ad.buf))) / 512)
				ad.blk = randBlk*512 + access.sectionStart
				access.lastBlk = ad.blk
			default:
				fmt.Printf("Invalid opType=%d ... should be impossible", access.opType)
				os.Exit(1)
			}
			// We get a copy of the element stored in the list. So, update the element
			// with any changes that have been made.
			e.Value = access

			// Now set the op type.
			switch access.opType {
			case ReadRandType, ReadSeqType:
				ad.op = ReadBaseType
			case WriteRandType, WriteSeqType:
				ad.op = WriteBaseType
			case RwseqType, RwrandType:
				if rand.Intn(100) < access.readPercent {
					ad.op = ReadBaseType
				} else {
					ad.op = WriteBaseType
				}
			}
			break
		} else {
			section -= access.sectionPercent
		}
	}
	return ad
}
