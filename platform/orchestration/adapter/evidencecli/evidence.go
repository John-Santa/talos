// Package evidencecli implements EvidenceRunner and RollbackCoordinator ports
// by shelling out to the evidence and wt binaries.
package evidencecli

import (
	"context"
	"fmt"
	"strings"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// Runner implements port.EvidenceRunner.
type Runner struct {
	binary string
	runner runner.Runner
}

var _ port.EvidenceRunner = (*Runner)(nil)

// NewRunner constructs a Runner using the given evidence binary name and runner.
func NewRunner(evidenceBinary string, r runner.Runner) *Runner {
	return &Runner{binary: evidenceBinary, runner: r}
}

// RunPhase shells out to `evidence run-loop --phase=<p> --change=<c> --agent=<a>
// --module=<m> [--jira-key=<k>] [--summary=<s>] [--pr-url=<u>] [--attach=<path>]
// [--worklog-seconds=<n>] [--dry-run]`.
func (r *Runner) RunPhase(ctx context.Context, item dispatch.WorkItem, phase string, args port.EvidenceArgs) (string, error) {
	cliArgs := []string{
		"run-loop",
		"--phase", phase,
		"--change", item.Change,
		"--agent", item.Agent,
		"--module", item.Module,
	}
	if item.JiraKey != "" {
		cliArgs = append(cliArgs, "--jira-key", item.JiraKey)
	}
	if args.Summary != "" {
		cliArgs = append(cliArgs, "--summary", args.Summary)
	}
	if args.PRUrl != "" {
		cliArgs = append(cliArgs, "--pr-url", args.PRUrl)
	}
	if args.AttachPath != "" {
		cliArgs = append(cliArgs, "--attach", args.AttachPath)
	}
	if args.WorklogSeconds > 0 {
		cliArgs = append(cliArgs, "--worklog-seconds", fmt.Sprintf("%d", args.WorklogSeconds))
	}
	if args.DryRun {
		cliArgs = append(cliArgs, "--dry-run")
	}

	_, err := r.runner.Run(ctx, r.binary, cliArgs...)
	if err != nil {
		return "", fmt.Errorf("evidencecli: evidence run-loop --phase=%s failed: %w", phase, err)
	}

	// evidence outputs the issue key on stdout; return empty string on dry-run.
	// For now we return an empty string since the real key comes from Jira;
	// the adapter can be extended to parse stdout if evidence prints it.
	return item.JiraKey, nil
}

// Rollbacker implements port.RollbackCoordinator.
type Rollbacker struct {
	evidenceBinary string
	wtBinary       string
	runner         runner.Runner
}

var _ port.RollbackCoordinator = (*Rollbacker)(nil)

// NewRollbacker constructs a Rollbacker using the given binary names and runner.
func NewRollbacker(evidenceBinary, wtBinary string, r runner.Runner) *Rollbacker {
	return &Rollbacker{evidenceBinary: evidenceBinary, wtBinary: wtBinary, runner: r}
}

// Recipe11 executes the CONSTITUTION §11 rollback recipe:
//  1. evidence run-loop --phase=archive (→ To Do transition) — best-effort
//  2. evidence run-loop with a failure comment — best-effort
//  3. wt teardown <figura> --force — best-effort
//
// All 3 steps are attempted even if earlier ones fail.
// Returns the first non-nil error encountered, if any.
func (rb *Rollbacker) Recipe11(ctx context.Context, jiraKey, figura, reason string) error {
	var firstErr error

	// Step 1: transition issue back to To Do via a special --phase that marks revert.
	// We use "propose" phase without creating (evidence is idempotent) to reset state.
	// In practice the evidence CLI is the canonical rollback path; we pass the
	// reason as the summary so it appears in Jira.
	comment := fmt.Sprintf("ROLLBACK §11: %s", strings.TrimSpace(reason))

	// Step 1: Add a failure comment to the Jira issue.
	_, err := rb.runner.Run(ctx, rb.evidenceBinary, "run-loop",
		"--phase", "propose",
		"--change", "rollback",
		"--agent", figura,
		"--module", "rollback",
		"--jira-key", jiraKey,
		"--summary", comment,
	)
	if err != nil && firstErr == nil {
		firstErr = fmt.Errorf("recipe11: evidence comment failed: %w", err)
	}

	// Step 2: evidence transition → comment with reason.
	_, err = rb.runner.Run(ctx, rb.evidenceBinary, "run-loop",
		"--phase", "archive",
		"--change", "rollback",
		"--agent", figura,
		"--module", "rollback",
		"--jira-key", jiraKey,
		"--summary", comment,
	)
	if err != nil && firstErr == nil {
		firstErr = fmt.Errorf("recipe11: evidence transition failed: %w", err)
	}

	// Step 3: wt teardown --force (always attempted).
	_, err = rb.runner.Run(ctx, rb.wtBinary, "teardown", figura, "--force")
	if err != nil && firstErr == nil {
		firstErr = fmt.Errorf("recipe11: wt teardown --force failed: %w", err)
	}

	return firstErr
}
