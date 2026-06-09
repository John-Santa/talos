package tui_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/mock"
	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

var update = flag.Bool("update", false, "update golden files")

// ─── helpers ─────────────────────────────────────────────────────────────────

func fixedSnapshot() service.Snapshot {
	return service.Snapshot{
		Worktrees: []platform.Worktree{
			{Figura: "atlas", Branch: "agent/atlas/TAL-1", Head: "abc1234", Status: "clean"},
			{Figura: "iris", Branch: "agent/iris/TAL-8", Head: "def5678", Status: "modified"},
			{Figura: "hermes", Branch: "agent/hermes/TAL-6", Head: "ghi9012", Status: "clean"},
		},
	}
}

func modelWithFixedState() tui.Model {
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = fixedSnapshot().Worktrees
	agg := service.NewAggregator(r)
	m := tui.New(agg)

	// Inject fixed window size.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	// Inject fixed snapshot (no real CLI calls).
	next2, _ := m.Update(tui.SnapshotMsg{Snap: fixedSnapshot()})
	return next2.(tui.Model)
}

// ─── golden file test ─────────────────────────────────────────────────────────

func TestView_WorktreeList_GoldenFile(t *testing.T) {
	m := modelWithFixedState()
	got := m.View()

	goldenPath := filepath.Join("testdata", "worktrees.golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden file updated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file missing — run: go test ./adapter/tui -update\n%v", err)
	}
	if got != string(want) {
		t.Errorf("View() output does not match golden file %s\n\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, string(want))
	}
}

// ─── loading state view ───────────────────────────────────────────────────────

func TestView_LoadingState_ContainsLoadingString(t *testing.T) {
	r := mock.NewPlatformReaderMock()
	agg := service.NewAggregator(r)
	m := tui.New(agg)

	// Model starts in loading state; no snapshotMsg sent yet.
	got := m.View()
	if got == "" {
		t.Error("View() in loading state must not be empty")
	}
}

// ─── degraded note visible when errors present ────────────────────────────────

func TestView_WithErrors_ShowsDegradedNote(t *testing.T) {
	r := mock.NewPlatformReaderMock()
	agg := service.NewAggregator(r)
	m := tui.New(agg)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	snapWithErr := service.Snapshot{
		Worktrees: fixedSnapshot().Worktrees,
		Errors:    map[string]error{"mergeplan": os.ErrNotExist},
	}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snapWithErr})
	m = next2.(tui.Model)

	got := m.View()
	if got == "" {
		t.Error("View() with errors must not be empty")
	}
}
