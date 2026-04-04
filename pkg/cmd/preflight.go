package cmd

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"

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
	fmt.Println("\033[32m[OK]\033[0m All preflight checks passed")
	return nil
}

func checkDisk(hasErrors *bool) {
	out, err := exec.Command("df", "--output=avail", ".").Output()
	if err != nil {
		fmt.Printf("\033[33m[WARN]\033[0m Could not check disk space: %v\n", err)
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
		fmt.Printf("\033[33m[WARN]\033[0m Disk: %d GB available (< 5 GB)\n", availGB)
	} else {
		fmt.Printf("\033[32m[OK]\033[0m Disk: %d GB available\n", availGB)
	}
}

func checkPorts(slot int, cfg *config.Config, hasErrors *bool) {
	for svcName, svc := range cfg.Services {
		port := svc.Port(slot, cfg.PortOffset)
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Printf("\033[31m[ERR]\033[0m Port %d (%s) is already in use\n", port, svcName)
			*hasErrors = true
		} else {
			ln.Close()
			fmt.Printf("\033[32m[OK]\033[0m Port %d (%s) available\n", port, svcName)
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
			fmt.Printf("\033[31m[ERR]\033[0m Branch %q already checked out at %s\n", branch, conflict)
			*hasErrors = true
		} else {
			fmt.Printf("\033[32m[OK]\033[0m Branch %q not checked out elsewhere\n", branch)
		}
	}
}

func checkDocker(hasErrors *bool) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		fmt.Printf("\033[31m[ERR]\033[0m Docker is not running\n")
		*hasErrors = true
	} else {
		fmt.Printf("\033[32m[OK]\033[0m Docker: running\n")
	}
}

func checkNginx(hasErrors *bool) {
	out, _ := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if bytes.Contains(out, []byte("feature-router")) {
		fmt.Printf("\033[32m[OK]\033[0m Nginx: running\n")
	} else {
		fmt.Printf("\033[33m[WARN]\033[0m Nginx: not running (will be started on 'wt start')\n")
	}
}

func checkSlot(slot int, rootDir string, hasErrors *bool) {
	slotDir := config.SlotDir(rootDir, slot)
	if _, err := os.Stat(slotDir); err == nil {
		fmt.Printf("\033[31m[ERR]\033[0m Slot %d already in use — run 'wt destroy %d' first\n", slot, slot)
		*hasErrors = true
	} else {
		fmt.Printf("\033[32m[OK]\033[0m Slot %d available\n", slot)
	}
}

