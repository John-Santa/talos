package dispatch

import "fmt"

// ErrUnknownPhase is returned by PlanFor when the phase is not in the table.
type ErrUnknownPhase struct {
	Phase string
}

func (e *ErrUnknownPhase) Error() string {
	return fmt.Sprintf("orchestration: unknown phase %q", e.Phase)
}

// ErrOverlapBlock is returned when the overlap checker verdicts BLOCK.
type ErrOverlapBlock struct {
	Module string
	Agent  string
}

func (e *ErrOverlapBlock) Error() string {
	return fmt.Sprintf("orchestration: overlap BLOCK for module=%q agent=%q — dispatch aborted", e.Module, e.Agent)
}

// ErrMergeConflict is returned when the merge gate detects conflicts above threshold.
type ErrMergeConflict struct {
	ConflictRate float64
	Threshold    float64
}

func (e *ErrMergeConflict) Error() string {
	return fmt.Sprintf("orchestration: merge conflict rate %.2f exceeds threshold %.2f — recipe §11 triggered", e.ConflictRate, e.Threshold)
}

// ErrAlreadyDispatched is returned when more than one worktree exists for the same figura.
type ErrAlreadyDispatched struct {
	Figura string
	Count  int
}

func (e *ErrAlreadyDispatched) Error() string {
	return fmt.Sprintf("orchestration: figura %q has %d worktrees (expected <=1) — inconsistency detected", e.Figura, e.Count)
}

// ErrQueuedSerialize is returned when the overlap verdict is SERIALIZE.
// It is NOT a fatal error; the caller should defer dispatch and inform the operator.
type ErrQueuedSerialize struct {
	Module string
	Agent  string
}

func (e *ErrQueuedSerialize) Error() string {
	return fmt.Sprintf("orchestration: dispatch queued (SERIALIZE) for module=%q agent=%q — retry when prior work completes", e.Module, e.Agent)
}
