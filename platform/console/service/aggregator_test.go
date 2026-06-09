package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/mock"
	"github.com/John-Santa/talos/platform/console/service"
)

// --- fixtures ---

var sampleWorktrees = []platform.Worktree{
	{Figura: "hermes", Branch: "agent/hermes/TAL-3", Path: "/tmp/t", Head: "abc", Status: "active"},
}

var samplePlan = platform.MergePlan{
	BaseBranch:   "develop",
	BaseTip:      "abc123",
	ConflictRate: 0.05,
	Steps: []platform.MergePlanStep{
		{Position: 1, Branch: "agent/hermes/TAL-3", Figura: "hermes", CommitsAhead: 2, PredictedClean: true},
	},
}

var sampleOverlap = platform.Overlap{
	Verdict:        "OK",
	CollisionRate:  0.0,
	PairsEvaluated: 1,
	CollidingPairs: 0,
}

// --- tests ---

// TestAggregator_Snapshot_allOK verifies that when all three sources succeed,
// the snapshot is fully populated and Errors is empty.
func TestAggregator_Snapshot_allOK(t *testing.T) {
	t.Parallel()

	m := mock.NewPlatformReaderMock()
	m.WorktreesResult = sampleWorktrees
	m.MergePlanResult = samplePlan
	m.OverlapResult = sampleOverlap

	agg := service.NewAggregator(m)
	snap := agg.Snapshot(context.Background())

	if len(snap.Errors) != 0 {
		t.Errorf("Errors: want empty, got %v", snap.Errors)
	}
	if len(snap.Worktrees) != 1 {
		t.Errorf("Worktrees: want 1, got %d", len(snap.Worktrees))
	}
	if snap.Worktrees[0].Figura != "hermes" {
		t.Errorf("Worktrees[0].Figura = %q, want hermes", snap.Worktrees[0].Figura)
	}
	if snap.MergePlan.BaseBranch != "develop" {
		t.Errorf("MergePlan.BaseBranch = %q, want develop", snap.MergePlan.BaseBranch)
	}
	if snap.Overlap.Verdict != "OK" {
		t.Errorf("Overlap.Verdict = %q, want OK", snap.Overlap.Verdict)
	}

	// Confirm all three sources were called exactly once.
	m.AssertCallCount(t, "Worktrees", 1)
	m.AssertCallCount(t, "MergePlan", 1)
	m.AssertCallCount(t, "Overlap", 1)
	m.AssertNotCalled(t, "Labels")
}

// TestAggregator_Snapshot_worktreesFails verifies degrade-on-fail:
// when Worktrees errors, its error is captured in Errors["worktrees"],
// but MergePlan and Overlap are still populated.
func TestAggregator_Snapshot_worktreesFails(t *testing.T) {
	t.Parallel()

	errWt := errors.New("wt: binary not found")

	m := mock.NewPlatformReaderMock()
	m.WorktreesErr = errWt
	m.MergePlanResult = samplePlan
	m.OverlapResult = sampleOverlap

	agg := service.NewAggregator(m)
	snap := agg.Snapshot(context.Background())

	if snap.Errors["worktrees"] == nil {
		t.Errorf("expected Errors[worktrees] to be set")
	}
	if !errors.Is(snap.Errors["worktrees"], errWt) {
		t.Errorf("Errors[worktrees]: want %v, got %v", errWt, snap.Errors["worktrees"])
	}
	if len(snap.Worktrees) != 0 {
		t.Errorf("Worktrees: want empty on error, got %d entries", len(snap.Worktrees))
	}
	if snap.MergePlan.BaseBranch != "develop" {
		t.Errorf("MergePlan.BaseBranch = %q, want develop (others should still populate)", snap.MergePlan.BaseBranch)
	}
	if snap.Overlap.Verdict != "OK" {
		t.Errorf("Overlap.Verdict = %q, want OK (others should still populate)", snap.Overlap.Verdict)
	}
	// Only one error key.
	if len(snap.Errors) != 1 {
		t.Errorf("Errors: want 1 key, got %d: %v", len(snap.Errors), snap.Errors)
	}
}

// TestAggregator_Snapshot_overlapFails triangulates the second failing-source case:
// when Overlap errors, its error lands in Errors["overlap"],
// but Worktrees and MergePlan are still populated.
func TestAggregator_Snapshot_overlapFails(t *testing.T) {
	t.Parallel()

	errOv := errors.New("ov: scan failed")

	m := mock.NewPlatformReaderMock()
	m.WorktreesResult = sampleWorktrees
	m.MergePlanResult = samplePlan
	m.OverlapErr = errOv

	agg := service.NewAggregator(m)
	snap := agg.Snapshot(context.Background())

	if snap.Errors["overlap"] == nil {
		t.Errorf("expected Errors[overlap] to be set")
	}
	if !errors.Is(snap.Errors["overlap"], errOv) {
		t.Errorf("Errors[overlap]: want %v, got %v", errOv, snap.Errors["overlap"])
	}
	if len(snap.Worktrees) != 1 {
		t.Errorf("Worktrees: want 1, got %d", len(snap.Worktrees))
	}
	if snap.MergePlan.BaseBranch != "develop" {
		t.Errorf("MergePlan.BaseBranch = %q, want develop", snap.MergePlan.BaseBranch)
	}
	if len(snap.Errors) != 1 {
		t.Errorf("Errors: want 1 key, got %d: %v", len(snap.Errors), snap.Errors)
	}
}

// TestAggregator_Snapshot_twoSourcesFail verifies that two simultaneous failures
// both land in Errors without blocking each other.
func TestAggregator_Snapshot_twoSourcesFail(t *testing.T) {
	t.Parallel()

	errWt := errors.New("wt: offline")
	errMo := errors.New("mo: plan failed")

	m := mock.NewPlatformReaderMock()
	m.WorktreesErr = errWt
	m.MergePlanErr = errMo
	m.OverlapResult = sampleOverlap

	agg := service.NewAggregator(m)
	snap := agg.Snapshot(context.Background())

	if len(snap.Errors) != 2 {
		t.Errorf("Errors: want 2 keys, got %d: %v", len(snap.Errors), snap.Errors)
	}
	if !errors.Is(snap.Errors["worktrees"], errWt) {
		t.Errorf("Errors[worktrees]: want %v, got %v", errWt, snap.Errors["worktrees"])
	}
	if !errors.Is(snap.Errors["mergeplan"], errMo) {
		t.Errorf("Errors[mergeplan]: want %v, got %v", errMo, snap.Errors["mergeplan"])
	}
	if snap.Overlap.Verdict != "OK" {
		t.Errorf("Overlap.Verdict = %q, want OK (should still succeed)", snap.Overlap.Verdict)
	}
}
