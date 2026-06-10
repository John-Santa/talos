package runscli_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/adapter/runscli"
	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
)

// fakeRunner captures calls to the injected runner for inspection.
type fakeRunner struct {
	calls []struct {
		name string
		args []string
	}
	err error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, struct {
		name string
		args []string
	}{name, args})
	return nil, f.err
}

func (f *fakeRunner) lastArgs() []string {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1].args
}

func (f *fakeRunner) argMap() map[string]string {
	args := f.lastArgs()
	m := make(map[string]string)
	for i := 0; i < len(args)-1; i++ {
		if strings.HasPrefix(args[i], "--") {
			m[args[i]] = args[i+1]
		}
	}
	return m
}

func testItem() dispatch.WorkItem {
	return dispatch.WorkItem{
		JiraKey: "TAL-42",
		Change:  "orchestration",
		Agent:   "hephaestus",
		Module:  "module:orchestration",
	}
}

// TestRecordDispatch_Running verifies the correct args are passed to `runs record`.
func TestRecordDispatch_Running(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	rec := runscli.NewRecorder("runs", r)

	if err := rec.RecordDispatch(context.Background(), testItem(), "propose", "running"); err != nil {
		t.Fatalf("RecordDispatch: %v", err)
	}

	if len(r.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(r.calls))
	}
	args := r.argMap()
	if args["--kind"] != "dispatch" {
		t.Errorf("--kind = %q, want %q", args["--kind"], "dispatch")
	}
	if args["--jira-key"] != "TAL-42" {
		t.Errorf("--jira-key = %q, want %q", args["--jira-key"], "TAL-42")
	}
	if args["--agent"] != "hephaestus" {
		t.Errorf("--agent = %q, want %q", args["--agent"], "hephaestus")
	}
	if args["--phase"] != "propose" {
		t.Errorf("--phase = %q, want %q", args["--phase"], "propose")
	}
	if args["--status"] != "running" {
		t.Errorf("--status = %q, want %q", args["--status"], "running")
	}
}

// TestRecordActivity verifies correct args for activity events.
func TestRecordActivity(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	rec := runscli.NewRecorder("runs", r)

	if err := rec.RecordActivity(context.Background(), "TAL-42", "hephaestus", "worktree ready"); err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}

	args := r.argMap()
	if args["--kind"] != "activity" {
		t.Errorf("--kind = %q, want %q", args["--kind"], "activity")
	}
	if args["--text"] != "worktree ready" {
		t.Errorf("--text = %q, want %q", args["--text"], "worktree ready")
	}
}

// TestRecordMetric verifies correct args for metric events.
func TestRecordMetric(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{}
	rec := runscli.NewRecorder("runs", r)

	if err := rec.RecordMetric(context.Background(), "TAL-42", "conflict_rate", 0.42); err != nil {
		t.Fatalf("RecordMetric: %v", err)
	}

	args := r.argMap()
	if args["--kind"] != "metric" {
		t.Errorf("--kind = %q, want %q", args["--kind"], "metric")
	}
	if args["--metric"] != "conflict_rate" {
		t.Errorf("--metric = %q, want %q", args["--metric"], "conflict_rate")
	}
	// value should be present
	if _, ok := args["--value"]; !ok {
		t.Error("--value flag missing from RecordMetric call")
	}
}

// TestRunnerError_ReturnedAsIs verifies that runner errors are returned (caller handles fail-soft).
func TestRunnerError_ReturnedAsIs(t *testing.T) {
	t.Parallel()
	sentinel := fmt.Errorf("runs: binary not found")
	r := &fakeRunner{err: sentinel}
	rec := runscli.NewRecorder("runs", r)

	err := rec.RecordActivity(context.Background(), "TAL-42", "hephaestus", "test")
	if err == nil {
		t.Fatal("expected error from runner, got nil")
	}
}
