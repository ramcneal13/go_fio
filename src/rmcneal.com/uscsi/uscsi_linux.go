package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
	"reflect"
)

//noinspection GoUnusedParameter
func getMediaInfo(fp *os.File) (*dkiocGetMediaInfoExt, error) {
	return nil, fmt.Errorf("linux version not supported")
}

const (
	SgDxferFromDev = -3
	SgDxferToDev   = -2
)

type sgIoHdr struct {
	InterfaceId    int32          // offset: 0
	DxferDirection int32          // 4
	CmdLen         uint8          // 8
	MxSbLen        uint8          // 9
	IovecCount     uint16         // 10
	DxferLen       uint32         // 12
	Dxferp         unsafe.Pointer // 16
	Cmdp           unsafe.Pointer // 24
	Sbp            unsafe.Pointer // 32
	Timeout        uint32         // 40
	Flags          uint32         // 44
	PackId         int32          // 48
	UserPtr        unsafe.Pointer // 56
	Status         uint8          // 57
	MaskedStatus   uint8          // 58
	MsgStatus      uint8
	SbLenWr        uint8
	HostStatus     uint16
	DriverStatus   uint16
	Resid          int32
	Duration       uint32
	Info           uint32
}

func osSpecificOpen(inputDevice string) (*os.File, error) {
	fp, err := os.OpenFile(inputDevice, os.O_RDWR, 0)
	return fp, err
}

//noinspection GoUnusedParameter
func sendUSCSI(fp *os.File, cdb []byte, data []byte, flag int32) (int, error) {

	var siohdr sgIoHdr
	/*
	 * Linux SCSI ioctl data structure is variable in size as follows:
	 *    uint inlen;
	 *    uint outlen;
	 *    char cmd[x]; <--- here's where the variable length starts. 6, 10, 12, or 16 bytes will be common
	 *    char wdata[y]; <--- also variable based on the command.
	 */
	sbBuffer := make([]byte, 255)

	siohdr.InterfaceId = 'S'
	switch cdb[0] {
	case INQUIRY, SECURITY_PROTO_IN:
		siohdr.DxferDirection = SgDxferFromDev
	default:
		siohdr.DxferDirection = SgDxferToDev
	}
	siohdr.CmdLen = byte(len(cdb))
	siohdr.MxSbLen = byte(len(sbBuffer))
	siohdr.IovecCount = 0
	siohdr.DxferLen = uint32(len(data))

	slice := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	siohdr.Dxferp = unsafe.Pointer(slice.Data)

	slice = (*reflect.SliceHeader)(unsafe.Pointer(&cdb))
	siohdr.Cmdp = unsafe.Pointer(slice.Data)

	slice = (*reflect.SliceHeader)(unsafe.Pointer(&sbBuffer))
	siohdr.Sbp = unsafe.Pointer(slice.Data)

	siohdr.Timeout = 60 * 1000
	siohdr.PackId = 0

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fp.Fd(), 0x2285, uintptr(unsafe.Pointer(&siohdr))); err != 0 {
		return 0, fmt.Errorf("syscall error: %s", err)
	} else if siohdr.MaskedStatus != 0 {
		fmt.Printf("  ioctl failed, masked Status 0x%x\n", siohdr.MaskedStatus)
		return 0, fmt.Errorf("syscall Status error=0x%x", 0)
	}

	return len(data) - int(siohdr.Resid), nil
}

