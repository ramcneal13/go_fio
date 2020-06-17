package main

import (
	"os"
	"fmt"
	"rmcneal.com/support"
	"bytes"
)

type inquiryNameAndFunc struct {
	name	string
	decode	func([]byte, int)
}

var pageCodeFuncs map[byte]inquiryNameAndFunc
var designatorType map[byte]string
var NAAField map[byte]string
var codeSet map[byte]string
var deviceType map[byte]string
var ataCommandCode map[byte]string
var extendInquiry []bitMaskBitDump
var modePagePolicy map[byte]string
var powerConditionBits []bitMaskBitDump
var powerConditionBytes []multiByteDump
var powerConsumptionUnits map[byte]string
var standardInquiryBits []bitMaskBitDump
var blockLimitsBytes []multiByteDump

func init() {
	extendInquiry = []bitMaskBitDump{
		{0, 0, 0x1f, "device type"},
		{4, 6, 0x3, "Active Microcode"},
		{4, 3, 0x7, "SPT"},
		{4, 2, 0x1, "GRD_CHK"},
		{4, 1, 0x1, "APP_CHK"},
		{4, 0, 0x1, "REF_CHK"},
		{5, 5, 0x1, "UASK_SUP"},
		{5, 4, 0x1, "GROUP_SUP"},
		{5, 3, 0x1, "PRIOR_SUP"},
		{5, 2, 0x1, "HEADSUP"},
		{5, 1, 1, "ORDSUP"},
		{5, 0, 1, "SIMPSUP"},
		{6, 3, 1, "WU_SUP"},
		{6, 2, 1, "CRD_SUP"},
		{6, 1, 1, "NV_SUP"},
		{6, 0, 1, "V_SUP"},
		{7, 4, 1, "P_I_I_SUP"},
		{7, 0, 1, "LUICLR"},
		{8, 4, 1, "R_SUP"},
		{8, 0, 1, "CBCS"},
		{9, 0, 0xf, "MULTI I_T MICROCODE DOWNLOAD"},
		{12, 7, 1, "POA_SUP"},
		{12, 6, 1, "HRA_SUP"},
		{12, 5, 1, "VSA_SUP"},
		{13, 0, 0xff, "Maximum supported sense data length"},
	}

	powerConditionBits = []bitMaskBitDump{
		{4, 1, 1, "Standby_y"},
		{4, 0, 1, "Standby_z"},
		{5, 2, 1, "Idle_c" },
		{5, 1, 1, "Idle_b"},
		{5, 0, 1, "Idle_a"},
	}

	powerConditionBytes = []multiByteDump {
		{6, 2, "Stopped condition recovery time (ms)"},
		{8, 2, "Standby_z condition recovery time (ms)"},
		{10, 2, "Standby_y condition recovery time (ms)"},
		{12, 2, "Idle_a condition recovery time (ms)"},
		{14, 2, "Idle_b condition recovery time (ms)"},
		{16, 2, "Idle_c condition recovery time (ms)"},
	}

	standardInquiryBits = []bitMaskBitDump {
		{1,7,1, "RMB"},
		{1,6,1, "LU_CONG"},
		{2,0,0xff, "Version"},
		{3,5,1, "NormACA"},
		{3,4,1, "HiSup"},
		{3,0,0xf, "Response_Data_Format"},
		{5,7,1, "SCCS"},
		{5,6,1, "ACC"},
		{5,4, 0x3, "TPGS"},
		{5,3,1,"3PC"},
		{5, 0,1,"Protect"},
		{6,6,1,"EncServ"},
		{6,5,1,"VS"},
		{6,4,1,"MultiP"},
		{6,0,1,"ADDR16"},
		{7,5,1,"WBUS16"},
		{7,4,1,"SYNC"},
		{7,1, 1,"CmdQue"},
		{7,0,1,"VS"},
		{56,2,3,"Clocking"},
		{56,1,1,"QAS"},
		{56,1,1,"IUS"},
	}

	pageCodeFuncs = map[byte]inquiryNameAndFunc {
		0x00: {"Supported VPD Pages",decodeInquiryPage00},
		0x80: {"Unit Serial Number",decodeInquiryPage80},
		0x83: {"Device Identification",decodeInquiryPage83},
		0x86: {"Extended INQUIRY Data",decodeInquiryPage86},
		0x87: {"Mode Page Policy",decodeInquiryPage87},
		0x88: {"SCSI Ports",decodeInquiryPage88},
		0x89: {"ASCII Information",decodeInquiryPage89},
		0x8a: {"Power Condition",decodeInquiryPage8a},
		0x8d: {"Power Consumption",decodeInquriyPage8d},
		0x90: {"Protocol Specific Logical Unit Information",decodeInquiryPage90},
		0x91: {"Protocol Specific Port Information",decodeInquiryPage91},
		0xb0: {"Block Limits",decodeInquiryPageb0},
		0xb1: {"Block Device Characteristics", decodeInquiryPageb1},
		0xb2: {"Logical Block Provisioning", nil},
		0xb7: {"Unknown page", nil},
		0xcd: {"Unknown page", nil},
	}

	designatorType = map[byte]string {
		0x0: "Vendor Specific",
		0x1: "T10 vendor ID",
		0x2: "EUI-64",
		0x3: "NAA",
		0x4: "Relative target port",
		0x5: "Target port group",
		0x6: "Logical unit group",
		0x7: "MD5 logical unit",
		0x8: "SCSI name",
		0x9: "Protocol specific port",
	}

	NAAField = map[byte]string {
		0x2: "IEEE Extended",
		0x3: "Locally Assigned",
		0x5: "IEEE Registered",
		0x6: "IEEE Registered Extened",
	}

	codeSet = map[byte]string {
		0x0: "Reserved",
		0x1: "Binary",
		0x2: "ASCII printable",
		0x3: "UTF-8 codes",
	}

	deviceType = map[byte]string {
		0x00: "SBC-3 Direct access",
		0x01: "SSC-4 Sequential access",
		0x02: "SSC Printer",
		0x03: "SPC-2 Processor device",
		0x04: "SBC Write-once device",
		0x05: "MMC-6 CD/DVD",
		0x07: "SBC Optical memory",
		0x08: "SMC-3 Media changer",
		0x0c: "SCC-2 Storage array controller",
		0x0d: "SES-2 Enclosure services",
		0x0e: "RBC Simplified direct-access",
		0x0f: "OCRW Optical card reader/writer",
		0x11: "OSD-2 Object-based Storage",
		0x12: "ADC-3 Automation/Driver Interface",
	}

	ataCommandCode = map[byte]string {
		0xec: "IDENTIFY DEVICE",
		0xa1: "IDENTIFY PACKET DEVICE",
		0x00: "Other device types",
	}

	modePagePolicy = map[byte]string {
		0x00: "Shared",
		0x01: "Per target port",
		0x02: "Obsolete",
		0x03: "Per I_T nexus",
	}

	powerConsumptionUnits = map[byte]string {
		0x00: "Gigawatts",
		0x01: "Megawatts",
		0x02: "Kilowatts",
		0x03: "Watts",
		0x04: "Milliwatts",
		0x05: "Microwatts",
	}

	blockLimitsBytes = []multiByteDump {
		{5,1,"Maxiumum compare and write length"},
		{6,2,"Optimal transfer length granularity"},
		{8,4,"Maxiumum transfer length"},
		{12,4,"Optimal transfer length"},
		{16,4,"Maximum prefetch length"},
		{20,4,"Maximum unmap LBA count"},
		{24,4,"Maximum unmap block descriptor count"},
		{28,4,"Optimal unmap granularity"},
		{36,8,"Maximum write same langth"},
	}
}

func diskinfoInquiry(d *diskInfoData) {
	if data, _, err := scsiInquiry(d.fp, 0, 0); err == nil {
		bp := bytes.NewBuffer(data[8:16])
		d.vendor = bp.String()
		bp = bytes.NewBuffer(data[16:32])
		d.productID = bp.String()
	} else {
		fmt.Printf("inquiry error: %s\n", err)
	}

	if data, _, err := scsiInquiry(d.fp, 1, 0xb1); err == nil {
		converter := dataToInt{data, 4,2}
		if converter.getInt() == 1 {
			d.isSSD = true
		}
	}
}

func scsiInquiryCommand(fp *os.File) {

	evpd := byte(0)
	if inquiryEVPD || pageRequest != 0 || showAll {
		evpd = 1
	}

	if data, length, err := scsiInquiry(fp, evpd, byte(pageRequest)); err == nil {
		if debugOutput {
			fmt.Printf("DataIn:\n")
			for offset := 0; offset < length; offset += 16 {
				curLen := min(16, length-offset)
				dumpLine(data[offset:], curLen, int64(offset), 4)
			}
		}

		if showAll {
			for index := 4; index < length; index++ {
				if pageData, pageLength, pageErr := scsiInquiry(fp, evpd, data[byte(index)]); pageErr == nil {
					if naf, ok := pageCodeFuncs[data[byte(index)]]; ok {
						fmt.Printf("Page 0x%x: %s\n", data[byte(index)], naf.name)
						if naf.decode != nil {
							naf.decode(pageData, pageLength)
						}
						fmt.Printf("\n")
					} else {
						fmt.Printf("Failed to find decode function for page 0x%x\n", data[byte(index)])
					}
				}
			}
		} else {
			if evpd == 1 {
				page := byte(pageRequest)
				if naf, ok := pageCodeFuncs[page]; ok {
					fmt.Printf("%s\n", naf.name)
					if naf.decode != nil {
						naf.decode(data, length)
					}
				} else {
					fmt.Printf("Failed to find decode function for page 0x%x\n", page)
				}
			} else {
				decodeStandInquiry(data, length)
			}
		}

	} else {
		fmt.Printf("uscsi failed: %s\n", err)
	}
}

func scsiInquiry(fp *os.File, evpd byte, pageCode byte) ([]byte, int, error) {
	var cdb []byte
	var data []byte

	cdb = make([]byte, 6)
	data = make([]byte, 256)

	cdb[0] = 0x12
	cdb[1] = evpd
	cdb[2] = pageCode
	cdb[4] = 0xff
	if debugOutput {
		fmt.Printf("CDB:\n")
		dumpLine(cdb, len(cdb), 0, 2)
	}

	dataLen, err := sendUSCSI(fp, cdb, data, 0)

	return data, dataLen, err
}

func decodeStandInquiry(data []byte, dataLen int) {
	fmt.Printf("Standard INQUIRY Data\n")
	fmt.Printf("  Device type     : %s\n", deviceType[data[0] & 0x1f])
	fmt.Printf("  T10 Vendor ID   : ")
	for i := 8; i < 16; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n  Product ID      : ")
	for i := 16; i < 32; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n  Product Revision: ")
	for i := 32; i < 36; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n    ")
	outputLen := 4
	for _, bits := range standardInquiryBits {
		str := fmt.Sprintf("%s=%d", bits.name, data[bits.byteOffset] >> bits.rightShift & bits.mask)
		if outputLen + len(str) + 1 >= 80 {
			fmt.Printf("\n    ")
			outputLen = 4
		}
		outputLen += len(str) + 1
		fmt.Printf("%s ", str)
	}
	fmt.Printf("\n  Remaining %d bytes of INQUIRY data\n", data[4])
	dumpMemory(data[36:], dataLen - 36, "    ")
}

func decodeInquiryPage00(data []byte, dataLen int) {
	var supportedPage string
	var naf inquiryNameAndFunc
	var ok bool

	longestTitle := 0
	for index := 4; index < dataLen; index++ {
		if naf, ok := pageCodeFuncs[data[index]]; ok {
			longestTitle = max(longestTitle, len(naf.name))
		}
	}
	supportedTitle := "Supported"
	fmt.Printf("    %4s | %-*s | %s\n", "Page", longestTitle, "Title", supportedTitle)
	fmt.Printf("  %s\n", support.DashLine(6, longestTitle + 2, len(supportedTitle) + 2))
	for index := 4; index < dataLen; index += 1 {
		if naf, ok = pageCodeFuncs[data[index]]; ok {
			if naf.decode == nil {
				supportedPage = "(not yet)"
			} else {
				supportedPage = ""
			}
		}
		fmt.Printf("  |  %02x  | %-*s | %s\n", data[index], longestTitle, naf.name, supportedPage)
	}
}

func decodeInquiryPage80(data []byte, dataLen int) {
	fmt.Printf("  Device type  : %s\n", deviceType[data[0] & 0x1f])
	fmt.Printf("  Serial number: ")
	for i := 4; i < dataLen; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n")
}

//noinspection GoUnusedParameter
func decodeInquiryPage83(data []byte, unused int) {
	count := 1
	for offset := 4; offset < int(data[2] << 8 | data[3]); count++ {
		fmt.Printf("  ---- Destriptor %d ----\n", count)
		offset += designationDescDecode(data[offset:])
	}
}

func designationDescDecode(data []byte) int {
	// Check to see if PIV bit is set.

	if data[1] & 0x80 != 0 {

		// With PIV set now check to see
		if data[1] & 0x30 == 0x10 {
			fmt.Printf("  Target Port Designator\n")
		} else if data[1] & 0x30 == 0x20 {
			fmt.Printf("  SCSI Target Device Designator\n")
		} else {
			fmt.Printf("  Addressed Logic Unit Designator\n")
		}
		fmt.Printf("  Protocol Indentifer: %s\n", protocolIndentifier[data[0] >> 4])
	}
	fmt.Printf("  Designator Type: %s, code_set: %s\n", designatorType[data[1] & 0xf], codeSet[data[0] & 0xf])
	switch data[1] & 0xf {
	case 0, 2, 5, 6, 7, 9:
		fmt.Printf("    ---- Not Decoded yet (%d) ----\n", data[1] & 0xf)

	case 1:
		converter := dataToInt{data, 4, 8}
		val := converter.getInt64()
		fmt.Printf("  T10 Vendor ID: 0x%x\n", val)

		converter.setOffsetCount(8, int(data[3] + 4))
		val = converter.getInt64()
		fmt.Printf("  IEEE Company ID: 0x%x\n", val)

	case 3:
		fmt.Printf("  %s\n    ", NAAField[data[4] >> 4])
		converter := dataToInt{data, 4, 8}
		val := converter.getInt64()
		fmt.Printf("[0x%x]\n", val)

	case 4:
		fmt.Printf("  Port ID: 0x%x\n", data[6] << 8 | data[7])

	case 8:
		fmt.Printf("  SCSI Name:\n    ")
		for i := 4; i < int(data[3] + 4); i++ {
			fmt.Printf("%c", data[i])
		}
		fmt.Printf("\n")

	default:
		fmt.Printf("  ---- Unexpected and impossible: %d\n", data[1] & 0xf)
	}

	return int(data[3]) + 4
}

//noinspection GoUnusedParameter
func decodeInquiryPage86(data []byte, unused int) {
	lineLength := 2
	fmt.Printf("  ")
	for _, bits := range extendInquiry {
		str := fmt.Sprintf("%s=%d", bits.name, (data[bits.byteOffset] >> bits.rightShift) & bits.mask)
		if lineLength + len(str) + 1 >= 80 {
			fmt.Printf("\n  ")
			lineLength = 2
		}
		fmt.Printf("%s ", str)
		lineLength += len(str) + 1
	}
	fmt.Printf("\n  Self-test completion minutes=%d\n", data[10] << 8 | data[11])
}

func decodeInquiryPage87(data []byte, dataLen int) {
	for i := 4; i < dataLen; i += 4 {
		fmt.Printf("  Policy page code: 0x%x, subpage code: 0x%x\n", data[i], data[i + 1])
		fmt.Printf("    MLUS=%d, Policy: %s\n", data[i + 2] >> 7, modePagePolicy[data[i + 2] & 0x3])
	}
}

func decodeInquiryPage88(data []byte, dataLen int) {
	fmt.Printf("  Page Length: %d\n", int(data[2]) << 8 | int(data[3]))
	for offset := 4; offset < dataLen; {
		offset += decodeSCSIPort(data[offset:])
	}
}

func decodeSCSIPort(data []byte) int {
	fmt.Printf("  Relative Port Identifer: 0x%x\n", int(data[2]) << 8 | int(data[3]))

	initiatorLength := int(data[6]) << 8 | int(data[7])
	fmt.Printf("  Initiator Port ID length: %d\n", initiatorLength)
	dumpMemory(data[8:], initiatorLength, "    ")

	targetPortLength := int(data[8 + initiatorLength + 2]) << 8 | int(data[8 + initiatorLength + 3])
	fmt.Printf("  Target Port descriptors length: %d\n", targetPortLength)

	targetPortData := data[8 + initiatorLength + 4:]
	for targetOffset := 0; targetOffset < targetPortLength; {
		targetOffset = decodeTargetPort(targetPortData, targetOffset)
	}

	return 8 + initiatorLength + 4 + targetPortLength
}

func decodeTargetPort(data []byte, offset int) int {
	targetPortLength := int(data[3])
	fmt.Printf("    Protocol: %s\n", protocolIndentifier[data[0] >> 4])
	fmt.Printf("    Code set: %s\n", codeSet[data[0] & 0xf])
	fmt.Printf("    Designator type: %s\n", designatorType[data[1] & 0xf])
	fmt.Printf("    [")
	for i := 0; i < targetPortLength; i++ {
		fmt.Printf("%02x", data[i + 4])
	}
	fmt.Printf("]\n")

	return offset + targetPortLength + 4
}

func decodeInquiryPage89(data []byte, dataLen int) {
	fmt.Printf("  Device type         : %s\n", deviceType[data[0] & 0x1f])
	fmt.Printf("  SAT Vendor ID       : ")
	for i := 8; i < 16; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n  SAT Product ID      : ")
	for i := 16; i < 32; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n  SAT Product revision: ")
	for i := 32; i < 36; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n  ATA Device signature:\n    ")
	dumpMemory(data[36:], 20, "    ")
	fmt.Printf("\n  ATA command         : %s\n", ataCommandCode[data[56]])
	fmt.Printf("  Serial number       : ")
	for i := 10; i <= 19; i++ {
		fmt.Printf("%c%c", data[60 + (i * 2) + 1], data[60 + (i * 2)])
	}
	fmt.Printf("\n  Firmware version    : ")
	for i := 23; i <= 26; i++ {
		fmt.Printf("%c%c", data[60 + (i * 2) + 1], data[60 + (i * 2)])
	}
	fmt.Printf("\n  Model number        : ")
	for i := 27; i <= 46; i++ {
		fmt.Printf("%c%c", data[60 + (i * 2) + 1], data[60 + (i * 2)])
	}
	fmt.Printf("\n  Raw ATA IDENTIFY DATA\n")
	dumpMemory(data[60:], dataLen- 60, "    ")
}

//noinspection GoUnusedParameter
func decodeInquiryPage8a(data []byte, unused int) {
	outputLength := 2
	fmt.Printf("  ")
	for _, bits := range powerConditionBits {
		str := fmt.Sprintf("%s=%d", bits.name, (data[bits.byteOffset] >> bits.rightShift) & bits.mask)
		if outputLength + len(str) + 1 >= 80 {
			fmt.Printf("\n  ")
			outputLength = 2
		}
		outputLength += len(str) + 1
		fmt.Printf("%s ", str)
	}
	fmt.Printf("\n")
	for _, pcBytes := range powerConditionBytes {
		converter := dataToInt{data, pcBytes.byteOffset, pcBytes.numberBytes}
		val := converter.getInt64()
		fmt.Printf("  %s: %d\n", pcBytes.name, val)
	}
}

func decodeInquriyPage8d(data []byte, dataLen int) {
	for offset := 4; offset < dataLen; offset += 4 {
		fmt.Printf("  Power consumption ID: %d is %d in %s\n", data[offset],
			int(data[offset + 2]) << 8 | int(data[offset + 3]), powerConsumptionUnits[data[offset + 1] & 0x7])
	}
}

func decodeInquiryPage90(data []byte, dataLen int) {
	for offset := 4; offset < dataLen; {
		offset = decodeLogicalUnit(data[offset:], offset)
	}
}

func decodeLogicalUnit(data []byte, offset int) int {
	fmt.Printf("  Relative port identifier: %d\n", int(data[0]) << 8 | int(data[1]))
	fmt.Printf("  Protocol: %s\n", protocolIndentifier[data[2] & 0xf])
	protocolLen := int(data[6]) << 8 | int(data[7])
	dumpMemory(data[8:], protocolLen, "    ")

	return offset + protocolLen + 8
}

func decodeInquiryPage91(data []byte, dataLen int) {
	for offset := 4; offset < dataLen; {
		offset = decodeProtocolSpecificPort(data[offset:], offset)
	}
}

func decodeProtocolSpecificPort(data []byte, offset int) int {
	fmt.Printf("  Relative port identifier: %d\n", int(data[0]) << 8 | int(data[1]))
	fmt.Printf("  Protocol: %s\n", protocolIndentifier[data[2] & 0xf])
	portLen := int(data[6]) << 8 | int(data[7])
	dumpMemory(data[8:], portLen, "    ")
	return offset + portLen + 8
}

func decodeInquiryPageb0(data []byte, dataLen int) {
	converter := dataToInt{data, 2, 2}
	if dataLen != converter.getInt() + 4 {
		fmt.Printf("  Invalid data count returned (%d) verses page length (%d)\n",
			dataLen - 4, converter.getInt())
	}
	for _, blb := range blockLimitsBytes {
		converter.setOffsetCount(blb.byteOffset,blb.numberBytes)
		fmt.Printf("  %s: %d\n", blb.name, converter.getInt64())
	}
	converter.setOffsetCount(32,4)
	if converter.getInt() & 0x80000000 != 0 {
		fmt.Printf("  Unmap grandularity alignment: %d\n", converter.getInt() & 0x7fffffff)
	}
}

var pageB1ProductType = map[byte]string {
	0x00: "Not indicated",
	0x01: "CFast",
	0x02: "Compact Flash",
	0x03: "Memory stick",
	0x04: "MultiMedia Card",
	0x05: "Secure digital card",
	0x06: "XQD",
	0x07: "Universal flash storage",
}

var pageB1Bits = []bitMaskBitDump {
	{7,6, 3,"WABEREQ"},
	{7,4, 3,"WACEREQ"},
	{8,1, 1,"FUAB"},
	{8,0,1,"VBULS"},
}

var pageB1NominalType = map[byte]string {
	0x00: "Not reported",
	0x01: "5.25 inch",
	0x02: "3.5 inch",
	0x03: "2.5 inch",
	0x04: "1.8 inch",
	0x05: "Less than 1.8 inch",
}

func decodeInquiryPageb1(data []byte, dataLen int) {
	fmt.Printf("  Rotation rate: ")
	converter := dataToInt{data, 4,2}
	rotationRate := converter.getInt()
	if rotationRate == 0 {
		fmt.Printf("Not reported\n")
	} else if rotationRate == 1 {
		fmt.Printf("Solid State\n")
	} else {
		fmt.Printf("%d RPM\n")
	}
	fmt.Printf("  Product type: %s\n", pageB1ProductType[data[6]])
	fmt.Printf("  Nominal Form Factor: %s\n", pageB1NominalType[data[7] & 0xf])
	outputCols := 4
	fmt.Printf("    ")
	for _, pb := range pageB1Bits {
		str := fmt.Sprintf("%s=%d ", pb.name, data[pb.byteOffset] >> pb.rightShift & pb.mask)
		if outputCols + len(str) >= 80 {
			fmt.Printf("\n    ")
			outputCols = 4
		}
		outputCols += len(str)
		fmt.Printf("%s", str)
	}
	fmt.Printf("\n")
}