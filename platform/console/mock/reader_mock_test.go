package mock_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/mock"
)

func TestPlatformReaderMock_Worktrees_returnsResult(t *testing.T) {
	t.Parallel()

	want := []platform.Worktree{
		{Figura: "hermes", Branch: "agent/hermes/TAL-3", Path: "/tmp/t", Head: "abc123", Status: "active"},
	}

	m := mock.NewPlatformReaderMock()
	m.WorktreesResult = want

	got, err := m.Worktrees(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("want %d worktrees, got %d", len(want), len(got))
	}
	if got[0].Figura != want[0].Figura {
		t.Errorf("Figura = %q, want %q", got[0].Figura, want[0].Figura)
	}
	m.AssertCallCount(t, "Worktrees", 1)
}

func TestPlatformReaderMock_Worktrees_returnsError(t *testing.T) {
	t.Parallel()

	sentinel := fmt.Errorf("mock: programmed error")
	m := mock.NewPlatformReaderMock()
	m.WorktreesErr = sentinel

	_, err := m.Worktrees(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != sentinel {
		t.Errorf("want sentinel error, got %v", err)
	}
	m.AssertCallCount(t, "Worktrees", 1)
}
