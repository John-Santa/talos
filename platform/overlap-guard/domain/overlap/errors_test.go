package overlap_test

import (
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name: "ErrSameFileParallel contains file and agents",
			err: &overlap.ErrSameFileParallel{
				File: "platform/foo/bar.go",
				A:    overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"platform/foo/bar.go"}, overlap.SourceActual),
				B:    overlap.NewClaim("hermes", "mod:qa", "branch-b", []string{"platform/foo/bar.go"}, overlap.SourceActual),
			},
			contains: "platform/foo/bar.go",
		},
		{
			name:     "ErrEscalateZeus contains reason",
			err:      &overlap.ErrEscalateZeus{Reason: "unresolvable overlap"},
			contains: "unresolvable overlap",
		},
		{
			name:     "ErrChecklistMissing contains issue key",
			err:      &overlap.ErrChecklistMissing{IssueKey: "TAL-99"},
			contains: "TAL-99",
		},
		{
			name:     "ErrNoClaims non-empty",
			err:      &overlap.ErrNoClaims{},
			contains: "no claims",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := tc.err.Error()
			if msg == "" {
				t.Errorf("Error() returned empty string")
			}
			if !strings.Contains(msg, tc.contains) {
				t.Errorf("Error() = %q, want it to contain %q", msg, tc.contains)
			}
		})
	}
}
