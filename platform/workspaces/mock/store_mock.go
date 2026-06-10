// Package mock provides hand-written test doubles for the port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// WorkspaceStoreMock is a hand-written test double implementing port.WorkspaceStore.
type WorkspaceStoreMock struct {
	Calls      []Call
	Workspaces []workspace.Workspace
	ActiveName string

	ListErr      error
	GetErr       error
	AddErr       error
	RemoveErr    error
	SetActiveErr error
	ActiveErr    error
}

func (m *WorkspaceStoreMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method.
func (m *WorkspaceStoreMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (m *WorkspaceStoreMock) List(_ context.Context) ([]workspace.Workspace, error) {
	m.record("List")
	return m.Workspaces, m.ListErr
}

func (m *WorkspaceStoreMock) Get(_ context.Context, name string) (workspace.Workspace, error) {
	m.record("Get", name)
	if m.GetErr != nil {
		return workspace.Workspace{}, m.GetErr
	}
	for _, ws := range m.Workspaces {
		if ws.Name == name {
			return ws, nil
		}
	}
	return workspace.Workspace{}, &workspace.ErrNotFound{Name: name}
}

func (m *WorkspaceStoreMock) Add(_ context.Context, ws workspace.Workspace) error {
	m.record("Add", ws.Name)
	if m.AddErr != nil {
		return m.AddErr
	}
	for _, existing := range m.Workspaces {
		if existing.Name == ws.Name {
			return &workspace.ErrDuplicate{Name: ws.Name}
		}
	}
	m.Workspaces = append(m.Workspaces, ws)
	return nil
}

func (m *WorkspaceStoreMock) Remove(_ context.Context, name string) error {
	m.record("Remove", name)
	if m.RemoveErr != nil {
		return m.RemoveErr
	}
	for i, ws := range m.Workspaces {
		if ws.Name == name {
			m.Workspaces = append(m.Workspaces[:i], m.Workspaces[i+1:]...)
			return nil
		}
	}
	return &workspace.ErrNotFound{Name: name}
}

func (m *WorkspaceStoreMock) SetActive(_ context.Context, name string) error {
	m.record("SetActive", name)
	if m.SetActiveErr != nil {
		return m.SetActiveErr
	}
	for _, ws := range m.Workspaces {
		if ws.Name == name {
			m.ActiveName = name
			return nil
		}
	}
	return &workspace.ErrNotFound{Name: name}
}

func (m *WorkspaceStoreMock) Active(_ context.Context) (string, error) {
	m.record("Active")
	return m.ActiveName, m.ActiveErr
}

// ErrSentinel is a convenience sentinel for programmable errors in tests.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }
