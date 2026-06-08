package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/port"
)

// GitIntegratorMock is a hand-written test double implementing port.GitIntegrator.
//
// This mock exposes exactly ONE method: RebaseOnto.
// The absence of any Merge, Push, or OpenPR method is the structural proof
// that mo cannot merge to develop (HG3 structural enforcement, ADR-M1).
type GitIntegratorMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// RebaseConflicts is returned by RebaseOnto when RebaseErr is nil.
	RebaseConflicts []string
	// RebaseErr is the error returned by RebaseOnto.
	RebaseErr error
}

// NewGitIntegratorMock returns an initialized, empty mock.
func NewGitIntegratorMock() *GitIntegratorMock {
	return &GitIntegratorMock{}
}

func (m *GitIntegratorMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *GitIntegratorMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertMethodOrder checks that the mock's overall call history matches the given ordered method names exactly.
func (m *GitIntegratorMock) AssertMethodOrder(t interface {
	Helper()
	Errorf(string, ...any)
}, methods ...string) {
	t.Helper()
	if len(m.Calls) != len(methods) {
		t.Errorf("mock: call count = %d, want %d; got %v, want %v",
			len(m.Calls), len(methods), integratorMethodNames(m.Calls), methods)
		return
	}
	for i, want := range methods {
		if m.Calls[i].Method != want {
			t.Errorf("mock: call[%d] = %q, want %q", i, m.Calls[i].Method, want)
		}
	}
}

// AssertNotCalled reports an error via t if method was called at all.
func (m *GitIntegratorMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

func integratorMethodNames(calls []Call) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Method
	}
	return names
}

func (m *GitIntegratorMock) RebaseOnto(_ context.Context, branch, base string) ([]string, error) {
	m.record("RebaseOnto", branch, base)
	return m.RebaseConflicts, m.RebaseErr
}

var _ port.GitIntegrator = (*GitIntegratorMock)(nil)
