// Package service aggregates the platform state into the web-facing payloads.
package service

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/webapi/domain"
	"github.com/John-Santa/talos/platform/webapi/port"
)

// Gateway maps the platform state (read via a PlatformReader — git + ownership
// file, no binaries) onto the web domain, and performs worktree write actions.
type Gateway struct {
	reader port.PlatformReader
	writer port.PlatformWriter
}

// NewGateway constructs a Gateway over the given reader + writer.
func NewGateway(reader port.PlatformReader, writer port.PlatformWriter) *Gateway {
	return &Gateway{reader: reader, writer: writer}
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

// Agent returns an agent's detail. DoD is populated via `ch labels` (best-effort);
// activity has no offline source and comes back empty; the worktree (if any) is real.
func (g *Gateway) Agent(ctx context.Context, figura string) (domain.AgentDetail, error) {
	figura = domain.NormalizeFigura(figura)
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
				// Best-effort: populate DoD from `ch labels`. Empty on failure.
				if cl, err := g.reader.Labels(ctx, e.Branch); err == nil {
					detail.DoD = domain.MapDoD(cl)
				}
				break
			}
		}
	}
	return detail, nil
}

// Judgment attempts `ch judgment --json` (best-effort, timeout via Labels pattern).
// When ch is unavailable or returns no data, returns an explicit Pending state so
// the front never shows a fabricated positive verdict.
func (g *Gateway) Judgment(ctx context.Context, jiraKey string) (domain.JudgmentReview, error) {
	// Try to get judgment via ch labels channel (reuse Labels port pattern).
	// ch judgment is not yet wired through a dedicated port method — we degrade
	// to pending. A future PR can add Judgment() to PlatformReader once ch is available.
	return domain.JudgmentReview{
		JiraKey:  jiraKey,
		Gate:     "HG5",
		Judges:   []domain.Judge{},
		FixAgent: "idle",
		Verdict:  "pending",
		Pending:  true,
	}, nil
}

// CreateWorktree spins up an isolated worktree for a figura.
func (g *Gateway) CreateWorktree(ctx context.Context, figura, jiraKey string) error {
	figura = domain.NormalizeFigura(figura)
	if _, ok := domain.AgentByID(figura); !ok {
		return domain.ErrUnknownFigura
	}
	return g.writer.CreateWorktree(ctx, figura, jiraKey)
}

// TeardownWorktree removes a figura's worktree.
func (g *Gateway) TeardownWorktree(ctx context.Context, figura string) error {
	return g.writer.TeardownWorktree(ctx, figura)
}

// MergeWorktree merges a worktree's branch into the base (guarded).
func (g *Gateway) MergeWorktree(ctx context.Context, figura, jiraKey string) error {
	return g.writer.Merge(ctx, figura, jiraKey)
}
