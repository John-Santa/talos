package tomlstore_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/adapter/tomlstore"
	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

func makeWS(name string) workspace.Workspace {
	return workspace.Workspace{
		Name:     name,
		RepoPath: "/abs/" + name,
		Jira: workspace.JiraBinding{
			SiteURL:       "https://example.atlassian.net",
			ProjectKey:    "EX",
			ProjectID:     "10001",
			IssueTypeName: "Story",
			StateMapping: map[workspace.StatusCategory][]string{
				workspace.CategoryNew:           {"To Do"},
				workspace.CategoryIndeterminate: {"In Progress"},
				workspace.CategoryDone:          {"Done"},
			},
		},
		CredRef: name,
	}
}

func newStore(t *testing.T) *tomlstore.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := tomlstore.New(dir)
	if err != nil {
		t.Fatalf("tomlstore.New: %v", err)
	}
	return s
}

func TestStore_AddAndList(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	ws := makeWS("my-project")
	if err := s.Add(ctx, ws); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len=%d, want 1", len(list))
	}
	if list[0].Name != "my-project" {
		t.Errorf("List()[0].Name=%q, want %q", list[0].Name, "my-project")
	}
}

func TestStore_Add_Duplicate(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	ws := makeWS("proj")
	_ = s.Add(ctx, ws)
	err := s.Add(ctx, ws)
	var target *workspace.ErrDuplicate
	if !errors.As(err, &target) {
		t.Errorf("expected ErrDuplicate, got %T: %v", err, err)
	}
}

func TestStore_Get(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	ws := makeWS("proj")
	_ = s.Add(ctx, ws)

	got, err := s.Get(ctx, "proj")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Name != "proj" {
		t.Errorf("Get().Name=%q, want %q", got.Name, "proj")
	}
	if got.RepoPath != ws.RepoPath {
		t.Errorf("Get().RepoPath=%q, want %q", got.RepoPath, ws.RepoPath)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestStore_Remove(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_ = s.Add(ctx, makeWS("proj"))
	if err := s.Remove(ctx, "proj"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	list, _ := s.List(ctx)
	if len(list) != 0 {
		t.Errorf("List() after Remove len=%d, want 0", len(list))
	}
}

func TestStore_Remove_NotFound(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	err := s.Remove(ctx, "nonexistent")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestStore_SetActiveAndActive(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_ = s.Add(ctx, makeWS("proj-a"))
	_ = s.Add(ctx, makeWS("proj-b"))

	if err := s.SetActive(ctx, "proj-a"); err != nil {
		t.Fatalf("SetActive() error: %v", err)
	}

	name, err := s.Active(ctx)
	if err != nil {
		t.Fatalf("Active() error: %v", err)
	}
	if name != "proj-a" {
		t.Errorf("Active()=%q, want %q", name, "proj-a")
	}
}

func TestStore_Active_EmptyWhenNone(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	name, err := s.Active(ctx)
	if err != nil {
		t.Fatalf("Active() error: %v", err)
	}
	if name != "" {
		t.Errorf("Active()=%q, want empty", name)
	}
}

func TestStore_SetActive_NotFound(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	err := s.SetActive(ctx, "nonexistent")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestStore_Persistence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// write via first instance
	s1, _ := tomlstore.New(dir)
	ctx := context.Background()
	_ = s1.Add(ctx, makeWS("persisted"))
	_ = s1.SetActive(ctx, "persisted")

	// read via second instance (same dir)
	s2, _ := tomlstore.New(dir)
	list, err := s2.List(ctx)
	if err != nil {
		t.Fatalf("List() via second instance: %v", err)
	}
	if len(list) != 1 || list[0].Name != "persisted" {
		t.Errorf("second instance List()=%+v, want [{Name:persisted ...}]", list)
	}

	active, _ := s2.Active(ctx)
	if active != "persisted" {
		t.Errorf("second instance Active()=%q, want %q", active, "persisted")
	}
}

func TestStore_FileCreatedInTalosHome(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := tomlstore.New(dir)
	ctx := context.Background()

	_ = s.Add(ctx, makeWS("proj"))

	tomlPath := filepath.Join(dir, "workspaces.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		t.Errorf("workspaces.toml not created at %q", tomlPath)
	}
}

func TestStore_StateMapping_RoundTrip(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	ws := makeWS("proj")
	ws.Jira.StateMapping[workspace.CategoryNew] = []string{"Backlog", "To Do"}
	ws.Jira.StateMapping[workspace.CategoryIndeterminate] = []string{"In Progress", "Review"}

	_ = s.Add(ctx, ws)

	got, err := s.Get(ctx, "proj")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if len(got.Jira.StateMapping[workspace.CategoryNew]) != 2 {
		t.Errorf("StateMapping[new] len=%d, want 2", len(got.Jira.StateMapping[workspace.CategoryNew]))
	}
	if len(got.Jira.StateMapping[workspace.CategoryIndeterminate]) != 2 {
		t.Errorf("StateMapping[indeterminate] len=%d, want 2", len(got.Jira.StateMapping[workspace.CategoryIndeterminate]))
	}
}
