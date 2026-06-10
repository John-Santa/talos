// Package wtcli implements the WorktreeManager port by shelling out to the wt binary.
package wtcli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// Manager implements port.WorktreeManager.
type Manager struct {
	binary string
	runner runner.Runner
}

var _ port.WorktreeManager = (*Manager)(nil)

// NewManager constructs a Manager using the given wt binary name and runner.
func NewManager(wtBinary string, r runner.Runner) *Manager {
	return &Manager{binary: wtBinary, runner: r}
}

// Ensure shells out to `wt create <figura> <jiraKey>`.
// If the worktree already exists, wt is idempotent and returns a zero exit.
// The returned path is the conventional talos.wt/agent-<figura> path.
func (m *Manager) Ensure(ctx context.Context, figura, jiraKey string) (string, error) {
	_, err := m.runner.Run(ctx, m.binary, "create", figura, jiraKey)
	if err != nil {
		return "", fmt.Errorf("wtcli: wt create %s %s: %w", figura, jiraKey, err)
	}
	// Derive the deterministic path (talos.wt/agent-<figura>), matching the
	// naming convention from the worktree-orchestrator domain.
	path := filepath.Join("talos.wt", "agent-"+figura)
	return path, nil
}

// Teardown shells out to `wt teardown <figura> [--force]`.
func (m *Manager) Teardown(ctx context.Context, figura string, force bool) error {
	args := []string{"teardown", figura}
	if force {
		args = append(args, "--force")
	}
	_, err := m.runner.Run(ctx, m.binary, args...)
	if err != nil {
		return fmt.Errorf("wtcli: wt teardown %s: %w", figura, err)
	}
	return nil
}
