package main

import "rmcneal.com/support"

import (
	"fmt"
	"bytes"
)

var protocolIdentifier = map[byte]string{
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
		fmt.Printf("%02x", buf[byteIndex])
		if (byteIndex % 4) == 3 {
			fmt.Printf(" ")
		}
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

func isLineZeros(buf []byte, n int) bool {
	for byteIndex := 0; byteIndex < n; byteIndex += 1 {
		if buf[byteIndex] != 0 {
			return false
		}
	}
	return true
}

func dumpMemory(buf []byte, n int, prefix string) {
	ow := 8
	lastLineZero := false
	printContinue := true
	if n < 0x100 {
		ow = 2
	} else if n < 0x10000 {
		ow = 4
	}
	for offset := 0; offset < n; offset += 16 {
		if isLineZeros(buf[offset:], min(16, n-offset)) {
			// Even if the last couple of lines in the buffer are zero print out
			// the last line which shows the offset and contents .
			if offset+16 >= n {
				lastLineZero = false
			}

			if lastLineZero {
				if printContinue {
					fmt.Printf("%s        ....\n", prefix)
					printContinue = false
				}
				continue
			} else {
				lastLineZero = true
			}
		} else {
			lastLineZero = false
			printContinue = true
		}
		fmt.Printf("%s", prefix)
		dumpLine(buf[offset:], min(16, n-offset), int64(offset), ow)
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

func (d *dataToInt) setOffsetCount(offset int, count int) {
	d.offset = offset
	d.count = count
}

func (d *dataToInt) getInt() int {
	return int(d.getInt64())
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

type comPacket struct {
	header []byte
	payload []byte
	subpacket []byte
	totalLen	uint32
	description	string
}

func createPacket(g *tcgData, name string) *comPacket {
	pd := &comPacket{}
	pd.description = name
	pd.header = make([]byte, 20)
	pd.payload = make([]byte, 24)

	pd.putShortInHeader(g.comID, 4)

	// 24 is the fixed size of the payload in the comPacket
	pd.totalLen = uint32(24)
	pd.putIntInPayload(g.spSessionID, 0) // TSN
	pd.putIntInPayload(1, 4)             // HSN
	g.sequenceNum++
	pd.putIntInPayload(g.sequenceNum, 8) // Sequence number

	return pd
}

func (p *comPacket) fini() {
	// The subpacket length must not contain the pad bytes.
	p.putIntInSub((uint32)(len(p.subpacket)) - 12, 8)

	if len(p.subpacket) % 4 != 0 {
		bytesToAdd := 4 - (len(p.subpacket) % 4)
		for ; bytesToAdd > 0; bytesToAdd-- {
			p.addByteToSub(0)
		}
	}
	subLen := (uint32)(len(p.subpacket))

	p.totalLen += subLen
	p.putIntInPayload(subLen, 20)

	p.putIntInHeader(p.totalLen, 16)
}

func (p *comPacket) getFullPayload() []byte {
	full := make([]byte, 0, 512)
	full = Append(full, p.header)
	full = Append(full, p.payload)
	full = Append(full, p.subpacket)
	padding := make([]byte, 512 - len(full))
	full = Append(full, padding)

	fmt.Printf("  Header len: %d, payload len: %d, sub pkt len: %d, total: %d\n",
		len(p.header), len(p.payload), len(p.subpacket), len(full))
	dumpMemory(full, len(full), "  ")

	return full
}

func (p *comPacket) putIntInHeader(val uint32, offset int) {
	intAtData(p.header, val, offset)
}

func (p *comPacket) putShortInHeader(val uint16, offset int) {
	shortAtData(p.header, val, offset)
}

func (p *comPacket) putIntInPayload(val uint32, offset int) {
	intAtData(p.payload, val, offset)
}

func (p *comPacket) putShortInPayload(val uint16, offset int) {
	shortAtData(p.payload, val, offset)
}

func (p *comPacket) putIntInSub(val uint32, offset int) {
	intAtData(p.subpacket, val, offset)
}

func (p *comPacket) putShortInSub(val uint16, offset int) {
	shortAtData(p.subpacket, val, offset)
}

func (p *comPacket) addByteToSub(val byte) {
	p.subpacket = append(p.subpacket, val)
}

func (p *comPacket) addShortToSub(val uint16) {
	p.subpacket = append(p.subpacket, (byte)((val >> 8) & 0xff))
	p.subpacket = append(p.subpacket, (byte)(val & 0xff))
}

func (p *comPacket) addIntToSub(val uint32) {
	p.subpacket = append(p.subpacket, (byte)((val >> 24) & 0xff))
	p.subpacket = append(p.subpacket, (byte)((val >> 16) & 0xff))
	p.subpacket = append(p.subpacket, (byte)((val >> 8) & 0xff))
	p.subpacket = append(p.subpacket, (byte)(val & 0xff))
}

//noinspection GoUnusedFunction
func longAtData(data []byte, val uint64, offset int) {
	data[offset] = (byte)((val >> 56) & 0xff)
	data[offset+1] = (byte)((val >> 48) & 0xff)
	data[offset+2] = (byte)((val >> 40) & 0xff)
	data[offset+3] = (byte)((val >> 32) & 0xff)
	data[offset+4] = (byte)((val >> 24) & 0xff)
	data[offset+5] = (byte)((val >> 16) & 0xff)
	data[offset+6] = (byte)((val >> 8) & 0xff)
	data[offset+7] = (byte)(val & 0xff)
}

func intAtData(data []byte, val uint32, offset int) {
	data[offset] = (byte)((val >> 24) & 0xff)
	data[offset+1] = (byte)((val >> 16) & 0xff)
	data[offset+2] = (byte)((val >> 8) & 0xff)
	data[offset+3] = (byte)(val & 0xff)
}

func shortAtData(data []byte, val uint16, offset int) {
	data[offset] = (byte)((val >> 8) & 0xff)
	data[offset+1] = (byte)(val & 0xff)
}
