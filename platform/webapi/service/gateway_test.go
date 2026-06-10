package service

import (
	"context"
	"fmt"
	"testing"
)

// fakeExec returns canned CLI output keyed by "<name> <subcommand>".
type fakeExec struct {
	out map[string]string
}

func (f fakeExec) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + args[0]
	}
	if v, ok := f.out[key]; ok {
		return []byte(v), nil
	}
	return nil, fmt.Errorf("no fake output for %q", key)
}

var canned = map[string]string{
	"wt list":      `[{"figura":"hermes","branch":"agent/hermes/TAL-15","head":"2e61d7e","status":"active"},{"figura":"iris","branch":"agent/iris/TAL-22","head":"ec3ff1a","status":"active"}]`,
	"mo plan":      `{"base_branch":"develop","conflict_rate":0.0,"threshold":0.15,"steps":[{"position":1,"branch":"agent/hermes/TAL-15","figura":"hermes","commits_ahead":4,"predicted_clean":true}]}`,
	"ov scan":      `{"verdict":"ok","collision_rate":0.0,"pairs_evaluated":0,"colliding_pairs":0}`,
	"ch ownership": `{"modules":{"module:devops":"hermes","module:frontend":"iris"}}`,
	"ch judgment":  `{"judges":["jd-judge-a","jd-judge-b"],"verdict":"changes","violations":["verify-report missing"]}`,
}

func newGateway() *Gateway { return NewGateway(fakeExec{out: canned}) }

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

func TestGatewayJudgment(t *testing.T) {
	rev, err := newGateway().Judgment(context.Background(), "TAL-15")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Verdict != "conflict" || rev.EscalateTo != "zeus" {
		t.Errorf("judgment mapped wrong: %+v", rev)
	}
}

func TestGatewayAgents(t *testing.T) {
	if got := len(newGateway().Agents(context.Background())); got != 10 {
		t.Errorf("agents = %d, want 10", got)
	}
}
