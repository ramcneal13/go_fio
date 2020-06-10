package main

import (
	"flag"
	"fmt"
	"os"
)

var inputFile string

func init() {
	const (
		defaultFile = "/dev/null"
		usage = "filename to dump"
	)
	flag.StringVar(&inputFile, "file", defaultFile, usage)
	flag.StringVar(&inputFile, "f", defaultFile, usage+" (shorthand)")
}

func main() {
	var fp *os.File
	var err error
	var bytesRead int

	defer fp.Close()
	flag.Parse()

	fp, err = os.Open(inputFile)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
	inputBuf := make([]byte, 16)
	offset := int64(0)
	bytesRead, err = fp.Read(inputBuf)
	for err == nil {
		dumpLine(inputBuf, bytesRead, offset)
		offset += 16
		bytesRead, err = fp.Read(inputBuf)
	}
}

func dumpLine(buf []byte, n int, offset int64) {
	byteIndex := 0
	fmt.Printf("%08x: ", offset)
	for byteIndex = 0; byteIndex < n; byteIndex += 1 {
		fmt.Printf("%02x ", buf[byteIndex])
	}
	fmt.Printf("%*s  ", (16 - n) * 3, "")
	for byteIndex = 0; byteIndex < n; byteIndex += 1 {
		if buf[byteIndex] >= ' ' && buf[byteIndex] <= '~' {
			fmt.Printf("%c", buf[byteIndex])
		} else {
			fmt.Printf(".")
		}
	}
	fmt.Printf("\n")
}