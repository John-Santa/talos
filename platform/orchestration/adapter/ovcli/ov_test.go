package ovcli_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/adapter/ovcli"
	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
)

// fakeRunner captures calls and returns canned output.
type fakeRunner struct {
	out []byte
	err error
	// captured
	lastArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.lastArgs = append([]string{name}, args...)
	return f.out, f.err
}

func ovJSON(verdict string) []byte {
	b, _ := json.Marshal(map[string]any{
		"verdict":         verdict,
		"collision_rate":  0.0,
		"pairs_evaluated": 1,
		"colliding_pairs": 0,
	})
	return b
}

// TestCheck_OK verifies the adapter parses OK verdict correctly.
func TestCheck_OK(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: ovJSON("OK")}
	a := ovcli.NewChecker("ov", r)

	v, err := a.Check(context.Background(), "module:orchestration", "hephaestus")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if v != dispatch.VerdictOK {
		t.Errorf("verdict = %q, want %q", v, dispatch.VerdictOK)
	}
}

// TestCheck_BLOCK verifies the adapter parses BLOCK verdict correctly.
func TestCheck_BLOCK(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: ovJSON("BLOCK")}
	a := ovcli.NewChecker("ov", r)

	v, err := a.Check(context.Background(), "module:orchestration", "hephaestus")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if v != dispatch.VerdictBlock {
		t.Errorf("verdict = %q, want %q", v, dispatch.VerdictBlock)
	}
}

// TestCheck_SERIALIZE verifies the adapter parses SERIALIZE verdict correctly.
func TestCheck_SERIALIZE(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: ovJSON("SERIALIZE")}
	a := ovcli.NewChecker("ov", r)

	v, err := a.Check(context.Background(), "module:orchestration", "hephaestus")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if v != dispatch.VerdictSerialize {
		t.Errorf("verdict = %q, want %q", v, dispatch.VerdictSerialize)
	}
}

// TestCheck_MalformedJSON verifies error is returned on bad output.
func TestCheck_MalformedJSON(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("not json at all")}
	a := ovcli.NewChecker("ov", r)

	_, err := a.Check(context.Background(), "module:orchestration", "hephaestus")
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

// TestCheck_BinaryArgs verifies the correct flags are passed to ov.
func TestCheck_BinaryArgs(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: ovJSON("OK")}
	a := ovcli.NewChecker("ov", r)

	_, err := a.Check(context.Background(), "module:x", "atlas")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}

	// Verify the runner was called with the right binary and flags
	if len(r.lastArgs) == 0 || r.lastArgs[0] != "ov" {
		t.Errorf("expected binary 'ov', got %v", r.lastArgs)
	}
	foundModule := false
	foundAgent := false
	for i, a := range r.lastArgs {
		if a == "--module" && i+1 < len(r.lastArgs) && r.lastArgs[i+1] == "module:x" {
			foundModule = true
		}
		if a == "--agent" && i+1 < len(r.lastArgs) && r.lastArgs[i+1] == "atlas" {
			foundAgent = true
		}
	}
	if !foundModule {
		t.Errorf("--module flag not found in args: %v", r.lastArgs)
	}
	if !foundAgent {
		t.Errorf("--agent flag not found in args: %v", r.lastArgs)
	}
}
