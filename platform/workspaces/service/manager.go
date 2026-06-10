// Package service contains the workspace registry use cases.
package service

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

// WorkspaceView is the read model returned to callers; never exposes credentials.
type WorkspaceView struct {
	Name           string
	RepoPath       string
	Jira           workspace.JiraBinding
	Active         bool
	HasCredentials bool
}

// Manager is the primary use-case service for the workspace registry.
type Manager struct {
	store port.WorkspaceStore
	vault port.CredentialVault
	probe port.RepoProbe
	mat   port.Materializer
}

// NewManager constructs a Manager with the given outbound adapters.
func NewManager(store port.WorkspaceStore, vault port.CredentialVault, probe port.RepoProbe, mat port.Materializer) *Manager {
	return &Manager{store: store, vault: vault, probe: probe, mat: mat}
}

// Add validates and registers a new workspace and stores its credentials.
// Returns ErrInvalidName, ErrRepoNotFound, ErrNotGitRepo, ErrIncompleteBinding, or ErrDuplicate.
func (m *Manager) Add(ctx context.Context, name, repoPath string, jira workspace.JiraBinding, creds port.Credentials) error {
	ws, err := workspace.NewWorkspace(name, repoPath, jira, m.probe)
	if err != nil {
		return err
	}

	if err := m.store.Add(ctx, ws); err != nil {
		return err
	}

	if err := m.vault.Store(ctx, ws.CredRef, creds); err != nil {
		// best-effort rollback: remove from store to avoid orphaned entries
		_ = m.store.Remove(ctx, ws.Name)
		return fmt.Errorf("storing credentials for %q: %w", name, err)
	}

	return nil
}

// Remove deletes a workspace registration and optionally purges its credentials.
func (m *Manager) Remove(ctx context.Context, name string, purgeCreds bool) error {
	if err := m.store.Remove(ctx, name); err != nil {
		return err
	}

	if purgeCreds {
		if err := m.vault.Delete(ctx, name); err != nil {
			return fmt.Errorf("purging credentials for %q: %w", name, err)
		}
	}

	return nil
}

// Use marks the named workspace as active and materializes its config files.
func (m *Manager) Use(ctx context.Context, name string) error {
	ws, err := m.store.Get(ctx, name)
	if err != nil {
		return err
	}

	creds, err := m.vault.Load(ctx, ws.CredRef)
	if err != nil {
		return fmt.Errorf("loading credentials for %q: %w", name, err)
	}

	if err := m.store.SetActive(ctx, name); err != nil {
		return err
	}

	if err := m.mat.Apply(ctx, ws, creds); err != nil {
		return fmt.Errorf("materializing workspace %q: %w", name, err)
	}

	return nil
}

// List returns all workspaces with the active flag set on the current active workspace.
func (m *Manager) List(ctx context.Context) ([]WorkspaceView, error) {
	workspaces, err := m.store.List(ctx)
	if err != nil {
		return nil, err
	}

	activeName, err := m.store.Active(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]WorkspaceView, 0, len(workspaces))
	for _, ws := range workspaces {
		views = append(views, WorkspaceView{
			Name:     ws.Name,
			RepoPath: ws.RepoPath,
			Jira:     ws.Jira,
			Active:   ws.Name == activeName,
		})
	}
	return views, nil
}

// Show returns the detail view for a named workspace, including credential presence.
func (m *Manager) Show(ctx context.Context, name string) (WorkspaceView, error) {
	ws, err := m.store.Get(ctx, name)
	if err != nil {
		return WorkspaceView{}, err
	}

	activeName, err := m.store.Active(ctx)
	if err != nil {
		return WorkspaceView{}, err
	}

	_, credErr := m.vault.Load(ctx, ws.CredRef)
	hasCreds := credErr == nil

	return WorkspaceView{
		Name:           ws.Name,
		RepoPath:       ws.RepoPath,
		Jira:           ws.Jira,
		Active:         ws.Name == activeName,
		HasCredentials: hasCreds,
	}, nil
}

// Current returns the name of the active workspace, or "" if none.
func (m *Manager) Current(ctx context.Context) (string, error) {
	return m.store.Active(ctx)
}
