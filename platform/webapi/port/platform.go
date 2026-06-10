package port

import (
	"context"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// PlatformReader produces orchestration inputs from the local platform state.
// The native implementation reads them with git + the ownership file only — no
// wt/mo/ov/ch binaries, no network, no Jira.
type PlatformReader interface {
	Worktrees(ctx context.Context) ([]domain.WtEntry, error)
	MergePlan(ctx context.Context) (domain.MoPlan, error)
	Overlap(ctx context.Context) (domain.OvScan, error)
	Ownership(ctx context.Context) (map[string]string, error)
	// Ready reports whether the underlying repo/tooling is usable (for /readyz).
	Ready(ctx context.Context) error
	// Labels calls `ch labels --branch <branch> --json` (best-effort).
	// If the ch binary is absent or fails, returns an empty ChLabels without error.
	Labels(ctx context.Context, branch string) (domain.ChLabels, error)
}

// PlatformWriter performs worktree write actions (native git, no binaries).
type PlatformWriter interface {
	CreateWorktree(ctx context.Context, figura, jiraKey string) error
	TeardownWorktree(ctx context.Context, figura string) error
	Merge(ctx context.Context, figura, jiraKey string) error
}
