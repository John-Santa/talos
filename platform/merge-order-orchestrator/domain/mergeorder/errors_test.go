package mergeorder_test

import (
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
)

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "ErrDependencyCycle contains branch names",
			err:      &mergeorder.ErrDependencyCycle{Branches: []string{"feat/a", "feat/b"}},
			contains: "feat/a",
		},
		{
			name:     "ErrNoCandidates non-empty",
			err:      &mergeorder.ErrNoCandidates{},
			contains: "no merge candidates",
		},
		{
			name:     "ErrBranchBehind contains branch",
			err:      &mergeorder.ErrBranchBehind{Branch: "feat/x"},
			contains: "feat/x",
		},
		{
			name:     "ErrMergeConflict contains branch",
			err:      &mergeorder.ErrMergeConflict{Branch: "feat/y", Files: []string{"main.go"}},
			contains: "feat/y",
		},
		{
			name:     "ErrWtBinaryNotFound contains binary",
			err:      &mergeorder.ErrWtBinaryNotFound{Binary: "wt"},
			contains: "wt",
		},
		{
			name:     "ErrWtOutputMalformed contains fragment",
			err:      &mergeorder.ErrWtOutputMalformed{Fragment: "bad{json", Cause: nil},
			contains: "bad{json",
		},
		{
			name:     "ErrSegmentationBad contains rate",
			err:      &mergeorder.ErrSegmentationBad{Rate: 0.25, Threshold: 0.15},
			contains: "0.2500",
		},
		{
			name:     "ErrDevelopNotAvailable non-empty",
			err:      &mergeorder.ErrDevelopNotAvailable{},
			contains: "develop",
		},
		{
			name:     "ErrBranchNotFound contains branch",
			err:      &mergeorder.ErrBranchNotFound{Branch: "missing"},
			contains: "missing",
		},
		{
			name:     "ErrRebaseConflict contains branch",
			err:      &mergeorder.ErrRebaseConflict{Branch: "feat/z", Files: []string{"a.go"}},
			contains: "feat/z",
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
