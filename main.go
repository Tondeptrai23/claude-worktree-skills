package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/cmd"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"

	"github.com/urfave/cli/v2"
)

var version = "dev"

//go:embed worktree worktree-agent
var embeddedSkills embed.FS

func main() {
	// Make embedded skills available to the install command
	cmd.SkillFiles = embeddedSkills

	app := &cli.App{
		Name:    "wt",
		Usage:   "Multi-feature worktree management",
		Version: version,
		Commands: []*cli.Command{
			cmd.InstallCommand(),
			cmd.CreateCommand(),
			cmd.StartCommand(),
			cmd.StopCommand(),
			cmd.DestroyCommand(),
			cmd.StatusCommand(),
			cmd.LogsCommand(),
			cmd.VerifyCommand(),
		},
		Before: func(c *cli.Context) error {
			// Skip config loading for help/version/install
			if c.NArg() == 0 {
				return nil
			}
			subCmd := c.Args().Get(0)
			if subCmd == "install" || subCmd == "help" {
				return nil
			}

			cfg, rootDir, err := config.Load()
			if err != nil {
				return fmt.Errorf("cannot find worktree.yml: %w", err)
			}

			if c.App.Metadata == nil {
				c.App.Metadata = make(map[string]interface{})
			}
			c.App.Metadata["config"] = cfg
			c.App.Metadata["rootDir"] = rootDir
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
