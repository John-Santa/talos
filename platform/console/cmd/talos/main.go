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
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/console/adapter/cli"
	"github.com/John-Santa/talos/platform/console/adapter/tui"
	"github.com/John-Santa/talos/platform/console/internal/repoctx"
	"github.com/John-Santa/talos/platform/console/service"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	reader := cli.NewReader("wt", "mo", "ov", "ch")
	actor := cli.NewActor("wt", "mo")
	agg := service.NewAggregator(reader)
	m := tui.NewWithActor(agg, actor).WithRepoLabel(resolveRepoLabel())

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "talos: %v\n", err)
		os.Exit(1)
	}
}

// resolveRepoLabel runs two cheap git commands to determine the repo slug.
// Any error (no git, no remote, bare repo) degrades gracefully to "".
func resolveRepoLabel() string {
	remoteURL := gitOutput("remote", "get-url", "origin")
	toplevel := gitOutput("rev-parse", "--show-toplevel")
	return repoctx.ParseRepoSlug(remoteURL, toplevel)
}

// gitOutput runs git with the given args and returns trimmed stdout.
// Returns "" on any error.
func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
