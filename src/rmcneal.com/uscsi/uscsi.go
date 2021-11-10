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

//noinspection ALL,GoUnusedConst
const (
	UscsiWrite    = 0
	UscsiSilent   = 1
	UscsiDiagnose = 2
	UscsiIsolate  = 4
	UscsiRead     = 8
	UscsiReset    = 0x4000
	UscsiRQEnable = 0x10000
)

var inputDevice string
var pageRequest int
var inquiryEVPD bool
var debugOutput int
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
	flag.IntVar(&debugOutput, "debug", 0, usageDebug)
	flag.IntVar(&debugOutput, "D", 0, usageDebug+" (shorthand)")
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
		if inputDevice == "" {
			flag.Usage()
			os.Exit(1)
		}
	}

	if fp, err = osSpecificOpen(inputDevice); err != nil {
		fmt.Printf("Failed to open '%s', err = %s\n", inputDevice, err)
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
	os.Exit(0)
}
