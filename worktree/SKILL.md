---
name: worktree
description: "Bootstrap and manage multi-feature git worktree workflows for any project. Creates isolated worktrees with dedicated ports, nginx reverse proxy routing, and per-slot environment files. Use when setting up parallel feature development, managing worktree slots, or bootstrapping worktree config for a new project."
user_invocable: true
---

# Multi-Feature Worktree Manager

Manage parallel feature development using git worktrees with isolated ports and nginx reverse proxy routing. Works with any project structure.

## Commands

| Intent | Trigger | Action |
|--------|---------|--------|
| **Bootstrap** | `/worktree`, `/worktree bootstrap` | Analyze project, generate `worktree.yml` + scripts |
| **Create** | `/worktree create <slot> <name>` | Create worktree slot |
| **Start** | `/worktree start <slot>` | Start services in slot |
| **Stop** | `/worktree stop <slot>` | Stop services |
| **Destroy** | `/worktree destroy <slot>` | Remove worktree slot |
| **Status** | `/worktree status` | Show all active slots |

If `worktree.yml` doesn't exist and the user requests a non-bootstrap operation, run bootstrap first. If it already exists and user runs bootstrap, ask whether to overwrite.

---

## Bootstrap

### Step 0: Pre-flight

Run `make doctor` if the project has it:
```bash
grep -qE '^doctor:|^check-deps:' Makefile 2>/dev/null && make doctor
```

Otherwise, run `--version` checks for detected tools:
- Always: `git --version`, `docker --version`, `docker compose version`, `bash --version`
- If `build.gradle`/`pom.xml` found: `java --version`
- If `package.json` found: `node --version` + package manager (`pnpm`/`yarn`/`npm`)
- If `requirements.txt`/`pyproject.toml` found: `python3 --version`

Check that each service directory has `.git`. If missing, tell user to clone repos first.

If a critical tool is missing, stop. Otherwise proceed.

### Step 1: Analyze the project

Use Explore agents to gather:

1. **Project structure** — top-level directories, which are services
2. **Git topology** — monorepo (single `.git`) or multi-repo (each service has own `.git`)
3. **Service detection** — `package.json`, `build.gradle`, `requirements.txt`, `go.mod`, `Cargo.toml`, `Dockerfile`
4. **Ports** — read Vite configs, Spring Boot `application.yml`, uvicorn commands, docker-compose `ports:`, `.env.sample`
5. **Environment variables** — which env vars reference other services, where `.env`/`.env.sample` live, which are browser-consumed (`VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`)
6. **Infrastructure** — `docker-compose.yml` for databases, caches, auth servers
7. **Start commands** — Makefile targets, `package.json` scripts, direct CLI commands
8. **CORS** — search backend services for CORS config. See [references/cors-audit.md](references/cors-audit.md) for where to look per framework
9. **Database migrations** — detect Flyway, Alembic, Prisma, Django migrations, Hibernate ddl-auto. See [references/db-isolation.md](references/db-isolation.md)
10. **Private files** — read each service's `.gitignore` (and root `.gitignore`) to find gitignored files that exist on disk (credentials, keys, certs). Common patterns: `*service-account*.json`, `*.pem`, `*.key`, `*.p12`, `*credentials*.json`, `*firebase*.json`. List any matches found — these need to be copied to worktrees at start time.

### Step 2: Present findings and resolve

Present ALL findings in ONE `AskUserQuestion` call. Batch everything — don't ask multiple times:

```
Here's what I found:

Services:
  - api/ (Spring Boot, port 8080) — git repo ✓
  - web/ (Vite + React, port 3000) — git repo ✓
  - worker/ (FastAPI, port 8081) — git repo ✓

Git topology: multi-repo (each service has own .git)

CORS:
  - api/: env-driven (ALLOWED_ORIGINS) — will auto-configure ✓
  - worker/: env-driven (ALLOWED_ORIGINS) — will auto-configure ✓

Database: Hibernate ddl-auto=update, no versioned migrations
  → Recommending: schema isolation (separate schema per slot)

Concerns:
  - OAuth redirect URIs may need worktree origins added

Private files found (gitignored but exist on disk):
  - be/firebase-service-account.json
  - genai/service-account.json

Questions:
  1. Are these all the services, or did I miss any?
  2. Dev mode (host processes) or Docker mode?
  3. Schema isolation OK, or prefer separate DB containers?
  4. What branch naming convention do you use?
     Examples: feature/{name}, {name}, JIRA-{ticket}-{name}, BIZ-{ticket}-{name}
     (default: feature/{name})
  5. Should I copy the private files listed above to worktrees?
     (They'll be copied at start time alongside .env — agents won't see them)
```

The branch prefix is stored in `worktree.yml` as `branch_prefix` and used as the default when `--*-branch` is not explicitly passed to `create`. For Jira-based workflows, the slot name typically IS the ticket number (e.g., `make feature-create SLOT=1 NAME=BIZ-123`).

**You CAN infer** (don't ask): framework defaults (Spring Boot=8080, Vite=5173), common env patterns (`DATABASE_URL`, `API_URL`), git topology, browser-consumed env vars.

**You MUST ask** (don't guess): which directories are services, unclear port assignments, cross-service URL mappings that aren't obvious, database isolation preference, branch naming convention.

### Step 3: Generate `worktree.yml`

Write the config to the project root. See [references/worktree-schema.md](references/worktree-schema.md) for the full schema.

Key things to encode:
- `env_overrides` per service with `# browser` comments for browser-consumed URLs
- `cors` section documenting audit results
- `database.migrations` per service
- `nginx.subdomains` mapping

### Step 4: Generate scripts

All generated files go in `.claude/worktree/` — this is personal dev tooling, not shared project code. It's gitignored and regenerable via `/worktree bootstrap`.

Check if `.claude/worktree/scripts/feature-worktree.sh` already exists. If it does, skip — just regenerate `worktree.yml`. Only generate scripts for fresh bootstraps.

Use [assets/](assets/) as templates. Replace `REPLACE` marker blocks with values from config:

```
.claude/worktree/
├── worktree.yml
├── scripts/
│   ├── feature-worktree.sh
│   ├── generate-env.sh
│   ├── merge-env.sh
│   └── nginx-gen.sh
└── nginx/
    ├── nginx.conf
    ├── docker-compose.nginx.yml
    └── conf.d/
```

Then add to `.gitignore`: `.worktrees/`, `.env.overrides`, `.claude/worktree/`

**Ask the user**: "Do you want `feature-*` targets added to your Makefile? This modifies a shared project file. If no, you can run scripts directly via `.claude/worktree/scripts/feature-worktree.sh`."

If yes, add Makefile targets that delegate to `.claude/worktree/scripts/`. If no, skip — the scripts work standalone.

For URL resolution in env_overrides, see [references/url-resolution.md](references/url-resolution.md).
For database isolation implementation, see [references/db-isolation.md](references/db-isolation.md).

macOS: use `ports: ["80:80"]` instead of `network_mode: host` for nginx. Use awk instead of sed for .env manipulation.

---

## Slot Operations

Run the generated scripts directly or via Makefile targets (if user opted in):

```bash
# Direct (always works)
.claude/worktree/scripts/feature-worktree.sh create <slot> <name> [--services svc1,svc2] [--<svc>-branch <branch>]
.claude/worktree/scripts/feature-worktree.sh start <slot> [--services svc1,svc2]
.claude/worktree/scripts/feature-worktree.sh stop <slot>
.claude/worktree/scripts/feature-worktree.sh destroy <slot> [--drop-schema]
.claude/worktree/scripts/feature-worktree.sh status

# Via Makefile (if targets were added)
make feature-create SLOT=1 NAME=my-feature SERVICES=be,fe
make feature-start SLOT=1
```

`--services` on both `create` and `start` controls which services are affected (default: all).

Key behaviors the generated scripts must implement:
- **`create --services`**: only creates git worktrees, env overrides, and installs deps for specified services. Skips DB schema creation if no backend service is in the slot.
- **`start`**: auto-starts nginx if not running. Runs `merge-env.sh` before launching services.
- **`start --services`**: only starts specified services (but still merges env for all services in the slot).

**Shared repo constraint**: if multiple services share a repo (e.g., fe-app and fe-admin in `./fe`), they share one branch per worktree. Warn if user requests different branches for services in the same repo.

See [references/integration.md](references/integration.md) for how other skills/workflows can use these scripts.

---

## Related Skills

- **`/worktree-agent`** — spawn agents in fully provisioned worktrees with running services and test URLs

## Notes

- All generated files live in `.claude/worktree/` — personal dev tooling, gitignored, regenerable via `/worktree bootstrap`
- `.worktrees/` (runtime state) lives at project root, also gitignored
- Makefile targets are optional — user chooses during bootstrap whether to modify the shared Makefile
- Scripts require bash 4.2+ (macOS: `brew install bash`, Windows: use WSL/Git Bash)
- Templates use `REPLACE` markers, not a template engine — Claude fills in real values
- `.env.overrides` = ports/URLs only (safe for agents). `.env` and `private_files` = secrets (generated/copied at start time by `merge-env.sh`, agents should not read)
- Max 3 slots by default, configurable in `worktree.yml`

### Platform support

Scripts use POSIX-compatible commands (`date '+%Y...'`, `df -k`, `awk`). Platform-specific handling:
- **Linux**: `network_mode: host` for nginx, `proxy_pass http://localhost:...`
- **macOS**: `ports: ["80:80"]` for nginx, `proxy_pass http://host.docker.internal:...`, bash 4.2+ via brew
- **Windows**: requires WSL or Git Bash. Docker Desktop with WSL2 backend recommended
