// Package port defines the outbound ports for the worktree-orchestrator.
package port

import "context"

// GitRunner is the outbound port for all git operations required by the worktree-orchestrator.
type GitRunner interface {
	// Fetch fetches from origin before branching.
	Fetch(ctx context.Context) error

	// BranchExists reports whether the named branch exists in the local repo.
	BranchExists(ctx context.Context, branch string) (bool, error)

	// WorktreeAdd creates a new worktree at path on a new branch from baseBranch.
	WorktreeAdd(ctx context.Context, path, branch, baseBranch string) error

	// WorktreeList returns the raw stdout of `git worktree list --porcelain`.
	WorktreeList(ctx context.Context) (string, error)

	// WorktreeRemove removes the worktree at path; maps dirty-worktree stderr to ErrDirtyWorktreeSentinel when force is false.
	WorktreeRemove(ctx context.Context, path string, force bool) error

	// Prune runs `git worktree prune` to clean up stale state after removal.
	Prune(ctx context.Context) error

	// BranchDelete deletes the named branch using `git branch -d` (safe: refuses unmerged branches).
	BranchDelete(ctx context.Context, branch string) error
}
