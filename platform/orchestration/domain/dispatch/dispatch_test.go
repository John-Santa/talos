package dispatch_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
)

// TestDecideOnOverlap_OK verifies that OK verdict returns PROCEED.
func TestDecideOnOverlap_OK(t *testing.T) {
	t.Parallel()
	got := dispatch.DecideOnOverlap(dispatch.VerdictOK)
	if got != dispatch.ActionProceed {
		t.Errorf("DecideOnOverlap(OK) = %q, want %q", got, dispatch.ActionProceed)
	}
}

// TestDecideOnOverlap_BLOCK verifies that BLOCK verdict returns ABORT.
func TestDecideOnOverlap_BLOCK(t *testing.T) {
	t.Parallel()
	got := dispatch.DecideOnOverlap(dispatch.VerdictBlock)
	if got != dispatch.ActionAbort {
		t.Errorf("DecideOnOverlap(BLOCK) = %q, want %q", got, dispatch.ActionAbort)
	}
}

// TestDecideOnOverlap_SERIALIZE verifies that SERIALIZE verdict returns QUEUE.
func TestDecideOnOverlap_SERIALIZE(t *testing.T) {
	t.Parallel()
	got := dispatch.DecideOnOverlap(dispatch.VerdictSerialize)
	if got != dispatch.ActionQueue {
		t.Errorf("DecideOnOverlap(SERIALIZE) = %q, want %q", got, dispatch.ActionQueue)
	}
}

// TestDecideOnOverlap_Unknown verifies unknown verdicts return ABORT (fail-loud).
func TestDecideOnOverlap_Unknown(t *testing.T) {
	t.Parallel()
	got := dispatch.DecideOnOverlap("UNKNOWN_VERDICT")
	if got != dispatch.ActionAbort {
		t.Errorf("DecideOnOverlap(UNKNOWN) = %q, want %q (fail-loud default)", got, dispatch.ActionAbort)
	}
}

// TestPlanFor_propose verifies propose phase has EnsureWorktree+OverlapGate, no MergeGate/Cleanup.
func TestPlanFor_propose(t *testing.T) {
	t.Parallel()
	plan, err := dispatch.PlanFor(dispatch.PhasePropose)
	if err != nil {
		t.Fatalf("PlanFor(propose) unexpected error: %v", err)
	}
	if !plan.EnsureWorktree {
		t.Error("propose: EnsureWorktree should be true")
	}
	if !plan.OverlapGate {
		t.Error("propose: OverlapGate should be true")
	}
	if plan.MergeGate {
		t.Error("propose: MergeGate should be false")
	}
	if plan.Cleanup {
		t.Error("propose: Cleanup should be false")
	}
}

// TestPlanFor_verify verifies verify phase has MergeGate and EnsureWorktree, no Cleanup.
func TestPlanFor_verify(t *testing.T) {
	t.Parallel()
	plan, err := dispatch.PlanFor(dispatch.PhaseVerify)
	if err != nil {
		t.Fatalf("PlanFor(verify) unexpected error: %v", err)
	}
	if !plan.EnsureWorktree {
		t.Error("verify: EnsureWorktree should be true")
	}
	if !plan.OverlapGate {
		t.Error("verify: OverlapGate should be true")
	}
	if !plan.MergeGate {
		t.Error("verify: MergeGate should be true")
	}
	if plan.Cleanup {
		t.Error("verify: Cleanup should be false")
	}
}

// TestPlanFor_archive verifies archive phase has MergeGate AND Cleanup.
func TestPlanFor_archive(t *testing.T) {
	t.Parallel()
	plan, err := dispatch.PlanFor(dispatch.PhaseArchive)
	if err != nil {
		t.Fatalf("PlanFor(archive) unexpected error: %v", err)
	}
	if !plan.MergeGate {
		t.Error("archive: MergeGate should be true")
	}
	if !plan.Cleanup {
		t.Error("archive: Cleanup should be true")
	}
}

// TestPlanFor_spec verifies spec phase does NOT have OverlapGate (intermediate phases).
func TestPlanFor_spec(t *testing.T) {
	t.Parallel()
	plan, err := dispatch.PlanFor(dispatch.PhaseSpec)
	if err != nil {
		t.Fatalf("PlanFor(spec) unexpected error: %v", err)
	}
	if !plan.EnsureWorktree {
		t.Error("spec: EnsureWorktree should be true (idempotent ensure)")
	}
	if plan.MergeGate {
		t.Error("spec: MergeGate should be false")
	}
	if plan.Cleanup {
		t.Error("spec: Cleanup should be false")
	}
}

// TestPlanFor_UnknownPhase verifies that unknown phase returns ErrUnknownPhase.
func TestPlanFor_UnknownPhase(t *testing.T) {
	t.Parallel()
	_, err := dispatch.PlanFor("invalid-phase")
	if err == nil {
		t.Fatal("PlanFor(invalid) expected error, got nil")
	}
	if _, ok := err.(*dispatch.ErrUnknownPhase); !ok {
		t.Errorf("PlanFor(invalid) error type = %T, want *ErrUnknownPhase", err)
	}
}

// TestAllPhasesDefined verifies each known phase returns a plan without error.
func TestAllPhasesDefined(t *testing.T) {
	t.Parallel()
	phases := []dispatch.Phase{
		dispatch.PhasePropose,
		dispatch.PhaseSpec,
		dispatch.PhaseDesign,
		dispatch.PhaseTasks,
		dispatch.PhaseApply,
		dispatch.PhaseVerify,
		dispatch.PhaseArchive,
	}
	for _, p := range phases {
		p := p
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			_, err := dispatch.PlanFor(p)
			if err != nil {
				t.Errorf("PlanFor(%q) returned unexpected error: %v", p, err)
			}
		})
	}
}
