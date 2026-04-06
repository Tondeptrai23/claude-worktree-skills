package db

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/shell"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/template"
)

// Setup creates the database isolation for a slot.
// For "database" isolation, it spins up a Docker container using the configured image.
// For other modes, it runs the user-configured setup command if provided.
func Setup(cfg *config.Config, slot int) error {
	if cfg.Database.Isolation == "database" {
		return createDBContainer(cfg, slot)
	}
	if cfg.Database.Setup == "" {
		return nil
	}
	cmd := template.Resolve(cfg.Database.Setup, "", slot, "", cfg)
	return runShell(cmd)
}

// Teardown removes the database isolation for a slot.
func Teardown(cfg *config.Config, slot int) error {
	if cfg.Database.Isolation == "database" {
		return removeDBContainer(cfg, slot)
	}
	if cfg.Database.Teardown == "" {
		return nil
	}
	cmd := template.Resolve(cfg.Database.Teardown, "", slot, "", cfg)
	return runShell(cmd)
}

func containerName(cfg *config.Config, slot int) string {
	return fmt.Sprintf("%s-db-slot-%d", cfg.ProjectName, slot)
}

func createDBContainer(cfg *config.Config, slot int) error {
	name := containerName(cfg, slot)
	hostPort := cfg.Database.PortBase + slot
	containerPort := cfg.Database.ContainerPort

	args := []string{"run", "-d",
		"--name", name,
		"-p", fmt.Sprintf("%d:%d", hostPort, containerPort),
	}

	// Add user-configured environment variables, resolving templates
	for k, v := range cfg.Database.Env {
		resolved := template.Resolve(v, "", slot, "", cfg)
		args = append(args, "-e", k+"="+resolved)
	}

	args = append(args, cfg.Database.Image)

	cmd := exec.Command("docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting DB container: %w\n%s", err, out)
	}

	// Wait for readiness if a check command is configured
	if cfg.Database.Readiness != "" {
		readinessCmd := template.Resolve(cfg.Database.Readiness, "", slot, "", cfg)
		for i := 0; i < 30; i++ {
			check := exec.Command("docker", "exec", name, "sh", "-c", readinessCmd)
			if check.Run() == nil {
				return nil
			}
			time.Sleep(time.Second)
		}
		return fmt.Errorf("DB container %s did not become ready", name)
	}

	return nil
}

func removeDBContainer(cfg *config.Config, slot int) error {
	name := containerName(cfg, slot)
	exec.Command("docker", "rm", "-f", name).Run()
	return nil
}

// RunSeed executes the seed script against the slot's database, if configured.
func RunSeed(cfg *config.Config, slot int, rootDir string) error {
	if cfg.Database.SeedScript == "" {
		return nil
	}

	scriptPath := filepath.Join(rootDir, cfg.Database.SeedScript)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("seed script not found: %s", scriptPath)
	}

	resolved := template.Resolve(scriptPath, "", slot, "", cfg)
	cmd := shell.Command(resolved)
	cmd.Dir = rootDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("seed script failed: %w\n%s", err, out)
	}
	return nil
}

// RunMigrations runs the configured migration command for each service in the slot.
func RunMigrations(cfg *config.Config, slot int, slotDir string) error {
	for svcName, migration := range cfg.Database.Migrations {
		if migration.Run == "" {
			continue
		}
		svc, ok := cfg.Services[svcName]
		if !ok {
			continue
		}

		workDir := svc.WorkDir(slotDir)
		resolved := template.Resolve(migration.Run, svcName, slot, "", cfg)

		fmt.Printf("\033[32m[*]\033[0m Running %s migration (%s)\n", svcName, migration.Tool)
		cmd := shell.Command(resolved)
		cmd.Dir = workDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("migration for %s failed: %w\n%s", svcName, err, out)
		}
	}
	return nil
}

func runShell(command string) error {
	cmd := shell.Command(command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("db command failed: %w\n%s", err, out)
	}
	return nil
}
