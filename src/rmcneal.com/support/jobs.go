package support

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

type AccessData struct {
	blk int64
	op  int
	buf []byte
}

type JobReport struct {
	JobID       int
	Name        string
	ReadErrors  int
	WriteErrors int
	ReadIOs     int
	WriteIOs    int
}

type Job struct {
	TargetName   string
	JobParams    *JobData
	Stats        *StatsState
	pathName     string
	fp           *os.File
	lastErr      error
	lcgPattern   *RandLCG
	lcgBlk       *RandLCG
	threadRun    bool
	remove       bool
	nextBlks     chan AccessData
	thrCompletes chan JobReport
	bailOnError  bool
	statIdx      int
	validInit    bool
}

func (j *Job) Init(tracker *tracking) error {
	jd := j.JobParams
	j.validInit = false
	openFlags := os.O_RDWR
	if jd.Name[0] == '/' {
		j.pathName = jd.Name
	} else {
		j.pathName = jd.Directory + "/" + jd.Name
	}
	if _, err := os.Stat(j.pathName); err == nil {
		j.remove = false
	} else {
		j.remove = true
		openFlags |= os.O_CREATE
	}
	if j.fp, j.lastErr = os.OpenFile(j.pathName, openFlags, 0666); j.lastErr != nil {
		return j.lastErr
	}
	j.thrCompletes = make(chan JobReport)
	j.nextBlks = make(chan AccessData, 1000)
	j.lcgPattern = new(RandLCG)
	j.lcgPattern.Init()
	j.lcgBlk = new(RandLCG)
	j.lcgBlk.Init()
	j.threadRun = false
	j.bailOnError = false
	if fileinfo, err := j.fp.Stat(); err == nil {
		if fileinfo.Mode().IsRegular() {
			if fileinfo.Size() < j.JobParams.fileSize {
				j.fileFill(tracker)
				j.fp.Seek(0, 0)
				j.fp.Sync()
			} else if j.JobParams.fileSize == 0 {
				j.JobParams.fileSize = fileinfo.Size()
				j.JobParams.Size = Humanize(j.JobParams.fileSize, 1)
			}
			if j.JobParams.fileSize == 0 {
				return fmt.Errorf("must set file size or use preexisting file")
			}
		} else {
			if pos, err := j.fp.Seek(0, 2); err != nil {
				return err
			} else {
				if pos == 0 {
					pos = j.JobParams.fileSize
				}
				if pos == 0 {
					return fmt.Errorf("can't find the size of device")
				}
				j.JobParams.fileSize = pos
				j.fp.Seek(0, 0)
			}
			j.JobParams.Size = Humanize(j.JobParams.fileSize, 1)
		}
	} else {
		return err
	}

	if j.JobParams.Verbose {
		j.statIdx = j.Stats.NextHistogramIdx()
		j.Stats.Send(StatsRecord{OpType: StatSetHistogram, opSize: j.JobParams.fileSize, opIdx: j.statIdx})
	}

	currentBlk := int64(0)
	for e := j.JobParams.accessPattern.Front(); e != nil; e = e.Next() {
		access := e.Value.(AccessPattern)
		access.sectionStart = currentBlk
		currentBlk += j.JobParams.fileSize * int64(access.sectionPercent) / 100
		access.sectionEnd = currentBlk - access.blkSize
		access.buf = make([]byte, access.blkSize)
		j.patternFill(access.buf[0:access.blkSize])
		e.Value = access
	}

	j.validInit = true
	return nil
}

func (j *Job) ShowConfig() {
	str := map[string]string{
		"size":    "File Size",
		"bitmap":  "Bitmap",
		"iodepth": "IODepth",
	}
	maxStr := 0
	for _, value := range str {
		if len(value) > maxStr {
			maxStr = len(value)
		}
	}
	fmt.Printf("\t%*s: %s\n", maxStr, str["size"], Humanize(j.JobParams.fileSize, 1))
	fmt.Printf("\t%*s: %d\n", maxStr, str["iodepth"], j.JobParams.IODepth)
}

func (j *Job) GetName() string {
	return j.TargetName
}

func (j *Job) GetJobdata() *JobData {
	return j.JobParams
}

func (j *Job) Start(jobExits chan JobReport) {
	thrExit := 0
	keepRunning := true
	finalReport := JobReport{ReadErrors: 0, WriteErrors: 0, Name: j.TargetName}

	if j.validInit == false {
		jobExits <- finalReport
		return
	}
	j.threadRun = true
	boom := time.After(j.JobParams.runtime)

	if j.JobParams.delayStart > 0 {
		time.Sleep(j.JobParams.delayStart)
	}

	go j.genAccessData()
	for i := 0; i < j.JobParams.IODepth; i++ {
		go j.ioWorker(i)
	}

	for keepRunning {
		select {
		case rpt := <-j.thrCompletes:
			finalReport.ReadErrors += rpt.ReadErrors
			finalReport.WriteErrors += rpt.WriteErrors
			finalReport.ReadIOs += rpt.ReadIOs
			finalReport.WriteIOs += rpt.WriteIOs
			thrExit++
			if thrExit == j.JobParams.IODepth {
				// Once all of the ioWorker threads and generation thread
				// have been collected end the loop here so that the
				// main loop can collect the threads it's waiting
				// for.
				keepRunning = false
				break
			}
		case <-boom:
			// By setting threadRun to false the ioWorker threads
			// will exit which will cause them to send a 1 to the
			// thrCompletes channel.
			j.threadRun = false
			break
		}
	}

	// Signal the main loop that we've completed.
	jobExits <- finalReport
}

func (j *Job) Fini() {
	j.fp.Close()
	if j.remove {
		os.Remove(j.pathName)
	}
}

func (j *Job) Error() string {
	return fmt.Sprintf("%s: [%s]", j.TargetName, j.lastErr)
}

// []--------------------------------------------------------------[]
// | Non public class methods										|
// []--------------------------------------------------------------[]

func (j *Job) patternFill(bp []byte) {
	switch {
	case j.JobParams.Block_Pattern == PatternRand:
		for i := range bp {
			bp[i] = byte(rand.Intn(256))
		}
	case j.JobParams.Block_Pattern == PatternIncr:
		for i := range bp {
			bp[i] = byte(i)
		}
	case j.JobParams.Block_Pattern == PatternLCG:
		for i := range bp {
			bp[i] = byte(j.lcgPattern.Value(256))
		}
	case j.JobParams.Block_Pattern == PatternZero:
		for i := range bp {
			bp[i] = byte(0)
		}
	}
}

func (j *Job) oneAD() AccessData {
	ad := AccessData{}
	section := rand.Intn(100)
	for e := j.JobParams.accessPattern.Front(); e != nil; e = e.Next() {
		access := e.Value.(AccessPattern)
		// If the current requeted section is less than the percentage
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

func (j *Job) fileFill(tracker *tracking) {
	fillJobs := 16
	lastBlock := j.JobParams.fileSize
	fillSize := int64(1024 * 1024)
	buf := make([]byte, fillSize)
	j.patternFill(buf)
	j.threadRun = true
	for i := 0; i < fillJobs; i++ {
		go j.ioWorker(i)
	}
	go func() {
		for curBlock := int64(0); curBlock < lastBlock; curBlock += fillSize {
			ad := AccessData{op: WriteBaseType, buf: buf, blk: curBlock}
			j.nextBlks <- ad
		}
		for i := 0; i < fillJobs; i++ {
			ad := AccessData{op: StopType}
			j.nextBlks <- ad
		}
	}()
	ticker := time.Tick(time.Second)
	for {
		select {
		case <-ticker:
			if fileinfo, err := j.fp.Stat(); err != nil {
				fmt.Printf("Stat(%s) failed; %s\n", err, j.JobParams.Name)
				return
			} else {
				tracker.UpdateName(j.TargetName, fmt.Sprintf("-%.1f", float64(fileinfo.Size())/float64(lastBlock)*100.0))
			}
		case <-j.thrCompletes:
			fillJobs--
			if fillJobs == 0 {
				j.threadRun = false
				return
			}
		}
	}
}

func (j *Job) genAccessData() {
	for {
		j.nextBlks <- j.oneAD()
	}
}

func (ad *AccessData) String() string {
	return fmt.Sprintf("op=%s, blk=0x%x, size=%s\n", opToString(ad.op), ad.blk, Humanize(int64(len(ad.buf)), 1))
}

func opToString(op int) string {
	switch op {
	case ReadBaseType:
		return "Read"
	case WriteBaseType:
		return "Write"
	case NoneType:
		return "None"
	default:
		return "Unknown"
	}
}

func (j *Job) Stop() {
	j.threadRun = false
}

func (j *Job) ioWorker(workId int) {
	var statType int
	rpt := JobReport{JobID: workId, ReadErrors: 0, WriteErrors: 0, ReadIOs: 0, WriteIOs: 0}
	opCnt := 0
	for j.threadRun {
		ad := <-j.nextBlks
		ioStart := time.Now()
		switch ad.op {
		case ReadBaseType:
			statType = StatRead
			rpt.ReadIOs++
			if _, err := j.fp.ReadAt(ad.buf, ad.blk); err != nil {
				rpt.ReadErrors++
				if j.bailOnError {
					fmt.Printf("ReadAt error(0x%x:0x%x)\n  : %s\n", ad.blk, len(ad.buf), err)
					j.threadRun = false
					break
				} else {
					continue
				}
			}
		case WriteBaseType:
			statType = StatWrite
			rpt.WriteIOs++
			if _, err := j.fp.WriteAt(ad.buf, ad.blk); err != nil {
				rpt.WriteErrors++
				if j.bailOnError {
					fmt.Printf("WriteAt error(0x%x:0x%x)\n  : %s\n", ad.blk, len(ad.buf), err)
					j.threadRun = false
					break
				} else {
					continue
				}
			}
		case StopType:
			j.thrCompletes <- rpt
			return
		}
		ioDuration := time.Now().Sub(ioStart)
		if (j.JobParams.Fsync != 0) && (opCnt >= j.JobParams.Fsync) {
			opCnt = 0
			j.fp.Sync()
		}
		j.Stats.Send(StatsRecord{opSize: int64(len(ad.buf)), OpType: statType, opDuration: ioDuration,
			opBlk: ad.blk, opIdx: j.statIdx})
	}
	j.thrCompletes <- rpt
}
