package main

import (
	"flag"
	"os"
	"fmt"
)

var source string
var target string
var optionStr string
var configFile string

func init() {
	flag.StringVar(&source, "source", "", "Source to read")
	flag.StringVar(&source, "s", "", "Source to read (shorthand)")
	flag.StringVar(&target, "target", "", "target to write")
	flag.StringVar(&target, "t", "", "target to write (shorthand")
	flag.StringVar(&optionStr, "options", "threads=32,blocksize=64k", "option string")
	flag.StringVar(&optionStr, "o", "threads=32,blocksize=64k", "option string (shorthand)")
	flag.StringVar(&configFile, "config", "", "configuration file")
	flag.StringVar(&configFile, "c", "", "configuration file (shorthand)")
}

func main() {
	var listOfWorkers []*WorkerConfig = nil
	flag.Parse()
	if configFile != "" {
		fmt.Println("Need to add code to parse configuration file")
		os.Exit(1)
	} else if source != "" && target != "" {
		w := &WorkerConfig{SourceName: source, TargetName: target, Options: optionStr}
		listOfWorkers = append(listOfWorkers, w)
	} else {
		flag.Usage()
		os.Exit(1)
	}

	for _, w := range listOfWorkers {
		if w.Validate() == false {
			os.Exit(1)
		}
	}

	stats := StartStats()
	for _, w := range listOfWorkers {
		stats.Start()
		w.Start(stats)
		stats.Stop()
		stats.Display()
		stats.Clear()
	}
}