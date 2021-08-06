package main

import (
	"fmt"
	"os"
)

//noinspection GoUnusedParameter
func getMediaInfo(fp *os.File) (*dkiocGetMediaInfoExt, error) {
	return nil, fmt.Errorf("linux version not supported")
}

//noinspection GoUnusedParameter
func sendUSCSI(fp *os.File, cdb []byte, data []byte, flag int32) (int, error) {
	return 0, fmt.Errorf("uscsi not supported on Linux")
}

