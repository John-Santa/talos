// Package mock provides hand-written test doubles for the port interfaces.
// Zero third-party dependencies — uses only the stdlib testing package.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// JiraClientMock is a hand-written test double implementing port.JiraClient.
// It records all calls made and returns pre-programmed results.
//
// Usage:
//
//	m := mock.NewJiraClientMock()
//	m.CreateIssueResult = evidence.Issue{Key: "TAL-1"}
//	// ... inject m into service, run, inspect m.Calls
type JiraClientMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	// Programmable return values — set before the test runs.
	CreateIssueResult evidence.Issue
	CreateIssueErr    error

	SearchResults []evidence.Issue
	SearchErr     error

	GetTransitionsResult []evidence.Transition
	GetTransitionsErr    error

	DoTransitionErr error

	AddCommentErr error

	AddWorklogErr error

	CreateRemoteLinkErr error

	AddAttachmentErr error
}

// NewJiraClientMock returns an initialized, empty mock.
func NewJiraClientMock() *JiraClientMock {
	return &JiraClientMock{}
}

func (m *JiraClientMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *JiraClientMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount panics with a descriptive message if method was not called
// exactly n times. Designed for use inside test functions only.
func (m *JiraClientMock) AssertCallCount(t interface{ Helper(); Errorf(string, ...any) }, method string, n int) {
	t.Helper()
	got := len(m.CallsFor(method))
	if got != n {
		t.Errorf("mock: %s called %d time(s), want %d", method, got, n)
	}
}

// AssertMethodOrder checks that the mock's overall call history matches the
// given ordered method names exactly.
func (m *JiraClientMock) AssertMethodOrder(t interface{ Helper(); Errorf(string, ...any) }, wantOrder []string) {
	t.Helper()
	if len(m.Calls) != len(wantOrder) {
		t.Errorf("mock: call count = %d, want %d; got %v", len(m.Calls), len(wantOrder), methodNames(m.Calls))
		return
	}
	for i, want := range wantOrder {
		if m.Calls[i].Method != want {
			t.Errorf("mock: call[%d] = %q, want %q", i, m.Calls[i].Method, want)
		}
	}
}

func methodNames(calls []Call) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Method
	}
	return names
}

// ---------------------------------------------------------------------------
// port.JiraClient implementation
// ---------------------------------------------------------------------------

func (m *JiraClientMock) CreateIssue(_ context.Context, req evidence.CreateIssueRequest) (evidence.Issue, error) {
	m.record("CreateIssue", req)
	return m.CreateIssueResult, m.CreateIssueErr
}

func (m *JiraClientMock) Search(_ context.Context, jql string, maxResults int) ([]evidence.Issue, error) {
	m.record("Search", jql, maxResults)
	return m.SearchResults, m.SearchErr
}

func (m *JiraClientMock) GetTransitions(_ context.Context, issueKey string) ([]evidence.Transition, error) {
	m.record("GetTransitions", issueKey)
	return m.GetTransitionsResult, m.GetTransitionsErr
}

func (m *JiraClientMock) DoTransition(_ context.Context, issueKey, transitionID string) error {
	m.record("DoTransition", issueKey, transitionID)
	return m.DoTransitionErr
}

func (m *JiraClientMock) AddComment(_ context.Context, issueKey string, body evidence.ADFDocument) error {
	m.record("AddComment", issueKey, body)
	return m.AddCommentErr
}

func (m *JiraClientMock) AddWorklog(_ context.Context, issueKey string, worklog evidence.Worklog) error {
	m.record("AddWorklog", issueKey, worklog)
	return m.AddWorklogErr
}

func (m *JiraClientMock) CreateRemoteLink(_ context.Context, issueKey string, link evidence.RemoteLink) error {
	m.record("CreateRemoteLink", issueKey, link)
	return m.CreateRemoteLinkErr
}

func (m *JiraClientMock) AddAttachment(_ context.Context, issueKey string, attachment evidence.Attachment) error {
	m.record("AddAttachment", issueKey, attachment)
	return m.AddAttachmentErr
}

// Compile-time assertion: JiraClientMock must satisfy port.JiraClient.
// We do this via an interface-typed variable so we don't import port in this
// package (domain stays clean; mock only knows domain).
var _ interface {
	CreateIssue(context.Context, evidence.CreateIssueRequest) (evidence.Issue, error)
	Search(context.Context, string, int) ([]evidence.Issue, error)
	GetTransitions(context.Context, string) ([]evidence.Transition, error)
	DoTransition(context.Context, string, string) error
	AddComment(context.Context, string, evidence.ADFDocument) error
	AddWorklog(context.Context, string, evidence.Worklog) error
	CreateRemoteLink(context.Context, string, evidence.RemoteLink) error
	AddAttachment(context.Context, string, evidence.Attachment) error
} = (*JiraClientMock)(nil)

// ErrSentinel is a convenience sentinel for programmable errors in tests.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }
