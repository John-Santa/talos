// Package service contains the EvidenceLoop use case and its configuration.
// It depends only on domain/evidence and port — never on adapter/rest.
package service

import "fmt"

// StatusCategory constants mirror Jira's statusCategory.key values. They are
// locale-independent (Decision 3 — category-primary transition resolution).
const (
	StatusCategoryNew           = "new"
	StatusCategoryIndeterminate = "indeterminate"
	StatusCategoryDone          = "done"
)

// Credentials holds the Jira authentication data sourced exclusively from
// environment variables. They must never appear in flags, logs, or config files.
type Credentials struct {
	Email    string
	APIToken string
}

// Config is the fully validated configuration injected into EvidenceLoop.
// No values are hardcoded outside this struct — instance values (site, project,
// issue type, board states) are injected at construction time (REQ-CFG).
type Config struct {
	// SiteURL is the Jira Cloud base URL, e.g. "https://myorg.atlassian.net".
	SiteURL string

	// CloudID is the Jira Cloud tenant ID (used by some v3 endpoints).
	CloudID string

	// ProjectKey is the Jira project key, e.g. "TAL".
	ProjectKey string

	// ProjectID is the Jira project numeric ID, e.g. "10099".
	ProjectID string

	// IssueTypeName is the issue type name to use when creating issues, e.g. "Tarea".
	IssueTypeName string

	// OrderedStates maps each StatusCategory to an ordered slice of state
	// names. The ordering drives ordinal-secondary disambiguation when
	// multiple transitions share the same category (e.g. 3 indeterminate
	// states on the TAL board: En curso, En revisión, Bloqueado).
	//
	// Real TAL board values (captured 2026-06-06):
	//   new:           ["Por hacer"]
	//   indeterminate: ["En curso", "En revisión", "Bloqueado"]
	//   done:          ["Listo"]
	OrderedStates map[string][]string

	// DefaultWorklogSeconds is the fallback time-spent value when the caller
	// does not supply one explicitly.
	DefaultWorklogSeconds int

	// RemoteLinkRelationship is the human-readable relation label for remote
	// links, e.g. "is implemented by".
	RemoteLinkRelationship string

	// Credentials holds Basic-auth data from env vars (REQ-AUTH).
	Credentials Credentials
}

// NewConfig validates the provided Config and returns it if valid, or an error
// describing the first violation found.
func NewConfig(c Config) (Config, error) {
	if c.SiteURL == "" {
		return Config{}, fmt.Errorf("config: SiteURL is required")
	}
	if c.ProjectKey == "" {
		return Config{}, fmt.Errorf("config: ProjectKey is required")
	}
	if c.ProjectID == "" {
		return Config{}, fmt.Errorf("config: ProjectID is required")
	}
	if c.IssueTypeName == "" {
		return Config{}, fmt.Errorf("config: IssueTypeName is required")
	}
	if len(c.OrderedStates) == 0 {
		return Config{}, fmt.Errorf("config: OrderedStates must not be empty")
	}
	required := []string{StatusCategoryNew, StatusCategoryIndeterminate, StatusCategoryDone}
	for _, cat := range required {
		if len(c.OrderedStates[cat]) == 0 {
			return Config{}, fmt.Errorf("config: OrderedStates[%q] must have at least one entry", cat)
		}
	}
	if c.RemoteLinkRelationship == "" {
		return Config{}, fmt.Errorf("config: RemoteLinkRelationship is required")
	}
	if c.Credentials.Email == "" {
		return Config{}, fmt.Errorf("config: Credentials.Email is required")
	}
	if c.Credentials.APIToken == "" {
		return Config{}, fmt.Errorf("config: Credentials.APIToken is required")
	}
	return c, nil
}

// DefaultTALConfig returns a Config seeded with the real TAL board values.
// Callers must still supply SiteURL, CloudID, and Credentials before the
// config is valid.
func DefaultTALConfig() Config {
	return Config{
		ProjectKey:    "TAL",
		ProjectID:     "10099",
		IssueTypeName: "Tarea",
		OrderedStates: map[string][]string{
			StatusCategoryNew:           {"Por hacer"},
			StatusCategoryIndeterminate: {"En curso", "En revisión", "Bloqueado"},
			StatusCategoryDone:          {"Listo"},
		},
		DefaultWorklogSeconds:  3600,
		RemoteLinkRelationship: "is implemented by",
	}
}
