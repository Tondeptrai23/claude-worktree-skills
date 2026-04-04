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

Bootstrap uses three agents: an **Explore agent** to detect project structure (handles ambiguity), a **Generation agent** to produce correct YAML (clean context, focused rules), and **you as the orchestrator** in between.

### Step 0: Pre-flight

Verify `wt` CLI exists at `.claude/bin/wt`. If not, tell user to run the install script.

Run basic tool checks:
- `git --version`, `docker --version`, `docker compose version`
- If `package.json` found: `node --version`
- If `requirements.txt`/`pyproject.toml` found: `python3 --version`

### Step 1: Explore the project

Spawn an **Explore agent** (`subagent_type: Explore`, thoroughness: `very thorough`) with this prompt:

```
Analyze this project for multi-feature worktree setup. Report structured findings.

## What to detect

1. **Git topology** — single .git at project root (monorepo) or each service has own .git (multi-repo)?
2. **Services** — which directories are services? Look for: package.json, build.gradle, pom.xml, requirements.txt, pyproject.toml, go.mod, Cargo.toml, Dockerfile
3. **Ports** — read Vite configs, Spring Boot application.yml, uvicorn commands, docker-compose ports:, .env.sample
4. **Start commands** — package.json scripts, Gradle tasks, uvicorn commands
5. **Env files** — where .env.sample/.env.example live. Read them for variable structure.
6. **Cross-service env vars** — in .env.sample, find variables whose values are URLs containing localhost:PORT where PORT matches another service's port. Note the EXACT key name and which service it points to. Check browser-consumed prefixes (VITE_*, NEXT_PUBLIC_*, REACT_APP_*).
7. **CORS** — search backend services for CORS config (env-driven? hardcoded? which env var?)
8. **Database** — docker-compose files for infrastructure, migration tools (Flyway, Alembic, TypeORM, Prisma, etc.)
9. **Private files** — gitignored files that exist on disk (credentials, keys, certs). Check .gitignore for patterns like *service-account*.json, *.pem, *.key, *credentials*.json
10. **Infrastructure** — docker-compose services for databases, caches, auth servers. Note the compose file path relative to project root.

## IMPORTANT
- Read .env.sample or .env.example — NEVER read .env (contains secrets)
- Do NOT generate any YAML config — just report findings
- If something is ambiguous, say so and explain what you found

## Report Format

### Git Topology
type: monorepo | multi-repo

### Services
For each service:
- name: <suggested key, e.g., "backend", "frontend">
  directory: <relative path>
  has_git: true/false
  framework: <e.g., NestJS, Vite+React, Spring Boot, FastAPI>
  port_base: <default port number>
  port_source: <where you found it>
  start_command: <dev start command>
  install_command: <or "none">
  env_file: <which env file exists: .env.sample / .env.example / none>
  env_loader: <spring-dotenv / dotenv / vite / none>
  port_env: <env var that controls port, or "none">

### Cross-Service Env Vars
For each service with cross-service references:
- service: <name>
  file_scanned: <path>
  variables:
    - key: <EXACT key name>
      target_service: <which service, based on port>
      browser_consumed: true/false

### CORS
For each backend service:
- service: <name>
  type: env-driven | hardcoded | open | none
  env_var: <if env-driven>
  config_file: <path>

### Database
infrastructure_compose: <path relative to project root>
infrastructure_services: [list]
migrations:
  - service: <name>
    tool: <tool name>
    location: <migration files directory>

### Private Files
<list>

### Notes
<anything ambiguous or unusual>
```

Capture the Explore agent's output — you'll pass it to both the user and the Generation agent.

### Step 2: Present findings and resolve

**IMPORTANT**: Use `AskUserQuestion` to present findings and ask questions. Batch ALL questions into a single call.

Summarize the Explore agent's findings for the user, then ask:

1. Are these all the services, or did I miss any?
2. Dev mode (host processes) or Docker mode?
3. Schema isolation OK, or prefer separate DB containers?
4. What naming convention do you use for features/branches?
   This determines both branch names AND nginx subdomains.

   a) feature/{name}  (default)
      `wt create 1 auth-redesign` → branch: feature/auth-redesign, URLs: auth-redesign-api.localhost

   b) {name}  (plain)
      `wt create 1 auth-redesign` → branch: auth-redesign, URLs: auth-redesign-api.localhost

   c) JIRA-{name}  (ticket ID)
      `wt create 1 123` → branch: JIRA-123, URLs: 123-api.localhost

   d) Custom: ___

5. Should I copy the private files listed above to worktrees?

**You CAN infer** (don't ask): framework defaults, common env patterns, git topology, browser-consumed env vars.
**You MUST ask** (don't guess): which directories are services, unclear port assignments, database isolation, naming convention.

### Step 3: Generate `worktree.yml`

Spawn a **Generation agent** (`subagent_type: general-purpose`) with the prompt below. Fill in `{exploration_findings}` and `{user_answers}` from Steps 1-2.

```
Generate a worktree.yml config file for this project. Write it to `.claude/worktree/worktree.yml`.

## Project Findings

{exploration_findings}

## User's Answers

{user_answers}

## Schema Reference

Read the full schema at `.claude/skills/worktree/references/worktree-schema.md` for the YAML structure and template variable reference.

Also read:
- `.claude/skills/worktree/references/url-resolution.md` for how {{svc.url}} resolves
- `.claude/skills/worktree/references/db-isolation.md` for database isolation config
- `.claude/skills/worktree/references/cors-audit.md` for CORS env_overrides

## CRITICAL RULES

1. **Monorepo path/subdir** — if monorepo, every service MUST use `path: "."` with `subdir: <dir>`.
   Do NOT use `path: "./<dir>"` — wt will look for .git inside the subdirectory and fail.
   ```yaml
   # CORRECT for monorepo:
   backend:
     path: .
     subdir: TLL_backend
   # WRONG — wt looks for TLL_backend/.git:
   backend:
     path: ./TLL_backend
   ```

2. **env_overrides exact key names** — for every cross-service URL variable in the findings, add an entry using the EXACT key name. Do NOT rename or guess keys.
   ```yaml
   # Findings say: VITE_API_BASE_URL → backend
   env_overrides:
     VITE_API_BASE_URL: "{{backend.url}}"
   # NOT: VITE_API_URL: "{{backend.url}}"
   ```

3. **Browser vs server URLs** — VITE_*, NEXT_PUBLIC_*, REACT_APP_* are browser-consumed → use `{{svc.url}}` (resolves to nginx subdomain). All other URL vars are server-consumed → use `http://localhost:{{svc.port}}`.

4. **Subdomain pattern must include {name}** — default: `{name}-{svc}.localhost`. The {name} is what users pass to `wt create` (feature name or ticket ID). Do NOT use {slot} instead of {name}.

5. **Every service needs port override** — every service must have its port env var in env_overrides (e.g., `PORT: "{{self.port}}"`) so slots get unique ports.

6. **Infrastructure compose_file** — must be the path relative to the project root, not relative to a service directory.

## Output

Write the complete `.claude/worktree/worktree.yml` file using the Write tool. Include brief comments for non-obvious choices.
```

### Step 4: Verify and fix loop

Run `wt verify` to validate the generated config:

```bash
.claude/bin/wt verify
```

This checks service paths, subdirs, template vars, and — critically — **scans `.env` files internally** (safe: only outputs key names and URL values, never secrets) to catch cross-service URLs missing from env_overrides.

If `wt verify` reports errors or warnings, use `SendMessage` to the Generation agent with the verify output. The agent fixes `worktree.yml` in its preserved context. Repeat until clean.

**IMPORTANT**: Do NOT read `.env` files yourself — they contain secrets. Let `wt verify` handle the scanning.

### Step 5: Scaffold nginx

Generate nginx config files from templates in [assets/](assets/):

```
.claude/worktree/nginx/
  nginx.conf                    # from nginx.conf.template (replace {{nginx_port}}, {{project_name}})
  docker-compose.nginx.yml      # from docker-compose.nginx.yml.template (replace {{nginx_port}}, {{project_name}})
  conf.d/                       # empty dir — wt generates slot configs here
```

Then add to `.gitignore`: `.worktrees/`, `.env.overrides`

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
