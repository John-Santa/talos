package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/cli"
)

// writeFakeWtBinary writes a shell script that exits 0 and emits nothing to stdout.
func writeFakeWtBinary(t *testing.T, dir, name string) {
	t.Helper()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeFakeWtBinaryNonZero writes a shell script that exits non-zero and writes to stderr.
func writeFakeWtBinaryNonZero(t *testing.T, dir, name string) {
	t.Helper()
	script := "#!/bin/sh\nprintf 'teardown failed: no such worktree' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestActor_TeardownWorktree_HappyPath verifies that TeardownWorktree returns nil
// when the wt binary exits with code 0.
func TestActor_TeardownWorktree_HappyPath(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeFakeWtBinary(t, dir, "wt")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	a := cli.NewActor("wt")
	if err := a.TeardownWorktree(context.Background(), "iris", "TAL-18"); err != nil {
		t.Fatalf("TeardownWorktree: unexpected error: %v", err)
	}
}

// TestActor_TeardownWorktree_NonZeroExit verifies that TeardownWorktree returns
// ErrActionFailed (wrapping the non-zero exit) when the wt binary exits non-zero.
func TestActor_TeardownWorktree_NonZeroExit(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeFakeWtBinaryNonZero(t, dir, "wt")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	a := cli.NewActor("wt")
	err := a.TeardownWorktree(context.Background(), "iris", "TAL-18")
	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !errors.Is(err, cli.ErrActionFailed) {
		t.Errorf("want errors.Is(err, ErrActionFailed); got %v", err)
	}
}

// TestActor_TeardownWorktree_BinaryNotFound verifies that TeardownWorktree returns
// ErrBinaryNotFound when the wt binary is not on PATH.
func TestActor_TeardownWorktree_BinaryNotFound(t *testing.T) {
	t.Parallel()

	a := cli.NewActor("wt-does-not-exist-actor-test-only")
	err := a.TeardownWorktree(context.Background(), "iris", "TAL-18")
	if err == nil {
		t.Fatal("expected error when wt binary missing, got nil")
	}
	if !errors.Is(err, cli.ErrBinaryNotFound) {
		t.Errorf("want errors.Is(err, ErrBinaryNotFound); got %v", err)
	}
}

// writeFakeWtBinaryRecordsArgs writes a shell script that records its arguments to
// a file (one per line) and exits 0.
func writeFakeWtBinaryRecordsArgs(t *testing.T, dir, name, argsFile string) {
	t.Helper()
	// $@ expands to all arguments; each on its own line via printf "%s\n".
	script := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\"; done > " + argsFile + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestActor_CreateWorktree_HappyPath verifies that CreateWorktree returns nil
// when the wt binary exits 0, and that it was called with "create <figura> <key>".
func TestActor_CreateWorktree_HappyPath(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "wt-args.txt")
	writeFakeWtBinaryRecordsArgs(t, dir, "wt", argsFile)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	a := cli.NewActor("wt")
	if err := a.CreateWorktree(context.Background(), "iris", "TAL-19"); err != nil {
		t.Fatalf("CreateWorktree: unexpected error: %v", err)
	}

	// Verify the args passed to the binary: should be "create", "iris", "TAL-19".
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading args file: %v", err)
	}
	lines := splitLines(string(raw))
	if len(lines) != 3 {
		t.Fatalf("expected 3 args (create <figura> <key>), got %d: %v", len(lines), lines)
	}
	if lines[0] != "create" {
		t.Errorf("arg[0] = %q, want %q", lines[0], "create")
	}
	if lines[1] != "iris" {
		t.Errorf("arg[1] = %q, want %q", lines[1], "iris")
	}
	if lines[2] != "TAL-19" {
		t.Errorf("arg[2] = %q, want %q", lines[2], "TAL-19")
	}
}

// TestActor_CreateWorktree_NonZeroExit verifies that CreateWorktree returns
// ErrActionFailed when the wt binary exits non-zero.
func TestActor_CreateWorktree_NonZeroExit(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeFakeWtBinaryNonZero(t, dir, "wt")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	a := cli.NewActor("wt")
	err := a.CreateWorktree(context.Background(), "iris", "TAL-19")
	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !errors.Is(err, cli.ErrActionFailed) {
		t.Errorf("want errors.Is(err, ErrActionFailed); got %v", err)
	}
}

// splitLines splits s on newlines, dropping empty entries.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
