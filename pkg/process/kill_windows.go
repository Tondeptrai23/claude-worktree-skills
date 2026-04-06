//go:build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// KillByPidFile terminates the process tracked by a PID file.
// Uses taskkill /T /F to kill the process tree on Windows.
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
	if !isProcessRunning(pid) {
		os.Remove(pidPath)
		return pid, nil // already dead
	}

	// Kill entire process tree with taskkill
	pidStr := strconv.Itoa(pid)
	exec.Command("taskkill", "/T", "/F", "/PID", pidStr).Run()

	// Wait for process to exit
	deadline := time.After(gracePeriod)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			// Force kill again
			exec.Command("taskkill", "/T", "/F", "/PID", pidStr).Run()
			os.Remove(pidPath)
			return pid, nil
		case <-ticker.C:
			if !isProcessRunning(pid) {
				os.Remove(pidPath)
				return pid, nil
			}
		}
	}
}
