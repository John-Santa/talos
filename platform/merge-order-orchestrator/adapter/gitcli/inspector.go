// Package gitcli provides os/exec-backed adapters for the merge-order-orchestrator ports.
package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// Inspector implements port.GitInspector via os/exec against a real git installation.
type Inspector struct {
	repoRoot string
}

// NewInspector constructs an Inspector for the given repository root path.
func NewInspector(repoRoot string) *Inspector {
	return &Inspector{repoRoot: repoRoot}
}

var _ port.GitInspector = (*Inspector)(nil)

func (r *Inspector) run(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.repoRoot

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// Fetch fetches from origin.
func (r *Inspector) Fetch(ctx context.Context) error {
	_, errOut, err := r.run(ctx, "fetch", "origin")
	if err != nil {
		return fmt.Errorf("git fetch origin: %w (stderr: %s)", err, strings.TrimSpace(errOut))
	}
	return nil
}

// RevParse resolves a ref to its full commit SHA.
func (r *Inspector) RevParse(ctx context.Context, ref string) (string, error) {
	out, errOut, err := r.run(ctx, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w (stderr: %s)", ref, err, strings.TrimSpace(errOut))
	}
	return strings.TrimSpace(out), nil
}

// MergeBase returns the best common ancestor commit between a and b.
func (r *Inspector) MergeBase(ctx context.Context, a, b string) (string, error) {
	out, errOut, err := r.run(ctx, "merge-base", a, b)
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w (stderr: %s)", a, b, err, strings.TrimSpace(errOut))
	}
	return strings.TrimSpace(out), nil
}

// CommitsAhead returns the number of commits branch has ahead of base.
func (r *Inspector) CommitsAhead(ctx context.Context, base, branch string) (int, error) {
	out, errOut, err := r.run(ctx, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count %s..%s: %w (stderr: %s)", base, branch, err, strings.TrimSpace(errOut))
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("parsing rev-list count %q: %w", strings.TrimSpace(out), err)
	}
	return n, nil
}

// MergeTreeConflicts simulates a merge of branch onto base using git merge-tree.
// Exit 0 → clean (no conflicts). Exit 1 → conflict paths from stdout. Other exit → error.
func (r *Inspector) MergeTreeConflicts(ctx context.Context, base, branch string) (conflicts []string, clean bool, err error) {
	cmd := exec.CommandContext(ctx, "git", "merge-tree", "--write-tree", "--name-only", base, branch)
	cmd.Dir = r.repoRoot

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			return nil, false, fmt.Errorf("git merge-tree %s %s: %w", base, branch, runErr)
		}
		exitCode = exitErr.ExitCode()
		if exitCode != 1 {
			return nil, false, fmt.Errorf("git merge-tree %s %s: exit %d (stderr: %s)", base, branch, exitCode, strings.TrimSpace(errBuf.String()))
		}
	}

	if exitCode == 0 {
		return nil, true, nil
	}

	// exit 1: parse conflicting file paths from stdout
	var paths []string
	for _, line := range strings.Split(strings.TrimRight(outBuf.String(), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, false, nil
}

// ChangedFiles returns the files changed between base and branch (three-dot diff).
func (r *Inspector) ChangedFiles(ctx context.Context, base, branch string) ([]string, error) {
	out, errOut, err := r.run(ctx, "diff", "--name-only", base+"..."+branch)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s...%s: %w (stderr: %s)", base, branch, err, strings.TrimSpace(errOut))
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// BranchCreatedAt returns the committer timestamp of the latest commit on branch,
// satisfying port.GitInspector for FIFO candidate ordering.
func (r *Inspector) BranchCreatedAt(ctx context.Context, branch string) (time.Time, error) {
	out, errOut, err := r.run(ctx, "log", "-1", "--format=%cI", branch)
	if err != nil {
		return time.Time{}, fmt.Errorf("git log -1 --format=%%cI %s: %w (stderr: %s)", branch, err, strings.TrimSpace(errOut))
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return time.Time{}, nil
	}
	ts, parseErr := time.Parse(time.RFC3339, raw)
	if parseErr != nil {
		// try with offset format like 2006-01-02T15:04:05+07:00 — RFC3339 covers it
		return time.Time{}, fmt.Errorf("parsing branch timestamp %q: %w", raw, parseErr)
	}
	return ts, nil
}
