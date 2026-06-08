package overlap_test

import (
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestBuildPreAssignmentJQL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		project string
		module  string
		owner   string
		want    string
	}{
		{
			name:    "standard TAL project check",
			project: "TAL",
			module:  "core",
			owner:   "atlas",
			want:    `project = TAL AND statusCategory != Done AND labels = "module:core" AND labels NOT IN ("agent:atlas")`,
		},
		{
			name:    "different project and module",
			project: "PROJ",
			module:  "qa",
			owner:   "hermes",
			want:    `project = PROJ AND statusCategory != Done AND labels = "module:qa" AND labels NOT IN ("agent:hermes")`,
		},
		{
			name:    "module with colon prefix as-is",
			project: "TAL",
			module:  "data",
			owner:   "gaia",
			want:    `project = TAL AND statusCategory != Done AND labels = "module:data" AND labels NOT IN ("agent:gaia")`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := overlap.BuildPreAssignmentJQL(tc.project, tc.module, tc.owner)
			if got != tc.want {
				t.Errorf("BuildPreAssignmentJQL(%q, %q, %q)\ngot:  %q\nwant: %q", tc.project, tc.module, tc.owner, got, tc.want)
			}
		})
	}
}
