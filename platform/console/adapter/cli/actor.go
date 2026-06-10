package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/console/port"
)

// ErrActionFailed is returned when a write command exits with a non-zero status.
// The error wraps the original exec error and includes stderr output.
var ErrActionFailed = errors.New("wt action failed")

// runCmd shells out to bin with args. Unlike runJSON it does NOT parse output;
// it only captures stderr for diagnostics. A non-zero exit returns ErrActionFailed.
//
// Pattern: LookPath → CommandContext → capture stderr → typed sentinel on non-zero.
func runCmd(ctx context.Context, bin string, args ...string) error {
	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBinaryNotFound, bin)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if runErr := cmd.Run(); runErr != nil {
		stderr := strings.TrimSpace(errBuf.String())
		if stderr != "" {
			return fmt.Errorf("%w: %s %s: %v (stderr: %s)", ErrActionFailed, bin, strings.Join(args, " "), runErr, stderr)
		}
		return fmt.Errorf("%w: %s %s: %v", ErrActionFailed, bin, strings.Join(args, " "), runErr)
	}
	return nil
}

// Actor implements port.PlatformActor by shelling out to the wt and mo binaries.
type Actor struct {
	wtBinary string
	moBinary string
}

// NewActor constructs an Actor using the given binary names for the wt and mo CLIs.
// Both binaries must be discoverable on PATH at call time.
func NewActor(wtBinary, moBinary string) *Actor {
	return &Actor{wtBinary: wtBinary, moBinary: moBinary}
}

// compile-time check: Actor must satisfy port.PlatformActor.
var _ port.PlatformActor = (*Actor)(nil)

// TeardownWorktree tears down the worktree for the given figura and jiraKey by
// running `wt teardown <figura> <jiraKey>`.
//
// NOTE: wt teardown has a known bug (TAL-10) and may fail in live usage.
// The TUI surfaces any error as a toast notification.
func (a *Actor) TeardownWorktree(ctx context.Context, figura, jiraKey string) error {
	return runCmd(ctx, a.wtBinary, "teardown", figura, jiraKey)
}

// CreateWorktree creates a new worktree for the given figura and jiraKey by
// running `wt create <figura> <jiraKey>`.
func (a *Actor) CreateWorktree(ctx context.Context, figura, jiraKey string) error {
	return runCmd(ctx, a.wtBinary, "create", figura, jiraKey)
}

// ExecuteMerge runs `mo execute --yes` to merge all ready worktree branches
// into the base branch. The --yes flag is required by the mo CLI to confirm
// the destructive operation; the TUI type-to-confirm gate is the caller-side
// safety check before this method is invoked.
func (a *Actor) ExecuteMerge(ctx context.Context) error {
	return runCmd(ctx, a.moBinary, "execute", "--yes")
}
