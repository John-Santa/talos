// Package service contains the ci-checks use cases.
package service

// Config holds the runtime configuration for the ci-checks service.
type Config struct {
	// Project is the Jira project key prefix used to validate issue keys.
	Project string
	// OwnershipPath is the path to the ownership Markdown file relative to repo root.
	OwnershipPath string
	// RequiredKeys are the label keys that must each appear exactly once on the issue.
	RequiredKeys []string
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
func DefaultTALConfig() Config {
	return Config{
		Project:       "TAL",
		OwnershipPath: "team-context/ownership.md",
		RequiredKeys:  []string{"agent", "module"},
	}
}
