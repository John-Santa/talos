package port

import "context"

// PlatformActor is the write port for the console module.
// It provides mutating operations on the TALOS platform.
//
// Actions to be added in later slices:
//   - CreateWorktree(ctx context.Context, figura, jiraKey string) error
//   - ExecuteMerge(ctx context.Context, figura, jiraKey string) error
//   - TransitionIssue(ctx context.Context, jiraKey, transition string) error
//   - RunEvidence(ctx context.Context, jiraKey string) error
type PlatformActor interface {
	// TeardownWorktree tears down the worktree for the given agent figura and
	// Jira issue key by invoking `wt teardown <figura> <jiraKey>`.
	// NOTE: wt teardown has a known bug (TAL-10) and may fail in live usage;
	// the TUI surfaces any error as a toast notification.
	TeardownWorktree(ctx context.Context, figura, jiraKey string) error
}
