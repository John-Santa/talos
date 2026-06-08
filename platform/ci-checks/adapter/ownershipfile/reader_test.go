package ownershipfile_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/adapter/ownershipfile"
)

func fixturePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", "ownership.md")
}

// TestReader_Ownership_OK verifies that the fixture parses to the expected lowercase map.
func TestReader_Ownership_OK(t *testing.T) {
	r := ownershipfile.NewReader(fixturePath())
	m, err := r.Ownership(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "hermes"
	got, ok := m["module:devops"]
	if !ok {
		t.Fatalf("module:devops not found in map; map=%v", m)
	}
	if got != want {
		t.Errorf("module:devops = %q, want %q", got, want)
	}
}

// TestReader_Ownership_Lowercase verifies all values are lowercase.
func TestReader_Ownership_Lowercase(t *testing.T) {
	r := ownershipfile.NewReader(fixturePath())
	m, err := r.Ownership(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for k, v := range m {
		if v != strings.ToLower(v) {
			t.Errorf("value for %q = %q is not lowercase", k, v)
		}
	}
}

// TestReader_Ownership_SkipsSecondTable verifies entries from the second table are excluded.
func TestReader_Ownership_SkipsSecondTable(t *testing.T) {
	r := ownershipfile.NewReader(fixturePath())
	m, err := r.Ownership(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The second table has path-based keys; none of them should appear in the map.
	for k := range m {
		if strings.HasPrefix(k, "platform/") {
			t.Errorf("second-table key %q must not appear in module map", k)
		}
	}
}

// TestReader_Ownership_FileNotFound verifies that a missing file returns an error.
func TestReader_Ownership_FileNotFound(t *testing.T) {
	r := ownershipfile.NewReader("/no/such/file/ownership.md")
	_, err := r.Ownership(context.Background())
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestReader_Integration_RealOwnership uses the real ownership.md from the repo; skipped in short mode.
func TestReader_Integration_RealOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — reads real team-context/ownership.md")
	}
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	path := filepath.Join(repoRoot, "team-context", "ownership.md")
	r := ownershipfile.NewReader(path)
	m, err := r.Ownership(context.Background())
	if err != nil {
		t.Fatalf("unexpected error reading real ownership.md: %v", err)
	}
	for _, k := range []string{"module:devops", "module:qa"} {
		if _, ok := m[k]; !ok {
			t.Errorf("expected module %q in real ownership map", k)
		}
	}
}
