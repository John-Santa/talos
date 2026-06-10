package mocli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/adapter/mocli"
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

func moPlanJSON(conflictRate, threshold float64, clean bool) []byte {
	b, _ := json.Marshal(map[string]any{
		"base_branch":      "develop",
		"base_tip":         "abc123",
		"conflict_rate":    conflictRate,
		"threshold":        threshold,
		"segmentation_bad": !clean,
		"steps":            []any{},
	})
	return b
}

// TestPlan_Clean verifies clean plan parsing.
func TestPlan_Clean(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: moPlanJSON(0.0, 0.3, true)}
	m := mocli.NewCoordinator("mo", r)

	plan, err := m.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if !plan.Clean {
		t.Error("plan.Clean should be true")
	}
	if plan.ConflictRate != 0.0 {
		t.Errorf("ConflictRate = %f, want 0.0", plan.ConflictRate)
	}
	if plan.Threshold != 0.3 {
		t.Errorf("Threshold = %f, want 0.3", plan.Threshold)
	}
}

// TestPlan_Dirty verifies dirty plan is correctly reflected.
func TestPlan_Dirty(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: moPlanJSON(0.8, 0.3, false)}
	m := mocli.NewCoordinator("mo", r)

	plan, err := m.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if plan.Clean {
		t.Error("plan.Clean should be false for dirty plan")
	}
	if plan.ConflictRate != 0.8 {
		t.Errorf("ConflictRate = %f, want 0.8", plan.ConflictRate)
	}
}

// TestPlan_MalformedJSON verifies error on bad output.
func TestPlan_MalformedJSON(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("not json")}
	m := mocli.NewCoordinator("mo", r)

	_, err := m.Plan(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

// TestPlan_BinaryArgs verifies correct flags passed to mo.
func TestPlan_BinaryArgs(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: moPlanJSON(0.0, 0.3, true)}
	m := mocli.NewCoordinator("mo", r)

	_, _ = m.Plan(context.Background())

	if len(r.lastArgs) == 0 || r.lastArgs[0] != "mo" {
		t.Errorf("expected binary 'mo', got %v", r.lastArgs)
	}
	if r.lastArgs[1] != "plan" {
		t.Errorf("expected subcommand 'plan', got %q", r.lastArgs[1])
	}
	foundJSON := false
	for _, a := range r.lastArgs {
		if a == "--json" {
			foundJSON = true
			break
		}
	}
	if !foundJSON {
		t.Errorf("--json flag not found in args: %v", r.lastArgs)
	}
}

// TestExecute_CallsMoExecute verifies the Execute method calls mo execute --yes.
func TestExecute_CallsMoExecute(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	m := mocli.NewCoordinator("mo", r)

	err := m.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if len(r.lastArgs) < 2 || r.lastArgs[1] != "execute" {
		t.Errorf("expected subcommand 'execute', got %v", r.lastArgs)
	}
	foundYes := false
	for _, a := range r.lastArgs {
		if a == "--yes" {
			foundYes = true
			break
		}
	}
	if !foundYes {
		t.Errorf("--yes flag not found in mo execute args: %v", r.lastArgs)
	}
}

// TestExecute_RunnerError verifies error propagation from Execute.
func TestExecute_RunnerError(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{err: fmt.Errorf("mo execute failed")}
	m := mocli.NewCoordinator("mo", r)

	err := m.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
