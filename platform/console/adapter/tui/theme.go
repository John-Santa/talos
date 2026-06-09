// Package tui is the inbound adapter that renders the Talos TUI using Bubbletea.
package tui

import "github.com/charmbracelet/lipgloss"

// ─── Colors ───────────────────────────────────────────────────────────────────

// Rose Pinecolors lifted directly from engram's palette so the two tools
// feel cohesive on a developer's terminal.
var (
	colorBase     = lipgloss.Color("#191724") // deep purple/black base
	colorSurface  = lipgloss.Color("#1f1d2e") // panel background
	colorOverlay  = lipgloss.Color("#6e6a86") // muted borders
	colorText     = lipgloss.Color("#e0def4") // primary text
	colorSubtle   = lipgloss.Color("#908caa") // dim / timestamp
	colorLavender = lipgloss.Color("#c4a7e7") // brand accent / selected
	colorGreen    = lipgloss.Color("#9ccfd8") // success / clean status
	colorPeach    = lipgloss.Color("#f6c177") // warm accent / figura badge
	colorRed      = lipgloss.Color("#eb6f92") // error
	colorMauve    = lipgloss.Color("#ebbcba") // header / title
)

// ─── Theme ────────────────────────────────────────────────────────────────────

// Theme holds the compiled lipgloss styles used to render the TUI.
// It is swappable — store it as a field on Model to support future theme commands.
type Theme struct {
	Header       lipgloss.Style
	ListItem     lipgloss.Style
	ListSelected lipgloss.Style
	Subtle       lipgloss.Style
	Error        lipgloss.Style
	Footer       lipgloss.Style
	Badge        lipgloss.Style
	StatusOK     lipgloss.Style
	StatusDirty  lipgloss.Style
	Border       lipgloss.Style
}

// RosePine returns the default Rosé Pine theme.
// Named colors are intentionally kept as package-level vars so they can be
// referenced directly from the same package without threading the theme
// through every helper.
func RosePine() Theme {
	return Theme{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorOverlay).
			PaddingBottom(1).
			MarginBottom(1),

		ListItem: lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(2),

		ListSelected: lipgloss.NewStyle().
			Foreground(colorLavender).
			Bold(true).
			PaddingLeft(1),

		Subtle: lipgloss.NewStyle().
			Foreground(colorSubtle).
			Italic(true),

		Error: lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(colorSubtle).
			MarginTop(1),

		Badge: lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true),

		StatusOK: lipgloss.NewStyle().
			Foreground(colorGreen),

		StatusDirty: lipgloss.NewStyle().
			Foreground(colorPeach),

		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorOverlay).
			Padding(0, 1),
	}
}
