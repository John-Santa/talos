// Package worktree contains the pure domain logic for the worktree-orchestrator.
package worktree

import "fmt"

// ErrInvalidFigure is returned when a figura string is not in the agent roster.
type ErrInvalidFigure struct {
	Figura string
}

func (e *ErrInvalidFigure) Error() string {
	return fmt.Sprintf("invalid figura %q: not in CONSTITUTION §1 roster", e.Figura)
}

// ErrInvalidKey is returned when a Jira key does not match the <projectKey>-<n> pattern.
type ErrInvalidKey struct {
	Key        string
	ProjectKey string
}

func (e *ErrInvalidKey) Error() string {
	return fmt.Sprintf("invalid jira key %q: must match %s-<n> (n >= 1)", e.Key, e.ProjectKey)
}

// ErrWorktreeExists is returned when a worktree for the given figura already exists.
type ErrWorktreeExists struct {
	Figura string
	Path   string
}

func (e *ErrWorktreeExists) Error() string {
	return fmt.Sprintf("worktree for figura %q already exists at %q", e.Figura, e.Path)
}

// ErrBranchExists is returned when the target branch already exists in the repo.
type ErrBranchExists struct {
	Branch string
}

func (e *ErrBranchExists) Error() string {
	return fmt.Sprintf("branch %q already exists", e.Branch)
}

// ErrWorktreeNotFound is returned when no worktree for the given figura is found.
type ErrWorktreeNotFound struct {
	Figura string
	Path   string
}

func (e *ErrWorktreeNotFound) Error() string {
	return fmt.Sprintf("worktree for figura %q not found at %q", e.Figura, e.Path)
}

// ErrDirtyWorktree is returned when a worktree has uncommitted changes and --force was not supplied.
type ErrDirtyWorktree struct {
	Figura    string
	Path      string
	FileCount int
}

func (e *ErrDirtyWorktree) Error() string {
	if e.FileCount > 0 {
		return fmt.Sprintf("worktree for figura %q at %q has %d uncommitted file(s); use --force to override",
			e.Figura, e.Path, e.FileCount)
	}
	return fmt.Sprintf("worktree for figura %q at %q has uncommitted changes; use --force to override", e.Figura, e.Path)
}

// ErrDirtyWorktreeSentinel is the adapter-level sentinel for a dirty worktree; the service enriches it with the figura.
type ErrDirtyWorktreeSentinel struct {
	Path      string
	FileCount int
}

func (e *ErrDirtyWorktreeSentinel) Error() string {
	return fmt.Sprintf("worktree at %q is dirty (%d dirty file(s))", e.Path, e.FileCount)
}
