# Integrating Worktrees Into Other Skills

The worktree system exposes a CLI interface via the `wt` binary (`.claude/bin/wt`). Any skill or workflow can use these commands as building blocks — no dependency on the `/worktree` or `/worktree-agent` skills.

## CLI API

The `wt` CLI reads `worktree.yml` for all configuration. Run `/worktree bootstrap` once to generate the config.

```bash
# Create a worktree slot (only specified services, or all by default)
.claude/bin/wt create <slot> <name> [--services svc1,svc2] [--<svc>-branch <branch>]

# Start services (auto-starts nginx, merges .env + .env.overrides)
.claude/bin/wt start <slot> [--services svc1,svc2]

# Stop services
.claude/bin/wt stop <slot>

# Destroy slot (remove worktree, optionally teardown DB)
.claude/bin/wt destroy <slot> [--teardown-db]

# Show all active slots
.claude/bin/wt status

# View logs
.claude/bin/wt logs <slot> [service]
```

Key behaviors:
- `create --services fe` only creates fe/ worktree, skips other services entirely. Skips DB setup if no backend service is in the slot.
- `create` automatically installs deps, runs DB setup + seed + migrations, and regenerates nginx.
- `start` auto-starts nginx if not running (finds available port if default is occupied). Merges env files (copies main .env secrets + applies .env.overrides) before launching services.
- `start --services fe` only starts fe services (but still merges env for all services in the slot).
- Feature names are sanitized for DNS: `TICKET-123` -> `ticket-123.be.localhost`.

### Slot metadata

After creation, `.worktrees/slot-{N}/.slot-meta.yml` contains:
```yaml
slot: 1
feature_name: my-feature
mode: dev
services:
  be:
    branch: feature/my-feature
    port: 8180
    repo_key: be
```

### Nginx subdomains

Default pattern: `{name}.{svc}.localhost` (configurable in `worktree.yml`).

Example for `wt create 1 ticket-123`:
- `http://ticket-123.be.localhost` — backend API
- `http://ticket-123.fe.localhost` — frontend

## Integration Pattern

Add this to your existing skill:

```markdown
## Worktree Integration (optional)

If `worktree.yml` exists in the project root, you can use isolated worktrees
for implementation. This gives you dedicated ports, running services, and
test URLs without affecting the main checkout.

### Setup (before implementing)
1. Find a free slot: `ls .worktrees/` to see which slots are taken (1-3)
2. Create: `.claude/bin/wt create $SLOT $FEATURE_NAME --services $SERVICES`
3. Start: `.claude/bin/wt start $SLOT`
   (nginx and env merge happen automatically)
4. Work inside `.worktrees/slot-$SLOT/` directory

### Your existing workflow runs here
- Implement, test, lint — all inside the worktree directory
- Services auto-reload on file changes
- Test via nginx URLs or direct localhost:port

### Teardown (after PR is created)
5. Stop: `.claude/bin/wt stop $SLOT`
6. Destroy: `.claude/bin/wt destroy $SLOT`
```

## Environment Files

- `.env.overrides` — safe to read/edit (ports and URLs only)
- `.env.sample` — safe to read (variable names and descriptions)
- `.env` — DO NOT read (contains secrets, regenerated each `start`)

If your workflow needs to add a new env var, write it to `.env.overrides` and restart the affected service. The restart re-merges your override on top of secrets.

## Example: Adding to a /implement Skill

```markdown
# In your /implement SKILL.md, add before the implementation steps:

### Optional: Isolated Worktree

Check if worktree infrastructure is available:
- If `worktree.yml` exists AND the task modifies backend/frontend code:
  1. Allocate a slot and create worktree (only the services the task touches)
  2. Start services (nginx auto-starts)
  3. Implement inside the worktree
  4. Test against running services
  5. Commit and push from the worktree branch
  6. Create PR
  7. Ask user: keep worktree for review, or destroy?

- If `worktree.yml` does not exist:
  - Implement in the main checkout as usual
```

## Example: Adding to a /commit Skill

The worktree branch is a regular git branch. Your commit rules apply as-is:

```bash
# Inside the worktree
cd .worktrees/slot-1/be
git add -A
git commit -m "feat: your commit message format"
git push origin feature/my-feature
```

## Example: Adding to a /create-pr Skill

```bash
# From the worktree service directory
cd .worktrees/slot-1/be
gh pr create --title "feat: ..." --body "..."
```

The PR is created from the worktree's feature branch to the main branch — standard git workflow.
