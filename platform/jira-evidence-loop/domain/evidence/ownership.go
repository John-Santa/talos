package evidence

import "fmt"

// ErrOwnershipViolation is returned when the claimed agent does not own the
// specified module according to the ownership map.
type ErrOwnershipViolation struct {
	Module        string
	ClaimedAgent  string
	ExpectedAgent string // empty when module is not registered
}

func (e *ErrOwnershipViolation) Error() string {
	if e.ExpectedAgent == "" {
		return fmt.Sprintf(
			"ownership violation: module %q is not registered in the ownership map (claimed by %q)",
			e.Module, e.ClaimedAgent,
		)
	}
	return fmt.Sprintf(
		"ownership violation: module %q is owned by %q, not %q",
		e.Module, e.ExpectedAgent, e.ClaimedAgent,
	)
}

// ValidateOwnership checks that the given agent legitimately owns the given
// module according to the provided ownership map. It is a pure function with
// no I/O.
//
// Rules (REQ-LABEL, Design step-0):
//   - module must be non-empty
//   - agent must be non-empty
//   - module must exist in the ownership map
//   - ownership[module] must equal agent (case-sensitive)
func ValidateOwnership(module, agent string, ownership map[string]string) error {
	if module == "" {
		return &ErrOwnershipViolation{
			Module:       module,
			ClaimedAgent: agent,
		}
	}
	if agent == "" {
		return &ErrOwnershipViolation{
			Module:       module,
			ClaimedAgent: agent,
		}
	}
	expected, ok := ownership[module]
	if !ok {
		return &ErrOwnershipViolation{
			Module:       module,
			ClaimedAgent: agent,
		}
	}
	if expected != agent {
		return &ErrOwnershipViolation{
			Module:        module,
			ClaimedAgent:  agent,
			ExpectedAgent: expected,
		}
	}
	return nil
}
