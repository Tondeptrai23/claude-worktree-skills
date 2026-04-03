# Integrating Worktrees Into Other Skills

The worktree system exposes a CLI interface via bash scripts. Any skill or workflow can use these as building blocks — no dependency on the `/worktree` or `/worktree-agent` skills.

## Script API

Scripts are in the project's scripts directory (locate by checking `worktree.yml` or searching for `feature-worktree.sh`). They require `worktree.yml` to exist (run `/worktree bootstrap` once).

### feature-worktree.sh

```bash
# Create a worktree slot (only specified services, or all by default)
scripts/feature-worktree.sh create <slot> <name> [--services svc1,svc2] [--<svc>-branch <branch>]

# Start services (auto-starts nginx, merges .env + .env.overrides)
scripts/feature-worktree.sh start <slot> [--services svc1,svc2]

# Stop services
scripts/feature-worktree.sh stop <slot>

# Destroy slot (remove worktree, optionally drop DB schema)
scripts/feature-worktree.sh destroy <slot> [--drop-schema]

# Show all active slots
scripts/feature-worktree.sh status
```

Key behaviors:
- `create --services fe` only creates fe/ worktree, skips be/ and genai/ entirely (no git worktree, no env overrides, no dep install). Skips DB schema creation if no backend service is in the slot.
- `start` auto-starts nginx if not running. Runs `merge-env.sh` (copies main .env secrets + applies .env.overrides) before launching services.
- `start --services fe` only starts fe services (but still merges env for all services in the slot).

### Slot metadata

After creation, `.worktrees/slot-{N}/.slot-meta` contains:
```bash
SLOT=1
FEATURE_NAME=my-feature
BE_PORT=8180
FE_APP_PORT=5273
# ... etc
```

Source this file to get port numbers and branch names for your workflow.

### Nginx subdomains

Read `nginx/conf.d/slot-{N}.conf` or derive from `worktree.yml`:
- `http://f{N}.localhost` — frontend
- `http://f{N}-api.localhost` — backend API
- Pattern: configured in `worktree.yml` under `nginx.subdomains`

## Integration Pattern

Add this to your existing skill:

```markdown
## Worktree Integration (optional)

If `worktree.yml` exists in the project root, you can use isolated worktrees
for implementation. This gives you dedicated ports, running services, and
test URLs without affecting the main checkout.

### Setup (before implementing)
1. Find a free slot: `ls .worktrees/` to see which slots are taken (1-3)
2. Create: `scripts/feature-worktree.sh create $SLOT $FEATURE_NAME --services $SERVICES`
3. Start: `scripts/feature-worktree.sh start $SLOT`
   (nginx and env merge happen automatically)
4. Read `.worktrees/slot-$SLOT/.slot-meta` for ports and URLs
5. Work inside `.worktrees/slot-$SLOT/` directory

### Your existing workflow runs here
- Implement, test, lint — all inside the worktree directory
- Services auto-reload on file changes
- Test via nginx URLs or direct localhost:port

### Teardown (after PR is created)
6. Stop: `scripts/feature-worktree.sh stop $SLOT`
7. Destroy: `scripts/feature-worktree.sh destroy $SLOT`
```

## Environment Files

- `.env.overrides` — safe to read/edit (ports and URLs only)
- `.env.sample` — safe to read (variable names and descriptions)
- `.env` — DO NOT read (contains secrets, regenerated each `start` by `merge-env.sh`)

If your workflow needs to add a new env var, write it to `.env.overrides` and restart the affected service. The restart re-runs `merge-env.sh` which applies your new override on top of secrets.

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
