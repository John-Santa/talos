package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/mock"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// populatedModelForCreate returns a ready Model + actor for create-modal tests.
func populatedModelForCreate() (tui.Model, *mock.PlatformActorMock) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)
	return m, actor
}

// ─── (A) n key opens the create modal ────────────────────────────────────────

func TestUpdate_NKey_OpensCreateModal(t *testing.T) {
	m, _ := populatedModelForCreate()

	if m.ModalActive {
		t.Fatal("precondition: ModalActive must start false")
	}

	m = sendKey(m, "n")

	if !m.ModalActive {
		t.Error("after n: ModalActive should be true")
	}
	if m.ModalMode != tui.ModalModeCreate {
		t.Errorf("after n: ModalMode = %v, want ModalModeCreate", m.ModalMode)
	}
}

func TestUpdate_NKey_FiguraInputIsFocused(t *testing.T) {
	// Triangulate: after n, the figura input is focused.
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	if !m.CreateFiguraInput.Focused() {
		t.Error("after n: figura input must be focused")
	}
}

// ─── (B) typing updates the focused input ────────────────────────────────────

func TestUpdate_CreateModal_TypingUpdatesFiguraInput(t *testing.T) {
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "atlas" {
		m = sendKey(m, string(ch))
	}

	if m.CreateFiguraInput.Value() != "atlas" {
		t.Errorf("figura input value = %q, want %q", m.CreateFiguraInput.Value(), "atlas")
	}
}

func TestUpdate_CreateModal_TabAdvancesToJiraKeyInput(t *testing.T) {
	// Triangulate: Tab moves focus from figura → jiraKey.
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "iris" {
		m = sendKey(m, string(ch))
	}

	m = sendKeyType(m, tea.KeyTab)

	if m.CreateFiguraInput.Focused() {
		t.Error("after Tab: figura input should no longer be focused")
	}
	if !m.CreateJiraKeyInput.Focused() {
		t.Error("after Tab: jiraKey input must be focused")
	}
}

// ─── (C) invalid input blocks confirm ────────────────────────────────────────

func TestUpdate_CreateModal_InvalidFigura_EnterDoesNotFireAction(t *testing.T) {
	// figura must be lowercase letters only; uppercase is invalid.
	m, actor := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "IRIS" {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if cmd != nil {
		t.Error("enter with invalid figura: cmd should be nil (action blocked)")
	}
	if !got.ModalActive {
		t.Error("enter with invalid figura: modal should remain open")
	}
	actor.AssertNotCalled(t, "CreateWorktree")
}

func TestUpdate_CreateModal_InvalidJiraKey_EnterDoesNotFireAction(t *testing.T) {
	// Triangulate: valid figura + invalid jiraKey also blocks confirm.
	m, actor := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "iris" {
		m = sendKey(m, string(ch))
	}
	m = sendKeyType(m, tea.KeyTab)

	for _, ch := range "notakey" {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if cmd != nil {
		t.Error("enter with invalid jiraKey: cmd should be nil (action blocked)")
	}
	if !got.ModalActive {
		t.Error("enter with invalid jiraKey: modal should remain open")
	}
	actor.AssertNotCalled(t, "CreateWorktree")
}

// ─── (D) valid input + confirm fires action ───────────────────────────────────

func TestUpdate_CreateModal_ValidInput_EnterFiresActionAndClosesModal(t *testing.T) {
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "iris" {
		m = sendKey(m, string(ch))
	}
	m = sendKeyType(m, tea.KeyTab)

	for _, ch := range "TAL-19" {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after valid confirm: ModalActive should be false")
	}
	if cmd == nil {
		t.Error("after valid confirm: cmd should be non-nil (async create in flight)")
	}
}

func TestUpdate_CreateModal_ValidInput_ConfirmOnJiraKeyField(t *testing.T) {
	// Triangulate: Enter while jiraKey is focused with valid data also confirms.
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	for _, ch := range "hermes" {
		m = sendKey(m, string(ch))
	}
	m = sendKeyType(m, tea.KeyTab)

	for _, ch := range "TAL-20" {
		m = sendKey(m, string(ch))
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("enter on jiraKey field with valid input: modal should close")
	}
	if cmd == nil {
		t.Error("enter on jiraKey field with valid input: cmd must be non-nil")
	}
}

// ─── (E) esc cancels with no cmd ─────────────────────────────────────────────

func TestUpdate_CreateModal_Esc_CancelsWithNoCmd(t *testing.T) {
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(tui.Model)

	if got.ModalActive {
		t.Error("after esc: ModalActive should be false")
	}
	if cmd != nil {
		t.Error("after esc: cmd should be nil (cancel has no side-effect)")
	}
}

// ─── (F) nav keys ignored (routed to input) while create modal open ───────────

func TestUpdate_CreateModal_JKey_DoesNotMoveCursor(t *testing.T) {
	m, _ := populatedModelForCreate()
	m = sendKey(m, "n")

	cursorBefore := m.Cursor
	// 'j' routes to the textinput, not the list cursor.
	m = sendKey(m, "j")

	if m.Cursor != cursorBefore {
		t.Errorf("cursor moved while create modal open: got %d, want %d", m.Cursor, cursorBefore)
	}
	if !m.ModalActive {
		t.Error("modal should remain open")
	}
}

// ─── (G) golden file for the create modal view ───────────────────────────────

func TestView_CreateModal_GoldenFile(t *testing.T) {
	actor := mock.NewPlatformActorMock()
	m := populatedModel(actor)

	m = sendKey(m, "n") // open create modal

	// Pre-fill figura, then tab to jiraKey, pre-fill that too.
	for _, ch := range "iris" {
		m = sendKey(m, string(ch))
	}
	m = sendKeyType(m, tea.KeyTab)
	for _, ch := range "TAL-19" {
		m = sendKey(m, string(ch))
	}

	got := m.View()
	goldenPath := filepath.Join("testdata", "create-modal.golden")

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
		t.Errorf("View() create-modal output does not match golden file %s\n\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, string(want))
	}
}
