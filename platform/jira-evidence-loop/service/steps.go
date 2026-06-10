package service

import "fmt"

// ---------------------------------------------------------------------------
// Step type and constants
// ---------------------------------------------------------------------------

// Step identifies a single unit of work in the EvidenceLoop. Steps are
// numbered to match the documented 7-step flow (0-indexed for clarity).
type Step int

const (
	// StepOwnership (0): pure ownership-guard check — no API call.
	StepOwnership Step = iota
	// StepCreate (1): idempotent JQL search + CreateIssue if needed.
	StepCreate
	// StepTransitionInProgress (2): transition issue → indeterminate (In Progress).
	StepTransitionInProgress
	// StepComment (3): add an ADF comment to the issue.
	StepComment
	// StepWorklog (4): add a worklog entry to the issue.
	StepWorklog
	// StepRemoteLink (5): upsert a remote link (PR URL) — fail-loud (§7).
	StepRemoteLink
	// StepAttach (6): upload an attachment — fail-loud (§7).
	StepAttach
	// StepTransitionDone (7): transition issue → done.
	StepTransitionDone
	// StepTransitionToDo (8): transition issue → new (To Do). Used by the
	// "reset" preset (CONSTITUTION §11 rollback) to undo a dispatch that failed
	// mid-flight, so the issue does not remain In Progress or Done.
	StepTransitionToDo
)

// StepSet is the set of steps to execute in a single RunSteps call.
// A step is enabled when its key maps to true.
type StepSet map[Step]bool

// AllSteps returns a StepSet containing every step — equivalent to the
// original Run(ctx, in) behaviour. Back-compat: Run delegates to
// RunSteps(ctx, in, AllSteps()).
func AllSteps() StepSet {
	return StepSet{
		StepOwnership:            true,
		StepCreate:               true,
		StepTransitionInProgress: true,
		StepComment:              true,
		StepWorklog:              true,
		StepRemoteLink:           true,
		StepAttach:               true,
		StepTransitionDone:       true,
	}
}

// PhasePreset returns the StepSet for the given SDD phase name, following
// the phase→steps mapping from Design §D3. Returns an error for unknown phases
// (fail-loud per §7).
//
// Mapping:
//
//	propose  → ownership, create, →InProgress, comment
//	spec     → comment
//	design   → comment
//	tasks    → comment, worklog
//	apply    → comment, worklog
//	verify   → comment, remote-link, attach
//	archive  → →Done
func PhasePreset(phase string) (StepSet, error) {
	switch phase {
	case "propose":
		return StepSet{
			StepOwnership:            true,
			StepCreate:               true,
			StepTransitionInProgress: true,
			StepComment:              true,
		}, nil
	case "spec", "design":
		return StepSet{
			StepComment: true,
		}, nil
	case "tasks", "apply":
		return StepSet{
			StepComment:  true,
			StepWorklog:  true,
		}, nil
	case "verify":
		return StepSet{
			StepComment:    true,
			StepRemoteLink: true,
			StepAttach:     true,
		}, nil
	case "archive":
		return StepSet{
			StepTransitionDone: true,
		}, nil
	case "reset":
		// §11 rollback preset: transition issue back to To Do + add a failure
		// comment. Does NOT create, log work, or transition to Done.
		return StepSet{
			StepTransitionToDo: true,
			StepComment:        true,
		}, nil
	default:
		return nil, fmt.Errorf("evidence: unknown SDD phase %q; valid phases: propose, spec, design, tasks, apply, verify, archive, reset", phase)
	}
}
