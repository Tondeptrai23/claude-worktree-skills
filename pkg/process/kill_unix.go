//go:build !windows

package process

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// KillByPidFile terminates the process tracked by a PID file.
// Sends SIGTERM to the process group, waits gracePeriod, then SIGKILL.
func KillByPidFile(pidPath string, gracePeriod time.Duration) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, fmt.Errorf("reading PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidPath)
		return 0, fmt.Errorf("invalid PID: %w", err)
	}

	// Check if alive
	if err := syscall.Kill(pid, 0); err != nil {
		os.Remove(pidPath)
		return pid, nil // already dead
	}

	// Try to get process group
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Fallback: kill just the PID
		syscall.Kill(pid, syscall.SIGTERM)
	} else {
		// Kill the entire process group
		syscall.Kill(-pgid, syscall.SIGTERM)
	}

	// Wait for process to exit
	deadline := time.After(gracePeriod)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			// Force kill
			if pgid > 0 {
				syscall.Kill(-pgid, syscall.SIGKILL)
			} else {
				syscall.Kill(pid, syscall.SIGKILL)
			}
			os.Remove(pidPath)
			return pid, nil
		case <-ticker.C:
			if err := syscall.Kill(pid, 0); err != nil {
				// Process exited
				os.Remove(pidPath)
				return pid, nil
			}
		}
	}
}
