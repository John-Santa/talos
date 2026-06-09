// Package mock provides hand-written test doubles for the port interfaces.
package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/port"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// PlatformReaderMock is a hand-written test double implementing port.PlatformReader.
// It records all calls in order and returns pre-programmed results.
type PlatformReaderMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// WorktreesResult is the slice returned by Worktrees when WorktreesErr is nil.
	WorktreesResult []platform.Worktree
	// WorktreesErr is the error returned by Worktrees (overrides result when non-nil).
	WorktreesErr error

	// MergePlanResult is returned by MergePlan when MergePlanErr is nil.
	MergePlanResult platform.MergePlan
	// MergePlanErr is the error returned by MergePlan (overrides result when non-nil).
	MergePlanErr error

	// OverlapResult is returned by Overlap when OverlapErr is nil.
	OverlapResult platform.Overlap
	// OverlapErr is the error returned by Overlap (overrides result when non-nil).
	OverlapErr error

	// LabelsResult is returned by Labels when LabelsErr is nil.
	LabelsResult platform.Labels
	// LabelsErr is the error returned by Labels (overrides result when non-nil).
	LabelsErr error
}

// compile-time check: PlatformReaderMock must satisfy port.PlatformReader.
var _ port.PlatformReader = (*PlatformReaderMock)(nil)

// NewPlatformReaderMock returns an initialized, empty mock.
func NewPlatformReaderMock() *PlatformReaderMock {
	return &PlatformReaderMock{}
}

func (m *PlatformReaderMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *PlatformReaderMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *PlatformReaderMock) AssertCallCount(t interface {
	Helper()
	Errorf(string, ...any)
}, method string, n int) {
	t.Helper()
	got := len(m.CallsFor(method))
	if got != n {
		t.Errorf("mock: %s called %d time(s), want %d", method, got, n)
	}
}

// AssertNotCalled reports an error via t if method was called at all.
func (m *PlatformReaderMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

// Worktrees returns the programmed WorktreesResult/WorktreesErr and records the call.
func (m *PlatformReaderMock) Worktrees(_ context.Context) ([]platform.Worktree, error) {
	m.record("Worktrees")
	return m.WorktreesResult, m.WorktreesErr
}

// MergePlan returns the programmed MergePlanResult/MergePlanErr and records the call.
func (m *PlatformReaderMock) MergePlan(_ context.Context) (platform.MergePlan, error) {
	m.record("MergePlan")
	return m.MergePlanResult, m.MergePlanErr
}

// Overlap returns the programmed OverlapResult/OverlapErr and records the call.
func (m *PlatformReaderMock) Overlap(_ context.Context) (platform.Overlap, error) {
	m.record("Overlap")
	return m.OverlapResult, m.OverlapErr
}

// Labels returns the programmed LabelsResult/LabelsErr and records the call.
// The branch argument is captured in the call record as Args[0].
func (m *PlatformReaderMock) Labels(_ context.Context, branch string) (platform.Labels, error) {
	m.record("Labels", branch)
	return m.LabelsResult, m.LabelsErr
}
