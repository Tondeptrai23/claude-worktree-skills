package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"

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

	// Run tail as a subprocess so we can color its output
	cmd := exec.Command("tail", append([]string{"-f"}, logFiles...)...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "==>") && strings.HasSuffix(line, "<==") {
			// Extract the file path from "==> /path/to/backend.log <=="
			inner := strings.TrimPrefix(strings.TrimSuffix(line, " <=="), "==> ")
			color, ok := fileColors[inner]
			if !ok {
				color = ColorCyan
			}
			svc := strings.TrimSuffix(filepath.Base(inner), ".log")
			fmt.Println(SprintColor(color, "====== ■ %s ■ ======", svc))
			fmt.Println(line)
		} else {
			fmt.Println(line)
		}
	}

	return cmd.Wait()
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
