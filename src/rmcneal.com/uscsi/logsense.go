package main

import (
	"os"
	"fmt"
	"rmcneal.com/support"
	"bytes"
)

type startStopDecode struct {
	name	string
	process	func([]byte) string
}

type statisticsFuncs struct {
	name	string
	process	func([]byte) int
}

type tableArrayT map[byte]string

var logsenseCodeFuncs map[byte]logsenseNameAndFunc
var logTemperatureStrings map[int]string
var logParameterBits []bitMaskBitDump
var overUnderParameterCode map[byte]string
var rwErrorParameterCode map[byte]string
var startStopFuncs map[byte]startStopDecode
var generalStatsBytes []multiByteDump
var page19StatFuncs map[int]statisticsFuncs
var nonMediumErrorCountCode map[byte]string
var tableArray []tableArrayT

type logsenseNameAndFunc struct {
	name	string
	decode	func(*os.File, []byte, int)
}

func init() {
	logsenseCodeFuncs = map[byte]logsenseNameAndFunc {
		0x00: {"Supported Log Pages", decodeLogPage00},
		0x01: {"Buffer Over-Run/Under-Run", decodeLogPageCommon},
		0x02: {"Write Error Counters", decodeLogPageCommon},
		0x03: {"Read Error Counters", decodeLogPageCommon},
		0x04: {"Read Reverse Error Counters", decodeLogPageCommon},
		0x05: {"Verify Error Counters", decodeLogPageCommon},
		0x06: {"Non-Medium Error", decodeLogPageCommon},
		0x0d: {"Temperature",decodeLogPage0d},
		0x0e: {"Start-Stop Cycle Counter",decodeLogPage0e},
		0x0f: {"Application client",decodeLogPage0f},
		0x10: {"Self-Test Results",decodeLogPage10},
		0x11: {"Solid State Media",decodeLogPage11},
		0x12: {"(restricted)", showAsRestricted},
		0x13: {"(restricted)",showAsRestricted},
		0x14: {"(restricted)",showAsRestricted},
		0x15: {"Background Scan Results",decodeLogPage15},
		0x16: {"ATA PASS-THROUGH",showAsRestricted},
		0x17: {"(restricted)",showAsRestricted},
		0x18: {"Protocol Specific Port",decodeLogPage18},
		0x19: {"General statistics",decodeLogPage19},
		0x1a: {"Power Condition Transitions",decodeLogPage1a},
		0x2f: {"Informal Exceptions",decodeLogPage2f},
	}
	logTemperatureStrings = map[int]string {
		0x0000: "Temperature",
		0x0001: "Reference Temperature",
	}
	logParameterBits = []bitMaskBitDump {
		{2,7,1,"DU"},
		{2,5,1,"TSD"},
		{2,4,1,"ETC"},
		{2,2,3,"TMC"},
		{2,0,3,"Format_and_linking"},
	}
	overUnderParameterCode = map[byte]string {
		0x00: "under-run (undefined)",
		0x01: "over-run (undefined)",
		0x20: "Command under-run",
		0x21: "Command over-run",
		0x40: "I_T Nexus under-run",
		0x41: "I_T Nexus over-run",
		0x80: "Unit of time under-run",
		0x81: "Unit of time over-run",
		0x02: "under-run (undefined) service delivery",
		0x03: "over-run (undefined) service delivery",
		0x22: "Command under-run; service delivery",
		0x23: "Command over-run; service delivery",
		0x42: "I_T Nexus under-run; service delivery",
		0x43: "I_T Nexus over-run; service delivery",
		0x82: "Unit of time under-run; service delivery",
		0x83: "Unit of time over-run; service delivery",
	}
	rwErrorParameterCode = map[byte]string {
		0x00: "Errors corrected w/o delay",
		0x01: "Errors corrected with delay",
		0x02: "Total rewrites or rereads",
		0x03: "Total errors corrected",
		0x04: "Total correction algorithm processed",
		0x05: "Total bytes processed",
		0x06: "Total uncorrected errors",
	}
	nonMediumErrorCountCode = map[byte]string {
		0x00: "Non-Medium Error Count",
	}
	startStopFuncs = map[byte]startStopDecode{
		0x01: {"Date of Manufacture", domDecode},
		0x02: {"Accounting Date",domDecode}, // Uses the same format as Date-of-Manufacture
		0x03: {"Specified Cycle Count Over Device Lifetime", commonStartStopDataDecode},
		0x04: {"Accumulated Start-Stop Cycles", commonStartStopDataDecode},
		0x05: {"Specified Load-Unload Count Over Lifetime", commonStartStopDataDecode},
		0x06: {"Accumulated Load-Unload Cycles", commonStartStopDataDecode},
	}
	generalStatsBytes = []multiByteDump {
		{4,8,"Read commands"},
		{12,8,"Write commands"},
		{20,8,"Logical blocks received"},
		{28,8,"Logical blocks transmitted"},
		{36,8,"Read command processing intervals"},
		{44,8,"Write command processing intervals"},
		{52,8,"Weighted number of read commands plus write"},
		{60,8,"Weighted read command processing plus write"},
	}
	page19StatFuncs = map[int]statisticsFuncs {
		0x01: {"General Access Statistics and Performance", statsPage1 },
		0x02: {"Idle Time",statsPage2 },
		0x03: {"Time Interval", statsPage3 },
		0x04: {"Force Unit Access Statistics and Performance", statsPage4 },
	}
	tableArray = []tableArrayT {
		overUnderParameterCode,
		rwErrorParameterCode,
		rwErrorParameterCode,
		rwErrorParameterCode,
		rwErrorParameterCode,
		rwErrorParameterCode,
		nonMediumErrorCountCode,
	}
}

func diskinfoLogSense(d *diskInfoData) {
	if data, length, err := scsiLogSense(d.fp, 0, 0); err == nil {
		for index := 4; index < length; index++ {
			if data[index] == 0x11 {
				d.isSSD = true
				if data, _, err := scsiLogSense(d.fp, 0x11, 0); err == nil {
					d.wearValue = int(data[7])
				}
			}
		}
	}
}

func scsiLogSenseCommand(fp *os.File) {

	if data, length, err := scsiLogSense(fp, byte(pageRequest), 0); err == nil {
		if debugOutput {
			fmt.Printf("DataIn:\n")
			for offset := 0; offset < length; offset += 16 {
				curLen := min(16, length-offset)
				dumpLine(data[offset:], curLen, int64(offset), 4)
			}
		}

		if showAll {
			for index := 4; index < length; index++ {
				if pageData, pageLength, pageError := scsiLogSense(fp, data[byte(index)], 0); pageError == nil {
					if naf, ok := logsenseCodeFuncs[data[byte(index)]]; ok {
						fmt.Printf("Page 0x%x: %s\n", data[byte(index)], naf.name)
						naf.decode(fp, pageData, pageLength)
						fmt.Printf("\n")
					} else {
						fmt.Printf("Page 0x%x not supported (yet)\n", data[byte(index)])
					}
				}
			}
		} else {
			page := byte(pageRequest)
			if naf, ok := logsenseCodeFuncs[page]; ok {
				fmt.Printf("%s\n", naf.name)
				naf.decode(fp, data, length)
			} else {
				fmt.Printf("Failed to find decode function for page 0x%x\n", page)
			}
		}

	} else {
		fmt.Printf("uscsi failed: %s\n", err)
	}

}

func scsiLogSense(fp *os.File, page byte, subpage byte) ([]byte, int, error) {
	cdb := make([]byte, 10)
	data := make([]byte, 65536)

	cdb[0] = 0x4d
	cdb[1] = 0
	cdb[2] = 0x40 | page // Only cumulative values are valid
	cdb[3] = subpage
	cdb[7] = 0xff
	cdb[8] = 0xff
	if debugOutput {
		fmt.Printf("CDB:\n")
		dumpMemory(cdb, len(cdb), "  ")
	}

	dataLen, err := sendUSCSI(fp, cdb, data, 0)

	return data, dataLen, err
}

//noinspection GoUnusedParameter
func showAsRestricted(fp *os.File, data []byte, dataLen int) {
	fmt.Printf("    Restricted (see applicable protocol standard)\n")
}

func decodeLogPage00(fp *os.File, data []byte, dataLen int) {
	var name string

	longest := 0
	for index := 4; index < dataLen; index++ {
		str := fmt.Sprintf("%s", logsenseCodeFuncs[data[index]].name)
		longest = max(len(str), longest)
	}
	longest += 1
	fmt.Printf("    Num    %-*s  Sub Pages\n", longest, "Name")
	fmt.Printf("  %s\n", support.DashLine(6, longest + 1, 19))
	for index := 4; index < dataLen; index++ {
		if naf, ok := logsenseCodeFuncs[data[index]]; ok {
			name = naf.name
		} else {
			name = "Unsupported"
		}
		fmt.Printf("  |  %02x  | %-*s", data[index], longest, name)
		if data[index] == 0 {
			fmt.Printf("|\n")
			continue
		}
		fmt.Printf("|")
		if subdata, sublen, err := scsiLogSense(fp, data[index], 0xff); err == nil {
			for subIndex := 4; subIndex < sublen; subIndex += 2 {
				if subdata[subIndex + 1] == 0xff {
					// Already know that 0xff is supported for this log sense page
					// since a) it's required and b) that's the page/subpage just returned.
					continue
				}
				fmt.Printf(" %02d", subdata[subIndex + 1])
			}
		}
		fmt.Printf("\n")
	}
}

func dumpParameterBits(data []byte) {
	outputLen := 4
	fmt.Printf("    ")
	for _, bits := range logParameterBits {
		str := fmt.Sprintf("%s=%d ", bits.name,
			data[bits.byteOffset] >> bits.rightShift & bits.mask)
		if outputLen + len(str) >= 80 {
			fmt.Printf("\n")
			outputLen = 4
		}
		fmt.Printf("%s", str)
		outputLen += len(str)
	}
	fmt.Printf("\n")

}

//noinspection GoUnusedParameter
func decodeLogPageCommon(fp *os.File, data []byte, dataLen int) {
	pageCode := data[0] & 0x3f
	table := tableArray[pageCode]
	decodeParameterLoop(data, dataLen, table)
}

func decodeParameterLoop(data []byte, dataLen int, table map[byte]string) {
	longestName := 0
	for _, name := range table {
		longestName = max(longestName, len(name))
	}
	fmt.Printf("    %-*s | Value\n  %s\n", longestName, "Parameter Name", support.DashLine(longestName + 2, 8))
	for offset := 4; offset < dataLen; {
		offset += decodeParameterCode(data[offset:], table, longestName)
	}
}

func decodeParameterCode(data []byte, table map[byte]string, longestName int) int {
	converter := dataToInt{data, 4, int(data[3])}
	val := converter.getInt()
	fmt.Printf("  | %-*s | %d", longestName, table[data[1]], val)
	if val > 0x10000 {
		fmt.Printf(" (%s)", support.Humanize(int64(val), 1))
	}
	fmt.Printf("\n")

	return int(data[3]) + 4
}

//noinspection GoUnusedParameter
func decodeLogPage0d(fp *os.File, data []byte, dataLen int) {
	for index := 4; index < dataLen; index += 6 {
		code := int(data[index]) << 8 | int(data[index + 1])
		fmt.Printf("  %s: %d\n    ", logTemperatureStrings[code], data[index + 5])
		dumpParameterBits(data[index:])
	}
}

//noinspection GoUnusedParameter
func decodeLogPage0e(fp *os.File, data []byte, dataLen int) {
	for offset := 4; offset < dataLen; {
		parameterCode := data[offset + 1]
		parameterLen := data[offset + 3]
		if ss, ok := startStopFuncs[parameterCode]; ok {
			fmt.Printf("  %s: %s\n", ss.name, ss.process(data[offset:]))
		} else {
			fmt.Printf("  Invalid parameterCode: %d\n", parameterCode)
		}
		offset += int(parameterLen) + 4
	}
}

func domDecode(data []byte) string {
	year := new(bytes.Buffer)
	year.Write(data[4:8])

	week := new(bytes.Buffer)
	week.Write(data[8:10])

	return fmt.Sprintf("%s.%s", year.String(), week.String())
}

func commonStartStopDataDecode(data []byte) string {
	val := 0
	for i := 4; i < 8; i++ {
		val = val << 8 | int(data[i])
	}
	return fmt.Sprintf("%d", val)
}

//noinspection GoUnusedParameter
func decodeLogPage0f(fp *os.File, data []byte, dataLen int) {
	dumpParameterBits(data)
	if dataLen > 64 {
		dataLen = 68
		fmt.Printf("  Only showing first 64 bytes out of %d available\n", dataLen)
	}
	dumpMemory(data[4:], dataLen - 4, "    ")
}

//noinspection GoUnusedParameter
func decodeLogPage10(fp *os.File, data []byte, dataLen int) {
	pageLength := int(data[2]) << 8 | int(data[3])
	if pageLength != 0x190 {
		fmt.Printf("Specification violation: Page length should be 0x190 and is %x\n", pageLength)
	}

	atLeastOneValid := false
	for offset := 4; offset < dataLen; offset += 20 {
		if decodeTestResults(data[offset:]) {
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		fmt.Printf("  No self-tests have been run\n")
	}
}

func decodeTestResults(data []byte) bool {
	checkForZero := 0
	for i := 4; i < 20; i++ {
		checkForZero += int(data[i])
	}
	if checkForZero == 0 {
		return false
	}
	fmt.Printf("  Test code=%d, Test results=%d, Test number=%d\n", data[4] >> 5 & 0x7,
		data[4] & 0xf, data[5])
	converter := dataToInt{data, 6, 2}
	hours := converter.getInt()
	fmt.Printf("  Accumulated power on hours: %d\n", hours)
	converter = dataToInt{data, 8, 8}
	addressOfFailure := converter.getInt64()
	fmt.Printf("  Address of first failure: 0x%x\n", addressOfFailure)
	fmt.Printf("  Sense key: %d\n", data[16] & 0xf)
	fmt.Printf("  ASC: %d, ASCQ: %d\n", data[17], data[18])

	return true
}

//noinspection GoUnusedParameter
func decodeLogPage11(fp *os.File, data []byte, dataLen int) {
	if dataLen != int(data[3]) + 4 {
		fmt.Printf("  Invalid parameter length\n")
	}
	fmt.Printf("  Percentage used endurance indicator: %d%%\n", data[7])
}

type backgroundScan struct {
	name	string
	decode	func([]byte, int) int
}

//noinspection GoUnusedParameter
func decodeLogPage15(fp *os.File, data []byte, dataLen int) {
	backgroundDecode := map[int]backgroundScan {
		0x00: {"Background Scan Status", decodeBackgroundScan},
		0x01: {"Background Scan Results", decodeBackgroundResults},
	}
	for offset := 4; offset < dataLen; {
		convert := dataToInt{data, offset, 2}
		pc := convert.getInt()
		if bd, ok := backgroundDecode[pc]; ok {
			fmt.Printf("  %s\n", bd.name)
			offset += bd.decode(data[offset:], dataLen - offset)
		}
	}
}

//noinspection GoUnusedParameter
func decodeBackgroundScan(data []byte, dataLen int) int {
	converter := dataToInt{data,4, 4}
	fmt.Printf("    Accumulated power on minutes: %d\n", converter.getInt64())
	fmt.Printf("    Scan status: %d\n", data[9])

	converter.setOffsetCount(10,2)
	fmt.Printf("    Number of scans performed: %d\n", converter.getInt())

	converter.setOffsetCount(12,2)
	fmt.Printf("    Scan progress: %d%%\n", converter.getInt())

	converter.setOffsetCount(14,2)
	fmt.Printf("    Number of medium scans performed: %d\n", converter.getInt())

	return int(data[3]) + 4
}

func decodeBackgroundResults(data []byte, dataLen int) int {
	dumpMemory(data, dataLen, "    ")
	return int(data[3]) + 4
}

//noinspection GoUnusedParameter
func decodeLogPage18(fp *os.File, data []byte, dataLen int) {
	for offset := 4; offset < dataLen; {
		offset += decodePortLog(data[offset:])
	}
}

func decodePortLog(data []byte) int {
	paramLen := int(data[3])
	fmt.Printf("  Protocol Identifer: %s\n", protocolIndentifier[data[4] & 0xf])
	dumpMemory(data[5:], paramLen - 1, "    ")

	return paramLen + 4
}

//noinspection GoUnusedParameter
func decodeLogPage19(fp *os.File, data []byte, dataLen int) {
	for offset :=4; offset < dataLen; {
		converter := dataToInt{data, offset, 2}
		paramCode := converter.getInt()
		if statPage, ok := page19StatFuncs[paramCode]; ok {
			fmt.Printf("  %s\n", statPage.name)
			offset += statPage.process(data[offset:])
		} else {
			fmt.Printf("  [Invalid parameter code: %d]\n", paramCode)
			return
		}
	}
}

func statsPage1(data []byte) int {

	for _, statMulti := range generalStatsBytes {
		converter := dataToInt{data, statMulti.byteOffset, statMulti.numberBytes}
		val := converter.getInt64()
		fmt.Printf("    %s: %d", statMulti.name, val)
		if val > 10000 {
			fmt.Printf(" (%s)", support.Humanize(val, 1))
		}
		fmt.Printf("\n")
	}
	return int(data[3] + 4)
}

func statsPage2(data []byte) int {
	conv := dataToInt{data, 4, 8}
	val := conv.getInt64()
	fmt.Printf("    Idle time intervals: %d\n", val)

	return int(data[3] + 4)
}

func statsPage3(data []byte) int {
	fmt.Printf("    Return len %d + 4\n", data[3])
	return int(data[3] + 4)
}

func statsPage4(data []byte) int {
	return int(data[3] + 4)
}

//noinspection GoUnusedParameter
func decodeLogPage1a(fp *os.File, data []byte, dataLen int) {

}

//noinspection GoUnusedParameter
func decodeLogPage2f(fp *os.File, data []byte, dataLen int) {

}