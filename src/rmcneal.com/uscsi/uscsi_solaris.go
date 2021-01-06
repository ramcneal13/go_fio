package main

import (
	"reflect"
	"unsafe"
	"syscall"
	"fmt"
	"os"
)

type uscsiCmd struct {
	flags              int32
	status             int16
	timeout            int16
	cdb                unsafe.Pointer
	buf                unsafe.Pointer
	bufLen             int64
	resid              int64
	cdbLen             int8
	senseRequestLen    int8
	senseRequestStatus int8
	senseRequestResid  int8
	senseBuf           unsafe.Pointer
	pathInstance       int64
}

const (
	UscsiWrite    = 0
	UscsiSilent   = 1
	UscsiDiagnose = 2
	UscsiIsolate  = 4
	UscsiRead     = 8
	UscsiReset    = 0x4000
	UscsiRQEnable = 0x10000
)

func sendUSCSI(fp *os.File, cdb []byte, data []byte, flags int32) (int, error) {
//	slice := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
//	for offset := 0; offset < len(buf); offset += 512 {
//		marker := (*markerBlock)(unsafe.Pointer(uintptr(unsafe.Pointer(slice.Data)) + uintptr(offset)))

	var cmd uscsiCmd

	senseBuf := make([]byte, 256)
	cmd.flags = flags
	switch cdb[0] {
	case INQUIRY, SECURITY_PROTO_IN:
		cmd.flags |= UscsiRead
	}
	cmd.timeout = 30

	slice := (*reflect.SliceHeader)(unsafe.Pointer(&cdb))
	cmd.cdb = unsafe.Pointer(slice.Data)
	cmd.cdbLen = int8(len(cdb))

	slice = (*reflect.SliceHeader)(unsafe.Pointer(&data))
	cmd.buf = unsafe.Pointer(slice.Data)
	cmd.bufLen = int64(len(data))

	slice = (*reflect.SliceHeader)(unsafe.Pointer(&senseBuf))
	cmd.senseBuf = unsafe.Pointer(slice.Data)
	cmd.senseRequestLen = int8(len(senseBuf))

	if debugOutput {
		fmt.Printf("CDB:\n")
		dumpMemory(cdb, len(cdb), "    ")
	}

	if _, _, err := syscall.Syscall(54, fp.Fd(), uintptr((4 << 8)|201), uintptr(unsafe.Pointer(&cmd))); err != 0 {
		return 0, fmt.Errorf("syscall error: %s", err)
	}
	if cmd.status != 0 {
		return 0, fmt.Errorf("status: %d", cmd.status)
	}

	if debugOutput && ((cmd.flags & UscsiRead) != 0) {
		fmt.Printf("DataIn (available: %d):\n", cmd.bufLen-cmd.resid)
		dumpMemory(data, len(data), "    ")
	}

	return int(cmd.bufLen - cmd.resid), nil
}

