package cichecks

import "fmt"

// InvariantInput holds all the data needed to evaluate the label invariant §4.
type InvariantInput struct {
	// Labels is the parsed set of Jira labels for the issue.
	Labels LabelSet
	// Ownership is the lowercased module→agent map from team-context/ownership.md.
	Ownership map[string]string
	// BranchFigura is the agent figure extracted from the branch name (lowercase).
	BranchFigura string
}

// InvariantResult holds the outcome of a label invariant check.
type InvariantResult struct {
	// Violations contains one message per violated sub-rule; empty means all rules passed.
	Violations []string
}

// CheckLabelInvariant evaluates all four sub-rules of the label invariant §4.
// It accumulates ALL violations rather than stopping at the first.
// The second return value is always nil; violations are returned in InvariantResult.Violations.
func CheckLabelInvariant(in InvariantInput) (InvariantResult, error) {
	var violations []string

	// Sub-rule (a): exactly one agent:* label.
	if err := in.Labels.Validate([]string{"agent"}); err != nil {
		agentCount := countKey(in.Labels, "agent")
		violations = append(violations, fmt.Sprintf(
			"sub-rule (a): %d labels agent:* found (required exactly 1)", agentCount,
		))
	}

	// Sub-rule (b): exactly one module:* label.
	if err := in.Labels.Validate([]string{"module"}); err != nil {
		moduleCount := countKey(in.Labels, "module")
		violations = append(violations, fmt.Sprintf(
			"sub-rule (b): %d labels module:* found (required exactly 1)", moduleCount,
		))
	}

	// Extract agent and module values; sub-rules (c) and (d) require exactly one of each.
	agentVal, hasAgent := in.Labels.Get("agent")
	moduleVal, hasModule := in.Labels.Get("module")

	// Sub-rule (c): ownership[module] == agent.
	if hasAgent && hasModule {
		moduleLabel := "module:" + moduleVal
		if err := ValidateOwnership(moduleLabel, agentVal, in.Ownership); err != nil {
			violations = append(violations, fmt.Sprintf(
				"sub-rule (c): ownership check failed: %s", err.Error(),
			))
		}
	}

	// Sub-rule (d): branch figura == label agent.
	if hasAgent && in.BranchFigura != agentVal {
		violations = append(violations, fmt.Sprintf(
			"sub-rule (d): branch figura %q != label agent %q",
			in.BranchFigura, agentVal,
		))
	}

	return InvariantResult{Violations: violations}, nil
}

// countKey counts how many labels in ls have the given key.
func countKey(ls LabelSet, key string) int {
	n := 0
	for _, l := range ls {
		if l.Key == key {
			n++
		}
	}
	return n
}
