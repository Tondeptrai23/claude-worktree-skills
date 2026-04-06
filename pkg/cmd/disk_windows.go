//go:build windows

package cmd

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func checkDisk(hasErrors *bool) {
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
		PrintWarn("Could not check disk space: %v\n", err)
		return
	}

	availGB := int64(freeBytesAvailable / 1024 / 1024 / 1024)
	if availGB < 5 {
		PrintWarn("Disk: %d GB available (< 5 GB)\n", availGB)
	} else {
		PrintOK("Disk: %d GB available\n", availGB)
	}
}
