package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"

	"github.com/urfave/cli/v2"
)

func VerifyCommand() *cli.Command {
	return &cli.Command{
		Name:  "verify",
		Usage: "Validate worktree.yml against the actual project on disk",
		Action: runVerify,
	}
}

type verifyResult struct {
	errors   []string
	warnings []string
	info     []string
}

func (r *verifyResult) errorf(format string, args ...interface{}) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *verifyResult) warnf(format string, args ...interface{}) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

func (r *verifyResult) infof(format string, args ...interface{}) {
	r.info = append(r.info, fmt.Sprintf(format, args...))
}

var templateVarRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func runVerify(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	result := &verifyResult{}

	PrintInfo("Verifying worktree.yml...\n")
	fmt.Println()

	// 1. Structural checks
	verifyStructure(cfg, result)

	// 2. Per-service checks
	for svcName, svc := range cfg.Services {
		verifyService(svcName, svc, cfg, rootDir, result)
	}

	// 3. Infrastructure checks
	verifyInfrastructure(cfg, rootDir, result)

	// 4. Cross-service env var audit
	verifyCrossServiceEnvVars(cfg, rootDir, result)

	// Print results
	fmt.Println()
	for _, msg := range result.info {
		PrintOK("%s\n", msg)
	}
	for _, msg := range result.warnings {
		PrintWarn("%s\n", msg)
	}
	for _, msg := range result.errors {
		PrintErr("%s\n", msg)
	}

	fmt.Println()
	fmt.Printf("Summary: %d error(s), %d warning(s)\n", len(result.errors), len(result.warnings))

	if len(result.errors) > 0 {
		return fmt.Errorf("verification failed with %d error(s)", len(result.errors))
	}
	return nil
}

func verifyStructure(cfg *config.Config, r *verifyResult) {
	r.infof("project_name: %s", cfg.ProjectName)
	r.infof("git_topology: %s", cfg.GitTopology)

	if cfg.Nginx.Enabled {
		r.infof("nginx.subdomain_pattern: %s", cfg.Nginx.SubdomainPattern)
	}

	if len(cfg.Services) == 0 {
		r.errorf("no services defined")
	}

	// Monorepo consistency check
	if cfg.GitTopology == "monorepo" {
		for svcName, svc := range cfg.Services {
			if svc.Path != "." {
				r.errorf("service %s: monorepo topology requires path: \".\" (got %q) — use subdir for the service directory", svcName, svc.Path)
			}
			if svc.Subdir == "" {
				r.warnf("service %s: monorepo topology usually needs subdir set (path is \".\" but no subdir)", svcName)
			}
		}
	}
}

func verifyService(svcName string, svc config.Service, cfg *config.Config, rootDir string, r *verifyResult) {
	fmt.Printf("\n--- Service: %s ---\n", svcName)

	// Check git repo exists
	repoDir := filepath.Join(rootDir, svc.Path)
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		r.errorf("service %s: no .git at %s", svcName, repoDir)
	} else {
		r.infof("service %s: .git found at %s", svcName, svc.Path)
	}

	// Check subdir exists
	if svc.Subdir != "" {
		subdirPath := filepath.Join(repoDir, svc.Subdir)
		if _, err := os.Stat(subdirPath); os.IsNotExist(err) {
			r.errorf("service %s: subdir %q does not exist at %s", svcName, svc.Subdir, subdirPath)
		} else {
			r.infof("service %s: subdir %s exists", svcName, svc.Subdir)
		}
	}

	r.infof("service %s: port_base %d", svcName, svc.PortBase)

	// Check dev mode start command
	if mode, ok := svc.Modes["dev"]; ok {
		if mode.Start == "" {
			r.warnf("service %s: modes.dev.start is empty", svcName)
		}
	} else {
		r.warnf("service %s: no dev mode defined", svcName)
	}

	// Check env_overrides template vars reference defined services
	for key, tmpl := range svc.EnvOverrides {
		matches := templateVarRe.FindAllStringSubmatch(tmpl, -1)
		for _, match := range matches {
			varName := strings.TrimSpace(match[1])
			validateTemplateVar(svcName, key, varName, cfg, r)
		}
	}

	// Check env_overrides keys exist in actual .env file
	mainDir := svc.MainDir(rootDir)
	envVars := readEnvKeys(mainDir, svc.EnvFile, svc.EnvSample)
	for key := range svc.EnvOverrides {
		if len(envVars) > 0 {
			if _, exists := envVars[key]; !exists {
				// Not necessarily an error — the override could be adding a new var
				r.warnf("service %s: env_overrides key %q not found in .env (will be appended)", svcName, key)
			}
		}
	}
}

func validateTemplateVar(svcName, envKey, varName string, cfg *config.Config, r *verifyResult) {
	// Known global vars
	switch varName {
	case "slot", "name", "project_name", "self.port", "self.url":
		return
	case "db.host", "db.port", "db.name", "db.user", "db.password", "db.schema":
		return
	}

	// Check svc.port, svc.url references
	parts := strings.SplitN(varName, ".", 2)
	if len(parts) == 2 {
		refSvc := parts[0]
		prop := parts[1]
		if prop == "port" || prop == "url" {
			if _, ok := cfg.Services[refSvc]; !ok {
				r.errorf("service %s: env_overrides.%s references undefined service %q in {{%s}}", svcName, envKey, refSvc, varName)
			}
			return
		}
	}

	r.warnf("service %s: env_overrides.%s has unknown template variable {{%s}}", svcName, envKey, varName)
}

func verifyInfrastructure(cfg *config.Config, rootDir string, r *verifyResult) {
	// We don't have an Infrastructure field in the config struct currently,
	// so skip this if the raw YAML doesn't have it.
	// The config struct would need to be extended for full infrastructure validation.
}

// verifyCrossServiceEnvVars scans .env files for URL values that match another
// service's port_base but are NOT covered by env_overrides. This is the key check
// that catches the VITE_API_BASE_URL=http://localhost:3000 problem.
func verifyCrossServiceEnvVars(cfg *config.Config, rootDir string, r *verifyResult) {
	fmt.Printf("\n--- Cross-service env var audit ---\n")

	// Build port → service name map
	portToService := make(map[string]string)
	for name, svc := range cfg.Services {
		portToService[fmt.Sprintf("%d", svc.PortBase)] = name
	}

	// URL pattern: matches http://localhost:PORT or http://127.0.0.1:PORT
	urlPortRe := regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1):(\d+)`)

	for svcName, svc := range cfg.Services {
		mainDir := svc.MainDir(rootDir)
		envVars := readEnvVars(mainDir, svc.EnvFile, svc.EnvSample)

		for key, value := range envVars {
			// Skip if already in env_overrides
			if _, covered := svc.EnvOverrides[key]; covered {
				continue
			}

			// Check if value contains a URL pointing to another service's port
			matches := urlPortRe.FindStringSubmatch(value)
			if matches == nil {
				continue
			}

			port := matches[1]
			targetSvc, isServicePort := portToService[port]
			if !isServicePort || targetSvc == svcName {
				continue
			}

			// Determine suggested template
			suggested := fmt.Sprintf("{{%s.url}}", targetSvc)

			// Check if it's a browser-consumed variable
			isBrowser := strings.HasPrefix(key, "VITE_") ||
				strings.HasPrefix(key, "NEXT_PUBLIC_") ||
				strings.HasPrefix(key, "REACT_APP_")

			context := "server-consumed"
			if isBrowser {
				context = "browser-consumed"
			}

			r.warnf("service %s: %s=%s (%s) matches %s port_base but is missing from env_overrides\n         suggested: %s: %q",
				svcName, key, value, context, targetSvc, key, suggested)
		}
	}
}

// readEnvKeys reads a .env file and returns a set of key names.
func readEnvKeys(dir, envFile, envSample string) map[string]bool {
	keys := make(map[string]bool)
	for _, name := range []string{envFile, envSample, ".env", ".env.sample", ".env.example"} {
		if name == "" {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.IndexByte(line, '=')
			if idx > 0 {
				keys[line[:idx]] = true
			}
		}
		f.Close()
		return keys // Return on first file found
	}
	return keys
}

// readEnvVars reads a .env file and returns key-value pairs.
func readEnvVars(dir, envFile, envSample string) map[string]string {
	vars := make(map[string]string)
	for _, name := range []string{envFile, envSample, ".env", ".env.sample", ".env.example"} {
		if name == "" {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.IndexByte(line, '=')
			if idx > 0 {
				vars[line[:idx]] = line[idx+1:]
			}
		}
		f.Close()
		return vars // Return on first file found
	}
	return vars
}
