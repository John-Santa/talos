package envfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/internal/envfile"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "blank line ignored",
			input: "\n",
			want:  map[string]string{},
		},
		{
			name:  "comment line ignored",
			input: "# comment\n",
			want:  map[string]string{},
		},
		{
			name:  "first-= split",
			input: "K=a=b=c",
			want:  map[string]string{"K": "a=b=c"},
		},
		{
			name:  "trim spaces",
			input: "  K  =  V  ",
			want:  map[string]string{"K": "V"},
		},
		{
			name:  "no-= ignored",
			input: "NOEQUALS",
			want:  map[string]string{},
		},
		{
			name:  "CRLF tolerated",
			input: "K=V\r\n",
			want:  map[string]string{"K": "V"},
		},
		{
			name:  "empty input",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "multiple valid pairs",
			input: "A=1\nB=2\n",
			want:  map[string]string{"A": "1", "B": "2"},
		},
		{
			name:  "inline hash not a comment",
			input: "K=value#notcomment",
			want:  map[string]string{"K": "value#notcomment"},
		},
		{
			name:  "leading hash is comment",
			input: "#K=value",
			want:  map[string]string{},
		},
		{
			name:  "whitespace-only line ignored",
			input: "   \n",
			want:  map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := envfile.Parse([]byte(tc.input))
			if got == nil {
				t.Fatal("Parse returned nil, want non-nil map")
			}
			if len(got) != len(tc.want) {
				t.Errorf("len(got)=%d, len(want)=%d; got=%v, want=%v", len(got), len(tc.want), got, tc.want)
				return
			}
			for k, wantV := range tc.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("key %q missing from result", k)
				} else if gotV != wantV {
					t.Errorf("key %q: got %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestLoadInto(t *testing.T) {
	writeFile := func(t *testing.T, dir, name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("writeFile: %v", err)
		}
		return p
	}

	t.Run("key already set not overwritten", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, ".env", "K=file\n")

		env := map[string]string{"K": "real"}
		setenv := func(k, v string) error { env[k] = v; return nil }
		getenv := func(k string) string { return env[k] }

		if err := envfile.LoadInto(setenv, getenv, p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := env["K"]; got != "real" {
			t.Errorf("K=%q, want %q", got, "real")
		}
	})

	t.Run("unset key gets populated", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, ".env", "K=V\n")

		env := map[string]string{}
		setenv := func(k, v string) error { env[k] = v; return nil }
		getenv := func(k string) string { return env[k] }

		if err := envfile.LoadInto(setenv, getenv, p); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := env["K"]; got != "V" {
			t.Errorf("K=%q, want %q", got, "V")
		}
	})

	t.Run("first-file-wins", func(t *testing.T) {
		dir := t.TempDir()
		first := writeFile(t, dir, "first.env", "K=first\n")
		second := writeFile(t, dir, "second.env", "K=second\n")

		env := map[string]string{}
		setenv := func(k, v string) error { env[k] = v; return nil }
		getenv := func(k string) string { return env[k] }

		if err := envfile.LoadInto(setenv, getenv, first, second); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := env["K"]; got != "first" {
			t.Errorf("K=%q, want %q", got, "first")
		}
	})

	t.Run("missing file skipped silently", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "nonexistent.env")

		env := map[string]string{}
		setenv := func(k, v string) error { env[k] = v; return nil }
		getenv := func(k string) string { return env[k] }

		if err := envfile.LoadInto(setenv, getenv, missing); err != nil {
			t.Errorf("expected nil error for missing file, got: %v", err)
		}
		if len(env) != 0 {
			t.Errorf("env should be empty, got: %v", env)
		}
	})

	t.Run("read-error surfaces", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, "unreadable.env", "K=V\n")
		if err := os.Chmod(p, 0o000); err != nil {
			t.Skip("cannot set permissions on this system")
		}
		t.Cleanup(func() { os.Chmod(p, 0o600) })

		env := map[string]string{}
		setenv := func(k, v string) error { env[k] = v; return nil }
		getenv := func(k string) string { return env[k] }

		err := envfile.LoadInto(setenv, getenv, p)
		if err == nil {
			t.Error("expected error for unreadable file, got nil")
		}
	})
}
