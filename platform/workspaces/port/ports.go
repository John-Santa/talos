// Package port defines the outbound ports for the workspaces module.
package port

import (
	"context"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

// WorkspaceStore is the persistence port for workspace registry operations.
type WorkspaceStore interface {
	// List returns all registered workspaces.
	List(ctx context.Context) ([]workspace.Workspace, error)
	// Get returns the workspace by name; ErrNotFound if absent.
	Get(ctx context.Context, name string) (workspace.Workspace, error)
	// Add registers a new workspace; ErrDuplicate if name already exists.
	Add(ctx context.Context, ws workspace.Workspace) error
	// Remove deletes the workspace by name; ErrNotFound if absent.
	Remove(ctx context.Context, name string) error
	// SetActive marks the named workspace as active; ErrNotFound if absent.
	SetActive(ctx context.Context, name string) error
	// Active returns the active workspace name, or "" if none is set.
	Active(ctx context.Context) (string, error)
}

// Credentials holds the secret values for a workspace.
type Credentials struct {
	Email    string
	APIToken string
}

// CredentialVault is the storage port for workspace credentials.
type CredentialVault interface {
	// Store writes credentials for the given ref; overwrites if exists.
	Store(ctx context.Context, ref string, c Credentials) error
	// Load retrieves credentials for the given ref; ErrNotFound if absent.
	Load(ctx context.Context, ref string) (Credentials, error)
	// Delete removes credentials for the given ref; no-op if absent.
	Delete(ctx context.Context, ref string) error
}

// RepoProbe validates and resolves repository paths.
type RepoProbe interface {
	// Resolve returns the absolute, symlink-cleaned path; returns ErrRepoNotFound if path does not exist.
	Resolve(path string) (string, error)
	// IsGitRepo reports whether the path is inside a git work tree.
	IsGitRepo(path string) (bool, error)
}

// Materializer writes workspace configuration to the repo's runtime files.
type Materializer interface {
	// Apply writes <repo>/.talos/project.env (non-secret) and <repo>/.env (secret, 0600).
	Apply(ctx context.Context, ws workspace.Workspace, c Credentials) error
}
