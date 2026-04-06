//go:build windows

package process

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) *exec.Cmd {
	return exec.Command("cmd", "/C", command)
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
