package main

import (
	"flag"
	"os"
	"fmt"
	"strings"
)

var inputDevice string
var inquiryPage int
var inquiryEVPD bool
var debugOutput bool
var commandName string
var scsiCommands map[string]func(*os.File)

func init() {
	const (
		defaultDevice = "/dev/null"
		usage = "device to use"
		defaultPage = 0
		usagePage = "INQUIRY page to return"
		usageEVPD = "Used to request page 0"
		usageDebug = "Dump out raw data"
		usageCommand = "Direct selection of command to run"
	)
	flag.StringVar(&inputDevice, "device", defaultDevice, usage)
	flag.StringVar(&inputDevice, "d", defaultDevice, usage+" (shorthand)")
	flag.IntVar(&inquiryPage, "page", defaultPage, usagePage)
	flag.IntVar(&inquiryPage, "p", defaultPage, usagePage+" (shorthand)")
	flag.BoolVar(&inquiryEVPD, "evpd", false, usageEVPD)
	flag.BoolVar(&inquiryEVPD, "e", false, usageEVPD+" (shorthand)")
	flag.BoolVar(&debugOutput, "debug", false, usageDebug)
	flag.BoolVar(&debugOutput, "D", false, usageDebug+" (shorthand)")
	flag.StringVar(&commandName, "command", "", usageCommand)
	flag.StringVar(&commandName, "C", "", usageCommand+" (shorthand)")

	scsiCommands = map[string]func(*os.File){}
	scsiCommands["inquiry"] = scsiInquiryCommand
}

func main() {
	var fp *os.File
	var err error

	lastIndex := strings.LastIndex(os.Args[0], "/")
	if lastIndex == -1 {
		lastIndex = 0
	} else {
		lastIndex = lastIndex + 1
	}

	progName := os.Args[0][lastIndex:]

	flag.Parse()
	fp, err = os.Open(inputDevice)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	defer fp.Close()

	if commandName != "" {
		progName = commandName
	}

	if cmdPointer, ok := scsiCommands[progName]; ok {
		cmdPointer(fp)
	} else {
		fmt.Printf("No sub function '%s' found\n", progName)
		fmt.Printf("Here's a list of available commands which can be used with -C\n")
		for key := range scsiCommands {
			fmt.Printf("%s\n", key)
		}
	}
}

func hexDump(buf []byte, n int, offset int64, offset_width int) {
	fmt.Printf("%0*x: ", offset_width, offset)
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		fmt.Printf("%02x ", buf[byteIndex])
	}
}

func asciiDump(buf []byte, n int) {
	fmt.Printf("%*s  ", (16 - n) * 3, "")
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		if buf[byteIndex] >= ' ' && buf[byteIndex] <= '~' {
			fmt.Printf("%c", buf[byteIndex])
		} else {
			fmt.Printf(".")
		}
	}
}

func dumpMemory(buf []byte, n int, prefix string) {
	ow := 8
	if n < 0x100 {
		ow = 2
	} else if n < 0x10000 {
		ow = 4
	}
	for offset := int64(0); offset < int64(n); offset += 16 {
		fmt.Printf("%s", prefix)
		dumpLine(buf[offset:], min(16, int(int64(n) - offset)), offset, ow)
	}
}

func dumpLine(buf []byte, n int, offset int64, offset_width int) {
	hexDump(buf, n, offset, offset_width)
	asciiDump(buf, n, offset)
	fmt.Printf("\n")
}

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}