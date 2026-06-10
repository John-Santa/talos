// Package mock provides hand-written test doubles for the orchestration port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// Call records a single method invocation.
type Call struct {
	Method string
	Args   []any
}

// ErrSentinel returns a deterministic error for tests.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }

// callRecorder is embedded by all mocks.
type callRecorder struct {
	Calls []Call
}

func (r *callRecorder) record(method string, args ...any) {
	r.Calls = append(r.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method.
func (r *callRecorder) CallsFor(method string) []Call {
	var out []Call
	for _, c := range r.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error if method was not called exactly n times.
func (r *callRecorder) AssertCallCount(t interface {
	Helper()
	Errorf(string, ...any)
}, method string, n int) {
	t.Helper()
	if got := len(r.CallsFor(method)); got != n {
		t.Errorf("mock: %s called %d time(s), want %d", method, got, n)
	}
}

// AssertNotCalled reports an error if method was called.
func (r *callRecorder) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(r.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

// AssertMethodOrder checks the overall call history matches ordered method names.
func (r *callRecorder) AssertMethodOrder(t interface {
	Helper()
	Errorf(string, ...any)
}, methods ...string) {
	t.Helper()
	got := make([]string, len(r.Calls))
	for i, c := range r.Calls {
		got[i] = c.Method
	}
	if len(got) != len(methods) {
		t.Errorf("mock: call count = %d, want %d; got %v, want %v", len(got), len(methods), got, methods)
		return
	}
	for i, want := range methods {
		if got[i] != want {
			t.Errorf("mock: call[%d] = %q, want %q", i, got[i], want)
		}
	}
}

// ---- WorktreeManagerMock ----

// WorktreeManagerMock is a test double for port.WorktreeManager.
type WorktreeManagerMock struct {
	callRecorder
	EnsurePath string
	EnsureErr  error
	TeardownErr error
}

var _ port.WorktreeManager = (*WorktreeManagerMock)(nil)

func (m *WorktreeManagerMock) Ensure(_ context.Context, figura, jiraKey string) (string, error) {
	m.record("Ensure", figura, jiraKey)
	return m.EnsurePath, m.EnsureErr
}

func (m *WorktreeManagerMock) Teardown(_ context.Context, figura string, force bool) error {
	m.record("Teardown", figura, force)
	return m.TeardownErr
}

// ---- OverlapCheckerMock ----

// OverlapCheckerMock is a test double for port.OverlapChecker.
type OverlapCheckerMock struct {
	callRecorder
	Verdict dispatch.OverlapVerdict
	Err     error
}

var _ port.OverlapChecker = (*OverlapCheckerMock)(nil)

func (m *OverlapCheckerMock) Check(_ context.Context, module, agent string) (dispatch.OverlapVerdict, error) {
	m.record("Check", module, agent)
	return m.Verdict, m.Err
}

// ---- EvidenceRunnerMock ----

// EvidenceRunnerMock is a test double for port.EvidenceRunner.
type EvidenceRunnerMock struct {
	callRecorder
	IssueKey string
	Err      error
}

var _ port.EvidenceRunner = (*EvidenceRunnerMock)(nil)

func (m *EvidenceRunnerMock) RunPhase(_ context.Context, item dispatch.WorkItem, phase string, args port.EvidenceArgs) (string, error) {
	m.record("RunPhase", item, phase, args)
	return m.IssueKey, m.Err
}

// ---- MergeCoordinatorMock ----

// MergeCoordinatorMock is a test double for port.MergeCoordinator.
type MergeCoordinatorMock struct {
	callRecorder
	PlanResult  port.MergePlan
	PlanErr     error
	ExecuteErr  error
}

var _ port.MergeCoordinator = (*MergeCoordinatorMock)(nil)

func (m *MergeCoordinatorMock) Plan(_ context.Context) (port.MergePlan, error) {
	m.record("Plan")
	return m.PlanResult, m.PlanErr
}

func (m *MergeCoordinatorMock) Execute(_ context.Context) error {
	m.record("Execute")
	return m.ExecuteErr
}

// ---- RollbackCoordinatorMock ----

// RollbackCoordinatorMock is a test double for port.RollbackCoordinator.
type RollbackCoordinatorMock struct {
	callRecorder
	Err error
}

var _ port.RollbackCoordinator = (*RollbackCoordinatorMock)(nil)

func (m *RollbackCoordinatorMock) Recipe11(_ context.Context, jiraKey, figura, reason string) error {
	m.record("Recipe11", jiraKey, figura, reason)
	return m.Err
}
