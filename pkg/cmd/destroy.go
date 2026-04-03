package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/db"
	gitops "github.com/Tondeptrai23/claude-worktree-skills/pkg/git"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/nginx"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"

	"github.com/urfave/cli/v2"
)

func DestroyCommand() *cli.Command {
	return &cli.Command{
		Name:      "destroy",
		Usage:     "Destroy a worktree slot",
		ArgsUsage: "<slot>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "teardown-db", Usage: "Also run the database teardown command"},
		},
		Action: runDestroy,
	}
}

func runDestroy(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 1 {
		return fmt.Errorf("usage: wt destroy <slot> [--teardown-db]")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	slotDir := config.SlotDir(rootDir, slotNum)
	if _, err := os.Stat(slotDir); os.IsNotExist(err) {
		return fmt.Errorf("slot %d does not exist", slotNum)
	}

	// Stop services first
	fmt.Printf("\033[32m[*]\033[0m Stopping services for slot %d\n", slotNum)
	stopServices(slotDir)

	fmt.Printf("\033[32m[*]\033[0m Destroying feature slot %d\n", slotNum)

	// Load metadata to know which repos to remove
	meta, _ := slot.Load(slotDir)

	// Remove git worktrees
	removedRepos := make(map[string]bool)
	if meta != nil {
		for svcName, svcMeta := range meta.Services {
			repoKey := svcMeta.RepoKey
			if removedRepos[repoKey] {
				continue
			}

			svc, ok := cfg.Services[svcName]
			if !ok {
				continue
			}

			worktreeDir := filepath.Join(slotDir, repoKey)
			if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
				continue
			}

			repoDir := filepath.Join(rootDir, svc.Path)
			fmt.Printf("\033[32m[*]\033[0m Removing %s/ worktree\n", repoKey)
			gitops.RemoveWorktree(repoDir, worktreeDir)
			removedRepos[repoKey] = true
		}
	}

	// Run database teardown if requested
	if c.Bool("teardown-db") {
		fmt.Printf("\033[32m[*]\033[0m Running database teardown\n")
		if err := db.Teardown(cfg, slotNum); err != nil {
			fmt.Printf("\033[33m[!]\033[0m Database teardown failed: %v\n", err)
		}
	}

	// Remove slot directory
	os.RemoveAll(slotDir)

	// Regenerate nginx
	nginx.Generate(cfg, rootDir)
	nginx.Reload()

	fmt.Printf("\033[32m[OK]\033[0m Feature slot %d destroyed\n", slotNum)
	return nil
}

func stopServices(slotDir string) {
	pidsDir := filepath.Join(slotDir, ".pids")
	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		return
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
		} else if pid > 0 {
			fmt.Printf("\033[32m[OK]\033[0m Stopped %s (PID %d)\n", svcName, pid)
		}
	}
}
