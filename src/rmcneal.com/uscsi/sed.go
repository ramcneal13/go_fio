package main

import (
	"os"
	"fmt"
)

func sedCommand(fp *os.File) {

	var cdb []byte
	var data []byte

	cdb = make([]byte, 12)
	data = make([]byte, 512)

	cdb[0] = 0xa2
	cdb[1] = 1
	cdb[2] = 0
	cdb[3] = 1
	cdb[4] = 0x80 // INC_512 bit.

	if debugOutput {
		fmt.Printf("CDB:\n")
		dumpLine(cdb, len(cdb), 0, 2)
	}

	if dataLen, err := sendUSCSI(fp, cdb, data, 0); err != nil {
		fmt.Printf("USCSI failed, err=%s\n", err)
		return
	} else {
		dumpLevelZeroDiscovery(data, dataLen)
	}
}

func dumpLevelZeroDiscovery(data []byte, len int) {
	if len < 48 {
		fmt.Printf("Invalid Discovery 0 header\n")
		return
	}

	header := dataToInt{data, 0, 4}
	paramLen := header.getInt() - 44

	header.offset = 4
	header.count = 2
	majorVers := header.getInt()

	header.offset = 6
	minorVers := header.getInt()

	fmt.Printf("Level 0 Discovery\n  Data available: %d\n  Version       : %d.%d\n", paramLen, majorVers, minorVers)
	for offset := 48; offset < paramLen ; {
		offset += dumpDescriptor(data[offset:], paramLen - offset)
	}
}

var codeStr = map[int]string {
	1: "TPer",
	2: "Locking",
	3: "Geometry Reporting",
	0x304: "Ruby SSC",
}

func dumpDescriptor(data []byte, len int) int {
	template := dataToInt{data, 0, 2}
	code := template.getInt()

	template.offset = 3
	template.count = 1
	featureLen := template.getInt()

	if codeStr[code] != "" {
		fmt.Printf("  %s, len: %d\n", codeStr[code], featureLen)
		switch code {
		case 1:
			dumpTPerFeature(data, 16)
		case 2:
			dumpLockingFeature(data, 16)
		case 0x304:
			dumpRubyFeature(data, 20)
		}
	} else {
		fmt.Printf("  Unknown code: %d (0x%x)\n", code, code)
	}
	return featureLen + 4
}

var tperBitMap = []bitMaskBitDump{
	{2, 4, 0xf, "Version"},
	{3, 0, 0xff, "Length"},
	{4, 6, 1, "ComID_Mgmt"},
	{4, 4, 1, "Streaming"},
	{4, 3,1, "Buffer_Mgmt"},
	{4, 2, 1, "ACK/NAK"},
	{4, 1, 1, "Async"},
	{4, 0, 1, "Sync"},
}

func dumpTPerFeature(data []byte, len int) {
	doBitDump(tperBitMap, data)
}

var lockingBitMap = []bitMaskBitDump {
	{2, 4, 0xf, "Version"},
	{3,0,0xff,"Length"},
	{4,5,1,"MBR_Done"},
	{4,4,1,"MBR_Enabled"},
	{4,3,1,"Media_Encryption"},
	{4,2,1,"Locked"},
	{4,1,1,"Locking_Enabled"},
	{4,0,1,"Locking_Supported"},
}

func dumpLockingFeature(data []byte, len int) {
	doBitDump(lockingBitMap, data)
}

var rubyMulitByte = []multiByteDump {
	{0,2,"Feature Code"},
	{3,1,"Length"},
	{4,2,"Base ComID"},
	{6,2,"Number of ComIDs"},
	{9,2,"# locking Admin SPs supported"},
	{11,2,"# locking User SPs supported"},
}

func dumpRubyFeature(data []byte, len int) {
	doMultiByteDump(rubyMulitByte, data)
}