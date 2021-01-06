package main

import (
	"flag"
	"os"
	"fmt"
	"strings"
)

//noinspection GoSnakeCaseUsage,GoSnakeCaseUsage
const (
	INQUIRY            = 0x12
	SECURITY_PROTO_IN  = 0xa2
	SECURITY_PROTO_OUT = 0xb5
)

var inputDevice string
var pageRequest int
var inquiryEVPD bool
var debugOutput bool
var commandName string
var scsiCommands map[string]func(*os.File)
var showAll bool
var sedOption string

func init() {
	const (
		usage= "device to use"
		defaultPage= 0
		usagePage= "page to return"
		usageEVPD= "Used to request page 0"
		usageDebug= "Dump out raw data"
		usageCommand= "Direct selection of command to run"
		usageSED = "Subcommand for SED devices"
	)
	flag.StringVar(&inputDevice, "device", "", usage)
	flag.StringVar(&inputDevice, "d", "", usage+" (shorthand)")
	flag.IntVar(&pageRequest, "page", defaultPage, usagePage)
	flag.IntVar(&pageRequest, "p", defaultPage, usagePage+" (shorthand)")
	flag.BoolVar(&inquiryEVPD, "evpd", false, usageEVPD)
	flag.BoolVar(&inquiryEVPD, "e", false, usageEVPD+" (shorthand)")
	flag.BoolVar(&debugOutput, "debug", false, usageDebug)
	flag.BoolVar(&debugOutput, "D", false, usageDebug+" (shorthand)")
	flag.StringVar(&commandName, "command", "", usageCommand)
	flag.StringVar(&commandName, "C", "", usageCommand+" (shorthand)")
	flag.BoolVar(&showAll, "all", false, "show all pages")
	flag.StringVar(&sedOption, "sed", "", usageSED)
	flag.StringVar(&sedOption, "s", "", usageSED+ "(shorthand)")

	scsiCommands = map[string]func(*os.File) {
		"inquiry":  scsiInquiryCommand,
		"logsense": scsiLogSenseCommand,
		"readcap": scsiReadCapCommand,
		"diskinfo": diskInfo,
		"tcg": sedCommand,
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

	if showAll && pageRequest != 0 {
		fmt.Printf("%s: using --all and setting a page number are not compatible\n", progName)
		os.Exit(1)
	}

	if inputDevice == "" {
		inputDevice = flag.Arg(0)
	}

	fp, err = os.Open(inputDevice)
	if err != nil {
		/*
		 * See if the user just gave us the last component part of the device name.
		 */
		 if fp, err = os.Open("/dev/rdsk/"+inputDevice); err == nil {
		 	inputDevice = "/dev/rdsk/"+inputDevice
		 } else if fp, err = os.Open("/dev/rdsk/" + inputDevice + "p0"); err == nil {
			 inputDevice = "/dev/rdsk/" + inputDevice + "p0"
		 } else {
			 fmt.Printf("%s\n", err)
			 os.Exit(1)
		 }
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
