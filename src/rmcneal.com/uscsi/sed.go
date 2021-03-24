package main

import (
	"os"
	"fmt"
)

//noinspection ALL,GoSnakeCaseUsage
const (
	START_LIST      = 0xf0
	END_LIST        = 0xf1
	END_OF_DATA     = 0xf9
	tcgDeviceOpalV1 = 1
	tcgDeviceOpalV2 = 2
	tcgDeviceRuby   = 3
)

type tcgData struct {
	deviceType       int
	lockingEnabled   bool
	lockingSupported bool
	comID            uint16
	sequenceNum      uint32
	spSessionID      uint32
	msid             []byte
	psid             string
	master           string
	range1UID        []byte
}

type commonCallOut struct {
	method func(fp *os.File, data *tcgData) (bool, int)
	name   string
}

var user1Pin = []byte{0x75, 0x73, 0x65, 0x72, 0x31,}

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

type nameStateDescriptor struct {
	stateTable map[int]commonCallOut
	helpString string
}

func sedCommand(fp *os.File) {
	var ok bool
	var currentTable nameStateDescriptor
	currentState := 0

	// Default to the device being an Opal device. It may not provide
	// a feature code page 0x203 during Level 0 Discovery
	tcgGlobal := &tcgData{tcgDeviceOpalV1, false, false,
		0, 0, 0x0, nil, "", "", nil}

	if err := loadPSIDpairs(tcgGlobal); err != nil {
		fmt.Printf("Loading of PSID failed: %s\n", err)
	}

	if currentTable, ok = nameToState[sedOption]; !ok {
		fmt.Printf("Valid options are:\n")
		for name, table := range nameToState {
			fmt.Printf("  %s -- %s\n", name, table.helpString)
		}
		return
	}

	for {
		callOut, ok := currentTable.stateTable[currentState]
		if !ok {
			fmt.Printf("Invalid starting state: %d\n", currentState)
			return
		}

		// Methods should return true to proceed to the next state. Under normal
		// conditions the last method will return false to end the state machine.
		// Should a method encounter an error it's expected that the method will
		// display any appropriate error messages.
		fmt.Printf("[%d]---- %s ----[]\n", currentState, callOut.name)
		if ok, exitCode := callOut.method(fp, tcgGlobal); !ok {
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return
		}
		currentState++
	}
}

var nameToState = map[string]nameStateDescriptor{
	"discovery":    {discoveryStateTable, "Run just discovery phase"},
	"enable":       {enableStateTable, "Enable locking for drive"},
	"revert":       {revertStateTable, "Revert drive to factory settings"},
	"erase-opalv1": {eraseOpalV1StateTable, "Secure Opal V1 erase drive"},
	"pin":          {resetMasterStateTable, "CAUTION: Reset Master password"},
	"master":       {masterRevertStateTable, "Revert using Master key"},
	"erase":        {eraseStateTable, "Secure Erase drive"},
	"lock":         {lockStateTable, "Lock Range 1"},
	"unlock":       {unlockStateTable, "Unlock Range 1"},
	"experiment":   {opalv1experiment, "Experiment for Opal v1 support"},
}

var deviceTypeToName = map[int]string{
	tcgDeviceOpalV1: "Opal v1",
	tcgDeviceOpalV2: "Opal v2",
	tcgDeviceRuby:   "Ruby",
}

var opalv1experiment = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openAdminSession, "Open Admin session"},
	3: {closeSession, "Close Seession"},
	4: {stopStateMachine, "Stop State Machine"},
}

var eraseStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openLockingSPSession, "Open SP Locking Session"},
	3: {setLockingRange1, "Set Locking Range 1"},
	4: {retrieveLockingUID, "Retrieve Locking UID"},
	5: {eraseWithGenKey, "Erase with Genkey"},
	6: {closeSession, "Close Session"},
	7: {stopStateMachine, "Stop State Machine"},
}

var lockStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openLockingSPSession, "Open SP Locking Session"},
	3: {retrieveLockingUID, "Retrieve Locking UID"},
	4: {lockRange1, "Lock Range 1"},
	5: {closeSession, "Close Session"},
	6: {stopStateMachine, "Stop State Machine"},
}

var unlockStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openLockingSPSession, "Open SP Locking Session"},
	3: {retrieveLockingUID, "Retrieve Locking UID"},
	4: {unlockRange1, "Unlock Range 1"},
	5: {closeSession, "Close Session"},
	6: {stopStateMachine, "Stop State Machine"},
}
var discoveryStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {stopStateMachine, "Stop State Machine"},
}

var enableStateTable = map[int]commonCallOut{
	0:  {runDiscovery, "Discovery"},
	1:  {updateComID, "Update COMID"},
	2:  {openAdminSession, "Open Admin Session"},
	3:  {getMSID, "Get MSID"},
	4:  {closeSession, "Close Session"},
	5:  {openMSIDLockingSession, "Open Locking Session"},
	6:  {setSIDpinFromMaster, "Set SID PIN from Master Request"},
	7:  {closeSession, "Close Session"},
	8:  {openAdminWithMasterKey, "Open PIN Locking Session"},
	9:  {activateLockingRequest, "Activate Locking Request"},
	10: {closeSession, "Close Session"},
	11: {openLockingSPSession, "Open SP Locking Session"},
	12: {setAdmin1Password, "Set Admin1 Password"},
	13: {enableUser1Password, "Enable User1 Password"},
	14: {changeUser1Password, "Change User1 Password"},
	15: {setDatastoreWrite, "Set Data Store Write Access"},
	16: {setDatastoreRead, "Set Data Store Read Access"},
	17: {enableRange0RWLock, "Enable Range0 RW Lock"},
	18: {setLockingRange1, "Set Locking Range 1"},
	19: {closeSession, "End SP Locking Session"},
	20: {openUser1LockingSession, "Open Locking SP Session"},
	21: {setDatastore, "Set Data Store"},
	22: {closeSession, "Close Session"},
	23: {stopStateMachine, "Stop State Machine"},
}

var revertStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openPSIDLockingSession, "Open Locking Session"},
	3: {revertTPer, "Revert"},
	4: {closeSession, "Close Session"},
	5: {stopStateMachine, "Stop State Machine"},
}

var eraseOpalV1StateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openAdminSession, "Open Admin Session"},
	3: {getMSID, "Get MSID"},
	4: {closeSession, "Close Session"},
	5: {openTestLockingSession, "Open Test Locking Session"},
	6: {authUser, "Authenticate User"},
	7: {secureErase, "Secure Erase"},
	8: {closeSession, "Close Session"},
	9: {stopStateMachine, "Stop State Machine"},
}

var resetMasterStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openAdminSession, "OpenAdminSession"},
	3: {getRandomPIN, "Get Random PIN"},
	4: {closeSession, "Close Session"},
	5: {stopStateMachine, "Stop State Machine"},
}

var masterRevertStateTable = map[int]commonCallOut{
	0: {runDiscovery, "Discovery"},
	1: {updateComID, "Update COMID"},
	2: {openAdminWithMasterKey, "Open PIN Locking Session"},
	3: {revertTPer, "Revert"},
	4: {closeSession, "Close Session"},
	5: {stopStateMachine, "Stop State Machine"},
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
	if pkt.globalData.deviceType == tcgDeviceOpalV1 {
		shortAtData(cdb, 0x8000, 4)
		cdb[9] = 1
	}

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
	g.sequenceNum = 0
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
	g.sequenceNum = 0
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
	g.sequenceNum = 0
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
	g.sequenceNum = 0
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

func openTestLockingSession(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	g.sequenceNum = 0
	pkt := createPacket("Open Locking SP Session", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xa8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02,
		0xf0,
		0x84, 0x10, 0x00, 0x00, 0x00,                         // HSN
		0xa8, 0x00, 0x00, 0x02, 0x05, 0x00, 0x00, 0x00, 0x01, // Locking SP UID
		0x01,                                                 // Write
		0xf1,
		0xf9,
		0xf0, 00, 00, 00, 0xf1,
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

func openAdminWithMasterKey(fp *os.File, g *tcgData) (bool, int) {
	g.spSessionID = 0
	g.sequenceNum = 0
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

	if len(g.master) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.master)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.master)))
	}
	for _, v := range g.master {
		newBuf = append(newBuf, byte(v))
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
	g.sequenceNum = 0
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

	if len(g.master) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.master)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.master)))
	}
	for _, v := range g.master {
		newBuf = append(newBuf, byte(v))
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
		randomPIN := make([]byte, 0x20)
		copy(randomPIN, reply[0x3b:])
		fmt.Printf("    Randdom PIN:\n")
		dumpMemory(randomPIN, len(randomPIN), "    ")
		g.master = randomToMaster(randomPIN, 32)
		updateMaster(g.master)
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

	if len(g.master) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.master)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.master)))
	}
	for _, v := range g.master {
		newBuf = append(newBuf, byte(v))
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
		0xa8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x03, 0x00, 0x01, // Locking SP Authority Table User1 UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // "Set" method
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

func setLockingRange1(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Locking Range1", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x03, 0x00, 0x01, // Locking Range1 UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // "Set" method UID
		0xf0,
		0xf2,
		0x01, // "Values"
		0xf0, 0xf2,
		0x03,             // "RangeStart"
		0x82, 0x00, 0x00, // Starting range
		0xf3,
		0xf2,
		0x04,             // "RangeLength"
		0x82, 0x01, 0x00, // Range length
		0xf3, 0xf2,
		0x05, // ReadLockEnabled
		0x01, // True
		0xf3,
		0xf2,
		0x06, // WriteLockEnabled
		0x01, // True
		0xf3, 0xf1, 0xf3, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func retrieveLockingUID(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set Locking Range1", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x03, 0x00, 0x01, // Locking Range1 UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x16, // "Get" method UID
		0xf0, 0xf0, 0xf2,
		0x03, // "startColumn"
		0x0a, // <ActiveKey>
		0xf3, 0xf2,
		0x04, // "endColumn"
		0x0a, // <ActiveKey>
		0xf3, 0xf1, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}
	pkt.subpacket = hardCoded
	pkt.fini()

	if ok, payload := sendSecurityOutIn(pkt); ok {
		g.range1UID = getLockingRange1UID(payload)
		dumpMemory(g.range1UID, 8, "    ")
	}
	return true, 1
}

func eraseWithGenKey(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Authenticate User", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.range1UID) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.range1UID)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.range1UID)))
	}
	for _, v := range g.range1UID {
		newBuf = append(newBuf, v)
	}
	newBuf = append(newBuf, 0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x10) // Genkey Method UID
	newBuf = append(newBuf, 0xf0, 0xf1, 0xf9, 0xf0, 0x00, 0x00, 0x00, 0xf1)

	pkt.subpacket = newBuf
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func lockRange1(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Authenticate User", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x03, 0x00, 0x01, // Locking Range1 UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // "Set" method UID
		0xf0, 0xf2,
		0x01, // "Values"
		0xf0, 0xf2,
		0x07, // "ReadLocked"
		0x01, // True
		0xf3, 0xf2,
		0x08, // "WriteLocked"
		0x01, // True
		0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func unlockRange1(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Authenticate User", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x03, 0x00, 0x01, // Locking Range1 UID
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // "Set" method UID
		0xf0, 0xf2,
		0x01, // "Values"
		0xf0, 0xf2,
		0x07, // "ReadLocked"
		0x00, // False
		0xf3, 0xf2,
		0x08, // "WriteLocked"
		0x00, // False
		0xf3, 0xf1, 0xf3, 0xf1, 0xf9,
		0xf0, 0x00, 0x00, 0x00, 0xf1,
	}

	pkt.subpacket = hardCoded
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func enableRange0RWLock(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Enable Range0 RW Lock", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf8,
		0xa8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x00, 0x00, 0x01, // Locking Global Range
		0xa8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x17, // "Set" Method UID
		0xf0,                                                 // Start List Token
		0xf2,                                                 // Start Name Token
		0x01,
		0xf0, // Start Token List
		0xf2, // Start Name Token
		0x05, 0x01,
		0xf3, // End Name Token
		0xf2, // Start Name Token
		0x06, 0x01,
		0xf3, // End Name Token
		0xf1, // End List Token
		0xf3, // End Name Token
		0xf1, // End List Token
		0xf9,
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

	if len(g.master) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.master)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.master)))
	}
	for _, v := range g.master {
		newBuf = append(newBuf, byte(v))
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

func commonSID(pkt *comPacket, sid string) bool {
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
		newBuf = append(newBuf, byte(v))
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

func setSIDpinFromMaster(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Set SID PIN Request", g, fp)
	return commonSID(pkt, g.master), 1
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

func getLockingRange1UID(payload []byte) []byte {
	if payload[0x3c] == 0xa8 {
		return payload[0x3d:0x45]
	} else {
		return nil
	}
}

func authUser(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Authenticate User", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
		0xA8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xA8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x0C,
		0xF0,
		0xA8, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x06, // SID_Authority_object UID
		0xF2,
		0xA9, 0x43, 0x68, 0x61, 0x6C, 0x6C, 0x65, 0x6E, 0x67, 0x65, // Challenge
	}

	newBuf := make([]byte, len(hardCoded))
	copy(newBuf, hardCoded)

	if len(g.msid) <= 15 {
		newBuf = append(newBuf, byte(0xa0|len(g.msid)))
	} else {
		newBuf = append(newBuf, byte(0xd0), byte(len(g.msid)))
	}
	for _, v := range g.msid {
		newBuf = append(newBuf, byte(v))
	}
	newBuf = append(newBuf, 0xF3, 0xF1, 0xF9, 0xF0, 0x00, 0x00, 0x00, 0xF1)

	pkt.subpacket = newBuf
	pkt.fini()

	sendSecurityOutIn(pkt)
	return true, 1
}

func secureErase(fp *os.File, g *tcgData) (bool, int) {
	pkt := createPacket("Secure Erase", g, fp)

	hardCoded := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0xF8,
		0xA8, 0x00, 0x00, 0x08, 0x02, 0x00, 0x00, 0x00, 0x01, // Locking Global Range
		0xA8, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x08, 0x03, // Erase method UID
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
		if name, ok := deviceTypeToName[g.deviceType]; ok {
			fmt.Printf("  Device Type: %s\n", name)
		} else {
			fmt.Printf("  Failed to determine device type or unsupported device type set\n")
			return false, 1
		}

	}
	return true, 0
}

func updateComID(fp *os.File, g *tcgData) (bool, int) {
	cdb := make([]byte, 12)
	data := make([]byte, 512)

	if g.deviceType == tcgDeviceOpalV2 {
		// Section 3.3.4.3.1 Storage Architecture Core Spec v2.01_r1.00
		cdb[1] = 2             // Protocol ID 2 == GET_COMID
		shortAtData(cdb, 0, 2) // COMID must be equal zero

		data = make([]byte, 512)
		data[5] = 0
		data[19] = 255

		cdb[0] = SECURITY_PROTO_IN
		if dataLen, err := sendUSCSI(fp, cdb, data, 0); err != nil {
			fmt.Printf("  USCSI Protocol 2 failed, assuming Opal v1\n")
			return false, 1
		} else {
			if debugOutput {
				fmt.Printf("  []---- Response ----[]\n")
				dumpMemory(data, dataLen, "    ")
			}
			converter := dataToInt{data, 0, 2}
			g.comID = uint16(converter.getInt())
		}
	} else if g.deviceType == tcgDeviceRuby {
		if g.comID == 0 {
			fmt.Printf("Failed to get Ruby ComID\n")
			return false, 1
		}
	} else if g.deviceType == tcgDeviceOpalV1 {
		g.comID = 0x7fe
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
	g.deviceType = tcgDeviceOpalV2
	doMultiByteDump(rubyMulitByte, data)
}

func dumpRubyFeature(data []byte, g *tcgData) {
	g.deviceType = tcgDeviceRuby
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
	{4, 6, 0x1, "Range_P"},
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