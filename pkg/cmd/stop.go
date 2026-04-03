package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-worktree-skill/pkg/config"
	"claude-worktree-skill/pkg/process"

	"github.com/urfave/cli/v2"
)

func StopCommand() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "Stop services in a worktree slot",
		ArgsUsage: "<slot>",
		Action:    runStop,
	}
}

func runStop(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 1 {
		return fmt.Errorf("usage: wt stop <slot>")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	slotDir := config.SlotDir(rootDir, slotNum)
	pidsDir := filepath.Join(slotDir, ".pids")

	if _, err := os.Stat(pidsDir); os.IsNotExist(err) {
		fmt.Printf("\033[33m[!]\033[0m No PID files for slot %d\n", slotNum)
		return nil
	}

	fmt.Printf("\033[32m[*]\033[0m Stopping services for slot %d\n", slotNum)

	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}

		svcName := strings.TrimSuffix(entry.Name(), ".pid")
		pidFile := filepath.Join(pidsDir, entry.Name())

		pid, err := process.KillByPidFile(pidFile, 5*time.Second)
		if err != nil {
			fmt.Printf("\033[31m[!]\033[0m Error stopping %s: %v\n", svcName, err)
		} else {
			fmt.Printf("\033[32m[OK]\033[0m Stopped %s (PID %d)\n", svcName, pid)
		}
	}

	fmt.Printf("\033[32m[OK]\033[0m All services stopped for slot %d\n", slotNum)
	return nil
}
