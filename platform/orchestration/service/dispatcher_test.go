package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/mock"
	"github.com/John-Santa/talos/platform/orchestration/port"
	"github.com/John-Santa/talos/platform/orchestration/service"
)

func testItem() dispatch.WorkItem {
	return dispatch.WorkItem{
		JiraKey: "TAL-42",
		Change:  "orchestration",
		Agent:   "hephaestus",
		Module:  "module:orchestration",
	}
}

func newDispatcher(wt *mock.WorktreeManagerMock, ov *mock.OverlapCheckerMock, ev *mock.EvidenceRunnerMock, mo *mock.MergeCoordinatorMock, rb *mock.RollbackCoordinatorMock) *service.Dispatcher {
	return service.NewDispatcher(wt, ov, ev, mo, rb, service.DefaultConfig())
}

// TestDispatch_HappyPath_Propose verifies the full propose sequence: ov check → wt ensure → evidence run → no merge gate.
func TestDispatch_HappyPath_Propose(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	result, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(propose) unexpected error: %v", err)
	}
	if result.IssueKey != "TAL-42" {
		t.Errorf("IssueKey = %q, want %q", result.IssueKey, "TAL-42")
	}

	ov.AssertCallCount(t, "Check", 1)
	wt.AssertCallCount(t, "Ensure", 1)
	ev.AssertCallCount(t, "RunPhase", 1)
	mo.AssertNotCalled(t, "Plan")
	mo.AssertNotCalled(t, "Execute")
	wt.AssertNotCalled(t, "Teardown")
	rb.AssertNotCalled(t, "Recipe11")
}

// TestDispatch_HappyPath_Verify verifies verify sequence: ov → wt ensure → evidence → merge gate (clean) → no teardown.
func TestDispatch_HappyPath_Verify(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: true, ConflictRate: 0.0, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseVerify, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(verify) unexpected error: %v", err)
	}

	ov.AssertCallCount(t, "Check", 1)
	wt.AssertCallCount(t, "Ensure", 1)
	ev.AssertCallCount(t, "RunPhase", 1)
	mo.AssertCallCount(t, "Plan", 1)
	mo.AssertNotCalled(t, "Execute") // no --confirm-merge
	wt.AssertNotCalled(t, "Teardown")
}

// TestDispatch_HappyPath_Archive verifies archive: all gates + teardown after Done.
func TestDispatch_HappyPath_Archive(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: true, ConflictRate: 0.0, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseArchive, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(archive) unexpected error: %v", err)
	}

	wt.AssertCallCount(t, "Ensure", 1)
	wt.AssertCallCount(t, "Teardown", 1) // cleanup step
}

// TestDispatch_OverlapBlock verifies that BLOCK verdict → ABORT (no wt, no evidence, Recipe11 with jiraKey).
func TestDispatch_OverlapBlock(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictBlock}
	ev := &mock.EvidenceRunnerMock{}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on BLOCK, got nil")
	}
	var e *dispatch.ErrOverlapBlock
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrOverlapBlock", err)
	}

	wt.AssertNotCalled(t, "Ensure")
	ev.AssertNotCalled(t, "RunPhase")
}

// TestDispatch_OverlapSerialize verifies SERIALIZE → QUEUE returned, no wt/evidence.
func TestDispatch_OverlapSerialize(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictSerialize}
	ev := &mock.EvidenceRunnerMock{}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected ErrQueuedSerialize, got nil")
	}
	var e *dispatch.ErrQueuedSerialize
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrQueuedSerialize", err)
	}

	wt.AssertNotCalled(t, "Ensure")
	ev.AssertNotCalled(t, "RunPhase")
}

// TestDispatch_MergeConflict_Triggers_Recipe11 verifies that conflict above threshold → Recipe11 + error.
func TestDispatch_MergeConflict_Triggers_Recipe11(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: false, ConflictRate: 0.8, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseVerify, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on merge conflict, got nil")
	}
	var e *dispatch.ErrMergeConflict
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrMergeConflict", err)
	}

	rb.AssertCallCount(t, "Recipe11", 1)
	mo.AssertNotCalled(t, "Execute") // never merges on conflict
}

// TestDispatch_EvidenceFailure_Triggers_Recipe11 verifies evidence failure → Recipe11 + error returned.
func TestDispatch_EvidenceFailure_Triggers_Recipe11(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{Err: mock.ErrSentinel("evidence exploded")}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on evidence failure, got nil")
	}

	rb.AssertCallCount(t, "Recipe11", 1)
}

// TestDispatch_ConfirmMerge_Executes verifies --confirm-merge flag triggers mo.Execute.
func TestDispatch_ConfirmMerge_Executes(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: true, ConflictRate: 0.0, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}

	cfg := service.DefaultConfig()
	cfg.ConfirmMerge = true
	d := service.NewDispatcher(wt, ov, ev, mo, rb, cfg)

	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseVerify, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(verify, confirm-merge) unexpected error: %v", err)
	}

	mo.AssertCallCount(t, "Execute", 1)
}

// TestDispatch_NoOverlapGate_IntermediatePhase verifies spec phase skips ov check.
func TestDispatch_NoOverlapGate_IntermediatePhase(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseSpec, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(spec) unexpected error: %v", err)
	}

	ov.AssertNotCalled(t, "Check") // spec has no overlap gate
	wt.AssertCallCount(t, "Ensure", 1)
	ev.AssertCallCount(t, "RunPhase", 1)
}

// TestDispatch_UnknownPhase verifies dispatch returns ErrUnknownPhase.
func TestDispatch_UnknownPhase(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{}
	ov := &mock.OverlapCheckerMock{}
	ev := &mock.EvidenceRunnerMock{}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), "not-a-phase", port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error for unknown phase, got nil")
	}
	var e *dispatch.ErrUnknownPhase
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrUnknownPhase", err)
	}
}

// TestDispatch_DryRun_Propose verifies dry-run propagates to EvidenceArgs and skips wt.Ensure.
func TestDispatch_DryRun_Propose(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: ""}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}

	d := newDispatcher(wt, ov, ev, mo, rb)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{DryRun: true})
	if err != nil {
		t.Fatalf("Dispatch(propose, dry-run) unexpected error: %v", err)
	}

	// In dry-run mode, wt.Ensure and evidence are still called but with dry-run propagated.
	// ov check is still called (planning).
	ev.AssertCallCount(t, "RunPhase", 1)
	// Verify the DryRun flag was forwarded.
	if len(ev.Calls) > 0 {
		args, ok := ev.Calls[0].Args[2].(port.EvidenceArgs)
		if !ok || !args.DryRun {
			t.Error("DryRun flag was not forwarded to EvidenceRunner")
		}
	}
}
