package run

import (
	"sort"
	"time"
)

// ActivityEntry is a timeline entry — json tags MUST match webapi/domain.ActivityEntry exactly.
// webapi: At string `json:"at"`, Text string `json:"text"`
type ActivityEntry struct {
	At   string `json:"at"`
	Text string `json:"text"`
}

// DoDItem is a Definition-of-Done checklist entry — json tags MUST match webapi/domain.DoDItem exactly.
// webapi: Label string `json:"label"`, State string `json:"state"`, Kind string `json:"kind"`
type DoDItem struct {
	Label string `json:"label"`
	State string `json:"state"`
	Kind  string `json:"kind"`
}

// Judge is one blind judge's verdict — json tags MUST match webapi/domain.Judge exactly.
// webapi: ID string `json:"id"`, Verdict string `json:"verdict"`, Note string `json:"note"`
type Judge struct {
	ID      string `json:"id"`
	Verdict string `json:"verdict"`
	Note    string `json:"note"`
}

// JudgmentReview is the Judgment Day payload — json tags MUST match webapi/domain.JudgmentReview exactly.
// webapi: jiraKey, gate, judges, fixAgent, verdict, escalateTo (omitempty), pending (omitempty)
type JudgmentReview struct {
	JiraKey    string  `json:"jiraKey"`
	Gate       string  `json:"gate"`
	Judges     []Judge `json:"judges"`
	FixAgent   string  `json:"fixAgent"`
	Verdict    string  `json:"verdict"`
	EscalateTo string  `json:"escalateTo,omitempty"`
	Pending    bool    `json:"pending,omitempty"`
}

// RunView is the read-model for a single dispatch event entry.
type RunView struct {
	At      time.Time `json:"at"`
	JiraKey string    `json:"jiraKey"`
	Change  string    `json:"change,omitempty"`
	Phase   Phase     `json:"phase,omitempty"`
	Agent   string    `json:"agent,omitempty"`
	Module  string    `json:"module,omitempty"`
	Status  string    `json:"status,omitempty"`
	Outcome string    `json:"outcome,omitempty"`
}

// Filter selects events by optional criteria.
type Filter struct {
	JiraKey string
	Agent   string
	Kind    Kind
	Since   time.Time
	Limit   int
}

// ProjectActivity projects kind=activity events into an ordered timeline.
// Events are sorted ascending by At. Returns a non-nil slice.
func ProjectActivity(evs []RunEvent) []ActivityEntry {
	result := make([]ActivityEntry, 0)
	var acts []RunEvent
	for _, e := range evs {
		if e.Kind == KindActivity {
			acts = append(acts, e)
		}
	}
	sort.Slice(acts, func(i, j int) bool {
		return acts[i].At.Before(acts[j].At)
	})
	for _, e := range acts {
		result = append(result, ActivityEntry{
			At:   e.At.UTC().Format(time.RFC3339),
			Text: e.Text,
		})
	}
	return result
}

// ProjectDoD projects kind=metric metric="dod" events into a checklist.
// Last-wins per label (by At). Returns a non-nil slice.
func ProjectDoD(evs []RunEvent) []DoDItem {
	result := make([]DoDItem, 0)
	// last-wins by label: track latest At per label
	type entry struct {
		item DoDItem
		at   time.Time
	}
	byLabel := make(map[string]entry)
	for _, e := range evs {
		if e.Kind != KindMetric || e.Metric != "dod" {
			continue
		}
		prev, ok := byLabel[e.Label]
		if !ok || e.At.After(prev.at) {
			byLabel[e.Label] = entry{
				item: DoDItem{
					Label: e.Label,
					State: e.DoDState,
					Kind:  kindFor(e.Label),
				},
				at: e.At,
			}
		}
	}
	// stable ordering by label
	labels := make([]string, 0, len(byLabel))
	for l := range byLabel {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for _, l := range labels {
		result = append(result, byLabel[l].item)
	}
	return result
}

// ProjectJudgment projects kind=judgment events for a jiraKey into a JudgmentReview.
// Takes the last event by At (last-wins). Returns Pending:true if no events.
func ProjectJudgment(jiraKey string, evs []RunEvent) JudgmentReview {
	var latest *RunEvent
	for i := range evs {
		e := &evs[i]
		if e.Kind != KindJudgment || e.JiraKey != jiraKey {
			continue
		}
		if latest == nil || e.At.After(latest.At) {
			latest = e
		}
	}
	if latest == nil {
		return JudgmentReview{JiraKey: jiraKey, Pending: true}
	}
	judges := make([]Judge, 0, len(latest.Judges))
	for _, id := range latest.Judges {
		judges = append(judges, Judge{ID: id, Verdict: latest.Verdict})
	}
	return JudgmentReview{
		JiraKey:    jiraKey,
		Gate:       "HG5",
		Judges:     judges,
		FixAgent:   latest.FixAgent,
		Verdict:    latest.Verdict,
		EscalateTo: latest.EscalateTo,
		Pending:    false,
	}
}

// ProjectRuns projects kind=dispatch events according to a Filter into RunViews.
// Returns a non-nil slice sorted ascending by At.
func ProjectRuns(evs []RunEvent, f Filter) []RunView {
	result := make([]RunView, 0)
	for _, e := range evs {
		if e.Kind != KindDispatch {
			continue
		}
		if f.JiraKey != "" && e.JiraKey != f.JiraKey {
			continue
		}
		if f.Agent != "" && e.Agent != f.Agent {
			continue
		}
		if !f.Since.IsZero() && e.At.Before(f.Since) {
			continue
		}
		result = append(result, RunView{
			At:      e.At,
			JiraKey: e.JiraKey,
			Change:  e.Change,
			Phase:   e.Phase,
			Agent:   e.Agent,
			Module:  e.Module,
			Status:  e.Status,
			Outcome: e.Outcome,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].At.Before(result[j].At)
	})
	if f.Limit > 0 && len(result) > f.Limit {
		result = result[:f.Limit]
	}
	return result
}

// kindFor infers a DoDItem.Kind from its label text.
// This is a heuristic helper; callers can override via e.Label conventions.
func kindFor(label string) string {
	switch label {
	case "PR linked":
		return "pr"
	case "CI green":
		return "ci"
	case "verify-report":
		return "verify"
	default:
		return "check"
	}
}
