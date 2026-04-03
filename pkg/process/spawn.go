package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Spawn starts a background process and tracks its PID.
func Spawn(workDir, command string, env map[string]string, logPath, pidPath string) error {
	os.MkdirAll(filepath.Dir(logPath), 0755)
	os.MkdirAll(filepath.Dir(pidPath), 0755)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Build environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Create new process group so we can kill the entire tree
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

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
	err = syscall.Kill(pid, 0)
	return pid, err == nil
}
