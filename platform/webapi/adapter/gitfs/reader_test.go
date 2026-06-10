package gitfs

import (
	"context"
	"fmt"
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
