package cmd

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	gitops "github.com/Tondeptrai23/claude-worktree-skills/pkg/git"

	"github.com/urfave/cli/v2"
)

func PreflightCommand() *cli.Command {
	return &cli.Command{
		Name:      "preflight",
		Usage:     "Run pre-creation checks for a slot (disk, ports, branch, docker, nginx)",
		ArgsUsage: "<slot> <name>",
		Action:    runPreflight,
	}
}

func runPreflight(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 2 {
		return fmt.Errorf("usage: wt preflight <slot> <name>")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	name := sanitizeName(c.Args().Get(1))
	hasErrors := false

	// 1. Disk space
	checkDisk(&hasErrors)

	// 2. Port availability
	checkPorts(slotNum, cfg, &hasErrors)

	// 3. Branch conflicts
	checkBranches(slotNum, name, cfg, rootDir, &hasErrors)

	// 4. Docker
	checkDocker(&hasErrors)

	// 5. Nginx status
	checkNginx(&hasErrors)

	// 6. Slot availability
	checkSlot(slotNum, rootDir, &hasErrors)

	fmt.Println()
	if hasErrors {
		return fmt.Errorf("preflight failed — fix the errors above before creating")
	}
	PrintOK("All preflight checks passed\n")
	return nil
}

// checkDisk is defined in disk_unix.go and disk_windows.go

func checkPorts(slot int, cfg *config.Config, hasErrors *bool) {
	for svcName, svc := range cfg.Services {
		port := svc.Port(slot, cfg.PortOffset)
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			PrintErr("Port %d (%s) is already in use\n", port, svcName)
			*hasErrors = true
		} else {
			ln.Close()
			PrintOK("Port %d (%s) available\n", port, svcName)
		}
	}
}

func checkBranches(slot int, name string, cfg *config.Config, rootDir string, hasErrors *bool) {
	branch := cfg.ResolveBranch(name)

	checked := make(map[string]bool)
	for _, svc := range cfg.Services {
		repoKey := svc.RepoKey()
		if checked[repoKey] {
			continue
		}
		checked[repoKey] = true

		if conflict := gitops.WorktreeHasBranch(rootDir, svc.Path, branch); conflict != "" {
			PrintErr("Branch %q already checked out at %s\n", branch, conflict)
			*hasErrors = true
		} else {
			PrintOK("Branch %q not checked out elsewhere\n", branch)
		}
	}
}

func checkDocker(hasErrors *bool) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		PrintErr("Docker is not running\n")
		*hasErrors = true
	} else {
		PrintOK("Docker: running\n")
	}
}

func checkNginx(hasErrors *bool) {
	out, _ := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if bytes.Contains(out, []byte("feature-router")) {
		PrintOK("Nginx: running\n")
	} else {
		PrintWarn("Nginx: not running (will be started on 'wt start')\n")
	}
}

func checkSlot(slot int, rootDir string, hasErrors *bool) {
	slotDir := config.SlotDir(rootDir, slot)
	if _, err := os.Stat(slotDir); err == nil {
		PrintErr("Slot %d already in use — run 'wt destroy %d' first\n", slot, slot)
		*hasErrors = true
	} else {
		PrintOK("Slot %d available\n", slot)
	}
}

