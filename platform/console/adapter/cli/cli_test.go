package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/console/adapter/cli"
)

// singleLineJSON returns a one-liner JSON fixture matching wt list --json output.
func singleLineJSON() string {
	return `[{"figura":"hermes","branch":"agent/hermes/TAL-3","path":"/tmp/t","head":"abc123","status":"active"}]`
}

func TestReader_Worktrees_happyPath(t *testing.T) {
	// t.Parallel() is intentionally absent: t.Setenv requires sequential execution.

	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '" + singleLineJSON() + "'\n"
	scriptPath := filepath.Join(dir, "wt")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	worktrees, err := r.Worktrees(context.Background())
	if err != nil {
		t.Fatalf("Worktrees: unexpected error: %v", err)
	}
	if len(worktrees) != 1 {
		t.Fatalf("want 1 worktree, got %d", len(worktrees))
	}
	if worktrees[0].Figura != "hermes" {
		t.Errorf("Figura = %q, want hermes", worktrees[0].Figura)
	}
	if worktrees[0].Branch != "agent/hermes/TAL-3" {
		t.Errorf("Branch = %q, want agent/hermes/TAL-3", worktrees[0].Branch)
	}
	if worktrees[0].Path != "/tmp/t" {
		t.Errorf("Path = %q, want /tmp/t", worktrees[0].Path)
	}
	if worktrees[0].Head != "abc123" {
		t.Errorf("Head = %q, want abc123", worktrees[0].Head)
	}
	if worktrees[0].Status != "active" {
		t.Errorf("Status = %q, want active", worktrees[0].Status)
	}
}

func TestReader_Worktrees_malformedJSON(t *testing.T) {
	// t.Parallel() is intentionally absent: t.Setenv requires sequential execution.

	dir := t.TempDir()
	script := "#!/bin/sh\nprintf 'not-json'\n"
	scriptPath := filepath.Join(dir, "wt")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	_, err := r.Worktrees(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
	if !errors.Is(err, cli.ErrOutputMalformed) {
		t.Errorf("want errors.Is(err, ErrOutputMalformed); got %v", err)
	}
}

func TestReader_Worktrees_binaryNotFound(t *testing.T) {
	t.Parallel()

	r := cli.NewReader("wt-does-not-exist-console-test-only", "mo", "ov", "ch")
	_, err := r.Worktrees(context.Background())
	if err == nil {
		t.Fatal("expected error when wt binary missing, got nil")
	}
	if !errors.Is(err, cli.ErrBinaryNotFound) {
		t.Errorf("want errors.Is(err, ErrBinaryNotFound); got %v", err)
	}
}

// mergePlanJSON returns a fixture matching mo plan --json output.
func mergePlanJSON() string {
	return `{"base_branch":"develop","base_tip":"abc123","conflict_rate":0.05,"threshold":0.15,"segmentation_bad":false,"steps":[{"position":1,"branch":"agent/hermes/TAL-3","figura":"hermes","commits_ahead":2,"predicted_clean":true}]}`
}

// overlapJSON returns a fixture matching ov scan --json output.
func overlapJSON() string {
	return `{"verdict":"OK","collision_rate":0.0,"pairs_evaluated":1,"colliding_pairs":0,"file_collisions":[],"module_overlaps":[],"advisories":[]}`
}

// labelsJSON returns a fixture matching ch labels --branch <branch> --json output.
func labelsJSON() string {
	return `{"branch":"agent/hermes/TAL-3","jira_key":"TAL-3","figura":"hermes","verdict":"OK","labels":["agent:hermes","module:console"],"agent":"hermes","module":"console","violations":[]}`
}

// writeFakeBinary writes a shell script that outputs the given payload to stdout.
func writeFakeBinary(t *testing.T, dir, name, payload string) {
	t.Helper()
	script := "#!/bin/sh\nprintf '" + payload + "'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeMalformedBinary writes a shell script that outputs non-JSON garbage.
func writeMalformedBinary(t *testing.T, dir, name string) {
	t.Helper()
	script := "#!/bin/sh\nprintf 'not-json'\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestReader_MergePlan_happyPath(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeFakeBinary(t, dir, "mo", mergePlanJSON())
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	plan, err := r.MergePlan(context.Background())
	if err != nil {
		t.Fatalf("MergePlan: unexpected error: %v", err)
	}
	if plan.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want develop", plan.BaseBranch)
	}
	if plan.BaseTip != "abc123" {
		t.Errorf("BaseTip = %q, want abc123", plan.BaseTip)
	}
	if plan.ConflictRate != 0.05 {
		t.Errorf("ConflictRate = %f, want 0.05", plan.ConflictRate)
	}
	if plan.SegmentationBad {
		t.Errorf("SegmentationBad = true, want false")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Figura != "hermes" {
		t.Errorf("step Figura = %q, want hermes", plan.Steps[0].Figura)
	}
	if plan.Steps[0].Position != 1 {
		t.Errorf("step Position = %d, want 1", plan.Steps[0].Position)
	}
}

func TestReader_MergePlan_malformedJSON(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeMalformedBinary(t, dir, "mo")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	_, err := r.MergePlan(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
	if !errors.Is(err, cli.ErrOutputMalformed) {
		t.Errorf("want errors.Is(err, ErrOutputMalformed); got %v", err)
	}
}

func TestReader_Overlap_happyPath(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeFakeBinary(t, dir, "ov", overlapJSON())
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	ov, err := r.Overlap(context.Background())
	if err != nil {
		t.Fatalf("Overlap: unexpected error: %v", err)
	}
	if ov.Verdict != "OK" {
		t.Errorf("Verdict = %q, want OK", ov.Verdict)
	}
	if ov.CollisionRate != 0.0 {
		t.Errorf("CollisionRate = %f, want 0.0", ov.CollisionRate)
	}
	if ov.PairsEvaluated != 1 {
		t.Errorf("PairsEvaluated = %d, want 1", ov.PairsEvaluated)
	}
	if ov.CollidingPairs != 0 {
		t.Errorf("CollidingPairs = %d, want 0", ov.CollidingPairs)
	}
}

func TestReader_Overlap_malformedJSON(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeMalformedBinary(t, dir, "ov")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	_, err := r.Overlap(context.Background())
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
	if !errors.Is(err, cli.ErrOutputMalformed) {
		t.Errorf("want errors.Is(err, ErrOutputMalformed); got %v", err)
	}
}

func TestReader_Labels_happyPath(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	// The fake ch binary ignores its args and just prints the fixture.
	writeFakeBinary(t, dir, "ch", labelsJSON())
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	labels, err := r.Labels(context.Background(), "agent/hermes/TAL-3")
	if err != nil {
		t.Fatalf("Labels: unexpected error: %v", err)
	}
	if labels.Branch != "agent/hermes/TAL-3" {
		t.Errorf("Branch = %q, want agent/hermes/TAL-3", labels.Branch)
	}
	if labels.JiraKey != "TAL-3" {
		t.Errorf("JiraKey = %q, want TAL-3", labels.JiraKey)
	}
	if labels.Figura != "hermes" {
		t.Errorf("Figura = %q, want hermes", labels.Figura)
	}
	if labels.Verdict != "OK" {
		t.Errorf("Verdict = %q, want OK", labels.Verdict)
	}
	if labels.Agent != "hermes" {
		t.Errorf("Agent = %q, want hermes", labels.Agent)
	}
	if labels.Module != "console" {
		t.Errorf("Module = %q, want console", labels.Module)
	}
}

func TestReader_Labels_malformedJSON(t *testing.T) {
	// t.Parallel() intentionally absent: t.Setenv incompatible with t.Parallel in Go 1.26.

	dir := t.TempDir()
	writeMalformedBinary(t, dir, "ch")
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	r := cli.NewReader("wt", "mo", "ov", "ch")
	_, err := r.Labels(context.Background(), "agent/hermes/TAL-3")
	if err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
	if !errors.Is(err, cli.ErrOutputMalformed) {
		t.Errorf("want errors.Is(err, ErrOutputMalformed); got %v", err)
	}
}
