// Package cichecks contains the pure domain logic for the ci-checks CLI.
package cichecks

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNoJiraKey is returned when the branch name does not match the canonical
// agent/<figura>/<projectKey>-N format and no Jira key can be extracted.
var ErrNoJiraKey = errors.New("ci-checks: branch has no extractable Jira key (expected agent/<figura>/<projectKey>-N)")

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

// ErrNoJudgmentReport is returned when the judgment-report.md file is absent
// at the canonical path openspec/changes/{change}/judgment-report.md.
var ErrNoJudgmentReport = errors.New("ci-checks: judgment-report.md not found")

// ErrJudgmentNotApproved is returned when the report exists but the verdict is
// not APPROVED (ESCALATED, malformed header, Round < 1, change-mismatch, or
// missing terminal JUDGMENT: line).
type ErrJudgmentNotApproved struct {
	// Change is the slug that was requested.
	Change string
	// Verdict is the parsed verdict string, or "MALFORMED" / "ESCALATED".
	Verdict string
}

// Error returns a message describing the non-approved state.
func (e *ErrJudgmentNotApproved) Error() string {
	return fmt.Sprintf("ci-checks: judgment report for %q is not approved (verdict: %s)", e.Change, e.Verdict)
}

// ErrMalformedJudgment is an internal sentinel for parse failures inside the
// domain; it is collapsed to ErrJudgmentNotApproved at the cmd I/O boundary.
type ErrMalformedJudgment struct {
	// Detail describes the specific parse failure.
	Detail string
}

// Error returns a message identifying the malformed field or condition.
func (e *ErrMalformedJudgment) Error() string {
	return fmt.Sprintf("ci-checks: malformed judgment report: %s", e.Detail)
}
