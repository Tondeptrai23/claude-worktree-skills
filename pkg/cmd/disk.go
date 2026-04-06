package cmd

import "github.com/Tondeptrai23/claude-worktree-skills/pkg/process"

func checkDisk(hasErrors *bool) {
	avail, err := process.AvailableDiskGB()
	if err != nil {
		PrintWarn("Could not check disk space: %v\n", err)
		return
	}

	availGB := avail / 1024 / 1024
	if availGB < 5 {
		PrintWarn("Disk: %d GB available (< 5 GB)\n", availGB)
	} else {
		PrintOK("Disk: %d GB available\n", availGB)
	}
}
