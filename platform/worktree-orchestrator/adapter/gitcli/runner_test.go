// Package gitcli_test contains integration tests for the gitcli adapter.
// All tests are gated with testing.Short() and require a real git installation.
package gitcli_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/adapter/gitcli"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

// initRepo creates a temporary git repository at dir with an initial commit
// on branch "develop". Returns the repo root path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "develop")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("init\n"), 0644); err != nil {
		t.Fatalf("writing README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")

	return dir
}

func TestRunner_WorktreeAdd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-atlas")
	err := r.WorktreeAdd(ctx, wtPath, "agent/atlas/TAL-1", "develop")
	if err != nil {
		t.Fatalf("WorktreeAdd() error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("WorktreeAdd() did not create directory %q", wtPath)
	}
}

func TestRunner_WorktreeList_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-hermes")
	if err := r.WorktreeAdd(ctx, wtPath, "agent/hermes/TAL-2", "develop"); err != nil {
		t.Fatalf("WorktreeAdd setup: %v", err)
	}

	stdout, err := r.WorktreeList(ctx)
	if err != nil {
		t.Fatalf("WorktreeList() error: %v", err)
	}

	infos, err := worktree.ParseWorktreeList(stdout)
	if err != nil {
		t.Fatalf("ParseWorktreeList() error: %v", err)
	}

	found := false
	for _, info := range infos {
		if info.Branch == "agent/hermes/TAL-2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("WorktreeList() round-trip: branch agent/hermes/TAL-2 not found in %v", infos)
	}
}

func TestRunner_WorktreeRemove_Dirty(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-hermes")
	if err := r.WorktreeAdd(ctx, wtPath, "agent/hermes/TAL-2", "develop"); err != nil {
		t.Fatalf("WorktreeAdd setup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("creating dirty file: %v", err)
	}

	err := r.WorktreeRemove(ctx, wtPath, false)
	if err == nil {
		t.Fatal("WorktreeRemove(dirty, force=false) expected error, got nil")
	}

	var dirty *worktree.ErrDirtyWorktreeSentinel
	if !errors.As(err, &dirty) {
		t.Errorf("error type = %T, want *ErrDirtyWorktreeSentinel; err = %v", err, err)
	}
	if dirty.Path == "" {
		t.Error("ErrDirtyWorktreeSentinel.Path is empty")
	}
	if dirty.FileCount < 1 {
		t.Errorf("ErrDirtyWorktreeSentinel.FileCount = %d, want >= 1", dirty.FileCount)
	}
}

func TestRunner_WorktreeRemove_DirtyForce(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-hermes")
	if err := r.WorktreeAdd(ctx, wtPath, "agent/hermes/TAL-2", "develop"); err != nil {
		t.Fatalf("WorktreeAdd setup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("creating dirty file: %v", err)
	}

	err := r.WorktreeRemove(ctx, wtPath, true)
	if err != nil {
		t.Fatalf("WorktreeRemove(dirty, force=true) unexpected error: %v", err)
	}
}

func TestRunner_Prune(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-hermes")
	if err := r.WorktreeAdd(ctx, wtPath, "agent/hermes/TAL-2", "develop"); err != nil {
		t.Fatalf("WorktreeAdd setup: %v", err)
	}

	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("removing worktree dir: %v", err)
	}

	if err := r.Prune(ctx); err != nil {
		t.Fatalf("Prune() error: %v", err)
	}

	stdout, err := r.WorktreeList(ctx)
	if err != nil {
		t.Fatalf("WorktreeList after Prune: %v", err)
	}

	if strings.Contains(stdout, "agent/hermes/TAL-2") {
		t.Errorf("Prune() did not remove stale entry; stdout still contains branch")
	}
}

func TestRunner_BranchExists(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	exists, err := r.BranchExists(ctx, "develop")
	if err != nil {
		t.Fatalf("BranchExists(develop) error: %v", err)
	}
	if !exists {
		t.Error("BranchExists(develop) = false, want true")
	}

	exists, err = r.BranchExists(ctx, "no-such-branch")
	if err != nil {
		t.Fatalf("BranchExists(no-such-branch) error: %v", err)
	}
	if exists {
		t.Error("BranchExists(no-such-branch) = true, want false")
	}
}

func TestRunner_BranchDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: real git")
	}
	dir := initRepo(t)
	r := gitcli.NewRunner(dir)
	ctx := context.Background()

	wtPath := filepath.Join(dir, "talos.wt", "agent-atlas")
	if err := r.WorktreeAdd(ctx, wtPath, "agent/atlas/TAL-1", "develop"); err != nil {
		t.Fatalf("WorktreeAdd setup: %v", err)
	}

	if err := r.WorktreeRemove(ctx, wtPath, false); err != nil {
		t.Fatalf("WorktreeRemove before BranchDelete: %v", err)
	}
	if err := r.Prune(ctx); err != nil {
		t.Fatalf("Prune before BranchDelete: %v", err)
	}

	err := r.BranchDelete(ctx, "agent/atlas/TAL-1")
	if err != nil {
		t.Fatalf("BranchDelete() error: %v", err)
	}

	exists, err := r.BranchExists(ctx, "agent/atlas/TAL-1")
	if err != nil {
		t.Fatalf("BranchExists after delete: %v", err)
	}
	if exists {
		t.Error("BranchDelete() did not remove the branch")
	}
}
