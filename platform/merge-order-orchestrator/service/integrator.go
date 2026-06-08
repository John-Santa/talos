package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// ExecuteOptions holds the per-call options for IntegrationRunner.Execute.
type ExecuteOptions struct {
	// Deps is the optional branch dependency map for ordering.
	Deps map[string][]string
	// NoFetch skips the initial fetch when true.
	NoFetch bool
	// Confirmed must be true; without it Execute refuses immediately (--yes gate).
	Confirmed bool
}

// IntegrationRunner sequences branches for integration, halting at the PR boundary.
type IntegrationRunner struct {
	inspector  port.GitInspector
	integrator port.GitIntegrator
	lister     port.WorktreeLister
	cfg        Config
}

// NewIntegrationRunner constructs an IntegrationRunner with all required ports.
func NewIntegrationRunner(inspector port.GitInspector, integrator port.GitIntegrator, lister port.WorktreeLister, cfg Config) *IntegrationRunner {
	return &IntegrationRunner{
		inspector:  inspector,
		integrator: integrator,
		lister:     lister,
		cfg:        cfg,
	}
}

// Execute sequences integration: Plan → Gate HG6 → per-step Re-snapshot → Re-check → Rebase then HALT.
//
// Execute halts after AT MOST ONE rebase and NEVER merges to develop (HG3, ADR-M1).
func (r *IntegrationRunner) Execute(ctx context.Context, opts ExecuteOptions) error {
	if !opts.Confirmed {
		return errors.New("--yes is required to execute; use 'mo execute --yes' to confirm")
	}

	planCfg := r.cfg
	planCfg.NoFetch = opts.NoFetch
	planner := NewPlanner(r.inspector, r.lister, planCfg)

	report, err := planner.Plan(ctx, opts.Deps)
	if err != nil {
		return err
	}

	if report.SegmentationBad {
		return &mergeorder.ErrSegmentationBad{
			Rate:      report.ConflictRate,
			Threshold: r.cfg.MaxConflictRate,
		}
	}

	for _, step := range report.Plan.Steps {
		if !step.PredictedClean {
			return &mergeorder.ErrMergeConflict{
				Branch: step.Candidate.Branch,
				Files:  step.ConflictFiles,
			}
		}

		advancedTip, err := r.inspector.RevParse(ctx, r.cfg.BaseBranch)
		if err != nil {
			return fmt.Errorf("re-snapshot of %s: %w", r.cfg.BaseBranch, err)
		}

		ahead, err := r.inspector.CommitsAhead(ctx, advancedTip, step.Candidate.Branch)
		if err != nil {
			return fmt.Errorf("re-check commits ahead for %s: %w", step.Candidate.Branch, err)
		}
		if ahead == 0 {
			return &mergeorder.ErrBranchBehind{Branch: step.Candidate.Branch}
		}

		conflicts, clean, err := r.inspector.MergeTreeConflicts(ctx, advancedTip, step.Candidate.Branch)
		if err != nil {
			return fmt.Errorf("re-check conflicts for %s: %w", step.Candidate.Branch, err)
		}
		if !clean {
			return &mergeorder.ErrMergeConflict{
				Branch: step.Candidate.Branch,
				Files:  conflicts,
			}
		}

		if _, err := r.integrator.RebaseOnto(ctx, step.Candidate.Branch, r.cfg.BaseBranch); err != nil {
			var rc *mergeorder.ErrRebaseConflict
			if errors.As(err, &rc) {
				return &mergeorder.ErrMergeConflict{
					Branch: step.Candidate.Branch,
					Files:  rc.Files,
				}
			}
			return fmt.Errorf("rebasing %s: %w", step.Candidate.Branch, err)
		}

		// HALT after first successful rebase — print gh command hint then stop.
		fmt.Printf("\nRebase complete for %s. Open PR with:\n  gh pr create --base %s --head %s\n",
			step.Candidate.Branch, r.cfg.BaseBranch, step.Candidate.Branch)
		return nil
	}
	return nil
}
