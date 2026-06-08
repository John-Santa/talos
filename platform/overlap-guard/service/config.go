// Package service contains the overlap-guard use cases.
package service

// Config holds the runtime configuration for overlap-guard.
type Config struct {
	// RepoRoot is the absolute path to the git repository root.
	RepoRoot string
	// BaseBranch is the branch candidates are compared against; defaults to "develop".
	BaseBranch string
	// WtBinary is the wt CLI binary name on PATH; defaults to "wt".
	WtBinary string
	// SiteURL is the Jira site URL for T0 checks.
	SiteURL string
	// Project is the Jira project key; defaults to "TAL".
	Project string
	// Threshold is the CollisionRate threshold for HG6; defaults to 0.15.
	Threshold float64
	// NoFetch skips the initial git fetch when true.
	NoFetch bool
	// MaxResults caps the Jira search result count for T0 checks; defaults to 100.
	MaxResults int
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
func DefaultTALConfig() Config {
	return Config{
		BaseBranch: "develop",
		WtBinary:   "wt",
		Project:    "TAL",
		Threshold:  0.15,
		MaxResults: 100,
	}
}
