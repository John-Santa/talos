package envmaterializer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/adapter/envmaterializer"
	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/port"
)

func makeWS(repoPath string) workspace.Workspace {
	return workspace.Workspace{
		Name:     "my-project",
		RepoPath: repoPath,
		Jira: workspace.JiraBinding{
			SiteURL:       "https://example.atlassian.net",
			ProjectKey:    "EX",
			ProjectID:     "10001",
			IssueTypeName: "Story",
			StateMapping: map[workspace.StatusCategory][]string{
				workspace.CategoryNew:           {"To Do"},
				workspace.CategoryIndeterminate: {"In Progress"},
				workspace.CategoryDone:          {"Done"},
			},
		},
		CredRef: "my-project",
	}
}

func TestMaterializer_Apply_WritesProjectEnv(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	ws := makeWS(repoDir)
	creds := port.Credentials{Email: "user@example.com", APIToken: "tok-secret"}

	m := envmaterializer.New()
	if err := m.Apply(context.Background(), ws, creds); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// .talos/project.env should exist with non-secret vars
	projEnvPath := filepath.Join(repoDir, ".talos", "project.env")
	b, err := os.ReadFile(projEnvPath)
	if err != nil {
		t.Fatalf("reading project.env: %v", err)
	}
	content := string(b)

	for _, want := range []string{
		"JIRA_SITE_URL=https://example.atlassian.net",
		"JIRA_PROJECT_KEY=EX",
		"JIRA_PROJECT_ID=10001",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("project.env missing %q\nfull content:\n%s", want, content)
		}
	}

	// project.env must NOT contain secrets
	if strings.Contains(content, "JIRA_API_TOKEN") {
		t.Error("project.env contains JIRA_API_TOKEN — must not expose secrets")
	}
	if strings.Contains(content, "user@example.com") {
		t.Error("project.env contains email — must not expose secrets")
	}
}

func TestMaterializer_Apply_WritesSecretEnv(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	ws := makeWS(repoDir)
	creds := port.Credentials{Email: "user@example.com", APIToken: "tok-secret"}

	m := envmaterializer.New()
	_ = m.Apply(context.Background(), ws, creds)

	// .env should exist with secrets
	dotEnvPath := filepath.Join(repoDir, ".env")
	b, err := os.ReadFile(dotEnvPath)
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	content := string(b)

	for _, want := range []string{
		"JIRA_EMAIL=user@example.com",
		"JIRA_API_TOKEN=tok-secret",
	} {
		if !strings.Contains(content, want) {
			t.Errorf(".env missing %q\nfull content:\n%s", want, content)
		}
	}
}

func TestMaterializer_Apply_SecretFilePermissions(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	ws := makeWS(repoDir)
	creds := port.Credentials{Email: "user@example.com", APIToken: "tok-secret"}

	m := envmaterializer.New()
	_ = m.Apply(context.Background(), ws, creds)

	dotEnvPath := filepath.Join(repoDir, ".env")
	info, err := os.Stat(dotEnvPath)
	if err != nil {
		t.Fatalf("stat .env: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf(".env permissions = %o, want 0600", perm)
	}
}

func TestMaterializer_Apply_CreatesTaglosDir(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	ws := makeWS(repoDir)
	creds := port.Credentials{Email: "u@e.com", APIToken: "tok"}

	m := envmaterializer.New()
	_ = m.Apply(context.Background(), ws, creds)

	talosDir := filepath.Join(repoDir, ".talos")
	if info, err := os.Stat(talosDir); err != nil || !info.IsDir() {
		t.Errorf(".talos directory not created at %q", talosDir)
	}
}

func TestMaterializer_Apply_InjectableWriteFile(t *testing.T) {
	t.Parallel()
	var written []string
	m := envmaterializer.NewWithWriteFile(func(path string, data []byte, perm os.FileMode) error {
		written = append(written, path)
		return nil
	})

	repoDir := t.TempDir()
	ws := makeWS(repoDir)
	creds := port.Credentials{Email: "u@e.com", APIToken: "tok"}

	if err := m.Apply(context.Background(), ws, creds); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if len(written) != 2 {
		t.Errorf("WriteFile called %d time(s), want 2 (project.env + .env)", len(written))
	}
}
