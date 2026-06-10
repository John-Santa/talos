package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

// MaterializerMock is a hand-written test double implementing port.Materializer.
type MaterializerMock struct {
	Calls     []Call
	ApplyErr  error
	LastWS    workspace.Workspace
	LastCreds port.Credentials
}

func (m *MaterializerMock) Apply(_ context.Context, ws workspace.Workspace, c port.Credentials) error {
	m.Calls = append(m.Calls, Call{Method: "Apply", Args: []any{ws.Name}})
	m.LastWS = ws
	m.LastCreds = c
	return m.ApplyErr
}
