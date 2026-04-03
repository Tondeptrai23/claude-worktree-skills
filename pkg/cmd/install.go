package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

// SkillFiles is set by main.go with the embedded skill files.
var SkillFiles embed.FS

// SkillFilesRoot is the prefix to strip from embedded paths (e.g., "worktree" or "").
var SkillFilesRoot string

func InstallCommand() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "Install worktree skills and CLI into the current project",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Usage: "Overwrite existing files"},
		},
		Action: runInstall,
	}
}

func runInstall(c *cli.Context) error {
	// Must be in a git repo root
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("run this from your project root (no .git directory found)")
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	force := c.Bool("force")
	claudeDir := filepath.Join(projectRoot, ".claude")
	skillsDir := filepath.Join(claudeDir, "skills")
	binDir := filepath.Join(claudeDir, "bin")

	// 1. Extract skill files
	fmt.Println("\033[32m[*]\033[0m Installing skills...")

	err = fs.WalkDir(SkillFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		relPath := path

		targetPath := filepath.Join(skillsDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		if !force {
			if _, err := os.Stat(targetPath); err == nil {
				return nil // skip existing
			}
		}

		data, err := SkillFiles.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0644)
	})
	if err != nil {
		return fmt.Errorf("extracting skills: %w", err)
	}

	fmt.Println("\033[32m[OK]\033[0m Installed skills to", skillsDir)

	// 2. Copy self to .claude/bin/wt
	fmt.Println("\033[32m[*]\033[0m Installing wt CLI...")

	os.MkdirAll(binDir, 0755)
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding self: %w", err)
	}

	wtPath := filepath.Join(binDir, "wt")
	selfData, err := os.ReadFile(selfPath)
	if err != nil {
		return fmt.Errorf("reading self: %w", err)
	}
	if err := os.WriteFile(wtPath, selfData, 0755); err != nil {
		return fmt.Errorf("writing %s: %w", wtPath, err)
	}

	fmt.Println("\033[32m[OK]\033[0m Installed", wtPath)

	// 3. Update settings.local.json
	if err := updateSettings(claudeDir); err != nil {
		fmt.Printf("\033[33m[!]\033[0m Could not update settings: %v\n", err)
	}

	// 4. Update .gitignore
	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	ensureGitignore(gitignorePath, ".claude/bin/")
	ensureGitignore(gitignorePath, ".worktrees/")
	fmt.Println("\033[32m[OK]\033[0m Updated .gitignore")

	// 5. Update CLAUDE.md
	if err := updateClaudeMD(projectRoot); err != nil {
		fmt.Printf("\033[33m[!]\033[0m Could not update CLAUDE.md: %v\n", err)
	}

	fmt.Println()
	fmt.Println("\033[32m[OK]\033[0m Worktree skills installed.")
	fmt.Println()
	fmt.Println("  Skills:", skillsDir)
	fmt.Println("  CLI:   ", wtPath)
	fmt.Println()
	fmt.Println("  Next:   Open Claude Code and run /worktree to bootstrap your project.")
	fmt.Println()
	return nil
}

func updateSettings(claudeDir string) error {
	settingsPath := filepath.Join(claudeDir, "settings.local.json")

	permissions := []string{
		"Read(.claude/skills/worktree/assets/*)",
		"Read(.claude/skills/worktree/references/*)",
		"Bash(.claude/bin/wt:*)",
		"Bash(git worktree:*)",
		"Bash(git branch:*)",
		"Bash(git checkout:*)",
		"Bash(git -C:*)",
	}

	type settingsFile struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}

	var settings settingsFile

	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &settings)
	}

	// Merge permissions (deduplicate)
	existing := make(map[string]bool)
	for _, p := range settings.Permissions.Allow {
		existing[p] = true
	}
	for _, p := range permissions {
		if !existing[p] {
			settings.Permissions.Allow = append(settings.Permissions.Allow, p)
		}
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	os.MkdirAll(claudeDir, 0755)
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return err
	}

	fmt.Println("\033[32m[OK]\033[0m Updated settings.local.json")
	return nil
}

func ensureGitignore(path, pattern string) {
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == pattern {
				return
			}
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, pattern)
}

func updateClaudeMD(projectRoot string) error {
	claudeMD := filepath.Join(projectRoot, "CLAUDE.md")
	marker := ".claude/bin/wt"

	if data, err := os.ReadFile(claudeMD); err == nil {
		if strings.Contains(string(data), marker) {
			return nil // already has it
		}
		f, err := os.OpenFile(claudeMD, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		fmt.Fprintf(f, "\n## Worktree\n\nThe `wt` CLI is at `.claude/bin/wt`. Use `/worktree` to bootstrap worktree config for this project.\n")
		fmt.Println("\033[32m[OK]\033[0m Appended worktree section to CLAUDE.md")
	} else {
		content := "# Project Notes\n\n## Worktree\n\nThe `wt` CLI is at `.claude/bin/wt`. Use `/worktree` to bootstrap worktree config for this project.\n"
		if err := os.WriteFile(claudeMD, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Println("\033[32m[OK]\033[0m Created CLAUDE.md")
	}
	return nil
}
