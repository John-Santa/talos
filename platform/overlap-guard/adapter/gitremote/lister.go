// Package gitremote provides the adapter that lists open agent branches via
// `git ls-remote --heads origin 'agent/*'` — no worktrees, no wt binary.
// This is the CI-friendly lister used by `ov scan --remote`.
package gitremote

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// ErrLsRemoteFailed is returned when `git ls-remote` exits with a non-zero status.
type ErrLsRemoteFailed struct {
	Cause error
}

func (e *ErrLsRemoteFailed) Error() string {
	return fmt.Sprintf("git ls-remote: %v", e.Cause)
}

func (e *ErrLsRemoteFailed) Unwrap() error { return e.Cause }

// RunnerFunc is the injectable command runner: receives git sub-args (e.g. ["ls-remote", "--heads", "origin", "agent/*"]),
// returns stdout as a string. This mirrors the injected-runner pattern used by gitcli/wtcli for testability.
type RunnerFunc func(ctx context.Context, args []string) (string, error)

// Lister implements port.WorktreeLister by listing open agent/* branches from the remote.
// All discovered branches are returned with Status="active" — every branch tracked on origin
// is considered in-flight for collision purposes.
type Lister struct {
	runner RunnerFunc
}

var _ port.WorktreeLister = (*Lister)(nil)

// NewLister constructs a Lister that shells out to the real git binary.
// repoRoot is the working directory for the git invocation.
func NewLister(repoRoot string) *Lister {
	return &Lister{runner: realRunner(repoRoot)}
}

// NewListerWithRunner constructs a Lister using the provided runner — used in tests to avoid real exec.
func NewListerWithRunner(runner RunnerFunc) *Lister {
	return &Lister{runner: runner}
}

// List runs `git ls-remote --heads origin agent/*` and returns one WorktreeEntry per agent branch.
// Branch is stored as "origin/<branch>" so that ChangedFiles(baseSHA, entry.Branch) works after fetch.
// Figura is derived by parsing the second segment of the agent/<figura>/... path.
// Status is always "active" — every remote agent branch is considered in-flight.
func (l *Lister) List(ctx context.Context) ([]port.WorktreeEntry, error) {
	stdout, err := l.runner(ctx, []string{"ls-remote", "--heads", "origin", "agent/*"})
	if err != nil {
		return nil, &ErrLsRemoteFailed{Cause: err}
	}
	return ParseLsRemoteOutput(stdout)
}

// ParseLsRemoteOutput parses the tab-separated output of `git ls-remote --heads origin 'agent/*'`
// into WorktreeEntry slices. Lines that are malformed or don't match the agent/<figura>/... pattern
// are silently skipped.
//
// Input format (one line per ref):
//
//	<sha>\trefs/heads/agent/<figura>/<ticket>
func ParseLsRemoteOutput(raw string) ([]port.WorktreeEntry, error) {
	var entries []port.WorktreeEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			// Malformed line — skip silently.
			continue
		}
		ref := strings.TrimSpace(parts[1])
		// ref must be: refs/heads/agent/<figura>/<ticket...>
		const prefix = "refs/heads/"
		if !strings.HasPrefix(ref, prefix) {
			continue
		}
		branchPath := strings.TrimPrefix(ref, prefix) // e.g. "agent/themis/TAL-16"
		segments := strings.SplitN(branchPath, "/", 3)
		// segments[0]="agent", segments[1]=<figura>, segments[2]=<ticket>
		if len(segments) < 3 || segments[0] != "agent" || segments[1] == "" || segments[2] == "" {
			continue
		}
		figura := segments[1]
		entries = append(entries, port.WorktreeEntry{
			Figura: figura,
			Branch: "origin/" + branchPath, // e.g. "origin/agent/themis/TAL-16"
			Status: "active",
		})
	}
	return entries, nil
}

// realRunner returns a RunnerFunc that shells out to the real `git` binary with the given repoRoot as Dir.
func realRunner(repoRoot string) RunnerFunc {
	return func(ctx context.Context, args []string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = repoRoot
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(errBuf.String()))
		}
		return outBuf.String(), nil
	}
}
