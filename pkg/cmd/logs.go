package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"claude-worktree-skill/pkg/config"

	"github.com/urfave/cli/v2"
)

func LogsCommand() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "Tail logs for a feature slot",
		ArgsUsage: "<slot> [service]",
		Action:    runLogs,
	}
}

func runLogs(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)
	_ = cfg

	if c.NArg() < 1 {
		return fmt.Errorf("usage: wt logs <slot> [service]")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	slotDir := config.SlotDir(rootDir, slotNum)
	logsDir := filepath.Join(slotDir, ".logs")

	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return fmt.Errorf("no logs found for slot %d", slotNum)
	}

	service := ""
	if c.NArg() > 1 {
		service = c.Args().Get(1)
	}

	// Build tail arguments
	var tailArgs []string
	tailArgs = append(tailArgs, "-f")

	if service != "" {
		logFile := filepath.Join(logsDir, service+".log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("no log file for service '%s'", service)
		}
		tailArgs = append(tailArgs, logFile)
	} else {
		matches, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
		if len(matches) == 0 {
			return fmt.Errorf("no log files in %s", logsDir)
		}
		tailArgs = append(tailArgs, matches...)
	}

	// Replace current process with tail -f
	tailPath, err := findTail()
	if err != nil {
		return err
	}

	return syscall.Exec(tailPath, append([]string{"tail"}, tailArgs...), os.Environ())
}

func findTail() (string, error) {
	for _, p := range []string{"/usr/bin/tail", "/bin/tail"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("tail not found")
}
