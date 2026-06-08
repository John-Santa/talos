// Package overlap contains the pure domain logic for the overlap-guard CLI.
package overlap

import "fmt"

// ErrSameFileParallel is returned when a file collision is detected between two distinct agents.
type ErrSameFileParallel struct {
	File string
	A, B Claim
}

func (e *ErrSameFileParallel) Error() string {
	return fmt.Sprintf("file collision on %q: agents %q and %q are both touching it in parallel", e.File, e.A.Agent, e.B.Agent)
}

// ErrEscalateZeus signals an overlap condition requiring human judgement beyond ATHENA's authority.
type ErrEscalateZeus struct {
	Reason string
}

func (e *ErrEscalateZeus) Error() string {
	return fmt.Sprintf("escalate to ZEUS: %s", e.Reason)
}

// ErrChecklistMissing is emitted as an advisory when an issue body has no parseable files: section.
type ErrChecklistMissing struct {
	IssueKey string
}

func (e *ErrChecklistMissing) Error() string {
	return fmt.Sprintf("issue %s has no parseable files: checklist (advisory only — not treated as no overlap)", e.IssueKey)
}

// ErrNoClaims is returned when the files-file provided to ov check contains zero parseable paths.
type ErrNoClaims struct{}

func (e *ErrNoClaims) Error() string {
	return "no claims: the provided files list contains zero parseable paths"
}
