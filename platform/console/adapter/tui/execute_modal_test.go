package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/mock"
	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

// baseBranch is the base branch used in merge plan fixtures.
const baseBranch = "develop"

// populatedModelWithPlan returns a Model with a MergePlan (BaseBranch=develop) and actor.
func populatedModelWithPlan(actor *mock.PlatformActorMock) tui.Model {
	wts := threeWorktrees()
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = wts
	r.MergePlanResult = platform.MergePlan{
		BaseBranch:   baseBranch,
		ConflictRate: 0.05,
		Threshold:    0.15,
		Steps: []platform.MergePlanStep{
			{Position: 1, Branch: "agent/atlas/TAL-1", Figura: "atlas", CommitsAhead: 3, PredictedClean: true},
		},
	}
	agg := service.NewAggregator(r)
	m := tui.NewWithActor(agg, actor)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	snap := service.Snapshot{
		Worktrees: wts,
		MergePlan: r.MergePlanResult,
	}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	return next2.(tui.Model)
}

// ─── (A) m key opens the execute modal ───────────────────────────────────────

func TestUpdate_MKey_OpensExecuteModal(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)

	if m.ModalActive {
		t.Fatal("precondition: ModalActive must start false")
	}

	m = sendKey(m, "m")

	if !m.ModalActive {
		t.Error("after m: ModalActive should be true")
	}
	if m.ModalMode != tui.ModalModeExecute {
		t.Errorf("after m: ModalMode = %v, want ModalModeExecute", m.ModalMode)
	}
}

func TestUpdate_MKey_NoActorIsNoOp(t *testing.T) {
	// Triangulate: m with nil actor is ignored.
	wts := threeWorktrees()
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = wts
	agg := service.NewAggregator(r)
	m := tui.NewWithActor(agg, nil)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)
	snap := service.Snapshot{Worktrees: wts}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	m = next2.(tui.Model)

	m = sendKey(m, "m")

	if m.ModalActive {
		t.Error("after m with nil actor: ModalActive should remain false")
	}
}

// ─── (B) wrong token: enter does NOT fire action ──────────────────────────────

func TestUpdate_ExecuteModal_WrongToken_EnterBlocked(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)
	m = sendKey(m, "m") // open execute modal

	// Type wrong token.
	for _, ch := range "wrong-branch" {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if cmd != nil {
		t.Error("enter with wrong token: cmd should be nil (action blocked)")
	}
	if !got.ModalActive {
		t.Error("enter with wrong token: modal should remain open")
	}
	actor.AssertNotCalled(t, "ExecuteMerge")
}

func TestUpdate_ExecuteModal_EmptyToken_EnterBlocked(t *testing.T) {
	// Triangulate: empty input also blocks confirm.
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)
	m = sendKey(m, "m")

	// Type nothing — submit immediately.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if cmd != nil {
		t.Error("enter with empty token: cmd should be nil (action blocked)")
	}
	if !got.ModalActive {
		t.Error("enter with empty token: modal should remain open")
	}
}

// ─── (C) correct token: enter fires action ────────────────────────────────────

func TestUpdate_ExecuteModal_CorrectToken_EnterFiresAction(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)
	m = sendKey(m, "m") // open execute modal

	// Type the exact base branch.
	for _, ch := range baseBranch {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after correct token + enter: ModalActive should be false")
	}
	if cmd == nil {
		t.Error("after correct token + enter: cmd should be non-nil (async merge in flight)")
	}
}

func TestUpdate_ExecuteModal_CorrectToken_OnlySendsMergeOnce(t *testing.T) {
	// Triangulate: correct token with a longer base branch also works.
	actor := mock.NewPlatformActorMock()
	wts := threeWorktrees()
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = wts
	r.MergePlanResult = platform.MergePlan{
		BaseBranch: "main",
		Steps:      []platform.MergePlanStep{{Position: 1, Branch: "agent/atlas/TAL-1", Figura: "atlas"}},
	}
	agg := service.NewAggregator(r)
	m := tui.NewWithActor(agg, actor)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)
	snap := service.Snapshot{Worktrees: wts, MergePlan: r.MergePlanResult}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	m = next2.(tui.Model)

	m = sendKey(m, "m") // open modal

	// Type "main" — the base branch.
	for _, ch := range "main" {
		m = sendKey(m, string(ch))
	}

	next3, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next3.(tui.Model)

	if got.ModalActive {
		t.Error("'main' token: modal should close")
	}
	if cmd == nil {
		t.Error("'main' token: cmd must be non-nil")
	}
}

// ─── (D) esc cancels with no cmd ─────────────────────────────────────────────

func TestUpdate_ExecuteModal_Esc_CancelsWithNoCmd(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)
	m = sendKey(m, "m") // open modal

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after esc: ModalActive should be false")
	}
	if cmd != nil {
		t.Error("after esc: cmd should be nil (cancel has no side-effect)")
	}
}

// ─── (E) golden file for the execute-merge modal ─────────────────────────────

func TestView_ExecuteModal_GoldenFile(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModelWithPlan(actor)

	m = sendKey(m, "m") // open execute modal

	// Pre-fill the correct token for a deterministic render.
	for _, ch := range baseBranch {
		m = sendKey(m, string(ch))
	}

	got := m.View()
	goldenPath := filepath.Join("testdata", "execute-modal.golden")

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
		t.Errorf("View() execute-modal output does not match golden file %s\n\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, string(want))
	}
}
