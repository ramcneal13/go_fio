package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"time"
)

type WorkerConfig struct {
	SourceName	string
	TargetName	string
	Options		string

	// Values converted from Options string
	sizeToUse	int64
	threads		int
	blkSize		int

	// State of worker
	srcFile		*os.File
	tgtFile		*os.File
	acChan		chan AccessControl
}

func (w *WorkerConfig) parseOptions() bool {
	w.sizeToUse = 0
	w.blkSize = 64 * 1024
	w.threads = 32

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
		default:
			fmt.Printf("Unknown key: %s\n", kvPair[0])
			return false
		}
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
	if fp, err = os.OpenFile(w.SourceName, os.O_RDONLY, 0666); err != nil {
		fmt.Printf("Source error: %s\n", err)
		return false
	} else if trueSize, err = fp.Seek(0, 2); err != nil {
		fmt.Printf("Failed to get size of source: %s\n", err)
		return false
	} else {
		if w.sizeToUse == 0 || w.sizeToUse >= trueSize {
			w.sizeToUse = trueSize
		}
	}
	w.srcFile = nil
	w.tgtFile = nil

	return true
}

func (w *WorkerConfig) Start(stats *StatData) {
	var err error

	defer func() {
		if w.srcFile != nil {
			w.srcFile.Close()
		}
		if w.tgtFile != nil {
			w.tgtFile.Close()
		}
	}()

	fmt.Printf("WorkerConfig Start called\n")
	fmt.Printf("    Threads: %d\n    Block Size: %s\n    Copy Size: %s\n    From: %s\n    To: %s\n",
		w.threads, Humanize(int64(w.blkSize), 1), Humanize(w.sizeToUse, 1),
		w.SourceName, w.TargetName)
	if w.srcFile, err = os.OpenFile(w.SourceName, os.O_RDONLY, 0666); err != nil {
		fmt.Printf("Failed to open: %s, err=%s\n", w.SourceName, err)
		return
	}

	if w.tgtFile, err = os.OpenFile(w.TargetName, os.O_RDWR | os.O_CREATE, 0666); err != nil {
		fmt.Printf("Failed to open for writing: %s, err=%s\n", w.TargetName, err)
		return
	}

	w.acChan = make(chan AccessControl, 1000)
	completeChan := make(chan int, 10)
	go w.blockControl()
	for i := 0; i < w.threads; i++ {
		go w.readWriteWorker(i, stats, completeChan)
	}
	for i := 0; i < w.threads; i++ {
		<- completeChan
	}
}

type AccessControl struct {
	seekPos	int64
	stopAccess	bool
}

func (w *WorkerConfig) blockControl() {
	var curPos int64 = 0
	var ac AccessControl

	for curPos < w.sizeToUse {
		ac.seekPos = curPos
		ac.stopAccess = false
		w.acChan <- ac
		curPos += int64(w.blkSize)
	}
	for i := 0; i < w.threads; i++ {
		ac.stopAccess = true
		w.acChan <- ac
	}
}

func (w *WorkerConfig) readWriteWorker(thrId int, stats *StatData, completeChan chan int) {
	var ac AccessControl
	var readElapsed time.Duration
	var writeElapsed time.Duration
	var startTime time.Time
	var endTime time.Time
	buf := make([]byte, w.blkSize)

	defer func() {
		completeChan <- thrId
	}()
	for {
		ac = <- w.acChan
		if ac.stopAccess {
			return
		}

		startTime = time.Now()
		if cnt, err := w.srcFile.ReadAt(buf, ac.seekPos); err != nil {
			fmt.Printf("Read(0x%x) failed, expected %d, got %d; err=%s\n", ac.seekPos, w.blkSize, cnt, err)
			return
		}
		endTime = time.Now()
		readElapsed = endTime.Sub(startTime)
		startTime = time.Now()
		if cnt, err := w.tgtFile.WriteAt(buf, ac.seekPos); err != nil {
			fmt.Printf("Write(0x%x) failed, expected %d, got %d; err=%s\n", ac.seekPos, w.blkSize, cnt, err)
			return
		}
		endTime = time.Now()
		writeElapsed = endTime.Sub(startTime)
		stats.Record(readElapsed, writeElapsed, int64(len(buf)))
	}
}