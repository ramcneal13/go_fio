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

func osSpecificOpen(inputFile string) (*os.File, error) {
	var fp *os.File
	var err error

	if fp, err = os.Open(inputDevice); err != nil {
		/*
		 * See if the user just gave us the last component part of the device name.
		 */
		if fp, err = os.Open("/dev/rdsk/" + inputDevice); err == nil {
			inputDevice = "/dev/rdsk/" + inputDevice
		} else if fp, err = os.Open("/dev/rdsk/" + inputDevice + "p0"); err == nil {
			inputDevice = "/dev/rdsk/" + inputDevice + "p0"
		} else {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}
	}
	return fp, err
}

func getMediaInfo(fp *os.File) (*dkiocGetMediaInfoExt, error) {
	var cmd dkiocGetMediaInfoExt

	// DKIOCGMEDIAINFOEXT is (4 << 8) | 48
	if _, _, err := syscall.Syscall(54, fp.Fd(), uintptr((4<<8)|42), uintptr(unsafe.Pointer(&cmd))); err != 0 {
		return nil, fmt.Errorf("ioctl call failed: err=%s", err)
	} else {
		return &cmd, nil
	}
}

func sendUSCSI(fp *os.File, cdb []byte, data []byte, flags int32) (int, error) {
//	slice := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
//	for offset := 0; offset < len(buf); offset += 512 {
//		marker := (*markerBlock)(unsafe.Pointer(uintptr(unsafe.Pointer(slice.Data)) + uintptr(offset)))

	var cmd uscsiCmd

	senseBuf := make([]byte, 252) // Per SPC-3 standard, the maximum length of sense data is 252 bytes
	cmd.flags = flags | UscsiRQEnable
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

	if _, _, err := syscall.Syscall(54, fp.Fd(), uintptr((4 << 8)|201), uintptr(unsafe.Pointer(&cmd))); err != 0 {
		if debugOutput > 0 {
			fmt.Printf("    SenseBuf:\n")
			dumpMemory(senseBuf, len(senseBuf), "    ")
		}
		return 0, fmt.Errorf("syscall error: %s", err)
	}
	if cmd.status != 0 {
		return 0, fmt.Errorf("Status: %d", cmd.status)
	}

	return int(cmd.bufLen - cmd.resid), nil
}

