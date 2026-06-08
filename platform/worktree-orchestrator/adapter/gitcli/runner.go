// Package gitcli provides the concrete adapter that implements port.GitRunner using os/exec.
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
type Runner struct {
	repoRoot string
}

// NewRunner constructs a Runner for the given repository root path.
func NewRunner(repoRoot string) *Runner {
	return &Runner{repoRoot: repoRoot}
}

func (r *Runner) run(ctx context.Context, args ...string) (stdout string, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.repoRoot

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// Fetch fetches from origin.
func (r *Runner) Fetch(ctx context.Context) error {
	_, errOut, err := r.run(ctx, "fetch", "origin")
	if err != nil {
		return fmt.Errorf("git fetch origin: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return nil
}

// BranchExists reports whether the named branch exists locally.
func (r *Runner) BranchExists(ctx context.Context, branch string) (bool, error) {
	_, _, err := r.run(ctx, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		if isExitError(err) {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse --verify refs/heads/%s: %w", branch, err)
	}
	return true, nil
}

// WorktreeAdd creates a new worktree at path on a new branch from baseBranch.
func (r *Runner) WorktreeAdd(ctx context.Context, path, branch, baseBranch string) error {
	_, errOut, err := r.run(ctx, "worktree", "add", "-b", branch, path, baseBranch)
	if err != nil {
		return fmt.Errorf("git worktree add -b %q %q %q: %w (stderr: %s)",
			branch, path, baseBranch, err, strings.TrimSpace(errOut))
	}
	return nil
}

// WorktreeList returns the raw stdout of `git worktree list --porcelain`.
func (r *Runner) WorktreeList(ctx context.Context) (string, error) {
	out, errOut, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git worktree list --porcelain: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return out, nil
}

// WorktreeRemove removes the worktree at path, mapping dirty-worktree stderr to ErrDirtyWorktreeSentinel when force is false.
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
			return &worktree.ErrDirtyWorktreeSentinel{
				Path:      path,
				FileCount: r.countDirtyFiles(ctx, path),
			}
		}
		return fmt.Errorf("git worktree remove %q: %w (stderr: %s)",
			path, err, strings.TrimSpace(errOut))
	}
	return nil
}

func (r *Runner) countDirtyFiles(ctx context.Context, wtPath string) int {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// Prune runs `git worktree prune` to clean up stale state for removed worktrees.
func (r *Runner) Prune(ctx context.Context) error {
	_, errOut, err := r.run(ctx, "worktree", "prune")
	if err != nil {
		return fmt.Errorf("git worktree prune: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return nil
}

// BranchDelete deletes the named branch using `git branch -d` (safe: refuses unmerged branches).
func (r *Runner) BranchDelete(ctx context.Context, branch string) error {
	_, errOut, err := r.run(ctx, "branch", "-d", branch)
	if err != nil {
		return fmt.Errorf("git branch -d %q: %w (stderr: %s)",
			branch, err, strings.TrimSpace(errOut))
	}
	return nil
}

func isExitError(err error) bool {
	var e *exec.ExitError
	_ = e
	_, ok := err.(*exec.ExitError)
	return ok
}
