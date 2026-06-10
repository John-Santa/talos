package gitremote_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitremote"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// TestParseLsRemoteOutput_NoBranches verifies an empty output produces an empty slice.
func TestParseLsRemoteOutput_NoBranches(t *testing.T) {
	t.Parallel()

	entries, err := gitremote.ParseLsRemoteOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestParseLsRemoteOutput_SingleBranch verifies a single agent branch is parsed correctly.
func TestParseLsRemoteOutput_SingleBranch(t *testing.T) {
	t.Parallel()

	raw := "abc1234567890123456789012345678901234567890\trefs/heads/agent/themis/TAL-16\n"
	entries, err := gitremote.ParseLsRemoteOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := port.WorktreeEntry{
		Figura: "themis",
		Branch: "origin/agent/themis/TAL-16",
		Status: "active",
	}
	got := entries[0]
	if got.Figura != want.Figura {
		t.Errorf("Figura = %q, want %q", got.Figura, want.Figura)
	}
	if got.Branch != want.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, want.Branch)
	}
	if got.Status != want.Status {
		t.Errorf("Status = %q, want %q", got.Status, want.Status)
	}
}

// TestParseLsRemoteOutput_MultipleBranches verifies multiple agent branches are all parsed.
func TestParseLsRemoteOutput_MultipleBranches(t *testing.T) {
	t.Parallel()

	raw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/agent/atlas/TAL-5\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/agent/hermes/TAL-6\n" +
		"cccccccccccccccccccccccccccccccccccccccc\trefs/heads/agent/themis/TAL-16\n"

	entries, err := gitremote.ParseLsRemoteOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	cases := []struct {
		figura string
		branch string
	}{
		{"atlas", "origin/agent/atlas/TAL-5"},
		{"hermes", "origin/agent/hermes/TAL-6"},
		{"themis", "origin/agent/themis/TAL-16"},
	}
	for i, c := range cases {
		if entries[i].Figura != c.figura {
			t.Errorf("entries[%d].Figura = %q, want %q", i, entries[i].Figura, c.figura)
		}
		if entries[i].Branch != c.branch {
			t.Errorf("entries[%d].Branch = %q, want %q", i, entries[i].Branch, c.branch)
		}
		if entries[i].Status != "active" {
			t.Errorf("entries[%d].Status = %q, want %q", i, entries[i].Status, "active")
		}
	}
}

// TestParseLsRemoteOutput_SkipsNonAgentBranches verifies non-agent/* refs are ignored.
func TestParseLsRemoteOutput_SkipsNonAgentBranches(t *testing.T) {
	t.Parallel()

	raw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/main\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/develop\n" +
		"cccccccccccccccccccccccccccccccccccccccc\trefs/heads/agent/atlas/TAL-5\n"

	entries, err := gitremote.ParseLsRemoteOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only agent/*), got %d", len(entries))
	}
	if entries[0].Figura != "atlas" {
		t.Errorf("Figura = %q, want %q", entries[0].Figura, "atlas")
	}
}

// TestParseLsRemoteOutput_MalformedLine_Skipped verifies that lines without a tab separator are skipped gracefully.
func TestParseLsRemoteOutput_MalformedLine_Skipped(t *testing.T) {
	t.Parallel()

	raw := "not-a-valid-line\n" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/agent/atlas/TAL-5\n"

	entries, err := gitremote.ParseLsRemoteOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
}

// TestParseLsRemoteOutput_BranchTooShort_Skipped verifies that refs/heads/agent with fewer than 3 parts are skipped.
func TestParseLsRemoteOutput_BranchTooShort_Skipped(t *testing.T) {
	t.Parallel()

	// refs/heads/agent → only 3 segments after split → no figura/ticket — skip
	raw := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/agent\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/agent/atlas/TAL-5\n"

	entries, err := gitremote.ParseLsRemoteOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (short branch skipped), got %d", len(entries))
	}
}

// TestLister_List_UsesInjectedRunner verifies the Lister calls ls-remote with the right args
// and returns parsed entries when the runner returns valid output.
func TestLister_List_UsesInjectedRunner(t *testing.T) {
	t.Parallel()

	raw := "abc1234567890123456789012345678901234567890\trefs/heads/agent/atlas/TAL-5\n"

	var capturedArgs []string
	runner := func(_ context.Context, args []string) (string, error) {
		capturedArgs = args
		return raw, nil
	}

	lister := gitremote.NewListerWithRunner(runner)
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify runner received ls-remote invocation
	if len(capturedArgs) < 3 {
		t.Fatalf("expected at least 3 args to runner, got %v", capturedArgs)
	}
	if capturedArgs[0] != "ls-remote" {
		t.Errorf("args[0] = %q, want %q", capturedArgs[0], "ls-remote")
	}
	if capturedArgs[1] != "--heads" {
		t.Errorf("args[1] = %q, want %q", capturedArgs[1], "--heads")
	}
	if capturedArgs[2] != "origin" {
		t.Errorf("args[2] = %q, want %q", capturedArgs[2], "origin")
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Figura != "atlas" {
		t.Errorf("entries[0].Figura = %q, want %q", entries[0].Figura, "atlas")
	}
}

// TestLister_List_RunnerError propagates runner errors.
func TestLister_List_RunnerError(t *testing.T) {
	t.Parallel()

	runner := func(_ context.Context, _ []string) (string, error) {
		return "", &gitremote.ErrLsRemoteFailed{Cause: errSentinel("git ls-remote failed")}
	}

	lister := gitremote.NewListerWithRunner(runner)
	_, err := lister.List(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var lsErr *gitremote.ErrLsRemoteFailed
	if !isLsRemoteErr(err, &lsErr) {
		t.Errorf("expected *gitremote.ErrLsRemoteFailed, got %T: %v", err, err)
	}
}

// TestLister_InterfaceCompliance is a compile-time assertion that *Lister satisfies port.WorktreeLister.
var _ port.WorktreeLister = (*gitremote.Lister)(nil)

// --- TAL-16 fix B (--branches): explicit open-PR branch set overrides git ls-remote. ---
// `git ls-remote agent/*` returns stale squash-merged branches → false positives. The CI knows
// the in-flight set (`gh pr list --state open`) and passes it via --branches; this lister builds
// entries from that list directly, bypassing git, so stale branches are excluded by construction.

// TestNewListerFromBranches_List_BuildsEntriesWithoutGit verifies the explicit-branch lister
// returns ls-remote-shaped entries (Figura, origin/-prefixed Branch, Status=active) and never
// shells out (a nil runner would panic if the git path were taken).
func TestNewListerFromBranches_List_BuildsEntriesWithoutGit(t *testing.T) {
	t.Parallel()

	lister := gitremote.NewListerFromBranches([]string{"agent/themis/TAL-16", "agent/atlas/TAL-5"})
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []port.WorktreeEntry{
		{Figura: "themis", Branch: "origin/agent/themis/TAL-16", Status: "active"},
		{Figura: "atlas", Branch: "origin/agent/atlas/TAL-5", Status: "active"},
	}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d (%+v)", len(want), len(entries), entries)
	}
	for i, w := range want {
		if entries[i].Figura != w.Figura || entries[i].Branch != w.Branch || entries[i].Status != w.Status {
			t.Errorf("entries[%d] = %+v, want %+v", i, entries[i], w)
		}
	}
}

// TestNewListerFromBranches_SkipsNonAgentAndBlank verifies non-agent refs, blanks and malformed
// branch names are dropped — only well-formed agent/<figura>/<ticket> survive.
func TestNewListerFromBranches_SkipsNonAgentAndBlank(t *testing.T) {
	t.Parallel()

	lister := gitremote.NewListerFromBranches([]string{
		"agent/atlas/TAL-5", "main", "", "   ", "feature/x", "agent/incomplete",
	})
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid agent entry, got %d (%+v)", len(entries), entries)
	}
	if entries[0].Figura != "atlas" {
		t.Errorf("Figura = %q, want %q", entries[0].Figura, "atlas")
	}
}

// TestNewListerFromBranches_Empty_NoEntries verifies an empty in-flight set yields zero claims
// (no open PRs → no collision, exit 0 via ErrNoClaims upstream).
func TestNewListerFromBranches_Empty_NoEntries(t *testing.T) {
	t.Parallel()

	lister := gitremote.NewListerFromBranches(nil)
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty list, got %d", len(entries))
	}
}

// helpers

func errSentinel(msg string) error {
	return fmt.Errorf("%s", msg)
}

func isLsRemoteErr(err error, target **gitremote.ErrLsRemoteFailed) bool {
	var e *gitremote.ErrLsRemoteFailed
	ok := errors.As(err, &e)
	if ok && target != nil {
		*target = e
	}
	return ok
}
