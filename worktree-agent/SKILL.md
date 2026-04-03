---
name: worktree-agent
description: "Spawn a Claude agent in a fully isolated worktree with running services, nginx routing, and test URLs. Use when implementing a feature that needs a testable dev environment, or when running multiple features in parallel. Requires worktree.yml to exist (run /worktree bootstrap first)."
user_invocable: true
---

# Worktree Agent — Feature Development with Live Testing

Spawn a Claude agent in a fully provisioned worktree: git isolation, dedicated ports, running services, nginx routing, and test URLs. The agent can implement AND test against live services.

## Usage

```
/worktree-agent "implement the new auth flow"
/worktree-agent "refactor payment service" --slot 2 --services be,genai
/worktree-agent "fix login bug on issue #123" --slot 1 --services be,fe --keep
```

## Arguments

Parse from the user's slash command:

| Argument | Required | Default | Description |
|----------|----------|---------|-------------|
| prompt | yes | — | The implementation task for the agent |
| `--slot N` | no | next free slot | Which slot (1-max_slots) to use |
| `--services x,y` | no | all services | Which services to create/start |
| `--branch name` | no | `feature/<slug>` | Branch name for the worktree |
| `--keep` | no | false | Keep worktree running after agent finishes |
| `--background` | no | false | Run agent in background |

## Workflow

### Step 0: Pre-flight checks

Before doing anything, verify:

1. **Locate worktree config and scripts.** Search for `worktree.yml` and `feature-worktree.sh`:
   - Check `.claude/worktree/worktree.yml` (standard location)
   - Check project root `worktree.yml` (legacy)
   - Find `feature-worktree.sh` via: `find . -name feature-worktree.sh -not -path '*/.worktrees/*' | head -1`
   - If neither found: tell user "Run `/worktree bootstrap` first." and stop.
   - Store the script path as `$SCRIPTS_DIR` for all subsequent commands.

2. **Read `worktree.yml`** to get: `max_slots`, service definitions, port scheme, DB isolation mode, branch prefix.

3. **Slot allocation** — if `--slot` not specified, find the first free slot:
   ```bash
   for n in $(seq 1 $MAX_SLOTS); do
       [[ ! -d .worktrees/slot-${n} ]] && echo "$n" && break
   done
   ```
   If all slots are occupied, tell the user which slots exist and ask them to destroy one.

4. **Quick health checks** (fail fast, don't waste time creating a worktree that can't start):
   - Disk space: `df --output=avail . | tail -1` — warn if < 5GB
   - Port conflicts: for each service in the slot, check if the port is already in use:
     ```bash
     lsof -i :PORT -sTCP:LISTEN 2>/dev/null
     ```
     If occupied, report which process and suggest a different slot.
   - Infrastructure: if `database.isolation` is `schema` or `none`, check the DB is reachable:
     ```bash
     pg_isready -h HOST -p PORT 2>/dev/null
     ```
   - Branch: check if the branch is already checked out in another worktree:
     ```bash
     git -C SERVICE_DIR worktree list | grep BRANCH
     ```

5. **Staleness check** — compare `.env.sample` mtime against `worktree.yml` mtime. If any .env.sample is newer, warn: "Environment config may have changed since bootstrap. Consider re-running `/worktree bootstrap`."

6. **`.env` file access** — the create script copies `.env` files from the main checkout into the worktree. Verify access:
   ```bash
   # Check that .env files exist and are readable for each service
   for svc_dir in be fe/app fe/presentation fe/admin genai; do
       [[ -f "$svc_dir/.env" ]] || [[ -f "$svc_dir/.env.sample" ]] || echo "WARN: no .env for $svc_dir"
   done
   ```
   If `.env` files are missing (only `.env.sample` exists), the script will fall back to `.env.sample` — but the resulting worktree will have placeholder/empty values for secrets (API keys, etc.). In that case, warn the user:
   "Service {X} has no `.env` file (only `.env.sample`). The worktree will use sample values — API keys, secrets, and credentials will be empty. The agent can implement code but services requiring real credentials won't function."

7. **Docker running** — `docker info >/dev/null 2>&1`. Required if `database.isolation: database` or nginx is used.

8. **Nginx running** (if enabled) — `docker ps | grep feature-router`. If not running, suggest: `make feature-nginx-start`

If any critical check fails, report the issue and stop. Do NOT proceed with a broken setup.

**Note**: Tool versions, platform compatibility, and DNS resolution are validated during `/worktree bootstrap`. If the user hits those issues here, tell them to re-run `/worktree bootstrap` which includes comprehensive pre-flight checks.

### Step 1: Create the worktree

Run the project's `feature-worktree.sh create` script. Pass `--services` to only create the services needed for the task:

```bash
$SCRIPTS_DIR/feature-worktree.sh create $SLOT $FEATURE_NAME --services $SERVICES
```

This handles: git worktree creation, env overrides, dependency install, DB schema (if backend is included), nginx config. Only creates worktrees for the specified services — saves disk and time.

If the script fails, report the error and clean up.

### Step 2: Start services

```bash
$SCRIPTS_DIR/feature-worktree.sh start $SLOT --services $SERVICES
```

This auto-starts nginx if not running, merges env files (secrets + overrides), then launches the services.

### Step 3: Wait for services to be healthy

For each started service, poll its health endpoint (or just check the port is listening):

```bash
for attempt in $(seq 1 30); do
    curl -sf http://localhost:$PORT/health > /dev/null 2>&1 && break
    # or: lsof -i :$PORT -sTCP:LISTEN > /dev/null 2>&1 && break
    sleep 2
done
```

If a service doesn't come up within 60 seconds, check its log:
```bash
tail -20 .worktrees/slot-$SLOT/.logs/$SERVICE.log
```
Report the error and ask the user how to proceed.

### Step 4: Spawn the implementation agent

Use the Agent tool to spawn an agent. **Do NOT use `isolation: "worktree"`** — we already set up the worktree with full infrastructure. Instead, spawn a regular agent that works in the worktree directory.

Read `worktree.yml` to build the test URLs for the agent prompt. The nginx subdomain pattern tells you the URLs.

**Agent prompt structure:**

```
You are implementing a feature in a worktree environment with running services.

## Task
{user's implementation prompt}

## Your Working Directory
{absolute path to .worktrees/slot-N/}

## Services & Directories
Each service has its own subdirectory:
{for each service in slot:}
  - {service}: {.worktrees/slot-N/repo_key/subdir/}
    Port: {port}
    Logs: .worktrees/slot-N/.logs/{service}.log

## Test URLs
Your changes are served at these URLs (via nginx):
{for each exposed service:}
  - {service}: {subdomain URL}

Direct ports (bypass nginx):
{for each service:}
  - {service}: http://localhost:{port}

## Database
{if isolation == "schema":}
  Schema: feature_{slot} (isolated from other slots)
  Connection: {jdbc/sqlalchemy URL}
{elif isolation == "database":}
  Dedicated DB container on port {db_port}
{else:}
  Shared database (be careful with schema changes)

## How to Test
1. Make your code changes
2. Services auto-reload on file changes (Vite HMR, Spring DevTools, uvicorn --reload)
3. Test via the URLs above
4. Use `curl` to verify API endpoints
5. Run the project's test suite if applicable:
   {test commands from worktree.yml}

## Environment Files
Services load .env files that are auto-generated at start time (secrets from main checkout + port overrides merged together). You should NEVER read .env files directly.

Instead:
  - Read `.env.sample` to understand which variables exist and their purpose
  - Read/edit `.env.overrides` to change ports, URLs, or add new variables

If your feature needs a NEW environment variable:
  1. Add it to the appropriate `.env.overrides` file in your worktree
     (e.g., {project_root}/.worktrees/slot-{N}/be/.env.overrides)
  2. Restart the affected service to re-merge:
     {project_root}/$SCRIPTS_DIR/feature-worktree.sh stop {N}
     {project_root}/$SCRIPTS_DIR/feature-worktree.sh start {N} --services {service}
     (merge-env.sh re-applies your .env.overrides on top of secrets automatically)
  3. Do NOT modify .env files directly — they are overwritten on every restart

## Rules
- Only modify files within your working directory ({project_root}/.worktrees/slot-{N}/)
- Do NOT modify scripts/, nginx/, or files in the main checkout
- Read `.env.sample` and `.env.overrides` — do NOT read `.env` (contains secrets)
- Do NOT stop or restart services unless adding new env vars (they auto-reload on code changes)
- If a service crashes, check its log file and fix the code
- Do NOT commit `.env.overrides` files — they are slot-specific
- Commit your changes to the feature branch when done
```

Set `run_in_background` based on the `--background` flag.

### Step 5: Report results

When the agent completes:

1. Show the agent's output (summary of what was implemented)
2. Show the diff:
   ```bash
   git -C .worktrees/slot-$SLOT/$REPO diff
   ```
3. Show test URLs for manual verification:
   ```
   Your feature is running at:
     http://f{slot}.localhost
     http://f{slot}-api.localhost
   ```

### Step 6: Cleanup decision

If `--keep` was specified, leave everything running and tell the user:
```
Worktree slot {N} is still running. When done testing:
  make feature-stop SLOT={N}
  make feature-destroy SLOT={N}
```

If `--keep` was NOT specified, ask the user:
```
Agent finished. What would you like to do?
  1. Keep worktree running for manual testing
  2. Destroy worktree (stop services, remove branch, clean up)
```

Use `AskUserQuestion` for this.

---

## Running Multiple Agents in Parallel

When the user wants multiple features implemented simultaneously:

```
/worktree-agent "implement auth" --slot 1 --services be --background
/worktree-agent "redesign dashboard" --slot 2 --services fe --background
/worktree-agent "add RAG pipeline" --slot 3 --services genai --background
```

Each invocation is independent. The `--background` flag spawns the agent without blocking.

You can also detect this intent when the user says something like "implement these 3 features in parallel" — in that case, orchestrate all three yourself:

1. Parse the features from the user's message
2. Allocate slots 1, 2, 3
3. Create all worktrees (can be sequential — fast)
4. Start all services (can be parallel)
5. Spawn all agents with `run_in_background: true` in a single message (multiple Agent tool calls)
6. As each agent completes, report its results

### Background Agent Completion

When a background agent finishes, you will be automatically notified. On notification:

1. **Immediately report to the user** — don't wait for them to ask:
   ```
   Background agent for slot {N} ("{feature}") has completed.

   Summary: {agent's output summary}

   Diff: {number of files changed, insertions, deletions}

   Test URLs (still running):
     http://f{N}.localhost
     http://f{N}-api.localhost

   Run `make feature-status` to see all slots.
   ```

2. **Do NOT auto-destroy** — the user may want to manually test. Background agents always imply `--keep`.

3. If multiple background agents complete around the same time, batch the notifications into one message.

---

## Error Handling

| Error | Action |
|-------|--------|
| `worktree.yml` not found | Tell user to run `/worktree bootstrap` |
| All slots occupied | Show `feature-status`, ask user to destroy one |
| Port conflict | Report which process, suggest different slot |
| Service fails to start | Show last 20 lines of log, ask user |
| Agent fails | Show error, keep worktree for debugging |
| Disk space < 5GB | Warn but allow user to proceed |
| Branch already in worktree | Report which slot has it, offer to reuse |

---

## Related Skills

- **`/worktree`** — Run `/worktree bootstrap` first to analyze the project, audit CORS + DB migrations, and generate `worktree.yml` + scripts. This skill requires that setup to exist.

---

## Fallback: Scripts Exist But No `worktree.yml`

If `$SCRIPTS_DIR/feature-worktree.sh` exists but `worktree.yml` doesn't, the project was set up with hardcoded scripts (before the skill system). In this case:

1. **Do NOT fail** — the scripts still work.
2. Run the scripts directly instead of reading config:
   - `$SCRIPTS_DIR/feature-worktree.sh status` → discover existing slots and ports
   - `$SCRIPTS_DIR/feature-worktree.sh create $SLOT $NAME` → create worktree
   - `$SCRIPTS_DIR/feature-worktree.sh start $SLOT` → start services
3. Read `.worktrees/slot-$N/.slot-meta` for port numbers and branch info.
4. Read `nginx/conf.d/slot-$N.conf` for subdomain URLs.
5. Construct the agent prompt from these sources.
6. **Suggest** the user runs `/worktree bootstrap` to generate `worktree.yml` for a better experience next time.

---

## Important Notes

- **This skill replaces `isolation: "worktree"`** for feature development. The built-in worktree isolation only creates a git checkout — no ports, no services, no testing. This skill provides the full environment.
- **Always read `worktree.yml`** before doing anything. All port numbers, service names, URLs, and commands come from there. If `worktree.yml` is missing, fall back to reading scripts + `.slot-meta` (see Fallback section).
- **Do NOT hardcode any project-specific values** in the agent prompt. Everything comes from the config or `.slot-meta`.
- **Services auto-reload** — Vite HMR, Spring DevTools, uvicorn `--reload`. The agent should NOT restart services after code changes.
- **The agent works in the worktree directory**, not the project root. File paths in the agent prompt must be absolute or relative to the worktree.
- **Background agents imply `--keep`** — never auto-destroy a worktree from a background agent. The user must explicitly destroy it.
