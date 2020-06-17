package main

import (
	"fmt"
	"os"
	"strings"
	"rmcneal.com/support"
	"flag"
)

type yesNo bool

type diskInfoData struct {
	name         string
	isSSD        yesNo
	capacity     int64
	pathToDevice string
	vendor       string
	productID    string
	serialNumber string
	fp           *os.File
	problem      error
}

type processName struct {
	id	int
	response	chan diskInfoData
	request		chan string
}

var noPostProcessing bool

func init() {
	flag.BoolVar(&noPostProcessing, "raw",false,"Don't post process output")
}

func (b yesNo) String() string {
	if b {
		return "Yes"
	} else {
		return "no"
	}
}

func diskInfo(fp *os.File) {
	var err error
	var diskList []string

	if fp, err = os.Open("/dev/rdsk"); err == nil {
		var nameList []string
		if nameList, err = fp.Readdirnames(0); err == nil {
			for _, singleName := range nameList {
				if strings.HasSuffix(singleName, "p0") {
					// fmt.Printf("%s\n", singleName)
					diskList = append(diskList, singleName)
				}
			}
		}
	}
	if noPostProcessing {
		for _, name := range diskList {
			d := gatherData(name)
			if d.problem == nil {
				fmt.Printf("%s %s %s %s | %s\n", d.name, d.vendor, d.productID,
					support.Humanize(d.capacity, 1), d.isSSD)
			}
		}
	} else {
		runPostProcessing(diskList)
	}
}

func runPostProcessing(diskList []string) {
	var processedList []diskInfoData

	threadsToRun := 10
	workerList := make([]*processName, threadsToRun)
	response := make(chan diskInfoData, 10)

	for i := 0; i < threadsToRun; i++ {
		p := &processName{i,response,make(chan string)}
		workerList[i] = p
		go p.Run()
	}

	go func() {
		i := 0
		for _, name := range diskList {
			if len(name) == 0 {
				fmt.Printf("Found invalid name\n")
				continue
			}
			workerList[i % threadsToRun].request <- name
			i++
		}
	}()

	for i := 0; i < len(diskList); i++ {
		r := <-response
		if r.problem == nil {
			processedList = append(processedList, r)
		}
	}

	maxDeviceName := 0
	for _, r := range processedList {
		maxDeviceName = max(maxDeviceName, len(r.name))
	}
	fmt.Printf("  %-*s   %-*s   %-*s   %-*s   SSD\n", maxDeviceName, "Device Name",
		8, "Vendor", 16, "Product ID", 6, "Size")
	fmt.Printf("%s\n", support.DashLine(maxDeviceName+2, 8+2, 16+2, 6+2, 3+2))
	for _, r := range processedList {
		fmt.Printf("| %-*s | %s | %s | %6s | %s\n", maxDeviceName, r.name, r.vendor, r.productID,
			support.Humanize(r.capacity, 1), r.isSSD)
	}
}

func (p *processName) Run() {
	for {
		select {
		case name := <-p.request:
			p.response <- gatherData(name)
		}
	}
}

func gatherData(name string) diskInfoData {
	reply := diskInfoData{name, false, 0, "/dev/rdsk/" + name,
		"", "", "",nil, nil}

	if fp, err := os.Open("/dev/rdsk/" + name); err == nil {
		reply.fp = fp
		diskinfoInquiry(&reply)
		diskinfoLogSense(&reply)
		diskinfoReadCap(&reply)
		fp.Close()
	} else {
		reply.problem = err
	}
	return reply
}
