// Package runner provides the injectable command runner used by all adapters.
// Tests inject a fake runner; production uses the real exec.CommandContext wrapper.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner is the injectable interface for executing external commands.
// All adapters accept a Runner to allow test doubles without spawning real processes.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner is the production implementation that delegates to exec.CommandContext.
type ExecRunner struct{}

// New returns a production ExecRunner.
func New() *ExecRunner { return &ExecRunner{} }

// Run executes name with args and returns combined stdout bytes on success.
// On non-zero exit, it wraps stderr into the error message.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		stderr := strings.TrimSpace(errBuf.String())
		if stderr != "" {
			return nil, fmt.Errorf("%s %v: %w (stderr: %s)", name, args, err, stderr)
		}
		return nil, fmt.Errorf("%s %v: %w", name, args, err)
	}
	return outBuf.Bytes(), nil
}
