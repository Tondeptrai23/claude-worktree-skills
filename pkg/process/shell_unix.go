//go:build !windows

package process

import "os/exec"

// Command returns an exec.Cmd that runs the given command string through bash.
func Command(command string) *exec.Cmd {
	return exec.Command("bash", "-c", command)
}
