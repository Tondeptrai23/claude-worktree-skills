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

Run from your project root:

```bash
curl -sSL https://raw.githubusercontent.com/Tondeptrai23/claude-worktree-skills/main/install.sh | bash
```

Or clone and run locally:

```bash
git clone https://github.com/Tondeptrai23/claude-worktree-skills.git /tmp/claude-worktree-skills
/tmp/claude-worktree-skills/install.sh
```

This installs skills to `.claude/skills/`, builds the `wt` CLI to `.claude/bin/`, and updates `.gitignore`.

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

## Requirements

- bash 4.2+ (macOS: `brew install bash`)
- git, docker, docker compose
- Project-specific: node/pnpm, java, python, etc.

## File Structure

```
worktree/
├── SKILL.md                    # Bootstrap skill
├── assets/                     # Script templates
│   ├── feature-worktree.sh.template
│   ├── generate-env.sh.template
│   ├── merge-env.sh.template
│   ├── nginx-gen.sh.template
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
```
