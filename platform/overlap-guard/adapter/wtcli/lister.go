// Package wtcli provides the adapter that shells out to the wt binary to list agent worktrees.
package wtcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// ErrWtBinaryNotFound is returned when the wt binary cannot be located on PATH.
type ErrWtBinaryNotFound struct {
	Binary string
}

func (e *ErrWtBinaryNotFound) Error() string {
	return fmt.Sprintf("wt binary %q not found on PATH", e.Binary)
}

// ErrWtOutputMalformed is returned when wt list --json emits non-JSON or unexpected output.
type ErrWtOutputMalformed struct {
	Fragment string
	Cause    error
}

func (e *ErrWtOutputMalformed) Error() string {
	return fmt.Sprintf("wt list --json output malformed (fragment: %q): %v", e.Fragment, e.Cause)
}

// Lister implements port.WorktreeLister by shelling out to the wt binary.
type Lister struct {
	wtBinary string
}

var _ port.WorktreeLister = (*Lister)(nil)

// NewLister constructs a Lister using the given wt binary name (e.g. "wt").
func NewLister(wtBinary string) *Lister {
	return &Lister{wtBinary: wtBinary}
}

// List shells `wt list --json` and decodes the output into []port.WorktreeEntry.
func (l *Lister) List(ctx context.Context) ([]port.WorktreeEntry, error) {
	path, err := exec.LookPath(l.wtBinary)
	if err != nil {
		return nil, &ErrWtBinaryNotFound{Binary: l.wtBinary}
	}

	cmd := exec.CommandContext(ctx, path, "list", "--json")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if runErr := cmd.Run(); runErr != nil {
		return nil, fmt.Errorf("wt list --json: %w (stderr: %s)", runErr, strings.TrimSpace(errBuf.String()))
	}

	entries, parseErr := ParseWorktreeJSONToEntries(outBuf.Bytes())
	if parseErr != nil {
		return nil, parseErr
	}
	return entries, nil
}

// ParseWorktreeJSON validates that raw is valid JSON; returns ErrWtOutputMalformed if not.
func ParseWorktreeJSON(raw []byte) error {
	_, err := ParseWorktreeJSONToEntries(raw)
	return err
}

// ParseWorktreeJSONToEntries decodes raw JSON bytes into []port.WorktreeEntry.
func ParseWorktreeJSONToEntries(raw []byte) ([]port.WorktreeEntry, error) {
	var entries []port.WorktreeEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fragment := string(raw)
		if len(fragment) > 80 {
			fragment = fragment[:80]
		}
		return nil, &ErrWtOutputMalformed{Fragment: fragment, Cause: err}
	}
	return entries, nil
}
