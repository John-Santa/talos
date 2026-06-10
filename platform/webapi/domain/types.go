// Package domain holds the web-facing types served by the gateway and the pure
// logic that maps the platform CLIs (wt/mo/ov/ch) onto them. The JSON tags match
// the talos-front TalosRepository contract exactly (camelCase), so the front's
// httpRepository consumes these responses verbatim.
package domain

// Worktree is one agent worktree as the console shows it.
type Worktree struct {
	Agent   string `json:"agent"`
	JiraKey string `json:"jiraKey"`
	Branch  string `json:"branch"`
	Head    string `json:"head"`
	Module  string `json:"module"`
	Status  string `json:"status"`
	Ahead   int    `json:"ahead"`
}

// MergeItem is one ordered step in the merge plan.
type MergeItem struct {
	N             int      `json:"n"`
	Agent         string   `json:"agent"`
	JiraKey       string   `json:"jiraKey"`
	Ahead         int      `json:"ahead"`
	Ready         bool     `json:"ready"`
	ConflictFiles []string `json:"conflictFiles,omitempty"`
}

// MergeOrder is the merge plan toward the base branch.
type MergeOrder struct {
	Base         string      `json:"base"`
	ConflictRate float64     `json:"conflictRate"`
	Threshold    float64     `json:"threshold"`
	Items        []MergeItem `json:"items"`
}

// Pairs counts colliding vs evaluated module pairs.
type Pairs struct {
	Colliding int `json:"colliding"`
	Total     int `json:"total"`
}

// Overlap is the collision verdict across active worktrees.
type Overlap struct {
	CollisionRate  float64  `json:"collisionRate"`
	Pairs          Pairs    `json:"pairs"`
	Verdict        string   `json:"verdict"`
	FileCollisions []string `json:"fileCollisions,omitempty"`
	Advisories     []string `json:"advisories,omitempty"`
}

// Gate is an approval gate (HG0..HG7).
type Gate struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Note  string `json:"note,omitempty"`
}

// Slots reports worktree capacity usage.
type Slots struct {
	Used  int `json:"used"`
	Total int `json:"total"`
}

// OrchestrationSnapshot is the shared dataset for the Fiel/Consola/Flow screens.
type OrchestrationSnapshot struct {
	Worktrees  []Worktree `json:"worktrees"`
	MergeOrder MergeOrder `json:"mergeOrder"`
	Overlap    Overlap    `json:"overlap"`
	Gate       Gate       `json:"gate"`
	IdleAgents []string   `json:"idleAgents"`
	Slots      Slots      `json:"slots"`
}

// Agent is a figura in the roster.
type Agent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Model    string `json:"model"`
	HueToken string `json:"hueToken"`
}

// DoDItem is a Definition-of-Done checklist entry.
type DoDItem struct {
	Label string `json:"label"`
	State string `json:"state"`
	Kind  string `json:"kind"`
}

// ActivityEntry is a timeline entry on an agent detail.
type ActivityEntry struct {
	At   string `json:"at"`
	Text string `json:"text"`
}

// AgentDetail is the agent detail view payload.
type AgentDetail struct {
	Agent    Agent           `json:"agent"`
	Worktree *Worktree       `json:"worktree,omitempty"`
	DoD      []DoDItem       `json:"dod"`
	Activity []ActivityEntry `json:"activity"`
}

// Judge is one blind judge's verdict.
type Judge struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Note    string `json:"note"`
}

// JudgmentReview is the Judgment Day payload for an issue.
type JudgmentReview struct {
	JiraKey    string  `json:"jiraKey"`
	Gate       string  `json:"gate"`
	Judges     []Judge `json:"judges"`
	FixAgent   string  `json:"fixAgent"`
	Verdict    string  `json:"verdict"`
	EscalateTo string  `json:"escalateTo,omitempty"`
	// Pending is true when the judgment source (ch/Jira) is unavailable.
	// The front must not render a positive verdict when Pending is set.
	Pending bool `json:"pending,omitempty"`
}
