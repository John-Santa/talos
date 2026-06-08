package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// WorktreeListerMock is a hand-written test double implementing port.WorktreeLister.
type WorktreeListerMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// ListResult is returned by List when ListErr is nil.
	ListResult []port.WorktreeEntry
	// ListErr is the error returned by List.
	ListErr error
}

// NewWorktreeListerMock returns an initialized, empty mock.
func NewWorktreeListerMock() *WorktreeListerMock {
	return &WorktreeListerMock{}
}

func (m *WorktreeListerMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *WorktreeListerMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *WorktreeListerMock) AssertCallCount(t interface {
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
func (m *WorktreeListerMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

func (m *WorktreeListerMock) List(_ context.Context) ([]port.WorktreeEntry, error) {
	m.record("List")
	return m.ListResult, m.ListErr
}

var _ port.WorktreeLister = (*WorktreeListerMock)(nil)
