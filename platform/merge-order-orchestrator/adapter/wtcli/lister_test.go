package wtcli_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/adapter/wtcli"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// golden JSON fixture matching the wt list --json output shape
var goldenJSON = `[
  {
    "figura": "hermes",
    "branch": "agent/hermes/TAL-3",
    "path": "/tmp/talos.wt/hermes/TAL-3",
    "head": "abc1234def5678901234567890123456789abcde",
    "status": "active"
  },
  {
    "figura": "atlas",
    "branch": "agent/atlas/TAL-1",
    "path": "/tmp/talos.wt/atlas/TAL-1",
    "head": "bcd2345ef6789012345678901234567890bcdef",
    "status": "idle"
  }
]`

func TestLister_Decode_golden(t *testing.T) {
	// unit test: decode golden fixture, no real binary needed
	var entries []port.WorktreeEntry
	if err := json.Unmarshal([]byte(goldenJSON), &entries); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Figura != "hermes" {
		t.Errorf("entries[0].Figura = %q, want hermes", entries[0].Figura)
	}
	if entries[0].Branch != "agent/hermes/TAL-3" {
		t.Errorf("entries[0].Branch = %q, want agent/hermes/TAL-3", entries[0].Branch)
	}
	if entries[0].Status != "active" {
		t.Errorf("entries[0].Status = %q, want active", entries[0].Status)
	}
	if entries[1].Figura != "atlas" {
		t.Errorf("entries[1].Figura = %q, want atlas", entries[1].Figura)
	}
}

func TestLister_List_fakeWt(t *testing.T) {
	// unit test: provide a fake wt binary via a temp script and verify List decodes output
	// build a small shell script that acts as "wt list --json"
	dir := t.TempDir()

	// write the fake wt script
	script := "#!/bin/sh\nprintf '" + singleLineJSON() + "'\n"
	scriptPath := filepath.Join(dir, "wt")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// prepend dir to PATH so our fake wt is found first
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	lister := wtcli.NewLister("wt")
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Figura != "hermes" {
		t.Errorf("entry.Figura = %q, want hermes", entries[0].Figura)
	}
	if entries[0].Status != "active" {
		t.Errorf("entry.Status = %q, want active", entries[0].Status)
	}
}

func singleLineJSON() string {
	return `[{"figura":"hermes","branch":"agent/hermes/TAL-3","path":"/tmp/t","head":"abc123","status":"active"}]`
}

func TestLister_List_wtNotOnPath(t *testing.T) {
	// unit test: wt binary missing → ErrWtBinaryNotFound
	// use a binary name that definitely won't exist
	lister := wtcli.NewLister("wt-does-not-exist-mo-test-only")
	_, err := lister.List(context.Background())
	if err == nil {
		t.Fatal("expected error when wt binary missing, got nil")
	}
}

func TestLister_List_malformedOutput(t *testing.T) {
	// unit test: wt emits garbage JSON → ErrWtOutputMalformed
	dir := t.TempDir()

	script := "#!/bin/sh\nprintf 'not-json'\n"
	scriptPath := filepath.Join(dir, "wt")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	lister := wtcli.NewLister("wt")
	_, err := lister.List(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestLister_List_realWt(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real wt binary")
	}
	_, err := exec.LookPath("wt")
	if err != nil {
		t.Skip("wt not on PATH — skipping real-wt integration test")
	}

	lister := wtcli.NewLister("wt")
	entries, err := lister.List(context.Background())
	if err != nil {
		t.Fatalf("List with real wt: %v", err)
	}
	// just verify it returns a slice (possibly empty) without error
	t.Logf("real wt returned %d entries", len(entries))
	_ = entries
}
