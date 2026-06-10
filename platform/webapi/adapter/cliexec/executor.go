// Package cliexec is the concrete CLIExecutor backed by os/exec.
package cliexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// Executor invokes platform CLI binaries. When binDir is set, binaries are
// resolved there; otherwise they are looked up on PATH.
type Executor struct {
	binDir string
}

// New builds an Executor resolving binaries under binDir (empty = PATH).
func New(binDir string) *Executor {
	return &Executor{binDir: binDir}
}

// NewFromEnv reads TALOS_BIN_DIR.
func NewFromEnv() *Executor {
	return &Executor{binDir: os.Getenv("TALOS_BIN_DIR")}
}

// Run executes `<name> <args...>` and returns stdout.
func (e *Executor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	bin := name
	if e.binDir != "" {
		bin = filepath.Join(e.binDir, name)
	}
	return exec.CommandContext(ctx, bin, args...).Output()
}
