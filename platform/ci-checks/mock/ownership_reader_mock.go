package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/ci-checks/port"
)

// OwnershipReaderMock is a hand-written test double implementing port.OwnershipReader.
type OwnershipReaderMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// OwnershipMap is the map returned by Ownership.
	OwnershipMap map[string]string
	// OwnershipErr is the error returned by Ownership; nil means no error.
	OwnershipErr error
}

// NewOwnershipReaderMock returns an initialized, empty mock.
func NewOwnershipReaderMock() *OwnershipReaderMock {
	return &OwnershipReaderMock{}
}

func (m *OwnershipReaderMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *OwnershipReaderMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *OwnershipReaderMock) AssertCallCount(t interface {
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
func (m *OwnershipReaderMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

// AssertMethodOrder reports an error via t if the Calls slice does not contain
// the given methods in the specified relative order.
func (m *OwnershipReaderMock) AssertMethodOrder(t interface {
	Helper()
	Errorf(string, ...any)
}, methods ...string) {
	t.Helper()
	pos := 0
	for _, want := range methods {
		found := false
		for pos < len(m.Calls) {
			if m.Calls[pos].Method == want {
				pos++
				found = true
				break
			}
			pos++
		}
		if !found {
			t.Errorf("mock: method %q not found in expected order %v (calls: %v)", want, methods, m.Calls)
			return
		}
	}
}

// Ownership records the call and returns the configured ownership map or error.
func (m *OwnershipReaderMock) Ownership(_ context.Context) (map[string]string, error) {
	m.record("Ownership")
	if m.OwnershipErr != nil {
		return nil, m.OwnershipErr
	}
	return m.OwnershipMap, nil
}

var _ port.OwnershipReader = (*OwnershipReaderMock)(nil)
