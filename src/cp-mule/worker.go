package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type WorkerConfig struct {
	SourceName string
	TargetName string
	Options    string

	// Values converted from Options string
	sizeToUse     int64
	threads       int
	blkSize       int
	inputOffset   int64
	outputOffset  int64
	alternateSize int
	openFlags     int

	// State of worker
	srcFile      *os.File
	ignoreSrc    bool
	tgtFile      *os.File
	ignoreTgt    bool
	acChan       chan AccessControl
	thrComplete  chan int
	workerFinish chan int
	keepRunning  bool
	stats        *StatData
}

func (w *WorkerConfig) parseOptions() bool {
	w.sizeToUse = 0
	w.blkSize = 64 * 1024
	w.threads = 32
	w.inputOffset = 0
	w.outputOffset = 0
	w.alternateSize = 0

	opts := strings.Split(w.Options, ",")
	for _, kvStr := range opts {
		kvPair := strings.Split(kvStr, "=")
		if len(kvPair) != 2 {
			fmt.Printf("Invalid key/value pair: %s\n", kvStr)
			return false
		}
		switch kvPair[0] {
		case "threads":
			if c, err := strconv.ParseInt(kvPair[1], 0, 32); err != nil {
				fmt.Printf("Invalid thread count: %s\n", kvPair[1])
				return false
			} else {
				w.threads = int(c)
			}
		case "size":
			if c, err := blkStringToInt64(kvPair[1]); err == false {
				fmt.Printf("Invalid size value: %s\n", kvPair[1])
				return false
			} else {
				w.sizeToUse = c
			}
		case "blksize":
			if c, err := blkStringToInt64(kvPair[1]); err == false {
				fmt.Printf("Invalid block size: %s\n", kvPair[1])
				return false
			} else {
				w.blkSize = int(c)
			}
		case "offset":
			if c, err := blkStringToInt64(kvPair[1]); err == false {
				fmt.Printf("Invalid offset: %s\n", kvPair[1])
				return false
			} else {
				// Right now the code only supports setting the input and output
				// position to the same value. It would take much if desired to
				// have something like "i_off" and "o_off" to represent the
				// two different values.
				w.inputOffset = c
				w.outputOffset = c
			}
		case "alternate":
			if c, err := blkStringToInt64(kvPair[1]); err == false {
				fmt.Printf("Invalid alternate size: %s\n", kvPair[1])
			} else {
				w.alternateSize = int(c)
			}
		case "flags":
			flagPairs := strings.Split(kvPair[1], ":")
			for _, flag := range flagPairs {
				switch flag {
				case "sync":
					w.openFlags |= os.O_SYNC
				}
			}
		default:
			fmt.Printf("Unknown key: %s\n", kvPair[0])
			return false
		}
	}
	if w.alternateSize > w.blkSize {
		fmt.Printf("alternate(%d) is larger than blksize(%d)\n",
			w.alternateSize, w.blkSize)
	}
	return true
}

func (w *WorkerConfig) Validate() bool {
	var err error = nil
	var fp *os.File = nil
	var trueSize int64

	defer func() {
		// Close the file descriptor since it's possible we'll validate multiple worker
		// configurations before actually starting a single worker.
		if fp != nil {
			fp.Close()
		}
	}()

	if w.parseOptions() == false {
		return false
	}
	if w.SourceName != "" {
		if fp, err = os.OpenFile(w.SourceName, os.O_RDONLY, 0666); err != nil {
			fmt.Printf("Source error: %s\n", err)
			return false
		} else if trueSize, err = fp.Seek(0, 2); err != nil {
			fmt.Printf("Failed to get size of source: %s\n", err)
			return false
		} else if trueSize != 0 {
			if w.sizeToUse == 0 || w.sizeToUse >= trueSize {
				w.sizeToUse = trueSize
			}
		}
		w.ignoreSrc = false
	} else {
		w.ignoreSrc = true
	}
	if w.TargetName == "" {
		w.ignoreTgt = true
	} else {
		w.ignoreTgt = false
	}
	w.srcFile = nil
	w.tgtFile = nil

	return true
}

func (w *WorkerConfig) Start(stats *StatData, exitChan chan int) {
	var err error

	w.workerFinish = exitChan
	w.keepRunning = true
	fmt.Printf("WorkerConfig Start called\n")
	fmt.Printf("    Threads: %d\n    Block Size: %s\n    Copy Size: %s\n    From: %s\n    To: %s\n",
		w.threads, Humanize(int64(w.blkSize), 1), Humanize(w.sizeToUse, 1),
		w.SourceName, w.TargetName)
	if w.alternateSize != 0 {
		fmt.Printf("    Alternate Block: %s\n", Humanize(int64(w.alternateSize), 1))
	}
	if w.openFlags != 0 {
		fmt.Printf("    Open flags: %s\n", flagsToStr(w.openFlags))
	}

	if w.ignoreSrc == false {
		if w.srcFile, err = os.OpenFile(w.SourceName, os.O_RDONLY|w.openFlags, 0666); err != nil {
			fmt.Printf("Failed to open: %s, err=%s\n", w.SourceName, err)
			return
		}
	}

	if w.ignoreTgt == false {
		if w.tgtFile, err = os.OpenFile(w.TargetName, os.O_RDWR|os.O_CREATE|w.openFlags, 0666); err != nil {
			fmt.Printf("Failed to open for writing: %s, err=%s\n", w.TargetName, err)
			return
		}
	}

	w.acChan = make(chan AccessControl, 10000)
	go w.blockControl()

	w.thrComplete = make(chan int, 10)
	w.stats = stats
	for i := 0; i < w.threads; i++ {
		go w.readWriteWorker(i, stats)
	}
}

func flagsToStr(flags int) string {
	flagStr := map[int]string{
		os.O_SYNC:  "O_SYNC",
		os.O_TRUNC: "O_TRUNC",
	}
	rtnVal := ""
	for k, v := range flagStr {
		if (flags & k) != 0 {
			rtnVal += " " + v
		}
	}
	return rtnVal
}

func (w *WorkerConfig) Stop() {
	w.keepRunning = false
}

type AccessControl struct {
	inputSeekPos  int64
	outputSeekPos int64
	blkSize       int
	stopAccess    bool
}

func (w *WorkerConfig) blockControl() {
	var inputCurPos = w.inputOffset
	var outputCurPos = w.outputOffset
	var ac AccessControl
	flip := true

	for inputCurPos < w.sizeToUse && w.keepRunning {
		ac.inputSeekPos = inputCurPos
		ac.outputSeekPos = outputCurPos
		ac.stopAccess = false
		ac.blkSize = w.blkSize
		if w.alternateSize != 0 {
			if flip {
				flip = false
				ac.blkSize = w.alternateSize
			} else {
				flip = true
			}
		}
		w.acChan <- ac
		inputCurPos += int64(ac.blkSize)
		outputCurPos += int64(ac.blkSize)
	}

	// Send the stop signal to the threads
	for i := 0; i < w.threads; i++ {
		ac.stopAccess = true
		w.acChan <- ac
	}

	// Wait for them to finish
	blockedIO := 0
	for i := 0; i < w.threads; i++ {
		blockedIO += <-w.thrComplete
	}
	w.stats.Stop()
	if blockedIO != 0 {
		fmt.Printf("\nBlocked I/O count: %d\n", blockedIO)
	}

	if w.srcFile != nil {
		w.srcFile.Close()
	}
	if w.tgtFile != nil {
		w.tgtFile.Close()
	}
	// Let the main thread know we've tidied everything up.
	w.workerFinish <- 1
}

func (w *WorkerConfig) readWriteWorker(thrId int, stats *StatData) {
	var readElapsed time.Duration
	var startTime time.Time
	var endTime time.Time
	buf := make([]byte, w.blkSize)
	blockedIO := 0

	defer func() {
		w.thrComplete <- blockedIO
	}()

	//
	// These booleans, countAfterFirst, doSelect, and count countOnce, are all part of an
	// experiment to see if the code would block while trying to get the next block to perform
	// an I/O with. Originally acChan was created with a buffer of 1,000 and the blockIO value
	// was in the millions. Clearly the code was spinning quite a bit while waiting for something.
	// countOnce was added to reduce the counts that were from just spinning. That lowered the
	// value return substantially, but it was still non-zero. It also became obvious that the thread
	// which sends data to the channel probably isn't schedule to run first or gets the change to run
	// first a prime the channel. So, countAfterFirst was added to further reduce the noise. At that
	// point the code reported that around 50 times one or more of the threads would attempt to read
	// from the channel and be blocked. So, the channel buffer was increased from 1,000 to 10,000 and
	// the problem no longer happens. Leaving this code here as a reminder of why and how this was
	// determined.
	//
	// countAfterFirst := false
	// for {
	//	 doSelect := true
	//	 countOnce := true
	//	 for doSelect {
	//		select {
	//		case ac = <-w.acChan:
	//			if ac.stopAccess {
	//				return
	//			}
	//			countAfterFirst = true
	//			doSelect = false
	//		default:
	//			if countAfterFirst && countOnce {
	//				countOnce = false
	//				blockedIO++
	//			}
	//		}
	//	}

	for ac := range w.acChan {
		if ac.stopAccess {
			return
		}

		if w.ignoreSrc == false {
			startTime = time.Now()
			if cnt, err := w.srcFile.ReadAt(buf[0:ac.blkSize], ac.inputSeekPos); err != nil {
				fmt.Printf("Read(0x%x) failed, expected %d, got %d; err=%s\n", ac.inputSeekPos, w.blkSize, cnt, err)
				return
			}
			endTime = time.Now()
			readElapsed = endTime.Sub(startTime)
		} else {
			readElapsed = 0
		}

		if w.ignoreTgt == false {
			startTime = time.Now()
			if cnt, err := w.tgtFile.WriteAt(buf[0:ac.blkSize], ac.outputSeekPos); err != nil {
				fmt.Printf("Write(0x%x) failed, expected %d, got %d; err=%s\n", ac.inputSeekPos, w.blkSize, cnt, err)
				return
			}
			endTime = time.Now()
		} else {
			startTime = time.Now()
			endTime = startTime
		}
		blockedIO += stats.Record(readElapsed, endTime.Sub(startTime), int64(ac.blkSize))
	}
}
