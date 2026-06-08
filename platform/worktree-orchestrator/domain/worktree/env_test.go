package worktree_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

func TestAgentResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		figura     string
		wantPort   int
		wantSchema string
	}{
		{"atlas", 8100, "wt_atlas"},
		{"hephaestus", 8101, "wt_hephaestus"},
		{"cronos", 8102, "wt_cronos"},
		{"iris", 8103, "wt_iris"},
		{"gaia", 8104, "wt_gaia"},
		{"themis", 8105, "wt_themis"},
		{"hermes", 8106, "wt_hermes"},
		{"argos", 8107, "wt_argos"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.figura, func(t *testing.T) {
			t.Parallel()
			f := worktree.Figura(tc.figura)
			res, err := worktree.AgentResources(f)
			if err != nil {
				t.Fatalf("AgentResources(%q) unexpected error: %v", tc.figura, err)
			}
			if res.Port != tc.wantPort {
				t.Errorf("AgentResources(%q).Port = %d, want %d", tc.figura, res.Port, tc.wantPort)
			}
			if res.DBSchema != tc.wantSchema {
				t.Errorf("AgentResources(%q).DBSchema = %q, want %q", tc.figura, res.DBSchema, tc.wantSchema)
			}
		})
	}

	t.Run("unknown_figura", func(t *testing.T) {
		t.Parallel()
		_, err := worktree.AgentResources(worktree.Figura("zeus"))
		if err == nil {
			t.Fatal("AgentResources(zeus) expected error, got nil")
		}
		var e *worktree.ErrInvalidFigure
		if !errors.As(err, &e) {
			t.Errorf("error type = %T, want *ErrInvalidFigure", err)
		}
	})
}

func TestAgentResourcesDisjoint(t *testing.T) {
	t.Parallel()
	figuras := []worktree.Figura{
		"atlas", "hephaestus", "cronos", "iris", "gaia", "themis", "hermes", "argos",
	}

	ports := make(map[int]string)
	schemas := make(map[string]string)

	for _, f := range figuras {
		res, err := worktree.AgentResources(f)
		if err != nil {
			t.Fatalf("AgentResources(%q) unexpected error: %v", f, err)
		}
		if prev, ok := ports[res.Port]; ok {
			t.Errorf("port collision: %d shared by %q and %q", res.Port, prev, f)
		}
		ports[res.Port] = string(f)

		if prev, ok := schemas[res.DBSchema]; ok {
			t.Errorf("schema collision: %q shared by %q and %q", res.DBSchema, prev, f)
		}
		schemas[res.DBSchema] = string(f)
	}
}

func TestRenderEnv(t *testing.T) {
	t.Parallel()

	spec := worktree.WorktreeSpec{
		Figura:  worktree.Figura("hermes"),
		JiraKey: "TAL-2",
		Branch:  "agent/hermes/TAL-2",
		Path:    "talos.wt/agent-hermes",
	}
	res := worktree.Resources{
		Port:     8106,
		DBSchema: "wt_hermes",
	}

	got := worktree.RenderEnv(spec, res)

	want := "PORT=8106\nDB_SCHEMA=wt_hermes\n"
	if got != want {
		t.Errorf("RenderEnv() =\n%q\nwant:\n%q", got, want)
	}

	if strings.Contains(got, "JIRA_") {
		t.Errorf("RenderEnv() output contains JIRA_ credential — Decision 5 violation:\n%s", got)
	}

	got2 := worktree.RenderEnv(spec, res)
	if got != got2 {
		t.Errorf("RenderEnv() is not deterministic:\nfirst:  %q\nsecond: %q", got, got2)
	}
}
