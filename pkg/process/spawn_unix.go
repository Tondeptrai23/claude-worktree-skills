//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) *exec.Cmd {
	return exec.Command("bash", "-c", command)
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
