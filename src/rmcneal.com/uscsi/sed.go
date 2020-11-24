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
	sequenceNum	uint32
}

type TcgInterface interface {
	openSession() bool
	closeSession() bool
	setActivate() bool
	getRandom() bool
}

type tcgDevice struct {
	t TcgInterface
}

func sedCommand(fp *os.File) {

	var cdb []byte
	var data []byte
	var tcgCurrent tcgDevice

	cdb = make([]byte, 12)
	data = make([]byte, 512)

	// Default to the device being an Opal device. It may not provide
	// a feature code page 0x203 during Level 0 Discovery
	tcgGlobal := &tcgData{true,false,false,false,
	0, 0}

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

	if tcgGlobal.lockingSupported {
		if tcgGlobal.lockingEnabled {
			fmt.Printf("Our work is done here. Locking supported and enabled")
			return
		}
	} else {
		fmt.Printf("Locking is not supported\n")
		return
	}
	if tcgGlobal.opalDevice {
		opalD := opalDevice{fp, tcgGlobal}
		tcgCurrent = makeTCG(opalD)
	} else if tcgGlobal.rubyDevice {
		rubyD := rubyDevice{fp, tcgGlobal}
		tcgCurrent = makeTCG(rubyD)
	}

	fmt.Printf("ComID: 0x%x\n", tcgGlobal.comID)
	switch sedOption {
	case "":
		fmt.Printf("No command requested\n")

	case "mgmt":
		if !updateComID(fp, tcgGlobal) {
			return
		}
		if tcgCurrent.t.openSession() {
			tcgCurrent.t.setActivate()
			tcgCurrent.t.closeSession()
		}

	case "random":
		if !updateComID(fp, tcgGlobal) {
			return
		}
		if tcgCurrent.t.openSession() {
			tcgCurrent.t.getRandom()
			tcgCurrent.t.closeSession()
		}
	}
}

func makeTCG(n TcgInterface) tcgDevice {
	tcg := tcgDevice{n}
	return tcg
}

func updateComID(fp *os.File, g *tcgData) bool {
	cdb := make([]byte, 12)
	data := make([]byte, 512)

	if g.opalDevice && sedOption == "" {
		cdb[0] = 0xa2
		// Section 3.3.4.3.1 Storage Architecture Core Spec v2.01_r1.00
		cdb[1] = 2             // Protocol ID 2 == GET_COMID
		shortAtData(cdb, 0, 2) // COMID must be equal zero

		data = make([]byte, 512)
		data[5] = 0
		data[19] = 255

		if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("USCSI Protocol 2 failed, err=%s\n", err)
			return false
		} else {
			converter := dataToInt{data, 0, 2}
			g.comID = uint16(converter.getInt())
			g.comID = 0x7fe
		}
	} else if g.opalDevice {
		g.comID = 0x7fe
	} else if g.rubyDevice {
		if g.comID == 0 {
			fmt.Printf("Failed to get Ruby ComID\n")
			return false
		}
	} else {
		fmt.Printf("Device type uknown\n")
		g.comID = 0x7ffe
	}
	return true
}

type opalDevice struct {
	fp *os.File
	g *tcgData
}

func (o opalDevice) openSession() bool {
	pkt := createPacket(o.g, "Open Session")

	hardCoded := [...]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, 0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02, 0xf0,
		0x01, 0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x1,
		0x01, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	newBuf := make([]byte, 0, len(hardCoded))
	for _, v := range hardCoded {
		newBuf = append(newBuf, v)
	}
	pkt.subpacket = newBuf
	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, o.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(o.fp, cdb, full, 0); err != nil {
		fmt.Printf("Failed to open session for Opal device\n")
		return false
	} else {
		return true
	}
}

func (o opalDevice) closeSession() bool {
	pkt := createPacket(o.g, "Close Session")

	hardCoded := [...]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xfa,
	}
	newBuf := make([]byte, 0, len(hardCoded))
	for _, v := range hardCoded {
		newBuf = append(newBuf, v)
	}
	pkt.subpacket = newBuf
	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, o.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(o.fp, cdb, full, 0); err != nil {
		fmt.Printf("Failed to close session: %s\n", err)
		return false
	} else {
		return true
	}
}

func (o opalDevice) setActivate() bool {
	fmt.Printf("open session for Opal not implemented\n")
	return false
}

func (o opalDevice) getRandom() bool {
	pkt := createPacket(o.g, "Get Random")

	// Call Token
	pkt.subpacket = append(pkt.subpacket, 0xf8)

	// InvokingID
	pkt.addByteToSub(0xa8)
	pkt.addIntToSub(0)
	pkt.addIntToSub(0x1)

	// MethodID
	pkt.addByteToSub(0xa8)
	pkt.addIntToSub(0x00000006)
	pkt.addIntToSub(0x00000601)

	// Status Code List
	pkt.addByteToSub(0xf0)
	pkt.addByteToSub(0)
	pkt.addByteToSub(0xf1)

	// End of Data Token
	pkt.addByteToSub(0xf9)

	// Status Code List
	pkt.addByteToSub(0xf0)
	pkt.addByteToSub(0)
	pkt.addByteToSub(0)
	pkt.addByteToSub(0)
	pkt.addByteToSub(0xf1)

	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, o.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(o.fp, cdb, full, 0); err != nil {
		return false
	} else {
		cdb[0] = 0xa2
		if dataLen, err := sendUSCSI(o.fp, cdb, full, 0); err != nil {
			fmt.Printf("Failed to read Random results\n")
			return false
		} else {
			fmt.Printf("Random results: len=%d\n", dataLen)
			dumpMemory(full, dataLen, "  ")
			return true
		}
	}
}

type rubyDevice struct {
	fp *os.File
	g *tcgData
}

func (r rubyDevice) getRandom() bool {
	fmt.Printf("Random not implemented for Ruby devices\n")
	return false
}

func (r rubyDevice) setActivate() bool {
	pkt := createPacket(r.g, "Locking Activate")

	hardCoded := [...]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, 0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x02, 0xa8, 0x00, 0x00, 0x00, 0x06,
		0x00, 0x00, 0x02, 0x03, 0xf0, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	newBuf := make([]byte, 0, len(hardCoded))
	for _, v := range hardCoded {
		newBuf = append(newBuf, v)
	}
	pkt.subpacket = newBuf
	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, r.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(r.fp, cdb, full, 0); err != nil {
		fmt.Printf("Failed to set locking\n")
		return false
	} else {
		return true
	}
}


func (r rubyDevice) openSession() bool {

	pkt := createPacket(r.g, "Open Session")

	hardCoded := [...]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, 0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xa8, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0xff, 0x02, 0xf0, 0x01, 0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, 0x1,
		0xf2, 0x00, 0xd0, 0x12, 0x3c, 0x6e, 0x65, 0x77, 0x5f, 0x53, 0x49, 0x44, 0x5f, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f,
		0x72, 0x64, 0x3e, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x06, 0xf3,
		0xf1, 0xf9, 0xf0, 0x00,
		0x00, 0x00, 0xf1,
	}

	newBuf := make([]byte, 0, len(hardCoded))
	for _, v := range hardCoded {
		newBuf = append(newBuf, v)
	}
	pkt.subpacket = newBuf
	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, r.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(r.fp, cdb, full, 0); err != nil {
		fmt.Printf("Failed to open session for Ruby device\n")
		return false
	} else {
		return true
	}
}

func (r rubyDevice) closeSession() bool {

	pkt := createPacket(r.g, "Close Session")

	hardCoded := [...]byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xfa,
	}
	newBuf := make([]byte, 0, len(hardCoded))
	for _, v := range hardCoded {
		newBuf = append(newBuf, v)
	}
	pkt.subpacket = newBuf

	pkt.fini()

	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = 0xb5
	cdb[1] = 1
	shortAtData(cdb, r.g.comID, 2)
	intAtData(cdb, (uint32)(len(full)), 6)

	if _, err := sendUSCSI(r.fp, cdb, full, 0); err != nil {
		return false
	} else {
		return true
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