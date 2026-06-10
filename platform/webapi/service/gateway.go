// Package service aggregates the platform CLIs into the web-facing payloads.
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/John-Santa/talos/platform/webapi/domain"
	"github.com/John-Santa/talos/platform/webapi/port"
)

// Gateway reads orchestration state through the CLI executor and maps it onto
// the web domain. wt is required; mo/ov/ch are best-effort (zero on failure).
type Gateway struct {
	exec port.CLIExecutor
}

// NewGateway constructs a Gateway over the given executor.
func NewGateway(e port.CLIExecutor) *Gateway {
	return &Gateway{exec: e}
}

func (g *Gateway) worktrees(ctx context.Context) ([]domain.WtEntry, error) {
	out, err := g.exec.Run(ctx, "wt", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("wt list: %w", err)
	}
	var entries []domain.WtEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("wt list decode: %w", err)
	}
	return entries, nil
}

func (g *Gateway) plan(ctx context.Context) domain.MoPlan {
	var p domain.MoPlan
	if out, err := g.exec.Run(ctx, "mo", "plan", "--json"); err == nil {
		_ = json.Unmarshal(out, &p)
	}
	return p
}

func (g *Gateway) scan(ctx context.Context) domain.OvScan {
	var s domain.OvScan
	if out, err := g.exec.Run(ctx, "ov", "scan", "--json"); err == nil {
		_ = json.Unmarshal(out, &s)
	}
	return s
}

func (g *Gateway) ownership(ctx context.Context) map[string]string {
	var o domain.ChOwnership
	if out, err := g.exec.Run(ctx, "ch", "ownership", "--json"); err == nil {
		_ = json.Unmarshal(out, &o)
	}
	return o.Modules
}

// Orchestration returns the merged snapshot for the main screen.
func (g *Gateway) Orchestration(ctx context.Context) (domain.OrchestrationSnapshot, error) {
	wts, err := g.worktrees(ctx)
	if err != nil {
		return domain.OrchestrationSnapshot{}, err
	}
	return domain.BuildSnapshot(wts, g.plan(ctx), g.scan(ctx), g.ownership(ctx)), nil
}

// Agents returns the full roster.
func (g *Gateway) Agents(_ context.Context) []domain.Agent {
	return domain.AllAgents()
}

// Agent returns an agent's detail. DoD and activity have no CLI source yet, so
// they come back empty; the worktree (if any) is real.
func (g *Gateway) Agent(ctx context.Context, figura string) (domain.AgentDetail, error) {
	agent, ok := domain.AgentByID(figura)
	if !ok {
		return domain.AgentDetail{}, fmt.Errorf("unknown figura %q", figura)
	}
	detail := domain.AgentDetail{
		Agent:    agent,
		DoD:      []domain.DoDItem{},
		Activity: []domain.ActivityEntry{},
	}
	if wts, err := g.worktrees(ctx); err == nil {
		own := g.ownership(ctx)
		plan := g.plan(ctx)
		for _, e := range wts {
			if domain.NormalizeFigura(e.Figura) == figura {
				w := domain.MapWorktree(e, own, plan)
				detail.Worktree = &w
				break
			}
		}
	}
	return detail, nil
}

// Judgment returns the Judgment Day payload for an issue (best-effort from ch).
func (g *Gateway) Judgment(ctx context.Context, jiraKey string) (domain.JudgmentReview, error) {
	review := domain.JudgmentReview{
		JiraKey:  jiraKey,
		Gate:     "HG5",
		Judges:   []domain.Judge{},
		FixAgent: "idle",
		Verdict:  "agree",
	}
	if out, err := g.exec.Run(ctx, "ch", "judgment", "--json"); err == nil {
		var cj domain.ChJudgment
		if json.Unmarshal(out, &cj) == nil {
			review = domain.MapJudgment(jiraKey, cj)
		}
	}
	return review, nil
}
