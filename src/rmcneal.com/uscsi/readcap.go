package main

import (
	"fmt"
	"os"
	"rmcneal.com/support"
)

func init() {
}

func diskinfoReadCap(d *diskInfoData) {
	if data, _, err := scsiReadCap(d.fp); err == nil {
		converter := dataToInt{data, 0, 8}
		totalBlocks := converter.getInt64()
		converter.setOffsetCount(8, 4)
		blockSize := converter.getInt64()
		d.capacity = totalBlocks * blockSize
	}
}

func scsiReadCapCommand(fp *os.File) {
	if data, dataLen, err := scsiReadCap(fp); err == nil {
		if dataLen != 32 {
			fmt.Printf("  READCAP returned unexpected length (%d) instead of 32 bytes\n", dataLen)
		}

		converter := dataToInt{data, 0, 8}
		totalBlocks := converter.getInt64()
		converter.setOffsetCount(8,4)
		blockSize := converter.getInt64()

		fmt.Printf("  Last LBA: %d, block length: %d\n  Capacity: %s\n", totalBlocks, blockSize,
			support.Humanize(totalBlocks * blockSize, 1))
	} else {
		fmt.Printf("  readcap error: %s\n", err)
	}
}

func scsiReadCap(fp *os.File) ([]byte, int, error) {
	cdb := make([]byte, 16)
	data := make([]byte, 256)

	cdb[0] = 0x9e
	cdb[1] = 0x10
	cdb[12] = 1
	cdb[13] = 0
	if debugOutput {
		fmt.Printf("CDB:\n")
		dumpMemory(cdb, len(cdb), "    ")
	}

	dataLen, err := sendUSCSI(fp, cdb, data, 0)

	return data, dataLen, err
}

