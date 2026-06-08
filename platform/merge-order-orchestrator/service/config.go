// Package service contains the merge-order-orchestrator use cases.
package service

// Config holds the runtime configuration for the merge-order services.
type Config struct {
	// RepoRoot is the absolute path to the git repository root.
	RepoRoot string
	// BaseBranch is the integration branch candidates are merged into; defaults to "develop".
	BaseBranch string
	// WtBinary is the wt CLI binary name on PATH; defaults to "wt".
	WtBinary string
	// MaxConflictRate is the conflict rate threshold; SegmentationBad when strictly exceeded.
	MaxConflictRate float64
	// NoFetch skips the initial fetch when true.
	NoFetch bool
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
func DefaultTALConfig() Config {
	return Config{
		BaseBranch:      "develop",
		WtBinary:        "wt",
		MaxConflictRate: 0.15,
	}
}
