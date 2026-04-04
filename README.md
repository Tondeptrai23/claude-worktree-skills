# Claude Code Worktree Skills

Two Claude Code skills for managing parallel feature development using git worktrees with isolated ports, nginx reverse proxy routing, and per-slot environment files.

## Skills

### `/worktree` — Bootstrap & Manage

Analyzes any project and generates worktree infrastructure: config, scripts, nginx, and environment management.

- Detects services, ports, frameworks, git topology
- Audits CORS and database migrations
- Generates `worktree.yml` config and bash scripts
- Supports partial slots (`--services be,fe` to only create what you need)

### `/worktree-agent` — Feature Development with Live Testing

Spawns a Claude agent in a fully provisioned worktree with running services and test URLs.

```
/worktree-agent "implement auth redesign" --services be,fe
```

Creates worktree, starts services, provides nginx-routed test URLs (`http://f1.localhost`, `http://f1-api.localhost`), then runs the agent with full context.

## Installation

Requires Go 1.23+. From your **project root**:

```bash
go run github.com/Tondeptrai23/claude-worktree-skills@main install
```

**To update** (bypasses Go's module proxy cache to get the latest):

```bash
GONOPROXY=github.com/Tondeptrai23/* go run github.com/Tondeptrai23/claude-worktree-skills@main install
```

This builds the `wt` CLI and installs skills + binary into your project. Re-running cleanly replaces old files.

## How It Works

1. Run `/worktree` on any project to bootstrap (one-time setup)
2. Run `/worktree-agent "task"` to implement features in isolated environments
3. Each slot gets: dedicated ports, nginx subdomains, isolated DB schema, auto-merged env files

### Port Scheme

Each slot gets ports offset by 100 from the base:

| Service | Slot 0 | Slot 1 | Slot 2 |
|---------|--------|--------|--------|
| Backend | 8080 | 8180 | 8280 |
| Frontend | 3000 | 3100 | 3200 |

### Environment Security

- `.env.overrides` — ports/URLs only, safe for agents to read
- `.env` — secrets, generated at start time from main checkout, agents should not read
- `.env.sample` — variable names and descriptions, safe to read

## Building from Source

```bash
# Build the wt binary
go build -o wt .

# Build with version tag
go build -ldflags "-X main.version=1.2.3" -o wt .

# Run directly without building
go run . <command>

# Install into a project for local testing (from this repo root)
go run . install
```

The embedded skill files (`worktree/`, `worktree-agent/`) are baked into the binary at build time via `//go:embed`, so the binary is self-contained.

## Requirements

- Go 1.23+ (to build the `wt` CLI)
- git, docker, docker compose
- Project-specific: node/pnpm, java, python, etc.

## File Structure

```
worktree/
├── SKILL.md                    # Bootstrap skill
├── assets/                     # Nginx scaffolding templates
│   ├── nginx.conf.template
│   └── docker-compose.nginx.yml.template
└── references/                 # Reference docs
    ├── worktree-schema.md      # worktree.yml schema
    ├── cors-audit.md           # CORS audit guide
    ├── db-isolation.md         # DB isolation modes
    ├── url-resolution.md       # URL template resolution
    └── integration.md          # How to integrate with other skills

worktree-agent/
└── SKILL.md                    # Agent spawning skill

pkg/                            # Go CLI source (wt)
main.go
```
