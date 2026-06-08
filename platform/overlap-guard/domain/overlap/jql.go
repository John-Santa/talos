package overlap

import "fmt"

// BuildPreAssignmentJQL builds the overlap-protocol.md JQL for the T0 gate, excluding the owner's own agent label.
func BuildPreAssignmentJQL(project, module, owner string) string {
	return fmt.Sprintf(
		`project = %s AND statusCategory != Done AND labels = "module:%s" AND labels NOT IN ("agent:%s")`,
		project, module, owner,
	)
}
