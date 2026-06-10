package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/mock"
	"github.com/John-Santa/talos/platform/workspaces/port"
	"github.com/John-Santa/talos/platform/workspaces/service"
)

func makeBinding() workspace.JiraBinding {
	return workspace.JiraBinding{
		SiteURL:       "https://example.atlassian.net",
		ProjectKey:    "EX",
		ProjectID:     "10001",
		IssueTypeName: "Story",
		StateMapping: map[workspace.StatusCategory][]string{
			workspace.CategoryNew:           {"To Do"},
			workspace.CategoryIndeterminate: {"In Progress"},
			workspace.CategoryDone:          {"Done"},
		},
	}
}

func makeCreds() port.Credentials {
	return port.Credentials{Email: "test@example.com", APIToken: "secret-token"}
}

func newManager(store *mock.WorkspaceStoreMock, vault *mock.CredentialVaultMock, probe *mock.ProbeMock, mat *mock.MaterializerMock) *service.Manager {
	return service.NewManager(store, vault, probe, mat)
}

func TestManager_Add_HappyPath(t *testing.T) {
	t.Parallel()
	store := &mock.WorkspaceStoreMock{}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	creds := makeCreds()
	err := mgr.Add(context.Background(), "my-project", "/abs/repo", makeBinding(), creds)
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	// verify store received Add call
	if len(store.CallsFor("Add")) != 1 {
		t.Errorf("store.Add called %d time(s), want 1", len(store.CallsFor("Add")))
	}
	// verify vault received Store call
	if len(vault.Calls) != 1 || vault.Calls[0].Method != "Store" {
		t.Errorf("vault.Store not called; calls: %+v", vault.Calls)
	}
	stored := vault.Store_["my-project"]
	if stored.Email != creds.Email {
		t.Errorf("vault stored email %q, want %q", stored.Email, creds.Email)
	}
}

func TestManager_Add_InvalidName(t *testing.T) {
	t.Parallel()
	store := &mock.WorkspaceStoreMock{}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Add(context.Background(), "Bad Name", "/abs/repo", makeBinding(), makeCreds())
	var target *workspace.ErrInvalidName
	if !errors.As(err, &target) {
		t.Errorf("expected ErrInvalidName, got %T: %v", err, err)
	}
}

func TestManager_Add_Duplicate(t *testing.T) {
	t.Parallel()
	store := &mock.WorkspaceStoreMock{AddErr: &workspace.ErrDuplicate{Name: "existing"}}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Add(context.Background(), "existing", "/abs/repo", makeBinding(), makeCreds())
	var target *workspace.ErrDuplicate
	if !errors.As(err, &target) {
		t.Errorf("expected ErrDuplicate, got %T: %v", err, err)
	}
}

func TestManager_Remove_HappyPath(t *testing.T) {
	t.Parallel()
	ws := workspace.Workspace{
		Name: "my-project", RepoPath: "/abs/repo",
		Jira: makeBinding(), CredRef: "my-project",
	}
	store := &mock.WorkspaceStoreMock{Workspaces: []workspace.Workspace{ws}}
	vault := mock.NewCredentialVaultMock()
	vault.Store_["my-project"] = makeCreds()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Remove(context.Background(), "my-project", true)
	if err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}
	// vault.Delete should have been called
	if len(vault.Calls) != 1 || vault.Calls[0].Method != "Delete" {
		t.Errorf("vault.Delete not called; calls: %+v", vault.Calls)
	}
}

func TestManager_Remove_NoPurgeCreds(t *testing.T) {
	t.Parallel()
	ws := workspace.Workspace{Name: "my-project", RepoPath: "/abs/repo", Jira: makeBinding(), CredRef: "my-project"}
	store := &mock.WorkspaceStoreMock{Workspaces: []workspace.Workspace{ws}}
	vault := mock.NewCredentialVaultMock()
	vault.Store_["my-project"] = makeCreds()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Remove(context.Background(), "my-project", false)
	if err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}
	// vault.Delete should NOT have been called
	for _, c := range vault.Calls {
		if c.Method == "Delete" {
			t.Error("vault.Delete was called but purgeCreds=false")
		}
	}
}

func TestManager_Use_HappyPath(t *testing.T) {
	t.Parallel()
	ws := workspace.Workspace{
		Name: "my-project", RepoPath: "/abs/repo",
		Jira: makeBinding(), CredRef: "my-project",
	}
	store := &mock.WorkspaceStoreMock{Workspaces: []workspace.Workspace{ws}}
	vault := mock.NewCredentialVaultMock()
	creds := makeCreds()
	vault.Store_["my-project"] = creds
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Use(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("Use() unexpected error: %v", err)
	}
	if len(store.CallsFor("SetActive")) != 1 {
		t.Errorf("store.SetActive not called")
	}
	if len(mat.Calls) != 1 {
		t.Errorf("materializer.Apply not called; calls: %+v", mat.Calls)
	}
	if mat.LastWS.Name != "my-project" {
		t.Errorf("materializer.Apply got ws.Name=%q, want %q", mat.LastWS.Name, "my-project")
	}
}

func TestManager_Use_NotFound(t *testing.T) {
	t.Parallel()
	store := &mock.WorkspaceStoreMock{}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	err := mgr.Use(context.Background(), "nonexistent")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestManager_List(t *testing.T) {
	t.Parallel()
	ws1 := workspace.Workspace{Name: "proj-a", RepoPath: "/abs/a", Jira: makeBinding(), CredRef: "proj-a"}
	ws2 := workspace.Workspace{Name: "proj-b", RepoPath: "/abs/b", Jira: makeBinding(), CredRef: "proj-b"}
	store := &mock.WorkspaceStoreMock{Workspaces: []workspace.Workspace{ws1, ws2}, ActiveName: "proj-a"}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	views, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(views))
	}
	// find the active one
	var activeSeen bool
	for _, v := range views {
		if v.Name == "proj-a" && v.Active {
			activeSeen = true
		}
	}
	if !activeSeen {
		t.Error("List() did not mark proj-a as active")
	}
}

func TestManager_Show(t *testing.T) {
	t.Parallel()
	ws := workspace.Workspace{Name: "my-project", RepoPath: "/abs/repo", Jira: makeBinding(), CredRef: "my-project"}
	store := &mock.WorkspaceStoreMock{Workspaces: []workspace.Workspace{ws}}
	vault := mock.NewCredentialVaultMock()
	vault.Store_["my-project"] = makeCreds()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	view, err := mgr.Show(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("Show() unexpected error: %v", err)
	}
	if view.Name != "my-project" {
		t.Errorf("Show() Name=%q, want %q", view.Name, "my-project")
	}
	if !view.HasCredentials {
		t.Error("Show() HasCredentials=false, want true")
	}
}

func TestManager_Current(t *testing.T) {
	t.Parallel()
	store := &mock.WorkspaceStoreMock{ActiveName: "my-project"}
	vault := mock.NewCredentialVaultMock()
	probe := mock.NewProbeMock("/abs/repo", true)
	mat := &mock.MaterializerMock{}
	mgr := newManager(store, vault, probe, mat)

	name, err := mgr.Current(context.Background())
	if err != nil {
		t.Fatalf("Current() unexpected error: %v", err)
	}
	if name != "my-project" {
		t.Errorf("Current()=%q, want %q", name, "my-project")
	}
}
