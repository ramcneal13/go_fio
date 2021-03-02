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
}

func sedCommand(fp *os.File) {

	// Default to the device being an Opal device. It may not provide
	// a feature code page 0x203 during Level 0 Discovery
	tcgGlobal := &tcgData{true,false,false,false,
		0, 0, 0x0, nil, nil}

	/*
	randomPin1 := []byte {
		0xb1, 0x19, 0x1f, 0x25, 0x81, 0x9f, 0x58, 0x59,
		0x2c, 0x45, 0x2d, 0xb6, 0x8e, 0xda, 0x29, 0xd0,
		0x9f, 0x80, 0x97, 0xe1, 0xed, 0x05, 0x57, 0xc6,
		0xf9, 0xc0, 0xc6, 0x67, 0x29, 0x10, 0x56, 0x1c,
	}
	*/
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
	4: {setSIDpin, "Set SID PIN"},
	5: {closeSession, "Close Session"},
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
	15: {openLockingSession, "Open Locking Session"},
	16: {setSIDpinRequest, "Set SID PIN Request"},
	17: {closeSession, "Close Session"},
	18: {openPINLockingSession, "Open PIN Locking Session"},
	19: {activateLockingRequest, "Activate Locking Request"},
	20: {closeSession, "Close Session"},
	21: {startLockingSPSession, "Open SP Locking Session"},
	22: {setAdmin1Password, "Set Admin1 Password"},
	23: {enableUser1Password, "Enable User1 Password"},
	24: {changeUser1Password, "Change User1 Password"},
	25: {setDatastoreWrite, "Set Data Store Write Access"},
	26: {setDatastoreRead, "Set Data Store Read Access"},
	27: {enableRange0RWLock, "Enable Range0 RW Lock"},
	28: {closeSession, "End SP Locking Session"},
	29: {openLockingSPSession, "Open Locking SP Session"},
	30: {setDatastore, "Set Data Store"},
	31: {closeSession, "Close Session"},
	32: {stopStateMachine, "Stop State Machine"},
	// Test of opening with PIN
	33: {runDiscovery, "Discovery"},
	34: {updateComID, "Update COMID"},
	35: {startLockingSPSession, "Open SP Locking Session"},
	36: {setAdmin1Password, "Set Admin1 Password"},
	37: {enableUser1Password, "Enable User1 Password"},
	38: {changeUser1Password, "Change User1 Password"},
	39: {setDatastoreWrite, "Set Data Store Write Access"},
	40: {setDatastoreRead, "Set Data Store Read Access"},
	41: {enableRange0RWLock, "Enable Range0 RW Lock"},
	42: {closeSession, "End SP Locking Session"},
	43: {openLockingSPSession, "Open Locking SP Session"},
	44: {setDatastore, "Set Data Store"},
	45: {closeSession, "Close Session"},
	46: {stopStateMachine, "Stop State Machine"},
	// Test of TPer revert
	47: {runDiscovery, "Discovery"},
	48: {updateComID, "Update COMID"},
	49: {openRevertLockingSession, "Open Revert Locking"},
	50: {revertTPer, "Revert TPer"},
	51: {closeSession, "Close Session"},
	52: {stopStateMachine, "Stop State Machine"},
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
	intAtData(cdb, (uint32)(len(full)), 6)

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

	pkt.subpacket = hardCoded
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

func openLockingSession(fp *os.File, g *tcgData) (bool, int) {
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

	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}
}

func startLockingSPSession(fp *os.File, g *tcgData) (bool, int) {
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
	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
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

func openLockingSPSession(fp *os.File, g *tcgData) (bool, int) {
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

	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
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

func openPINLockingSession(fp *os.File, g *tcgData) (bool, int) {
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

	pkt.subpacket = newBuf
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
		return true, 0
	} else {
		return false, 1
	}

}

func openRevertLockingSession(fp *os.File, g *tcgData) (bool, int) {
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
		0xd0, 0x12,
		0x3c, 0x6e, 0x65, 0x77, 0x5f, 0x53, 0x49, 0x44, 0x5f, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x3e,
		0xf3, 0xf2, 0x03,
		0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x06,
		0xf3, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, reply := sendSecurityOutIn(pkt); ok {
		pkt.globalData.spSessionID = getSPSessionID(reply)
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
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x02, 0x02,
		0xf0, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	ok, _ := sendSecurityOutIn(pkt)
	return ok, 1
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

func setSIDpinRequest(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set SID PIN Request", g, fp)

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

	if len(g.randomPIN) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.randomPIN)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.randomPIN)))
	}
	for _, v := range g.randomPIN {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.subpacket = newBuf
	pkt.fini()

	ok, _ := sendSecurityOutIn(pkt)

	return ok, 1
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

func setSIDpin(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Get MSID", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x0b, 0x00, 0x00, 0x00, 0x01, // SID UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // set SID PIN
		0xf0,
		0xf2, 0x01, 0xf0, 0xf2, 0x03,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	ok, _ := sendSecurityOutIn(pkt)
	return ok, 1
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

		cdb[0] = SECURITY_PROTO_OUT
		if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("    Hmm... SECURITY_OUT failed for ComID request\n")
		} else {
			fmt.Printf("    SECURITY_OUT okay\n")
		}

		cdb[0] = SECURITY_PROTO_IN
		if _, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("USCSI Protocol 2 failed, err=%s\n", err)
			return false, 1
		} else {
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
	for offset := 48; offset < paramLen ; {
		offset += dumpDescriptor(data[offset:], g)
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

var geometryMultiByte = []multiByteDump{
	{0, 2, "Feature Code"},
	{3, 1, "Length"},
}

//noinspection GoUnusedParameter
func dumpGeometryFeature(data []byte, g *tcgData) {
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
