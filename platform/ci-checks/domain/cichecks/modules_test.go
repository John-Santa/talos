package cichecks_test

import (
	"reflect"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func TestModulesFromChangedPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			name:  "deep file under platform module",
			paths: []string{"platform/ci-checks/domain/cichecks/label.go"},
			want:  []string{"platform/ci-checks"},
		},
		{
			name:  "file directly under module root (go.mod)",
			paths: []string{"platform/ci-checks/go.mod"},
			want:  []string{"platform/ci-checks"},
		},
		{
			name:  "multiple files in same module deduplicated",
			paths: []string{"platform/ci-checks/a.go", "platform/ci-checks/b.go", "platform/ci-checks/sub/c.go"},
			want:  []string{"platform/ci-checks"},
		},
		{
			name:  "two modules sorted",
			paths: []string{"platform/foo/a.go", "platform/bar/b.go"},
			want:  []string{"platform/bar", "platform/foo"},
		},
		{
			name:  "non-platform path ignored",
			paths: []string{"openspec/changes/x.md"},
			want:  []string{},
		},
		{
			name:  "bare platform/<m> with no file segment included",
			paths: []string{"platform/ci-checks"},
			want:  []string{"platform/ci-checks"},
		},
		{
			name:  "empty input",
			paths: []string{},
			want:  []string{},
		},
		{
			name:  "mix of platform and non-platform paths",
			paths: []string{"README.md", "platform/ci-checks/main.go", ".github/workflows/pr-checks.yml"},
			want:  []string{"platform/ci-checks"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cichecks.ModulesFromChangedPaths(tc.paths)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ModulesFromChangedPaths(%v) = %v, want %v", tc.paths, got, tc.want)
			}
		})
	}
}
