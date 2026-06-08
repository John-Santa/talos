// Package mock provides hand-written test doubles for the port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// ErrSentinel returns a convenience error for programmable mock failures.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }

// IssueSearcherMock is a hand-written test double implementing port.IssueSearcher.
type IssueSearcherMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// SearchResults maps jql → result slice.
	SearchResults map[string][]port.IssueResult
	// SearchErrs maps jql → error.
	SearchErrs map[string]error
	// DefaultResult is returned when no specific jql key is found in SearchResults.
	DefaultResult []port.IssueResult
	// DefaultErr is returned when no specific jql key is found in SearchErrs.
	DefaultErr error
}

// NewIssueSearcherMock returns an initialized, empty mock.
func NewIssueSearcherMock() *IssueSearcherMock {
	return &IssueSearcherMock{}
}

func (m *IssueSearcherMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *IssueSearcherMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *IssueSearcherMock) AssertCallCount(t interface {
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
func (m *IssueSearcherMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

func (m *IssueSearcherMock) Search(_ context.Context, jql string, maxResults int) ([]port.IssueResult, error) {
	m.record("Search", jql, maxResults)

	if m.SearchErrs != nil {
		if err, ok := m.SearchErrs[jql]; ok {
			return nil, err
		}
	}
	if m.DefaultErr != nil {
		return nil, m.DefaultErr
	}

	if m.SearchResults != nil {
		if results, ok := m.SearchResults[jql]; ok {
			return results, nil
		}
	}
	return m.DefaultResult, nil
}

var _ port.IssueSearcher = (*IssueSearcherMock)(nil)
