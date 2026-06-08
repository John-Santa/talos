package port

import "context"

// WorktreeEntry mirrors the JSON shape emitted by `wt list --json` (change #1).
// Field names and json tags match wt's listEntry exactly (figura/branch/path/head/status).
type WorktreeEntry struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}

// WorktreeLister is the outbound port for listing agent worktrees via the wt binary.
type WorktreeLister interface {
	// List returns all agent worktrees reported by `wt list --json`.
	List(ctx context.Context) ([]WorktreeEntry, error)
}
