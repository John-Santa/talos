package workspace_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
)

// probeStub is a minimal RepoProbe implementation for domain tests.
// It avoids importing the port package from tests (no circular dep).
type probeStub struct {
	absPath  string
	isGit    bool
	resolveErr error
	isGitErr   error
}

func (p *probeStub) Resolve(path string) (string, error) {
	if p.resolveErr != nil {
		return "", p.resolveErr
	}
	if p.absPath != "" {
		return p.absPath, nil
	}
	return path, nil
}

func (p *probeStub) IsGitRepo(path string) (bool, error) {
	return p.isGit, p.isGitErr
}

func validBinding() workspace.JiraBinding {
	return workspace.JiraBinding{
		SiteURL:       "https://example.atlassian.net",
		ProjectKey:    "EX",
		ProjectID:     "10001",
		IssueTypeName: "Story",
		StateMapping: map[workspace.StatusCategory][]string{
			workspace.CategoryNew:           {"To Do"},
			workspace.CategoryIndeterminate: {"In Progress"},
			workspace.CategoryDone:          {"Done"},
		},
	}
}

func TestNewWorkspace_HappyPath(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: true}

	ws, err := workspace.NewWorkspace("my-project", "/abs/repo", validBinding(), probe)
	if err != nil {
		t.Fatalf("NewWorkspace() unexpected error: %v", err)
	}
	if ws.Name != "my-project" {
		t.Errorf("Name = %q, want %q", ws.Name, "my-project")
	}
	if ws.RepoPath != "/abs/repo" {
		t.Errorf("RepoPath = %q, want %q", ws.RepoPath, "/abs/repo")
	}
	if ws.CredRef != "my-project" {
		t.Errorf("CredRef = %q, want %q", ws.CredRef, "my-project")
	}
}

func TestNewWorkspace_InvalidName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"spaces", "my project"},
		{"leading-hyphen", "-bad"},
		{"trailing-hyphen", "bad-"},
		{"uppercase", "MyProject"},
		{"special-chars", "my@project"},
	}
	probe := &probeStub{absPath: "/abs/repo", isGit: true}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := workspace.NewWorkspace(tc.input, "/abs/repo", validBinding(), probe)
			var target *workspace.ErrInvalidName
			if !errors.As(err, &target) {
				t.Errorf("expected ErrInvalidName for %q, got %T: %v", tc.input, err, err)
			}
		})
	}
}

func TestNewWorkspace_RepoNotFound(t *testing.T) {
	t.Parallel()
	probe := &probeStub{resolveErr: &workspace.ErrRepoNotFound{Path: "/no/path"}}

	_, err := workspace.NewWorkspace("ok-name", "/no/path", validBinding(), probe)
	var target *workspace.ErrRepoNotFound
	if !errors.As(err, &target) {
		t.Errorf("expected ErrRepoNotFound, got %T: %v", err, err)
	}
}

func TestNewWorkspace_NotGitRepo(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: false}

	_, err := workspace.NewWorkspace("ok-name", "/abs/repo", validBinding(), probe)
	var target *workspace.ErrNotGitRepo
	if !errors.As(err, &target) {
		t.Errorf("expected ErrNotGitRepo, got %T: %v", err, err)
	}
}

func TestNewWorkspace_IncompleteBinding_EmptySiteURL(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: true}
	b := validBinding()
	b.SiteURL = ""

	_, err := workspace.NewWorkspace("ok-name", "/abs/repo", b, probe)
	var target *workspace.ErrIncompleteBinding
	if !errors.As(err, &target) {
		t.Errorf("expected ErrIncompleteBinding, got %T: %v", err, err)
	}
}

func TestNewWorkspace_IncompleteBinding_EmptyProjectKey(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: true}
	b := validBinding()
	b.ProjectKey = ""

	_, err := workspace.NewWorkspace("ok-name", "/abs/repo", b, probe)
	var target *workspace.ErrIncompleteBinding
	if !errors.As(err, &target) {
		t.Errorf("expected ErrIncompleteBinding, got %T: %v", err, err)
	}
}

func TestNewWorkspace_IncompleteBinding_MissingCategory(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: true}
	b := validBinding()
	delete(b.StateMapping, workspace.CategoryDone)

	_, err := workspace.NewWorkspace("ok-name", "/abs/repo", b, probe)
	var target *workspace.ErrIncompleteBinding
	if !errors.As(err, &target) {
		t.Errorf("expected ErrIncompleteBinding for missing category, got %T: %v", err, err)
	}
}

func TestNewWorkspace_IncompleteBinding_EmptyCategory(t *testing.T) {
	t.Parallel()
	probe := &probeStub{absPath: "/abs/repo", isGit: true}
	b := validBinding()
	b.StateMapping[workspace.CategoryNew] = []string{}

	_, err := workspace.NewWorkspace("ok-name", "/abs/repo", b, probe)
	var target *workspace.ErrIncompleteBinding
	if !errors.As(err, &target) {
		t.Errorf("expected ErrIncompleteBinding for empty category, got %T: %v", err, err)
	}
}

func TestWorkspace_ValidNameFormats(t *testing.T) {
	t.Parallel()
	valid := []string{
		"my-project",
		"talos",
		"project-123",
		"a",
		"abc-def-ghi",
		"project1",
	}
	probe := &probeStub{isGit: true}

	for _, name := range valid {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe.absPath = "/abs/" + name
			_, err := workspace.NewWorkspace(name, "/abs/"+name, validBinding(), probe)
			if err != nil {
				t.Errorf("NewWorkspace(%q) unexpected error: %v", name, err)
			}
		})
	}
}
