package support

import (
	"container/list"
	"fmt"
	"gopkg.in/gcfg.v1"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	Rwrand        = "rw"
	ReadSeq       = "read"
	ReadSeqVerify = "readv"
	ReadRand      = "randread"
	WriteSeq      = "write"
	WriteRand     = "randwrite"
	Rwseq         = "rwseq"
	PatternZero   = "zero"
	PatternRand   = "rand"
	PatternIncr   = "incr"
	PatternLCG    = "lcg"
	RwrandVerify  = "rwv"
	None          = "none"
)
const (
	_           = iota
	ReadSeqType = iota + 1
	ReadRandType
	ReadSeqVerifyType
	WriteSeqType
	WriteRandType
	RwrandType
	RwseqType
	ReadBaseType
	ReadBaseVerifyType
	WriteBaseType
	WriteBaseVerifyType
	RwrandVerifyType
	NoneType
	StopType // Used to halt fileFill loop jobs
)

//noinspection GoSnakeCaseUsage
type JobData struct {
	/*
	 * Don't change the names of these members without really thinking about it.
	 * The config file parsing code only works with names which have the first
	 * letter capitalized and '_' is used in place of a dash. So, Block_Pattern
	 * here becomes block-pattern in the file. Changes to the name will require
	 * config file changes.
	 */
	Version            int
	Directory          string
	Name               string
	Block_Pattern      string
	IODepth            int
	Size               string
	Runtime            string
	Rate               int
	Verbose            bool
	Record_Time        string
	Record_File        string
	Delay_Start        string
	Barrier            bool
	Job_Order          string
	Fsync              int
	Access_Pattern     string
	Linear             string
	Slave_Host         string
	Intermediate_Stats string
	Save_On_Create     bool
	Force_Fill         bool
	Reset_Buf		int

	// To make things easier for the user certain values
	// in the config file need to be processed beyond
	// what the gcfg package can handle. For example
	// the block size can be 4096 or 4k. These struct
	// values are what will be really used during a run.
	fileSize          int64
	runtime           time.Duration
	recordTime        time.Duration
	delayStart        time.Duration
	rateDelay         time.Duration
	intermediateStats time.Duration
	jobOrder          []string
	barrierOrder      [][]string
	accessPattern     *list.List
	linearParams      [3]time.Duration
	doLinear          bool
}

var accessType map[string]int
var accessStrMap map[int]string

type Configs struct {
	Global JobData
	Job    map[string]*JobData
}

func init() {
	accessType = map[string]int{}
	accessType[ReadSeq] = ReadSeqType
	accessType[ReadRand] = ReadRandType
	accessType[ReadSeqVerify] = ReadSeqVerifyType
	accessType[WriteSeq] = WriteSeqType
	accessType[WriteRand] = WriteRandType
	accessType[Rwrand] = RwrandType
	accessType[Rwseq] = RwseqType
	accessType[RwrandVerify] = RwrandVerifyType
	accessType[None] = NoneType
	accessStrMap = map[int]string{}
	accessStrMap[ReadSeqType] = ReadSeq
	accessStrMap[ReadSeqVerifyType] = ReadSeqVerify
	accessStrMap[ReadRandType] = ReadRand
	accessStrMap[WriteSeqType] = WriteSeq
	accessStrMap[WriteRandType] = WriteRand
	accessStrMap[RwrandType] = Rwrand
	accessStrMap[RwseqType] = Rwseq
	accessStrMap[RwrandVerifyType] = RwrandVerify
	accessStrMap[NoneType] = None
}

func ReadConfig(filename string) (*Configs, error) {
	var err error

	cfg := &Configs{}

	if err = gcfg.ReadFileInto(cfg, filename); err != nil {
		return nil, err
	}

	if err := cfg.validateGlobal(); err != nil {
		return nil, err
	}
	if err := cfg.UpdateJobs(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (j *JobData) GetRuntime() time.Duration {
	return j.runtime
}

func (c *Configs) GetJobsList() *[]string {
	return &c.Global.jobOrder
}

func (c *Configs) GetBarrierOrder() *[][]string {
	return &c.Global.barrierOrder
}

func (c *Configs) GetIntermediateStats() time.Duration {
	return c.Global.intermediateStats
}

func (j *JobData) GetJobConfig() map[string]string {
	d := map[string]string{}
	d["directory"] = j.Directory
	d["name"] = j.Name
	d["block-pattern"] = j.Block_Pattern
	d["iodepth"] = string(j.IODepth)
	d["file-size"] = Humanize(j.fileSize, 1)
	d["runtime"] = fmt.Sprintf("%d", j.runtime)
	d["verbose"] = strconv.FormatBool(j.Verbose)
	d["record-time"] = string(j.recordTime)
	d["record-file"] = j.Record_File
	d["delay-start"] = string(j.delayStart)
	d["barrier"] = strconv.FormatBool(j.Barrier)
	d["job-order"] = fmt.Sprintf("%s", j.jobOrder)
	d["fsync"] = string(j.Fsync)
	return d
}

func apOpTypeToString(op int) string {
	return accessStrMap[op]
}

func (ap *AccessPattern) String() string {
	return fmt.Sprintf("Percent %d%%, Op=%s, Block Size=%s, Start=0x%x, End=0x%x", ap.sectionPercent,
		apOpTypeToString(ap.opType), Humanize(ap.blkSize, 1), ap.sectionStart, ap.sectionEnd)
}

// []--------------------------------------------------------------[]
// | Non public class methods										|
// []--------------------------------------------------------------[]

type AccessPattern struct {
	sectionPercent int
	opType         int
	blkSize        int64

	// For read/write operations the percentage of Reads verses Writes can be
	// changed. The default will be 50/50.
	readPercent int

	// Used to hold last block created for this section. Primarily needed
	// for seqential access so that different threads receive the next
	// block in sequence.
	lastBlk int64

	// Block ranges for this section. These will be computed once the
	// underlying storage has been opened and it's size determined
	sectionStart int64
	sectionEnd   int64
}

//
// parseAccessPattern -- convert one or more tuples into an AccessPattern
//
// <SectionSize>:<Operation>[|<readPercent>]:<BlockSize>[,]
//
// The <SectionSize> is the percentage of the volume that this tuple will work on. Tuples start
// at 0, are additive, and can't add up to more than 100.
//
// <Operation> is one of read, write, rw, randread, randwrite, or rwseq. If the <Operation> is
// followed by an optional '|' and an integer the value will be the percentage of Reads in the given
// area.
//
// <BlockSize> used for I/O.
//
// Each tuple can be followed by an optional ',' indicated more to follow.
//
func (j *JobData) parseAccessPattern() error {
	var ok bool
	total := 0
	l := list.New()
	sections := strings.Split(j.Access_Pattern, ",")
	if len(sections) == 0 {
		sections = []string{j.Access_Pattern}
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
			lastBlk: 0, sectionStart: 0, sectionEnd: 0}
		l.PushBack(e)
	}
	j.accessPattern = l
	return nil
}

func (j *JobData) validate(section string) error {
	var err error
	if j.Access_Pattern != "" {
		if err = j.parseAccessPattern(); err != nil {
			return fmt.Errorf("[%s] contains invalid access pattern '%s', specific portion '%s'", section, j.Access_Pattern, err)
		}
	}
	if j.Name, err = EnvStrReplace(j.Name); err != nil {
		return err
	}
	if j.Directory != "" {
		var fi os.FileInfo
		if j.Directory, err = EnvStrReplace(j.Directory); err != nil {
			return err
		}
		if fi, err = os.Stat(j.Directory); err != nil {
			return fmt.Errorf("[%s] %s doesn't exist [%s]", section, j.Directory, err)
		}
		if fi.Mode().IsDir() != true {
			return fmt.Errorf("[%s] %s exists, but isn't a directory", section, j.Directory)
		}
	} else {
		j.Directory = "./"
	}

	if j.IODepth == 0 {
		j.IODepth = 8
	}

	if j.Size == "" {
		j.Size = "0"
	}
	var ok bool

	if j.fileSize, ok = BlkStringToInt64(j.Size); !ok {
		return fmt.Errorf("[second %s]/file-size: %s", section, j.Size)
	}

	if j.Runtime == "" {
		j.Runtime = "24h"
	}
	/*
	 * ParseDuration only goes up to 'h' for hours. So, deal with 'd' as a specical case.
	 */
	if strings.HasSuffix(j.Runtime, "d") {
		daysStr := strings.TrimSuffix(j.Runtime, "d")
		if numberOfDays, err := strconv.ParseInt(daysStr, 0, 32); err == nil {
			j.Runtime = fmt.Sprintf("%dh", numberOfDays * 24)
		} else {
			return err
		}
	}
	if dur, err := time.ParseDuration(j.Runtime); err == nil {
		j.runtime = dur
	} else {
		return err
	}

	if j.Rate != 0 {
		j.rateDelay = time.Duration(1000000/j.Rate) * time.Microsecond
	}

	if j.Block_Pattern == "" {
		j.Block_Pattern = PatternLCG
	}
	switch j.Block_Pattern {
	case PatternRand, PatternZero, PatternIncr, PatternLCG:
	default:
		return fmt.Errorf("[section %s]/Invalid pattern %s", section,
			j.Block_Pattern)
	}

	if j.Record_Time == "" {
		j.Record_Time = "5s"
	}
	if dur, err := time.ParseDuration(j.Record_Time); err != nil {
		return fmt.Errorf("[section %s]/Invalid record-time value\n", section)
	} else {
		j.recordTime = dur
	}

	if j.Record_File == "" {
		j.Record_File = "/dev/null"
	}

	if j.Delay_Start == "" {
		j.Delay_Start = "0s"
	}
	if dur, err := time.ParseDuration(j.Delay_Start); err != nil {
		//noinspection GoPlaceholderCount
		return fmt.Errorf("[section %s]/Invalid delay-start value\n", section)
	} else {
		j.delayStart = dur
	}
	if j.Slave_Host == "" {
		j.Slave_Host = "127.0.0.1"
	}

	if j.Reset_Buf == 0 {
		j.Reset_Buf = 1
	}
	return nil
}

func (c *Configs) validateGlobal() error {
	if c.Global.Version != 1 {
		return fmt.Errorf("invalid configuration file 'version'. valid config version is 1")
	}
	if c.Global.Linear != "" {
		vals := strings.Split(c.Global.Linear, ",")
		if len(vals) != 3 {
			return fmt.Errorf("invalid Linear configuration (min, max, increment)")
		}
		for idx, val := range vals {
			c.Global.linearParams[idx], _ = time.ParseDuration(strings.TrimSpace(val))
		}
		c.Global.doLinear = true
	} else {
		c.Global.doLinear = false
	}
	if c.Global.Intermediate_Stats == "" {
		c.Global.intermediateStats = 0
	} else {
		if dur, err := time.ParseDuration(c.Global.Intermediate_Stats); err != nil {
			return fmt.Errorf("invalid intermediate-stats value: %s", c.Global.Intermediate_Stats)
		} else {
			c.Global.intermediateStats = dur
		}
	}
	if c.Global.Job_Order == "" {
		var barrierList []string = nil
		for name := range c.Job {
			barrierList = append(barrierList, name)
			c.Global.jobOrder = append(c.Global.jobOrder, name)
		}
		c.Global.barrierOrder = append(c.Global.barrierOrder, barrierList)
	} else {
		var barrierList []string = nil
		for _, name := range strings.FieldsFunc(c.Global.Job_Order, FindComma) {
			name = strings.TrimSpace(name)
			if _, ok := c.Job[name]; ok {
				barrierList = append(barrierList, name)
			} else if name == "barrier" {
				c.Global.barrierOrder = append(c.Global.barrierOrder, barrierList)
				barrierList = nil
			} else {
				return fmt.Errorf("unknown job [%s]", name)
			}
			c.Global.jobOrder = append(c.Global.jobOrder, name)
		}
		c.Global.barrierOrder = append(c.Global.barrierOrder, barrierList)
	}

	return c.Global.validate("global")
}

func (c *Configs) UpdateJobs() error {
	// There's got to be better way to scan through the elements of JobData.
	// Considering that gcfg does something like that need to look.
	for jobName, jd := range c.Job {
		if jd.Directory == "" {
			jd.Directory = c.Global.Directory
		}
		if jd.Name == "" {
			jd.Name = c.Global.Name + jobName
		}
		if jd.IODepth == 0 {
			jd.IODepth = c.Global.IODepth
		}
		if jd.Size == "" {
			jd.Size = c.Global.Size
		}
		if jd.Runtime == "" {
			jd.Runtime = c.Global.Runtime
		}
		if jd.Rate == 0 {
			jd.Rate = c.Global.Rate
		}
		if jd.Record_Time == "" {
			jd.Record_Time = c.Global.Record_Time
		}
		if jd.Record_File == "" {
			jd.Record_File = c.Global.Record_File
		}
		if jd.Delay_Start == "" {
			jd.Delay_Start = c.Global.Delay_Start
		}
		if jd.Job_Order == "" {
			jd.Job_Order = c.Global.Job_Order
		}
		if jd.Slave_Host == "" {
			jd.Slave_Host = c.Global.Slave_Host
		}
		if jd.Access_Pattern == "" {
			jd.Access_Pattern = c.Global.Access_Pattern
		}
		if jd.Reset_Buf == 0 {
			jd.Reset_Buf = c.Global.Reset_Buf
		}
		// -- Don't copy Verbose from the global settings to each job. Verbose
		// is specific to each section.
		// -- Same with fsync.
		if err := jd.validate(jobName); err != nil {
			return err
		}
	}
	return nil
}
