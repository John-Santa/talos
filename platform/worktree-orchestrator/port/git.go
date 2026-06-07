// Package port defines the outbound ports for the worktree-orchestrator
// hexagonal architecture. Adapters implement these interfaces; the service
// depends only on them, never on concrete adapters.
//
// The GitRunner interface is a TEST-SEAM port (ADR-D1): its primary value is
// enabling mock-based service tests with error injection and call-order
// verification, not backend swapability.
package port

import "context"

// GitRunner is the outbound port for all git operations required by the
// worktree-orchestrator. Each method maps to a specific git operation — the
// adapter builds the actual argv; the service uses only semantic method calls.
//
// All implementations must:
//   - Return non-nil errors on any non-zero git exit code.
//   - Never swallow errors silently.
//   - Map git stderr to typed domain errors where specified (e.g. ErrDirtyWorktree).
type GitRunner interface {
	// Fetch fetches from origin to ensure develop is up to date before
	// branching. Called by Create unless --no-fetch is set.
	Fetch(ctx context.Context) error

	// BranchExists reports whether the named branch exists in the local repo.
	// Used by Create to pre-check for branch collisions (REQ-CREATE-5).
	BranchExists(ctx context.Context, branch string) (bool, error)

	// WorktreeAdd creates a new worktree at path on a new branch branching
	// from baseBranch. Equivalent to: git worktree add -b <branch> <path> <baseBranch>.
	WorktreeAdd(ctx context.Context, path, branch, baseBranch string) error

	// WorktreeList returns the raw stdout of `git worktree list --porcelain`.
	// The caller (service or adapter) is responsible for passing the output to
	// domain.ParseWorktreeList. The adapter MUST NOT parse the output.
	WorktreeList(ctx context.Context) (string, error)

	// WorktreeRemove removes the worktree at path. When force is false, git
	// will refuse if the worktree has uncommitted changes; the adapter must
	// map that stderr to domain.ErrDirtyWorktree. When force is true, passes
	// --force to git worktree remove.
	WorktreeRemove(ctx context.Context, path string, force bool) error

	// Prune runs `git worktree prune` to clean up stale administrative state
	// for removed worktrees. Always called after WorktreeRemove.
	Prune(ctx context.Context) error

	// BranchDelete deletes the named branch using `git branch -d` (safe: refuses
	// unmerged branches). NEVER uses -D (ADR-D2). Called only when --delete-branch
	// is explicitly set by the user.
	BranchDelete(ctx context.Context, branch string) error
}
