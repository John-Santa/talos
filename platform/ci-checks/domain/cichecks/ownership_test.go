package cichecks_test

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func TestValidateOwnership_OK(t *testing.T) {
	err := cichecks.ValidateOwnership("module:devops", "hermes", map[string]string{"module:devops": "hermes"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateOwnership_Mismatch(t *testing.T) {
	err := cichecks.ValidateOwnership("module:devops", "atlas", map[string]string{"module:devops": "hermes"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ve *cichecks.ErrOwnershipViolation
	if !errors.As(err, &ve) {
		t.Fatalf("error type = %T, want *ErrOwnershipViolation", err)
	}
	if ve.ExpectedAgent != "hermes" {
		t.Errorf("ExpectedAgent = %q, want %q", ve.ExpectedAgent, "hermes")
	}
	if ve.ClaimedAgent != "atlas" {
		t.Errorf("ClaimedAgent = %q, want %q", ve.ClaimedAgent, "atlas")
	}
}

func TestValidateOwnership_UnknownModule(t *testing.T) {
	err := cichecks.ValidateOwnership("module:unknown", "hermes", map[string]string{"module:devops": "hermes"})
	if err == nil {
		t.Fatal("expected error for unknown module, got nil")
	}
	var ve *cichecks.ErrOwnershipViolation
	if !errors.As(err, &ve) {
		t.Fatalf("error type = %T, want *ErrOwnershipViolation", err)
	}
	if ve.ExpectedAgent != "" {
		t.Errorf("ExpectedAgent = %q, want empty string for unregistered module", ve.ExpectedAgent)
	}
}

func TestErrOwnershipViolation_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *cichecks.ErrOwnershipViolation
	}{
		{
			name: "mismatch includes module and both agents",
			err: &cichecks.ErrOwnershipViolation{
				Module:        "module:devops",
				ClaimedAgent:  "atlas",
				ExpectedAgent: "hermes",
			},
		},
		{
			name: "unregistered module includes module and claimed agent",
			err: &cichecks.ErrOwnershipViolation{
				Module:       "module:unknown",
				ClaimedAgent: "hermes",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			if msg == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}
