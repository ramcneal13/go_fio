package main

import (
	"flag"
	"os"
	"fmt"
	"strings"
)

type bitMaskBitDump struct {
	byteOffset	byte
	rightShift	uint8
	mask		byte
	name		string
}

type multiByteDump struct {
	byteOffset	int
	numberBytes	int
	name		string
}

var inputDevice string
var pageRequest int
var inquiryEVPD bool
var debugOutput bool
var commandName string
var scsiCommands map[string]func(*os.File)
var protocolIndentifier map[byte]string

func init() {
	const (
		defaultDevice= "/dev/null"
		usage= "device to use"
		defaultPage= 0
		usagePage= "page to return"
		usageEVPD= "Used to request page 0"
		usageDebug= "Dump out raw data"
		usageCommand= "Direct selection of command to run"
	)
	flag.StringVar(&inputDevice, "device", defaultDevice, usage)
	flag.StringVar(&inputDevice, "d", defaultDevice, usage+" (shorthand)")
	flag.IntVar(&pageRequest, "page", defaultPage, usagePage)
	flag.IntVar(&pageRequest, "p", defaultPage, usagePage+" (shorthand)")
	flag.BoolVar(&inquiryEVPD, "evpd", false, usageEVPD)
	flag.BoolVar(&inquiryEVPD, "e", false, usageEVPD+" (shorthand)")
	flag.BoolVar(&debugOutput, "debug", false, usageDebug)
	flag.BoolVar(&debugOutput, "D", false, usageDebug+" (shorthand)")
	flag.StringVar(&commandName, "command", "", usageCommand)
	flag.StringVar(&commandName, "C", "", usageCommand+" (shorthand)")

	scsiCommands = map[string]func(*os.File){
		"inquiry":  scsiInquiryCommand,
		"logsense": scsiLogSenseCommand,
	}
	protocolIndentifier = map[byte]string{
		0x0: "FCP-4",
		0x1: "SPI-5",
		0x2: "SSA-S3P",
		0x3: "SBP-3",
		0x4: "SRP",
		0x5: "iSCSI",
		0x6: "SPL-3",
		0x7: "ADT-2",
		0x8: "ACS-2",
		0x9: "UAS-2",
		0xa: "SOP",
	}
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
	asciiDump(buf, n)
	fmt.Printf("\n")
}

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a int, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}


type dataToInt struct {
	buf		[]byte
	offset	int
	count	int
}

func (d *dataToInt) setBuf(b []byte) {
	d.buf = b
}

func (d *dataToInt) getInt() int {
	return int(d.getInt64())
}

func (d *dataToInt) setOffsetCount(offset int, count int) {
	d.offset = offset
	d.count = count
}

func (d *dataToInt) getInt64() int64 {
	val := int64(0)
	for i := 0; i < d.count; i++ {
		val = val << 8 | int64(d.buf[i + d.offset])
	}
	return val
}
