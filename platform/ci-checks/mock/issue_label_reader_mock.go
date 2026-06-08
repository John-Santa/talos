// Package mock provides hand-written test doubles for the ci-checks port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/ci-checks/port"
)

// Call records a single invocation of a mock method.
type Call struct {
	// Method is the name of the called method.
	Method string
	// Args holds the arguments passed to the method (excluding ctx).
	Args []any
}

// ErrSentinel returns a convenience error for programmable mock failures.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }

// IssueLabelReaderMock is a hand-written test double implementing port.IssueLabelReader.
type IssueLabelReaderMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// LabelsByKeyResults maps issue key → label slice.
	LabelsByKeyResults map[string][]string
	// LabelsByKeyErrs maps issue key → error.
	LabelsByKeyErrs map[string]error
	// DefaultResult is returned when no specific key is found in LabelsByKeyResults.
	DefaultResult []string
	// DefaultErr is returned when no specific key is found in LabelsByKeyErrs.
	DefaultErr error
}

// NewIssueLabelReaderMock returns an initialized, empty mock.
func NewIssueLabelReaderMock() *IssueLabelReaderMock {
	return &IssueLabelReaderMock{}
}

func (m *IssueLabelReaderMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *IssueLabelReaderMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *IssueLabelReaderMock) AssertCallCount(t interface {
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
func (m *IssueLabelReaderMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

// AssertMethodOrder reports an error via t if the overall Calls slice does not
// contain the given methods in the specified relative order.
func (m *IssueLabelReaderMock) AssertMethodOrder(t interface {
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

// LabelsByKey records the call and returns the configured result for the given key.
func (m *IssueLabelReaderMock) LabelsByKey(_ context.Context, key string) ([]string, error) {
	m.record("LabelsByKey", key)

	if m.LabelsByKeyErrs != nil {
		if err, ok := m.LabelsByKeyErrs[key]; ok {
			return nil, err
		}
	}
	if m.DefaultErr != nil {
		return nil, m.DefaultErr
	}
	if m.LabelsByKeyResults != nil {
		if results, ok := m.LabelsByKeyResults[key]; ok {
			return results, nil
		}
	}
	return m.DefaultResult, nil
}

var _ port.IssueLabelReader = (*IssueLabelReaderMock)(nil)
