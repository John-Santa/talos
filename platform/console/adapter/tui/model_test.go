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

// ─── (d) Layout toggle via Tab key ───────────────────────────────────────────

func TestUpdate_TabKey_TogglesLayoutToOverview(t *testing.T) {
	// Precondition: default layout is MasterDetail.
	m := newModel(nil)
	if m.Layout != tui.LayoutMasterDetail {
		t.Fatalf("precondition: default Layout must be LayoutMasterDetail, got %v", m.Layout)
	}

	m = sendKeyType(m, tea.KeyTab)

	if m.Layout != tui.LayoutOverview {
		t.Errorf("after Tab: Layout = %v, want LayoutOverview", m.Layout)
	}
}

func TestUpdate_TabKey_CyclesToHybrid(t *testing.T) {
	// Triangulate: a second Tab from Overview advances to Hybrid.
	m := newModel(nil)

	m = sendKeyType(m, tea.KeyTab) // → Overview
	if m.Layout != tui.LayoutOverview {
		t.Fatalf("after first Tab: Layout = %v, want LayoutOverview", m.Layout)
	}

	m = sendKeyType(m, tea.KeyTab) // → Hybrid
	if m.Layout != tui.LayoutHybrid {
		t.Errorf("after second Tab: Layout = %v, want LayoutHybrid", m.Layout)
	}
}

func TestUpdate_TabKey_WrapsBackToMasterDetail(t *testing.T) {
	// Triangulate: a third Tab from Hybrid wraps back to MasterDetail.
	m := newModel(nil)

	m = sendKeyType(m, tea.KeyTab) // → Overview
	m = sendKeyType(m, tea.KeyTab) // → Hybrid
	if m.Layout != tui.LayoutHybrid {
		t.Fatalf("after second Tab: Layout = %v, want LayoutHybrid", m.Layout)
	}

	m = sendKeyType(m, tea.KeyTab) // → MasterDetail (wrap)
	if m.Layout != tui.LayoutMasterDetail {
		t.Errorf("after third Tab: Layout = %v, want LayoutMasterDetail", m.Layout)
	}
}

func TestUpdate_TabKey_CyclesThreeTimes(t *testing.T) {
	// Triangulate full 3-way cycle: MasterDetail → Overview → Hybrid → MasterDetail.
	m := newModel(nil)

	sequence := []tui.LayoutMode{
		tui.LayoutOverview,
		tui.LayoutHybrid,
		tui.LayoutMasterDetail,
	}
	for i, want := range sequence {
		m = sendKeyType(m, tea.KeyTab)
		if m.Layout != want {
			t.Errorf("Tab press %d: Layout = %v, want %v", i+1, m.Layout, want)
		}
	}
}

// ─── (g) tick refresh ────────────────────────────────────────────────────────

func TestUpdate_TickMsg_SetsRefreshingFlag(t *testing.T) {
	// tickMsg should set Refreshing = true and return a non-nil cmd.
	m := newModel(threeWorktrees())
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	next, cmd := m.Update(tui.TickMsg{})
	got := next.(tui.Model)

	if !got.Refreshing {
		t.Error("after tickMsg: Refreshing should be true")
	}
	if cmd == nil {
		t.Error("after tickMsg: cmd should be non-nil (reload + re-arm)")
	}
}

func TestUpdate_TickMsg_RearmsTick(t *testing.T) {
	// Triangulate: a second tickMsg also returns non-nil cmd (tick keeps firing).
	m := newModel(threeWorktrees())
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	// First tick.
	next, cmd := m.Update(tui.TickMsg{})
	if cmd == nil {
		t.Fatal("first tickMsg: cmd should be non-nil")
	}
	m = next.(tui.Model)

	// Second tick.
	_, cmd2 := m.Update(tui.TickMsg{})
	if cmd2 == nil {
		t.Error("second tickMsg: cmd should be non-nil (re-armed)")
	}
}

// ─── (h) mouse: wheel and click ──────────────────────────────────────────────

func TestUpdate_MouseWheelDown_MovesCursorDown(t *testing.T) {
	wts := threeWorktrees()
	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)

	next, _ := m.Update(tea.MouseMsg{
		Type:   tea.MouseWheelDown,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	got := next.(tui.Model)
	if got.Cursor != 1 {
		t.Errorf("wheel down: cursor = %d, want 1", got.Cursor)
	}
}

func TestUpdate_MouseWheelUp_MovesCursorUp(t *testing.T) {
	// Triangulate: start at 1, wheel up → 0.
	wts := threeWorktrees()
	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)
	m = sendKey(m, "j") // cursor → 1

	next, _ := m.Update(tea.MouseMsg{
		Type:   tea.MouseWheelUp,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	got := next.(tui.Model)
	if got.Cursor != 0 {
		t.Errorf("wheel up: cursor = %d, want 0", got.Cursor)
	}
}

func TestUpdate_SnapshotMsg_ClearsRefreshing(t *testing.T) {
	// After a tickMsg sets Refreshing, the subsequent SnapshotMsg clears it.
	m := newModel(threeWorktrees())
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	next, _ := m.Update(tui.TickMsg{})
	m = next.(tui.Model)
	if !m.Refreshing {
		t.Fatal("precondition: Refreshing should be true after tickMsg")
	}

	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	got := next2.(tui.Model)
	if got.Refreshing {
		t.Error("Refreshing should be false after SnapshotMsg")
	}
}

// ─── (e) ? key toggles help short ↔ full ─────────────────────────────────────

func TestUpdate_QuestionMark_TogglesHelpToFull(t *testing.T) {
	m := newModel(nil)
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	// Default: short help (ShowAll == false).
	if m.HelpShowAll {
		t.Fatal("precondition: HelpShowAll must start false")
	}

	m = sendKey(m, "?")

	if !m.HelpShowAll {
		t.Error("after ?: HelpShowAll should be true (full help)")
	}
}

func TestUpdate_QuestionMark_TogglesHelpBackToShort(t *testing.T) {
	// Triangulate: second ? returns to short.
	m := newModel(nil)
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	m = sendKey(m, "?")
	if !m.HelpShowAll {
		t.Fatal("after first ?: HelpShowAll should be true")
	}

	m = sendKey(m, "?")
	if m.HelpShowAll {
		t.Error("after second ?: HelpShowAll should be false (back to short)")
	}
}

func TestUpdate_QuestionMark_TriangulateCycle(t *testing.T) {
	// Triangulate: three ? presses → true, false, true.
	m := newModel(nil)
	snap := service.Snapshot{Worktrees: threeWorktrees()}
	m = m.WithSnapshot(snap)

	expects := []bool{true, false, true}
	for i, want := range expects {
		m = sendKey(m, "?")
		if m.HelpShowAll != want {
			t.Errorf("press %d: HelpShowAll = %v, want %v", i+1, m.HelpShowAll, want)
		}
	}
}

// ─── (f) viewport: j past bottom advances YOffset ────────────────────────────

func manyWorktrees(n int) []platform.Worktree {
	out := make([]platform.Worktree, n)
	for i := range out {
		out[i] = platform.Worktree{
			Figura: "atlas",
			Branch: "agent/atlas/TAL-1",
			Head:   "abc1234",
			Status: "clean",
		}
	}
	return out
}

func TestUpdate_JKey_PastViewportBottom_AdvancesYOffset(t *testing.T) {
	// Use a window small enough that not all rows fit.
	// Height=10 means viewport body ≈ 5-6 lines; 20 rows guarantees overflow.
	wts := manyWorktrees(20)
	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m = next.(tui.Model)

	// Press j enough times to push past the visible window.
	for i := 0; i < 15; i++ {
		m = sendKey(m, "j")
	}

	if m.ViewportYOffset <= 0 {
		t.Errorf("after pressing j past viewport bottom, ViewportYOffset should be > 0, got %d", m.ViewportYOffset)
	}
}

func TestUpdate_KKey_AtTop_YOffsetStaysZero(t *testing.T) {
	// Triangulate: pressing k at top does not go negative.
	wts := manyWorktrees(20)
	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m = next.(tui.Model)

	m = sendKey(m, "k")
	if m.ViewportYOffset < 0 {
		t.Errorf("ViewportYOffset should never go negative, got %d", m.ViewportYOffset)
	}
}

func TestUpdate_JThenK_CursorComesBack_OffsetFollows(t *testing.T) {
	// Triangulate: after scrolling down with j, pressing k brings cursor back;
	// once cursor re-enters the top of the visible window, YOffset decreases.
	wts := manyWorktrees(20)
	m := newModel(wts)
	snap := service.Snapshot{Worktrees: wts}
	m = m.WithSnapshot(snap)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m = next.(tui.Model)

	// Scroll down a lot.
	for i := 0; i < 15; i++ {
		m = sendKey(m, "j")
	}
	highOffset := m.ViewportYOffset

	// Now scroll back fully.
	for i := 0; i < 15; i++ {
		m = sendKey(m, "k")
	}

	if m.ViewportYOffset >= highOffset {
		t.Errorf("after pressing k to return to top, YOffset (%d) should be less than peak (%d)", m.ViewportYOffset, highOffset)
	}
	if m.Cursor != 0 {
		t.Errorf("cursor should be back at 0, got %d", m.Cursor)
	}
}

// ─── RepoLabel field ─────────────────────────────────────────────────────────

func TestNew_RepoLabel_DefaultEmpty(t *testing.T) {
	m := newModel(nil)
	if m.RepoLabel != "" {
		t.Errorf("RepoLabel: got %q, want empty string by default", m.RepoLabel)
	}
}

func TestWithRepoLabel_SetsField(t *testing.T) {
	m := newModel(nil)
	m2 := m.WithRepoLabel("John-Santa/talos")
	if m2.RepoLabel != "John-Santa/talos" {
		t.Errorf("RepoLabel: got %q, want %q", m2.RepoLabel, "John-Santa/talos")
	}
}

func TestWithRepoLabel_DoesNotMutateOriginal(t *testing.T) {
	m := newModel(nil)
	_ = m.WithRepoLabel("John-Santa/talos")
	if m.RepoLabel != "" {
		t.Errorf("original RepoLabel should still be empty, got %q", m.RepoLabel)
	}
}

func TestWithRepoLabel_EmptyValue(t *testing.T) {
	m := newModel(nil)
	m2 := m.WithRepoLabel("")
	if m2.RepoLabel != "" {
		t.Errorf("RepoLabel with empty arg: got %q, want %q", m2.RepoLabel, "")
	}
}
