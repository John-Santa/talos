package mergeorder_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
)

func makeSteps(n int, conflictIndexes map[int]bool) []mergeorder.Step {
	steps := make([]mergeorder.Step, n)
	for i := range steps {
		steps[i] = mergeorder.Step{
			Position:       i + 1,
			PredictedClean: !conflictIndexes[i],
		}
		if conflictIndexes[i] {
			steps[i].ConflictFiles = []string{"conflict.go"}
		}
	}
	return steps
}

func TestNewPlanReport_ZeroSteps(t *testing.T) {
	t.Parallel()
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: nil}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictingCount != 0 {
		t.Errorf("ConflictingCount = %d, want 0", r.ConflictingCount)
	}
	if r.ConflictRate != 0.0 {
		t.Errorf("ConflictRate = %f, want 0.0", r.ConflictRate)
	}
	if r.SegmentationBad {
		t.Errorf("SegmentationBad = true, want false (no divide-by-zero)")
	}
}

func TestNewPlanReport_AllClean(t *testing.T) {
	t.Parallel()
	steps := makeSteps(3, nil)
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictingCount != 0 {
		t.Errorf("ConflictingCount = %d, want 0", r.ConflictingCount)
	}
	if r.ConflictRate != 0.0 {
		t.Errorf("ConflictRate = %f, want 0.0", r.ConflictRate)
	}
	if r.SegmentationBad {
		t.Errorf("SegmentationBad = true, want false")
	}
}

func TestNewPlanReport_OneConflictOfTwo(t *testing.T) {
	t.Parallel()
	steps := makeSteps(2, map[int]bool{0: true})
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictingCount != 1 {
		t.Errorf("ConflictingCount = %d, want 1", r.ConflictingCount)
	}
	if r.ConflictRate != 0.5 {
		t.Errorf("ConflictRate = %f, want 0.5", r.ConflictRate)
	}
	if !r.SegmentationBad {
		t.Errorf("SegmentationBad = false, want true (0.5 > 0.15)")
	}
}

func TestNewPlanReport_OneConflictOfFour(t *testing.T) {
	t.Parallel()
	steps := makeSteps(4, map[int]bool{1: true})
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictingCount != 1 {
		t.Errorf("ConflictingCount = %d, want 1", r.ConflictingCount)
	}
	if r.ConflictRate != 0.25 {
		t.Errorf("ConflictRate = %f, want 0.25", r.ConflictRate)
	}
	if !r.SegmentationBad {
		t.Errorf("SegmentationBad = false, want true (0.25 > 0.15)")
	}
}

func TestNewPlanReport_ExactlyAtThreshold(t *testing.T) {
	t.Parallel()
	// 15 conflicts out of 100 → rate 0.15 exactly → NOT bad (strict greater-than)
	steps := makeSteps(100, func() map[int]bool {
		m := map[int]bool{}
		for i := 0; i < 15; i++ {
			m[i] = true
		}
		return m
	}())
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictingCount != 15 {
		t.Errorf("ConflictingCount = %d, want 15", r.ConflictingCount)
	}
	if r.SegmentationBad {
		t.Errorf("SegmentationBad = true, want false (0.15 is exactly at threshold, not strictly greater)")
	}
}

func TestNewPlanReport_JustAboveThreshold(t *testing.T) {
	t.Parallel()
	// rate = 0.1501... is strictly greater than 0.15 → bad
	// Use 16 conflicts out of 100 for clean numbers
	steps := makeSteps(100, func() map[int]bool {
		m := map[int]bool{}
		for i := 0; i < 16; i++ {
			m[i] = true
		}
		return m
	}())
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if !r.SegmentationBad {
		t.Errorf("SegmentationBad = false, want true (0.16 > 0.15)")
	}
}

func TestNewPlanReport_ZeroConflicts(t *testing.T) {
	t.Parallel()
	steps := makeSteps(5, nil)
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictRate != 0.0 {
		t.Errorf("ConflictRate = %f, want 0.0", r.ConflictRate)
	}
	if r.SegmentationBad {
		t.Errorf("SegmentationBad = true, want false")
	}
}

func TestNewPlanReport_AllConflicts(t *testing.T) {
	t.Parallel()
	n := 5
	conflicts := map[int]bool{}
	for i := 0; i < n; i++ {
		conflicts[i] = true
	}
	steps := makeSteps(n, conflicts)
	plan := mergeorder.MergePlan{BaseBranch: "develop", Steps: steps}
	r := mergeorder.NewPlanReport(plan, 0.15)
	if r.ConflictRate != 1.0 {
		t.Errorf("ConflictRate = %f, want 1.0", r.ConflictRate)
	}
	if !r.SegmentationBad {
		t.Errorf("SegmentationBad = false, want true")
	}
}

func TestStep_Fields(t *testing.T) {
	t.Parallel()
	c := mergeorder.Candidate{Branch: "feat/x"}
	s := mergeorder.Step{
		Position:       1,
		Candidate:      c,
		PredictedClean: false,
		ConflictFiles:  []string{"x.go"},
	}
	if s.Position != 1 {
		t.Errorf("Position = %d, want 1", s.Position)
	}
	if s.Candidate.Branch != "feat/x" {
		t.Errorf("Candidate.Branch = %q, want %q", s.Candidate.Branch, "feat/x")
	}
	if s.PredictedClean {
		t.Errorf("PredictedClean = true, want false")
	}
	if len(s.ConflictFiles) != 1 {
		t.Errorf("ConflictFiles len = %d, want 1", len(s.ConflictFiles))
	}
}

func TestMergePlan_Fields(t *testing.T) {
	t.Parallel()
	plan := mergeorder.MergePlan{
		BaseBranch: "develop",
		BaseTip:    "abc123",
		Steps:      []mergeorder.Step{{Position: 1}},
	}
	if plan.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want %q", plan.BaseBranch, "develop")
	}
	if plan.BaseTip != "abc123" {
		t.Errorf("BaseTip = %q, want %q", plan.BaseTip, "abc123")
	}
	if len(plan.Steps) != 1 {
		t.Errorf("Steps len = %d, want 1", len(plan.Steps))
	}
}
