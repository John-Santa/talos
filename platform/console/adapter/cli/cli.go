// Package cli provides the adapter that shells out to the wt binary to read platform state.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/port"
)

// ErrBinaryNotFound is returned when a required binary cannot be located on PATH.
var ErrBinaryNotFound = errors.New("wt binary not found in PATH")

// ErrOutputMalformed is returned when the output of a CLI command cannot be parsed.
var ErrOutputMalformed = errors.New("wt output malformed")

// runJSON shells out to bin with args, captures stdout, and unmarshals the result into v.
// It mirrors the blessed pattern from platform/merge-order-orchestrator/adapter/wtcli/lister.go:
// LookPath → CommandContext → buffered stdout/stderr → Unmarshal → typed sentinel errors.
func runJSON(ctx context.Context, bin string, args []string, v any) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBinaryNotFound, bin)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("%s %s: %w (stderr: %s)", bin, strings.Join(args, " "), runErr, strings.TrimSpace(errBuf.String()))
	}

	if err := json.Unmarshal(outBuf.Bytes(), v); err != nil {
		fragment := outBuf.String()
		if len(fragment) > 80 {
			fragment = fragment[:80]
		}
		return fmt.Errorf("%w: %s (cause: %v)", ErrOutputMalformed, fragment, err)
	}
	return nil
}

// Reader implements port.PlatformReader by shelling out to the platform CLIs.
type Reader struct {
	wtBinary string
	moBinary string
	ovBinary string
	chBinary string
}

// NewReader constructs a Reader using the given binary names for each platform CLI.
// All four binaries must be discoverable on PATH at call time.
//
//	wt — worktree orchestrator (wt list --json)
//	mo — merge-order orchestrator (mo plan --json)
//	ov — overlap guard (ov scan --json)
//	ch — ci-checks (ch labels --branch <branch> --json)
func NewReader(wtBinary, moBinary, ovBinary, chBinary string) *Reader {
	return &Reader{
		wtBinary: wtBinary,
		moBinary: moBinary,
		ovBinary: ovBinary,
		chBinary: chBinary,
	}
}

// compile-time check: Reader must satisfy port.PlatformReader.
var _ port.PlatformReader = (*Reader)(nil)

// Worktrees runs `wt list --json` and returns the decoded list of agent worktrees.
func (r *Reader) Worktrees(ctx context.Context) ([]platform.Worktree, error) {
	var worktrees []platform.Worktree
	if err := runJSON(ctx, r.wtBinary, []string{"list", "--json"}, &worktrees); err != nil {
		return nil, err
	}
	return worktrees, nil
}

// MergePlan runs `mo plan --json` and returns the decoded merge plan.
func (r *Reader) MergePlan(ctx context.Context) (platform.MergePlan, error) {
	var plan platform.MergePlan
	if err := runJSON(ctx, r.moBinary, []string{"plan", "--json"}, &plan); err != nil {
		return platform.MergePlan{}, err
	}
	return plan, nil
}

// Overlap runs `ov scan --json` and returns the decoded overlap report.
func (r *Reader) Overlap(ctx context.Context) (platform.Overlap, error) {
	var ov platform.Overlap
	if err := runJSON(ctx, r.ovBinary, []string{"scan", "--json"}, &ov); err != nil {
		return platform.Overlap{}, err
	}
	return ov, nil
}

// Labels runs `ch labels --branch <branch> --json` and returns the decoded label state.
func (r *Reader) Labels(ctx context.Context, branch string) (platform.Labels, error) {
	var labels platform.Labels
	if err := runJSON(ctx, r.chBinary, []string{"labels", "--branch", branch, "--json"}, &labels); err != nil {
		return platform.Labels{}, err
	}
	return labels, nil
}
