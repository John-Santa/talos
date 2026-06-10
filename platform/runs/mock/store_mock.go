// Package mock provides hand-written test doubles for the port interfaces.
package mock

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/runs/domain/run"
)

// Call records a single invocation of a mock method.
type Call struct {
	Method string
	Args   []any
}

// RunStoreMock is a hand-written test double implementing port.RunStore.
type RunStoreMock struct {
	Calls     []Call
	Events    []run.RunEvent
	AppendErr error
	QueryErr  error
}

func (m *RunStoreMock) record(method string, args ...any) {
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// CallsFor returns all recorded calls for the given method.
func (m *RunStoreMock) CallsFor(method string) []Call {
	var out []Call
	for _, c := range m.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (m *RunStoreMock) Append(_ context.Context, e run.RunEvent) error {
	m.record("Append", e.Kind, e.JiraKey)
	if m.AppendErr != nil {
		return m.AppendErr
	}
	m.Events = append(m.Events, e)
	return nil
}

func (m *RunStoreMock) Query(_ context.Context, f run.Filter) ([]run.RunEvent, error) {
	m.record("Query", f)
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	var out []run.RunEvent
	for _, e := range m.Events {
		if f.JiraKey != "" && e.JiraKey != f.JiraKey {
			continue
		}
		if f.Agent != "" && e.Agent != f.Agent {
			continue
		}
		if f.Kind != "" && e.Kind != f.Kind {
			continue
		}
		if !f.Since.IsZero() && e.At.Before(f.Since) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// ErrSentinel is a convenience sentinel for programmable errors in tests.
func ErrSentinel(msg string) error { return fmt.Errorf("mock: %s", msg) }
