package evidence_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

func TestValidateOwnership(t *testing.T) {
	ownership := map[string]string{
		"jira-loop": "hermes",
		"argos-eye": "argos",
	}

	tests := []struct {
		name      string
		module    string
		agent     string
		wantErr   bool
		errTarget *evidence.ErrOwnershipViolation
	}{
		{
			name:    "valid ownership",
			module:  "jira-loop",
			agent:   "hermes",
			wantErr: false,
		},
		{
			name:    "wrong agent for module",
			module:  "jira-loop",
			agent:   "argos",
			wantErr: true,
		},
		{
			name:    "module not in ownership map",
			module:  "unknown-module",
			agent:   "hermes",
			wantErr: true,
		},
		{
			name:    "empty module",
			module:  "",
			agent:   "hermes",
			wantErr: true,
		},
		{
			name:    "empty agent",
			module:  "jira-loop",
			agent:   "",
			wantErr: true,
		},
		{
			name:    "valid second entry",
			module:  "argos-eye",
			agent:   "argos",
			wantErr: false,
		},
		{
			name:    "wrong agent for second entry",
			module:  "argos-eye",
			agent:   "hermes",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := evidence.ValidateOwnership(tc.module, tc.agent, ownership)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			// When an error is expected, it must be ErrOwnershipViolation.
			if tc.wantErr && err != nil {
				var ve *evidence.ErrOwnershipViolation
				if !errors.As(err, &ve) {
					t.Errorf("error type = %T, want *ErrOwnershipViolation", err)
				}
			}
		})
	}
}

func TestErrOwnershipViolation_Error(t *testing.T) {
	err := &evidence.ErrOwnershipViolation{
		Module:        "jira-loop",
		ClaimedAgent:  "argos",
		ExpectedAgent: "hermes",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("Error() returned empty string")
	}
}
