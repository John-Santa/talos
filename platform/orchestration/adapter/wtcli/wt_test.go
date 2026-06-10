package wtcli_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/adapter/wtcli"
)

type fakeRunner struct {
	out []byte
	err error
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.lastArgs = append([]string{name}, args...)
	return f.out, f.err
}

// TestEnsure_CallsCreate verifies wt create is called with the right args.
func TestEnsure_CallsCreate(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	m := wtcli.NewManager("wt", r)

	_, err := m.Ensure(context.Background(), "hephaestus", "TAL-42")
	if err != nil {
		t.Fatalf("Ensure() unexpected error: %v", err)
	}
	if len(r.lastArgs) == 0 || r.lastArgs[0] != "wt" {
		t.Errorf("expected binary 'wt', got %v", r.lastArgs)
	}
	if r.lastArgs[1] != "create" {
		t.Errorf("expected subcommand 'create', got %q", r.lastArgs[1])
	}
}

// TestEnsure_ReturnsPath verifies the returned path is deterministic.
func TestEnsure_ReturnsPath(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	m := wtcli.NewManager("wt", r)

	path, err := m.Ensure(context.Background(), "hephaestus", "TAL-42")
	if err != nil {
		t.Fatalf("Ensure() unexpected error: %v", err)
	}
	if path == "" {
		t.Error("Ensure() returned empty path")
	}
}

// TestEnsure_RunnerError verifies error propagation.
func TestEnsure_RunnerError(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{err: fmt.Errorf("wt create failed")}
	m := wtcli.NewManager("wt", r)

	_, err := m.Ensure(context.Background(), "hephaestus", "TAL-42")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestTeardown_CallsTeardown verifies wt teardown is called with the right args.
func TestTeardown_CallsTeardown(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	m := wtcli.NewManager("wt", r)

	err := m.Teardown(context.Background(), "hephaestus", false)
	if err != nil {
		t.Fatalf("Teardown() unexpected error: %v", err)
	}
	if r.lastArgs[1] != "teardown" {
		t.Errorf("expected subcommand 'teardown', got %q", r.lastArgs[1])
	}
}

// TestTeardown_Force verifies --force flag is passed when force=true.
func TestTeardown_Force(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	m := wtcli.NewManager("wt", r)

	err := m.Teardown(context.Background(), "hephaestus", true)
	if err != nil {
		t.Fatalf("Teardown(force) unexpected error: %v", err)
	}
	foundForce := false
	for _, a := range r.lastArgs {
		if a == "--force" {
			foundForce = true
			break
		}
	}
	if !foundForce {
		t.Errorf("--force flag not found in args: %v", r.lastArgs)
	}
}
