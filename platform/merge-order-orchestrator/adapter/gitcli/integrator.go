package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// Integrator implements port.GitIntegrator via os/exec.
// The repoRoot is used as the working directory for all git commands,
// matching the same convention as Inspector and wt's GitRunner (cmd.Dir = repoRoot).
type Integrator struct {
	repoRoot string
}

// NewIntegrator constructs an Integrator for the given repository root path.
func NewIntegrator(repoRoot string) *Integrator {
	return &Integrator{repoRoot: repoRoot}
}

var _ port.GitIntegrator = (*Integrator)(nil)

// RebaseOnto rebases branch onto base by running git rebase in the repoRoot directory.
// On conflict it aborts the rebase, returns the conflicting paths, and returns *mergeorder.ErrRebaseConflict.
func (r *Integrator) RebaseOnto(ctx context.Context, branch, base string) ([]string, error) {
	// checkout branch first
	if err := r.runIn(ctx, r.repoRoot, "checkout", branch); err != nil {
		return nil, fmt.Errorf("checkout %s: %w", branch, err)
	}

	// attempt rebase
	rebaseErr := r.runIn(ctx, r.repoRoot, "rebase", base)
	if rebaseErr == nil {
		return nil, nil
	}

	// collect conflicting files
	conflicts := r.conflictingFiles(ctx)

	// abort the rebase to leave repo in clean state
	_ = r.runIn(ctx, r.repoRoot, "rebase", "--abort")

	return conflicts, &mergeorder.ErrRebaseConflict{
		Branch: branch,
		Files:  conflicts,
	}
}

func (r *Integrator) runIn(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}

func (r *Integrator) conflictingFiles(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = r.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files
}
