package tui

import (
	"strings"

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
