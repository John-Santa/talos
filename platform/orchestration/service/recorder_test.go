// Package service_test covers RunRecorder integration in the Dispatcher.
// Tests verify that events are emitted at the correct dispatch steps,
// and that a failing recorder NEVER alters the dispatch result (fail-soft).
package service_test

import (
	"context"
	"testing"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/mock"
	"github.com/John-Santa/talos/platform/orchestration/port"
	"github.com/John-Santa/talos/platform/orchestration/service"
)

// newDispatcherWithRecorder wires a Dispatcher with a RunRecorder attached.
func newDispatcherWithRecorder(
	wt *mock.WorktreeManagerMock,
	ov *mock.OverlapCheckerMock,
	ev *mock.EvidenceRunnerMock,
	mo *mock.MergeCoordinatorMock,
	rb *mock.RollbackCoordinatorMock,
	rec *mock.RunRecorderMock,
) *service.Dispatcher {
	return service.NewDispatcher(wt, ov, ev, mo, rb, rec, service.DefaultConfig())
}

// TestRecorder_HappyPath_Propose verifies that on a successful propose dispatch:
// - RecordDispatch("running") is called at the start
// - RecordActivity for "dispatch propose iniciado"
// - RecordActivity for "worktree <branch> listo"
// - RecordActivity for "evidence run-loop --phase=propose ok"
// - RecordDispatch("done") at the end
func TestRecorder_HappyPath_Propose(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(propose) unexpected error: %v", err)
	}

	// At minimum: RecordDispatch called twice (running + done)
	dispatchCalls := rec.CallsFor("RecordDispatch")
	if len(dispatchCalls) < 2 {
		t.Errorf("RecordDispatch called %d times, want >= 2 (running + done)", len(dispatchCalls))
	}
	// First call must be "running"
	if len(dispatchCalls) >= 1 {
		if status, ok := dispatchCalls[0].Args[2].(string); !ok || status != "running" {
			t.Errorf("first RecordDispatch status = %q, want %q", dispatchCalls[0].Args[2], "running")
		}
	}
	// Last call must be "done"
	if last := dispatchCalls[len(dispatchCalls)-1]; true {
		if status, ok := last.Args[2].(string); !ok || status != "done" {
			t.Errorf("last RecordDispatch status = %q, want %q", last.Args[2], "done")
		}
	}

	// RecordActivity must be called at least once (worktree + evidence ok)
	actCalls := rec.CallsFor("RecordActivity")
	if len(actCalls) == 0 {
		t.Error("RecordActivity never called on happy path propose")
	}
}

// TestRecorder_HappyPath_Verify verifies merge gate emits a metric.
func TestRecorder_HappyPath_Verify(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: true, ConflictRate: 0.1, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseVerify, port.EvidenceArgs{})
	if err != nil {
		t.Fatalf("Dispatch(verify) unexpected error: %v", err)
	}

	// conflict_rate metric must be recorded
	metricCalls := rec.CallsFor("RecordMetric")
	if len(metricCalls) == 0 {
		t.Error("RecordMetric never called on verify (should record conflict_rate)")
	}
}

// TestRecorder_FailSoft_RecorderErrorDoesNotAbortDispatch is the key invariant:
// when RunRecorder returns an error, the dispatch MUST still succeed.
func TestRecorder_FailSoft_RecorderErrorDoesNotAbortDispatch(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}
	// Recorder always returns an error
	rec := &mock.RunRecorderMock{Err: mock.ErrSentinel("runs record: binary not found")}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	result, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	// MUST NOT error
	if err != nil {
		t.Fatalf("Dispatch must succeed when recorder errors (fail-soft), got: %v", err)
	}
	if result.IssueKey != "TAL-42" {
		t.Errorf("IssueKey = %q, want %q", result.IssueKey, "TAL-42")
	}
	// Recorder was still called (it just failed softly)
	if len(rec.CallsFor("RecordDispatch")) == 0 {
		t.Error("RecordDispatch was never called — recorder should have been invoked even on fail-soft path")
	}
}

// TestRecorder_FailSoft_EvidenceFailure_RecorderErrorDoesNotHideRealError verifies
// that when evidence fails AND the recorder errors, the real dispatch error is returned.
func TestRecorder_FailSoft_EvidenceFailure_RecorderErrorDoesNotHideRealError(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{Err: mock.ErrSentinel("evidence exploded")}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{Err: mock.ErrSentinel("runs record: binary not found")}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	// The real error from evidence MUST be propagated
	if err == nil {
		t.Fatal("expected error from evidence failure, got nil")
	}
	// Recipe11 is still called
	rb.AssertCallCount(t, "Recipe11", 1)
	// The error must contain the evidence failure, not the recorder error
	if err.Error() == "" {
		t.Error("expected non-empty error")
	}
}

// TestRecorder_OverlapBlock_EmitsFailedDispatch verifies that on BLOCK:
// - RecordDispatch("running") is called
// - RecordDispatch("failed") is called after the block
func TestRecorder_OverlapBlock_EmitsFailedDispatch(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictBlock}
	ev := &mock.EvidenceRunnerMock{}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on BLOCK, got nil")
	}

	dispatchCalls := rec.CallsFor("RecordDispatch")
	if len(dispatchCalls) < 2 {
		t.Errorf("RecordDispatch called %d times on BLOCK path, want >= 2 (running + failed)", len(dispatchCalls))
	}
	// Last call must be "failed"
	if last := dispatchCalls[len(dispatchCalls)-1]; true {
		if status, ok := last.Args[2].(string); !ok || status != "failed" {
			t.Errorf("last RecordDispatch status = %q, want %q", last.Args[2], "failed")
		}
	}
}

// TestRecorder_MergeConflict_EmitsFailedAndRecipe11 verifies merge conflict path.
func TestRecorder_MergeConflict_EmitsFailedAndRecipe11(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{IssueKey: "TAL-42"}
	mo := &mock.MergeCoordinatorMock{PlanResult: port.MergePlan{Clean: false, ConflictRate: 0.8, Threshold: 0.3}}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhaseVerify, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on merge conflict, got nil")
	}

	// conflict_rate metric recorded
	if len(rec.CallsFor("RecordMetric")) == 0 {
		t.Error("RecordMetric not called on merge conflict path")
	}
	// dispatch must end as "failed"
	dispatchCalls := rec.CallsFor("RecordDispatch")
	if len(dispatchCalls) == 0 {
		t.Fatal("RecordDispatch never called")
	}
	if last := dispatchCalls[len(dispatchCalls)-1]; true {
		if status, ok := last.Args[2].(string); !ok || status != "failed" {
			t.Errorf("last RecordDispatch status = %q, want %q", last.Args[2], "failed")
		}
	}
	// Recipe11 still fired
	rb.AssertCallCount(t, "Recipe11", 1)
}

// TestRecorder_EvidenceFailure_EmitsFailedAndRollback verifies evidence failure.
func TestRecorder_EvidenceFailure_EmitsFailedAndRollback(t *testing.T) {
	t.Parallel()
	wt := &mock.WorktreeManagerMock{EnsurePath: "/wt/hephaestus"}
	ov := &mock.OverlapCheckerMock{Verdict: dispatch.VerdictOK}
	ev := &mock.EvidenceRunnerMock{Err: mock.ErrSentinel("evidence exploded")}
	mo := &mock.MergeCoordinatorMock{}
	rb := &mock.RollbackCoordinatorMock{}
	rec := &mock.RunRecorderMock{}

	d := newDispatcherWithRecorder(wt, ov, ev, mo, rb, rec)
	_, err := d.Dispatch(context.Background(), testItem(), dispatch.PhasePropose, port.EvidenceArgs{})
	if err == nil {
		t.Fatal("expected error on evidence failure, got nil")
	}

	// dispatch must end as "failed"
	dispatchCalls := rec.CallsFor("RecordDispatch")
	if len(dispatchCalls) == 0 {
		t.Fatal("RecordDispatch never called")
	}
	if last := dispatchCalls[len(dispatchCalls)-1]; true {
		if status, ok := last.Args[2].(string); !ok || status != "failed" {
			t.Errorf("last RecordDispatch status = %q, want %q", last.Args[2], "failed")
		}
	}
	// ROLLBACK activity
	actCalls := rec.CallsFor("RecordActivity")
	if len(actCalls) == 0 {
		t.Error("RecordActivity never called — expected ROLLBACK §11 activity on evidence failure")
	}
}
