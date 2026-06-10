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

// Agent returns an agent's detail. Activity is populated via `runs timeline`
// (best-effort); DoD is populated via `runs dod` when runs has data, falling
// back to `ch labels` (best-effort); the worktree (if any) is real.
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

				jiraKey := domain.ParseJiraKey(e.Branch)

				// Activity: populated from `runs timeline` (best-effort).
				// Ensure Activity is always a non-nil slice (front expects []).
				if acts, err := g.reader.Activity(ctx, jiraKey); err == nil && acts != nil {
					detail.Activity = acts
				}

				// DoD: prefer `runs dod` (historical source); fall back to `ch labels` (live state).
				if runsItems, err := g.reader.RunsDoD(ctx, jiraKey); err == nil && len(runsItems) > 0 {
					detail.DoD = runsItems
				} else if cl, err := g.reader.Labels(ctx, e.Branch); err == nil {
					detail.DoD = domain.MapDoD(cl)
				}

				break
			}
		}
	}
	return detail, nil
}

// Judgment calls `runs judgment --jira-key <k> --json` (best-effort via runsRun).
// When runs has a recorded JudgmentReview and Pending is false, that real verdict
// is returned. In all other cases (runs absent, error, or no events) the method
// degrades to an explicit Pending state so the front never shows a fabricated verdict.
func (g *Gateway) Judgment(ctx context.Context, jiraKey string) (domain.JudgmentReview, error) {
	pending := domain.JudgmentReview{
		JiraKey:  jiraKey,
		Gate:     "HG5",
		Judges:   []domain.Judge{},
		FixAgent: "idle",
		Verdict:  "pending",
		Pending:  true,
	}
	rev, err := g.reader.RunsJudgment(ctx, jiraKey)
	if err != nil || rev.Pending {
		return pending, nil
	}
	return rev, nil
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
