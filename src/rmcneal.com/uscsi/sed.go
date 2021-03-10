package main

import (
	"os"
	"fmt"
	"strconv"
)

//noinspection ALL,GoSnakeCaseUsage
const (
	START_LIST  = 0xf0
	END_LIST    = 0xf1
	END_OF_DATA = 0xf9
)

type tcgData struct {
	opalDevice       bool
	rubyDevice       bool
	lockingEnabled   bool
	lockingSupported bool
	comID            uint16
	sequenceNum      uint32
	spSessionID      uint32
	msid             []byte
	randomPIN        []byte
	psid             string
}

func sedCommand(fp *os.File) {

	// Default to the device being an Opal device. It may not provide
	// a feature code page 0x203 during Level 0 Discovery
	tcgGlobal := &tcgData{true,false,false,false,
		0, 0, 0x0, nil, nil, ""}

	if err := loadPSIDpairs(tcgGlobal); err != nil {
		fmt.Printf("Loading of PSID failed: %s\n", err)
		os.Exit(1)
	}

	randomPin2 := []byte{
		0x2c, 0x6b, 0x6b, 0xca, 0x26, 0x0a, 0x1d, 0xe2, 0x49, 0x01, 0x89, 0xca, 0x86, 0xd7, 0xc5, 0x41,
		0xa0, 0xde, 0x64, 0x1b, 0x1d, 0x49, 0x93, 0xf6, 0x66, 0xef, 0x00, 0xbd, 0x5d, 0xdb, 0xcb, 0x3e,
	}

	tcgGlobal.randomPIN = randomPin2

	if currentState, err := strconv.ParseInt(sedOption, 0, 32); err != nil {
		fmt.Printf("Invalid starting state number: %s, err=%s\n", sedOption, err)
		return
	} else {
		for {
			callout, ok := stateTable[currentState]
			if !ok {
				fmt.Printf("Invalid starting state: %d\n", currentState)
				return
			}

			// Methods should return true to proceed to the next state. Under normal
			// conditions the last method will return false to end the state machine.
			// Should a method encounter an error it's expected that the method will
			// display any appropriate error messages.
			fmt.Printf("[%d]---- %s ----[]\n", currentState, callout.name)
			if ok, exitCode := callout.method(fp, tcgGlobal); !ok {
				if exitCode != 0 {
					os.Exit(exitCode)
				}
				return
			}
			currentState++
		}
	}
}

type commonCallOut struct {
	method func(fp *os.File, data *tcgData) (bool, int)
	name   string
}

var statusCodes = map[byte]string{
	0x00: "Success",
	0x01: "Not Authorized",
	0x02: "Obsolete",
	0x03: "SP Busy",
	0x04: "SP Failed",
	0x05: "SP Disabled",
	0x06: "SP Frozen",
	0x07: "No Sessions Available",
	0x08: "Uniqueness Conflict",
	0x09: "Insufficient Space",
	0x0a: "Insufficient Rows",
	0x0c: "Invalid Parameter",
	0x0d: "Obsolete",
	0x0e: "Obsolete",
	0x0f: "TPer Malfunction",
	0x10: "Transaction Failure",
	0x11: "Response Overflow",
	0x12: "Authority Locked Out",
	0x3f: "Fail",
}

var stateTable = map[int64]commonCallOut{
	1: {runDiscovery, "Discovery"},
	2: {updateComID, "Update COMID"},
	3: {stopStateMachine, "Stop State Machine"},

	5: {tperReset, "TPer Reset"},
	6: {stopStateMachine, "Stop State Machine"},
	// Enable locking support
	7:  {runDiscovery, "Discovery"},
	8:  {updateComID, "Update COMID"},
	9:  {openAdminSession, "Open Admin Session"},
	10: {getMSID, "Get MSID"},
	11: {closeSession, "Close Session"},
	12: {openAdminSession, "Open Admin Session"},
	13: {getRandomPIN, "Get Random PIN"},
	14: {closeSession, "Close Session"},
	15: {openMSIDLockingSession, "Open Locking Session"},
	16: {setSIDpinRandom, "Set SID PIN Request"},
	17: {closeSession, "Close Session"},
	18: {openRandomLockingSession, "Open PIN Locking Session"},
	19: {activateLockingRequest, "Activate Locking Request"},
	20: {closeSession, "Close Session"},
	21: {openLockingSPSession, "Open SP Locking Session"},
	22: {setAdmin1Password, "Set Admin1 Password"},
	23: {enableUser1Password, "Enable User1 Password"},
	24: {changeUser1Password, "Change User1 Password"},
	25: {setDatastoreWrite, "Set Data Store Write Access"},
	26: {setDatastoreRead, "Set Data Store Read Access"},
	27: {enableRange0RWLock, "Enable Range0 RW Lock"},
	28: {closeSession, "End SP Locking Session"},
	29: {openUser1LockingSession, "Open Locking SP Session"},
	30: {setDatastore, "Set Data Store"},
	31: {closeSession, "Close Session"},
	32: {stopStateMachine, "Stop State Machine"},
	// Test of opening with PIN
	33: {runDiscovery, "Discovery"},
	34: {updateComID, "Update COMID"},
	35: {openLockingSPSession, "Open SP Locking Session"},
	36: {setAdmin1Password, "Set Admin1 Password"},
	37: {enableUser1Password, "Enable User1 Password"},
	38: {changeUser1Password, "Change User1 Password"},
	39: {setDatastoreWrite, "Set Data Store Write Access"},
	40: {setDatastoreRead, "Set Data Store Read Access"},
	41: {enableRange0RWLock, "Enable Range0 RW Lock"},
	42: {closeSession, "End SP Locking Session"},
	43: {openUser1LockingSession, "Open Locking SP Session"},
	44: {setDatastore, "Set Data Store"},
	45: {closeSession, "Close Session"},
	46: {stopStateMachine, "Stop State Machine"},
	// Test of TPer revert
	47: {runDiscovery, "Discovery"},
	48: {updateComID, "Update COMID"},
	49: {openAdminSession, "Open Admin Session"},
	50: {getMSID, "Get MSID"},
	51: {closeSession, "Close Session"},
	52: {openAdminSession, "Open Locking Session"},
	53: {setSIDpinMSID, "Set SID using MSID"},
	54: {closeSession, "Close Session"},
	55: {openPSIDLockingSession, "Open Locking Session"},
	56: {revertTPer, "Revert"},
	57: {closeSession, "Close Session"},
	58: {stopStateMachine, "Stop State Machine"},
	// Test of Secure Erase
	59: {runDiscovery, "Discovery"},
	60: {updateComID, "Update COMID"},
	61: {openPSIDLockingSession, "Open PSID Session"},
	62: {secureErase, "Secure Erase"},
	63: {closeSession, "Close Session"},
	64: {stopStateMachine, "Stop State Machine"},
}

func checkReturnStatus(reply []byte) bool {
	if len(reply) < 0x38 {
		fmt.Printf("Invalid reply, doesn't include length in packet. Length %d\n", len(reply))
		return false
	} else if reply[0x38] == 0xfa {
		// Special case for Close Session reply
		return true
	} else if reply[0x37] < 6 {
		fmt.Printf("Invalid payload length: %d\n", reply[0x37])
		return false
	}
	for offset := 0x38; offset < len(reply); offset++ {
		if reply[offset] == END_OF_DATA {
			if reply[offset+1] == START_LIST && reply[offset+5] == END_LIST {
				status := reply[offset+2]
				if status != 0 {
					fmt.Printf("    ERROR: %s\n", statusCodes[status])
					return false
				} else {
					return true
				}
			}
		}
	}
	fmt.Printf("  []---- Failed to find EOD_OF_DATA code ----[]\n")
	return false
}

func sendSecurityOutIn(pkt *comPacket) (bool, []byte) {
	full := pkt.getFullPayload()

	cdb := make([]byte, 12)
	cdb[0] = SECURITY_PROTO_OUT
	cdb[1] = 1
	shortAtData(cdb, pkt.globalData.comID, 2)
	intAtData(cdb, 1, 6)

	if _, err := sendUSCSI(pkt.fp, cdb, full, 0); err != nil {
		fmt.Printf("Failed %s for device\n", pkt.description)
		return false, nil
	}

	cdb[0] = SECURITY_PROTO_IN
	reply := make([]byte, 512)
	if _, err := sendUSCSI(pkt.fp, cdb, reply, 0); err != nil {
		fmt.Printf("Failed SECURITY_PROTOCOL_IN for %s\n", pkt.description)
		return false, nil
	} else if debugOutput {
		fmt.Printf("  []---- Response ----[]\n")
		dumpMemory(reply, len(reply), "    ")
	}
	if !checkReturnStatus(reply) {
		return false, nil
	}

	return true, reply

}

//noinspection GoUnusedParameter
func tperReset(fp *os.File, g *tcgData) (bool, int) {
	emmptyPayload := make([]byte, 512)

	cdb := make([]byte, 12)
	cdb[0] = SECURITY_PROTO_OUT
	cdb[1] = 2
	shortAtData(cdb, 4, 2)
	intAtData(cdb, uint32(len(emmptyPayload)), 6)

	if _, err := sendUSCSI(fp, cdb, emmptyPayload, 0); err != nil {
		fmt.Printf("  TPer Reset failed: %s\n", err)
		return false, 1
	} else {
		return true, 0
	}
}

func openAdminSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Open Admin Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, // Admin SP UID
		0x01,
		0xf1,
		0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func openMSIDLockingSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Open Locking Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, // Admin SP UID
		0x01, 0xf2, 0x00,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.msid) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.msid)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.msid)))
	}
	for _, v := range g.msid {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x06, 0xf3, 0xf1,
		0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func openPSIDLockingSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Open Locking Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, // Admin SP UID
		0x01, 0xf2, 0x00,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.psid) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.psid)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.psid)))
	}
	for _, v := range g.psid {
		newBuf = append(newBuf, byte(v))
	}
	newBuf = append(newBuf, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x01, 0xff, 0x01, 0xf3, 0xf1,
		0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func openUser1LockingSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Open Locking SP Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0, 0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x02,
		0x01, 0xf2, 0x00,
	}
	user1Pin := []byte{
		0x75, 0x73, 0x65, 0x72, 0x31,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(user1Pin) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(user1Pin)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(user1Pin)))
	}
	for _, v := range user1Pin {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x03, 0x00, 0x01, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func openRandomLockingSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Open PIN Locking Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, // Admin SP UID
		0x01, 0xf2, 0x00,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.randomPIN) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.randomPIN)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.randomPIN)))
	}
	for _, v := range g.randomPIN {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x06, 0xf3, 0xf1,
		0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}

}

func openLockingSPSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	pkt := createPacket("Start Locking SP", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x02, // Locking SP UID
		0x01, 0xf2, 0x00,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.randomPIN) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.randomPIN)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.randomPIN)))
	}
	for _, v := range g.randomPIN {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf2, 0x03, 0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x01, 0x00, 0x01, 0xf3, 0xf1,
		0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.putIntInPayload(0, 4) // HSN
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func getMSID(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Get MSID", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x00, 0x84, 0x02,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x16,
		0xf0,
		0xf0,
		0xf2, 0x03, 0x03, 0xf3,
		0xf2, 0x04, 0x03, 0xf3,
		0xf1, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		copyMSID(g, reply)
		fmt.Printf("    MSID: ")
		for _, b := range g.msid {
			fmt.Printf("%c", b)
		}
		fmt.Printf("\n")
		return true, 0
	} else {
		return false, 1
	}
}

func copyMSID(g *tcgData, reply []byte) {
	for i := 0; i < len(reply); i++ {
		if reply[i] == 0xf2 && reply[i+1] == 0x03 {
			tokenHeader := reply[i+2] & 0xf0
			msidLength := byte(0)
			offset := 0

			if tokenHeader == 0x80 {
				msidLength = reply[i+2&0x0f]
				offset = i + 3
			} else if tokenHeader == 0xd0 {
				msidLength = reply[i+3]
				offset = i + 4
			}
			g.msid = make([]byte, msidLength)
			copy(g.msid, reply[offset:offset+int(msidLength)])

			break
		}
	}
}

func getRandomPIN(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Get Random PIN", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x06, 0x01,
		0xf0, 0x20, 0xf1,
		0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		g.randomPIN = make([]byte, 0x20)
		copy(g.randomPIN, reply[0x3b:])
		fmt.Printf("    Randdom PIN:\n")
		dumpMemory(g.randomPIN, len(g.randomPIN), "    ")
		return true, 0
	} else {
		return false, 1
	}
}

func setAdmin1Password(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Admin1 Password", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x01, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x01, 0xf0, 0xf2, 0x03,
	}
	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.randomPIN) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.randomPIN)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.randomPIN)))
	}
	for _, v := range g.randomPIN {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func enableUser1Password(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Enable User1", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x03, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x01, 0xf0, 0xf2, 0x05, 0x01, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func changeUser1Password(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Change User1 Password", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x03, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x01, 0xf0, 0xf2, 0x03,
	}
	user1Pin := []byte{
		0x75, 0x73, 0x65, 0x72, 0x31,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(user1Pin) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(user1Pin)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(user1Pin)))
	}
	for _, v := range user1Pin {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9, 0xf0, 00, 00, 00, 0xf1)
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func setDatastoreWrite(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Data Store Write", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x08, 0x00, 0x03, 0xfc, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x01, 0xf0, 0xf2, 0x03, 0xf0, 0xf2, 0xa4, 0x00, 0x00, 0x0c, 0x05,
		0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x03, 0x00, 0x01,
		0xf3, 0xf1, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func setDatastoreRead(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Data Store Write", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x08, 0x00, 0x03, 0xfc, 0x00,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x01, 0xf0, 0xf2, 0x03, 0xf0, 0xf2, 0xa4, 0x00, 0x00, 0x0c, 0x05,
		0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x03, 0x00, 0x01,
		0xf3, 0xf1, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func enableRange0RWLock(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Data Store Write", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x00, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0,
		0xf2, 0x01, 0xf0, 0xf2, 0x05, 0x01, 0xf3, 0xf2, 0x06, 0x01, 0xf3, 0xf1,
		0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func setDatastore(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Data Store", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0, 0xf2, 0x00, 0x00, 0xf3, 0xf2, 0x01,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.randomPIN) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.randomPIN)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.randomPIN)))
	}
	for _, v := range g.randomPIN {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.subpacket = newBuf
	pkt.fini()

	if ok, _ := sendSecurityOutIn(pkt); ok {
		return true, 0
	} else {
		return false, 1
	}
}

func revertTPer(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Revert TPer", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8, // Call Token
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x02, 0x02,
		0xf0, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1 // For now just ignore a failure so that we close the session.
}

func activateLockingRequest(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Activate Locking Request", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x02,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x02, 0x03,
		0xf0, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1, 0x00,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	ok, _ := sendSecurityOutIn(pkt)
	return ok, 1
}

func commonSID(pkt *comPacket, sid []byte) bool {
	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x00, 0x00, 0x01,
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17,
		0xf0,
		0xf2, 0x01, 0xf0, 0xf2, 0x03,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(sid) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(sid)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(sid)))
	}
	for _, v := range sid {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.subpacket = newBuf
	pkt.fini()

	ok, _ := sendSecurityOutIn(pkt)

	return ok
}

//noinspection ALL
func setSIDpinMSID(fp *os.File, g *tcgData) (bool, int) {
	/*
	pkt := createPacket("Set SID from MSID", g, fp)
	return commonSID(pkt, g.msid), 1
	*/
	fmt.Printf("    Non-functional for now\n")
	return true, 0
}

func setSIDpinRandom(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set SID PIN Request", g, fp)
	return commonSID(pkt, g.randomPIN), 1
}

func getSPSessionID(payload []byte) uint32 {
	// offset := SESSION_ID_OFFSET + (payload[SESSION_ID_OFFSET] & 0x3f) + 2
	returnVal := uint32(0)

	payloadLen := payload[0x51] & 0x3f
	if payloadLen <= 4 {
		converter := dataToInt{payload, 0x52, int(payloadLen)}
		returnVal = uint32(converter.getInt())
		fmt.Printf("    SessionID: 0x%x\n", returnVal)
	} else {
		fmt.Printf("  []---- Invalid SessionID ----[]\n")
	}
	return returnVal
}

func secureErase(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Secure Erase", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
		0xA8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x00, 0x00, 0x02,
		0xA8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x08, 0x03,
		0xF0, 0xF1, 0xF9,
		0xF0, 0x00, 0x00, 0x00, 0xF1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func closeSession(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Close Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xfa,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

//noinspection ALL
func stopStateMachine(fp *os.File, g *tcgData) (bool, int) {
	return false, 0
}

func runDiscovery(fp *os.File, g *tcgData) (bool, int) {

	cdb := make([]byte, 12)
	data := make([]byte, 512)

	cdb[0] = SECURITY_PROTO_OUT
	cdb[1] = 1 // Protocol 1 == Discovery
	cdb[2] = 0
	cdb[3] = 1
	cdb[4] = 0x80 // INC_512 bit.
	intAtData(cdb, 1, 6)

	if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
		fmt.Printf("Send of Level 0 discovery failed, err=%s\n", err)
		return false, 1
	}

	cdb[0] = SECURITY_PROTO_IN
	if dataLen, err := sendUSCSI(fp, cdb, data, 0); err != nil {
		fmt.Printf("USCSI failed, err=%s\n", err)
		return false, 1
	} else {
		if debugOutput {
			dumpMemory(data, dataLen, "  ")
		}
		dumpLevelZeroDiscovery(data, dataLen, g)
	}
	return true, 0
}

func updateComID(fp *os.File, g *tcgData) (bool, int) {
	cdb := make([]byte, 12)
	data := make([]byte, 512)

	if g.opalDevice {
		// Section 3.3.4.3.1 Storage Architecture Core Spec v2.01_r1.00
		cdb[1] = 2             // Protocol ID 2 == GET_COMID
		shortAtData(cdb, 0, 2) // COMID must be equal zero

		data = make([]byte, 512)
		data[5] = 0
		data[19] = 255

		/*
		cdb[0] = SECURITY_PROTO_OUT
		if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("    Hmm... SECURITY_OUT failed for ComID request\n")
		} else {
			fmt.Printf("    SECURITY_OUT okay\n")
		}
		*/

		cdb[0] = SECURITY_PROTO_IN
		if dataLen, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("USCSI Protocol 2 failed, err=%s\n", err)
			return false, 1
		} else {
			if debugOutput {
				fmt.Printf("  []---- Response ----[]\n")
				dumpMemory(data, dataLen, "    ")
			}
			converter := dataToInt{data, 0, 2}
			g.comID = uint16(converter.getInt())
		}
	} else if g.rubyDevice {
		if g.comID == 0 {
			fmt.Printf("Failed to get Ruby ComID\n")
			return false, 1
		}
	} else {
		fmt.Printf("Device type uknown\n")
		g.comID = 0x7ffe
	}
	fmt.Printf("    ComID: 0x%x\n", g.comID)
	return true, 0
}


func dumpLevelZeroDiscovery(data []byte, len int, g *tcgData) {
	if len < 48 {
		fmt.Printf("Invalid Discovery 0 header\n")
		return
	}

	if debugOutput {
		fmt.Printf("  []---- Response ----[]\n")
		dumpMemory(data, len, "    ")
	}
	header := dataToInt{data, 0, 4}
	paramLen := header.getInt() - 44
	if paramLen <= 0 {
		fmt.Printf("Invalid paramLen of %d\n", paramLen)
		return
	}

	header.offset = 4
	header.count = 2
	majorVers := header.getInt()

	header.offset = 6
	minorVers := header.getInt()

	fmt.Printf("Level 0 Discovery\n  Data available: %d\n  Version       : %d.%d\n", paramLen,
		majorVers, minorVers)
	if g.psid != "" {
		fmt.Printf("  PSID: %s\n", g.psid)
	} else {
		fmt.Printf("  PSID: NO VALUE AVAILABLE\n")
	}
	fmt.Println()

	for offset := 0; offset < paramLen; {
		offset += dumpDescriptor(data[offset+48:], g)
	}
}

type featureFuncName struct {
	name string
	dump func([]byte, *tcgData)
}

var codeStr = map[int]featureFuncName {
	0x0001: {"TPer", dumpTPerFeature},
	0x0002: {"Locking", dumpLockingFeature},
	0x0003: {"Geometry Reporting", dumpGeometryFeature},
	0x0201: {"Opal Single User mode", dumpOpalSingleUser},
	0x0202: {"Additional DataStore", dumpAdditionalDataStore},
	0x0203: {"Opal v2.01_rev1.00 SSC", dumpOpalV2Feature},
	0x0304: {"Ruby SSC", dumpRubyFeature},
	0x0402: {"Set Block SID", dumpSetBlockSID},
	0x0403: {"Configurable Namespace Locking", dumpCNL},
}

type desciptorCodeRanges struct {
	startRange int
	endRange   int
	name       string
}

var descriptorCodeRangeArray = []desciptorCodeRanges{
	{0x0000, 0x0000, "Reserved"},
	{0x0001, 0x0001, "TPer feature"},
	{0x0002, 0x0002, "Locking feature"},
	{0x0003, 0x00ff, "Reserved"},
	{0x0100, 0x03ff, "SSCs"},
	{0x0400, 0xbfff, "Reserved"},
	{0xc000, 0xffff, "Vendor Unique"},
}

func dumpDescriptor(data []byte, g *tcgData) int {
	template := dataToInt{data, 0, 2}
	code := template.getInt()

	template.offset = 3
	template.count = 1
	featureLen := template.getInt()

	if _, ok := codeStr[code]; ok {
		fmt.Printf("  Feature: %s\n", codeStr[code].name)
		codeStr[code].dump(data, g)
		fmt.Printf("\n")
	} else {
		for _, v := range descriptorCodeRangeArray {
			if code >= v.startRange && code <= v.endRange {
				fmt.Printf("  Feature: %s (code: 0x%x)\n", v.name, code)
				dumpMemory(data, featureLen, "    ")
				fmt.Println()
				break
			}
		}
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

//noinspection GoUnusedParameter
func dumpAdditionalDataStore(data []byte, g *tcgData) {
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

//noinspection GoUnusedParameter
func dumpTPerFeature(data []byte, g *tcgData) {
	doBitDump(tperBitMap, data)
}

var lockingBitMap = []bitMaskBitDump {
	{2, 4, 0x0f, "Version"},
	{3,0,0xff,"Length"},
	{4,5,1,"MBR_Done"},
	{4,4,1,"MBR_Enabled"},
	{4,3,1,"Media_Encryption"},
	{4,2,1,"Locked"},
	{4,1,1,"Locking_Enabled"},
	{4,0,1,"Locking_Supported"},
}

var singleUserBitMap = []bitMaskBitDump{
	{8, 2, 0x01, "Policy"},
	{8, 1, 0x01, "All"},
	{8, 0, 0x01, "Any"},
}
var singleUserMultiByte = []multiByteDump{
	{0, 2, "Feature Code"},
	{3, 1, "Length"},
	{4, 4, "# of Locking Objects Supported"},
}

//noinspection GoUnusedParameter
func dumpOpalSingleUser(data []byte, g *tcgData) {
	doBitDump(singleUserBitMap, data)
	doMultiByteDump(singleUserMultiByte, data)
}

func dumpLockingFeature(data []byte, g *tcgData) {
	if data[4] & 1 == 1 {
		g.lockingSupported = true
	}
	if ((data[4] >> 1) & 1) == 1 {
		g.lockingEnabled = true
	}
	doBitDump(lockingBitMap, data)
}

var geometryBitMap = []bitMaskBitDump{
	{4, 0, 0x01, "Align"},
}

var geometryMultiByte = []multiByteDump{
	{0, 2, "Feature Code"},
	{3, 1, "Length"},
	{12, 4, "Logical Block Size"},
	{16, 8, "Alignment Granularity"},
	{24, 8, "Lowest Aligned LBA"},
}

//noinspection GoUnusedParameter
func dumpGeometryFeature(data []byte, g *tcgData) {
	doBitDump(geometryBitMap, data)
	doMultiByteDump(geometryMultiByte, data)
}

var rubyMulitByte = []multiByteDump {
	{0,2,"Feature Code"},
	{3,1,"Length"},
	{4,2,"Base ComID"},
	{6,2,"Number of ComIDs"},
	{9,2,"# locking Admin SPs supported"},
	{11,2,"# locking User SPs supported"},
}

func dumpOpalV2Feature(data []byte, g *tcgData) {
	g.opalDevice = true
	doMultiByteDump(rubyMulitByte, data)
}

func dumpRubyFeature(data []byte, g *tcgData) {
	g.rubyDevice = true
	g.opalDevice = false
	converter := dataToInt{data, 4,2}
	g.comID = (uint16)(converter.getInt())
	doMultiByteDump(rubyMulitByte, data)
}

var cnlMultiByte = []multiByteDump{
	{8, 4, "Maximum Key Count"},
	{12, 4, "Unused Key Count"},
	{16, 4, "Maximum Ranges Per Namespace"},
}

var cnlBitMap = []bitMaskBitDump{
	{4, 7, 0x1, "Range_C"},
	{4, 6, 0x1, "Rance_P"},
}

//noinspection GoUnusedParameter
func dumpCNL(data []byte, g *tcgData) {
	doBitDump(cnlBitMap, data)
	doMultiByteDump(cnlMultiByte, data)
}

var sbsBitMap = []bitMaskBitDump{
	{4, 3, 0x01, "Locking_SP_Freeze_Lock_State"},
	{4, 2, 0x01, "Locking_SP_Freeze_Lock_supported"},
	{4, 1, 0x01, "SID_Authentication_Blocked_State"},
	{4, 0, 0x01, "SID_Value_State"},
	{5, 0, 0x01, "Hardware_Reset"},
}

//noinspection GoUnusedParameter
func dumpSetBlockSID(data []byte, g *tcgData) {
	doBitDump(sbsBitMap, data)
}