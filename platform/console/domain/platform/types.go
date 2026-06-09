// Package platform contains the pure domain types for the console module.
package platform

// MergePlanStep is a single entry in a MergePlan's Steps list.
// Fields map directly to the planStepJSON keys emitted by `mo plan --json`.
type MergePlanStep struct {
	Position       int      `json:"position"`
	Branch         string   `json:"branch"`
	Figura         string   `json:"figura"`
	CommitsAhead   int      `json:"commits_ahead"`
	PredictedClean bool     `json:"predicted_clean"`
	ConflictFiles  []string `json:"conflict_files,omitempty"`
}

// MergePlan is the JSON output shape for `mo plan --json`.
type MergePlan struct {
	BaseBranch      string          `json:"base_branch"`
	BaseTip         string          `json:"base_tip"`
	ConflictRate    float64         `json:"conflict_rate"`
	Threshold       float64         `json:"threshold"`
	SegmentationBad bool            `json:"segmentation_bad"`
	Steps           []MergePlanStep `json:"steps"`
}

// FileCollision is a single entry in Overlap's FileCollisions list.
type FileCollision struct {
	File   string   `json:"file"`
	Agents []string `json:"agents"`
}

// ModuleOverlap is a single entry in Overlap's ModuleOverlaps list.
type ModuleOverlap struct {
	Module string   `json:"module"`
	Agents []string `json:"agents"`
}

// Overlap is the JSON output shape for `ov scan --json`.
type Overlap struct {
	Verdict        string          `json:"verdict"`
	CollisionRate  float64         `json:"collision_rate"`
	PairsEvaluated int             `json:"pairs_evaluated"`
	CollidingPairs int             `json:"colliding_pairs"`
	FileCollisions []FileCollision `json:"file_collisions"`
	ModuleOverlaps []ModuleOverlap `json:"module_overlaps"`
	Advisories     []string        `json:"advisories"`
}

// Labels is the JSON output shape for `ch labels --branch <branch> --json`.
type Labels struct {
	Branch     string   `json:"branch"`
	JiraKey    string   `json:"jira_key"`
	Figura     string   `json:"figura"`
	Verdict    string   `json:"verdict"`
	Labels     []string `json:"labels"`
	Agent      string   `json:"agent"`
	Module     string   `json:"module"`
	Violations []string `json:"violations"`
}
