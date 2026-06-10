package evidencecli_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/adapter/evidencecli"
	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

type fakeRunner struct {
	out      []byte
	err      error
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.lastArgs = append([]string{name}, args...)
	return f.out, f.err
}

func testItem() dispatch.WorkItem {
	return dispatch.WorkItem{
		JiraKey: "TAL-42",
		Change:  "orchestration",
		Agent:   "hephaestus",
		Module:  "module:orchestration",
	}
}

// TestRunPhase_HappyPath verifies evidence run-loop is invoked with required flags.
func TestRunPhase_HappyPath(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	a := evidencecli.NewRunner("evidence", r)

	_, err := a.RunPhase(context.Background(), testItem(), "propose", port.EvidenceArgs{Summary: "test summary"})
	if err != nil {
		t.Fatalf("RunPhase() unexpected error: %v", err)
	}

	if len(r.lastArgs) == 0 || r.lastArgs[0] != "evidence" {
		t.Errorf("expected binary 'evidence', got %v", r.lastArgs)
	}
	if r.lastArgs[1] != "run-loop" {
		t.Errorf("expected subcommand 'run-loop', got %q", r.lastArgs[1])
	}

	hasFlag := func(flag, value string) bool {
		for i, a := range r.lastArgs {
			if a == flag && i+1 < len(r.lastArgs) && r.lastArgs[i+1] == value {
				return true
			}
		}
		return false
	}
	if !hasFlag("--phase", "propose") {
		t.Errorf("--phase propose not found in %v", r.lastArgs)
	}
	if !hasFlag("--change", "orchestration") {
		t.Errorf("--change orchestration not found in %v", r.lastArgs)
	}
	if !hasFlag("--agent", "hephaestus") {
		t.Errorf("--agent hephaestus not found in %v", r.lastArgs)
	}
	if !hasFlag("--module", "module:orchestration") {
		t.Errorf("--module module:orchestration not found in %v", r.lastArgs)
	}
}

// TestRunPhase_DryRun verifies --dry-run flag is forwarded.
func TestRunPhase_DryRun(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	a := evidencecli.NewRunner("evidence", r)

	_, err := a.RunPhase(context.Background(), testItem(), "propose", port.EvidenceArgs{DryRun: true})
	if err != nil {
		t.Fatalf("RunPhase(dry-run) unexpected error: %v", err)
	}

	foundDryRun := false
	for _, arg := range r.lastArgs {
		if arg == "--dry-run" {
			foundDryRun = true
			break
		}
	}
	if !foundDryRun {
		t.Errorf("--dry-run flag not found in %v", r.lastArgs)
	}
}

// TestRunPhase_WithJiraKey verifies --jira-key is forwarded when set.
func TestRunPhase_WithJiraKey(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	a := evidencecli.NewRunner("evidence", r)

	item := testItem()
	item.JiraKey = "TAL-99"
	_, err := a.RunPhase(context.Background(), item, "verify", port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("RunPhase(jira-key) unexpected error: %v", err)
	}

	hasFlag := func(flag, value string) bool {
		for i, a := range r.lastArgs {
			if a == flag && i+1 < len(r.lastArgs) && r.lastArgs[i+1] == value {
				return true
			}
		}
		return false
	}
	if !hasFlag("--jira-key", "TAL-99") {
		t.Errorf("--jira-key TAL-99 not found in %v", r.lastArgs)
	}
}

// TestRunPhase_Error verifies runner error is propagated.
func TestRunPhase_Error(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{err: fmt.Errorf("evidence crashed")}
	a := evidencecli.NewRunner("evidence", r)

	_, err := a.RunPhase(context.Background(), testItem(), "propose", port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestRecipe11_HappyPath verifies Recipe11 executes 3 steps.
func TestRecipe11_HappyPath(t *testing.T) {
	t.Parallel()
	callCount := 0
	calls := [][]string{}
	r := &multiCallRunner{onRun: func(name string, args []string) ([]byte, error) {
		callCount++
		calls = append(calls, append([]string{name}, args...))
		return []byte(""), nil
	}}
	rb := evidencecli.NewRollbacker("evidence", "wt", r)

	err := rb.Recipe11(context.Background(), "TAL-42", "hephaestus", "merge conflict detected")
	if err != nil {
		t.Fatalf("Recipe11() unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Recipe11 should make 3 runner calls, got %d", callCount)
	}
}

// multiCallRunner allows multiple independent runner invocations.
type multiCallRunner struct {
	onRun func(name string, args []string) ([]byte, error)
}

func (m *multiCallRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	return m.onRun(name, args)
}
