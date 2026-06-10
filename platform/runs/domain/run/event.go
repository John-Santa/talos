// Package run contains the pure domain logic for the runs module.
package run

import "time"

// Kind discriminates the type of run event.
type Kind string

const (
	KindDispatch Kind = "dispatch"
	KindActivity Kind = "activity"
	KindJudgment Kind = "judgment"
	KindMetric   Kind = "metric"
)

var validKinds = map[Kind]bool{
	KindDispatch: true,
	KindActivity: true,
	KindJudgment: true,
	KindMetric:   true,
}

// Phase is a canonical SDD lifecycle phase name.
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

// RunEvent is the polymorphic append-log record discriminated by Kind.
// Schema version v:1. One JSON object per JSONL line.
type RunEvent struct {
	V         int       `json:"v"`                    // schema version = 1
	Kind      Kind      `json:"kind"`
	At        time.Time `json:"at"`                   // ISO-8601; temporal ordering key
	Workspace string    `json:"workspace,omitempty"`
	JiraKey   string    `json:"jiraKey,omitempty"`
	Change    string    `json:"change,omitempty"`
	Phase     Phase     `json:"phase,omitempty"`
	Agent     string    `json:"agent,omitempty"`      // figura identifier
	Module    string    `json:"module,omitempty"`
	Status    string    `json:"status,omitempty"`     // dispatch: started|done|failed|queued
	Outcome   string    `json:"outcome,omitempty"`

	// activity fields
	Text string `json:"text,omitempty"`

	// judgment fields
	Verdict    string   `json:"verdict,omitempty"`
	Judges     []string `json:"judges,omitempty"`
	FixAgent   string   `json:"fixAgent,omitempty"`
	EscalateTo string   `json:"escalateTo,omitempty"`
	Violations []string `json:"violations,omitempty"`

	// metric fields
	Metric   string  `json:"metric,omitempty"`   // "conflict_rate" | "dod"
	Value    float64 `json:"value,omitempty"`
	Label    string  `json:"label,omitempty"`    // dod item label
	DoDState string  `json:"dodState,omitempty"` // done|pending
}

// Validate checks required invariants for a RunEvent.
// Returns typed domain errors: ErrInvalidKind, ErrMissingJiraKey, ErrInvalidEvent.
func (e RunEvent) Validate() error {
	if !validKinds[e.Kind] {
		return &ErrInvalidKind{Kind: e.Kind}
	}
	if e.At.IsZero() {
		return &ErrInvalidEvent{Reason: "at timestamp is required"}
	}
	// metrics may be global (no jiraKey); all other kinds require one
	if e.Kind != KindMetric && e.JiraKey == "" {
		return &ErrMissingJiraKey{}
	}
	return nil
}
