package main

import (
	"time"
	"fmt"
)

type StatData struct {
	totalOps	int64
	readBytes	int64	// No need to keep write bytes since they'll be the same as read
	readTime	time.Duration
	writeTime	time.Duration

	startTime time.Time
	endTime   time.Time
	elapsed   time.Duration
	statChan  chan StatRecord
	ackChan   chan int
}

type StatRecord struct {
	op			int
	readTime	time.Duration
	writeTime	time.Duration
	byteCount	int64
}

//noinspection ALL,GoSnakeCaseUsage
const (
	START_STATS   = 1
	STOP_STATS    = 2
	RECORD_STATS  = 3
	DISPLAY_STATS = 4
	CLEAR_STATS   = 5
	SHOW_CURRENT  = 6
)

func StartStats() *StatData {
	s := &StatData{}
	s.totalOps = 0
	s.readBytes = 0
	s.statChan = make(chan StatRecord, 1000)
	s.ackChan = make(chan int, 1)
	go s.worker()
	return s
}

func (s *StatData) Start() {
	s.statChan <- StatRecord{START_STATS, 0, 0, 0 }
	<- s.ackChan
}

func (s *StatData) Stop() {
	s.statChan <- StatRecord{STOP_STATS, 0, 0, 0 }
	<- s.ackChan
}

func (s *StatData) Display() {
	s.statChan <- StatRecord{DISPLAY_STATS, 0, 0, 0}
	<- s.ackChan
}

func (s *StatData) Clear() {
	s.statChan <- StatRecord{CLEAR_STATS, 0, 0, 0}
	<- s.ackChan
}

func (s *StatData) Record(readTime time.Duration, writeTime time.Duration, count int64) {
	s.statChan <- StatRecord{RECORD_STATS, readTime, writeTime, count}
}

func (s *StatData) Current() {
	s.statChan <- StatRecord{SHOW_CURRENT, 0, 0, 0}
}

func (s *StatData) worker() {
	tickSeconds := 0
	statsRunning := false
	var lastRead time.Duration = 0
	var lastWrite time.Duration = 0
	var lastOps int64 = 0

	for rec := range s.statChan {

		switch rec.op {
		case START_STATS:
			statsRunning = true
			s.startTime = time.Now()
			s.ackChan <- 1
		case STOP_STATS:
			if statsRunning {
				s.endTime = time.Now()
				s.elapsed = s.endTime.Sub(s.startTime)
				statsRunning = false
			}
			s.ackChan <- 1
		case DISPLAY_STATS:
			fmt.Printf("\nTotal Time: %s\n", s.elapsed)
			fmt.Printf("Total Bytes: %s\n", Humanize(s.readBytes, 1))
			fmt.Printf("IOPS: %s\n", Humanize(s.totalOps/int64(s.elapsed.Seconds()), 1))
			fmt.Printf("Throughput: %s\n", Humanize(s.readBytes/int64(s.elapsed.Seconds()), 1))
			fmt.Printf("Avg. Read Latency: %s\n", time.Duration(int64(s.readTime)/s.totalOps))
			fmt.Printf("Avg. Write Latency: %s\n", time.Duration(int64(s.writeTime)/s.totalOps))
			s.ackChan <- 1
		case CLEAR_STATS:
			s.totalOps = 0
			s.readBytes = 0
			s.readTime = 0
			s.writeTime = 0
			s.ackChan <- 1
		case RECORD_STATS:
			if statsRunning {
				s.readBytes += rec.byteCount
				s.readTime += rec.readTime
				s.writeTime += rec.writeTime
				s.totalOps++
			}
		case SHOW_CURRENT:
			if statsRunning {
				elapsed := int64(time.Since(s.startTime).Seconds())
				if elapsed != 0 {
					tickSeconds++
					fmt.Printf("[%s] xfer:%s IOPS:%s, r_lat:%s w_lat:%s\r",
						SecsToHMSstr(tickSeconds),
						Humanize(s.readBytes, 1),
						Humanize((s.totalOps-lastOps)/elapsed, 1),
						(s.readTime-lastRead)/time.Duration(elapsed),
						(s.writeTime-lastWrite)/time.Duration(elapsed))
					lastOps = s.totalOps
					lastRead = s.readTime
					lastWrite = s.writeTime
				}
			}
		}
	}
}