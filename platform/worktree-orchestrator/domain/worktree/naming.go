package worktree

import (
	"fmt"
	"regexp"
)

// Figura is the validated name of an agent.
type Figura string

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

var jiraKeyRe = regexp.MustCompile(`^TAL-[1-9][0-9]*$`)

// ParseFigura validates s against the agent roster and returns a Figura, or ErrInvalidFigure.
func ParseFigura(s string) (Figura, error) {
	if _, ok := roster[s]; !ok {
		return "", &ErrInvalidFigure{Figura: s}
	}
	return Figura(s), nil
}

// ValidateJiraKey checks that key matches TAL-<n> (n >= 1), returning ErrInvalidKey if not.
func ValidateJiraKey(key string) error {
	if !jiraKeyRe.MatchString(key) {
		return &ErrInvalidKey{Key: key}
	}
	return nil
}

// BranchName returns the canonical branch name agent/<figura>/<TAL-N>.
func BranchName(f Figura, jiraKey string) string {
	return fmt.Sprintf("agent/%s/%s", f, jiraKey)
}

// WorktreePath returns the filesystem path <base>/agent-<figura> for a worktree.
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

// NewWorktreeSpec validates figura and jiraKey, then builds a WorktreeSpec with pre-derived Branch and Path.
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
