package domain

import "strings"

// NormalizeFigura lowercases and strips an `agent:` prefix.
func NormalizeFigura(s string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "agent:"))
}

// ParseJiraKey extracts the JIRA-KEY from an `agent/<figura>/<JIRA-KEY>` branch.
func ParseJiraKey(branch string) string {
	parts := strings.Split(branch, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// asPercent converts a [0,1] fraction (the CLIs' unit) to a 0..100 percent.
func asPercent(fraction float64) float64 {
	return fraction * 100
}

func mapStatus(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return "active"
	case "stale", "orphan":
		return "idle"
	default:
		return "active"
	}
}

// moduleFor reverse-looks-up the module owned by a figura in the ownership map
// (which is module -> agent). The `module:` prefix is stripped from the result.
func moduleFor(figura string, ownership map[string]string) string {
	for module, agent := range ownership {
		if NormalizeFigura(agent) == figura {
			return strings.TrimPrefix(module, "module:")
		}
	}
	return ""
}

func aheadFor(branch string, plan MoPlan) int {
	for _, step := range plan.Steps {
		if step.Branch == branch {
			return step.CommitsAhead
		}
	}
	return 0
}

// MapWorktree maps a single CLI worktree entry to the web shape.
func MapWorktree(e WtEntry, ownership map[string]string, plan MoPlan) Worktree {
	figura := NormalizeFigura(e.Figura)
	return Worktree{
		Agent:   figura,
		JiraKey: ParseJiraKey(e.Branch),
		Branch:  e.Branch,
		Head:    e.Head,
		Module:  moduleFor(figura, ownership),
		Status:  mapStatus(e.Status),
		Ahead:   aheadFor(e.Branch, plan),
	}
}

func idleAgents(active map[string]bool) []string {
	out := make([]string, 0)
	for _, f := range devRoster {
		if !active[f] {
			out = append(out, f)
		}
	}
	return out
}

// DefaultGate is the synthesized gate state. The platform CLIs do not yet expose
// a live gate, so the gateway reports the standard final-merge gate.
func DefaultGate() Gate {
	return Gate{
		ID:    "HG3",
		State: "pending",
		Note:  "Merge final a main espera a ZEUS. DoD = PR + CI verde + verify-report.",
	}
}

// BuildSnapshot assembles the orchestration snapshot from the four CLI outputs.
func BuildSnapshot(wts []WtEntry, plan MoPlan, scan OvScan, ownership map[string]string) OrchestrationSnapshot {
	worktrees := make([]Worktree, 0, len(wts))
	active := map[string]bool{}
	for _, e := range wts {
		w := MapWorktree(e, ownership, plan)
		worktrees = append(worktrees, w)
		active[w.Agent] = true
	}

	items := make([]MergeItem, 0, len(plan.Steps))
	for _, s := range plan.Steps {
		items = append(items, MergeItem{
			N:       s.Position,
			Agent:   NormalizeFigura(s.Figura),
			JiraKey: ParseJiraKey(s.Branch),
			Ahead:   s.CommitsAhead,
			Ready:   s.PredictedClean,
		})
	}

	base := plan.BaseBranch
	if base == "" {
		base = "develop"
	}
	verdict := scan.Verdict
	if verdict == "" {
		verdict = "OK"
	}

	return OrchestrationSnapshot{
		Worktrees: worktrees,
		MergeOrder: MergeOrder{
			Base:         base,
			ConflictRate: asPercent(plan.ConflictRate),
			Threshold:    asPercent(plan.Threshold),
			Items:        items,
		},
		Overlap: Overlap{
			CollisionRate: asPercent(scan.CollisionRate),
			Pairs:         Pairs{Colliding: scan.CollidingPairs, Total: scan.PairsEvaluated},
			Verdict:       strings.ToUpper(verdict),
		},
		Gate:       DefaultGate(),
		IdleAgents: idleAgents(active),
		Slots:      Slots{Used: len(worktrees), Total: 7},
	}
}

// MapJudgment maps `ch judgment --json` to the Judgment Day payload. Per-judge
// verdicts are not exposed by the CLI, so they inherit the overall outcome.
func MapJudgment(jiraKey string, cj ChJudgment) JudgmentReview {
	conflict := len(cj.Violations) > 0 ||
		strings.EqualFold(cj.Verdict, "conflict") ||
		strings.EqualFold(cj.Verdict, "changes") ||
		strings.EqualFold(cj.Verdict, "changes_requested")

	judges := make([]Judge, 0, len(cj.Judges))
	verdict := "APPROVED"
	if conflict {
		verdict = "CHANGES"
	}
	for _, name := range cj.Judges {
		judges = append(judges, Judge{ID: name, Verdict: verdict, Note: strings.Join(cj.Violations, "; ")})
	}

	review := JudgmentReview{
		JiraKey:  jiraKey,
		Gate:     "HG5",
		Judges:   judges,
		FixAgent: "idle",
		Verdict:  "agree",
	}
	if conflict {
		review.Verdict = "conflict"
		review.EscalateTo = "zeus"
	}
	return review
}
