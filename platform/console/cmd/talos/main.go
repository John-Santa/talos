// Command talos is the composition root for the Talos TUI.
//
// It wires the platform CLI reader → Aggregator → TUI Model and hands
// control to Bubbletea.  All four platform binaries (wt, mo, ov, ch) must be
// discoverable on PATH; if any are missing the Aggregator degrades gracefully
// and the TUI shows a partial view with an error note.
package main

import (
	"fmt"
	"os"

	"github.com/John-Santa/talos/platform/console/adapter/cli"
	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	reader := cli.NewReader("wt", "mo", "ov", "ch")
	agg := service.NewAggregator(reader)
	m := tui.New(agg)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "talos: %v\n", err)
		os.Exit(1)
	}
}
