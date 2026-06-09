package tui_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/mock"
	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func threeWorktrees() []platform.Worktree {
	return []platform.Worktree{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Head: "abc1234", Status: "clean"},
		{Figura: "iris", Branch: "agent/iris/TAL-8", Head: "def5678", Status: "modified"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-6", Head: "ghi9012", Status: "clean"},
	}
}

func newModel(wts []platform.Worktree) tui.Model {
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = wts
	agg := service.NewAggregator(r)
	return tui.New(agg)
}

func sendKey(m tui.Model, key string) tui.Model {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	next, _ := m.Update(msg)
	return next.(tui.Model)
}

func sendKeyType(m tui.Model, t tea.KeyType) tui.Model {
	msg := tea.KeyMsg{Type: t}
	next, _ := m.Update(msg)
	return next.(tui.Model)
}

// ─── (a) WindowSizeMsg sets width / height ────────────────────────────────────

func TestUpdate_WindowSizeMsg_SetsWidthAndHeight(t *testing.T) {
	m := newModel(nil)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := next.(tui.Model)

	if got.Width != 120 {
		t.Errorf("Width: got %d, want 120", got.Width)
	}
	if got.Height != 40 {
		t.Errorf("Height: got %d, want 40", got.Height)
	}
}

func TestUpdate_WindowSizeMsg_OverwritesPreviousDims(t *testing.T) {
	m := newModel(nil)
	m, _ = func() (tui.Model, tea.Cmd) {
		next, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		return next.(tui.Model), cmd
	}()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	got := next.(tui.Model)

	if got.Width != 200 {
		t.Errorf("Width: got %d, want 200", got.Width)
	}
	if got.Height != 60 {
		t.Errorf("Height: got %d, want 60", got.Height)
	}
}

// ─── (b) j / k cursor movement (clamped at bounds) ───────────────────────────

func TestUpdate_JKey_IncrementsCursor(t *testing.T) {
	m := newModel(threeWorktrees())
	// Pre-populate snapshot so cursor knows the list length.
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	initial := m.Cursor
	m = sendKey(m, "j")

	if m.Cursor != initial+1 {
		t.Errorf("cursor after j: got %d, want %d", m.Cursor, initial+1)
	}
}

func TestUpdate_KKey_DecrementsCursor(t *testing.T) {
	m := newModel(threeWorktrees())
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)
	// Move to position 1 first so k has room to move back.
	m = sendKey(m, "j")

	m = sendKey(m, "k")

	if m.Cursor != 0 {
		t.Errorf("cursor after j then k: got %d, want 0", m.Cursor)
	}
}

func TestUpdate_KKey_ClampedAtZero(t *testing.T) {
	// Triangulate: pressing k at 0 stays at 0.
	m := newModel(threeWorktrees())
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	if m.Cursor != 0 {
		t.Fatalf("precondition: cursor must start at 0, got %d", m.Cursor)
	}
	m = sendKey(m, "k")

	if m.Cursor != 0 {
		t.Errorf("cursor after k at 0: got %d, want 0", m.Cursor)
	}
}

func TestUpdate_JKey_ClampedAtLastItem(t *testing.T) {
	// Triangulate: pressing j at last item stays at last.
	wts := threeWorktrees()
	last := len(wts) - 1

	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)

	// Move to last item.
	for i := 0; i < last; i++ {
		m = sendKey(m, "j")
	}
	if m.Cursor != last {
		t.Fatalf("precondition: cursor at last: got %d, want %d", m.Cursor, last)
	}

	// One more j — should not exceed last.
	m = sendKey(m, "j")
	if m.Cursor != last {
		t.Errorf("cursor after j at last: got %d, want %d", m.Cursor, last)
	}
}

func TestUpdate_JKey_EmptyList_NoChange(t *testing.T) {
	// Triangulate: j on empty snapshot stays at 0.
	m := newModel(nil)
	snap := service.Snapshot{Worktrees: nil}
	m = m.WithSnapshot(snap)

	m = sendKey(m, "j")

	if m.Cursor != 0 {
		t.Errorf("cursor after j on empty list: got %d, want 0", m.Cursor)
	}
}

// ─── (c) snapshotMsg populates snapshot, clears loading ──────────────────────

func TestUpdate_SnapshotMsg_PopulatesSnapshotAndClearsLoading(t *testing.T) {
	m := newModel(threeWorktrees())

	if !m.Loading {
		t.Fatal("precondition: new model must start in loading state")
	}

	snap := service.Snapshot{Worktrees: threeWorktrees()}
	next, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	got := next.(tui.Model)

	if got.Loading {
		t.Error("Loading should be false after snapshotMsg")
	}
	if len(got.Snap.Worktrees) != 3 {
		t.Errorf("Snap.Worktrees: got %d, want 3", len(got.Snap.Worktrees))
	}
}

func TestUpdate_SnapshotMsg_WithErrors_StillPopulatesWorktrees(t *testing.T) {
	m := newModel(threeWorktrees())

	snap := service.Snapshot{
		Worktrees: threeWorktrees(),
		Errors:    map[string]error{"mergeplan": errors.New("mo not found")},
	}
	next, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	got := next.(tui.Model)

	if got.Loading {
		t.Error("Loading should be false even when snapshot has errors")
	}
	if len(got.Snap.Worktrees) != 3 {
		t.Errorf("Snap.Worktrees: got %d, want 3", len(got.Snap.Worktrees))
	}
	if len(got.Snap.Errors) != 1 {
		t.Errorf("Snap.Errors: got %d, want 1", len(got.Snap.Errors))
	}
}

// ─── quit keys ───────────────────────────────────────────────────────────────

func TestUpdate_QKey_ReturnsQuitCmd(t *testing.T) {
	m := newModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("q should return a quit Cmd, got nil")
	}
}

func TestUpdate_CtrlC_ReturnsQuitCmd(t *testing.T) {
	m := newModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should return a quit Cmd, got nil")
	}
}
