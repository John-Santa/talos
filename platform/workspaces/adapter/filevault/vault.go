// Package filevault implements CredentialVault using 0600 files in TALOS_HOME/credentials/.
//
// Each workspace's credentials are stored in <talosHome>/credentials/<ref>.env with
// permissions 0600. The format matches the project's .env convention (KEY=value).
//
// This is the first-cut adapter: plaintext 0600 (same security posture as the
// existing per-repo .env files). A keychain adapter behind the same port is deferred
// (see adapter/keychain, build tag: keychain).
package filevault

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

// Vault implements port.CredentialVault with file-backed storage.
type Vault struct {
	dir string // path to credentials directory (<talosHome>/credentials)
}

// New creates a Vault whose credential directory is <talosHome>/credentials.
// The directory is created with mode 0700 if it does not exist.
func New(talosHome string) (*Vault, error) {
	dir := filepath.Join(talosHome, "credentials")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating credential directory %q: %w", dir, err)
	}
	return &Vault{dir: dir}, nil
}

// Store writes credentials for ref to <dir>/<ref>.env at mode 0600.
func (v *Vault) Store(_ context.Context, ref string, c port.Credentials) error {
	content := fmt.Sprintf("JIRA_EMAIL=%s\nJIRA_API_TOKEN=%s\n", c.Email, c.APIToken)
	if err := os.WriteFile(v.credPath(ref), []byte(content), fs.FileMode(0600)); err != nil {
		return fmt.Errorf("writing credential file for %q: %w", ref, err)
	}
	return nil
}

// Load reads credentials for ref; returns ErrNotFound if the file is absent.
func (v *Vault) Load(_ context.Context, ref string) (port.Credentials, error) {
	b, err := os.ReadFile(v.credPath(ref))
	if err != nil {
		if os.IsNotExist(err) {
			return port.Credentials{}, &workspace.ErrNotFound{Name: ref}
		}
		return port.Credentials{}, fmt.Errorf("reading credential file for %q: %w", ref, err)
	}
	return parseCredFile(b), nil
}

// Delete removes the credential file for ref. No-op if absent.
func (v *Vault) Delete(_ context.Context, ref string) error {
	err := os.Remove(v.credPath(ref))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting credential file for %q: %w", ref, err)
	}
	return nil
}

func (v *Vault) credPath(ref string) string {
	return filepath.Join(v.dir, ref+".env")
}

// parseCredFile reads KEY=value lines from a credential file.
func parseCredFile(b []byte) port.Credentials {
	var c port.Credentials
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		switch strings.TrimSpace(k) {
		case "JIRA_EMAIL":
			c.Email = strings.TrimSpace(v)
		case "JIRA_API_TOKEN":
			c.APIToken = strings.TrimSpace(v)
		}
	}
	return c
}
