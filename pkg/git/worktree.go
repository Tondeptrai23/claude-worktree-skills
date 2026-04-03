package git

import (
	"fmt"
	"os/exec"
)

// CreateWorktree creates a git worktree at targetDir on the given branch.
// Handles three cases: local branch exists, remote branch exists, or new branch.
func CreateWorktree(repoDir, targetDir, branch string) error {
	// 1. Local branch exists
	if refExists(repoDir, "refs/heads/"+branch) {
		return run(repoDir, "worktree", "add", targetDir, branch)
	}

	// 2. Remote branch exists
	if refExists(repoDir, "refs/remotes/origin/"+branch) {
		return run(repoDir, "worktree", "add", "--track", "-b", branch, targetDir, "origin/"+branch)
	}

	// 3. Create new branch from HEAD
	return run(repoDir, "worktree", "add", "-b", branch, targetDir)
}

// RemoveWorktree removes a git worktree. Falls back to force removal.
func RemoveWorktree(repoDir, worktreeDir string) error {
	err := run(repoDir, "worktree", "remove", worktreeDir, "--force")
	if err != nil {
		// Fallback: just remove the directory and let git prune later
		return run(repoDir, "worktree", "prune")
	}
	return nil
}

func refExists(repoDir, ref string) bool {
	cmd := exec.Command("git", "-C", repoDir, "show-ref", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}

func run(repoDir string, args ...string) error {
	fullArgs := append([]string{"-C", repoDir}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}
