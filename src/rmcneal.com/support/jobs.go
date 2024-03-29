package support

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

type AccessData struct {
	blk int64
	op  int
	len	int64
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
	startTime    time.Time
}

func JobInit(name string, jd *JobData, stats *StatsState) (*Job, error) {
	j := &Job{TargetName: name, JobParams: jd, Stats: stats}
	j.JobParams = jd
	j.validInit = false
	openFlags := os.O_RDWR
	// Used when writing out validation blocks
	j.startTime = time.Now()
	if jd.Name[0] == '/' {
		j.pathName = jd.Name
	} else {
		j.pathName = jd.Directory + "/" + jd.Name
	}
	if _, err := os.Stat(j.pathName); err == nil {
		j.remove = false
	} else {
		j.remove = !j.JobParams.Save_On_Create
		openFlags |= os.O_CREATE
	}
	if j.fp, j.lastErr = os.OpenFile(j.pathName, openFlags, 0666); j.lastErr != nil {
		return nil, j.lastErr
	}
	j.thrCompletes = make(chan JobReport)
	j.nextBlks = make(chan AccessData, 1000)
	j.lcgPattern = new(RandLCG)
	j.lcgPattern.Init()
	j.lcgBlk = new(RandLCG)
	j.lcgBlk.Init()
	j.threadRun = false
	j.bailOnError = true
	if fileinfo, err := j.fp.Stat(); err == nil {
		if fileinfo.Mode().IsRegular() {
			if j.JobParams.fileSize == 0 {
				j.JobParams.fileSize = fileinfo.Size()
				j.JobParams.Size = Humanize(j.JobParams.fileSize, 1)
			}
		} else {
			if pos, err := j.fp.Seek(0, 2); err != nil {
				return nil, err
			} else {
				// Override the size of the device with what the user specified.
				if j.JobParams.fileSize != 0 {
					pos = j.JobParams.fileSize
				}
				if pos == 0 {
					return nil, fmt.Errorf("can't find the size of device")
				}
				j.JobParams.fileSize = pos
			}
			j.JobParams.Size = Humanize(j.JobParams.fileSize, 1)

		}
	} else {
		return nil, err
	}

	if j.JobParams.Verbose {
		j.statIdx = j.Stats.NextHistogramIdx()
		j.Stats.Send(StatsRecord{OpType: StatSetHistogram, opSize: j.JobParams.fileSize, opIdx: j.statIdx})
	}

	currentBlk := int64(0)
	for e := j.JobParams.accessPattern.Front(); e != nil; e = e.Next() {
		access := e.Value.(AccessPattern)
		access.sectionStart = currentBlk
		access.lastBlk = currentBlk
		currentBlk += j.JobParams.fileSize * int64(access.sectionPercent) / 100
		access.sectionEnd = currentBlk - access.blkSize
		e.Value = access
	}

	j.validInit = true
	return j, nil
}

func (j *Job) FillAsNeeded(tracker *tracking) error {
	var fileinfo  os.FileInfo

	if fileinfo, j.lastErr = j.fp.Stat(); j.lastErr == nil {
		if fileinfo.Mode().IsRegular() {
			if fileinfo.Size() < j.JobParams.fileSize {
				j.fileFill(tracker)
				if j.lastErr == nil {
					_, _ = j.fp.Seek(0, 0)
					tracker.UpdateName(j.TargetName, "(syncing)")
					_ = j.fp.Sync()
					// Reload the stat structure after filling the file
					fileinfo, _ = j.fp.Stat()
				}
			}
			j.JobParams.fileSize = fileinfo.Size()
			j.JobParams.Size = Humanize(j.JobParams.fileSize, 1)
			if j.JobParams.fileSize == 0 {
				j.validInit = false
				return fmt.Errorf("must set file size or use a preexisting file")
			}
		} else {
			// This should really only be done if the configuration is going to request
			// some variant of a verify operation which will need the data pattern
			// correctly laid out on the device.
			if j.JobParams.Force_Fill {
				j.fileFill(tracker)
			}

			_, _ = j.fp.Seek(0, 0)
		}
	}

	// lastErr can also be set by calling AbortPrep()
	return j.lastErr
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

func (j *Job) Start() {
	thrExit := 0
	keepRunning := true
	finalReport := JobReport{ReadErrors: 0, WriteErrors: 0, Name: j.TargetName}

	if j.validInit == false {
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
			// By setting threadRun to false genAccessData() will stop
			// generating access data and send IODepth number of Stop
			// requests down the channel. That it turn will cause the
			// workers to stop.
			j.threadRun = false
			break
		}
	}
}

func (j *Job) Fini() {
	_ = j.fp.Close()
	if j.remove {
		_ = os.Remove(j.pathName)
	}
}

func (j *Job) Error() string {
	return fmt.Sprintf("%s: [%s]", j.TargetName, j.lastErr)
}

// []--------------------------------------------------------------[]
// | Non public class methods										|
// []--------------------------------------------------------------[]

func (j *Job) patternFill(bp []byte) {
	var up *int64
	var engine func()int64

	switch {
	case j.JobParams.Block_Pattern == PatternRand:
		engine = rand.Int63
	case j.JobParams.Block_Pattern == PatternLCG:
		engine = j.lcgPattern.Value63
	}

	slice := (*reflect.SliceHeader)(unsafe.Pointer(&bp))
	for offset := 0; offset < len(bp); offset += 8 {
		up = (*int64)(unsafe.Pointer(uintptr(unsafe.Pointer(slice.Data)) + uintptr(offset)))
		*up = engine()
	}
}

func (j *Job) genAccessData() {
	for j.threadRun {
		j.nextBlks <- j.oneAD()
	}

	for i := 0; i < j.JobParams.IODepth; i++ {
		j.nextBlks <- AccessData{0, StopType, 0}
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
			ad.len = access.blkSize
			// Generate the block number for the next request.
			switch access.opType {
			case ReadSeqType, WriteSeqType, RwseqType, ReadSeqVerifyType:
				access.lastBlk += access.blkSize
				if access.lastBlk >= access.sectionEnd {
					access.lastBlk = access.sectionStart
				}
				ad.blk = access.lastBlk

			case ReadRandType, WriteRandType, RwrandType, RwrandVerifyType:
				randBlk := rand.Int63n((access.sectionEnd - access.sectionStart - ad.len) / 512)
				ad.blk = randBlk*512 + access.sectionStart

			case NoneType:
				ad.blk = 0

			default:
				fmt.Printf("\nInvalid opType=%d ... should be impossible\n", access.opType)
				os.Exit(1)
			}

			// We get a copy of the element stored in the list. So, update the element
			// with any changes that have been made.
			e.Value = access

			// Now set the op type.
			switch access.opType {
			case ReadRandType, ReadSeqType:
				ad.op = ReadBaseType

			case ReadSeqVerifyType:
				ad.op = ReadBaseVerifyType

			case WriteRandType, WriteSeqType:
				ad.op = WriteBaseType

			case RwseqType, RwrandType:
				if rand.Intn(100) < access.readPercent {
					ad.op = ReadBaseType
				} else {
					ad.op = WriteBaseType
				}

			case RwrandVerifyType:
				if rand.Intn(100) < access.readPercent {
					ad.op = ReadBaseVerifyType
				} else {
					ad.op = WriteBaseVerifyType
				}

			case NoneType:
				ad.op = NoneType
			}
			break
		} else {
			section -= access.sectionPercent
		}
	}
	return ad
}

func (j *Job) fileFill(tracker *tracking) {
	j.JobParams.Force_Fill = true
	fillJobs := j.JobParams.IODepth
	lastBlock := j.JobParams.fileSize
	fillSize := int64(1024 * 1024)
	buf := make([]byte, fillSize)
	j.patternFill(buf)
	j.threadRun = true

	// Make sure when filling the file for the first time to use unique data
	// in every block. This will prevent file systems like ZFS from collapsing
	// the blocks to nothing and just a meta data reference. The aim is to
	// cause file systems to actually read data from the file system.
	savedResetCnt := j.JobParams.Reset_Buf
	j.JobParams.Reset_Buf = 1
	defer func () {
		j.JobParams.Reset_Buf = savedResetCnt
	}()

	go func() {
		var curBlock int64
		for curBlock = int64(0); (curBlock + fillSize) <= lastBlock; curBlock += fillSize {
			if !j.threadRun {
				for i := 0; i < j.JobParams.IODepth; i++ {
					j.thrCompletes <- JobReport{}
				}
				break
			}
			ad := AccessData{op: WriteBaseVerifyType, blk: curBlock, len: int64(1024 * 1024)}
			j.nextBlks <- ad
		}

		// If the device size is not a multiple of our initialization buffer there'll be chunk
		// at the end which doesn't get initialized, but during the actual run the worker
		// will access the data. So, create a final write request that accounts for that
		// last little bit.
		if j.threadRun && (lastBlock - curBlock) > 0 {
			ad := AccessData{op: WriteBaseVerifyType, blk: curBlock, len: lastBlock - curBlock}
			j.nextBlks <- ad
		}

		for i := 0; i < fillJobs; i++ {
			ad := AccessData{op: StopType}
			j.nextBlks <- ad
		}

	}()

	for i := 0; i < fillJobs; i++ {
		go j.ioWorker(i)
	}

	ticker := time.Tick(time.Second)
	startTime := time.Now()
	lastReportedSize := int64(0)
	fakeETA := int64(0)

	for {
		select {
		case <-ticker:
			fi, _ := j.fp.Stat()
			elapsed := time.Since(startTime)
			etaStr := ""

			bytesPerSecond := fi.Size() / int64(elapsed.Seconds())
			if bytesPerSecond == 0 {
				bytesPerSecond = 1 // Avoid divide by zero
			}
			secondsRemaining := (j.JobParams.fileSize - fi.Size()) / bytesPerSecond

			/*
			 * The whole business of lasteReportedSize and fake ETA is due to FileInfo.Size()
			 * not being updated with the actual file size on demand. It gets updated once every
			 * 60 to 75 seconds. This is not a bug in Go since one can see the same behavior using
			 * 'ls -l' on the file. This is also not unique to one particular flavor of Unix.
			 * It's a common practice of file system code to not report the in core data blocks, only
			 * the data actually on disk which kind of makes sense.
			 */
			if fi.Size() != lastReportedSize {
				etaStr = fmt.Sprintf(" (ETA:%s)",
					time.Duration(time.Duration(secondsRemaining)*time.Second))
				lastReportedSize = fi.Size()
				fakeETA = secondsRemaining
			} else if fakeETA != 0 {
				etaStr = fmt.Sprintf(" (ETA:%s)", time.Duration(time.Duration(fakeETA)*time.Second))
				fakeETA--
			} else {
				/*
				 * If we've run out of time on the fake ETA reset lastRecordSize so that the
				 * seconds remaining will get recalculated next time through the loop. Better
				 * than nothing.
				 */
				lastReportedSize = 0
			}

			tracker.UpdateName(j.TargetName, fmt.Sprintf(":%.1f%s",
				float64(fi.Size())/float64(j.JobParams.fileSize)*100.0, etaStr))

		case <-j.thrCompletes:
			fillJobs--
			if fillJobs == 0 {
				j.threadRun = false
				return
			}
		}
	}
}

func (ad *AccessData) String() string {
	return fmt.Sprintf("op=%s, blk=0x%x, size=%s\n", opToString(ad.op), ad.blk, Humanize(ad.len, 1))
}

func opToString(op int) string {
	switch op {
	case ReadBaseType:
		return "Read"
	case WriteBaseType:
		return "Write"
	case ReadBaseVerifyType:
		return "ReadVerify"
	case WriteBaseVerifyType:
		return "WriteVerify"
	case NoneType:
		return "None"
	default:
		return "Unknown"
	}
}

func (j *Job) Stop() {
	j.threadRun = false
}

func (j *Job) AbortPrep() {
	j.lastErr = fmt.Errorf("forced abort of prep")
	j.threadRun = false
}

const (
	MarkerSig = 0xdeadbeef00ff1122
)

type markerBlock struct {
	blockNumber int64
	signature   uint64
	tMarker     time.Time
	targetName  [64]byte
}

func (j *Job) validateBuf(buf []byte, blockNum int64) bool {
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	for offset := 0; offset < len(buf); offset += 512 {
		marker := (*markerBlock)(unsafe.Pointer(uintptr(unsafe.Pointer(slice.Data)) + uintptr(offset)))
		if marker.signature != MarkerSig {
			fmt.Printf("Invalid signature at block: 0x%x, offset: 0x%x\n", blockNum, offset)
			return false
		}

		if marker.blockNumber != blockNum {
			fmt.Printf("Bad block at block/offset: 0x%x/%x, found 0x%x\n", blockNum, offset, marker.blockNumber)
			return false
		}
		blockNum += 512

		bp := bytes.NewBuffer(marker.targetName[:len(j.TargetName)])
		if strings.Compare(bp.String(), j.TargetName) != 0 {
			fmt.Printf("Bad name in block: 0x%x/%x -- Got %s, Found %s\n", blockNum, offset,
				bp.String(), j.TargetName)
			fmt.Printf("len(bp)=%d, len(Targetname)=%d\n", len(bp.String()), len(j.TargetName))
			return false
		}

		// Only check the timestamp if this instance prefilled the target. Else we're using a previous
		// run which means this check is guaranteed to fail and that's not what's wanted.
		if j.JobParams.Force_Fill && marker.tMarker != j.startTime {
			fmt.Printf("Stale block: 0x%x\n", marker.blockNumber)
			return false
		}
	}
	return true
}

func (j *Job) initBuf(buf []byte, blockNum int64) {

	slice := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	for offset := 0; offset < len(buf); offset += 512 {
		marker := (*markerBlock)(unsafe.Pointer(uintptr(unsafe.Pointer(slice.Data)) + uintptr(offset)))
		marker.blockNumber = blockNum
		blockNum += 512
		marker.signature = MarkerSig
		marker.tMarker = j.startTime
		copy(marker.targetName[:], j.TargetName)
	}
}

func (j *Job) ioWorker(workId int) {
	var statType int
	var buf []byte

	lastSizeUsed := int64(0)
	resetBufCount := 0
	rpt := JobReport{JobID: workId, ReadErrors: 0, WriteErrors: 0, ReadIOs: 0, WriteIOs: 0}
	opCnt := 0
	for {
		ad := <-j.nextBlks
		if ad.len != lastSizeUsed {
			lastSizeUsed = ad.len
			buf = make([]byte, lastSizeUsed)
			j.patternFill(buf)
		}
		ioStart := time.Now()
		switch ad.op {
		case ReadBaseType, ReadBaseVerifyType:
			statType = StatRead
			rpt.ReadIOs++
			if _, err := j.fp.ReadAt(buf, ad.blk); err != nil {
				rpt.ReadErrors++
				if j.bailOnError {
					fmt.Printf("ReadAt error(0x%x:0x%x) : %s\n", ad.blk, ad.len, err)
					j.threadRun = false
					break
				} else {
					continue
				}
			}
			if ad.op == ReadBaseVerifyType {
				if !j.validateBuf(buf, ad.blk) {
					j.threadRun = false
				}
			}
		case WriteBaseType, WriteBaseVerifyType:
			statType = StatWrite
			rpt.WriteIOs++
			if ad.op == WriteBaseVerifyType {
				j.initBuf(buf, ad.blk)
			} else {
				if (resetBufCount % j.JobParams.Reset_Buf) == 0 {
					j.patternFill(buf)
				}
				resetBufCount += 1
			}
			if _, err := j.fp.WriteAt(buf, ad.blk); err != nil {
				rpt.WriteErrors++
				if j.bailOnError {
					fmt.Printf("WriteAt error(0x%x:0x%x)\n  : %s\n", ad.blk, ad.len, err)
					j.threadRun = false
					break
				} else {
					continue
				}
			}
		case NoneType:
			continue
		case StopType:
			j.thrCompletes <- rpt
			return
		}
		ioDuration := time.Now().Sub(ioStart)
		if (j.JobParams.Fsync != 0) && (opCnt >= j.JobParams.Fsync) {
			opCnt = 0
			_ = j.fp.Sync()
		}
		j.Stats.Send(StatsRecord{opSize: ad.len, OpType: statType, opDuration: ioDuration,
			opBlk: ad.blk, opIdx: j.statIdx})
	}
}
