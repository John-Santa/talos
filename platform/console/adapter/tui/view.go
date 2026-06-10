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

	var base string
	switch m.Layout {
	case LayoutOverview:
		base = m.viewOverview()
	case LayoutHybrid:
		base = m.viewHybrid()
	default:
		base = m.viewWorktreeList()
	}

	// Overlay the active modal.
	if m.ModalActive {
		if m.ModalMode == ModalModeCreate {
			return m.overlayCreateModal(base)
		}
		return m.overlayModal(base)
	}
	return base
}

// ─── Worktree list ────────────────────────────────────────────────────────────

func (m Model) viewWorktreeList() string {
	var b strings.Builder

	// Header.
	title := m.Theme.Header.Width(m.effectiveWidth()).Render("  TALOS — Worktree Monitor")
	b.WriteString(title)
	b.WriteString("\n")

	// Body: the bordered list (viewport-sliced).
	b.WriteString(m.renderWorktreeRows())
	b.WriteString("\n")

	// Degraded note — shown when at least one source failed.
	if len(m.Snap.Errors) > 0 {
		note := m.Theme.Subtle.Render(fmt.Sprintf("  ⚠  %d source(s) degraded — partial data", len(m.Snap.Errors)))
		b.WriteString(note)
		b.WriteString("\n")
	}

	// Toast — transient success/error notification after an action.
	if m.Toast != "" {
		b.WriteString(m.renderToast())
		b.WriteString("\n")
	}

	// Dynamic help bar (replaces static footer string).
	b.WriteString(m.renderHelpBar())

	return b.String()
}

// renderHelpBar renders the bubbles/help bar, styled with the footer colour.
func (m Model) renderHelpBar() string {
	rendered := m.helpModel.View(m.keys)
	return m.Theme.Footer.Render(rendered)
}

// renderWorktreeRows builds the bordered, viewport-windowed list of worktree rows.
func (m Model) renderWorktreeRows() string {
	wts := m.Snap.Worktrees
	if len(wts) == 0 {
		return m.Theme.Subtle.Render("  (no worktrees found)")
	}

	// Build all row strings first.
	var allRows []string
	for i, wt := range wts {
		allRows = append(allRows, m.renderRow(i, wt))
	}

	// Slice to visible window.
	inner := m.renderWorktreeViewport(allRows)

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

// ─── Overview layout ──────────────────────────────────────────────────────────

// viewOverview renders three side-by-side bordered panels:
//
//	[ Worktrees ] [ Merge Order ] [ Overlap ]
//
// followed by the shared dynamic help bar. Panel widths are derived from
// Model.Width so the layout stays responsive. Each panel degrades gracefully
// when its data source has an error in Snap.Errors.
func (m Model) viewOverview() string {
	var b strings.Builder

	// Header — same style as MasterDetail.
	title := m.Theme.Header.Width(m.effectiveWidth()).Render("  TALOS — Overview")
	b.WriteString(title)
	b.WriteString("\n")

	// Divide available width across three panels.
	// Allocate equally; remainder goes to the first panel.
	total := m.effectiveWidth()
	colW := total / 3
	col1W := total - colW*2 // absorbs remainder

	panelHeight := m.overviewPanelHeight()

	left := m.renderWorktreesPanel(col1W, panelHeight)
	mid := m.renderMergePlanPanel(colW, panelHeight)
	right := m.renderOverlapPanel(colW, panelHeight)

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)
	b.WriteString(row)
	b.WriteString("\n")

	// Dynamic help bar.
	b.WriteString(m.renderHelpBar())

	return b.String()
}

// overviewPanelHeight returns the inner content height for overview panels.
func (m Model) overviewPanelHeight() int {
	if m.Height > 6 {
		return m.Height - 6 // reserve header(3) + footer(2) + newline(1)
	}
	return 20 // fallback
}

// panelStyle returns a bordered panel style sized to (w, h).
func (m Model) panelStyle(w, h int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorOverlay).
		Width(w - 2).  // -2 for border chars
		Height(h - 2). // -2 for border chars
		Padding(0, 1)
}

// panelTitle renders a small bold title line inside a panel.
func (m Model) panelTitle(s string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMauve).
		Render(s)
}

// ─── Worktrees panel ──────────────────────────────────────────────────────────

func (m Model) renderWorktreesPanel(w, h int) string {
	var b strings.Builder
	b.WriteString(m.panelTitle("Worktrees"))
	b.WriteString("\n")

	if err, bad := m.Snap.Errors["worktrees"]; bad {
		b.WriteString(m.Theme.Error.Render("⚠ " + err.Error()))
	} else {
		wts := m.Snap.Worktrees
		if len(wts) == 0 {
			b.WriteString(m.Theme.Subtle.Render("(none)"))
		}
		for i, wt := range wts {
			line := fmt.Sprintf("%-8s  %s", wt.Figura, shortenBranch(wt.Branch))
			if i == m.Cursor {
				b.WriteString(m.Theme.ListSelected.Render("▶ " + line))
			} else {
				b.WriteString(m.Theme.ListItem.Render(line))
			}
			b.WriteString("\n")
		}
	}

	return m.panelStyle(w, h).Render(b.String())
}

// ─── Merge plan panel ─────────────────────────────────────────────────────────

func (m Model) renderMergePlanPanel(w, h int) string {
	var b strings.Builder
	b.WriteString(m.panelTitle("Merge Order"))
	b.WriteString("\n")

	if err, bad := m.Snap.Errors["mergeplan"]; bad {
		b.WriteString(m.Theme.Error.Render("⚠ " + err.Error()))
		return m.panelStyle(w, h).Render(b.String())
	}

	plan := m.Snap.MergePlan
	b.WriteString(m.Theme.Subtle.Render(fmt.Sprintf("base: %s", plan.BaseBranch)))
	b.WriteString("\n")
	b.WriteString(m.Theme.Subtle.Render(fmt.Sprintf("conflict rate: %.0f%%  (threshold %.0f%%)",
		plan.ConflictRate*100, plan.Threshold*100)))
	b.WriteString("\n\n")

	if len(plan.Steps) == 0 {
		b.WriteString(m.Theme.Subtle.Render("(no steps)"))
	}
	for _, step := range plan.Steps {
		icon := "✓"
		style := m.Theme.StatusOK
		if !step.PredictedClean {
			icon = "✗"
			style = m.Theme.StatusDirty
		}
		line := fmt.Sprintf("%d. %-8s %s %s",
			step.Position, step.Figura,
			shortenBranch(step.Branch),
			style.Render(fmt.Sprintf("[%s +%d]", icon, step.CommitsAhead)))
		b.WriteString(m.Theme.ListItem.Render(line))
		b.WriteString("\n")
	}

	return m.panelStyle(w, h).Render(b.String())
}

// ─── Overlap panel ────────────────────────────────────────────────────────────

func (m Model) renderOverlapPanel(w, h int) string {
	var b strings.Builder
	b.WriteString(m.panelTitle("Overlap"))
	b.WriteString("\n")

	if err, bad := m.Snap.Errors["overlap"]; bad {
		b.WriteString(m.Theme.Error.Render("⚠ " + err.Error()))
		return m.panelStyle(w, h).Render(b.String())
	}

	ov := m.Snap.Overlap

	verdictStyle := m.Theme.StatusOK
	if ov.Verdict != "clean" {
		verdictStyle = m.Theme.StatusDirty
	}
	b.WriteString(verdictStyle.Render(fmt.Sprintf("verdict: %s", ov.Verdict)))
	b.WriteString("\n")
	b.WriteString(m.Theme.Subtle.Render(fmt.Sprintf("collision rate: %.0f%%  (%d/%d pairs)",
		ov.CollisionRate*100, ov.CollidingPairs, ov.PairsEvaluated)))
	b.WriteString("\n\n")

	if len(ov.FileCollisions) > 0 {
		b.WriteString(m.Theme.Badge.Render("File collisions:"))
		b.WriteString("\n")
		for _, fc := range ov.FileCollisions {
			line := fmt.Sprintf("  %s ← %s", shortenFile(fc.File), strings.Join(fc.Agents, ", "))
			b.WriteString(m.Theme.Error.Render(line))
			b.WriteString("\n")
		}
	}

	if len(ov.Advisories) > 0 {
		b.WriteString("\n")
		b.WriteString(m.Theme.Badge.Render("Advisories:"))
		b.WriteString("\n")
		for _, adv := range ov.Advisories {
			b.WriteString(m.Theme.Subtle.Render("  · " + adv))
			b.WriteString("\n")
		}
	}

	return m.panelStyle(w, h).Render(b.String())
}

// ─── Hybrid layout ────────────────────────────────────────────────────────────

// viewHybrid renders the Hybrid layout:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│ TALOS — Hybrid                                                           │
//	├──────────────────────┬──────────────────────┬──────────────────────────┤
//	│ Worktrees            │ Merge (compact)       │ Overlap (compact)        │
//	├──────────────────────┴──────────────────────┴──────────────────────────┤
//	│ Detail: <selected worktree>  branch · head · status                     │
//	├─────────────────────────────────────────────────────────────────────────┤
//	│ help bar                                                                 │
//	└─────────────────────────────────────────────────────────────────────────┘
//
// The top row reuses the overview panel helpers; the detail pane reuses the
// master-detail row renderer. Heights are split so the detail pane takes ~⅓.
func (m Model) viewHybrid() string {
	var b strings.Builder

	// Header.
	title := m.Theme.Header.Width(m.effectiveWidth()).Render("  TALOS — Hybrid")
	b.WriteString(title)
	b.WriteString("\n")

	total := m.effectiveWidth()
	colW := total / 3
	col1W := total - colW*2

	// Reserve space: header(3) + detail-panel(~7) + footer(2) + newline(1).
	const detailPanelHeight = 7
	topPanelHeight := m.hybridTopPanelHeight(detailPanelHeight)

	// Top row — three compact panels (same as overview but shorter).
	left := m.renderWorktreesPanel(col1W, topPanelHeight)
	mid := m.renderMergePlanPanel(colW, topPanelHeight)
	right := m.renderOverlapPanel(colW, topPanelHeight)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right))
	b.WriteString("\n")

	// Detail pane for the currently-selected worktree.
	b.WriteString(m.renderSelectedDetail(total, detailPanelHeight))
	b.WriteString("\n")

	// Dynamic help bar.
	b.WriteString(m.renderHelpBar())

	return b.String()
}

// hybridTopPanelHeight computes the height for the top overview panels.
func (m Model) hybridTopPanelHeight(detailH int) int {
	const reserved = 6 // header(3) + footer(2) + newline(1)
	h := m.Height - reserved - detailH
	if h < 4 {
		return 8 // safe fallback
	}
	return h
}

// renderSelectedDetail renders a bordered detail pane for the currently-selected
// worktree. If the list is empty it shows a placeholder.
func (m Model) renderSelectedDetail(width, height int) string {
	var b strings.Builder
	b.WriteString(m.panelTitle("Selected Worktree"))
	b.WriteString("\n")

	wts := m.Snap.Worktrees
	if len(wts) == 0 || m.Cursor >= len(wts) {
		b.WriteString(m.Theme.Subtle.Render("(no worktree selected)"))
	} else {
		wt := wts[m.Cursor]
		// Branch line.
		b.WriteString(m.Theme.Badge.Render("branch  "))
		b.WriteString(m.Theme.ListItem.Render(wt.Branch))
		b.WriteString("\n")
		// Head line.
		b.WriteString(m.Theme.Badge.Render("head    "))
		b.WriteString(m.Theme.Subtle.Render(shortHead(wt.Head)))
		b.WriteString("\n")
		// Status line.
		b.WriteString(m.Theme.Badge.Render("status  "))
		b.WriteString(m.renderStatus(wt.Status))
		b.WriteString("\n")
		// Figura line.
		b.WriteString(m.Theme.Badge.Render("figura  "))
		b.WriteString(m.Theme.ListItem.Render(wt.Figura))
	}

	return m.panelStyle(width, height).Render(b.String())
}

// ─── Toast ────────────────────────────────────────────────────────────────────

// renderToast renders the transient action-result notification line.
// Success messages (starting with ✓) use StatusOK; errors use Error style.
func (m Model) renderToast() string {
	if strings.HasPrefix(m.Toast, "✓") {
		return m.Theme.StatusOK.Render("  " + m.Toast)
	}
	return m.Theme.Error.Render("  " + m.Toast)
}

// ─── Confirmation modal ───────────────────────────────────────────────────────

// overlayModal renders a centered confirmation dialog box over the base view.
// It uses lipgloss.Place to center the box both horizontally and vertically.
func (m Model) overlayModal(base string) string {
	w := m.effectiveWidth()
	h := m.Height
	if h <= 0 {
		h = 30
	}

	titleLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMauve).
		Render(m.ModalTitle)

	msgLine := m.Theme.ListItem.Render(m.ModalMessage)

	hint := m.Theme.Subtle.Render("[y/enter] confirm   [n/esc] cancel")

	inner := strings.Join([]string{titleLine, "", msgLine, "", hint}, "\n")

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorLavender).
		Padding(1, 3).
		Width(w / 2)

	box := boxStyle.Render(inner)

	// Place the modal box centered over the terminal canvas.
	// The whitespace fill uses colorBase so the box floats over a dark backdrop.
	return lipgloss.Place(w+4, h, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(colorBase),
	)
}

// ─── Create-worktree modal ────────────────────────────────────────────────────

// overlayCreateModal renders the text-input create modal centered over base.
func (m Model) overlayCreateModal(base string) string {
	w := m.effectiveWidth()
	h := m.Height
	if h <= 0 {
		h = 30
	}

	titleLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMauve).
		Render(m.ModalTitle)

	figuraLabel := m.Theme.Subtle.Render("figura  ")
	figuraField := m.CreateFiguraInput.View()

	jiraLabel := m.Theme.Subtle.Render("jira key")
	jiraField := m.CreateJiraKeyInput.View()

	// Validation hint — shown when inputs are non-empty but invalid.
	var hint string
	figVal := m.CreateFiguraInput.Value()
	jiraVal := m.CreateJiraKeyInput.Value()
	switch {
	case figVal != "" && !reValidFigura.MatchString(figVal):
		hint = m.Theme.Error.Render("figura: lowercase letters only")
	case jiraVal != "" && !reValidJiraKey.MatchString(jiraVal):
		hint = m.Theme.Error.Render("key: must match TAL-NNN")
	default:
		hint = m.Theme.Subtle.Render("[tab] next   [enter] confirm   [esc] cancel")
	}

	inner := strings.Join([]string{
		titleLine,
		"",
		figuraLabel + figuraField,
		jiraLabel + " " + jiraField,
		"",
		hint,
	}, "\n")

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorLavender).
		Padding(1, 3).
		Width(w / 2)

	box := boxStyle.Render(inner)

	return lipgloss.Place(w+4, h, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(colorBase),
	)
}

// ─── String helpers ───────────────────────────────────────────────────────────

// shortenBranch trims "agent/<figura>/" prefix for compact display.
func shortenBranch(b string) string {
	parts := strings.SplitN(b, "/", 3)
	if len(parts) == 3 && parts[0] == "agent" {
		return parts[2] // e.g. "TAL-1"
	}
	return b
}

// shortenFile keeps only the last two path segments for compact display.
func shortenFile(f string) string {
	parts := strings.Split(f, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return f
}
