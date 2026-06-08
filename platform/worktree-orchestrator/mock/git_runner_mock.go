// Package mock provides hand-written test doubles for the port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// GitRunnerMock is a hand-written test double implementing port.GitRunner.
// It records all calls in order and returns pre-programmed results.
type GitRunnerMock struct {
	// Calls is the ordered list of all method invocations received.
	Calls []Call

	FetchErr error

	// BranchExistsResult is the fallback when BranchExistsResultsByBranch does not contain the branch.
	BranchExistsResult          bool
	BranchExistsErr             error
	BranchExistsResultsByBranch map[string]bool

	WorktreeAddErr error

	WorktreeListResult string
	WorktreeListErr    error

	WorktreeRemoveErr error

	PruneErr error

	BranchDeleteErr error
}

// ErrDirtyWorktreeSentinel returns an ErrDirtyWorktreeSentinel for tests that need the adapter to signal a dirty worktree.
func ErrDirtyWorktreeSentinel(path string) error {
	return &worktree.ErrDirtyWorktreeSentinel{Path: path}
}

// ErrDirtyWorktreeSentinelWithCount returns an ErrDirtyWorktreeSentinel with a file count.
func ErrDirtyWorktreeSentinelWithCount(path string, count int) error {
	return &worktree.ErrDirtyWorktreeSentinel{Path: path, FileCount: count}
}

// NewGitRunnerMock returns an initialized, empty mock.
func NewGitRunnerMock() *GitRunnerMock {
	return &GitRunnerMock{}
}

func (m *GitRunnerMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method name.
func (m *GitRunnerMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// AssertCallCount reports an error via t if method was not called exactly n times.
func (m *GitRunnerMock) AssertCallCount(t interface {
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
func (m *GitRunnerMock) AssertMethodOrder(t interface {
	Helper()
	Errorf(string, ...any)
}, methods ...string) {
	t.Helper()
	if len(m.Calls) != len(methods) {
		t.Errorf("mock: call count = %d, want %d; got %v, want %v",
			len(m.Calls), len(methods), methodNames(m.Calls), methods)
		return
	}
	for i, want := range methods {
		if m.Calls[i].Method != want {
			t.Errorf("mock: call[%d] = %q, want %q", i, m.Calls[i].Method, want)
		}
	}
}

// AssertNotCalled reports an error via t if method was called at all.
func (m *GitRunnerMock) AssertNotCalled(t interface {
	Helper()
	Errorf(string, ...any)
}, method string) {
	t.Helper()
	if len(m.CallsFor(method)) > 0 {
		t.Errorf("mock: %s was called but should not have been", method)
	}
}

func methodNames(calls []Call) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Method
	}
	return names
}

func (m *GitRunnerMock) Fetch(_ context.Context) error {
	m.record("Fetch")
	return m.FetchErr
}

func (m *GitRunnerMock) BranchExists(_ context.Context, branch string) (bool, error) {
	m.record("BranchExists", branch)
	if m.BranchExistsResultsByBranch != nil {
		if result, ok := m.BranchExistsResultsByBranch[branch]; ok {
			return result, m.BranchExistsErr
		}
	}
	return m.BranchExistsResult, m.BranchExistsErr
}

func (m *GitRunnerMock) WorktreeAdd(_ context.Context, path, branch, baseBranch string) error {
	m.record("WorktreeAdd", path, branch, baseBranch)
	return m.WorktreeAddErr
}

func (m *GitRunnerMock) WorktreeList(_ context.Context) (string, error) {
	m.record("WorktreeList")
	return m.WorktreeListResult, m.WorktreeListErr
}

func (m *GitRunnerMock) WorktreeRemove(_ context.Context, path string, force bool) error {
	m.record("WorktreeRemove", path, force)
	return m.WorktreeRemoveErr
}

func (m *GitRunnerMock) Prune(_ context.Context) error {
	m.record("Prune")
	return m.PruneErr
}

func (m *GitRunnerMock) BranchDelete(_ context.Context, branch string) error {
	m.record("BranchDelete", branch)
	return m.BranchDeleteErr
}

var _ interface {
	Fetch(context.Context) error
	BranchExists(context.Context, string) (bool, error)
	WorktreeAdd(context.Context, string, string, string) error
	WorktreeList(context.Context) (string, error)
	WorktreeRemove(context.Context, string, bool) error
	Prune(context.Context) error
	BranchDelete(context.Context, string) error
} = (*GitRunnerMock)(nil)

// ErrSentinel is a convenience sentinel for programmable errors in tests.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }
