// Package dispatch contains the pure dispatch domain: WorkItem, Phase, Verdict,
// DispatchAction, DispatchPlan and the policy functions DecideOnOverlap and PlanFor.
// No I/O — no imports of external packages beyond the standard library.
package dispatch

// Phase represents an SDD workflow phase.
type Phase string

const (
	PhasePropose Phase = "propose"
	PhaseSpec    Phase = "spec"
	PhaseDesign  Phase = "design"
	PhaseTasks   Phase = "tasks"
	PhaseApply   Phase = "apply"
	PhaseVerify  Phase = "verify"
	PhaseArchive Phase = "archive"
)

// OverlapVerdict is the verdict returned by the overlap checker.
type OverlapVerdict string

const (
	VerdictBlock     OverlapVerdict = "BLOCK"
	VerdictSerialize OverlapVerdict = "SERIALIZE"
	VerdictOK        OverlapVerdict = "OK"
)

// DispatchAction is what the dispatcher should do given an overlap verdict.
type DispatchAction string

const (
	ActionProceed DispatchAction = "PROCEED"
	ActionQueue   DispatchAction = "QUEUE"
	ActionAbort   DispatchAction = "ABORT"
)

// WorkItem describes a unit of SDD work to dispatch.
type WorkItem struct {
	JiraKey string // TAL-N; may be empty for propose (evidence creates the issue)
	Change  string
	Agent   string // figura
	Module  string // e.g. module:orchestration
}

// DispatchPlan holds the booleans that control which non-Jira steps run for a phase.
type DispatchPlan struct {
	EnsureWorktree bool // create worktree idempotently
	OverlapGate    bool // run ov check before proceeding
	MergeGate      bool // run mo check/plan before proceeding (verify/archive)
	Cleanup        bool // teardown worktree after Done (archive)
}

// DecideOnOverlap maps an overlap verdict to a dispatch action.
// Unknown verdicts → ABORT (fail-loud).
func DecideOnOverlap(v OverlapVerdict) DispatchAction {
	switch v {
	case VerdictOK:
		return ActionProceed
	case VerdictSerialize:
		return ActionQueue
	case VerdictBlock:
		return ActionAbort
	default:
		return ActionAbort
	}
}

// planTable defines the non-Jira step matrix for each phase.
// propose is the entry point: worktree+overlap gate always runs.
// spec/design/tasks/apply: idempotent worktree ensure; no merge gate.
// verify: worktree ensure + overlap gate + merge gate; no cleanup.
// archive: worktree ensure + overlap gate + merge gate + cleanup (teardown).
var planTable = map[Phase]DispatchPlan{
	PhasePropose: {EnsureWorktree: true, OverlapGate: true, MergeGate: false, Cleanup: false},
	PhaseSpec:    {EnsureWorktree: true, OverlapGate: false, MergeGate: false, Cleanup: false},
	PhaseDesign:  {EnsureWorktree: true, OverlapGate: false, MergeGate: false, Cleanup: false},
	PhaseTasks:   {EnsureWorktree: true, OverlapGate: false, MergeGate: false, Cleanup: false},
	PhaseApply:   {EnsureWorktree: true, OverlapGate: false, MergeGate: false, Cleanup: false},
	PhaseVerify:  {EnsureWorktree: true, OverlapGate: true, MergeGate: true, Cleanup: false},
	PhaseArchive: {EnsureWorktree: true, OverlapGate: true, MergeGate: true, Cleanup: true},
}

// PlanFor returns the DispatchPlan for the given phase.
// Returns ErrUnknownPhase for unrecognised phases (fail-loud).
func PlanFor(p Phase) (DispatchPlan, error) {
	plan, ok := planTable[p]
	if !ok {
		return DispatchPlan{}, &ErrUnknownPhase{Phase: string(p)}
	}
	return plan, nil
}
