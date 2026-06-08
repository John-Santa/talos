package gitcli_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/adapter/gitcli"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"errors"
	"os"
)

func TestIntegrator_RebaseOnto_clean(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires git")
	}
	if !gitAvailable() {
		t.Skip("git not on PATH")
	}

	// set up: base branch, feature branch with clean rebase
	dir := initRepo(t, "develop")
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return string(out)
	}
	mustRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_ = run

	// create feature worktree-like: just a branch in same repo for testing
	mustRun("checkout", "-b", "feature/clean")
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("feature"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", ".")
	mustRun("commit", "-m", "feature commit")

	intg := gitcli.NewIntegrator(dir)
	conflicts, err := intg.RebaseOnto(context.Background(), "feature/clean", "develop")
	if err != nil {
		t.Fatalf("RebaseOnto clean: unexpected error %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("RebaseOnto clean: expected no conflicts, got %v", conflicts)
	}
}

func TestIntegrator_RebaseOnto_conflict(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires git")
	}
	if !gitAvailable() {
		t.Skip("git not on PATH")
	}

	dir := initRepo(t, "develop")
	mustRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// diverge: feature edits README, then develop also edits README
	mustRun("checkout", "-b", "feature/conflict")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("feature content"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", ".")
	mustRun("commit", "-m", "feature edit README")

	mustRun("checkout", "develop")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("develop diverged"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", ".")
	mustRun("commit", "-m", "develop edit README")

	intg := gitcli.NewIntegrator(dir)
	_, err := intg.RebaseOnto(context.Background(), "feature/conflict", "develop")
	if err == nil {
		t.Fatal("RebaseOnto conflict: expected error, got nil")
	}
	var rc *mergeorder.ErrRebaseConflict
	if !errors.As(err, &rc) {
		t.Errorf("RebaseOnto conflict: expected *ErrRebaseConflict, got %T: %v", err, err)
	}
}
