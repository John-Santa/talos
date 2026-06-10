package domain

// These mirror the JSON shapes emitted by the platform CLIs with their
// snake_case tags, so the gateway can unmarshal `wt/mo/ov/ch ... --json` output.

// WtEntry is one row of `wt list --json`.
type WtEntry struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}

// MoPlanStep is one step of `mo plan --json`.
type MoPlanStep struct {
	Position       int      `json:"position"`
	Branch         string   `json:"branch"`
	Figura         string   `json:"figura"`
	CommitsAhead   int      `json:"commits_ahead"`
	PredictedClean bool     `json:"predicted_clean"`
	ConflictFiles  []string `json:"conflict_files"`
}

// MoPlan is the `mo plan --json` output. Rates are fractions in [0,1].
type MoPlan struct {
	BaseBranch      string       `json:"base_branch"`
	BaseTip         string       `json:"base_tip"`
	ConflictRate    float64      `json:"conflict_rate"`
	Threshold       float64      `json:"threshold"`
	SegmentationBad bool         `json:"segmentation_bad"`
	Steps           []MoPlanStep `json:"steps"`
}

// OvScan is the `ov scan --json` output. CollisionRate is a fraction in [0,1].
type OvScan struct {
	Verdict        string  `json:"verdict"`
	CollisionRate  float64 `json:"collision_rate"`
	PairsEvaluated int     `json:"pairs_evaluated"`
	CollidingPairs int     `json:"colliding_pairs"`
}

// ChOwnership is the `ch ownership --json` output (module -> agent).
type ChOwnership struct {
	Modules map[string]string `json:"modules"`
}

// ChJudgment is the `ch judgment --json` output.
type ChJudgment struct {
	Change      string   `json:"change"`
	Round       int      `json:"round"`
	Judges      []string `json:"judges"`
	Implementor string   `json:"implementor"`
	Verdict     string   `json:"verdict"`
	Violations  []string `json:"violations"`
}
