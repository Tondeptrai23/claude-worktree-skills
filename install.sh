#!/usr/bin/env bash
# =============================================================================
# install.sh — Install Claude Code worktree skills into any project
#
# Usage (from your project root):
#   curl -sSL https://raw.githubusercontent.com/Tondeptrai23/claude-worktree-skills/main/install.sh | bash
#
# Or clone and run locally:
#   git clone https://github.com/Tondeptrai23/claude-worktree-skills.git /tmp/claude-worktree-skills
#   /tmp/claude-worktree-skills/install.sh
#
# What it does:
#   1. Downloads skill files into .claude/skills/
#   2. Builds the `wt` CLI tool and places it in .claude/bin/
#   3. Adds .claude/bin to CLAUDE.md so the agent can find it
#
# Requirements: git, go 1.23+
# Platforms:    Linux, macOS, Windows (Git Bash / WSL)
# =============================================================================
set -euo pipefail

REPO_URL="${CLAUDE_WORKTREE_REPO:-https://github.com/Tondeptrai23/claude-worktree-skills.git}"
REPO_BRANCH="${CLAUDE_WORKTREE_BRANCH:-main}"

# --- Helpers -----------------------------------------------------------------

info()  { printf '\033[32m[*]\033[0m %s\n' "$1"; }
warn()  { printf '\033[33m[!]\033[0m %s\n' "$1"; }
error() { printf '\033[31m[!]\033[0m %s\n' "$1" >&2; exit 1; }

check_cmd() {
    command -v "$1" >/dev/null 2>&1 || error "'$1' is required but not found. Please install it first."
}

# --- Pre-flight --------------------------------------------------------------

check_cmd git
check_cmd go

# Must be run from a git repo root
if [ ! -d ".git" ]; then
    error "Run this from your project root (no .git directory found)"
fi

PROJECT_ROOT="$(pwd)"
CLAUDE_DIR="$PROJECT_ROOT/.claude"
SKILLS_DIR="$CLAUDE_DIR/skills"
BIN_DIR="$CLAUDE_DIR/bin"

# --- Download ----------------------------------------------------------------

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading worktree skills..."
git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$TMPDIR/repo" 2>/dev/null \
    || error "Failed to clone $REPO_URL"

SRC="$TMPDIR/repo"

# --- Install skills ----------------------------------------------------------

info "Installing skills to $SKILLS_DIR/"

mkdir -p "$SKILLS_DIR/worktree/assets"
mkdir -p "$SKILLS_DIR/worktree/references"
mkdir -p "$SKILLS_DIR/worktree-agent"

cp "$SRC/worktree/SKILL.md" "$SKILLS_DIR/worktree/SKILL.md"
cp "$SRC/worktree/assets/"* "$SKILLS_DIR/worktree/assets/"
cp "$SRC/worktree/references/"* "$SKILLS_DIR/worktree/references/"
cp "$SRC/worktree-agent/SKILL.md" "$SKILLS_DIR/worktree-agent/SKILL.md"

info "Installed worktree skill"
info "Installed worktree-agent skill"

# --- Build CLI ---------------------------------------------------------------

info "Building wt CLI..."

mkdir -p "$BIN_DIR"

(
    cd "$SRC"
    CGO_ENABLED=0 go build -o "$BIN_DIR/wt" .
) || error "Failed to build wt binary. Check that Go 1.23+ is installed."

# Make executable (no-op on Windows but harmless)
chmod +x "$BIN_DIR/wt" 2>/dev/null || true

info "Built $BIN_DIR/wt"

# --- Update CLAUDE.md --------------------------------------------------------

CLAUDE_MD="$PROJECT_ROOT/CLAUDE.md"

# Add wt to PATH hint if CLAUDE.md exists or create a minimal one
WT_PATH_LINE="The \`wt\` CLI is at \`.claude/bin/wt\`. Add \`.claude/bin\` to PATH or invoke directly."

if [ -f "$CLAUDE_MD" ]; then
    if ! grep -qF ".claude/bin/wt" "$CLAUDE_MD"; then
        printf '\n## Worktree\n\n%s\n' "$WT_PATH_LINE" >> "$CLAUDE_MD"
        info "Appended worktree section to CLAUDE.md"
    fi
else
    cat > "$CLAUDE_MD" << 'CLAUDEEOF'
# Project Notes

## Worktree

The `wt` CLI is at `.claude/bin/wt`. Add `.claude/bin` to PATH or invoke directly.

Use `/worktree` to bootstrap worktree config for this project.
CLAUDEEOF
    info "Created CLAUDE.md"
fi

# --- Add to .gitignore -------------------------------------------------------

GITIGNORE="$PROJECT_ROOT/.gitignore"

ensure_ignored() {
    local pattern="$1"
    if [ -f "$GITIGNORE" ]; then
        grep -qxF "$pattern" "$GITIGNORE" 2>/dev/null && return
    fi
    echo "$pattern" >> "$GITIGNORE"
}

ensure_ignored ".claude/bin/"
ensure_ignored ".worktrees/"

info "Updated .gitignore"

# --- Done --------------------------------------------------------------------

printf '\n\033[32m[OK]\033[0m Worktree skills installed.\n'
echo ""
echo "  Skills: $SKILLS_DIR/worktree/"
echo "          $SKILLS_DIR/worktree-agent/"
echo "  CLI:    $BIN_DIR/wt"
echo ""
echo "  Next:   Open Claude Code and run /worktree to bootstrap your project."
echo ""
