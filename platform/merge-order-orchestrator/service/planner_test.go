package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/mock"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/service"
)

func defaultCfg() service.Config {
	return service.DefaultTALConfig()
}

func makeActiveEntry(figura, branch, path string) port.WorktreeEntry {
	return port.WorktreeEntry{
		Figura: figura,
		Branch: branch,
		Path:   path,
		Head:   "abc123",
		Status: "active",
	}
}

func makeOrphanEntry(figura, branch string) port.WorktreeEntry {
	return port.WorktreeEntry{
		Figura: figura,
		Branch: branch,
		Status: "orphan",
	}
}

func TestPlanner_Plan_HappyPath(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lister.ListResult = []port.WorktreeEntry{
		makeActiveEntry("atlas", "agent/atlas/TAL-1", "talos.wt/agent-atlas"),
		makeActiveEntry("hermes", "agent/hermes/TAL-3", "talos.wt/agent-hermes"),
		makeOrphanEntry("iris", "agent/iris/TAL-5"),
	}
	inspector.RevParseResults = map[string]string{
		"develop":              "dev-tip-sha",
		"agent/atlas/TAL-1":   "atlas-sha",
		"agent/hermes/TAL-3":  "hermes-sha",
	}
	inspector.CommitsAheadByBranch = map[string]int{
		"agent/atlas/TAL-1":  2,
		"agent/hermes/TAL-3": 0,
	}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1": {"a.go"},
	}
	// Set up CreatedAt via RevParse (atlas branch has ahead commits, hermes is dropped)
	inspector.RevParseResults["agent/atlas/TAL-1"] = "atlas-sha"

	cfg := defaultCfg()
	planner := service.NewPlanner(inspector, lister, cfg)

	report, err := planner.Plan(context.Background(), nil)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	// orphan dropped, hermes dropped (0 commits ahead) → 1 candidate: atlas
	if len(report.Plan.Steps) != 1 {
		t.Errorf("Steps len = %d, want 1", len(report.Plan.Steps))
	}

	// Verify call sequence: List → Fetch → RevParse(develop) → CommitsAhead × active branches → ChangedFiles × candidates-ahead → MergeTreeConflicts × steps
	calls := inspector.Calls
	if len(calls) == 0 {
		t.Fatal("inspector was not called")
	}

	// First inspector call after List should be Fetch
	if calls[0].Method != "Fetch" {
		t.Errorf("first inspector call = %q, want Fetch", calls[0].Method)
	}
	// Lister was called
	lister.AssertCallCount(t, "List", 1)

	_ = t0
}

func TestPlanner_Plan_NoFetch(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	lister.ListResult = []port.WorktreeEntry{
		makeActiveEntry("atlas", "agent/atlas/TAL-1", "talos.wt/agent-atlas"),
	}
	inspector.RevParseResults = map[string]string{
		"develop":            "dev-tip",
		"agent/atlas/TAL-1": "atlas-sha",
	}
	inspector.CommitsAheadByBranch = map[string]int{
		"agent/atlas/TAL-1": 1,
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	_, err := planner.Plan(context.Background(), nil)
	if err != nil {
		t.Fatalf("Plan(NoFetch) unexpected error: %v", err)
	}
	inspector.AssertNotCalled(t, "Fetch")
}

func TestPlanner_Plan_EmptyInventory_ErrNoCandidates(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	// All orphans
	lister.ListResult = []port.WorktreeEntry{
		makeOrphanEntry("atlas", "agent/atlas/TAL-1"),
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	_, err := planner.Plan(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ErrNoCandidates, got nil")
	}
	var e *mergeorder.ErrNoCandidates
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrNoCandidates", err)
	}
}

func TestPlanner_Plan_CycleInDeps(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	lister.ListResult = []port.WorktreeEntry{
		makeActiveEntry("atlas", "feat/a", "wt/a"),
		makeActiveEntry("hermes", "feat/b", "wt/b"),
	}
	inspector.RevParseResults = map[string]string{
		"develop": "dev-tip",
		"feat/a":  "sha-a",
		"feat/b":  "sha-b",
	}
	inspector.CommitsAheadByBranch = map[string]int{
		"feat/a": 2,
		"feat/b": 2,
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	deps := map[string][]string{
		"feat/a": {"feat/b"},
		"feat/b": {"feat/a"},
	}
	_, err := planner.Plan(context.Background(), deps)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle, got nil")
	}
	var e *mergeorder.ErrDependencyCycle
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrDependencyCycle", err)
	}
}

func TestPlanner_Plan_SegmentationBad(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	lister.ListResult = []port.WorktreeEntry{
		makeActiveEntry("atlas", "feat/a", "wt/a"),
	}
	inspector.RevParseResults = map[string]string{
		"develop": "dev-tip",
		"feat/a":  "sha-a",
	}
	inspector.CommitsAheadByBranch = map[string]int{"feat/a": 2}
	// Non-nil slice means conflict
	inspector.MergeTreeConflictsByBranch = map[string][]string{
		"feat/a": {"conflict.go"},
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	report, err := planner.Plan(context.Background(), nil)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}
	if !report.SegmentationBad {
		t.Errorf("SegmentationBad = false, want true (conflict rate 1.0 > 0.15)")
	}
}

func TestPlanner_Check_CleanBranch(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	inspector.RevParseResults = map[string]string{
		"develop": "dev-tip",
		"feat/x":  "sha-x",
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	step, err := planner.Check(context.Background(), "feat/x")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if !step.PredictedClean {
		t.Errorf("PredictedClean = false, want true")
	}
	lister.AssertNotCalled(t, "List")
}

func TestPlanner_Check_ConflictingBranch(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	inspector.RevParseResults = map[string]string{
		"develop": "dev-tip",
		"feat/x":  "sha-x",
	}
	inspector.MergeTreeConflictsByBranch = map[string][]string{
		"feat/x": {"conflict.go"},
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	step, err := planner.Check(context.Background(), "feat/x")
	if err != nil {
		t.Fatalf("Check() unexpected error: %v", err)
	}
	if step.PredictedClean {
		t.Errorf("PredictedClean = true, want false")
	}
	if len(step.ConflictFiles) == 0 {
		t.Errorf("ConflictFiles is empty, want at least one file")
	}
}

func TestPlanner_Check_UnknownBranch_ErrBranchNotFound(t *testing.T) {
	t.Parallel()
	inspector := mock.NewGitInspectorMock()
	lister := mock.NewWorktreeListerMock()

	inspector.RevParseErrs = map[string]error{
		"feat/missing": mock.ErrSentinel("unknown revision"),
	}
	inspector.RevParseResults = map[string]string{
		"develop": "dev-tip",
	}

	cfg := defaultCfg()
	cfg.NoFetch = true
	planner := service.NewPlanner(inspector, lister, cfg)

	_, err := planner.Check(context.Background(), "feat/missing")
	if err == nil {
		t.Fatal("expected error for unknown branch, got nil")
	}
	var e *mergeorder.ErrBranchNotFound
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrBranchNotFound", err)
	}
}
