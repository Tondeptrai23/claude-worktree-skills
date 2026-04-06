//go:build !windows

package process

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
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

	// Signal 0 checks existence without sending a signal
	err = unix.Kill(pid, 0)
	return pid, err == nil
}
