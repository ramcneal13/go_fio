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

	startTime	time.Time
	endTime		time.Time
	statChan	chan StatRecord
	ackChan		chan int
}

type StatRecord struct {
	op			int
	readTime	time.Duration
	writeTime	time.Duration
	byteCount	int64
}

const (
	START_STATS = 1
	STOP_STATS = 2
	RECORD_STATS = 3
	DISPLAY_STATS = 4
	CLEAR_STATS = 5
)

func StartStats() *StatData {
	s := &StatData{}
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

func (s *StatData) worker() {
	var rec StatRecord
	tick := time.Tick(time.Second)
	tickSeconds := 0

	for {
		select {
		case <- tick:
			elapsed := time.Now().Sub(s.startTime)
			if elapsed.Seconds() == 0 {
				break
			}
			tickSeconds++
			fmt.Printf("[%s] IOPS: %s, BW: %s\r", SecsToHMSstr(tickSeconds),
				Humanize(s.totalOps/int64(elapsed.Seconds()), 1),
				Humanize(s.readBytes/int64(elapsed.Seconds()), 1))
		case rec = <-s.statChan:
			switch rec.op {
			case START_STATS:
				s.startTime = time.Now()
				s.ackChan <- 1
			case STOP_STATS:
				fmt.Println()
				s.endTime = time.Now()
				s.ackChan <- 1
			case DISPLAY_STATS:
				elapsed := s.endTime.Sub(s.startTime)
				fmt.Printf("Total Time: %s\n", elapsed)
				fmt.Printf("Total Bytes: %s\n", Humanize(s.readBytes, 1))
				fmt.Printf("IOPS: %s\n", Humanize(s.totalOps/int64(elapsed.Seconds()), 1))
				fmt.Printf("Throughput: %s\n", Humanize(s.readBytes/int64(elapsed.Seconds()), 1))
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
				s.readBytes += rec.byteCount
				s.readTime += rec.readTime
				s.writeTime += rec.writeTime
				s.totalOps++
			}
		}
	}
}