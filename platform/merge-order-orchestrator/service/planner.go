package service

import (
	"context"
	"fmt"
	"time"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// Planner computes merge plans and single-branch checks using read-only ports.
//
// Planner holds ONLY GitInspector and WorktreeLister (ADR-M1).
// It must never hold a GitIntegrator — this is the compile-time enforcement
// that plan is side-effect-free and mo cannot merge to develop.
type Planner struct {
	inspector port.GitInspector
	lister    port.WorktreeLister
	cfg       Config
}

// NewPlanner constructs a Planner with the given read ports and configuration.
func NewPlanner(inspector port.GitInspector, lister port.WorktreeLister, cfg Config) *Planner {
	return &Planner{
		inspector: inspector,
		lister:    lister,
		cfg:       cfg,
	}
}

// Plan computes the full merge plan: list → filter → fetch → snapshot → per-branch ahead/files → order → predict conflicts.
func (p *Planner) Plan(ctx context.Context, deps map[string][]string) (mergeorder.PlanReport, error) {
	entries, err := p.lister.List(ctx)
	if err != nil {
		return mergeorder.PlanReport{}, fmt.Errorf("listing worktrees: %w", err)
	}

	if !p.cfg.NoFetch {
		if err := p.inspector.Fetch(ctx); err != nil {
			return mergeorder.PlanReport{}, fmt.Errorf("fetching origin: %w", err)
		}
	}

	baseTip, err := p.inspector.RevParse(ctx, p.cfg.BaseBranch)
	if err != nil {
		return mergeorder.PlanReport{}, fmt.Errorf("resolving %s tip: %w", p.cfg.BaseBranch, &mergeorder.ErrDevelopNotAvailable{})
	}

	candidates, err := p.buildCandidates(ctx, entries, baseTip)
	if err != nil {
		return mergeorder.PlanReport{}, err
	}

	ordered, err := mergeorder.Order(candidates, deps)
	if err != nil {
		return mergeorder.PlanReport{}, err
	}

	steps, err := p.predictConflicts(ctx, baseTip, ordered)
	if err != nil {
		return mergeorder.PlanReport{}, err
	}

	plan := mergeorder.MergePlan{
		BaseBranch: p.cfg.BaseBranch,
		BaseTip:    baseTip,
		Steps:      steps,
	}
	return mergeorder.NewPlanReport(plan, p.cfg.MaxConflictRate), nil
}

// Check performs a single-branch read-only conflict simulation against the base tip.
func (p *Planner) Check(ctx context.Context, branch string) (mergeorder.Step, error) {
	if !p.cfg.NoFetch {
		if err := p.inspector.Fetch(ctx); err != nil {
			return mergeorder.Step{}, fmt.Errorf("fetching origin: %w", err)
		}
	}

	baseTip, err := p.inspector.RevParse(ctx, p.cfg.BaseBranch)
	if err != nil {
		return mergeorder.Step{}, fmt.Errorf("resolving base branch: %w", err)
	}

	_, err = p.inspector.RevParse(ctx, branch)
	if err != nil {
		return mergeorder.Step{}, &mergeorder.ErrBranchNotFound{Branch: branch}
	}

	conflicts, clean, err := p.inspector.MergeTreeConflicts(ctx, baseTip, branch)
	if err != nil {
		return mergeorder.Step{}, fmt.Errorf("checking conflicts for %s: %w", branch, err)
	}

	return mergeorder.Step{
		Position:       1,
		Candidate:      mergeorder.Candidate{Branch: branch},
		PredictedClean: clean,
		ConflictFiles:  conflicts,
	}, nil
}

func (p *Planner) buildCandidates(ctx context.Context, entries []port.WorktreeEntry, baseTip string) ([]mergeorder.Candidate, error) {
	var candidates []mergeorder.Candidate
	for _, e := range entries {
		if e.Status != "active" {
			continue
		}
		ahead, err := p.inspector.CommitsAhead(ctx, baseTip, e.Branch)
		if err != nil {
			return nil, fmt.Errorf("checking commits ahead for %s: %w", e.Branch, err)
		}
		if ahead == 0 {
			continue
		}
		files, err := p.inspector.ChangedFiles(ctx, baseTip, e.Branch)
		if err != nil {
			return nil, fmt.Errorf("getting changed files for %s: %w", e.Branch, err)
		}
		var createdAt time.Time
		candidates = append(candidates, mergeorder.Candidate{
			Figura:       e.Figura,
			Branch:       e.Branch,
			Head:         e.Head,
			Path:         e.Path,
			CommitsAhead: ahead,
			ChangedFiles: files,
			CreatedAt:    createdAt,
		})
	}
	return candidates, nil
}

func (p *Planner) predictConflicts(ctx context.Context, baseTip string, ordered []mergeorder.Candidate) ([]mergeorder.Step, error) {
	steps := make([]mergeorder.Step, 0, len(ordered))
	for i, c := range ordered {
		conflicts, clean, err := p.inspector.MergeTreeConflicts(ctx, baseTip, c.Branch)
		if err != nil {
			return nil, fmt.Errorf("predicting conflicts for %s: %w", c.Branch, err)
		}
		steps = append(steps, mergeorder.Step{
			Position:       i + 1,
			Candidate:      c,
			PredictedClean: clean,
			ConflictFiles:  conflicts,
		})
	}
	return steps, nil
}
