// Package cichecks contains the pure domain logic for the ci-checks CLI.
package cichecks

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNoJiraKey is returned when the branch name does not match the canonical
// agent/<figura>/TAL-N format and no JIRA key can be extracted.
var ErrNoJiraKey = errors.New("ci-checks: branch has no extractable JIRA key (expected agent/<figura>/TAL-N)")

// ErrLabelInvariant is returned when one or more label invariant sub-rules (a)(b)(c)(d) are violated.
type ErrLabelInvariant struct {
	// Violations holds one message per violated sub-rule.
	Violations []string
}

// Error returns a formatted list of all violations.
func (e *ErrLabelInvariant) Error() string {
	return "ci-checks: label invariant violated: " + strings.Join(e.Violations, "; ")
}

// ErrMalformedOwnership is returned when the ownership table cannot be parsed correctly.
type ErrMalformedOwnership struct {
	// Detail names the problematic module or row.
	Detail string
}

// Error returns a message identifying the malformed field or row.
func (e *ErrMalformedOwnership) Error() string {
	return fmt.Sprintf("ci-checks: malformed ownership table: %s", e.Detail)
}

// ErrIssueNotFound is returned when Jira returns HTTP 404 for the requested key.
type ErrIssueNotFound struct {
	// Key is the JIRA key that was not found.
	Key string
}

// Error returns a message naming the missing key.
func (e ErrIssueNotFound) Error() string {
	return fmt.Sprintf("ci-checks: issue %q not found in Jira (HTTP 404)", e.Key)
}
