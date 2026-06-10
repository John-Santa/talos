package gitfs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRunner records git invocations and returns canned output, so write
// actions can be unit-tested without touching a real repository.
type fakeRunner struct {
	calls   [][]string
	outputs map[string]string
	errs    map[string]bool
}

func (f *fakeRunner) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{dir}, args...))
	key := strings.Join(args, " ")
	if f.errs[key] {
		return nil, fmt.Errorf("git %s failed", key)
	}
	return []byte(f.outputs[key]), nil
}

func (f *fakeRunner) called(sub string) bool {
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), sub) {
			return true
		}
	}
	return false
}

func newFakeReader(fr *fakeRunner) *Reader {
	r := New("/repo", "develop")
	r.run = fr.run
	return r
}

const twoWorktrees = "worktree /repo\nHEAD aaaaaaa\nbranch refs/heads/develop\n\n" +
	"worktree /repo/talos.wt/agent-hermes\nHEAD bbbbbbb\nbranch refs/heads/agent/hermes/TAL-15\n"

func TestWorktreesFiltersAgentBranches(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{"worktree list --porcelain": twoWorktrees}}
	wts, err := newFakeReader(fr).Worktrees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 1 || wts[0].Figura != "hermes" || wts[0].Branch != "agent/hermes/TAL-15" {
		t.Errorf("worktrees = %+v, want only the hermes agent worktree", wts)
	}
}

func TestCreateWorktreeIssuesGitAdd(t *testing.T) {
	fr := &fakeRunner{}
	if err := newFakeReader(fr).CreateWorktree(context.Background(), "atlas", "TAL-99"); err != nil {
		t.Fatal(err)
	}
	if !fr.called("worktree add talos.wt/agent-atlas -b agent/atlas/TAL-99 develop") {
		t.Errorf("expected git worktree add, got calls: %v", fr.calls)
	}
}

func TestTeardownRemovesByPath(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{"worktree list --porcelain": twoWorktrees}}
	if err := newFakeReader(fr).TeardownWorktree(context.Background(), "hermes"); err != nil {
		t.Fatal(err)
	}
	if !fr.called("worktree remove --force /repo/talos.wt/agent-hermes") {
		t.Errorf("expected git worktree remove, got calls: %v", fr.calls)
	}
}

func TestMergeRunsWhenClean(t *testing.T) {
	fr := &fakeRunner{outputs: map[string]string{"worktree list --porcelain": twoWorktrees}}
	if err := newFakeReader(fr).Merge(context.Background(), "hermes", "TAL-15"); err != nil {
		t.Fatal(err)
	}
	if !fr.called("merge --no-edit agent/hermes/TAL-15") {
		t.Errorf("expected the merge to run, got calls: %v", fr.calls)
	}
}

func TestMergeAbortsOnConflict(t *testing.T) {
	fr := &fakeRunner{
		outputs: map[string]string{"worktree list --porcelain": twoWorktrees},
		errs:    map[string]bool{"merge-tree --write-tree --name-only develop agent/hermes/TAL-15": true},
	}
	err := newFakeReader(fr).Merge(context.Background(), "hermes", "TAL-15")
	if err == nil {
		t.Fatal("expected a conflict error")
	}
	if fr.called("merge --no-edit") {
		t.Errorf("merge must NOT run when merge-tree reports a conflict; calls: %v", fr.calls)
	}
}

// --- PR2: MergePlan real conflict check (task 2.7) ---------------------------

const threeWorktrees = "worktree /repo\nHEAD aaaaaaa\nbranch refs/heads/develop\n\n" +
	"worktree /repo/talos.wt/agent-iris\nHEAD bbbbbbb\nbranch refs/heads/agent/iris/TAL-42\n\n" +
	"worktree /repo/talos.wt/agent-atlas\nHEAD ccccccc\nbranch refs/heads/agent/atlas/TAL-55\n"

func TestMergePlanConflictRate(t *testing.T) {
	tests := []struct {
		name             string
		ahead            map[string]string  // "rev-list --count develop..branch" → count
		conflictErrs     map[string]bool    // merge-tree key → error
		wantRate         float64
		wantClean        []bool // PredictedClean per step (in list order)
	}{
		{
			name: "all clean → ConflictRate 0",
			ahead: map[string]string{
				"rev-list --count develop..agent/iris/TAL-42":  "2",
				"rev-list --count develop..agent/atlas/TAL-55": "3",
			},
			conflictErrs: map[string]bool{},
			wantRate:     0,
			wantClean:    []bool{true, true},
		},
		{
			name: "1 of 2 conflicts → ConflictRate 0.5",
			ahead: map[string]string{
				"rev-list --count develop..agent/iris/TAL-42":  "2",
				"rev-list --count develop..agent/atlas/TAL-55": "3",
			},
			conflictErrs: map[string]bool{
				"merge-tree --write-tree --name-only develop agent/iris/TAL-42": true,
			},
			wantRate:  0.5,
			wantClean: []bool{false, true},
		},
		{
			name: "branch with ahead=0 skipped in rate",
			ahead: map[string]string{
				"rev-list --count develop..agent/iris/TAL-42":  "0",
				"rev-list --count develop..agent/atlas/TAL-55": "2",
			},
			conflictErrs: map[string]bool{},
			wantRate:     0,
			wantClean:    []bool{false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputs := map[string]string{"worktree list --porcelain": threeWorktrees}
			for k, v := range tt.ahead {
				outputs[k] = v
			}
			fr := &fakeRunner{outputs: outputs, errs: tt.conflictErrs}
			plan, err := newFakeReader(fr).MergePlan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if plan.ConflictRate != tt.wantRate {
				t.Errorf("ConflictRate = %v, want %v", plan.ConflictRate, tt.wantRate)
			}
			for i, step := range plan.Steps {
				if i >= len(tt.wantClean) {
					break
				}
				if step.PredictedClean != tt.wantClean[i] {
					t.Errorf("step[%d] PredictedClean = %v, want %v (branch %s)",
						i, step.PredictedClean, tt.wantClean[i], step.Branch)
				}
			}
		})
	}
}

// TestMergeExactFiguraMatch verifies that two worktrees sharing a jiraKey are
// distinguished by figura: only the exact agent/<figura>/<jiraKey> branch merges.
func TestMergeExactFiguraMatch(t *testing.T) {
	twoFiguras := "worktree /repo\nHEAD aaaaaaa\nbranch refs/heads/develop\n\n" +
		"worktree /repo/talos.wt/agent-iris\nHEAD bbbbbbb\nbranch refs/heads/agent/iris/TAL-42\n\n" +
		"worktree /repo/talos.wt/agent-atlas\nHEAD ccccccc\nbranch refs/heads/agent/atlas/TAL-42\n"

	t.Run("correct figura merges", func(t *testing.T) {
		fr := &fakeRunner{outputs: map[string]string{"worktree list --porcelain": twoFiguras}}
		if err := newFakeReader(fr).Merge(context.Background(), "iris", "TAL-42"); err != nil {
			t.Fatal(err)
		}
		if !fr.called("merge --no-edit agent/iris/TAL-42") {
			t.Errorf("expected iris branch to merge; calls: %v", fr.calls)
		}
		if fr.called("agent/atlas/TAL-42") {
			t.Errorf("atlas branch must NOT be touched; calls: %v", fr.calls)
		}
	})

	t.Run("wrong figura returns error", func(t *testing.T) {
		fr := &fakeRunner{outputs: map[string]string{"worktree list --porcelain": twoFiguras}}
		err := newFakeReader(fr).Merge(context.Background(), "cronos", "TAL-42")
		if err == nil {
			t.Fatal("expected error for non-matching figura")
		}
		if fr.called("merge --no-edit") {
			t.Errorf("merge must NOT run; calls: %v", fr.calls)
		}
	})
}

// --- integration tests with real git repos in t.TempDir --------------------

// initGitRepo creates a bare-minimum git repo in dir with an initial commit on
// the given branch name. Returns the repo path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "develop")
	run("config", "user.email", "test@test")
	run("config", "user.name", "test")
	// seed an initial file so develop has a commit
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-m", "init")
	return dir
}

// TestMergeTreeConflictCleanPair checks mergeTreeConflict returns clean=true
// when branches have no conflicting changes.
func TestMergeTreeConflictCleanPair(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create a feature branch with a new file (no conflict possible).
	run("checkout", "-b", "agent/iris/TAL-1")
	if err := os.WriteFile(filepath.Join(dir, "iris.txt"), []byte("iris work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "iris.txt")
	run("commit", "-m", "iris commit")
	run("checkout", "develop")

	r := New(dir, "develop")
	files, clean := r.mergeTreeConflict(context.Background(), "develop", "agent/iris/TAL-1")
	if !clean {
		t.Errorf("expected clean merge, got conflictFiles=%v", files)
	}
	if len(files) != 0 {
		t.Errorf("expected no conflict files, got %v", files)
	}
}

// TestMergeTreeConflictConflictPair checks mergeTreeConflict returns clean=false
// and lists the conflicting file when both branches edit the same line.
func TestMergeTreeConflictConflictPair(t *testing.T) {
	dir := initGitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Both branches edit the same line in base.txt → conflict.
	run("checkout", "-b", "agent/iris/TAL-2")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("iris edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-m", "iris edits base")
	run("checkout", "develop")

	// Develop also edits base.txt after branching.
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("develop edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-m", "develop edits base")

	r := New(dir, "develop")
	files, clean := r.mergeTreeConflict(context.Background(), "develop", "agent/iris/TAL-2")
	if clean {
		t.Error("expected conflict, got clean")
	}
	if len(files) == 0 {
		t.Error("expected at least one conflict file listed, got none")
	}
	found := false
	for _, f := range files {
		if strings.Contains(f, "base.txt") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected base.txt in conflict files, got %v", files)
	}
}
