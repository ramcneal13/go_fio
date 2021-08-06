package main

import (
	"fmt"
	"os"
	"strings"
	"rmcneal.com/support"
	"flag"
	"sort"
)

type yesNo bool

/*
 * If I ever get around to supporting this on OSX or Linux this structure will need to change to be more
 * generic in nature.
 */
type dkiocGetMediaInfoExt struct {
	mediaType     uint
	lbsize        uint
	capacity      uint64
	physBlockSize uint
	isSSD         uint
	rpm           uint
}

type diskInfoData struct {
	name         string
	isSSD        yesNo
	wearValue    int
	capacity     uint64
	pathToDevice string
	vendor       string
	productID    string
	serialNumber string
	fp           *os.File
	problem      error
}

type diskInfoArray struct {
	array []diskInfoData
}

// Len is part of sort.Interface.
func (a *diskInfoArray) Len() int {
	return len(a.array)
}

// Swap is part of sort.Interface.
func (a *diskInfoArray) Swap(i, j int) {
	a.array[i], a.array[j] = a.array[j], a.array[i]
}

// Less is part of sort.Interface.
func (a *diskInfoArray) Less(i, j int) bool {
	return a.array[i].name < a.array[j].name
}

type processName struct {
	id	int
	response	chan diskInfoData
	request		chan string
}

var noPostProcessing bool
var threadsToRun int

func init() {
	flag.BoolVar(&noPostProcessing, "raw",false,"Don't post process output")
	flag.IntVar(&threadsToRun, "threads", 10, "Number of threads to use")
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

	/*
	 * The noPostProcessing flag is just to see how much using threads helped
	 * or hurt. On a system with about 75 drives this diskinfo takes 1.4 seconds
	 * when using the go routines and 2 seconds for straight processing of each
	 * drive. The installed 'diskinfo' command takes a whooping 4+ seconds.
	 */
	if noPostProcessing {
		for _, name := range diskList {
			d := gatherData(name)
			if d.problem == nil {
				fmt.Printf("%s %s %s %s | %s\n", d.name, d.vendor, d.productID,
					support.Humanize(int64(d.capacity), 1), d.isSSD)
			}
		}
	} else {
		runPostProcessing(diskList)
	}
}

func runPostProcessing(diskList []string) {
	var processedList []diskInfoData

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

	sortedArray := &diskInfoArray{processedList}
	sort.Sort(sortedArray)

	fmt.Printf("  %-*s   %-*s   %-*s   %-*s   SSD (wear)\n", maxDeviceName, "Device Name",
		8, "Vendor", 16, "Product ID", 6, "Size")
	fmt.Printf("%s\n", support.DashLine(maxDeviceName+2, 8+2, 16+2, 6+2, 10+2))
	for _, r := range sortedArray.array {
		fmt.Printf("| %-*s | %s | %s | %6s | %s", maxDeviceName, r.name, r.vendor, r.productID,
			support.Humanize(int64(r.capacity), 1), r.isSSD)
		if r.isSSD {
			fmt.Printf(" (%d%%)", r.wearValue)
		}
		fmt.Printf("\n")
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

func diskMediaInfo(d *diskInfoData) {
	if cmd, err := getMediaInfo(d.fp); err == nil {
		if cmd.isSSD != 0 {
			d.isSSD = true
		} else {
			d.isSSD = false
		}
		fmt.Printf("cap: %d, lbsize=%d, physSize=%d\n", cmd.capacity, cmd.lbsize, cmd.physBlockSize)
		d.capacity = cmd.capacity * 512
	}
}

func gatherData(name string) diskInfoData {
	reply := diskInfoData{name, false, 0, 0,"/dev/rdsk/" + name,
		"", "", "",nil, nil}

	if fp, err := os.Open("/dev/rdsk/" + name); err == nil {
		reply.fp = fp
		diskinfoInquiry(&reply)
		diskinfoLogSense(&reply)
		diskinfoReadCap(&reply)
		diskMediaInfo(&reply)
		fp.Close()
	} else {
		reply.problem = err
	}
	return reply
}
