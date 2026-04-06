package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"

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

	var logFiles []string
	if service != "" {
		logFile := filepath.Join(logsDir, service+".log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("no log file for service '%s'", service)
		}
		logFiles = append(logFiles, logFile)
	} else {
		matches, _ := filepath.Glob(filepath.Join(logsDir, "*.log"))
		if len(matches) == 0 {
			return fmt.Errorf("no log files in %s", logsDir)
		}
		logFiles = matches
	}

	// Assign a color per file
	colors := Colorlist()
	fileColors := make(map[string]Color, len(logFiles))
	for i, f := range logFiles {
		fileColors[f] = colors[i%len(colors)]
	}

	printLogsHeader(slotNum, logFiles)

	// Convert to interface map for process.TailFiles
	colorMap := colorMap(fileColors)

	// Wrap SprintColor to match LineLogger signature
	lineLogger := func(color interface{}, args ...any) {
		if c, ok := color.(Color); ok {
			fmt.Println(SprintColor(c, "[%s] %s", args...))
			return
		}
		// Fallback: use default color
		fmt.Println(SprintColor(ColorCyan, "[%s] %s", args...))

	}

	return process.TailFiles(logFiles, colorMap, lineLogger)
}

func colorMap(fileColors map[string]Color) map[string]interface{} {
	m := make(map[string]interface{}, len(fileColors))
	for k, v := range fileColors {
		m[k] = v
	}
	return m
}

func printLogsHeader(slotNum int, logFiles []string) {
	colors := Colorlist()
	PrintEmptyLine()
	Print("%s\n", SprintColor(ColorCyan, "=== Logs for slot %d ===", slotNum))
	PrintEmptyLine()
	for i, f := range logFiles {
		color := colors[i%len(colors)]
		svc := filepath.Base(f)
		svc = svc[:len(svc)-len(filepath.Ext(svc))] // strip .log
		Print("  %s %s\n", SprintColor(color, "■"), svc)
	}
	PrintEmptyLine()
}
