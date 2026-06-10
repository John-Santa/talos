// Package service aggregates the platform state into the web-facing payloads.
package service

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/webapi/domain"
	"github.com/John-Santa/talos/platform/webapi/port"
)

// Gateway maps the platform state (read via a PlatformReader — git + ownership
// file, no binaries) onto the web domain.
type Gateway struct {
	reader port.PlatformReader
}

// NewGateway constructs a Gateway over the given reader.
func NewGateway(r port.PlatformReader) *Gateway {
	return &Gateway{reader: r}
}

// Ready reports whether the underlying repo/tooling is usable.
func (g *Gateway) Ready(ctx context.Context) error {
	return g.reader.Ready(ctx)
}

// Orchestration returns the merged snapshot for the main screen. Worktrees are
// required; merge-order/overlap/ownership are best-effort.
func (g *Gateway) Orchestration(ctx context.Context) (domain.OrchestrationSnapshot, error) {
	wts, err := g.reader.Worktrees(ctx)
	if err != nil {
		return domain.OrchestrationSnapshot{}, err
	}
	plan, _ := g.reader.MergePlan(ctx)
	scan, _ := g.reader.Overlap(ctx)
	own, _ := g.reader.Ownership(ctx)
	return domain.BuildSnapshot(wts, plan, scan, own), nil
}

// Agents returns the full roster.
func (g *Gateway) Agents(_ context.Context) []domain.Agent {
	return domain.AllAgents()
}

// Agent returns an agent's detail. DoD and activity have no offline source, so
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
	if wts, err := g.reader.Worktrees(ctx); err == nil {
		own, _ := g.reader.Ownership(ctx)
		plan, _ := g.reader.MergePlan(ctx)
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

// Judgment has no offline source (ch judgment needs Jira); returns a minimal
// review so the front degrades gracefully.
func (g *Gateway) Judgment(_ context.Context, jiraKey string) (domain.JudgmentReview, error) {
	return domain.JudgmentReview{
		JiraKey:  jiraKey,
		Gate:     "HG5",
		Judges:   []domain.Judge{},
		FixAgent: "idle",
		Verdict:  "agree",
	}, nil
}
