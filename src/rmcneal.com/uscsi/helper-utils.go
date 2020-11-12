package main

import "rmcneal.com/support"

import (
	"fmt"
	"bytes"
)

var protocolIndentifier = map[byte]string{
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

func hexDump(buf []byte, n int, offset int64, offsetWidth int) {
	fmt.Printf("%0*x: ", offsetWidth, offset)
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		fmt.Printf("%02x ", buf[byteIndex])
	}
}

func asciiDump(buf []byte, n int) {
	fmt.Printf("%*s  ", (16-n)*3, "")
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
		dumpLine(buf[offset:], min(16, int(int64(n)-offset)), offset, ow)
	}
}

func dumpLine(buf []byte, n int, offset int64, offsetWidth int) {
	hexDump(buf, n, offset, offsetWidth)
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
	buf    []byte
	offset int
	count  int
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
		val = val<<8 | int64(d.buf[i+d.offset])
	}
	return val
}

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

func htons(hd uint16) uint16 {
	return (hd >> 8) | ((hd & 0xff) << 8)
}

func ntohs(nd uint16) uint16 {
	return htons(nd)
}

func Append(slice, data []byte) []byte {
	l := len(slice)
	if l + len(data) > cap(slice) {  // reallocate
		// Allocate double what's needed, for future growth.
		newSlice := make([]byte, (l+len(data))*2)
		// The copy function is predeclared and works for any slice type.
		copy(newSlice, slice)
		slice = newSlice
	}
	slice = slice[0:l+len(data)]
	copy(slice[l:], data)
	return slice
}

func doBitDump(table []bitMaskBitDump, data []byte) {
	outputCols := 4
	fmt.Printf("    ")
	for _, pb := range table {
		str := fmt.Sprintf("%s=%d ", pb.name, data[pb.byteOffset]>>pb.rightShift&pb.mask)
		if outputCols+len(str) >= 80 {
			fmt.Printf("\n    ")
			outputCols = 4
		}
		outputCols += len(str)
		fmt.Printf("%s", str)
	}
	fmt.Printf("\n")
}

func doMultiByteDump(table []multiByteDump, data []byte) {
	longestStr := 0
	for _, mb := range table {
		longestStr = max(longestStr, len(mb.name))
	}
	fmt.Printf("  %s\n", support.DashLine(longestStr+2, 80-longestStr-7))
	for _, mb := range table {
		converter := dataToInt{data, mb.byteOffset, mb.numberBytes}
		val := converter.getInt64()
		fmt.Printf("  | %-*s | 0x%x", longestStr, mb.name, val)
		if val > 10000 {
			fmt.Printf(" (%s)", support.Humanize(val, 1))
		}
		fmt.Printf("\n")
	}
	fmt.Printf("  %s\n", support.DashLine(longestStr+2, 80-longestStr-7))
}

func dumpASCII(data []byte, offset int, count int) string {
	return bytes.NewBuffer(data[offset:offset + count]).String()
}
