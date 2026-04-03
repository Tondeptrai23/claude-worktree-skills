package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/envgen"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/nginx"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/template"

	"github.com/urfave/cli/v2"
)

func StartCommand() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start services in a worktree slot",
		ArgsUsage: "<slot>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "services", Aliases: []string{"s"}, Usage: "Comma-separated service list (default: all in slot)"},
		},
		Action: runStart,
	}
}

func runStart(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 1 {
		return fmt.Errorf("usage: wt start <slot>")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	slotDir := config.SlotDir(rootDir, slotNum)
	meta, err := slot.Load(slotDir)
	if err != nil {
		return fmt.Errorf("slot %d does not exist", slotNum)
	}

	servicesFilter := c.String("services")
	filterSet := parseServiceFilter(servicesFilter)

	// Merge env files (secrets + overrides)
	fmt.Printf("\033[32m[*]\033[0m Merging environment files for slot %d\n", slotNum)
	if err := envgen.MergeEnv(slotNum, cfg, rootDir, slotDir); err != nil {
		return fmt.Errorf("merging env: %w", err)
	}

	// Ensure nginx is running
	if err := nginx.EnsureRunning(cfg, rootDir); err != nil {
		fmt.Printf("\033[33m[!]\033[0m Could not start nginx: %v\n", err)
	}

	// Start services
	pidsDir := filepath.Join(slotDir, ".pids")
	logsDir := filepath.Join(slotDir, ".logs")

	for svcName, svcMeta := range meta.Services {
		if filterSet != nil && !filterSet[svcName] {
			continue
		}

		svc, ok := cfg.Services[svcName]
		if !ok {
			continue
		}

		mode, ok := svc.Modes[cfg.DefaultMode]
		if !ok || mode.Start == "" {
			continue
		}

		workDir := svc.WorkDir(slotDir)
		pidFile := filepath.Join(pidsDir, svcName+".pid")
		logFile := filepath.Join(logsDir, svcName+".log")

		// Skip if already running
		if pid, running := process.IsRunning(pidFile); running {
			fmt.Printf("\033[33m[!]\033[0m %s already running (PID %d)\n", svcName, pid)
			continue
		}

		// Resolve templates in start command
		startCmd := template.Resolve(mode.Start, svcName, slotNum, meta.FeatureName, cfg)

		// Build env overrides
		env := make(map[string]string)
		if svc.PortEnv != "" {
			env[svc.PortEnv] = fmt.Sprintf("%d", svcMeta.Port)
		}

		fmt.Printf("\033[32m[*]\033[0m Starting %s on port %d\n", svcName, svcMeta.Port)

		if err := process.Spawn(workDir, startCmd, env, logFile, pidFile); err != nil {
			fmt.Printf("\033[31m[!]\033[0m Failed to start %s: %v\n", svcName, err)
			continue
		}

		pid, _ := process.IsRunning(pidFile)
		fmt.Printf("\033[32m[OK]\033[0m %s started (PID %d)\n", svcName, pid)
	}

	fmt.Printf("\n\033[32m[OK]\033[0m Services started for slot %d\n", slotNum)
	fmt.Printf("  Logs: %s/\n", logsDir)
	fmt.Printf("  Stop: wt stop %d\n", slotNum)
	return nil
}

func parseServiceFilter(s string) map[string]bool {
	if s == "" {
		return nil
	}
	result := make(map[string]bool)
	for _, svc := range strings.Split(s, ",") {
		result[strings.TrimSpace(svc)] = true
	}
	return result
}
