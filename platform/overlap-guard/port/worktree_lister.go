package port

import "context"

// WorktreeEntry mirrors the JSON shape emitted by `wt list --json` (change #1).
type WorktreeEntry struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}

// WorktreeLister is the outbound port for listing agent worktrees via the wt binary.
type WorktreeLister interface {
	List(ctx context.Context) ([]WorktreeEntry, error)
}
