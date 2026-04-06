//go:build windows

package shell

import "os/exec"

// Command returns an exec.Cmd that runs the given command string through cmd.exe.
func Command(command string) *exec.Cmd {
	return exec.Command("cmd", "/C", command)
}
