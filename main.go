package main

import (
	"fmt"
	"log"
	"os"

	"claude-worktree-skill/pkg/cmd"
	"claude-worktree-skill/pkg/config"

	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := &cli.App{
		Name:    "wt",
		Usage:   "Multi-feature worktree management",
		Version: version,
		Commands: []*cli.Command{
			cmd.CreateCommand(),
			cmd.StartCommand(),
			cmd.StopCommand(),
			cmd.DestroyCommand(),
			cmd.StatusCommand(),
			cmd.LogsCommand(),
		},
		Before: func(c *cli.Context) error {
			// Skip config loading for help/version
			if c.NArg() == 0 {
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
