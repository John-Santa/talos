// Package platform contains the pure domain types for the console module.
package platform

// Worktree represents a single agent worktree as reported by `wt list --json`.
// Fields map directly to the JSON keys emitted by the wt binary.
type Worktree struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}
