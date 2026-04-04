package cmd

import (
	"fmt"
	"os"

	"path/filepath"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/template"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/urfave/cli/v2"
)

func NextSlotCommand() *cli.Command {
	return &cli.Command{
		Name:  "next-slot",
		Usage: "Print the next available slot number",
		Action: func(c *cli.Context) error {
			cfg := c.App.Metadata["config"].(*config.Config)
			rootDir := c.App.Metadata["rootDir"].(string)

			for n := 1; n <= cfg.MaxSlots; n++ {
				slotDir := config.SlotDir(rootDir, n)
				if _, err := os.Stat(slotDir); os.IsNotExist(err) {
					fmt.Println(n)
					return nil
				}
			}

			return fmt.Errorf("all %d slots are occupied — run 'wt status' to review", cfg.MaxSlots)
		},
	}
}

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

	PrintEmptyLine()
	Print("=== Feature Worktree Status ===\n")
	PrintEmptyLine()

	found := false
	for _, meta := range slots {
		if filterSlot != "" && fmt.Sprintf("%d", meta.Slot) != filterSlot {
			continue
		}
		found = true

		slotDir := config.SlotDir(rootDir, meta.Slot)

		PrintInfo("Slot %d: %s", meta.Slot, meta.FeatureName)
		Print("  Created: %s\n", meta.CreatedAt)
		Print("  Path:    %s\n", slotDir)
		PrintEmptyLine()

		// Service table
		PrintTable([]string{"Service", "Port", "Status", "Branch", "URLs"}, func() []table.Row {
			rows := []table.Row{}
			for svcName, svcMeta := range meta.Services {
				pidFile := filepath.Join(slotDir, ".pids", svcName+".pid")
				status := SprintColor(ColorRed, "Stopped")
				if pid, running := process.IsRunning(pidFile); running {
					status = SprintColor(ColorGreen, "Running (PID %d)", pid)
				}
				svc, ok := cfg.Services[svcName]
				url := "N/A"
				if ok && svc.Expose {
					url = template.Resolve("{{"+svcName+".url}}", svcName, meta.Slot, meta.FeatureName, cfg)
				}
				rows = append(rows, table.Row{svcName, svcMeta.Port, status, svcMeta.Branch, url})
			}
			return rows
		}())
	}

	if !found {
		Print("  No active feature slots.\n")
		PrintEmptyLine()
		Print("  Create one with: wt create 1 my-feature\n")
		PrintEmptyLine()
	}

	return nil
}
