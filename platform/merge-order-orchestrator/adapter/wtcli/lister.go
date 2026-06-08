// Package wtcli provides the adapter that shells out to the wt binary to list agent worktrees.
package wtcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// Lister implements port.WorktreeLister by shelling out to the wt binary.
type Lister struct {
	wtBinary string
}

// NewLister constructs a Lister using the given wt binary name (e.g. "wt").
func NewLister(wtBinary string) *Lister {
	return &Lister{wtBinary: wtBinary}
}

var _ port.WorktreeLister = (*Lister)(nil)

// List shells `wt list --json` and decodes the output into a []port.WorktreeEntry.
func (l *Lister) List(ctx context.Context) ([]port.WorktreeEntry, error) {
	path, err := exec.LookPath(l.wtBinary)
	if err != nil {
		return nil, &mergeorder.ErrWtBinaryNotFound{Binary: l.wtBinary}
	}

	cmd := exec.CommandContext(ctx, path, "list", "--json")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if runErr := cmd.Run(); runErr != nil {
		return nil, fmt.Errorf("wt list --json: %w (stderr: %s)", runErr, strings.TrimSpace(errBuf.String()))
	}

	var entries []port.WorktreeEntry
	if err := json.Unmarshal(outBuf.Bytes(), &entries); err != nil {
		fragment := outBuf.String()
		if len(fragment) > 80 {
			fragment = fragment[:80]
		}
		return nil, &mergeorder.ErrWtOutputMalformed{Fragment: fragment, Cause: err}
	}
	return entries, nil
}
