// Package service contains the worktree orchestrator use case.
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

// WorktreeStatus pairs a parsed worktree entry with its computed status (active, orphan, or stale).
type WorktreeStatus struct {
	Info   worktree.WorktreeInfo
	Status string
}

// Config holds the runtime configuration for the orchestrator.
type Config struct {
	// RepoRoot is the absolute path to the git repository root.
	RepoRoot string
	// WorktreeBase is the directory where agent worktrees are placed; defaults to "talos.wt".
	WorktreeBase string
	// BaseBranch is the branch that new agent branches are cut from; defaults to "develop".
	BaseBranch string
	// Project is the Jira project key used to validate issue keys; defaults to "TAL".
	Project string
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults.
func DefaultTALConfig() Config {
	return Config{
		WorktreeBase: "talos.wt",
		BaseBranch:   "develop",
		Project:      "TAL",
	}
}

// Orchestrator is the primary use-case service for managing agent worktrees.
type Orchestrator struct {
	runner port.GitRunner
	cfg    Config

	// WriteFile writes the .env file; defaults to os.WriteFile. Tests inject a fake to avoid disk I/O.
	WriteFile func(path string, data []byte, perm fs.FileMode) error
}

// NewOrchestrator constructs an Orchestrator with WriteFile defaulted to os.WriteFile.
func NewOrchestrator(runner port.GitRunner, cfg Config) *Orchestrator {
	return &Orchestrator{
		runner:    runner,
		cfg:       cfg,
		WriteFile: os.WriteFile,
	}
}

// Create creates a new agent worktree for the given figura and Jira key.
func (o *Orchestrator) Create(ctx context.Context, figura, jiraKey string, noFetch bool) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase, o.cfg.Project)
	if err != nil {
		return err
	}

	exists, err := o.runner.BranchExists(ctx, spec.Branch)
	if err != nil {
		return fmt.Errorf("checking branch existence: %w", err)
	}
	if exists {
		return &worktree.ErrBranchExists{Branch: spec.Branch}
	}

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

	if !noFetch {
		if err := o.runner.Fetch(ctx); err != nil {
			return fmt.Errorf("fetching origin: %w", err)
		}
	}

	if err := o.runner.WorktreeAdd(ctx, spec.Path, spec.Branch, o.cfg.BaseBranch); err != nil {
		return fmt.Errorf("adding worktree: %w", err)
	}

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

// List returns all agent worktrees with their computed status.
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

func (o *Orchestrator) classifyWorktree(ctx context.Context, info worktree.WorktreeInfo) (string, error) {
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		return "stale", nil
	}
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
func (o *Orchestrator) Teardown(ctx context.Context, figura, jiraKey string, force, deleteBranch bool) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase, o.cfg.Project)
	if err != nil {
		return err
	}

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

	if err := o.runner.Prune(ctx); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}

	if deleteBranch {
		if err := o.runner.BranchDelete(ctx, spec.Branch); err != nil {
			return fmt.Errorf("deleting branch %q: %w", spec.Branch, err)
		}
	}

	return nil
}

// Env re-derives and writes the .env file for an existing agent worktree, returning ErrWorktreeNotFound if absent.
func (o *Orchestrator) Env(ctx context.Context, figura, jiraKey string) error {
	spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase, o.cfg.Project)
	if err != nil {
		return err
	}

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
