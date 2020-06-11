package main

import (
	"os"
	"fmt"
)

var pageCodeStrings map[byte]string
var pageCodeFuncs map[byte]func([]byte, int)
var protocolIndentifier map[byte]string
var designatorType map[byte]string
var codeSet map[byte]string

func init() {
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
		0x83: decodePage83,
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
		0x1: "T10 vednor ID",
		0x2: "EUI-64",
		0x3: "NAA",
		0x4: "Relative target port",
		0x5: "Target port group",
		0x6: "Logical unit group",
		0x7: "MD5 logical unit",
		0x8: "SCSI name",
		0x9: "Protocol specific port",
	}

	codeSet = map[byte]string {
		0x0: "Reserved",
		0x1: "Binary",
		0x2: "ASCII printable",
		0x3: "UTF-8 codes",
	}
}

func scsiInquiryCommand(fp *os.File) {

	evpd := byte(0)
	if inquiryEVPD || inquiryPage != 0 {
		evpd = 1
	}

	if data, len, err := scsiInquiry(fp, evpd, byte(inquiryPage)); err == nil {
		if debugOutput {
			fmt.Printf("DataIn:\n")
			for offset := 0; offset < len; offset += 16 {
				curLen := min(16, len-offset)
				dumpLine(data[offset:], curLen, int64(offset))
			}
		}

		if evpd == 1 {
			decodePageData(byte(inquiryPage), data, len)
		} else {
			decodeStandInquiry(data, len)
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
		dumpLine(cdb, len(cdb), 0)
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

func decodePage83(data []byte, len int) {
	count := 1
	fmt.Printf("Qualifier: 0x%x, Device Type: 0x%x\n", data[0] >> 4, data[0] & 0xf)
	for offset := 4; offset < int(data[2] << 8 | data[3]); count++ {
		fmt.Printf("  ---- Destriptor %d ----\n", count)
		offset = designationDescDecode(data[offset:], offset)
	}
}

func designationDescDecode(data []byte, offset int) int {
	// Check to see if PIV bit is set.

	codeSetVal := data[0] & 0x7
	fmt.Printf("  Code Set: %s\n", codeSet[codeSetVal])
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
	fmt.Printf("  Designator Type: %s\n", designatorType[data[1] & 0x7])
	return offset + int(data[3]) + 3
}

func decodeStandInquiry(data []byte, len int) {

}
