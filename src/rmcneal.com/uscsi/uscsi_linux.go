package main

import (
	"fmt"
	"os"
)

//noinspection GoUnusedParameter
func sendUSCSI(fp *os.File, cdb []byte, data []byte, flag int32) (int, error) {
	return 0, fmt.Errorf("uscsi not supported on Linux")
}

