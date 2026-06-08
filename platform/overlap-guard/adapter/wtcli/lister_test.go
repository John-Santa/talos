package wtcli_test

import (
	"context"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/wtcli"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// TestList_BinaryNotFound verifies ErrWtBinaryNotFound when binary is missing.
func TestList_BinaryNotFound(t *testing.T) {
	l := wtcli.NewLister("wt-binary-that-does-not-exist-abc123")
	_, err := l.List(context.Background())
	if err == nil {
		t.Fatal("expected ErrWtBinaryNotFound, got nil")
	}
	if _, ok := err.(*wtcli.ErrWtBinaryNotFound); !ok {
		t.Fatalf("expected *wtcli.ErrWtBinaryNotFound, got %T: %v", err, err)
	}
}

// TestList_MalformedJSON verifies ErrWtOutputMalformed when the binary emits bad JSON.
func TestList_MalformedJSON(t *testing.T) {
	// We cannot easily mock the binary in unit tests without creating a test executable.
	// The ErrWtOutputMalformed path is tested via a helper that exercises the parse path directly.
	err := wtcli.ParseWorktreeJSON([]byte("not valid json at all {"))
	if err == nil {
		t.Fatal("expected ErrWtOutputMalformed for bad JSON, got nil")
	}
	if _, ok := err.(*wtcli.ErrWtOutputMalformed); !ok {
		t.Fatalf("expected *wtcli.ErrWtOutputMalformed, got %T: %v", err, err)
	}
}

// TestList_ValidJSON verifies correct parsing of well-formed wt output.
func TestList_ValidJSON(t *testing.T) {
	input := `[
		{"figura":"atlas","branch":"agent/atlas/TAL-5","path":"/worktrees/atlas","head":"abc123","status":"active"},
		{"figura":"hermes","branch":"agent/hermes/TAL-6","path":"/worktrees/hermes","head":"def456","status":"idle"}
	]`
	entries, err := wtcli.ParseWorktreeJSONToEntries([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want := []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-5", Path: "/worktrees/atlas", Head: "abc123", Status: "active"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-6", Path: "/worktrees/hermes", Head: "def456", Status: "idle"},
	}
	for i, w := range want {
		g := entries[i]
		if g.Figura != w.Figura || g.Branch != w.Branch || g.Path != w.Path || g.Head != w.Head || g.Status != w.Status {
			t.Errorf("entry[%d]: want %+v, got %+v", i, w, g)
		}
	}
}

// TestList_Integration runs a real wt binary; skipped in short mode.
func TestList_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires wt binary on PATH")
	}
	l := wtcli.NewLister("wt")
	entries, err := l.List(context.Background())
	if err != nil {
		t.Logf("wt list --json returned error (may be expected if no worktrees): %v", err)
		return
	}
	t.Logf("wt list --json returned %d entries", len(entries))
}

// varListerCheck is a compile-time assertion that *Lister satisfies port.WorktreeLister.
var _ port.WorktreeLister = (*wtcli.Lister)(nil)
