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
| **Bootstrap** | `/worktree`, `/worktree bootstrap` | Analyze project, generate `worktree.yml` + nginx scaffolding |
| **Create** | `/worktree create <slot> <name>` | `wt create <slot> <name>` |
| **Start** | `/worktree start <slot>` | `wt start <slot>` |
| **Stop** | `/worktree stop <slot>` | `wt stop <slot>` |
| **Destroy** | `/worktree destroy <slot>` | `wt destroy <slot>` |
| **Status** | `/worktree status` | `wt status` |

If `worktree.yml` doesn't exist and the user requests a non-bootstrap operation, run bootstrap first. If it already exists and user runs bootstrap, ask whether to overwrite.

The `wt` CLI is installed at `.claude/bin/wt`. All slot operations go through it — no shell scripts needed.

**Always cd to the project root** before running `wt` commands. Each `wt` invocation must be a **separate Bash call** using the relative path so permissions match:
```bash
# First Bash call: set working directory
cd "$(git rev-parse --show-toplevel)"

# Subsequent Bash calls: use relative path (standalone, not combined with &&)
.claude/bin/wt create 1 my-feature
```

---

## Bootstrap

### Step 0: Pre-flight

Run `make doctor` if the project has it:
```bash
grep -qE '^doctor:|^check-deps:' Makefile 2>/dev/null && make doctor
```

Otherwise, run `--version` checks for detected tools:
- Always: `git --version`, `docker --version`, `docker compose version`
- If `build.gradle`/`pom.xml` found: `java --version`
- If `package.json` found: `node --version` + package manager (`pnpm`/`yarn`/`npm`)
- If `requirements.txt`/`pyproject.toml` found: `python3 --version`

Check that each service directory has `.git`. If missing, tell user to clone repos first.

Verify `wt` CLI exists at `.claude/bin/wt`. If not, tell user to run the install script first.

### Step 1: Analyze the project

Use Explore agents to gather:

1. **Project structure** — top-level directories, which are services
2. **Git topology** — monorepo (single `.git`) or multi-repo (each service has own `.git`)
3. **Service detection** — `package.json`, `build.gradle`, `requirements.txt`, `go.mod`, `Cargo.toml`, `Dockerfile`
4. **Ports** — read Vite configs, Spring Boot `application.yml`, uvicorn commands, docker-compose `ports:`, `.env.sample`
5. **Environment variables** — which env vars reference other services, where `.env`/`.env.sample` live, which are browser-consumed (`VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`)
6. **Infrastructure** — `docker-compose.yml` for databases, caches, auth servers
7. **Start commands** — `package.json` scripts, direct CLI commands, docker-compose services
8. **CORS** — search backend services for CORS config. See [references/cors-audit.md](references/cors-audit.md) for where to look per framework
9. **Database migrations** — detect migration tools and write the `run` command. See [references/db-isolation.md](references/db-isolation.md)
10. **Private files** — read each service's `.gitignore` (and root `.gitignore`) to find gitignored files that exist on disk (credentials, keys, certs). Common patterns: `*service-account*.json`, `*.pem`, `*.key`, `*.p12`, `*credentials*.json`, `*firebase*.json`
11. **Cross-service env var audit** — for each service, read `.env.sample` or `.env.example` (NEVER read `.env` — it contains secrets). For every variable whose name suggests a URL to another service (e.g., `VITE_API_BASE_URL`, `API_URL`, `BACKEND_URL`), note the **exact key name** and which service it likely points to. Also check browser-consumed prefixes (`VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`). If `.env.sample` doesn't exist, infer from framework conventions and ask the user to confirm.
12. **Existing project skills** — scan `.claude/skills/` for workflow skills (plan, implement, test, commit, create-mr, etc.) to document in `worktree.yml`

### Step 2: Present findings and resolve

**IMPORTANT**: You MUST use the `AskUserQuestion` tool to present findings and ask questions. Do NOT use inline text — the user needs a structured prompt they can respond to. Batch ALL questions into a single `AskUserQuestion` call. Never ask multiple times.

Example `AskUserQuestion` content:

```
Here's what I found:

Services:
  - api/ (Spring Boot, port 8080) — git repo Y
  - web/ (Vite + React, port 3000) — git repo Y
  - worker/ (FastAPI, port 8081) — git repo Y

Git topology: multi-repo (each service has own .git)

CORS:
  - api/: env-driven (ALLOWED_ORIGINS) — will auto-configure Y
  - worker/: env-driven (ALLOWED_ORIGINS) — will auto-configure Y

Database: Hibernate ddl-auto=update, no versioned migrations
  -> Recommending: schema isolation (separate schema per slot)

Private files found (gitignored but exist on disk):
  - be/firebase-service-account.json
  - genai/service-account.json

Questions:
  1. Are these all the services, or did I miss any?
  2. Dev mode (host processes) or Docker mode?
  3. Schema isolation OK, or prefer separate DB containers?
  4. What branch naming convention do you use?
     Examples: feature/{name}, {name}, JIRA-{ticket}-{name}
     (default: feature/{name})
  5. Should I copy the private files listed above to worktrees?
```

**You CAN infer** (don't ask): framework defaults (Spring Boot=8080, Vite=5173), common env patterns (`DATABASE_URL`, `API_URL`), git topology, browser-consumed env vars.

**You MUST ask** (don't guess): which directories are services, unclear port assignments, cross-service URL mappings that aren't obvious, database isolation preference, branch naming convention.

### Step 3: Generate `worktree.yml`

Write the config to the project root (or `.claude/worktree/worktree.yml`). See [references/worktree-schema.md](references/worktree-schema.md) for the full schema.

**Critical rules:**

1. **Monorepo path/subdir** — if `git_topology: monorepo`, every service MUST use `path: "."` with `subdir: <dir>`. Do NOT use `path: "./<dir>"` for monorepos — that tells `wt` to look for `.git` inside the subdirectory, which doesn't exist.
   ```yaml
   # CORRECT for monorepo:
   services:
     backend:
       path: .
       subdir: TLL_backend
   # WRONG for monorepo:
   services:
     backend:
       path: ./TLL_backend    # ← wt will look for TLL_backend/.git and fail
   ```

2. **env_overrides exact key names** — for every cross-service URL variable found in Step 1 item 11, add an entry to `env_overrides` using the **exact key name** from the `.env` file. Do NOT guess or use names from examples.
   ```yaml
   # If .env has VITE_API_BASE_URL=http://localhost:3000
   env_overrides:
     VITE_API_BASE_URL: "{{backend.url}}"    # ← exact key from .env
   # NOT: VITE_API_URL: "{{backend.url}}"    # ← wrong key name, override won't replace
   ```

3. **Browser-consumed URLs** — variables prefixed with `VITE_*`, `NEXT_PUBLIC_*`, or `REACT_APP_*` are consumed by the browser. Use `{{svc.url}}` which resolves to the nginx subdomain when available. Server-consumed URLs (backend-to-backend) use `http://localhost:{{svc.port}}`.

Key things to encode:
- `env_overrides` per service with templates (`{{self.port}}`, `{{svc.url}}`, `{{db.schema}}`)
- `database.setup` / `database.teardown` commands for schema isolation
- `database.image`, `container_port`, `env`, `readiness` for container isolation
- `database.migrations` per service with the `run` command
- `nginx.subdomain_pattern` — default: `{name}.{svc}.localhost`

### Step 4: Scaffold nginx

Generate nginx config files from templates in [assets/](assets/):

```
.claude/worktree/nginx/
  nginx.conf                    # from nginx.conf.template (replace {{nginx_port}}, {{project_name}})
  docker-compose.nginx.yml      # from docker-compose.nginx.yml.template (replace {{nginx_port}}, {{project_name}})
  conf.d/                       # empty dir — wt generates slot configs here
```

Then add to `.gitignore`: `.worktrees/`, `.env.overrides`

For URL resolution in env_overrides, see [references/url-resolution.md](references/url-resolution.md).
For database isolation implementation, see [references/db-isolation.md](references/db-isolation.md).

### Step 5: Verify

Run `wt verify` to validate the generated `worktree.yml` against the actual project:

```bash
.claude/bin/wt verify
```

This checks:
- Service paths resolve to directories with `.git`
- Subdirs exist within repos
- `env_overrides` template vars reference defined services
- **Cross-service URL detection**: the binary internally reads `.env` files (safe — it only outputs key names and URL values, never secrets) and flags `localhost:PORT` URLs that match other services' `port_base` but are missing from `env_overrides`

If `wt verify` reports warnings about missing env_overrides, add the suggested entries to `worktree.yml` and re-run until clean.

**IMPORTANT**: Do NOT read `.env` files yourself — they contain secrets. Let `wt verify` handle the scanning. You should only ever read `.env.sample` or `.env.example`.

---

## Slot Operations

All operations use the `wt` CLI:

```bash
# Create a worktree slot
.claude/bin/wt create <slot> <name> [--services svc1,svc2] [--<svc>-branch <branch>]

# Start services
.claude/bin/wt start <slot> [--services svc1,svc2]

# Stop services
.claude/bin/wt stop <slot>

# Destroy slot
.claude/bin/wt destroy <slot> [--teardown-db]

# Show status
.claude/bin/wt status [slot]

# View logs
.claude/bin/wt logs <slot> [service]
```

Key behaviors:
- **`create`**: creates git worktrees, generates env overrides, installs deps, runs DB setup + seed + migrations, regenerates nginx
- **`create --services`**: only creates worktrees for specified services
- **`start`**: auto-starts nginx if not running (finds available port if default is occupied), merges env files, launches services
- **`destroy --teardown-db`**: also runs DB teardown (removes container or drops schema)

**Shared repo constraint**: if multiple services share a repo (e.g., fe-app and fe-admin in `./fe`), they share one branch per worktree. Warn if user requests different branches for services in the same repo.

See [references/integration.md](references/integration.md) for how other skills/workflows can use these commands.

---

## Related Skills

- **`/worktree-agent`** — spawn agents in fully provisioned worktrees with running services and test URLs

## Notes

- All generated config lives in `.claude/worktree/` — personal dev tooling, gitignored, regenerable via `/worktree bootstrap`
- `.worktrees/` (runtime state) lives at project root, also gitignored
- The `wt` CLI reads `worktree.yml` for all configuration — no hardcoded values
- `.env.overrides` = ports/URLs only (safe for agents). `.env` and `private_files` = secrets (merged at start time, agents should not read)
- Feature names are sanitized for DNS: `TICKET-123` -> `ticket-123.be.localhost`
- Max 3 slots by default, configurable in `worktree.yml`

---

## Manual Recovery

If `wt destroy` fails or the binary is broken, clean up manually:

```bash
# 1. Stop processes
kill $(cat .worktrees/slot-N/.pids/*.pid 2>/dev/null) 2>/dev/null

# 2. Remove git worktree references
# For monorepo (path: .):
git worktree remove --force .worktrees/slot-N

# For multi-repo (path: ./be, ./fe):
git -C be worktree remove --force .worktrees/slot-N/be
git -C fe worktree remove --force .worktrees/slot-N/fe

# 3. Prune any orphaned references
git worktree prune

# 4. Remove the slot directory
rm -rf .worktrees/slot-N

# 5. If using DB schema isolation, drop the schema
# PGPASSWORD=... psql -h localhost -U user -d dbname -c 'DROP SCHEMA IF EXISTS feature_N CASCADE'

# 6. If using DB container isolation, remove the container
# docker rm -f <project>-db-slot-N
```
