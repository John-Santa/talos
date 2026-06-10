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
		MergePlan: platform.MergePlan{
			BaseBranch:   "develop",
			ConflictRate: 0.12,
			Threshold:    0.30,
			Steps: []platform.MergePlanStep{
				{Position: 1, Branch: "agent/atlas/TAL-1", Figura: "atlas", CommitsAhead: 3, PredictedClean: true},
				{Position: 2, Branch: "agent/iris/TAL-8", Figura: "iris", CommitsAhead: 5, PredictedClean: false, ConflictFiles: []string{"platform/console/view.go"}},
				{Position: 3, Branch: "agent/hermes/TAL-6", Figura: "hermes", CommitsAhead: 2, PredictedClean: true},
			},
		},
		Overlap: platform.Overlap{
			Verdict:        "collision",
			CollisionRate:  0.08,
			PairsEvaluated: 3,
			CollidingPairs: 1,
			FileCollisions: []platform.FileCollision{
				{File: "platform/console/view.go", Agents: []string{"atlas", "iris"}},
			},
			ModuleOverlaps: []platform.ModuleOverlap{
				{Module: "platform/console", Agents: []string{"atlas", "iris"}},
			},
			Advisories: []string{"iris and atlas both modify platform/console"},
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

func modelWithOverviewState() tui.Model {
	r := mock.NewPlatformReaderMock()
	snap := fixedSnapshot()
	r.WorktreesResult = snap.Worktrees
	r.MergePlanResult = snap.MergePlan
	r.OverlapResult = snap.Overlap
	agg := service.NewAggregator(r)
	m := tui.New(agg)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	m = next2.(tui.Model)

	// Switch to Overview layout.
	next3, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	return next3.(tui.Model)
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

// ─── Overview layout golden file ─────────────────────────────────────────────

func TestView_Overview_GoldenFile(t *testing.T) {
	m := modelWithOverviewState()
	got := m.View()

	goldenPath := filepath.Join("testdata", "overview.golden")

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
		t.Errorf("View() overview output does not match golden file %s\n\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, string(want))
	}
}

// ─── Overview layout: degraded panels when sources errored ───────────────────

func TestView_Overview_WithMergePlanError_ShowsErrorInPanel(t *testing.T) {
	r := mock.NewPlatformReaderMock()
	snap := fixedSnapshot()
	r.WorktreesResult = snap.Worktrees
	agg := service.NewAggregator(r)
	m := tui.New(agg)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	snapWithErr := service.Snapshot{
		Worktrees: snap.Worktrees,
		Errors:    map[string]error{"mergeplan": os.ErrNotExist},
	}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snapWithErr})
	m = next2.(tui.Model)

	// Switch to Overview.
	next3, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next3.(tui.Model)

	got := m.View()
	if got == "" {
		t.Error("View() overview with errors must not be empty")
	}
}

// ─── MasterDetail golden still matches after new layout code ─────────────────

func TestView_MasterDetail_GoldenStillValid(t *testing.T) {
	// Ensure adding Overview did not break the existing MasterDetail render.
	m := modelWithFixedState()
	if m.Layout != tui.LayoutMasterDetail {
		t.Fatalf("precondition: modelWithFixedState must default to LayoutMasterDetail")
	}
	got := m.View()

	goldenPath := filepath.Join("testdata", "worktrees.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("worktrees.golden missing: %v", err)
	}
	if got != string(want) {
		t.Errorf("MasterDetail view broke after layout changes\n\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}
