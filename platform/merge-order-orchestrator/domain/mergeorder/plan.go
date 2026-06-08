package mergeorder

// Step holds the result of predicting a single merge in the plan.
type Step struct {
	Position       int
	Candidate      Candidate
	PredictedClean bool
	ConflictFiles  []string
}

// MergePlan is the ordered sequence of steps to integrate into the base branch.
type MergePlan struct {
	BaseBranch string
	BaseTip    string
	Steps      []Step
}

// PlanReport pairs a MergePlan with its computed health metrics.
type PlanReport struct {
	Plan            MergePlan
	ConflictingCount int
	ConflictRate    float64
	SegmentationBad bool
}

// NewPlanReport constructs a PlanReport by computing conflict metrics over the plan.
//
// SegmentationBad is true when ConflictRate is strictly greater than maxConflictRate (REQ-HEALTH-2).
func NewPlanReport(plan MergePlan, maxConflictRate float64) PlanReport {
	n := len(plan.Steps)
	if n == 0 {
		return PlanReport{Plan: plan}
	}

	conflicting := 0
	for _, s := range plan.Steps {
		if !s.PredictedClean {
			conflicting++
		}
	}

	rate := float64(conflicting) / float64(n)
	return PlanReport{
		Plan:            plan,
		ConflictingCount: conflicting,
		ConflictRate:    rate,
		SegmentationBad: rate > maxConflictRate,
	}
}
