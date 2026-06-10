// Package service contains the Dispatcher use case for the orchestration module.
package service

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// Config holds the runtime configuration for the Dispatcher.
type Config struct {
	// ConfirmMerge controls whether mo execute is called after a clean merge gate.
	// Default false: mo execute never runs without explicit --confirm-merge.
	ConfirmMerge bool
	// MergeConflictThreshold overrides the threshold from mo plan when set (0 = use mo plan's own value).
	MergeConflictThreshold float64
}

// DefaultConfig returns a safe Config (no confirm-merge, use mo plan threshold).
func DefaultConfig() Config {
	return Config{
		ConfirmMerge:           false,
		MergeConflictThreshold: 0,
	}
}

// DispatchResult carries the result of a successful dispatch.
type DispatchResult struct {
	// IssueKey is the Jira key that was created or updated.
	IssueKey string
	// WorktreePath is the absolute path to the agent's worktree.
	WorktreePath string
}

// Dispatcher is the primary use-case service. It composes the 5 ports to
// implement the dispatch sequence: overlap check → worktree ensure → evidence
// run-loop → merge gate (if applicable) → cleanup (if applicable).
type Dispatcher struct {
	wt  port.WorktreeManager
	ov  port.OverlapChecker
	ev  port.EvidenceRunner
	mo  port.MergeCoordinator
	rb  port.RollbackCoordinator
	cfg Config
}

// NewDispatcher constructs a Dispatcher with the given ports and config.
func NewDispatcher(
	wt port.WorktreeManager,
	ov port.OverlapChecker,
	ev port.EvidenceRunner,
	mo port.MergeCoordinator,
	rb port.RollbackCoordinator,
	cfg Config,
) *Dispatcher {
	return &Dispatcher{wt: wt, ov: ov, ev: ev, mo: mo, rb: rb, cfg: cfg}
}

// Dispatch executes the full dispatch sequence for the given WorkItem and Phase.
//
// Sequence (per phase plan):
//  1. PlanFor(phase) — fail-loud on unknown phase
//  2. If OverlapGate: ov.Check → BLOCK→ErrOverlapBlock, SERIALIZE→ErrQueuedSerialize, OK→continue
//  3. If EnsureWorktree: wt.Ensure (idempotent)
//  4. ev.RunPhase — on error: rb.Recipe11 + return error (fail-loud)
//  5. If MergeGate: mo.Plan → conflict above threshold → rb.Recipe11 + ErrMergeConflict
//     If ConfirmMerge && clean: mo.Execute
//  6. If Cleanup: wt.Teardown (after Done)
func (d *Dispatcher) Dispatch(ctx context.Context, item dispatch.WorkItem, phase dispatch.Phase, evArgs port.EvidenceArgs) (DispatchResult, error) {
	plan, err := dispatch.PlanFor(phase)
	if err != nil {
		return DispatchResult{}, err
	}

	// Step 2: overlap gate
	if plan.OverlapGate {
		verdict, err := d.ov.Check(ctx, item.Module, item.Agent)
		if err != nil {
			return DispatchResult{}, fmt.Errorf("orchestration: overlap check failed: %w", err)
		}
		action := dispatch.DecideOnOverlap(verdict)
		switch action {
		case dispatch.ActionAbort:
			return DispatchResult{}, &dispatch.ErrOverlapBlock{Module: item.Module, Agent: item.Agent}
		case dispatch.ActionQueue:
			return DispatchResult{}, &dispatch.ErrQueuedSerialize{Module: item.Module, Agent: item.Agent}
		}
		// ActionProceed: fall through
	}

	// Step 3: ensure worktree
	var wtPath string
	if plan.EnsureWorktree {
		wtPath, err = d.wt.Ensure(ctx, item.Agent, item.JiraKey)
		if err != nil {
			return DispatchResult{}, fmt.Errorf("orchestration: worktree ensure failed: %w", err)
		}
	}

	// Step 4: evidence run-loop
	issueKey, err := d.ev.RunPhase(ctx, item, string(phase), evArgs)
	if err != nil {
		// fail-loud: trigger Recipe §11 then propagate
		rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
		if rbErr != nil {
			return DispatchResult{}, fmt.Errorf("orchestration: evidence failed (%v); Recipe11 also failed: %w", err, rbErr)
		}
		return DispatchResult{}, fmt.Errorf("orchestration: evidence run-loop failed (Recipe11 executed): %w", err)
	}

	// Step 5: merge gate (verify/archive)
	if plan.MergeGate {
		mergePlan, err := d.mo.Plan(ctx)
		if err != nil {
			rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
			if rbErr != nil {
				return DispatchResult{}, fmt.Errorf("orchestration: mo plan failed (%v); Recipe11 also failed: %w", err, rbErr)
			}
			return DispatchResult{}, fmt.Errorf("orchestration: mo plan failed (Recipe11 executed): %w", err)
		}

		// Determine effective threshold
		threshold := mergePlan.Threshold
		if d.cfg.MergeConflictThreshold > 0 {
			threshold = d.cfg.MergeConflictThreshold
		}

		if !mergePlan.Clean || mergePlan.ConflictRate > threshold {
			conflictErr := &dispatch.ErrMergeConflict{
				ConflictRate: mergePlan.ConflictRate,
				Threshold:    threshold,
			}
			rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, conflictErr.Error())
			if rbErr != nil {
				return DispatchResult{}, fmt.Errorf("orchestration: merge conflict + Recipe11 failed: %w", rbErr)
			}
			return DispatchResult{}, conflictErr
		}

		// If --confirm-merge was explicitly passed, execute merge
		if d.cfg.ConfirmMerge {
			if err := d.mo.Execute(ctx); err != nil {
				rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
				if rbErr != nil {
					return DispatchResult{}, fmt.Errorf("orchestration: mo execute failed (%v); Recipe11 also failed: %w", err, rbErr)
				}
				return DispatchResult{}, fmt.Errorf("orchestration: mo execute failed (Recipe11 executed): %w", err)
			}
		}
	}

	// Step 6: cleanup (archive only)
	if plan.Cleanup {
		if err := d.wt.Teardown(ctx, item.Agent, false); err != nil {
			return DispatchResult{}, fmt.Errorf("orchestration: worktree teardown failed: %w", err)
		}
	}

	return DispatchResult{IssueKey: issueKey, WorktreePath: wtPath}, nil
}
