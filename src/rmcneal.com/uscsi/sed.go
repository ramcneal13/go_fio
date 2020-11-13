package main

import (
	"os"
	"fmt"
)

type tcgData struct {
	opalDevice bool
	rubyDevice bool
	lockingEnabled	bool
	lockingSupported bool
	comID      uint16
}

func sedCommand(fp *os.File) {

	var cdb []byte
	var data []byte

	cdb = make([]byte, 12)
	data = make([]byte, 512)

	// Default to the device being an Opal device. It may not provide
	// a feature code page 0x203 during Level 0 Discovery
	tcgGlobal := &tcgData{true,false,false,false,0}

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
		dumpLevelZeroDiscovery(data, dataLen, tcgGlobal)
	}

	if tcgGlobal.opalDevice {
		cdb[0] = 0xa2
		// Section 3.3.4.3.1 Storage Architecture Core Spec v2.01_r1.00
		cdb[1] = 2             // Protocol ID 2 == GET_COMID
		shortAtData(cdb, 0, 2) // COMID must be equal zero

		data = make([]byte, 512)
		data[5] = 0
		data[19] = 255

		if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("USCSI Protocol 2 failed, err=%s\n", err)
			return
		} else {
			converter := dataToInt{data,0,2}
			tcgGlobal.comID = uint16(converter.getInt())
		}
	} else if tcgGlobal.rubyDevice {
		if tcgGlobal.comID == 0 {
			fmt.Printf("Failed to get Ruby ComID\n")
			return
		}
	} else {
		fmt.Printf("Device type uknown\n")
		return
	}

	if tcgGlobal.lockingSupported {
		if tcgGlobal.lockingEnabled {
			fmt.Printf("Our work is done here. Locking supported and enabled")
			return
		}
	} else {
		fmt.Printf("Locking is not supported\n")
		return
	}

	fmt.Printf("ComID: 0x%x\n", tcgGlobal.comID)
	openSession(fp, tcgGlobal.comID)
}

func openSession(fp *os.File, comID uint16) {

	packet := createPacket()
	packet.putShortInHeader(comID, 4)

	// 24 is the fixed size of the payload in the comPacket
	totalLen := uint32(24)
	packet.putIntInPayload(1, 0) // TSN
	packet.putIntInPayload(1, 4) // HSN
	packet.putIntInPayload(1, 8) // Sequence number

	packet.addIntToSub(0)   // Reserved
	packet.addShortToSub(0) // Reserved
	packet.addShortToSub(0) // SubPacket Kind: 0 == data
	packet.addIntToSub(0)   // Length for now,

	// Call Token
	packet.subpacket = append(packet.subpacket, 0xf8)

	// InvokingID
	packet.addIntToSub(0)
	packet.addIntToSub(0xff)

	// MethodID
	packet.addIntToSub(0)
	packet.addIntToSub(0xff02)

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
	packet.putIntInSub(subLen - 12, 8)

	totalLen += subLen
	packet.putIntInPayload(subLen, 20)

	packet.putIntInHeader(totalLen, 16)
	fmt.Printf("Header len: %d, payload len: %d, sub packet len: %d\n", len(packet.header),
		len(packet.payload), len(packet.subpacket))

	full := make([]byte, 0, 64)
	full = Append(full, packet.header)
	full = Append(full, packet.payload)
	full = Append(full, packet.subpacket)
	dumpMemory(full, len(full), "")

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(fp, cdb, full, 0); err != nil {
		fmt.Printf("Open session failed: err=%s\n", err)
	} else {
		fmt.Printf("SECURITY_PROTOCOL_OUT: okay\n")
		cdb[0] = 0xa2
		if dataLen, err := sendUSCSI(fp, cdb, full, 0); err != nil {
			fmt.Printf("SECURITY_PROTOCOL_IN: err=%s\n", err)
		} else {
			fmt.Printf("Reply data len: %d\n", dataLen)
			dumpMemory(full, dataLen, "  ")
		}
	}
}

func dumpLevelZeroDiscovery(data []byte, len int, g *tcgData) {
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
		offset += dumpDescriptor(data[offset:], paramLen - offset, g)
	}
}

type featureFuncName struct {
	name	string
	dump	func([]byte, int, *tcgData)
}

var codeStr = map[int]featureFuncName {
	1: {"TPer", dumpTPerFeature},
	2: {"Locking", dumpLockingFeature},
	3: {"Geometry Reporting", dumpGeometryFeature},
	0x202: {"Additional DataStore", dumpAdditionalDataStore},
	0x203: {"Opal v2.01_rev1.00 SSC", dumpOpalV2Feature},
	0x304: {"Ruby SSC", dumpRubyFeature},
}

func dumpDescriptor(data []byte, len int, g *tcgData) int {
	template := dataToInt{data, 0, 2}
	code := template.getInt()

	template.offset = 3
	template.count = 1
	featureLen := template.getInt()

	if _, ok := codeStr[code]; ok {
		fmt.Printf("  %s, len: %d\n", codeStr[code].name, featureLen)
		codeStr[code].dump(data, featureLen, g)
	} else {
		fmt.Printf("  Unknown code: %d (0x%x)\n", code, code)
	}
	return featureLen + 4
}

var dataStoreMultiByte = []multiByteDump {
	{0,2,"Feature Code"},
	{3,1,"Length"},
	{6,2,"Max # of DataStorage tables"},
	{8,4,"Max total size of DataStore tables"},
	{12,4,"Datastore table size alignment"},
}

func dumpAdditionalDataStore(data []byte, len int, g *tcgData) {
	doMultiByteDump(dataStoreMultiByte, data)
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

func dumpTPerFeature(data []byte, len int, g *tcgData) {
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

func dumpLockingFeature(data []byte, len int, g *tcgData) {
	if data[4] & 1 == 1 {
		g.lockingSupported = true
	}
	if ((data[4] >> 1) & 1) == 1 {
		g.lockingEnabled = true
	}
	doBitDump(lockingBitMap, data)
}

func dumpGeometryFeature(data []byte, len int, g *tcgData) {

}

var rubyMulitByte = []multiByteDump {
	{0,2,"Feature Code"},
	{3,1,"Length"},
	{4,2,"Base ComID"},
	{6,2,"Number of ComIDs"},
	{9,2,"# locking Admin SPs supported"},
	{11,2,"# locking User SPs supported"},
}

func dumpOpalV2Feature(data []byte, len int, g *tcgData) {
	g.opalDevice = true
	doMultiByteDump(rubyMulitByte, data)
}

func dumpRubyFeature(data []byte, len int, g *tcgData) {
	g.rubyDevice = true
	g.opalDevice = false
	converter := dataToInt{data, 4,2}
	g.comID = (uint16)(converter.getInt())
	doMultiByteDump(rubyMulitByte, data)
}