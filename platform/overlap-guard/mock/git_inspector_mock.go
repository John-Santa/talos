package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// GitInspectorMock is a hand-written test double implementing port.GitInspector.
// It records all calls in order and returns pre-programmed per-ref/branch results.
type GitInspectorMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	FetchErr error

	// RevParseResults maps ref → SHA.
	RevParseResults map[string]string
	// RevParseErrs maps ref → error.
	RevParseErrs map[string]error

	// ChangedFilesByBranch maps branch → changed file list.
	ChangedFilesByBranch map[string][]string
	// ChangedFilesErrs maps branch → error.
	ChangedFilesErrs map[string]error
}

// NewGitInspectorMock returns an initialized, empty mock.
func NewGitInspectorMock() *GitInspectorMock {
	return &GitInspectorMock{}
}

func (m *GitInspectorMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *GitInspectorMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *GitInspectorMock) AssertCallCount(t interface {
	Helper()
	Errorf(string, ...any)
}, method string, n int) {
	t.Helper()
	got := len(m.CallsFor(method))
	if got != n {
		t.Errorf("mock: %s called %d time(s), want %d", method, got, n)
	}
}

// AssertMethodOrder checks that the mock's overall call history matches the given ordered method names exactly.
func (m *GitInspectorMock) AssertMethodOrder(t interface {
	Helper()
	Errorf(string, ...any)
}, methods ...string) {
	t.Helper()
	if len(m.Calls) != len(methods) {
		t.Errorf("mock: call count = %d, want %d; got %v, want %v",
			len(m.Calls), len(methods), inspectorMethodNames(m.Calls), methods)
		return
	}
	for i, want := range methods {
		if m.Calls[i].Method != want {
			t.Errorf("mock: call[%d] = %q, want %q", i, m.Calls[i].Method, want)
		}
	}
}

// AssertNotCalled reports an error via t if method was called at all.
func (m *GitInspectorMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

func inspectorMethodNames(calls []Call) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Method
	}
	return names
}

func (m *GitInspectorMock) Fetch(_ context.Context) error {
	m.record("Fetch")
	return m.FetchErr
}

func (m *GitInspectorMock) RevParse(_ context.Context, ref string) (string, error) {
	m.record("RevParse", ref)
	var err error
	if m.RevParseErrs != nil {
		err = m.RevParseErrs[ref]
	}
	sha := ""
	if m.RevParseResults != nil {
		sha = m.RevParseResults[ref]
	}
	return sha, err
}

func (m *GitInspectorMock) ChangedFiles(_ context.Context, base, branch string) ([]string, error) {
	m.record("ChangedFiles", base, branch)
	if m.ChangedFilesErrs != nil {
		if err, ok := m.ChangedFilesErrs[branch]; ok {
			return nil, err
		}
	}
	if m.ChangedFilesByBranch != nil {
		return m.ChangedFilesByBranch[branch], nil
	}
	return nil, nil
}

var _ port.GitInspector = (*GitInspectorMock)(nil)
