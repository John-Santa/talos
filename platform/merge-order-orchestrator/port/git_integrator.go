package port

import "context"

// GitIntegrator is the write outbound port for git integration operations.
//
// By design this interface exposes exactly ONE method: RebaseOnto.
// There is no Merge, Push, or OpenPR method — this absence is the structural
// enforcement of gate HG3: mo never auto-merges to develop (ADR-M1).
type GitIntegrator interface {
	// RebaseOnto rebases branch onto base in the branch's worktree directory.
	// Returns the conflicting file paths and a non-nil error on conflict.
	RebaseOnto(ctx context.Context, branch, base string) (conflicts []string, err error)
}
