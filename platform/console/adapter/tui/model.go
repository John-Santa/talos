package tui

import (
	"context"

	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Custom messages ──────────────────────────────────────────────────────────

// SnapshotMsg is delivered to the Model when the async Aggregator.Snapshot call
// completes. It is exported so tests can inject a fixed snapshot without
// triggering real CLI calls.
type SnapshotMsg struct {
	Snap service.Snapshot
}

// ─── Model ────────────────────────────────────────────────────────────────────

// Model is the single Bubbletea Model for the Talos TUI.
// It holds ALL state; sub-states will be added as screen constants in PR-4+.
type Model struct {
	agg    *service.Aggregator
	Snap   service.Snapshot
	Theme  Theme
	Width  int
	Height int
	Cursor int
	// Loading is true from construction until the first SnapshotMsg arrives.
	Loading bool
}

// New constructs the initial Model connected to agg.
// The model starts in the loading state; the first Init Cmd triggers the async
// data load.
func New(agg *service.Aggregator) Model {
	return Model{
		agg:     agg,
		Theme:   RosePine(),
		Loading: true,
	}
}

// WithSnapshot returns a copy of m with the given snapshot injected and
// Loading cleared to false. Used in tests to bypass the async load path.
func (m Model) WithSnapshot(snap service.Snapshot) Model {
	m.Snap = snap
	m.Loading = false
	return m
}

// Init returns the Bubbletea Cmd that launches the TUI (alt-screen) and kicks
// off the first async snapshot load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		loadSnapshot(m.agg),
	)
}

// loadSnapshot is the async command that calls Aggregator.Snapshot in the
// background and wraps the result in a SnapshotMsg.
func loadSnapshot(agg *service.Aggregator) tea.Cmd {
	return func() tea.Msg {
		snap := agg.Snapshot(context.Background())
		return SnapshotMsg{Snap: snap}
	}
}
