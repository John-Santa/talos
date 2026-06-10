// Package workspace contains the pure domain logic for the workspaces module.
package workspace

import (
	"fmt"
	"regexp"
	"strings"
)

// StatusCategory classifies a Jira workflow state bucket.
type StatusCategory string

const (
	CategoryNew           StatusCategory = "new"
	CategoryIndeterminate StatusCategory = "indeterminate"
	CategoryDone          StatusCategory = "done"
)

// JiraBinding holds the Jira project configuration for a workspace.
type JiraBinding struct {
	SiteURL       string
	ProjectKey    string
	ProjectID     string
	IssueTypeName string
	// StateMapping maps each StatusCategory to a list of Jira status names.
	// All three categories must be present with at least one status each.
	StateMapping map[StatusCategory][]string
}

// Workspace represents a registered project with its Jira binding and credential reference.
type Workspace struct {
	Name     string
	RepoPath string // absolute, git repo
	Jira     JiraBinding
	CredRef  string // opaque vault key; equals Name by convention
}

// repoProber is the minimal interface NewWorkspace needs from the port.RepoProbe.
// Using a local interface avoids an import cycle between domain and port.
type repoProber interface {
	Resolve(path string) (string, error)
	IsGitRepo(path string) (bool, error)
}

// slugRe matches valid workspace names: lowercase alphanumeric + hyphens, non-empty,
// not starting or ending with a hyphen.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// NewWorkspace constructs and validates a Workspace.
// Validation is pure (except for the RepoProbe check, which is injected).
// Returns typed errors: ErrInvalidName, ErrRepoNotFound, ErrNotGitRepo, ErrIncompleteBinding.
func NewWorkspace(name, repoPath string, jira JiraBinding, probe repoProber) (Workspace, error) {
	if err := validateName(name); err != nil {
		return Workspace{}, err
	}

	absPath, err := probe.Resolve(repoPath)
	if err != nil {
		return Workspace{}, err
	}

	isGit, err := probe.IsGitRepo(absPath)
	if err != nil {
		return Workspace{}, fmt.Errorf("checking git repo: %w", err)
	}
	if !isGit {
		return Workspace{}, &ErrNotGitRepo{Path: absPath}
	}

	if err := validateBinding(jira); err != nil {
		return Workspace{}, err
	}

	return Workspace{
		Name:     name,
		RepoPath: absPath,
		Jira:     jira,
		CredRef:  name,
	}, nil
}

// validateName enforces the slug rule: lowercase alphanumeric and hyphens,
// not empty, not starting or ending with a hyphen.
func validateName(name string) error {
	if name == "" || !slugRe.MatchString(name) {
		return &ErrInvalidName{Name: name}
	}
	return nil
}

// validateBinding ensures all required Jira fields are present.
func validateBinding(b JiraBinding) error {
	switch {
	case strings.TrimSpace(b.SiteURL) == "":
		return &ErrIncompleteBinding{Missing: "SiteURL"}
	case strings.TrimSpace(b.ProjectKey) == "":
		return &ErrIncompleteBinding{Missing: "ProjectKey"}
	case strings.TrimSpace(b.ProjectID) == "":
		return &ErrIncompleteBinding{Missing: "ProjectID"}
	case strings.TrimSpace(b.IssueTypeName) == "":
		return &ErrIncompleteBinding{Missing: "IssueTypeName"}
	}

	required := []StatusCategory{CategoryNew, CategoryIndeterminate, CategoryDone}
	for _, cat := range required {
		statuses, ok := b.StateMapping[cat]
		if !ok || len(statuses) == 0 {
			return &ErrIncompleteBinding{Missing: fmt.Sprintf("StateMapping[%s] must have at least one status", cat)}
		}
	}

	return nil
}
