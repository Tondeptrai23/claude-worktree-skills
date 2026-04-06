//go:build !windows

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tailFiles tails multiple log files, applying decoration to each line.
func tailFiles(logFiles []string, fileColors map[string]Color) error {
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
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "==>") && strings.HasSuffix(line, "<==") {
			inner := strings.TrimPrefix(strings.TrimSuffix(line, " <=="), "==> ")
			color, ok := fileColors[inner]
			if !ok {
				color = ColorCyan
			}
			svc := strings.TrimSuffix(filepath.Base(inner), ".log")
			fmt.Println(SprintColor(color, "====== ■ %s ■ ======", svc))
			fmt.Println(line)
		} else {
			fmt.Println(line)
		}
	}

	return cmd.Wait()
}
