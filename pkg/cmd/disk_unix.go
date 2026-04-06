//go:build !windows

package cmd

import (
	"bytes"
	"os/exec"
	"strconv"
)

func checkDisk(hasErrors *bool) {
	out, err := exec.Command("df", "--output=avail", ".").Output()
	if err != nil {
		PrintWarn("Could not check disk space: %v\n", err)
		return
	}
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	if len(lines) < 2 {
		return
	}
	avail, err := strconv.ParseInt(string(bytes.TrimSpace(lines[1])), 10, 64)
	if err != nil {
		return
	}
	availGB := avail / 1024 / 1024
	if availGB < 5 {
		PrintWarn("Disk: %d GB available (< 5 GB)\n", availGB)
	} else {
		PrintOK("Disk: %d GB available\n", availGB)
	}
}
