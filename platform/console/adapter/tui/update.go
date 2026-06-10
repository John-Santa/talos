package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all incoming messages. It follows the Bubbletea convention:
// return (tea.Model, tea.Cmd) where tea.Model is always a tui.Model value.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ─── Window resize ────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.vp = viewport.New(m.listViewportWidth(), m.listViewportHeight())
		m.ViewportYOffset = 0
		return m, nil

	// ─── Keyboard ─────────────────────────────────────────────────────────────
	case tea.KeyMsg:
		// When the confirmation modal is active, route all keys to it first.
		if m.ModalActive {
			return m.updateModal(msg)
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyTab:
			m.Layout = cycleLayout(m.Layout)
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j":
			m = m.moveCursorDown()
		case "k":
			m = m.moveCursorUp()
		case "?":
			m.HelpShowAll = !m.HelpShowAll
			m.helpModel.ShowAll = m.HelpShowAll
		case "x":
			m = m.openTeardownModal()
		case "n":
			m, cmd := m.openCreateModal()
			return m, cmd
		case "m":
			m, cmd := m.openExecuteModal()
			return m, cmd
		}
		return m, nil

	// ─── Action result (teardown etc.) ───────────────────────────────────────
	case ActionResultMsg:
		if msg.Err != nil {
			m.Toast = fmt.Sprintf("✗ %s", msg.Err.Error())
		} else {
			m.Toast = "✓ teardown complete"
		}
		// Bump the sequence so any previously-armed dismiss timer is invalidated.
		m.ToastSeq++
		seq := m.ToastSeq
		// Schedule auto-dismiss and trigger a snapshot reload.
		dismissCmd := tea.Tick(toastDuration, func(time.Time) tea.Msg {
			return ToastDismissMsg{Seq: seq}
		})
		return m, tea.Batch(loadSnapshot(m.agg), dismissCmd)

	// ─── Toast auto-dismiss ───────────────────────────────────────────────────
	case ToastDismissMsg:
		// Only clear the toast when the seq matches — stale timers are no-ops.
		if msg.Seq == m.ToastSeq {
			m.Toast = ""
		}
		return m, nil

	// ─── Async data loaded ────────────────────────────────────────────────────
	case SnapshotMsg:
		m.Snap = msg.Snap
		m.Loading = false
		m.Refreshing = false
		return m, nil

	// ─── Live refresh tick ────────────────────────────────────────────────────
	case TickMsg:
		m.Refreshing = true
		return m, tea.Batch(loadSnapshot(m.agg), tickCmd(refreshInterval))

	// ─── Mouse ────────────────────────────────────────────────────────────────
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			m = m.moveCursorDown()
		case tea.MouseButtonWheelUp:
			m = m.moveCursorUp()
		}
		return m, nil
	}

	return m, nil
}

// ─── Modal routing ────────────────────────────────────────────────────────────

// updateModal dispatches key events to the right modal handler based on ModalMode.
func (m Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.ModalMode {
	case ModalModeCreate:
		return m.updateCreateModal(msg)
	case ModalModeExecute:
		return m.updateExecuteModal(msg)
	default:
		return m.updateConfirmModal(msg)
	}
}

// updateConfirmModal handles key events for the yes/no confirmation dialog.
// y/enter = confirm; n/esc = cancel; all other keys are swallowed.
func (m Model) updateConfirmModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.confirmModal()
	case tea.KeyEsc:
		m.ModalActive = false
		return m, nil
	}
	switch msg.String() {
	case "y":
		return m.confirmModal()
	case "n":
		m.ModalActive = false
		return m, nil
	}
	// All other keys are swallowed while modal is open.
	return m, nil
}

// confirmModal closes the modal and dispatches the async teardown command.
func (m Model) confirmModal() (tea.Model, tea.Cmd) {
	m.ModalActive = false
	figura := m.modalFigura
	jiraKey := m.modalJiraKey
	actor := m.actor
	cmd := func() tea.Msg {
		err := actor.TeardownWorktree(context.Background(), figura, jiraKey)
		return ActionResultMsg{Err: err}
	}
	return m, cmd
}

// ─── Create-modal routing ─────────────────────────────────────────────────────

// reValidFigura matches a figura: one or more lowercase ASCII letters only.
var reValidFigura = regexp.MustCompile(`^[a-z]+$`)

// reValidJiraKey matches a Jira key in the TAL-NNN format.
var reValidJiraKey = regexp.MustCompile(`^TAL-[0-9]+$`)

// isCreateInputValid returns true when both inputs pass their validation rules.
func (m Model) isCreateInputValid() bool {
	return reValidFigura.MatchString(m.CreateFiguraInput.Value()) &&
		reValidJiraKey.MatchString(m.CreateJiraKeyInput.Value())
}

// updateCreateModal handles key events while the create-worktree input modal is open.
// Tab advances focus between the two inputs.
// Enter attempts to confirm (blocked when input is invalid).
// Esc cancels.
// All other keys are forwarded to the focused input.
func (m Model) updateCreateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.ModalActive = false
		m.CreateFiguraInput.Blur()
		m.CreateJiraKeyInput.Blur()
		return m, nil

	case tea.KeyTab:
		// Cycle focus: figura → jiraKey → figura.
		if m.CreateFiguraInput.Focused() {
			m.CreateFiguraInput.Blur()
			cmd := m.CreateJiraKeyInput.Focus()
			return m, cmd
		}
		m.CreateJiraKeyInput.Blur()
		cmd := m.CreateFiguraInput.Focus()
		return m, cmd

	case tea.KeyEnter:
		if !m.isCreateInputValid() {
			// Block confirm — invalid input; keep modal open, no cmd.
			return m, nil
		}
		return m.confirmCreateModal()
	}

	// Forward all other keys to the focused input.
	var cmd tea.Cmd
	if m.CreateFiguraInput.Focused() {
		m.CreateFiguraInput, cmd = m.CreateFiguraInput.Update(msg)
	} else {
		m.CreateJiraKeyInput, cmd = m.CreateJiraKeyInput.Update(msg)
	}
	return m, cmd
}

// confirmCreateModal closes the create modal and dispatches the async create command.
func (m Model) confirmCreateModal() (tea.Model, tea.Cmd) {
	m.ModalActive = false
	m.CreateFiguraInput.Blur()
	m.CreateJiraKeyInput.Blur()
	figura := m.CreateFiguraInput.Value()
	jiraKey := m.CreateJiraKeyInput.Value()
	actor := m.actor
	cmd := func() tea.Msg {
		err := actor.CreateWorktree(context.Background(), figura, jiraKey)
		return ActionResultMsg{Err: err}
	}
	return m, cmd
}

// openCreateModal resets the create inputs, focuses the figura field, and
// sets ModalMode to ModalModeCreate. It is a no-op when actor is nil.
func (m Model) openCreateModal() (Model, tea.Cmd) {
	if m.actor == nil {
		return m, nil
	}
	m.CreateFiguraInput.SetValue("")
	m.CreateJiraKeyInput.SetValue("")
	m.CreateJiraKeyInput.Blur()
	m.ModalActive = true
	m.ModalMode = ModalModeCreate
	m.ModalTitle = "New worktree"
	cmd := m.CreateFiguraInput.Focus()
	return m, cmd
}

// ─── Modal open helpers ───────────────────────────────────────────────────────

// ─── Execute-merge modal ──────────────────────────────────────────────────────

// updateExecuteModal handles key events while the execute-merge type-to-confirm
// modal is open.
// Enter attempts confirm — blocked when typed token != BaseBranch.
// Esc cancels.
// All other keys are forwarded to ExecuteTokenInput.
func (m Model) updateExecuteModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.ModalActive = false
		m.ExecuteTokenInput.Blur()
		return m, nil

	case tea.KeyEnter:
		if m.ExecuteTokenInput.Value() != m.modalBaseBranch {
			// Wrong token — keep modal open, no cmd.
			return m, nil
		}
		return m.confirmExecuteModal()
	}

	// Forward all other keys to the token input.
	var cmd tea.Cmd
	m.ExecuteTokenInput, cmd = m.ExecuteTokenInput.Update(msg)
	return m, cmd
}

// confirmExecuteModal closes the execute modal and dispatches the async merge command.
func (m Model) confirmExecuteModal() (tea.Model, tea.Cmd) {
	m.ModalActive = false
	m.ExecuteTokenInput.Blur()
	actor := m.actor
	cmd := func() tea.Msg {
		err := actor.ExecuteMerge(context.Background())
		return ActionResultMsg{Err: err}
	}
	return m, cmd
}

// openExecuteModal resets the token input, focuses it, and opens the execute modal.
// It is a no-op when actor is nil or when there is no merge plan.
func (m Model) openExecuteModal() (Model, tea.Cmd) {
	if m.actor == nil {
		return m, nil
	}
	if m.Snap.MergePlan.BaseBranch == "" && len(m.Snap.MergePlan.Steps) == 0 {
		return m, nil
	}
	m.modalBaseBranch = m.Snap.MergePlan.BaseBranch
	m.ExecuteTokenInput.SetValue("")
	m.ModalActive = true
	m.ModalMode = ModalModeExecute
	m.ModalTitle = "Execute merge"
	cmd := m.ExecuteTokenInput.Focus()
	return m, cmd
}

// ─── Modal open helpers ───────────────────────────────────────────────────────

// openTeardownModal opens the confirmation dialog for the currently-selected
// worktree. If the list is empty or the actor is nil, it is a no-op.
func (m Model) openTeardownModal() Model {
	if m.actor == nil {
		return m
	}
	wts := m.Snap.Worktrees
	if len(wts) == 0 || m.Cursor >= len(wts) {
		return m
	}
	wt := wts[m.Cursor]
	figura, jiraKey := parseAgentBranch(wt.Branch)
	if figura == "" {
		figura = wt.Figura
	}
	m.ModalActive = true
	m.ModalMode = ModalModeConfirm
	m.ModalTitle = "Confirm teardown"
	m.ModalMessage = fmt.Sprintf("Tear down worktree for %s (%s)?", figura, jiraKey)
	m.modalFigura = figura
	m.modalJiraKey = jiraKey
	return m
}

// parseAgentBranch extracts (figura, jiraKey) from a branch name matching
// the convention agent/<figura>/<JIRA-KEY>. Returns ("", "") if it does not match.
func parseAgentBranch(branch string) (figura, jiraKey string) {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) == 3 && parts[0] == "agent" {
		return parts[1], parts[2]
	}
	return "", ""
}

// ─── Cursor helpers ───────────────────────────────────────────────────────────

func (m Model) moveCursorDown() Model {
	last := len(m.Snap.Worktrees) - 1
	if last < 0 {
		return m // empty list — nothing to move into
	}
	if m.Cursor < last {
		m.Cursor++
	}
	m = m.syncViewportOffset()
	return m
}

func (m Model) moveCursorUp() Model {
	if m.Cursor > 0 {
		m.Cursor--
	}
	m = m.syncViewportOffset()
	return m
}

// syncViewportOffset adjusts ViewportYOffset so the cursor row stays visible
// inside the viewport. Each row is 1 line tall.
//
//   - If cursor scrolled below the bottom visible row  → advance offset.
//   - If cursor scrolled above the top visible row     → retreat offset.
//   - Offset is always clamped to [0, max].
func (m Model) syncViewportOffset() Model {
	h := m.listViewportHeight()
	if h <= 0 {
		return m
	}

	// Bottom of the visible window (exclusive).
	bottom := m.ViewportYOffset + h

	if m.Cursor >= bottom {
		m.ViewportYOffset = m.Cursor - h + 1
	} else if m.Cursor < m.ViewportYOffset {
		m.ViewportYOffset = m.Cursor
	}

	// Clamp.
	if m.ViewportYOffset < 0 {
		m.ViewportYOffset = 0
	}

	return m
}

// ─── Viewport size helpers ────────────────────────────────────────────────────

// listViewportHeight is the number of visible rows in the master-detail list.
// We reserve: 1 header + 1 border-top + 1 border-bottom + 1 newline after
// list + 1 degraded note (worst case) + 2 help lines (short or full).
const listHeaderLines = 4 // header(1) + sep(1) + border-top(1) + newline(1)
const listFooterLines = 3 // border-bottom(1) + newline(1) + help(1)

func (m Model) listViewportHeight() int {
	if m.Height <= listHeaderLines+listFooterLines {
		return 4 // safe fallback for very small windows / zero-height in tests
	}
	return m.Height - listHeaderLines - listFooterLines
}

func (m Model) listViewportWidth() int {
	if m.Width > 0 {
		return m.Width - 4
	}
	return 96
}

// ─── Viewport content helper ──────────────────────────────────────────────────

// visibleRows returns the slice of content lines that should be rendered inside
// the viewport based on the current ViewportYOffset and viewport height.
func (m Model) visibleRows(lines []string) []string {
	h := m.listViewportHeight()
	start := m.ViewportYOffset
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		return nil
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
	return lines[start:end]
}

// renderWorktreeViewport renders all rows, slices out the visible window, and
// pads the result to fill the viewport height (so the border stays stable).
func (m Model) renderWorktreeViewport(rows []string) string {
	visible := m.visibleRows(rows)
	h := m.listViewportHeight()
	// Pad with empty lines to fill the viewport.
	for len(visible) < h {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}
