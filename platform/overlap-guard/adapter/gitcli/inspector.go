// Package gitcli provides os/exec-backed git inspection for overlap-guard (ADR-OV1: self-contained).
package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// Inspector implements port.GitInspector via os/exec. Self-contained per ADR-OV1.
type Inspector struct {
	repoRoot string
}

var _ port.GitInspector = (*Inspector)(nil)

// NewInspector constructs an Inspector for the given repository root path.
func NewInspector(repoRoot string) *Inspector {
	return &Inspector{repoRoot: repoRoot}
}

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

// ChangedFiles returns files changed between base and branch using three-dot diff (git diff --name-only base...branch).
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
