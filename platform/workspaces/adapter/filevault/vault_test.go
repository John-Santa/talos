package filevault_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/adapter/filevault"
	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

func newVault(t *testing.T) *filevault.Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := filevault.New(dir)
	if err != nil {
		t.Fatalf("filevault.New: %v", err)
	}
	return v
}

func makeCreds() port.Credentials {
	return port.Credentials{Email: "user@example.com", APIToken: "tok-secret"}
}

func TestVault_StoreAndLoad(t *testing.T) {
	t.Parallel()
	v := newVault(t)
	ctx := context.Background()

	creds := makeCreds()
	if err := v.Store(ctx, "my-project", creds); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	got, err := v.Load(ctx, "my-project")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Email != creds.Email {
		t.Errorf("Email=%q, want %q", got.Email, creds.Email)
	}
	if got.APIToken != creds.APIToken {
		t.Errorf("APIToken=%q, want %q", got.APIToken, creds.APIToken)
	}
}

func TestVault_Load_NotFound(t *testing.T) {
	t.Parallel()
	v := newVault(t)
	ctx := context.Background()

	_, err := v.Load(ctx, "nonexistent")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestVault_Delete(t *testing.T) {
	t.Parallel()
	v := newVault(t)
	ctx := context.Background()

	_ = v.Store(ctx, "my-project", makeCreds())
	if err := v.Delete(ctx, "my-project"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err := v.Load(ctx, "my-project")
	var target *workspace.ErrNotFound
	if !errors.As(err, &target) {
		t.Errorf("after Delete, expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestVault_Delete_NoopIfAbsent(t *testing.T) {
	t.Parallel()
	v := newVault(t)
	ctx := context.Background()

	// delete non-existent: should not error
	if err := v.Delete(ctx, "nonexistent"); err != nil {
		t.Errorf("Delete(nonexistent) error: %v", err)
	}
}

func TestVault_FilePermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v, _ := filevault.New(dir)
	ctx := context.Background()

	_ = v.Store(ctx, "my-project", makeCreds())

	credFile := filepath.Join(dir, "credentials", "my-project.env")
	info, err := os.Stat(credFile)
	if err != nil {
		t.Fatalf("stat credential file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestVault_Store_Overwrite(t *testing.T) {
	t.Parallel()
	v := newVault(t)
	ctx := context.Background()

	_ = v.Store(ctx, "proj", port.Credentials{Email: "old@example.com", APIToken: "old-tok"})
	_ = v.Store(ctx, "proj", port.Credentials{Email: "new@example.com", APIToken: "new-tok"})

	got, _ := v.Load(ctx, "proj")
	if got.Email != "new@example.com" {
		t.Errorf("after overwrite, Email=%q, want %q", got.Email, "new@example.com")
	}
}
