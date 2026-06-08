// Package port defines the outbound ports for the merge-order-orchestrator.
package port

import "context"

// GitInspector is the read-only outbound port for git inspection operations.
type GitInspector interface {
	// Fetch fetches from origin to bring the local view up to date.
	Fetch(ctx context.Context) error

	// RevParse resolves a ref (branch name, HEAD, etc.) to its full commit SHA.
	RevParse(ctx context.Context, ref string) (string, error)

	// MergeBase returns the best common ancestor commit between a and b.
	MergeBase(ctx context.Context, a, b string) (string, error)

	// CommitsAhead returns how many commits branch has ahead of base.
	CommitsAhead(ctx context.Context, base, branch string) (int, error)

	// MergeTreeConflicts simulates a merge of branch onto base and returns
	// conflicting file paths. clean is true when exit 0 (no conflicts).
	MergeTreeConflicts(ctx context.Context, base, branch string) (conflicts []string, clean bool, err error)

	// ChangedFiles returns the files changed between base and branch (three-dot diff).
	ChangedFiles(ctx context.Context, base, branch string) ([]string, error)
}
