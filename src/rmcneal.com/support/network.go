package support

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	IDLE = iota
	PREP
	RUN
	CLEANUP
	FINISHED
)

const (
	deviceStr     = "device"
	blkSizeStr    = "blk-size"
	rwStr         = "rw"
	roStr         = "ro"
	woStr         = "wo"
	accessStr     = "access"
	accessRandStr = "rand"
	accessSeqStr  = "seq"
	readPerStr    = "read-per"
	sizeStr       = "size"
	runTimeStr    = "run-time"
	VERSION       = 1
)

type NetState struct {
	listen   net.Listener
	run      bool
	exitChan chan int
}

type Task struct {
	name       string
	devOrFile  string
	fd         *os.File
	blkSizes   map[int32]int32
	readPer    int32
	fileSize   int64
	removeFile bool
	params     map[string]string
	state      int
	stateChans chan int
	iopsRd     int64
	iopsWr     int64
	xferRd     int64
	xferWr     int64
}

type CommandEngine struct {
	conn      net.Conn
	incoming  *bufio.Reader
	taskLists []*Task
	curTask   *Task
}

func NetInit() (*NetState, error) {
	var err error
	n := &NetState{}
	n.run = true
	n.exitChan = make(chan int, 1)
	if n.listen, err = net.Listen("tcp", ":4646"); err != nil {
		fmt.Printf("Unable to listen on port 4646")
		return nil, err
	} else {
		go n.handler()
	}
	return n, nil
}

func (n *NetState) Finish() {
	<-n.exitChan
}

func (n *NetState) handler() {
	for n.run {
		if conn, err := n.listen.Accept(); err != nil {
			fmt.Printf("Listen error: %s\n", err)
		} else {
			ce := &CommandEngine{conn: conn, incoming: bufio.NewReader(conn)}
			go ce.control()
		}
	}
	n.exitChan <- 1
}

func (ce *CommandEngine) control() {
	fmt.Printf("Accepted connection\n")
	for {
		if line, err := ce.incoming.ReadString('\n'); err != nil {
			fmt.Printf("ReadString error=%s\n", err)
			cmdStop(ce, strings.Split("stop", " "))
		} else {
			args := strings.Split(strings.TrimSpace(line), " ")
			ce.runCmd(args)
		}
	}
}

var cmdTable = map[string]func(*CommandEngine, []string){
	"task":    cmdTask,
	"set":     cmdSet,
	"show":    cmdShow,
	"start":   cmdStart,
	"stop":    cmdStop,
	"exit":    cmdExit,
	"stats":   cmdStats,
	"version": cmdVersion,
	"clear":   cmdClear,
}

func (ce *CommandEngine) runCmd(args []string) {
	if cmd, ok := cmdTable[args[0]]; ok {
		cmd(ce, args)
	} else {
		fmt.Fprintf(ce.conn, "Invalid command: %s\n", args[0])
		for k := range cmdTable {
			fmt.Fprintf(ce.conn, "    %s\n", k)
		}
	}
}

//noinspection GoUnusedParameter
func cmdVersion(ce *CommandEngine, args []string) {
	fmt.Fprintf(ce.conn, "%d\n", VERSION)
}

//noinspection GoUnusedParameter
func cmdExit(ce *CommandEngine, args []string) {
	cmdStop(ce, strings.Split("stop", " "))
	os.Exit(0)
}

func cmdTask(ce *CommandEngine, args []string) {
	if len(args) < 2 {
		fmt.Fprint(ce.conn, "Usage: task name\n")
		return
	}
	t := &Task{name: args[1], params: map[string]string{
		deviceStr:  "/dev/null",
		blkSizeStr: "60:8k,40:4k",
		rwStr:      rwStr,
		sizeStr:    "5g",
		readPerStr: "50",
		accessStr:  accessRandStr,
		runTimeStr: "0",
	}, state: IDLE, stateChans: make(chan int), removeFile: false}
	ce.taskLists = append(ce.taskLists, t)
	ce.curTask = t
}

func cmdSet(ce *CommandEngine, args []string) {
	if len(args) < 3 {
		fmt.Fprint(ce.conn, "Usage: set name value\n")
		return
	}
	ce.curTask.params[args[1]] = strings.Join(args[2:], " ")
}

//noinspection GoUnusedParameter
func cmdShow(ce *CommandEngine, args []string) {
	for _, tVal := range ce.taskLists {
		t := tVal
		fmt.Fprintf(ce.conn, "Task: %s\n", t.name)
		for k, v := range t.params {
			fmt.Fprintf(ce.conn, "    %s: %s\n", k, v)
		}
		fmt.Fprintf(ce.conn, "    state: %s\n", stateToStr(t.state))
	}
}

//noinspection GoUnusedParameter
func cmdStart(ce *CommandEngine, args []string) {
	for _, tVal := range ce.taskLists {
		go tVal.runner(ce)
	}
}

func cmdStopOne(t *Task) {
	// Only in the run state will the task be looking at the channel
	if t.getState() == RUN {
		t.stateChans <- CLEANUP
		for {
			if t.getState() == FINISHED {
				break
			}
			time.Sleep(time.Second)
		}
	}
	// Reset here instead of just after the FINISHED state is detected because
	// the thread may have already finished. The stop command can then be used
	// to clean up the state.
	t.resetState()
}

func cmdStop(ce *CommandEngine, args []string) {
	for _, tVal := range ce.taskLists {
		if len(args) == 1 || strings.Compare(args[1], tVal.name) == 0 {
			fmt.Fprintf(ce.conn, "Stopping [%s] ... ", tVal.name)
			cmdStopOne(tVal)
			fmt.Fprint(ce.conn, "done\n")
		}
	}
}

func cmdClearOne(t *Task) {
	t.resetState()
}

func cmdClear(ce *CommandEngine, args []string) {
	for _, tVal := range ce.taskLists {
		if len(args) == 1 || strings.Compare(args[1], tVal.name) == 0 {
			cmdClearOne(tVal)
		}
	}
}

func cmdStats(ce *CommandEngine, args []string) {
	for _, tVal := range ce.taskLists {
		if len(args) == 1 || strings.Compare(args[1], tVal.name) == 0 {
			fmt.Fprintf(ce.conn, "%s: iops_rw: %d, iopsWr: %d, xferRd: %d, xferWr: %d\n", tVal.name,
				tVal.iopsRd, tVal.iopsWr, tVal.xferRd, tVal.xferWr)
		}
	}
}
func (t *Task) getState() int {
	return t.state
}

func (t *Task) resetState() {
	t.state = IDLE
	t.iopsRd = 0
	t.iopsWr = 0
	t.xferWr = 0
	t.xferRd = 0
}

func (t *Task) runner(ce *CommandEngine) {
	ticker := time.Tick(time.Second)

	if t.state != IDLE {
		fmt.Fprintf(ce.conn, "Task [%s] already running\n", t.name)
		return
	}
	t.state = PREP
	if t.prepVars(ce) == false {
		return
	}

	// Can't create and fill in file until all of the parameters have been parsed.
	// There's no guarantee that when the "device" parameter is parsed the file size
	// parameter will have been parsed. It all depends on which order the parameters
	// are found in the dictionary.
	if t.prepFile(ce) == false {
		return
	}
	t.state = RUN
	for t.state == RUN {
		select {
		case state := <-t.stateChans:
			t.state = state
		case <-ticker:
			break
		default:
			var buf []byte
			n := rand.Int31n(100)
			for randRange, blkSize := range t.blkSizes {
				if n < randRange {
					buf = make([]byte, blkSize)
					break
				} else {
					n -= randRange
				}
			}
			if strings.Compare(t.params[accessStr], accessSeqStr) == 0 {
				if rand.Int31n(100) < t.readPer {
					t.fd.Read(buf)
					t.iopsRd++
					t.xferRd += int64(len(buf))
				} else {
					t.fd.Write(buf)
					t.iopsWr++
					t.xferWr += int64(len(buf))
				}
			} else {
				blkNum := rand.Int63n(t.fileSize) >> 9 << 9
				if rand.Int31n(100) < t.readPer {
					t.fd.ReadAt(buf, blkNum)
					t.iopsRd++
					t.xferRd += int64(len(buf))
				} else {
					t.fd.WriteAt(buf, blkNum)
					t.iopsWr++
					t.xferWr += int64(len(buf))
				}
			}
		}
	}
	t.state = CLEANUP
	t.fd.Close()
	if t.removeFile {
		if err := os.Remove(t.params[deviceStr]); err != nil {
			fmt.Fprintf(ce.conn, "Failed to remove %s, error=%s\n", t.params[deviceStr], err)
		}
	}
	t.state = FINISHED
}

var taskPrepTables = map[string]func(*Task, *CommandEngine, string) bool{
	blkSizeStr: taskPrepBlkSize,
	rwStr:      taskPrepRw,
	readPerStr: taskPrepReadPer,
	deviceStr:  taskPrepDevice,
	sizeStr:    taskPrepSize,
	accessStr:  taskPrepAccess,
	runTimeStr: taskPrepRuntime,
}

func taskPrepBlkSize(t *Task, ce *CommandEngine, s string) bool {
	valStrs := strings.Split(s, ",")
	t.blkSizes = map[int32]int32{}
	for _, pair := range valStrs {
		pairSplit := strings.Split(pair, ":")
		if len(pairSplit) != 2 {
			fmt.Fprintf(ce.conn, "Invalid blk/val pair %s in %s\n", pairSplit, s)
			return false
		}
		randVal, err := strconv.Atoi(pairSplit[0])
		if err != nil || randVal < 0 || randVal > 100 {
			fmt.Fprintf(ce.conn, "Invalid random value %s\n", pairSplit[0])
			return false
		}
		if blkSize, err1 := strToSize(pairSplit[1]); err1 {
			t.blkSizes[int32(randVal)] = blkSize
		}
	}
	return true
}

//noinspection GoUnusedParameter
func taskPrepRw(t *Task, ce *CommandEngine, s string) bool {
	switch s {
	case rwStr, roStr, woStr:
		break
	default:
		fmt.Fprintf(ce.conn, "Invalid '%s' value of %s\n", rwStr, s)
		return false
	}
	return true
}

func taskPrepReadPer(t *Task, ce *CommandEngine, s string) bool {
	var err error
	var per int64
	per, err = strconv.ParseInt(s, 0, 32)
	if err != nil || per < 0 || per > 100 {
		fmt.Fprintf(ce.conn, "Invalid read percentage; %s\n", s)
		return false
	} else {
		t.readPer = int32(per)
	}
	return true
}

func taskPrepDevice(t *Task, ce *CommandEngine, s string) bool {
	var err error

	if _, err = os.Stat(s); err != nil {
		t.removeFile = true
	}

	t.fd, err = os.OpenFile(s, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Fprintf(ce.conn, "Failed to open %s: error=%s\n", s, err)
		return false
	}
	return true
}

func taskPrepSize(t *Task, ce *CommandEngine, s string) bool {
	var err error
	if idx := strings.IndexAny(s, "kmgtKMGT"); idx != -1 {
		t.fileSize, err = strconv.ParseInt(s[:idx], 0, 64)
		if err != nil {
			fmt.Fprintf(ce.conn, "Invalid size value: %s\n", s)
			return false
		}
		switch s[idx : idx+1] {
		case "k", "K":
			t.fileSize *= 1024
		case "m", "M":
			t.fileSize *= 1024 * 1024
		case "g", "G":
			t.fileSize *= 1024 * 1024 * 1024
		case "t", "T":
			t.fileSize *= 1024 * 1024 * 1024 * 1024
		}
	} else {
		t.fileSize, err = strconv.ParseInt(s, 0, 64)
	}
	return true
}

//noinspection GoUnusedParameter
func taskPrepAccess(t *Task, ce *CommandEngine, s string) bool {
	switch s {
	case accessRandStr, accessSeqStr:
		break
	default:
		fmt.Fprintf(ce.conn, "Invalid access pattern: %s\n", s)
		return false
	}
	return true
}

//noinspection GoUnusedParameter,GoUnusedParameter,GoUnusedParameter
func taskPrepRuntime(t *Task, ce *CommandEngine, s string) bool {
	return true
}

func (t *Task) prepVars(ce *CommandEngine) bool {
	for name, f := range taskPrepTables {
		if s, ok := t.params[name]; ok {
			if f(t, ce, s) == false {
				return false
			}
		} else {
			fmt.Fprintf(ce.conn, "No %s parameter\n", name)
			return false
		}
	}

	return true
}

func (t *Task) prepFile(ce *CommandEngine) bool {
	if fi, err := t.fd.Stat(); err != nil {
		fmt.Fprintf(ce.conn, "Can't stat file: %s\n", t.params[deviceStr])
		return false
	} else if fi.Mode().IsRegular() && strings.Compare(t.params[rwStr], roStr) == 0 &&
		fi.Size() < t.fileSize {
		// File is regular, read-only I/O request, and the associated file isn't
		// large enough
		var buf = make([]byte, 1024*1024)
		for n := int64(0); n < t.fileSize; n += 1024 * 1024 {
			t.fd.Write(buf)
		}
		t.fd.Seek(0, 0)
		t.removeFile = true
	}

	return true
}

func strToSize(s string) (blkSize int32, ok bool) {
	var err error
	var blk int64

	ok = true

	if idx := strings.IndexAny(s, "kmKM"); idx != -1 {
		blk, err = strconv.ParseInt(s[:idx], 0, 32)
		if err != nil {
			return 0, false
		}
		switch s[idx : idx+1] {
		case "k", "K":
			blkSize = int32(blk) * 1024
		case "m", "M":
			blkSize = int32(blk) * 1024 * 1024
		}
	} else {
		blk, err = strconv.ParseInt(s, 0, 32)
		if err != nil {
			return 0, false
		}
		blkSize = int32(blk)
	}
	return
}

func stateToStr(s int) string {
	switch s {
	case IDLE:
		return "IDLE"
	case RUN:
		return "RUN"
	case CLEANUP:
		return "CLEANUP"
	case FINISHED:
		return "FINISHED"
	}
	return "Unknown"
}
