package main

import (
	"os"
	"fmt"
)

func getMediaInfo(fp *os.File) (*dkiocGetMediaInfoExt, error) {
	return nil, fmt.Errorf("linux version not supported")
}

func sendUSCSI(fp *os.File, cdb []byte, data []byte, flag int32) (len int, err error) {
	return 0, fmt.Errorf("uscsi not supported on OSX")
}
