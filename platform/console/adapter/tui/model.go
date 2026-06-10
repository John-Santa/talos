package tui

import (
	"context"
	"time"

	"github.com/John-Santa/talos/platform/console/port"
	"github.com/John-Santa/talos/platform/console/service"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Layout mode ──────────────────────────────────────────────────────────────

// LayoutMode controls which panel arrangement the TUI renders.
type LayoutMode int

const (
	// LayoutMasterDetail is the default: full-width worktree list.
	LayoutMasterDetail LayoutMode = iota
	// LayoutOverview shows three panels: worktrees, merge-order, overlap.
	LayoutOverview
	// LayoutHybrid combines the overview panels with a detail pane for the
	// currently-selected worktree. It is the third layout in the cycle.
	LayoutHybrid
)

// cycleLayout advances to the next layout in the 3-way cycle, wrapping around.
func cycleLayout(l LayoutMode) LayoutMode {
	return (l + 1) % 3
}

// ─── Modal mode ───────────────────────────────────────────────────────────────

// ModalMode distinguishes the type of modal currently displayed.
type ModalMode int

const (
	// ModalModeConfirm is the yes/no confirmation dialog (e.g. teardown).
	ModalModeConfirm ModalMode = iota
	// ModalModeCreate is the text-input dialog for creating a new worktree.
	ModalModeCreate
	// ModalModeExecute is the type-to-confirm dialog for the execute-merge action.
	// The user must type the base branch name exactly to enable the confirm button.
	ModalModeExecute
)

// ─── Key bindings ─────────────────────────────────────────────────────────────

// keyMap declares the key bindings for the TUI. It implements help.KeyMap so
// bubbles/help can render short and full help bars automatically.
type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	SwitchLayout key.Binding
	Help         key.Binding
	Quit         key.Binding
}

// ShortHelp returns the bindings shown in the compact one-line help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.SwitchLayout, k.Help, k.Quit}
}

// FullHelp returns all bindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.SwitchLayout, k.Help, k.Quit},
	}
}

// defaultKeyMap is the singleton key map used by every Model.
var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "down"),
	),
	SwitchLayout: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "layout"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// ─── Custom messages ──────────────────────────────────────────────────────────

// SnapshotMsg is delivered to the Model when the async Aggregator.Snapshot call
// completes. It is exported so tests can inject a fixed snapshot without
// triggering real CLI calls.
type SnapshotMsg struct {
	Snap service.Snapshot
}

// ActionResultMsg is delivered when an async actor command (e.g. TeardownWorktree)
// completes. Err is nil on success.
type ActionResultMsg struct {
	Err error
}

// TickMsg is delivered on every live-refresh interval. It triggers a background
// reload and re-arms the next tick.
type TickMsg struct{}

// refreshInterval is the default live-refresh cadence.
const refreshInterval = 10 * time.Second

// tickCmd returns a Cmd that fires a TickMsg after the given interval.
func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return TickMsg{} })
}

// ─── Model ────────────────────────────────────────────────────────────────────

// Model is the single Bubbletea Model for the Talos TUI.
// It holds ALL state; sub-states will be added as screen constants in PR-4+.
type Model struct {
	agg   *service.Aggregator
	actor port.PlatformActor

	Snap   service.Snapshot
	Theme  Theme
	Width  int
	Height int
	Cursor int
	// Loading is true from construction until the first SnapshotMsg arrives.
	Loading bool
	// Refreshing is true while a live-refresh reload is in-flight (armed by
	// tickCmd). It is cleared when the resulting SnapshotMsg arrives.
	Refreshing bool
	// Layout controls which panel arrangement is rendered.
	Layout LayoutMode

	// ─── Help bar ──────────────────────────────────────────────────────────────
	// keys is the compiled key-binding map used by the help component.
	keys keyMap
	// helpModel renders the short/full help bar at the bottom of the screen.
	helpModel help.Model
	// HelpShowAll is true when the user has expanded the help bar with ?.
	HelpShowAll bool

	// ─── Viewport (worktree list) ──────────────────────────────────────────────
	// vp is the bubbles viewport that gives the worktree list smooth scrolling.
	vp viewport.Model
	// ViewportYOffset exposes the current scroll offset for testing.
	ViewportYOffset int

	// ─── Modal (shared) ────────────────────────────────────────────────────────
	// ModalActive is true when any modal dialog is displayed.
	ModalActive bool
	// ModalMode identifies which kind of modal is shown.
	ModalMode ModalMode
	// ModalTitle is the heading line of the modal.
	ModalTitle string
	// ModalMessage is the body text of the confirmation modal.
	ModalMessage string
	// modalFigura / modalJiraKey hold the teardown target captured when the
	// teardown modal was opened so confirm can dispatch the action.
	modalFigura  string
	modalJiraKey string

	// ─── Create-worktree modal ─────────────────────────────────────────────────
	// CreateFiguraInput is the text input for the agent figura (lowercase letters).
	CreateFiguraInput textinput.Model
	// CreateJiraKeyInput is the text input for the Jira issue key (TAL-NNN).
	CreateJiraKeyInput textinput.Model

	// ─── Execute-merge modal ───────────────────────────────────────────────────
	// ExecuteTokenInput is the text input where the user types the base branch
	// name to confirm the destructive execute-merge action.
	ExecuteTokenInput textinput.Model
	// modalBaseBranch holds the base branch captured from the snapshot when the
	// execute modal was opened; the typed token must match it exactly to confirm.
	modalBaseBranch string

	// ─── Toast ─────────────────────────────────────────────────────────────────
	// Toast is the transient single-line notification shown after an action
	// completes (success or error). Empty string means no toast is displayed.
	// Auto-dismiss timer is deferred to a later slice.
	Toast string
}

// New constructs the initial Model connected to agg with no write actor.
// The model starts in the loading state; the first Init Cmd triggers the async
// data load.
//
// Use NewWithActor when write operations (teardown, merge, etc.) are needed.
func New(agg *service.Aggregator) Model {
	return NewWithActor(agg, nil)
}

// NewWithActor constructs the initial Model connected to agg and actor.
// actor may be nil — the TUI degrades gracefully (action keys are ignored).
func NewWithActor(agg *service.Aggregator, actor port.PlatformActor) Model {
	h := help.New()
	h.ShowAll = false

	figuraInput := textinput.New()
	figuraInput.Placeholder = "figura (e.g. iris)"
	figuraInput.CharLimit = 32

	jiraKeyInput := textinput.New()
	jiraKeyInput.Placeholder = "Jira key (e.g. TAL-19)"
	jiraKeyInput.CharLimit = 20

	executeTokenInput := textinput.New()
	executeTokenInput.Placeholder = "type base branch to confirm"
	executeTokenInput.CharLimit = 64

	return Model{
		agg:                agg,
		actor:              actor,
		Theme:              RosePine(),
		Loading:            true,
		keys:               defaultKeyMap,
		helpModel:          h,
		vp:                 viewport.New(0, 0),
		CreateFiguraInput:  figuraInput,
		CreateJiraKeyInput: jiraKeyInput,
		ExecuteTokenInput:  executeTokenInput,
	}
}

// WithSnapshot returns a copy of m with the given snapshot injected and
// Loading cleared to false. Used in tests to bypass the async load path.
func (m Model) WithSnapshot(snap service.Snapshot) Model {
	m.Snap = snap
	m.Loading = false
	return m
}

// Init returns the Bubbletea Cmd that launches the TUI (alt-screen), kicks
// off the first async snapshot load, and arms the live-refresh tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		loadSnapshot(m.agg),
		tickCmd(refreshInterval),
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
