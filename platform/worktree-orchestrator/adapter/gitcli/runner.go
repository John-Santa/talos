// Package gitcli provides the concrete adapter that implements port.GitRunner
// using os/exec to invoke the local git binary. All git subcommand argv is
// built per-method (operation-shaped, ADR-D4 — no generic Run(args...) escape hatch).
//
// This adapter is NOT imported by service or domain. cmd/wt is the sole
// composition root that wires it in (hexagonal architecture, design §3).
package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

// Runner implements port.GitRunner against a real git installation.
// RepoRoot is the absolute path to the repository root; all git commands
// are executed with Dir=RepoRoot so relative paths in git output resolve
// consistently.
type Runner struct {
	repoRoot string
}

// NewRunner constructs a Runner for the given repository root path.
func NewRunner(repoRoot string) *Runner {
	return &Runner{repoRoot: repoRoot}
}

// run executes a git command in the repo root directory and returns the
// combined stdout and stderr buffers. Returns an error if git exits non-zero.
func (r *Runner) run(ctx context.Context, args ...string) (stdout string, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.repoRoot

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// ---------------------------------------------------------------------------
// port.GitRunner implementation
// ---------------------------------------------------------------------------

// Fetch fetches from origin. Equivalent to: git fetch origin.
func (r *Runner) Fetch(ctx context.Context) error {
	_, errOut, err := r.run(ctx, "fetch", "origin")
	if err != nil {
		return fmt.Errorf("git fetch origin: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return nil
}

// BranchExists reports whether the named branch exists locally.
// Uses git rev-parse to check — exits non-zero if the branch is absent.
func (r *Runner) BranchExists(ctx context.Context, branch string) (bool, error) {
	_, _, err := r.run(ctx, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		// Non-zero exit from rev-parse means the ref does not exist — that is
		// a normal "false" outcome, not an error.
		if isExitError(err) {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse --verify refs/heads/%s: %w", branch, err)
	}
	return true, nil
}

// WorktreeAdd creates a new worktree at path on a new branch from baseBranch.
// Equivalent to: git worktree add -b <branch> <path> <baseBranch>.
func (r *Runner) WorktreeAdd(ctx context.Context, path, branch, baseBranch string) error {
	_, errOut, err := r.run(ctx, "worktree", "add", "-b", branch, path, baseBranch)
	if err != nil {
		return fmt.Errorf("git worktree add -b %q %q %q: %w (stderr: %s)",
			branch, path, baseBranch, err, strings.TrimSpace(errOut))
	}
	return nil
}

// WorktreeList returns the raw stdout of `git worktree list --porcelain`.
// The adapter does NOT parse the output — parsing is the domain's responsibility
// (design §7, constraint rule 2 in tasks.md).
func (r *Runner) WorktreeList(ctx context.Context) (string, error) {
	out, errOut, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git worktree list --porcelain: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return out, nil
}

// WorktreeRemove removes the worktree at path. When force is false, git will
// refuse if the worktree has uncommitted changes; the adapter maps that
// specific stderr to domain.ErrDirtyWorktree. When force is true, passes
// --force to git to allow removal of dirty worktrees.
//
// Dirty-worktree detection strategy (design §8): string-match on stderr.
// git writes "contains modified or untracked files" or "is dirty" when it
// refuses to remove a non-clean worktree. We match both patterns.
func (r *Runner) WorktreeRemove(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	_, errOut, err := r.run(ctx, args...)
	if err != nil {
		stderrLower := strings.ToLower(errOut)
		if !force && (strings.Contains(stderrLower, "contains modified or untracked files") ||
			strings.Contains(stderrLower, "is dirty") ||
			strings.Contains(stderrLower, "uncommitted changes")) {
			return &worktree.ErrDirtyWorktree{Path: path}
		}
		return fmt.Errorf("git worktree remove %q: %w (stderr: %s)",
			path, err, strings.TrimSpace(errOut))
	}
	return nil
}

// Prune runs `git worktree prune` to clean up stale administrative state
// for removed worktrees. Always called after WorktreeRemove.
func (r *Runner) Prune(ctx context.Context) error {
	_, errOut, err := r.run(ctx, "worktree", "prune")
	if err != nil {
		return fmt.Errorf("git worktree prune: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return nil
}

// BranchDelete deletes the named branch using `git branch -d` (safe: refuses
// unmerged branches). NEVER uses -D (ADR-D2). Called only when --delete-branch
// is explicitly set by the user.
func (r *Runner) BranchDelete(ctx context.Context, branch string) error {
	_, errOut, err := r.run(ctx, "branch", "-d", branch)
	if err != nil {
		return fmt.Errorf("git branch -d %q: %w (stderr: %s)",
			branch, err, strings.TrimSpace(errOut))
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// isExitError reports whether err is an *exec.ExitError (non-zero exit code).
func isExitError(err error) bool {
	var e *exec.ExitError
	_ = e
	_, ok := err.(*exec.ExitError)
	return ok
}
