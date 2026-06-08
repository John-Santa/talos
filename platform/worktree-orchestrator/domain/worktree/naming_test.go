package worktree_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
)

func TestParseFigura(t *testing.T) {
	t.Parallel()
	validFiguras := []string{
		"atlas", "hephaestus", "cronos", "iris", "gaia", "themis", "hermes", "argos",
	}
	for _, f := range validFiguras {
		f := f
		t.Run("valid_"+f, func(t *testing.T) {
			t.Parallel()
			got, err := worktree.ParseFigura(f)
			if err != nil {
				t.Fatalf("ParseFigura(%q) returned unexpected error: %v", f, err)
			}
			if string(got) != f {
				t.Errorf("ParseFigura(%q) = %q, want %q", f, string(got), f)
			}
		})
	}

	invalidFiguras := []string{"zeus", "athena", "ATLAS", "Atlas", "", "unknown", "admin"}
	for _, f := range invalidFiguras {
		f := f
		t.Run("invalid_"+f, func(t *testing.T) {
			t.Parallel()
			_, err := worktree.ParseFigura(f)
			if err == nil {
				t.Fatalf("ParseFigura(%q) expected error, got nil", f)
			}
			var e *worktree.ErrInvalidFigure
			if !errors.As(err, &e) {
				t.Errorf("ParseFigura(%q) error type = %T, want *ErrInvalidFigure", f, err)
			}
		})
	}
}

func TestValidateJiraKey(t *testing.T) {
	t.Parallel()
	validKeys := []string{"TAL-1", "TAL-42", "TAL-100", "TAL-9999"}
	for _, k := range validKeys {
		k := k
		t.Run("valid_"+k, func(t *testing.T) {
			t.Parallel()
			if err := worktree.ValidateJiraKey(k); err != nil {
				t.Errorf("ValidateJiraKey(%q) returned unexpected error: %v", k, err)
			}
		})
	}

	invalidKeys := []string{"TAL-0", "tal-5", "FOO-1", "TAL-", "", "TAL", "1-TAL", "TAL-1a"}
	for _, k := range invalidKeys {
		k := k
		t.Run("invalid_"+k, func(t *testing.T) {
			t.Parallel()
			err := worktree.ValidateJiraKey(k)
			if err == nil {
				t.Fatalf("ValidateJiraKey(%q) expected error, got nil", k)
			}
			var e *worktree.ErrInvalidKey
			if !errors.As(err, &e) {
				t.Errorf("ValidateJiraKey(%q) error type = %T, want *ErrInvalidKey", k, err)
			}
		})
	}
}

func TestBranchName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		figura worktree.Figura
		key    string
		want   string
	}{
		{worktree.Figura("atlas"), "TAL-1", "agent/atlas/TAL-1"},
		{worktree.Figura("hermes"), "TAL-42", "agent/hermes/TAL-42"},
		{worktree.Figura("iris"), "TAL-100", "agent/iris/TAL-100"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := worktree.BranchName(tc.figura, tc.key)
			if got != tc.want {
				t.Errorf("BranchName(%q, %q) = %q, want %q", tc.figura, tc.key, got, tc.want)
			}
		})
	}
}

func TestWorktreePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		base   string
		figura worktree.Figura
		want   string
	}{
		{"talos.wt", worktree.Figura("atlas"), "talos.wt/agent-atlas"},
		{"talos.wt", worktree.Figura("hermes"), "talos.wt/agent-hermes"},
		{"/abs/path", worktree.Figura("iris"), "/abs/path/agent-iris"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := worktree.WorktreePath(tc.base, tc.figura)
			if got != tc.want {
				t.Errorf("WorktreePath(%q, %q) = %q, want %q", tc.base, tc.figura, got, tc.want)
			}
		})
	}
}

func TestNewWorktreeSpec(t *testing.T) {
	t.Parallel()

	t.Run("happy_path", func(t *testing.T) {
		t.Parallel()
		spec, err := worktree.NewWorktreeSpec("hermes", "TAL-2", "talos.wt")
		if err != nil {
			t.Fatalf("NewWorktreeSpec unexpected error: %v", err)
		}
		if string(spec.Figura) != "hermes" {
			t.Errorf("spec.Figura = %q, want %q", spec.Figura, "hermes")
		}
		if spec.JiraKey != "TAL-2" {
			t.Errorf("spec.JiraKey = %q, want %q", spec.JiraKey, "TAL-2")
		}
		if spec.Branch != "agent/hermes/TAL-2" {
			t.Errorf("spec.Branch = %q, want %q", spec.Branch, "agent/hermes/TAL-2")
		}
		if spec.Path != "talos.wt/agent-hermes" {
			t.Errorf("spec.Path = %q, want %q", spec.Path, "talos.wt/agent-hermes")
		}
	})

	t.Run("invalid_figura", func(t *testing.T) {
		t.Parallel()
		_, err := worktree.NewWorktreeSpec("zeus", "TAL-1", "talos.wt")
		if err == nil {
			t.Fatal("expected error for invalid figura, got nil")
		}
		var e *worktree.ErrInvalidFigure
		if !errors.As(err, &e) {
			t.Errorf("error type = %T, want *ErrInvalidFigure", err)
		}
	})

	t.Run("invalid_key", func(t *testing.T) {
		t.Parallel()
		_, err := worktree.NewWorktreeSpec("atlas", "TAL-0", "talos.wt")
		if err == nil {
			t.Fatal("expected error for invalid key, got nil")
		}
		var e *worktree.ErrInvalidKey
		if !errors.As(err, &e) {
			t.Errorf("error type = %T, want *ErrInvalidKey", err)
		}
	})
}
