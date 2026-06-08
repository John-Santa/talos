package overlap_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestParseFilesChecklist(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		body      string
		wantFiles []string
		wantErr   bool
	}{
		{
			name:      "marker absent returns ErrChecklistMissing",
			body:      "some body without the files: marker\n- [ ] foo.go\n",
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "files: section with zero items returns ErrChecklistMissing",
			body:      "files:\n\nsome other text\n",
			wantFiles: nil,
			wantErr:   true,
		},
		{
			name:      "case-insensitive marker FILES:",
			body:      "FILES:\n- [ ] a.go\n",
			wantFiles: []string{"a.go"},
			wantErr:   false,
		},
		{
			name:      "case-insensitive marker Files:",
			body:      "Files:\n- [ ] b.go\n",
			wantFiles: []string{"b.go"},
			wantErr:   false,
		},
		{
			name:      "unchecked item [ ] included",
			body:      "files:\n- [ ] platform/foo/bar.go\n",
			wantFiles: []string{"platform/foo/bar.go"},
			wantErr:   false,
		},
		{
			name:      "checked item [x] included",
			body:      "files:\n- [x] platform/foo/bar.go\n",
			wantFiles: []string{"platform/foo/bar.go"},
			wantErr:   false,
		},
		{
			name:      "checked item [X] included",
			body:      "files:\n- [X] platform/foo/bar.go\n",
			wantFiles: []string{"platform/foo/bar.go"},
			wantErr:   false,
		},
		{
			name:      "backtick path stripped",
			body:      "files:\n- [ ] `platform/foo/bar.go`\n",
			wantFiles: []string{"platform/foo/bar.go"},
			wantErr:   false,
		},
		{
			name:      "section ends at non-item non-blank line",
			body:      "files:\n- [ ] a.go\nsome text here\n- [ ] b.go\n",
			wantFiles: []string{"a.go"},
			wantErr:   false,
		},
		{
			name:      "multiple items parsed correctly",
			body:      "files:\n- [ ] a.go\n- [x] b.go\n- [X] c.go\n",
			wantFiles: []string{"a.go", "b.go", "c.go"},
			wantErr:   false,
		},
		{
			name:      "blank lines within section are skipped",
			body:      "files:\n- [ ] a.go\n\n- [x] b.go\n",
			wantFiles: []string{"a.go", "b.go"},
			wantErr:   false,
		},
		{
			name:      "content before marker is ignored",
			body:      "Title\nDescription\n- [ ] ignored.go\nfiles:\n- [ ] real.go\n",
			wantFiles: []string{"real.go"},
			wantErr:   false,
		},
		{
			name:      "asterisk bullet recognized",
			body:      "files:\n* [ ] a.go\n",
			wantFiles: []string{"a.go"},
			wantErr:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files, err := overlap.ParseFilesChecklist(tc.body)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected ErrChecklistMissing, got nil error with files: %v", files)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(files) != len(tc.wantFiles) {
				t.Errorf("files count = %d, want %d; got %v", len(files), len(tc.wantFiles), files)
				return
			}
			for i, f := range files {
				if f != tc.wantFiles[i] {
					t.Errorf("files[%d] = %q, want %q", i, f, tc.wantFiles[i])
				}
			}
		})
	}
}
