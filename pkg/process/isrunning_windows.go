//go:build windows

package process

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

// IsRunning checks if a PID file exists and the process is alive.
func IsRunning(pidPath string) (int, bool) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}

	return pid, isProcessRunning(pid)
}

// isProcessRunning checks if a Windows process is alive by opening its handle.
func isProcessRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}
