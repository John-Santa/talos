// Package mergeorder contains the pure domain logic for the merge-order-orchestrator.
package mergeorder

import (
	"fmt"
	"strings"
)

// ErrDependencyCycle is returned when the dependency graph contains a cycle.
type ErrDependencyCycle struct {
	Branches []string
}

func (e *ErrDependencyCycle) Error() string {
	return fmt.Sprintf("dependency cycle detected among branches: %s", strings.Join(e.Branches, ", "))
}

// ErrNoCandidates is returned when no active branches with commits ahead of the base are found.
type ErrNoCandidates struct{}

func (e *ErrNoCandidates) Error() string {
	return "no merge candidates found: all worktrees are orphaned or up-to-date"
}

// ErrBranchBehind is returned when a branch has zero commits ahead of the integration base.
type ErrBranchBehind struct {
	Branch string
}

func (e *ErrBranchBehind) Error() string {
	return fmt.Sprintf("branch %q is not ahead of the integration base", e.Branch)
}

// ErrMergeConflict is returned when a predicted or actual merge conflict is detected.
type ErrMergeConflict struct {
	Branch string
	Files  []string
}

func (e *ErrMergeConflict) Error() string {
	return fmt.Sprintf("merge conflict on branch %q: conflicting files: %s", e.Branch, strings.Join(e.Files, ", "))
}

// ErrWtBinaryNotFound is returned when the wt binary cannot be located.
type ErrWtBinaryNotFound struct {
	Binary string
}

func (e *ErrWtBinaryNotFound) Error() string {
	return fmt.Sprintf("wt binary %q not found in PATH", e.Binary)
}

// ErrWtOutputMalformed is returned when the output of wt list --json cannot be parsed.
type ErrWtOutputMalformed struct {
	Fragment string
	Cause    error
}

func (e *ErrWtOutputMalformed) Error() string {
	return fmt.Sprintf("malformed wt output (fragment: %q): %v", e.Fragment, e.Cause)
}

func (e *ErrWtOutputMalformed) Unwrap() error { return e.Cause }

// ErrSegmentationBad is returned when the conflict rate exceeds the configured threshold.
type ErrSegmentationBad struct {
	Rate      float64
	Threshold float64
}

func (e *ErrSegmentationBad) Error() string {
	return fmt.Sprintf("segmentation health check failed: conflict rate %.4f exceeds threshold %.4f", e.Rate, e.Threshold)
}

// ErrDevelopNotAvailable is returned when the integration branch cannot be resolved.
type ErrDevelopNotAvailable struct{}

func (e *ErrDevelopNotAvailable) Error() string {
	return "integration branch (develop) is not available"
}

// ErrBranchNotFound is returned when a requested branch does not exist in the repository.
type ErrBranchNotFound struct {
	Branch string
}

func (e *ErrBranchNotFound) Error() string {
	return fmt.Sprintf("branch %q not found", e.Branch)
}

// ErrRebaseConflict is returned when a rebase operation produces unresolvable conflicts.
type ErrRebaseConflict struct {
	Branch string
	Files  []string
}

func (e *ErrRebaseConflict) Error() string {
	return fmt.Sprintf("rebase conflict on branch %q: conflicting files: %s", e.Branch, strings.Join(e.Files, ", "))
}
