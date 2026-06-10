package tui

import (
	"fmt"
	"strings"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/charmbracelet/lipgloss"
)

// View returns the full terminal render for the current model state.
func (m Model) View() string {
	if m.Loading {
		return m.Theme.Subtle.Render("  loading…")
	}
	return m.viewWorktreeList()
}

// ─── Worktree list ────────────────────────────────────────────────────────────

func (m Model) viewWorktreeList() string {
	var b strings.Builder

	// Header.
	title := m.Theme.Header.Width(m.effectiveWidth()).Render("  TALOS — Worktree Monitor")
	b.WriteString(title)
	b.WriteString("\n")

	// Body: the bordered list.
	b.WriteString(m.renderWorktreeRows())
	b.WriteString("\n")

	// Degraded note — shown when at least one source failed.
	if len(m.Snap.Errors) > 0 {
		note := m.Theme.Subtle.Render(fmt.Sprintf("  ⚠  %d source(s) degraded — partial data", len(m.Snap.Errors)))
		b.WriteString(note)
		b.WriteString("\n")
	}

	// Footer hint.
	b.WriteString(m.Theme.Footer.Render("  j/k move · q quit"))

	return b.String()
}

// renderWorktreeRows builds the bordered list of worktree rows.
func (m Model) renderWorktreeRows() string {
	wts := m.Snap.Worktrees
	if len(wts) == 0 {
		return m.Theme.Subtle.Render("  (no worktrees found)")
	}

	var rows []string
	for i, wt := range wts {
		rows = append(rows, m.renderRow(i, wt))
	}

	inner := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorOverlay).
		Width(m.effectiveWidth()).
		Padding(0, 1).
		Render(inner)
}

// renderRow renders a single worktree as "figura · branch · head · status".
func (m Model) renderRow(i int, wt platform.Worktree) string {
	figura := m.Theme.Badge.Render(fmt.Sprintf("%-8s", wt.Figura))
	branch := wt.Branch
	head := m.Theme.Subtle.Render(shortHead(wt.Head))
	status := m.renderStatus(wt.Status)

	line := fmt.Sprintf("%s  %-40s  %s  %s", figura, branch, head, status)

	if i == m.Cursor {
		return m.Theme.ListSelected.Render("▶ " + line)
	}
	return m.Theme.ListItem.Render(line)
}

// renderStatus colours the status string: "clean" = green, anything else = peach.
func (m Model) renderStatus(s string) string {
	if s == "clean" {
		return m.Theme.StatusOK.Render(s)
	}
	return m.Theme.StatusDirty.Render(s)
}

// shortHead truncates a git SHA to 7 chars for display.
func shortHead(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// effectiveWidth returns the usable width, falling back to 100 if not yet set.
func (m Model) effectiveWidth() int {
	if m.Width > 0 {
		return m.Width - 4 // subtract outer padding
	}
	return 96 // fallback before first WindowSizeMsg
}
