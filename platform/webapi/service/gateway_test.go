package service

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// fakeReader returns canned platform state (no git, no files).
type fakeReader struct{}

func (fakeReader) Worktrees(context.Context) ([]domain.WtEntry, error) {
	return []domain.WtEntry{
		{Figura: "hermes", Branch: "agent/hermes/TAL-15", Head: "2e61d7e", Status: "active"},
		{Figura: "iris", Branch: "agent/iris/TAL-22", Head: "ec3ff1a", Status: "active"},
	}, nil
}

func (fakeReader) MergePlan(context.Context) (domain.MoPlan, error) {
	return domain.MoPlan{
		BaseBranch: "develop",
		Threshold:  0.15,
		Steps: []domain.MoPlanStep{
			{Position: 1, Branch: "agent/hermes/TAL-15", Figura: "hermes", CommitsAhead: 4, PredictedClean: true},
		},
	}, nil
}

func (fakeReader) Overlap(context.Context) (domain.OvScan, error) {
	return domain.OvScan{Verdict: "ok", PairsEvaluated: 1, CollidingPairs: 0}, nil
}

func (fakeReader) Ownership(context.Context) (map[string]string, error) {
	return map[string]string{"module:devops": "hermes", "module:frontend": "iris"}, nil
}

func (fakeReader) Ready(context.Context) error { return nil }

func (fakeReader) Labels(_ context.Context, _ string) (domain.ChLabels, error) {
	return domain.ChLabels{}, nil
}

func (fakeReader) CreateWorktree(context.Context, string, string) error { return nil }
func (fakeReader) TeardownWorktree(context.Context, string) error       { return nil }
func (fakeReader) Merge(_ context.Context, figura, jiraKey string) error {
	_ = figura
	_ = jiraKey
	return nil
}

func newGateway() *Gateway { return NewGateway(fakeReader{}, fakeReader{}) }

func TestGatewayOrchestration(t *testing.T) {
	snap, err := newGateway().Orchestration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Worktrees) != 2 {
		t.Errorf("worktrees = %d, want 2", len(snap.Worktrees))
	}
	if snap.MergeOrder.Threshold != 15 {
		t.Errorf("threshold = %v, want 15", snap.MergeOrder.Threshold)
	}
	if snap.Worktrees[0].Module != "devops" {
		t.Errorf("hermes module = %q, want devops", snap.Worktrees[0].Module)
	}
}

func TestGatewayAgentHasWorktree(t *testing.T) {
	detail, err := newGateway().Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Agent.Name != "Hermes" {
		t.Errorf("agent name = %q, want Hermes", detail.Agent.Name)
	}
	if detail.Worktree == nil || detail.Worktree.Branch != "agent/hermes/TAL-15" {
		t.Errorf("worktree mapped wrong: %+v", detail.Worktree)
	}
}

func TestGatewayAgentUnknown(t *testing.T) {
	if _, err := newGateway().Agent(context.Background(), "nobody"); err == nil {
		t.Error("want error for unknown figura")
	}
}

func TestGatewayAgents(t *testing.T) {
	if got := len(newGateway().Agents(context.Background())); got != 10 {
		t.Errorf("agents = %d, want 10", got)
	}
}

func TestGatewayReady(t *testing.T) {
	if err := newGateway().Ready(context.Background()); err != nil {
		t.Errorf("Ready = %v, want nil", err)
	}
}

func TestGatewayWriteDelegation(t *testing.T) {
	g := newGateway()
	ctx := context.Background()
	if err := g.CreateWorktree(ctx, "atlas", "TAL-99"); err != nil {
		t.Errorf("CreateWorktree = %v", err)
	}
	if err := g.TeardownWorktree(ctx, "atlas"); err != nil {
		t.Errorf("TeardownWorktree = %v", err)
	}
	if err := g.MergeWorktree(ctx, "iris", "TAL-15"); err != nil {
		t.Errorf("MergeWorktree = %v", err)
	}
}

func TestGatewayJudgmentMinimal(t *testing.T) {
	rev, err := newGateway().Judgment(context.Background(), "TAL-15")
	if err != nil {
		t.Fatal(err)
	}
	if rev.JiraKey != "TAL-15" || rev.Verdict != "agree" {
		t.Errorf("judgment = %+v, want minimal agree", rev)
	}
}

// --- PR2 tests ---------------------------------------------------------------

func TestAgentNormalizesCase(t *testing.T) {
	tests := []struct {
		name    string
		figura  string
		wantErr bool
	}{
		{name: "mixed-case Hermes → 200", figura: "Hermes", wantErr: false},
		{name: "all-caps HERMES → 200", figura: "HERMES", wantErr: false},
		{name: "lower hermes → 200", figura: "hermes", wantErr: false},
		{name: "unknown unicorn → error", figura: "unicorn", wantErr: true},
	}
	g := newGateway()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail, err := g.Agent(context.Background(), tt.figura)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for figura %q, got detail %+v", tt.figura, detail)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for figura %q: %v", tt.figura, err)
			}
			if !tt.wantErr && detail.Agent.ID != "hermes" {
				t.Errorf("agent.ID = %q, want hermes", detail.Agent.ID)
			}
		})
	}
}

func TestCreateWorktreeRejectsUnknownFigura(t *testing.T) {
	tests := []struct {
		name     string
		figura   string
		wantErr  bool
		wantCode bool // true = expect ErrUnknownFigura
	}{
		{name: "valid figura iris → ok", figura: "iris", wantErr: false},
		{name: "invalid figura bogus → ErrUnknownFigura", figura: "bogus", wantErr: true, wantCode: true},
		{name: "mixed-case Iris → ok (normalized)", figura: "Iris", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGateway()
			err := g.CreateWorktree(context.Background(), tt.figura, "TAL-99")
			if tt.wantErr && err == nil {
				t.Errorf("expected error for figura %q", tt.figura)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for figura %q: %v", tt.figura, err)
			}
			if tt.wantCode && !errors.Is(err, domain.ErrUnknownFigura) {
				t.Errorf("expected ErrUnknownFigura for figura %q, got: %v", tt.figura, err)
			}
		})
	}
}
