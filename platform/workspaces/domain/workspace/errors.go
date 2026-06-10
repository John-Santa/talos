// Package workspace contains the pure domain logic for the workspaces module.
package workspace

import "fmt"

// ErrInvalidName is returned when a workspace name is not a valid slug.
type ErrInvalidName struct {
	Name string
}

func (e *ErrInvalidName) Error() string {
	return fmt.Sprintf("invalid workspace name %q: must be a non-empty slug (alphanumeric, hyphens, no spaces)", e.Name)
}

// ErrRepoNotFound is returned when the repo path does not exist on the filesystem.
type ErrRepoNotFound struct {
	Path string
}

func (e *ErrRepoNotFound) Error() string {
	return fmt.Sprintf("repo path %q does not exist", e.Path)
}

// ErrNotGitRepo is returned when the repo path exists but is not a git repository.
type ErrNotGitRepo struct {
	Path string
}

func (e *ErrNotGitRepo) Error() string {
	return fmt.Sprintf("path %q is not a git repository", e.Path)
}

// ErrIncompleteBinding is returned when the Jira binding is missing required fields.
type ErrIncompleteBinding struct {
	Missing string
}

func (e *ErrIncompleteBinding) Error() string {
	return fmt.Sprintf("incomplete Jira binding: %s", e.Missing)
}

// ErrDuplicate is returned when a workspace name already exists in the store.
type ErrDuplicate struct {
	Name string
}

func (e *ErrDuplicate) Error() string {
	return fmt.Sprintf("workspace %q already exists", e.Name)
}

// ErrNotFound is returned when a workspace name does not exist in the store.
type ErrNotFound struct {
	Name string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("workspace %q not found", e.Name)
}
