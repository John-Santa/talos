package main

import (
	"testing"
)

// TestRepoRootDoesNotPanic verifies repoRoot() never panics and always returns
// a non-empty string (falls back to cwd when git is unavailable).
func TestRepoRootDoesNotPanic(t *testing.T) {
	t.Parallel()
	got := repoRoot()
	if got == "" {
		t.Error("repoRoot() returned empty string; expected cwd fallback")
	}
}

// TestMainWorktreeRootDoesNotPanic verifies mainWorktreeRoot() never panics
// and returns either a non-empty path or "" (both are valid).
func TestMainWorktreeRootDoesNotPanic(t *testing.T) {
	t.Parallel()
	// No assertion on value — only that it does not panic.
	_ = mainWorktreeRoot()
}

// TestMainWorktreeRootOutsideRepo verifies mainWorktreeRoot() returns "" when
// git-common-dir is unavailable or points to the same root as repoRoot
// (i.e. not a linked worktree). We cannot force "outside a repo" in the test
// runner, but we can confirm the function handles the normal repo case
// gracefully (returns "" or a valid path, no panic).
func TestMainWorktreeRootGraceful(t *testing.T) {
	t.Parallel()
	result := mainWorktreeRoot()
	// result is either "" (main worktree / error) or a valid dir path.
	// We only verify the invariant: if non-empty, it must be absolute.
	if result != "" {
		if len(result) == 0 || result[0] != '/' {
			t.Errorf("mainWorktreeRoot() returned non-absolute path: %q", result)
		}
	}
}
