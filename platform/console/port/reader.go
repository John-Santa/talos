// Package port defines the inbound/outbound interfaces for the console module.
package port

import (
	"context"

	"github.com/John-Santa/talos/platform/console/domain/platform"
)

// PlatformReader is the primary read port for the console module.
// It provides access to the current state of the TALOS platform.
//
// Future methods that will extend this interface in later slices:
//   - EvidenceState(ctx context.Context) ([]platform.EvidenceState, error)
//   - Ownership(ctx context.Context) ([]platform.Ownership, error)
//   - SDDChanges(ctx context.Context) ([]platform.SDDChange, error)
type PlatformReader interface {
	// Worktrees returns the list of active agent worktrees.
	Worktrees(ctx context.Context) ([]platform.Worktree, error)

	// MergePlan runs `mo plan --json` and returns the decoded merge plan.
	MergePlan(ctx context.Context) (platform.MergePlan, error)

	// Overlap runs `ov scan --json` and returns the decoded overlap report.
	Overlap(ctx context.Context) (platform.Overlap, error)

	// Labels runs `ch labels --branch <branch> --json` and returns the decoded label state.
	Labels(ctx context.Context, branch string) (platform.Labels, error)
}
