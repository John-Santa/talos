package worktree

import (
	"fmt"
	"regexp"
)

// Figura is the validated name of an agent from the CONSTITUTION §1 roster.
type Figura string

// roster is the authoritative list of valid figura values (CONSTITUTION §1).
var roster = map[string]struct{}{
	"atlas":      {},
	"hephaestus": {},
	"cronos":     {},
	"iris":       {},
	"gaia":       {},
	"themis":     {},
	"hermes":     {},
	"argos":      {},
}

// jiraKeyRe matches valid Jira keys: TAL-<n> where n >= 1 (no leading zeros, no zero).
var jiraKeyRe = regexp.MustCompile(`^TAL-[1-9][0-9]*$`)

// ParseFigura validates and returns a Figura from a raw string.
// Returns ErrInvalidFigure if the string is not in the CONSTITUTION §1 roster.
func ParseFigura(s string) (Figura, error) {
	if _, ok := roster[s]; !ok {
		return "", &ErrInvalidFigure{Figura: s}
	}
	return Figura(s), nil
}

// ValidateJiraKey checks that key matches the TAL-<n> pattern (n >= 1).
// Returns ErrInvalidKey if it does not match.
func ValidateJiraKey(key string) error {
	if !jiraKeyRe.MatchString(key) {
		return &ErrInvalidKey{Key: key}
	}
	return nil
}

// BranchName returns the canonical branch name for a given figura and jira key.
// Format: agent/<figura>/<TAL-N>
func BranchName(f Figura, jiraKey string) string {
	return fmt.Sprintf("agent/%s/%s", f, jiraKey)
}

// WorktreePath returns the filesystem path for a worktree given a base directory
// and figura. Format: <base>/agent-<figura>
func WorktreePath(base string, f Figura) string {
	return fmt.Sprintf("%s/agent-%s", base, f)
}

// WorktreeSpec holds the validated, derived fields for a single worktree operation.
type WorktreeSpec struct {
	Figura  Figura
	JiraKey string
	Branch  string
	Path    string
}

// NewWorktreeSpec validates figura and jiraKey, then builds and returns a
// WorktreeSpec with pre-derived Branch and Path fields.
// Returns ErrInvalidFigura or ErrInvalidKey on validation failure.
func NewWorktreeSpec(figura, jiraKey, worktreeBase string) (WorktreeSpec, error) {
	f, err := ParseFigura(figura)
	if err != nil {
		return WorktreeSpec{}, err
	}
	if err := ValidateJiraKey(jiraKey); err != nil {
		return WorktreeSpec{}, err
	}
	return WorktreeSpec{
		Figura:  f,
		JiraKey: jiraKey,
		Branch:  BranchName(f, jiraKey),
		Path:    WorktreePath(worktreeBase, f),
	}, nil
}
