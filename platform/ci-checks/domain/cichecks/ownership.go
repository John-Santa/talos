package cichecks

import "fmt"

// ErrOwnershipViolation is returned when the claimed agent does not own the
// specified module according to the ownership map.
type ErrOwnershipViolation struct {
	// Module is the module label value (e.g. "module:devops").
	Module string
	// ClaimedAgent is the agent that was found on the issue.
	ClaimedAgent string
	// ExpectedAgent is the owner from the table; empty when module is not registered.
	ExpectedAgent string
}

// Error returns a human-readable description of the ownership violation.
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
// module according to the provided ownership map. It is a pure function with no I/O.
// The comparison is case-sensitive; callers must lowercase both sides before calling.
func ValidateOwnership(module, agent string, ownership map[string]string) error {
	if module == "" {
		return &ErrOwnershipViolation{Module: module, ClaimedAgent: agent}
	}
	if agent == "" {
		return &ErrOwnershipViolation{Module: module, ClaimedAgent: agent}
	}
	expected, ok := ownership[module]
	if !ok {
		return &ErrOwnershipViolation{Module: module, ClaimedAgent: agent}
	}
	if expected != agent {
		return &ErrOwnershipViolation{Module: module, ClaimedAgent: agent, ExpectedAgent: expected}
	}
	return nil
}
