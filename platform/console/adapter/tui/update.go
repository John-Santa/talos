package tui

import (
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
		return m, nil

	// ─── Keyboard ─────────────────────────────────────────────────────────────
	case tea.KeyMsg:
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
		}
		return m, nil

	// ─── Async data loaded ────────────────────────────────────────────────────
	case SnapshotMsg:
		m.Snap = msg.Snap
		m.Loading = false
		return m, nil
	}

	return m, nil
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
	return m
}

func (m Model) moveCursorUp() Model {
	if m.Cursor > 0 {
		m.Cursor--
	}
	return m
}
