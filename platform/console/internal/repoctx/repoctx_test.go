package repoctx_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/console/internal/repoctx"
)

func TestParseRepoSlug(t *testing.T) {
	t.Run("ssh github URL with .git", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("git@github.com:John-Santa/talos.git", "/home/user/talos")
		want := "John-Santa/talos"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("https github URL with .git", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("https://github.com/John-Santa/talos.git", "/home/user/talos")
		want := "John-Santa/talos"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("https github URL without .git", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("https://github.com/John-Santa/talos", "/home/user/talos")
		want := "John-Santa/talos"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ssh URL without .git", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("git@github.com:John-Santa/talos", "/home/user/talos")
		want := "John-Santa/talos"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty remote URL falls back to toplevel basename", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("", "/home/user/projects/talos")
		want := "talos"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("garbage remote URL falls back to toplevel basename", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("not-a-valid-url", "/home/user/myrepo")
		want := "myrepo"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("both empty returns empty string", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("", "")
		want := ""
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("nested org and repo with .git", func(t *testing.T) {
		got := repoctx.ParseRepoSlug("git@github.com:org-name/repo-name.git", "/home/user/repo-name")
		want := "org-name/repo-name"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
