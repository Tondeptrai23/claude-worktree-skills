package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Spawn starts a background process and tracks its PID.
func Spawn(workDir, command string, env map[string]string, logPath, pidPath string) error {
	os.MkdirAll(filepath.Dir(logPath), 0755)
	os.MkdirAll(filepath.Dir(pidPath), 0755)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	cmd := shellCommand(command)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Build environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Create new process group so we can kill the entire tree
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("starting process: %w", err)
	}

	// Write PID
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}

	// Release — don't wait. Process outlives the wt binary.
	go func() {
		cmd.Wait()
		logFile.Close()
	}()

	return nil
}

// shellCommand and setSysProcAttr are defined in platform-specific files:
// spawn_unix.go and spawn_windows.go
