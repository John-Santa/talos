package gitprobe_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/adapter/gitprobe"
	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"errors"
)

func hasGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func TestProbe_IsGitRepo_True(t *testing.T) {
	t.Parallel()
	hasGit(t)
	dir := initGitRepo(t)

	p := gitprobe.New()
	ok, err := p.IsGitRepo(dir)
	if err != nil {
		t.Fatalf("IsGitRepo() error: %v", err)
	}
	if !ok {
		t.Error("IsGitRepo() = false, want true")
	}
}

func TestProbe_IsGitRepo_False(t *testing.T) {
	t.Parallel()
	hasGit(t)
	dir := t.TempDir() // plain dir, not a git repo

	p := gitprobe.New()
	ok, err := p.IsGitRepo(dir)
	if err != nil {
		t.Fatalf("IsGitRepo() unexpected error: %v", err)
	}
	if ok {
		t.Error("IsGitRepo() = true, want false for non-git dir")
	}
}

func TestProbe_Resolve_ExistingPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	p := gitprobe.New()
	resolved, err := p.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if resolved == "" {
		t.Error("Resolve() returned empty path")
	}
	// should be an absolute path
	if !filepath.IsAbs(resolved) {
		t.Errorf("Resolve() = %q, want absolute path", resolved)
	}
}

func TestProbe_Resolve_NotFound(t *testing.T) {
	t.Parallel()
	p := gitprobe.New()

	_, err := p.Resolve("/this/path/does/not/exist/anywhere")
	var target *workspace.ErrRepoNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrRepoNotFound, got %T: %v", err, err)
	}
}
