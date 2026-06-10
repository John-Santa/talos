// Package port defines the outbound ports for the orchestration module.
// All I/O is behind these interfaces; adapters implement them.
package port

import (
	"context"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
)

// WorktreeManager manages agent worktrees via the wt CLI.
type WorktreeManager interface {
	// Ensure creates the worktree if it does not exist, or reuses it if it does.
	// Returns the absolute path to the worktree.
	Ensure(ctx context.Context, figura, jiraKey string) (path string, err error)

	// Teardown removes the worktree for the given figura. If force is true,
	// the worktree is removed even when it has uncommitted changes.
	Teardown(ctx context.Context, figura string, force bool) error
}

// OverlapChecker checks for module/agent overlap via the ov CLI.
type OverlapChecker interface {
	// Check runs ov check for the given module and agent and returns the verdict.
	Check(ctx context.Context, module, agent string) (dispatch.OverlapVerdict, error)
}

// EvidenceArgs carries optional flags forwarded to the evidence run-loop.
type EvidenceArgs struct {
	Summary        string
	PRUrl          string
	AttachPath     string
	WorklogSeconds int
	DryRun         bool
}

// EvidenceRunner runs the evidence loop for a phase via the evidence CLI.
type EvidenceRunner interface {
	// RunPhase shells out to `evidence run-loop --phase=<phase> ...`.
	// Returns the Jira issue key that was created or updated (may be empty on dry-run).
	RunPhase(ctx context.Context, item dispatch.WorkItem, phase string, args EvidenceArgs) (issueKey string, err error)
}

// MergePlan is the result of a merge plan check.
type MergePlan struct {
	ConflictRate float64
	Threshold    float64
	Clean        bool
}

// MergeCoordinator manages merge order via the mo CLI.
type MergeCoordinator interface {
	// Plan runs `mo plan --json` and returns the conflict rate and threshold.
	Plan(ctx context.Context) (MergePlan, error)

	// Execute runs `mo execute --yes` to integrate branches in order.
	// Only called when --confirm-merge is explicitly passed.
	Execute(ctx context.Context) error
}

// RollbackCoordinator executes the CONSTITUTION §11 recipe on failure.
type RollbackCoordinator interface {
	// Recipe11 performs: (1) evidence transition → To Do,
	// (2) evidence comment with reason, (3) wt teardown --force <figura>.
	Recipe11(ctx context.Context, jiraKey, figura, reason string) error
}
