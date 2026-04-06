//go:build !windows

package process

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tailFilesImpl tails multiple log files using the system tail command (Unix).
func tailFilesImpl(logFiles []string, fileColors map[string]interface{}, logger LineLogger) error {
	cmd := exec.Command("tail", append([]string{"-f"}, logFiles...)...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	color, svc := interface{}(nil), ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "==>") && strings.HasSuffix(line, "<==") {
			fullPath := strings.TrimPrefix(strings.TrimSuffix(line, " <=="), "==> ")
			if c, ok := fileColors[fullPath]; ok {
				color = c
			} else {
				color = ""
			}
			svc = strings.TrimSuffix(filepath.Base(fullPath), ".log")
			logger(color, svc, line)
		} else {
			logger(color, svc, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return cmd.Wait()
}
