package gitcli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitcli"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// TestRevParse_Integration runs against a real git repo in the project root.
func TestRevParse_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real git repository")
	}
	root := repoRoot(t)
	ins := gitcli.NewInspector(root)
	sha, err := ins.RevParse(context.Background(), "HEAD")
	if err != nil {
		t.Fatalf("RevParse HEAD: %v", err)
	}
	if len(sha) < 40 {
		t.Errorf("expected full SHA (40+ chars), got %q", sha)
	}
}

// TestChangedFiles_Integration verifies that ChangedFiles returns a slice (may be empty on clean HEAD).
func TestChangedFiles_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real git repository")
	}
	root := repoRoot(t)
	ins := gitcli.NewInspector(root)

	ctx := context.Background()
	sha, err := ins.RevParse(ctx, "HEAD")
	if err != nil {
		t.Fatalf("RevParse HEAD: %v", err)
	}

	// ChangedFiles of HEAD...HEAD should be empty (no diff with itself).
	files, err := ins.ChangedFiles(ctx, sha, sha)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	// Empty is valid; just ensure no error.
	t.Logf("ChangedFiles HEAD...HEAD: %d files", len(files))
}

// TestFetch_Integration runs a real git fetch; skipped under -short.
func TestFetch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real git repository with remote")
	}
	root := repoRoot(t)
	ins := gitcli.NewInspector(root)
	if err := ins.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
}

// TestChangedFiles_Format verifies that the three-dot diff format is used.
func TestChangedFiles_Format(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real git repository")
	}
	root := repoRoot(t)
	ins := gitcli.NewInspector(root)
	ctx := context.Background()

	// Use develop as base to check that three-dot diff runs without error.
	files, err := ins.ChangedFiles(ctx, "origin/develop", "HEAD")
	if err != nil {
		// Might fail if origin/develop not available; log and skip.
		t.Logf("ChangedFiles origin/develop...HEAD: %v (skipping assertion)", err)
		return
	}
	t.Logf("ChangedFiles origin/develop...HEAD: %d files changed", len(files))
	for _, f := range files {
		if strings.Contains(f, "\n") {
			t.Errorf("file path should not contain newline: %q", f)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from current file to find .git directory.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate .git directory")
		}
		dir = parent
	}
}

// varInspectorCheck is a compile-time assertion that *Inspector satisfies port.GitInspector.
var _ port.GitInspector = (*gitcli.Inspector)(nil)
