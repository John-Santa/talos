package tui_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/mock"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Toast auto-dismiss ───────────────────────────────────────────────────────

// TestUpdate_ActionResultMsg_ReturnsNonNilCmd verifies that setting a toast via
// ActionResultMsg returns a non-nil cmd (the auto-dismiss timer).
func TestUpdate_ActionResultMsg_ReturnsNonNilCmd(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	_, cmd := m.Update(tui.ActionResultMsg{Err: nil})
	if cmd == nil {
		t.Error("ActionResultMsg should return non-nil cmd (dismiss timer + snapshot reload)")
	}
}

// TestUpdate_ActionResultMsg_BumpsToastSeq verifies that each ActionResultMsg
// increments the toast sequence counter so stale dismiss timers are ignored.
func TestUpdate_ActionResultMsg_BumpsToastSeq(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	seqBefore := m.ToastSeq

	next, _ := m.Update(tui.ActionResultMsg{Err: nil})
	got := next.(tui.Model)

	if got.ToastSeq <= seqBefore {
		t.Errorf("ToastSeq should increase after ActionResultMsg; before=%d after=%d",
			seqBefore, got.ToastSeq)
	}
}

// TestUpdate_ToastDismissMsg_Current_ClearsToast verifies that a
// toastDismissMsg carrying the current seq clears the toast.
func TestUpdate_ToastDismissMsg_Current_ClearsToast(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// Set a toast.
	next, _ := m.Update(tui.ActionResultMsg{Err: nil})
	m = next.(tui.Model)

	if m.Toast == "" {
		t.Fatal("precondition: Toast must be non-empty after ActionResultMsg")
	}

	// Dismiss with the CURRENT seq.
	next2, _ := m.Update(tui.ToastDismissMsg{Seq: m.ToastSeq})
	got := next2.(tui.Model)

	if got.Toast != "" {
		t.Errorf("Toast should be cleared by matching toastDismissMsg; got %q", got.Toast)
	}
}

// TestUpdate_ToastDismissMsg_Stale_DoesNotClearToast verifies that a
// toastDismissMsg carrying a stale seq does NOT clear the toast (the newer
// toast's timer should keep it visible).
func TestUpdate_ToastDismissMsg_Stale_DoesNotClearToast(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// First toast.
	next, _ := m.Update(tui.ActionResultMsg{Err: nil})
	m = next.(tui.Model)

	firstSeq := m.ToastSeq

	// Second toast overwrites the first (new action result).
	next2, _ := m.Update(tui.ActionResultMsg{Err: nil})
	m = next2.(tui.Model)

	secondSeq := m.ToastSeq
	if secondSeq == firstSeq {
		t.Fatal("precondition: second ActionResultMsg must bump seq")
	}

	// A dismiss carrying the FIRST (stale) seq should NOT clear.
	next3, _ := m.Update(tui.ToastDismissMsg{Seq: firstSeq})
	got := next3.(tui.Model)

	if got.Toast == "" {
		t.Error("stale toastDismissMsg should NOT clear the toast of a newer action")
	}
}

// TestUpdate_ToastDismissMsg_Triangulate_CurrentSeqAlwaysClears triangulates
// that the current seq always clears regardless of prior stale dismisses.
func TestUpdate_ToastDismissMsg_Triangulate_CurrentSeqAlwaysClears(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// Set toast.
	next, _ := m.Update(tui.ActionResultMsg{Err: nil})
	m = next.(tui.Model)
	seq := m.ToastSeq

	// A stale dismiss (seq-1 is definitely stale because seq starts at 0).
	if seq > 0 {
		next2, _ := m.Update(tui.ToastDismissMsg{Seq: seq - 1})
		m = next2.(tui.Model)
		if m.Toast == "" {
			t.Fatal("stale dismiss should not have cleared the toast")
		}
	}

	// The current seq dismiss clears it.
	next3, _ := m.Update(tui.ToastDismissMsg{Seq: m.ToastSeq})
	got := next3.(tui.Model)

	if got.Toast != "" {
		t.Errorf("current seq dismiss should clear toast; got %q", got.Toast)
	}
}

// ─── Action key bindings in the help map ─────────────────────────────────────

// TestKeyMap_ShortHelp_ContainsActionBindings verifies that the short help bar
// includes at least one of the new action keys (n/x/m).
func TestKeyMap_ShortHelp_ContainsActionBindings(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// Trigger a WindowSizeMsg so the help model gets a width.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)

	shortHelp := m.ShortHelpBindings()

	found := false
	for _, b := range shortHelp {
		h := b.Help()
		if h.Key == "n" || h.Key == "x" || h.Key == "m" ||
			h.Desc == "new" || h.Desc == "teardown" || h.Desc == "merge" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ShortHelp bindings should include at least one action key (n/x/m)")
	}
}

// TestKeyMap_FullHelp_ContainsAllActionBindings verifies that the full help
// includes bindings for all three action keys.
func TestKeyMap_FullHelp_ContainsAllActionBindings(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	fullHelp := m.FullHelpBindings()

	allKeys := make(map[string]bool)
	for _, col := range fullHelp {
		for _, b := range col {
			h := b.Help()
			allKeys[h.Key] = true
			allKeys[h.Desc] = true
		}
	}

	checks := []struct {
		key  string
		desc string
	}{
		{"n", "new"},
		{"x", "teardown"},
		{"m", "merge"},
	}

	for _, c := range checks {
		if !allKeys[c.key] && !allKeys[c.desc] {
			t.Errorf("FullHelp should contain action binding key=%q desc=%q", c.key, c.desc)
		}
	}
}
