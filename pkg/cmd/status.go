package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"path/filepath"

	"claude-worktree-skill/pkg/config"
	"claude-worktree-skill/pkg/process"
	"claude-worktree-skill/pkg/slot"
	"claude-worktree-skill/pkg/template"

	"github.com/urfave/cli/v2"
)

func StatusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show status of all feature slots",
		ArgsUsage: "[slot]",
		Action:    runStatus,
	}
}

func runStatus(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	filterSlot := ""
	if c.NArg() > 0 {
		filterSlot = c.Args().Get(0)
	}

	worktreesDir := config.WorktreesDir(rootDir)
	slots, _ := slot.DiscoverSlots(worktreesDir)

	fmt.Println()
	fmt.Println("=== Feature Worktree Status ===")
	fmt.Println()

	found := false
	for _, meta := range slots {
		if filterSlot != "" && fmt.Sprintf("%d", meta.Slot) != filterSlot {
			continue
		}
		found = true

		slotDir := config.SlotDir(rootDir, meta.Slot)

		fmt.Printf("\033[36mSlot %d: %s\033[0m\n", meta.Slot, meta.FeatureName)
		fmt.Printf("  Created: %s\n", meta.CreatedAt)
		fmt.Printf("  Path:    %s\n", slotDir)
		fmt.Println()

		// Service table
		w := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		for svcName, svcMeta := range meta.Services {
			pidFile := filepath.Join(slotDir, ".pids", svcName+".pid")
			status := "\033[31mstopped\033[0m"
			if pid, running := process.IsRunning(pidFile); running {
				status = fmt.Sprintf("\033[32mrunning\033[0m (PID %d)", pid)
			}
			fmt.Fprintf(w, "  %s\tport %d\t%s\tbranch: %s\n",
				svcName, svcMeta.Port, status, svcMeta.Branch)
		}
		w.Flush()

		// URLs
		if cfg.Nginx.Enabled {
			fmt.Println()
			fmt.Println("  URLs:")
			for svcName := range meta.Services {
				svc, ok := cfg.Services[svcName]
				if !ok || !svc.Expose {
					continue
				}
				url := template.Resolve("{{"+svcName+".url}}", svcName, meta.Slot, meta.FeatureName, cfg)
				fmt.Printf("    %s\n", url)
			}
		}
		fmt.Println()
	}

	if !found {
		fmt.Println("  No active feature slots.")
		fmt.Println()
		fmt.Println("  Create one with: wt create 1 my-feature")
		fmt.Println()
	}

	return nil
}
