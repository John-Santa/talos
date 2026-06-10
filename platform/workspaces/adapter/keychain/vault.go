//go:build keychain

// Package keychain provides a CredentialVault adapter backed by the OS keychain.
// This adapter is DEFERRED and requires the `keychain` build tag to activate.
// It is not compiled or tested in CI.
//
// Build with: go build -tags keychain ./...
// Requires: github.com/zalando/go-keyring (or equivalent) — add to go.mod when activating.
package keychain

// Vault would implement port.CredentialVault via go-keyring.
// Stub only — not implemented.
type Vault struct{}
