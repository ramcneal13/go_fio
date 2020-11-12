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
	cdb[1] = 1		// Protocol 1 == Discovery
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

	cdb[0] = 0xa2
	cdb[1] = 2
	cdb[2] = 0
	cdb[3] = 0
	data = make([]byte, 512)
	data[5] = 0
	data[19] = 255

	if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
		fmt.Printf("USCSI Protocol 2 failed, err=%s\n", err)
		return
	} else {
		comIDObj := dataToInt{data, 0, 2}
		comID := (uint16)(comIDObj.getInt())
		fmt.Printf("ComID: 0x%x\n", comID)
		openSession(fp, comID)
	}

}

type comPacket struct {
	header []byte
	payload []byte
	subpacket []byte
}

func createPacket() *comPacket {
	pd := &comPacket{}
	pd.header = make([]byte, 20)
	pd.payload = make([]byte, 24)
	pd.subpacket = make([]byte, 12 + 8 + 8)

	return pd
}

func (p *comPacket) intInHeader(val uint32, offset int) {
	intAtData(p.header, val, offset)
}

func (p *comPacket) shortInHeader(val uint16, offset int) {
	shortAtData(p.header, val, offset)
}

func (p *comPacket) intInPayload(val uint32, offset int) {
	intAtData(p.payload, val, offset)
}

func (p *comPacket) shortInPayload(val uint16, offset int) {
	shortAtData(p.payload, val, offset)
}

func (p *comPacket) intInSub(val uint32, offset int) {
	intAtData(p.subpacket, val, offset)
}

func (p *comPacket) shortInSub(val uint16, offset int) {
	shortAtData(p.subpacket, val, offset)
}

func (p *comPacket) shortAddSub(val uint16) {
	p.subpacket = append(p.subpacket, (byte)((val >> 8) & 0xff))
	p.subpacket = append(p.subpacket, (byte)(val & 0xff))
}

func (p *comPacket) intAddSub(val uint32) {
	p.subpacket = append(p.subpacket, (byte)((val >> 24) & 0xff))
	p.subpacket = append(p.subpacket, (byte)((val >> 16) & 0xff))
	p.subpacket = append(p.subpacket, (byte)((val >> 8) & 0xff))
	p.subpacket = append(p.subpacket, (byte)(val & 0xff))
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

func openSession(fp *os.File, comID uint16) {


	packet := createPacket()
	packet.shortInHeader(comID, 4)

	// 24 is the fixed size of the payload in the comPacket
	totalLen := uint32(24)
	packet.intInPayload(1, 0)	// TSN
	packet.intInPayload(1, 4)	// HSN
	packet.intInPayload(1, 8)	// Sequence number

	packet.intAddSub(0)	// Reserved
	packet.shortAddSub(0)	// Reserved
	packet.shortAddSub(0)	// SubPacket Kind: 0 == data
	packet.intAddSub(0)	// Length for now,

	// Call Token
	packet.subpacket = append(packet.subpacket, 0xf8)

	// InvokingID
	packet.intAddSub(0)
	packet.intAddSub(0xff)

	// MethodID
	packet.intAddSub(0)
	packet.intAddSub(0xff02)

	// End of Data Token
	packet.subpacket = append(packet.subpacket, 0xf9)

	// Status Code List
	packet.subpacket = append(packet.subpacket, 0xf0)	// Start List
	packet.subpacket = append(packet.subpacket, 0)
	packet.subpacket = append(packet.subpacket, 0xf1)	// End List

	if len(packet.subpacket) % 4 != 0 {
		bytesToAdd := 4 - (len(packet.subpacket) % 4)
		for ; bytesToAdd > 0; bytesToAdd-- {
			packet.subpacket = append(packet.subpacket, 0)
		}
	}
	subLen := (uint32)(len(packet.subpacket))
	packet.intInSub(subLen - 12, 8)

	totalLen += subLen
	packet.intInPayload(subLen, 20)

	packet.intInHeader(totalLen, 16)
	fmt.Printf("Header len: %d, payload len: %d, sub packet len: %d\n", len(packet.header),
		len(packet.payload), len(packet.subpacket))

	full := make([]byte, 0, 64)
	full = Append(full, packet.header)
	full = Append(full, packet.payload)
	full = Append(full, packet.subpacket)
	dumpMemory(full, len(full), "")

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 3
	cdb[2] = 0
	cdb[3] = 0
	intAtData(cdb, (uint32)(len(full)), 6)

	if dataLen, err := sendUSCSI(fp, cdb, full, 0); err != nil {
		fmt.Printf("Open session failed: err=%s\n", err)
	} else {
		fmt.Printf("dataLen=%d\n", dataLen)
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

type featureFuncName struct {
	name	string
	dump	func([]byte, int)
}

var codeStr = map[int]featureFuncName {
	1: {"TPer", dumpTPerFeature},
	2: {"Locking", dumpLockingFeature},
	3: {"Geometry Reporting", dumpGeometryFeature},
	0x202: {"Unknown SSC", dumpUnknownSSC},
	0x203: {"Opal v2.01_rev1.00 SSC", dumpOpalV2Feature},
	0x304: {"Ruby SSC", dumpRubyFeature},
}

func dumpDescriptor(data []byte, len int) int {
	template := dataToInt{data, 0, 2}
	code := template.getInt()

	template.offset = 3
	template.count = 1
	featureLen := template.getInt()

	if _, ok := codeStr[code]; ok {
		fmt.Printf("  %s, len: %d\n", codeStr[code].name, featureLen)
		codeStr[code].dump(data, featureLen)
	} else {
		fmt.Printf("  Unknown code: %d (0x%x)\n", code, code)
	}
	return featureLen + 4
}

func dumpUnknownSSC(data []byte, len int) {
	dumpMemory(data, len, "    ")
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

func dumpGeometryFeature(data []byte, len int) {

}

var rubyMulitByte = []multiByteDump {
	{0,2,"Feature Code"},
	{3,1,"Length"},
	{4,2,"Base ComID"},
	{6,2,"Number of ComIDs"},
	{9,2,"# locking Admin SPs supported"},
	{11,2,"# locking User SPs supported"},
}

func dumpOpalV1Feature(data []byte, len int) {
	doMultiByteDump(rubyMulitByte, data)
}

func dumpOpalV2Feature(data []byte, len int) {
	doMultiByteDump(rubyMulitByte, data)
}

func dumpRubyFeature(data []byte, len int) {
	doMultiByteDump(rubyMulitByte, data)
}