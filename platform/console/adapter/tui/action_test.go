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

// ─── helpers ──────────────────────────────────────────────────────────────────

// newModelWithActor constructs a Model wired to a PlatformActorMock.
func newModelWithActor(wts []platform.Worktree, actor *mock.PlatformActorMock) tui.Model {
	r := mock.NewPlatformReaderMock()
	r.WorktreesResult = wts
	agg := service.NewAggregator(r)
	return tui.NewWithActor(agg, actor)
}

// populatedModel returns a Model with 3 worktrees + window size injected.
func populatedModel(actor *mock.PlatformActorMock) tui.Model {
	wts := threeWorktrees()
	m := newModelWithActor(wts, actor)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = next.(tui.Model)
	snap := service.Snapshot{Worktrees: wts}
	next2, _ := m.Update(tui.SnapshotMsg{Snap: snap})
	return next2.(tui.Model)
}

// ─── (i) x key opens modal ────────────────────────────────────────────────────

func TestUpdate_XKey_OpensModal(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// Precondition: modal not active.
	if m.ModalActive {
		t.Fatal("precondition: ModalActive must start false")
	}

	m = sendKey(m, "x")

	if !m.ModalActive {
		t.Error("after x: ModalActive should be true")
	}
}

func TestUpdate_XKey_ModalMessageReferencesSelectedWorktree(t *testing.T) {
	// Triangulate: modal message names the currently selected worktree.
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	// Cursor is at 0 → atlas / TAL-1.
	m = sendKey(m, "x")

	if m.ModalMessage == "" {
		t.Error("after x: ModalMessage must not be empty")
	}
	// The message should reference something identifiable about the worktree.
	// We check for the figura or jira key fragment.
	hasAtlas := contains(m.ModalMessage, "atlas")
	hasKey := contains(m.ModalMessage, "TAL-1")
	if !hasAtlas && !hasKey {
		t.Errorf("ModalMessage should reference selected worktree (atlas/TAL-1); got: %q", m.ModalMessage)
	}
}

// ─── (ii) confirm (enter / y) closes modal, returns non-nil cmd ───────────────

func TestUpdate_ModalConfirm_Enter_CloseModalAndReturnsCmd(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	m = sendKey(m, "x") // open modal

	if !m.ModalActive {
		t.Fatal("precondition: modal must be open")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after enter: ModalActive should be false (modal closed)")
	}
	if cmd == nil {
		t.Error("after enter: cmd should be non-nil (async teardown in flight)")
	}
}

func TestUpdate_ModalConfirm_Y_CloseModalAndReturnsCmd(t *testing.T) {
	// Triangulate: y key also confirms.
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	m = sendKey(m, "x") // open modal

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after y: ModalActive should be false")
	}
	if cmd == nil {
		t.Error("after y: cmd should be non-nil")
	}
}

// ─── (iii) cancel (esc / n) closes modal, returns nil cmd ────────────────────

func TestUpdate_ModalCancel_Esc_CloseModalNoCmd(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	m = sendKey(m, "x") // open modal

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after esc: ModalActive should be false (modal closed)")
	}
	if cmd != nil {
		t.Error("after esc: cmd should be nil (cancel has no side-effect)")
	}
}

func TestUpdate_ModalCancel_N_CloseModalNoCmd(t *testing.T) {
	// Triangulate: n key also cancels.
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	m = sendKey(m, "x") // open modal

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after n: ModalActive should be false")
	}
	if cmd != nil {
		t.Error("after n: cmd should be nil")
	}
}

// ─── (iv) confirm vs cancel: navigation keys ignored when modal open ──────────

func TestUpdate_Modal_JKey_Ignored(t *testing.T) {
	// While modal is active, j should not move cursor.
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	m = sendKey(m, "x") // open modal

	cursorBefore := m.Cursor
	m = sendKey(m, "j")

	if m.Cursor != cursorBefore {
		t.Errorf("cursor moved while modal open: got %d, want %d", m.Cursor, cursorBefore)
	}
	if !m.ModalActive {
		t.Error("modal should still be active after ignored j")
	}
}

// ─── (v) actionResultMsg sets toast ──────────────────────────────────────────

func TestUpdate_ActionResultMsg_Success_SetsToast(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	next, _ := m.Update(tui.ActionResultMsg{Err: nil})
	got := next.(tui.Model)

	if got.Toast == "" {
		t.Error("after ActionResultMsg(nil): Toast should be non-empty (success message)")
	}
}

func TestUpdate_ActionResultMsg_Error_SetsToastWithError(t *testing.T) {
	// Triangulate: failed action shows an error toast.
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	someErr := errors.New("teardown: wt exited 1")
	next, _ := m.Update(tui.ActionResultMsg{Err: someErr})
	got := next.(tui.Model)

	if got.Toast == "" {
		t.Error("after ActionResultMsg(err): Toast should be non-empty (error message)")
	}
	if !contains(got.Toast, someErr.Error()) {
		t.Errorf("Toast should contain the error text %q; got %q", someErr.Error(), got.Toast)
	}
}

func TestUpdate_ActionResultMsg_TriggersSnapshotRefresh(t *testing.T) {
	// Triangulate: ActionResultMsg triggers a reload (non-nil cmd).
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	_, cmd := m.Update(tui.ActionResultMsg{Err: nil})
	if cmd == nil {
		t.Error("after ActionResultMsg: cmd should be non-nil (triggers snapshot reload)")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
