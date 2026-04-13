package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/process"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/db"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/envgen"
	gitops "github.com/Tondeptrai23/claude-worktree-skills/pkg/git"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/nginx"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/template"

	"github.com/urfave/cli/v2"
)

var branchFlagRe = regexp.MustCompile(`^--(\w[\w-]*)-branch$`)

func CreateCommand() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create a feature worktree slot",
		ArgsUsage: "<slot> <name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "services", Aliases: []string{"s"}, Usage: "Comma-separated service list (default: all)"},
		},
		SkipFlagParsing: false,
		Action:          runCreate,
	}
}

func runCreate(c *cli.Context) error {
	cfg := c.App.Metadata["config"].(*config.Config)
	rootDir := c.App.Metadata["rootDir"].(string)

	if c.NArg() < 2 {
		return fmt.Errorf("usage: wt create <slot> <name> [--services be,fe] [--<svc>-branch branch]")
	}

	slotNum, err := parseSlot(c.Args().Get(0), cfg.MaxSlots)
	if err != nil {
		return err
	}

	name := sanitizeName(c.Args().Get(1))
	servicesFilter := c.String("services")

	// Parse dynamic --<svc>-branch flags from os.Args
	branchOverrides := parseBranchFlags(os.Args)

	// Determine which services to create and their branches
	branches := resolveBranches(cfg, name, servicesFilter, branchOverrides)
	if len(branches) == 0 {
		return fmt.Errorf("no services selected")
	}

	slotDir := config.SlotDir(rootDir, slotNum)
	if _, err := os.Stat(slotDir); err == nil {
		return fmt.Errorf("slot %d already in use — run 'wt destroy %d' first", slotNum, slotNum)
	}

	PrintInfo("Creating feature slot %d: '%s'\n", slotNum, name)
	os.MkdirAll(slotDir, 0755)

	// Create git worktrees — group by repo to avoid duplicates
	createdRepos := make(map[string]bool)
	meta := slot.NewMeta(slotNum, name, cfg.DefaultMode)

	for svcName, branch := range branches {
		svc := cfg.Services[svcName]
		repoKey := svc.RepoKey()

		if createdRepos[repoKey] {
			// Repo already has a worktree — just register the service
			meta.Services[svcName] = slot.SlotServiceMeta{
				Branch:  branch,
				Port:    svc.Port(slotNum, cfg.PortOffset),
				RepoKey: repoKey,
			}
			continue
		}

		repoDir := filepath.Join(rootDir, svc.Path)
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
			PrintWarn("Skipping %s: no git repo at %s\n", svcName, repoDir)
			continue
		}

		targetDir := filepath.Join(slotDir, repoKey)
		PrintInfo("Creating %s/ worktree on branch '%s'\n", repoKey, branch)

		if err := gitops.CreateWorktree(repoDir, targetDir, branch); err != nil {
			return fmt.Errorf("creating worktree for %s: %w", repoKey, err)
		}

		// Write git excludes for runtime files so they don't appear in git status
		if err := gitops.WriteWorktreeExcludes(targetDir); err != nil {
			PrintWarn("Could not write git excludes for %s: %v\n", repoKey, err)
		}

		createdRepos[repoKey] = true
		meta.Services[svcName] = slot.SlotServiceMeta{
			Branch:  branch,
			Port:    svc.Port(slotNum, cfg.PortOffset),
			RepoKey: repoKey,
		}
	}

	if len(meta.Services) == 0 {
		os.Remove(slotDir)
		return fmt.Errorf("no worktrees created — check that service repos exist")
	}

	// Write metadata
	if err := meta.Write(slotDir); err != nil {
		return fmt.Errorf("writing slot metadata: %w", err)
	}

	// Generate env overrides
	PrintInfo("Generating environment override files\n")
	if err := envgen.GenerateOverrides(slotNum, name, cfg, slotDir); err != nil {
		return fmt.Errorf("generating env overrides: %w", err)
	}

	// Install dependencies (before DB setup — migration tools may come from install)
	installedRepos := make(map[string]bool)
	for svcName := range meta.Services {
		svc := cfg.Services[svcName]
		repoKey := svc.RepoKey()

		if installedRepos[repoKey] {
			continue
		}

		mode, ok := svc.Modes[cfg.DefaultMode]
		if !ok || mode.Install == "" {
			continue
		}

		workDir := svc.WorkDir(slotDir)
		PrintInfo("Installing dependencies for %s\n", svcName)

		installCmd := template.Resolve(mode.Install, svcName, slotNum, name, cfg)
		cmd := process.Command(installCmd)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			PrintWarn("Install failed for %s: %s\n", svcName, string(out))
		}

		installedRepos[repoKey] = true
	}

	// Run database setup → seed → migrations
	if cfg.Database.Isolation != "none" && cfg.Database.Isolation != "" {
		PrintInfo("Running database setup for slot %d\n", slotNum)
		if err := db.Setup(cfg, slotNum); err != nil {
			PrintWarn("Database setup failed: %v\n", err)
		} else {
			if err := db.RunSeed(cfg, slotNum, rootDir); err != nil {
				PrintWarn("Seed script failed: %v\n", err)
			}
			if err := db.RunMigrations(cfg, slotNum, slotDir); err != nil {
				PrintWarn("Migration failed: %v\n", err)
			}
		}
	}

	// Regenerate nginx
	nginx.Generate(cfg, rootDir)
	nginx.Reload()

	// Print summary
	svcNames := make([]string, 0, len(meta.Services))
	for name := range meta.Services {
		svcNames = append(svcNames, name)
	}

	fmt.Println()
	PrintOK("Feature slot %d created: '%s'\n", slotNum, name)
	fmt.Printf("  Services: %s\n", strings.Join(svcNames, ", "))
	fmt.Printf("  Path: %s\n", slotDir)

	if cfg.Nginx.Enabled {
		fmt.Println("  URLs:")
		for svcName := range meta.Services {
			url := template.Resolve("{{"+svcName+".url}}", svcName, slotNum, name, cfg)
			fmt.Printf("    %s: %s\n", svcName, url)
		}
	}

	fmt.Printf("\n  Next: wt start %d\n", slotNum)
	return nil
}

func parseBranchFlags(args []string) map[string]string {
	result := make(map[string]string)
	for i := 0; i < len(args)-1; i++ {
		matches := branchFlagRe.FindStringSubmatch(args[i])
		if matches != nil {
			result[matches[1]] = args[i+1]
			i++ // skip next arg (the branch value)
		}
	}
	return result
}

func resolveBranches(cfg *config.Config, name, servicesFilter string, branchOverrides map[string]string) map[string]string {
	branches := make(map[string]string)
	defaultBranch := strings.ReplaceAll(cfg.BranchPrefix, "{name}", name)

	if servicesFilter != "" {
		// Match by exact service name OR by repo key
		filterTokens := make(map[string]bool)
		for _, f := range strings.Split(servicesFilter, ",") {
			filterTokens[strings.TrimSpace(f)] = true
		}
		for svcName, svc := range cfg.Services {
			if filterTokens[svcName] || filterTokens[svc.RepoKey()] {
				if b, ok := branchOverrides[svcName]; ok {
					branches[svcName] = b
				} else {
					branches[svcName] = defaultBranch
				}
			}
		}
	} else if len(branchOverrides) > 0 {
		// Only services with explicit branch flags
		for svc, branch := range branchOverrides {
			if _, ok := cfg.Services[svc]; ok {
				branches[svc] = branch
			}
		}
	} else {
		// All services
		for svc := range cfg.Services {
			branches[svc] = defaultBranch
		}
	}

	return branches
}

var nonDNSChars = regexp.MustCompile(`[^a-z0-9-]+`)
var multiHyphen = regexp.MustCompile(`-{2,}`)

// sanitizeName normalizes a feature name for use in DNS subdomains and branch names.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = nonDNSChars.ReplaceAllString(name, "-")
	name = multiHyphen.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}
