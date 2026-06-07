package rest

import (
	"fmt"
	"strings"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// resolveTransition implements the category-primary / ordinal-secondary /
// name-tiebreak algorithm described in Design §Transition resolution algorithm.
//
// Returns:
//   - ("", nil)  — no candidates for targetCat → no-op (issue already there)
//   - (id, nil)  — transition id to apply
//   - ("", err)  — ambiguous / unreachable (ordinal out of bounds — never guess)
//
// The orderedStates map maps a StatusCategory key to an ordered slice of state
// names. When multiple transitions share the same category, the desired state
// is orderedStates[targetCat][ordinal]. Name matching is case-insensitive.
// If the name is not found, the function falls back to candidates[ordinal] by
// position. If ordinal is out of bounds for both, it returns an error.
func resolveTransition(
	transitions []evidence.Transition,
	targetCat string,
	ordinal int,
	orderedStates map[string][]string,
) (string, error) {
	// Step 1: filter by target category.
	var candidates []evidence.Transition
	for _, t := range transitions {
		if t.ToCategory == targetCat {
			candidates = append(candidates, t)
		}
	}

	// Step 2: zero candidates → no-op signal.
	if len(candidates) == 0 {
		return "", nil
	}

	// Step 3: exactly one candidate → return directly.
	if len(candidates) == 1 {
		return candidates[0].ID, nil
	}

	// Step 4: multiple candidates → ordinal-secondary + name-tiebreak.

	// Try name match first (case-insensitive).
	orderedNames := orderedStates[targetCat]
	if ordinal < len(orderedNames) {
		desired := strings.ToLower(orderedNames[ordinal])
		for _, c := range candidates {
			if strings.ToLower(c.ToName) == desired {
				return c.ID, nil
			}
		}
	}

	// Name not matched — fall back to ordinal position in candidates slice.
	if ordinal < len(candidates) {
		return candidates[ordinal].ID, nil
	}

	// Ordinal out of bounds for both ordered names and candidates — error.
	return "", fmt.Errorf(
		"rest: cannot resolve transition to category %q: ordinal %d out of bounds (candidates=%d, orderedNames=%d)",
		targetCat, ordinal, len(candidates), len(orderedNames),
	)
}
