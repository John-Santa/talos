// Package service contains the Dispatcher use case for the orchestration module.
package service

import (
	"context"
	"fmt"
	"log"

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
// RunRecorder is optional (nil = no-op): all recorder calls are fail-soft.
type Dispatcher struct {
	wt  port.WorktreeManager
	ov  port.OverlapChecker
	ev  port.EvidenceRunner
	mo  port.MergeCoordinator
	rb  port.RollbackCoordinator
	rec port.RunRecorder
	cfg Config
}

// NewDispatcher constructs a Dispatcher with the given ports and config.
// rec may be nil; the Dispatcher treats nil as a no-op recorder.
func NewDispatcher(
	wt port.WorktreeManager,
	ov port.OverlapChecker,
	ev port.EvidenceRunner,
	mo port.MergeCoordinator,
	rb port.RollbackCoordinator,
	rec port.RunRecorder,
	cfg Config,
) *Dispatcher {
	return &Dispatcher{wt: wt, ov: ov, ev: ev, mo: mo, rb: rb, rec: rec, cfg: cfg}
}

// record* helpers are fail-soft wrappers: they log on error but never propagate.

func (d *Dispatcher) recordDispatch(ctx context.Context, item dispatch.WorkItem, phase, status string) {
	if d.rec == nil {
		return
	}
	if err := d.rec.RecordDispatch(ctx, item, phase, status); err != nil {
		log.Printf("orchestration: RunRecorder.RecordDispatch: %v", err)
	}
}

func (d *Dispatcher) recordActivity(ctx context.Context, jiraKey, agent, text string) {
	if d.rec == nil {
		return
	}
	if err := d.rec.RecordActivity(ctx, jiraKey, agent, text); err != nil {
		log.Printf("orchestration: RunRecorder.RecordActivity: %v", err)
	}
}

func (d *Dispatcher) recordMetric(ctx context.Context, jiraKey, name string, value float64) {
	if d.rec == nil {
		return
	}
	if err := d.rec.RecordMetric(ctx, jiraKey, name, value); err != nil {
		log.Printf("orchestration: RunRecorder.RecordMetric: %v", err)
	}
}

// Dispatch executes the full dispatch sequence for the given WorkItem and Phase.
//
// Sequence (per phase plan):
//  1. PlanFor(phase) — fail-loud on unknown phase
//  2. RecordDispatch("running") + RecordActivity("dispatch <phase> iniciado") — fail-soft
//  3. If OverlapGate: ov.Check → BLOCK→ErrOverlapBlock, SERIALIZE→ErrQueuedSerialize, OK→continue
//  4. If EnsureWorktree: wt.Ensure (idempotent) + RecordActivity("worktree ready") — fail-soft
//  5. ev.RunPhase — on error: rb.Recipe11 + RecordActivity(rollback) + RecordDispatch("failed") + return error
//  6. RecordActivity("evidence ok") — fail-soft
//  7. If MergeGate: mo.Plan → RecordMetric("conflict_rate") — fail-soft
//     If conflict above threshold → RecordDispatch("failed") + rb.Recipe11 + ErrMergeConflict
//     If ConfirmMerge && clean: mo.Execute
//  8. If Cleanup: wt.Teardown (after Done)
//  9. RecordDispatch("done") — fail-soft
func (d *Dispatcher) Dispatch(ctx context.Context, item dispatch.WorkItem, phase dispatch.Phase, evArgs port.EvidenceArgs) (DispatchResult, error) {
	plan, err := dispatch.PlanFor(phase)
	if err != nil {
		return DispatchResult{}, err
	}

	phaseStr := string(phase)

	// Step 2: emit start events (fail-soft)
	d.recordDispatch(ctx, item, phaseStr, "running")
	d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("dispatch %s iniciado", phaseStr))

	// Step 3: overlap gate
	if plan.OverlapGate {
		verdict, err := d.ov.Check(ctx, item.Module, item.Agent)
		if err != nil {
			d.recordDispatch(ctx, item, phaseStr, "failed")
			return DispatchResult{}, fmt.Errorf("orchestration: overlap check failed: %w", err)
		}
		action := dispatch.DecideOnOverlap(verdict)
		switch action {
		case dispatch.ActionAbort:
			d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("BLOCK: overlap detected for module=%s agent=%s", item.Module, item.Agent))
			d.recordDispatch(ctx, item, phaseStr, "failed")
			return DispatchResult{}, &dispatch.ErrOverlapBlock{Module: item.Module, Agent: item.Agent}
		case dispatch.ActionQueue:
			d.recordDispatch(ctx, item, phaseStr, "failed")
			return DispatchResult{}, &dispatch.ErrQueuedSerialize{Module: item.Module, Agent: item.Agent}
		}
		// ActionProceed: fall through
	}

	// Step 4: ensure worktree
	var wtPath string
	if plan.EnsureWorktree {
		wtPath, err = d.wt.Ensure(ctx, item.Agent, item.JiraKey)
		if err != nil {
			d.recordDispatch(ctx, item, phaseStr, "failed")
			return DispatchResult{}, fmt.Errorf("orchestration: worktree ensure failed: %w", err)
		}
		d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("worktree %s listo", wtPath))
	}

	// Step 5: evidence run-loop
	issueKey, err := d.ev.RunPhase(ctx, item, phaseStr, evArgs)
	if err != nil {
		// fail-loud: trigger Recipe §11 then propagate
		d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("ROLLBACK §11: %s", err.Error()))
		d.recordDispatch(ctx, item, phaseStr, "failed")
		rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
		if rbErr != nil {
			return DispatchResult{}, fmt.Errorf("orchestration: evidence failed (%v); Recipe11 also failed: %w", err, rbErr)
		}
		return DispatchResult{}, fmt.Errorf("orchestration: evidence run-loop failed (Recipe11 executed): %w", err)
	}
	d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("evidence run-loop --phase=%s ok", phaseStr))

	// Step 6: merge gate (verify/archive)
	if plan.MergeGate {
		mergePlan, err := d.mo.Plan(ctx)
		if err != nil {
			d.recordDispatch(ctx, item, phaseStr, "failed")
			rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
			if rbErr != nil {
				return DispatchResult{}, fmt.Errorf("orchestration: mo plan failed (%v); Recipe11 also failed: %w", err, rbErr)
			}
			return DispatchResult{}, fmt.Errorf("orchestration: mo plan failed (Recipe11 executed): %w", err)
		}

		// Record conflict_rate metric (fail-soft)
		d.recordMetric(ctx, item.JiraKey, "conflict_rate", mergePlan.ConflictRate)

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
			d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("merge conflict: conflict_rate=%.2f threshold=%.2f", mergePlan.ConflictRate, threshold))
			d.recordDispatch(ctx, item, phaseStr, "failed")
			rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, conflictErr.Error())
			if rbErr != nil {
				return DispatchResult{}, fmt.Errorf("orchestration: merge conflict + Recipe11 failed: %w", rbErr)
			}
			return DispatchResult{}, conflictErr
		}

		// If --confirm-merge was explicitly passed, execute merge
		if d.cfg.ConfirmMerge {
			if err := d.mo.Execute(ctx); err != nil {
				d.recordActivity(ctx, item.JiraKey, item.Agent, fmt.Sprintf("ROLLBACK §11: mo execute failed: %s", err.Error()))
				d.recordDispatch(ctx, item, phaseStr, "failed")
				rbErr := d.rb.Recipe11(ctx, item.JiraKey, item.Agent, err.Error())
				if rbErr != nil {
					return DispatchResult{}, fmt.Errorf("orchestration: mo execute failed (%v); Recipe11 also failed: %w", err, rbErr)
				}
				return DispatchResult{}, fmt.Errorf("orchestration: mo execute failed (Recipe11 executed): %w", err)
			}
			d.recordActivity(ctx, item.JiraKey, item.Agent, "merge executed ok")
		}
	}

	// Step 7: cleanup (archive only)
	if plan.Cleanup {
		if err := d.wt.Teardown(ctx, item.Agent, false); err != nil {
			d.recordDispatch(ctx, item, phaseStr, "failed")
			return DispatchResult{}, fmt.Errorf("orchestration: worktree teardown failed: %w", err)
		}
	}

	// Final: emit done (fail-soft)
	d.recordDispatch(ctx, item, phaseStr, "done")

	return DispatchResult{IssueKey: issueKey, WorktreePath: wtPath}, nil
}
