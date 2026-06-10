package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/console/port"
)

// PlatformActorMock is a hand-written test double implementing port.PlatformActor.
// It records all calls in order and returns pre-programmed results.
type PlatformActorMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// TeardownErr is the error returned by TeardownWorktree (nil = success).
	TeardownErr error

	// CreateErr is the error returned by CreateWorktree (nil = success).
	CreateErr error
}

// compile-time check: PlatformActorMock must satisfy port.PlatformActor.
var _ port.PlatformActor = (*PlatformActorMock)(nil)

// NewPlatformActorMock returns an initialized, empty mock.
func NewPlatformActorMock() *PlatformActorMock {
	return &PlatformActorMock{}
}

// CallsFor returns all recorded calls for the given method name.
func (m *PlatformActorMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *PlatformActorMock) AssertCallCount(t interface {
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
func (m *PlatformActorMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

// TeardownWorktree records the call and returns the programmed TeardownErr.
func (m *PlatformActorMock) TeardownWorktree(_ context.Context, figura, jiraKey string) error {
	m.Calls = append(m.Calls, Call{Method: "TeardownWorktree", Args: []any{figura, jiraKey}})
	return m.TeardownErr
}

// CreateWorktree records the call and returns the programmed CreateErr.
func (m *PlatformActorMock) CreateWorktree(_ context.Context, figura, jiraKey string) error {
	m.Calls = append(m.Calls, Call{Method: "CreateWorktree", Args: []any{figura, jiraKey}})
	return m.CreateErr
}
