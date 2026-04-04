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

1. **Locate config.** Search for `worktree.yml`:
   - Check `.claude/worktree/worktree.yml` (standard location)
   - Check project root `worktree.yml`
   - If not found: tell user "Run `/worktree bootstrap` first." and stop.

2. **Verify `wt` CLI exists** at `.claude/bin/wt`. If not, tell user to run the install script.

   **Always cd to the project root** before running `wt` commands. The working directory persists between Bash calls, and each `wt` invocation must be a **separate Bash call** using the relative path so permissions match:
   ```bash
   # First Bash call: set working directory
   cd "$(git rev-parse --show-toplevel)"

   # Subsequent Bash calls: use relative path (standalone, not combined with &&)
   .claude/bin/wt status
   ```

2b. **Run `wt verify`** to catch config issues before creating infrastructure:
   ```bash
   .claude/bin/wt verify
   ```
   If errors are reported, fix `worktree.yml` before proceeding. If warnings about missing env_overrides are reported, add the suggested entries to `worktree.yml`.

3. **Read `worktree.yml`** to get: `max_slots`, service definitions, port scheme, DB isolation mode, branch prefix, nginx subdomain pattern.

4. **Slot allocation** — if `--slot` not specified, find the first free slot:
   ```bash
   .claude/bin/wt next-slot
   ```
   Prints the slot number, or exits with an error if all slots are occupied. If all slots are occupied, run `.claude/bin/wt status` and ask the user to destroy one.

5. **Pre-flight checks** — run all checks in one command:
   ```bash
   .claude/bin/wt preflight $SLOT $FEATURE_NAME
   ```
   This checks disk space, port availability, branch conflicts, Docker status, nginx, and slot availability. If any critical check fails, report the issue and stop.

### Step 1: Create the worktree

```bash
.claude/bin/wt create $SLOT $FEATURE_NAME --services $SERVICES
```

This handles: git worktree creation, env overrides, dependency install, DB setup + seed + migrations, nginx config regeneration.

If the command fails, report the error and clean up.

### Step 2: Start services

```bash
.claude/bin/wt start $SLOT --services $SERVICES
```

This auto-starts nginx if not running (finding an available port if needed), merges env files (secrets + overrides), then launches the services.

### Step 3: Wait for services to be healthy

After starting, verify all services are responding and get the test URLs:

```bash
.claude/bin/wt health $SLOT
```

This waits for each service to accept connections (up to 60s), checks nginx routing, and prints the test URLs (both nginx subdomains and direct ports). Use the URLs from this output in the agent prompt.

If a service fails, check its log:
```bash
.claude/bin/wt logs $SLOT $SERVICE
```
Report the error and ask the user how to proceed.

### Step 4: Spawn the implementation agent

Use the Agent tool to spawn an agent. **Do NOT use `isolation: "worktree"`** — we already set up the worktree with full infrastructure. Instead, spawn a regular agent that works in the worktree directory.

Read `worktree.yml` to build the test URLs for the agent prompt. The nginx subdomain pattern tells you the URLs (default: `{name}.{svc}.localhost`).

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
    Logs: .claude/bin/wt logs {N} {service}

## Test URLs
Your changes are served at these URLs (via nginx):
{for each exposed service:}
  - {service}: http://{name}.{svc}.localhost:{nginx_port}

Direct ports (bypass nginx):
{for each service:}
  - {service}: http://localhost:{port}

## Database
{if isolation == "schema":}
  Schema: {schema_prefix}{slot} (isolated from other slots)
{elif isolation == "database":}
  Dedicated DB container on port {db_port}
{else:}
  Shared database (be careful with schema changes)

## How to Test
1. Make your code changes
2. Services auto-reload on file changes (Vite HMR, Spring DevTools, uvicorn --reload)
3. Test via the URLs above
4. Use `curl` to verify API endpoints
5. Run the project's test suite if applicable

## Environment Files
Services load .env files that are auto-generated at start time (secrets from main checkout + port overrides merged together). You should NEVER read .env files directly.

Instead:
  - Read `.env.sample` to understand which variables exist and their purpose
  - Read/edit `.env.overrides` to change ports, URLs, or add new variables

If your feature needs a NEW environment variable:
  1. Add it to the appropriate `.env.overrides` file in your worktree
  2. Restart the affected service:
     .claude/bin/wt stop {N}
     .claude/bin/wt start {N} --services {service}
  3. Do NOT modify .env files directly — they are overwritten on every restart

## Rules
- Only modify files within your working directory
- Do NOT modify files in the main checkout
- Read `.env.sample` and `.env.overrides` — do NOT read `.env` (contains secrets)
- Do NOT stop or restart services unless adding new env vars (they auto-reload on code changes)
- If a service crashes, check its log and fix the code
- Commit your changes to the feature branch when done
- When committing, do NOT include these runtime/generated files:
  - `.logs/`, `.pids/`, `.slot-meta.yml` (worktree runtime state)
  - `**/.env.overrides` (generated port/URL overrides)
  - Stage only the files you actually changed (use `git add <file>`, not `git add -A`)
```

Set `run_in_background` based on the `--background` flag.

### Step 5: Report results

When the agent completes:

1. Show the agent's output (summary of what was implemented)
2. Show the diff:
   ```bash
   git -C .worktrees/slot-$SLOT/$REPO diff
   ```
3. Show test URLs for manual verification

### Step 6: Cleanup decision

If `--keep` was specified, leave everything running and tell the user:
```
Worktree slot {N} is still running. When done testing:
  .claude/bin/wt stop {N}
  .claude/bin/wt destroy {N}
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
3. Create all worktrees sequentially (fast)
4. Start all services (can be parallel)
5. Spawn all agents with `run_in_background: true` in a single message
6. As each agent completes, report its results

### Background Agent Completion

When a background agent finishes, you will be automatically notified. On notification:

1. **Immediately report to the user** — don't wait for them to ask
2. **Do NOT auto-destroy** — the user may want to manually test. Background agents always imply `--keep`.
3. If multiple background agents complete around the same time, batch the notifications.

---

## Error Handling

| Error | Action |
|-------|--------|
| `worktree.yml` not found | Tell user to run `/worktree bootstrap` |
| `wt` CLI not found | Tell user to run the install script |
| All slots occupied | Run `wt status`, ask user to destroy one |
| Port conflict | Report which process, suggest different slot |
| Service fails to start | Show log via `wt logs`, ask user |
| Agent fails | Show error, keep worktree for debugging |
| Disk space < 5GB | Warn but allow user to proceed |
| Branch already in worktree | Report which slot has it, offer to reuse |

---

## Important Notes

- **This skill replaces `isolation: "worktree"`** for feature development. The built-in worktree isolation only creates a git checkout — no ports, no services, no testing. This skill provides the full environment.
- **Always read `worktree.yml`** before doing anything. All port numbers, service names, URLs, and commands come from there.
- **Do NOT hardcode any project-specific values** in the agent prompt. Everything comes from the config.
- **Services auto-reload** — Vite HMR, Spring DevTools, uvicorn `--reload`. The agent should NOT restart services after code changes.
- **The agent works in the worktree directory**, not the project root.
- **Background agents imply `--keep`** — never auto-destroy a worktree from a background agent.
