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

// Lister implements port.WorktreeLister by listing open agent/* branches.
// All branches are returned with Status="active" — every in-flight branch is a collision candidate.
//
// Two modes:
//   - ls-remote mode (NewLister/NewListerWithRunner): List() shells out to `git ls-remote`.
//   - explicit mode (NewListerFromBranches): List() returns a precomputed entry set and never
//     touches git — used by `ov scan --remote --branches`, where CI supplies the open-PR set
//     (`gh pr list`) so stale squash-merged branches are excluded by construction.
type Lister struct {
	runner   RunnerFunc
	entries  []port.WorktreeEntry // explicit mode: returned verbatim
	explicit bool                 // true → return entries directly, never shell out (NewListerFromBranches)
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

// NewListerFromBranches constructs an explicit-mode Lister over the given agent/* branch names
// (the open-PR set, e.g. from `gh pr list --state open --json headRefName`). It bypasses
// `git ls-remote`, so stale squash-merged branches that ls-remote would surface are excluded.
// Blank and non-agent branch names are dropped.
func NewListerFromBranches(branches []string) *Lister {
	return &Lister{entries: entriesFromBranchNames(branches), explicit: true}
}

// List returns one WorktreeEntry per in-flight agent branch. In explicit mode (l.explicit, set by
// NewListerFromBranches) it returns the precomputed set; otherwise it runs `git ls-remote --heads origin agent/*`.
// Branch is stored as "origin/<branch>" so ChangedFiles(baseSHA, entry.Branch) works after fetch.
func (l *Lister) List(ctx context.Context) ([]port.WorktreeEntry, error) {
	if l.explicit {
		return l.entries, nil
	}
	// ls-remote mode. A nil runner here is a constructor bug, not "all clear": fail loud rather than
	// silently returning zero claims on a hard gate.
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
		if e, ok := entryFromBranchPath(branchPath); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// entriesFromBranchNames converts plain agent/* branch names (no refs/heads/ prefix) into entries,
// dropping blanks, non-agent branches, and exact-duplicate branches (so a repeated input doesn't
// emit duplicate file_collisions rows). Shared by NewListerFromBranches.
func entriesFromBranchNames(branches []string) []port.WorktreeEntry {
	var entries []port.WorktreeEntry
	seen := make(map[string]bool)
	for _, b := range branches {
		e, ok := entryFromBranchPath(b)
		if !ok || seen[e.Branch] {
			continue
		}
		seen[e.Branch] = true
		entries = append(entries, e)
	}
	return entries
}

// entryFromBranchPath builds a WorktreeEntry from an agent/<figura>/<ticket> branch path
// (no refs/heads/ prefix). Returns ok=false when the path is not a well-formed agent branch.
// Branch is "origin/"-prefixed so ChangedFiles diffs against the fetched remote-tracking ref.
func entryFromBranchPath(branchPath string) (port.WorktreeEntry, bool) {
	branchPath = strings.TrimSpace(branchPath)
	segments := strings.SplitN(branchPath, "/", 3)
	// segments[0]="agent", segments[1]=<figura>, segments[2]=<ticket>
	if len(segments) < 3 || segments[0] != "agent" || segments[1] == "" || segments[2] == "" {
		return port.WorktreeEntry{}, false
	}
	return port.WorktreeEntry{
		Figura: segments[1],
		Branch: "origin/" + branchPath,
		Status: "active",
	}, true
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
