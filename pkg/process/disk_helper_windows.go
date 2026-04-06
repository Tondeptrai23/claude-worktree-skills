//go:build windows

package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func AvailableDiskGB() (int64, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable uint64
	cwd, _ := windows.UTF16PtrFromString(".")
	ret, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(cwd)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		0,
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("failed to check disk space: %v", err)
	}

	availGB := int64(freeBytesAvailable / 1024 / 1024 / 1024)
	return availGB, nil
}
