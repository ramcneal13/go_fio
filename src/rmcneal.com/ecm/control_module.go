package main

import (
	"flag"
	"fmt"
	"os"
	"rmcneal.com/support"
	"time"
)

var configFile string

func init() {
	const (
		usage       = "File containing job information"
		defaultFile = "ecm.ini"
	)
	flag.StringVar(&configFile, "control_file", defaultFile, usage)
	flag.StringVar(&configFile, "c", defaultFile, usage)
}

func main() {
	var cfg *support.Configs
	var err error

	flag.Parse()

	printer := support.PrintInit()
	defer func() {
		printer.Exit()
		os.Exit(0)
	}()

	if cfg, err = support.ReadConfig(configFile); err != nil {
		printer.Send("Failed to parse configuration file: %s\n", err)
		return
	}

	slavePool := map[string]*support.SlaveController{}

	statCom := make(chan support.WorkerStat, 10)
	go statOutput(statCom)
	for _, barrierGroups := range *cfg.GetBarrierOrder() {
		track := support.TrackingInit(printer)

		track.SetTitle("Preparing")
		for _, name := range barrierGroups {
			jd := cfg.Job[name]
			slave := &support.SlaveController{JobConfig: jd, Name: name, Printer: printer, StatChan: statCom}
			slavePool[name] = slave
			track.RunFunc(name, func() bool {
				if err := slave.InitDial(); err == nil {
					return true
				} else {
					printer.Send("[%s] %s\n", slave.Name, err)
					slavePool[name] = nil
					return false
				}
			})
		}
		track.WaitForThreads()

		track.SetTitle("Starting")
		for _, name := range barrierGroups {
			slave := slavePool[name]
			if slave == nil {
				continue
			}
			track.RunFunc(name, func() bool {
				if err := slave.ClientStart(); err == nil {
					return true
				} else {
					slavePool[name] = nil
					return false
				}
			})
		}
		track.WaitForThreads()

		track.SetTitle("Waiting")
		for _, name := range barrierGroups {
			slave := slavePool[name]
			if slave == nil {
				/* ---- An error occurred during the start and job removed ---- */
				continue
			}
			track.RunFunc(name, func() bool {
				if err := slave.ClientWait(); err == nil {
					return true
				} else {
					fmt.Printf("%s", err)
					slavePool[name] = nil
					return false
				}
			})
		}
		track.WaitForThreads()
		aggregateStats(printer, slavePool)
	}
}

func aggregateStats(p *support.Printer, pool map[string]*support.SlaveController) {
	var readIOPS, writeIOPS int64 = 0, 0

	var colName, colRead, colWrite, colTime, colLat = len("Name"), len("Read"), len("Write"),
		len("Time"), len("Low/Avg/High")
	distro := support.DistroInit(p, "Latency Histogram")
	/*
	 * Calculate field sizes.
	 */
	for name, sc := range pool {
		if sc == nil {
			continue
		}
		p = sc.Printer
		if len(name) > colName {
			colName = len(name)
		}
		o := fmt.Sprintf("%.2f", float64(sc.Stats.Reads)/float64(sc.Stats.Elapsed/time.Second))
		if len(o) > colRead {
			colRead = len(o)
		}
		o = fmt.Sprintf("%.2f", float64(sc.Stats.Writes)/float64(sc.Stats.Elapsed/time.Second))
		if len(o) > colWrite {
			colWrite = len(o)
		}
		o = fmt.Sprintf("%s", sc.Stats.Elapsed)
		if len(o) > colTime {
			colTime = len(o)
		}
		o = fmt.Sprintf("%s,%s,%s", sc.Stats.LowResponse, sc.Stats.AvgResponse, sc.Stats.HighResponse)
		if len(o) > colLat {
			colLat = len(o)
		}
		distro.Add(sc.Stats.Histogram)
	}
	if p == nil {
		return
	}
	distro.Graph()
	p.Send("\n%*s +-%*sIOPS%*s-+%*s+-%*s-+\n", colName, "", colRead-1, "", colWrite, "",
		colTime+2, "", colLat, "Latency  ")
	p.Send("%*s | %*s | %*s | %*s | %*s |\n", colName, "Name", colRead, "Read", colWrite, "Write", colTime,
		"Time", colLat, "Low/Avg/High")
	p.Send("%s\n", support.DashLine(colName, colRead+2, colWrite+2, colTime+2, colLat+2))
	for name, sc := range pool {
		if sc != nil {
			latencyStr := fmt.Sprintf("%s/%s/%s", sc.Stats.LowResponse, sc.Stats.AvgResponse,
				sc.Stats.HighResponse)
			p.Send("%*s | %*.2f | %*.2f | %*s | %*s\n", colName, name,
				colRead, float64(sc.Stats.Reads)/float64(sc.Stats.Elapsed/time.Second),
				colWrite, float64(sc.Stats.Writes)/float64(sc.Stats.Elapsed/time.Second),
				colTime, sc.Stats.Elapsed, colLat, latencyStr)
			readIOPS += sc.Stats.Reads / int64(sc.Stats.Elapsed/time.Second)
			writeIOPS += sc.Stats.Writes / int64(sc.Stats.Elapsed/time.Second)
		}
	}
	p.Send("Aggregate Numbers\nRead IOPS: %s, Write IOPS: %s\n", support.Humanize(readIOPS, 1),
		support.Humanize(writeIOPS, 1))
}

func statOutput(c chan support.WorkerStat) {
	for {
		select {
		case ws := <- c:
			fmt.Printf("%s: Reads=%d, Writes=%d\n", ws.Elapsed, ws.Reads, ws.Writes)
		}
	}
}
