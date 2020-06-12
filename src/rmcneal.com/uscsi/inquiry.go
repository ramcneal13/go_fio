package main

import (
	"os"
	"fmt"
	"strings"
)

type bitMaskPage86 struct {
	byteOffset	byte
	rightShift	uint8
	mask		byte
	name		string
}

var pageCodeStrings map[byte]string
var pageCodeFuncs map[byte]func([]byte, int)
var protocolIndentifier map[byte]string
var designatorType map[byte]string
var NAAField map[byte]string
var codeSet map[byte]string
var deviceType map[byte]string
var ataCommandCode map[byte]string
var extendInquiry []bitMaskPage86

func init() {
	extendInquiry = []bitMaskPage86{}
	extendInquiry = append(extendInquiry, bitMaskPage86{0, 0, 0x1f, "device type"})
	extendInquiry = append(extendInquiry, bitMaskPage86{4, 6, 0x3, "Active Microcode"})
	extendInquiry = append(extendInquiry, bitMaskPage86{4, 3, 0x7, "SPT"})
	extendInquiry = append(extendInquiry, bitMaskPage86{4, 2, 0x1, "GRD_CHK"})
	extendInquiry = append(extendInquiry, bitMaskPage86{4, 1, 0x1, "APP_CHK"})
	extendInquiry = append(extendInquiry, bitMaskPage86{4, 0, 0x1, "REF_CHK"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 5, 0x1, "UASK_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 4, 0x1, "GROUP_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 3,  0x1, "PRIOR_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 2, 0x1, "HEADSUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 1, 1, "ORDSUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{5, 0, 1, "SIMPSUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{6, 3, 1, "WU_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{6, 2, 1, "CRD_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{6, 1, 1, "NV_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{6, 0, 1, "V_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{7, 4, 1, "P_I_I_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{7, 0, 1, "LUICLR"})
	extendInquiry = append(extendInquiry, bitMaskPage86{8, 4, 1, "R_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{8, 0, 1, "CBCS"})
	extendInquiry = append(extendInquiry, bitMaskPage86{9, 0, 0xf,
	"MULTI I_T MICROCODE DOWNLOAD"})
	extendInquiry = append(extendInquiry, bitMaskPage86{12, 7, 1, "POA_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{12, 6, 1, "HRA_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{12, 5, 1, "VSA_SUP"})
	extendInquiry = append(extendInquiry, bitMaskPage86{13, 0, 0xff,
	"Maximum supported sense data length"})

	pageCodeStrings = map[byte]string{
		0x89: "ASCII Information",
		0x8c: "CFA Profile Information",
		0x8b: "Device Constituents",
		0x83: "Device Identification",
		0x86: "Extended INQUIRY Data",
		0x85: "Management Network Addresses",
		0x87: "Mode Page Policy",
		0x8a: "Power Condition",
		0x8d: "Power Consumption",
		0x90: "Protocol Specific Logical Unit Information",
		0x91: "Protocol Specific Port Information",
		0x88: "SCSI Ports",
		0x84: "Software Interface Identification",
		0x00: "Supported VPD Pages",
		0x8f: "Third Part Copy",
		0x80: "Unit Serial Number",
	}

	pageCodeFuncs = map[byte]func([]byte, int) {
		0x80: decodePage80,
		0x83: decodePage83,
		0x86: decodePage86,
		0x89: decodePage89,
	}

	protocolIndentifier = map[byte]string {
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
}

func scsiInquiryCommand(fp *os.File) {

	evpd := byte(0)
	if inquiryEVPD || inquiryPage != 0 {
		evpd = 1
	}

	if data, length, err := scsiInquiry(fp, evpd, byte(inquiryPage)); err == nil {
		if debugOutput {
			fmt.Printf("DataIn:\n")
			for offset := 0; offset < length; offset += 16 {
				curLen := min(16, length-offset)
				dumpLine(data[offset:], curLen, int64(offset), 4)
			}
		}

		if evpd == 1 {
			decodePageData(byte(inquiryPage), data, length)
		} else {
			decodeStandInquiry(data, length)
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

	len, err := sendUSCSI(fp, cdb, data, 0)

	return data, len, err
}

func decodePageData(page byte, data []byte, len int) {
	switch page {
	case 0:
		fmt.Printf("%4s | %5s\n", "Page", "Title")
		fmt.Printf("-----+--------------------------\n")
		for index := 4; index < len; index += 1 {
			if cmdTitle, ok := pageCodeStrings[data[index]]; ok {
				fmt.Printf("  %02x | %s\n", data[index], cmdTitle)
			}
		}
	default:
		if dataDecoder, ok := pageCodeFuncs[page]; ok {
			fmt.Printf("%s\n", pageCodeStrings[page])
			dataDecoder(data, len)
		} else {
			fmt.Printf("Failed to find decode function for page 0x%x\n", page)
		}
	}
}

func decodePage80(data []byte, len int) {
	fmt.Printf("  Device type  : %s\n", deviceType[data[0] & 0x1f])
	fmt.Printf("  Serial number: ")
	for i := 4; i < len; i++ {
		fmt.Printf("%c", data[i])
	}
	fmt.Printf("\n")
}

func decodePage83(data []byte, len int) {
	count := 1
	for offset := 4; offset < int(data[2] << 8 | data[3]); count++ {
		fmt.Printf("  ---- Destriptor %d ----\n", count)
		offset = designationDescDecode(data[offset:], offset)
	}
}

func decodePage86(data []byte, len int) {
	lineLength := 2
	fmt.Printf("  ")
	for _, bits := range extendInquiry {
		str := fmt.Sprintf("%s=%d", bits.name, (data[bits.byteOffset] >> bits.rightShift) & bits.mask)
		nr := strings.NewReader(str)
		if lineLength + nr.Len() + 1 >= 80 {
			fmt.Printf("\n  ")
			lineLength = 2
		}
		fmt.Printf("%s ", str)
		lineLength = lineLength + nr.Len() + 1
	}
	fmt.Printf("\n  Self-test completion minutes=%d\n", data[10] << 8 | data[11])
}

func decodePage89(data []byte, len int) {
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
	dumpMemory(data[60:], len - 60, "    ")
}

func designationDescDecode(data []byte, offset int) int {
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
		val := int64(0)
		for i := 4; i < 12; i++ {
			val = (val << 8) | int64(data[i])
		}
		fmt.Printf("  T10 Vendor ID: 0x%x\n", val)

		val = 0
		for i := 8; i < int(data[3] + 3); i++ {
			val = (val << 8) | int64(data[i])
		}
		fmt.Printf("  IEEE Company ID: 0x%x\n", val)

	case 3:
		fmt.Printf("  %s\n    ", NAAField[data[4] >> 4])
		val := int64(0)
		for i := 4; i < 12; i++ {
			val = (val << 8) | int64(data[i])
		}
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

	return offset + int(data[3]) + 4
}

func decodeStandInquiry(data []byte, len int) {
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
	fmt.Printf("\n")
}
