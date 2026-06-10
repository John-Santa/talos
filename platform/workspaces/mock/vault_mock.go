package mock

import (
	"context"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

// CredentialVaultMock is a hand-written test double implementing port.CredentialVault.
type CredentialVaultMock struct {
	Calls []Call
	Store_ map[string]port.Credentials

	StoreErr error
	LoadErr  error
	DeleteErr error
}

func NewCredentialVaultMock() *CredentialVaultMock {
	return &CredentialVaultMock{Store_: make(map[string]port.Credentials)}
}

func (m *CredentialVaultMock) Store(_ context.Context, ref string, c port.Credentials) error {
	m.Calls = append(m.Calls, Call{Method: "Store", Args: []any{ref}})
	if m.StoreErr != nil {
		return m.StoreErr
	}
	m.Store_[ref] = c
	return nil
}

func (m *CredentialVaultMock) Load(_ context.Context, ref string) (port.Credentials, error) {
	m.Calls = append(m.Calls, Call{Method: "Load", Args: []any{ref}})
	if m.LoadErr != nil {
		return port.Credentials{}, m.LoadErr
	}
	c, ok := m.Store_[ref]
	if !ok {
		return port.Credentials{}, &workspace.ErrNotFound{Name: ref}
	}
	return c, nil
}

func (m *CredentialVaultMock) Delete(_ context.Context, ref string) error {
	m.Calls = append(m.Calls, Call{Method: "Delete", Args: []any{ref}})
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	delete(m.Store_, ref)
	return nil
}
