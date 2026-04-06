//go:build !windows

package process

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
)

func AvailableDiskGB() (int64, error) {
	out, err := exec.Command("df", "--output=avail", ".").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute df command: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected output from df command")
	}
	avail, err := strconv.ParseInt(string(bytes.TrimSpace(lines[1])), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse available disk space: %v", err)
	}

	return avail, nil
}
