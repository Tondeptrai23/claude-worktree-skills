package cmd

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/template"

	"github.com/urfave/cli/v2"
)

func HealthCommand() *cli.Command {
	return &cli.Command{
		Name:      "health",
		Usage:     "Check if services in a slot are responding",
		ArgsUsage: "<slot>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "timeout", Aliases: []string{"t"}, Value: 60, Usage: "Timeout in seconds"},
		},
		Action: runHealth,
	}
}

func runHealth(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 1 {
		return fmt.Errorf("usage: wt health <slot>")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	timeout := time.Duration(c.Int("timeout")) * time.Second

	slotDir := config.SlotDir(rootDir, slotNum)
	meta, err := slot.Load(slotDir)
	if err != nil {
		return fmt.Errorf("slot %d does not exist", slotNum)
	}

	PrintInfo("Checking health for slot %d...\n", slotNum)

	allHealthy := true

	// Check each service
	for svcName, svcMeta := range meta.Services {
		// First check if process is running
		pidFile := filepath.Join(slotDir, ".pids", svcName+".pid")
		if _, running := process.IsRunning(pidFile); !running {
			PrintErr("%s: process not running\n", svcName)
			allHealthy = false
			continue
		}

		// Wait for port to accept connections
		addr := fmt.Sprintf("localhost:%d", svcMeta.Port)
		start := time.Now()
		healthy := false

		for time.Since(start) < timeout {
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				conn.Close()
				elapsed := time.Since(start).Round(time.Millisecond)
				PrintOK("%s: localhost:%d responding (%s)\n", svcName, svcMeta.Port, elapsed)
				healthy = true
				break
			}
			time.Sleep(2 * time.Second)
		}

		if !healthy {
			PrintErr("%s: localhost:%d not responding after %s\n", svcName, svcMeta.Port, timeout)
			allHealthy = false
		}
	}

	// Check nginx
	if cfg.Nginx.Enabled {
		checkNginxHealth(slotNum, meta, cfg, &allHealthy)
	}

	PrintInfo("")
	if !allHealthy {
		return fmt.Errorf("some services are unhealthy — check logs with 'wt logs %d <service>'", slotNum)
	}

	// Print test URLs
	PrintInfo("Test URLs:")
	for svcName, svcMeta := range meta.Services {
		svc, ok := cfg.Services[svcName]
		if !ok || !svc.Expose {
			continue
		}
		url := template.Resolve("{{"+svcName+".url}}", svcName, slotNum, meta.FeatureName, cfg)
		PrintInfo("  %s: %s (direct: localhost:%d)\n", svcName, url, svcMeta.Port)
	}

	return nil
}

func checkNginxHealth(slotNum int, meta *slot.SlotMeta, cfg *config.Config, allHealthy *bool) {
	out, _ := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if !bytes.Contains(out, []byte("feature-router")) {
		PrintWarn("Nginx: not running (subdomain URLs won't work)\n")
		return
	}

	PrintOK("Nginx: running\n")

	// Verify nginx can route to each exposed service
	for svcName := range meta.Services {
		svc, ok := cfg.Services[svcName]
		if !ok || !svc.Expose {
			continue
		}

		url := template.Resolve("{{"+svcName+".url}}", svcName, slotNum, meta.FeatureName, cfg)
		// Quick TCP check on nginx port to the subdomain
		addr := fmt.Sprintf("localhost:%d", cfg.Nginx.Port)
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
			PrintOK("Nginx → %s: %s\n", svcName, url)
		} else {
			PrintWarn("Nginx → %s: port %d not reachable\n", svcName, cfg.Nginx.Port)
		}
	}
}
