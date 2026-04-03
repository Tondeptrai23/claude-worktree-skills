package envgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-worktree-skill/pkg/config"
	"claude-worktree-skill/pkg/template"
)

// GenerateOverrides writes .env.overrides files for each service in the slot.
// These contain only port/URL/schema overrides — never secrets.
func GenerateOverrides(slot int, featureName string, cfg *config.Config, slotDir string) error {
	for svcName, svc := range cfg.Services {
		workDir := svc.WorkDir(slotDir)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			continue
		}

		var lines []string
		lines = append(lines, fmt.Sprintf("# Worktree slot %d overrides", slot))

		for key, tmpl := range svc.EnvOverrides {
			resolved := template.Resolve(tmpl, svcName, slot, featureName, cfg)
			lines = append(lines, fmt.Sprintf("%s=%s", key, resolved))
		}

		overridePath := filepath.Join(workDir, ".env.overrides")
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(overridePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", overridePath, err)
		}

		fmt.Printf("  [env] Generated %s/.env.overrides (port %d)\n",
			svcName, svc.Port(slot, cfg.PortOffset))
	}

	return nil
}
