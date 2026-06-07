// Package service contains the worktree orchestrator use case.
// It depends only on domain/worktree and port — never on adapter/gitcli.
package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/port"
)

// WorktreeStatus pairs a parsed worktree entry with its computed status.
// Status is one of: "active", "orphan", "stale" (REQ-LIST-4).
//   - active: directory exists on disk AND branch still exists in git.
//   - orphan: directory exists on disk BUT branch has been deleted (merged/pruned).
//   - stale:  directory no longer exists on disk (prune candidate).
type WorktreeStatus struct {
	Info   worktree.WorktreeInfo
	Status string
}

// Config holds the runtime configuration for the orchestrator.
type Config struct {
	// RepoRoot is the absolute path to the git repository root.
	RepoRoot string
	// WorktreeBase is the directory (relative to RepoRoot or absolute) where
	// agent worktrees are placed. Defaults to "talos.wt".
	WorktreeBase string
	// BaseBranch is the branch that new agent branches are cut from.
	// Defaults to "develop".
	BaseBranch string
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
// Callers must still supply RepoRoot before use.
func DefaultTALConfig() Config {
	return Config{
		WorktreeBase: "talos.wt",
		BaseBranch:   "develop",
	}
}

// Orchestrator is the primary use-case service for managing agent worktrees.
// It coordinates domain validation, git operations (via GitRunner), and .env
// file writing (via the injected WriteFile func).
type Orchestrator struct {
	runner port.GitRunner
	cfg    Config

	// WriteFile is the function used to write the .env file after creating a
	// worktree. It defaults to os.WriteFile (ADR-D7: wired in NewOrchestrator).
	// Tests inject a fake to avoid disk I/O.
	WriteFile func(path string, data []byte, perm fs.FileMode) error
}

// NewOrchestrator constructs an Orchestrator with the given runner and config.
// CRITICAL (ADR-D7): WriteFile is wired to os.WriteFile by default here.
// This is non-negotiable: a missing wire leaves every Create/Env call silently
// skipping the .env write, leaving worktrees half-created in production.
func NewOrchestrator(runner port.GitRunner, cfg Config) *Orchestrator {
	return &Orchestrator{
		runner:    runner,
		cfg:       cfg,
		WriteFile: os.WriteFile, // ADR-D7: default wired here
	}
}

// Create creates a new agent worktree for the given figura and Jira key.
//
// Call sequence (REQ-CREATE-1..8, design §8):
//  1. Validate figura → ErrInvalidFigure
//  2. Validate jiraKey → ErrInvalidKey
//  3. BranchExists → ErrBranchExists if true
//  4. WorktreeList → ErrWorktreeExists if path occupied
//  5. Fetch (unless noFetch=true)
//  6. WorktreeAdd
//  7. WriteFile(.env) → partial state error if it fails
func (o *Orchestrator) Create(ctx context.Context, figura, jiraKey string, noFetch bool) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase)
	if err != nil {
		return err
	}

	// Pre-check: branch collision
	exists, err := o.runner.BranchExists(ctx, spec.Branch)
	if err != nil {
		return fmt.Errorf("checking branch existence: %w", err)
	}
	if exists {
		return &worktree.ErrBranchExists{Branch: spec.Branch}
	}

	// Pre-check: worktree path collision
	stdout, err := o.runner.WorktreeList(ctx)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}
	infos, err := worktree.ParseWorktreeList(stdout)
	if err != nil {
		return fmt.Errorf("parsing worktree list: %w", err)
	}
	for _, info := range infos {
		if info.Path == spec.Path {
			return &worktree.ErrWorktreeExists{Figura: figura, Path: spec.Path}
		}
	}

	// Fetch unless --no-fetch
	if !noFetch {
		if err := o.runner.Fetch(ctx); err != nil {
			return fmt.Errorf("fetching origin: %w", err)
		}
	}

	// Create the worktree
	if err := o.runner.WorktreeAdd(ctx, spec.Path, spec.Branch, o.cfg.BaseBranch); err != nil {
		return fmt.Errorf("adding worktree: %w", err)
	}

	// Write .env — worktree without .env is half-created (ADR-D7)
	res, err := worktree.AgentResources(spec.Figura)
	if err != nil {
		return fmt.Errorf("resolving agent resources: %w", err)
	}
	envContent := worktree.RenderEnv(spec, res)
	envPath := spec.Path + "/.env"
	if err := o.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("writing .env (partial state: worktree created but .env missing at %q): %w", envPath, err)
	}

	return nil
}

// List returns all agent worktrees with their computed status (REQ-LIST-4).
//
// Status classification (per worktree entry):
//   - "stale"  — path does not exist on disk; git admin state is dangling (prune candidate).
//   - "orphan" — path exists on disk but the branch has been deleted from git (merged/pruned).
//   - "active" — path exists on disk and the branch still exists in git.
//
// BranchExists is called once per entry; the call sequence is:
// WorktreeList → (per entry) BranchExists.
func (o *Orchestrator) List(ctx context.Context) ([]WorktreeStatus, error) {
	stdout, err := o.runner.WorktreeList(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}
	infos, err := worktree.ParseWorktreeList(stdout)
	if err != nil {
		return nil, fmt.Errorf("parsing worktree list: %w", err)
	}

	statuses := make([]WorktreeStatus, 0, len(infos))
	for _, info := range infos {
		status, err := o.classifyWorktree(ctx, info)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, WorktreeStatus{Info: info, Status: status})
	}
	return statuses, nil
}

// classifyWorktree derives the status string for a single WorktreeInfo entry.
func (o *Orchestrator) classifyWorktree(ctx context.Context, info worktree.WorktreeInfo) (string, error) {
	// Stale: directory no longer exists on disk regardless of branch state.
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		return "stale", nil
	}
	// Path exists — determine active vs orphan via branch existence.
	// Detached-HEAD worktrees have no branch ref to check; treat them as orphan.
	if info.Detached || info.Branch == "" {
		return "orphan", nil
	}
	branchExists, err := o.runner.BranchExists(ctx, info.Branch)
	if err != nil {
		return "", fmt.Errorf("checking branch %q existence: %w", info.Branch, err)
	}
	if branchExists {
		return "active", nil
	}
	return "orphan", nil
}

// Teardown removes the agent worktree for the given figura and Jira key.
//
// Call sequence (design §8):
//  1. WorktreeList → ErrWorktreeNotFound if absent
//  2. WorktreeRemove(force)
//  3. Prune
//  4. BranchDelete (only if deleteBranch=true; uses safe git branch -d, ADR-D2)
func (o *Orchestrator) Teardown(ctx context.Context, figura, jiraKey string, force, deleteBranch bool) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase)
	if err != nil {
		return err
	}

	// Find the worktree
	stdout, err := o.runner.WorktreeList(ctx)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}
	infos, err := worktree.ParseWorktreeList(stdout)
	if err != nil {
		return fmt.Errorf("parsing worktree list: %w", err)
	}

	found := false
	for _, info := range infos {
		if info.Path == spec.Path {
			found = true
			break
		}
	}
	if !found {
		return &worktree.ErrWorktreeNotFound{Figura: figura, Path: spec.Path}
	}

	// Remove the worktree; enrich adapter-level dirty sentinel with figura (W-4).
	if err := o.runner.WorktreeRemove(ctx, spec.Path, force); err != nil {
		var sentinel *worktree.ErrDirtyWorktreeSentinel
		if errors.As(err, &sentinel) {
			return &worktree.ErrDirtyWorktree{
				Figura:    figura,
				Path:      sentinel.Path,
				FileCount: sentinel.FileCount,
			}
		}
		return fmt.Errorf("removing worktree: %w", err)
	}

	// Always prune after remove
	if err := o.runner.Prune(ctx); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}

	// Optionally delete the branch (safe -d only, ADR-D2)
	if deleteBranch {
		if err := o.runner.BranchDelete(ctx, spec.Branch); err != nil {
			return fmt.Errorf("deleting branch %q: %w", spec.Branch, err)
		}
	}

	return nil
}

// Env re-derives and writes the .env file for an existing agent worktree.
// Returns ErrWorktreeNotFound if the worktree does not exist.
// The .env content is byte-identical to what Create writes (ADR-D6 + REQ-ENV-3).
func (o *Orchestrator) Env(ctx context.Context, figura, jiraKey string) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase)
	if err != nil {
		return err
	}

	// Verify the worktree exists
	stdout, err := o.runner.WorktreeList(ctx)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}
	infos, err := worktree.ParseWorktreeList(stdout)
	if err != nil {
		return fmt.Errorf("parsing worktree list: %w", err)
	}

	found := false
	for _, info := range infos {
		if info.Path == spec.Path {
			found = true
			break
		}
	}
	if !found {
		return &worktree.ErrWorktreeNotFound{Figura: figura, Path: spec.Path}
	}

	// Re-derive and write .env (same pure RenderEnv as Create — byte-identical)
	res, err := worktree.AgentResources(spec.Figura)
	if err != nil {
		return fmt.Errorf("resolving agent resources: %w", err)
	}
	envContent := worktree.RenderEnv(spec, res)
	envPath := spec.Path + "/.env"
	if err := o.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		return fmt.Errorf("writing .env at %q: %w", envPath, err)
	}

	return nil
}
