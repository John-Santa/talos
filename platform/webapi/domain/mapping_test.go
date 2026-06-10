package domain

import "testing"

func TestParseJiraKey(t *testing.T) {
	if got := ParseJiraKey("agent/hermes/TAL-15"); got != "TAL-15" {
		t.Errorf("ParseJiraKey = %q, want TAL-15", got)
	}
	if got := ParseJiraKey(""); got != "" {
		t.Errorf("ParseJiraKey(empty) = %q, want empty", got)
	}
}

func TestNormalizeFigura(t *testing.T) {
	if got := NormalizeFigura("agent:HERMES"); got != "hermes" {
		t.Errorf("NormalizeFigura = %q, want hermes", got)
	}
}

func TestBuildSnapshot(t *testing.T) {
	wts := []WtEntry{
		{Figura: "hermes", Branch: "agent/hermes/TAL-15", Head: "2e61d7e", Status: "active"},
		{Figura: "iris", Branch: "agent/iris/TAL-22", Head: "ec3ff1a", Status: "active"},
	}
	plan := MoPlan{
		BaseBranch:   "develop",
		ConflictRate: 0.0,
		Threshold:    0.15,
		Steps: []MoPlanStep{
			{Position: 1, Branch: "agent/hermes/TAL-15", Figura: "hermes", CommitsAhead: 4, PredictedClean: true},
		},
	}
	scan := OvScan{Verdict: "ok", CollisionRate: 0.0, PairsEvaluated: 0, CollidingPairs: 0}
	own := map[string]string{"module:devops": "hermes", "module:frontend": "iris"}

	snap := BuildSnapshot(wts, plan, scan, own)

	if len(snap.Worktrees) != 2 {
		t.Fatalf("worktrees = %d, want 2", len(snap.Worktrees))
	}
	w := snap.Worktrees[0]
	if w.JiraKey != "TAL-15" || w.Module != "devops" || w.Ahead != 4 || w.Status != "active" {
		t.Errorf("worktree[0] mapped wrong: %+v", w)
	}
	if snap.MergeOrder.Threshold != 15 {
		t.Errorf("threshold = %v, want 15 (0.15 * 100)", snap.MergeOrder.Threshold)
	}
	if snap.MergeOrder.ConflictRate != 0 {
		t.Errorf("conflictRate = %v, want 0", snap.MergeOrder.ConflictRate)
	}
	if len(snap.MergeOrder.Items) != 1 || !snap.MergeOrder.Items[0].Ready {
		t.Errorf("merge items mapped wrong: %+v", snap.MergeOrder.Items)
	}
	if snap.Overlap.Verdict != "OK" {
		t.Errorf("verdict = %q, want OK", snap.Overlap.Verdict)
	}
	// Slots.Total must derive from devRoster length, not be hardcoded.
	if snap.Slots.Used != 2 || snap.Slots.Total != len(devRoster) {
		t.Errorf("slots = %+v, want {Used:2, Total:%d}", snap.Slots, len(devRoster))
	}
	if len(snap.IdleAgents) != 5 {
		t.Errorf("idleAgents = %v, want 5 (devRoster minus hermes,iris)", snap.IdleAgents)
	}
	if snap.Gate.ID != "HG3" || snap.Gate.State != "pending" {
		t.Errorf("gate = %+v, want HG3 pending", snap.Gate)
	}
}

// TestSlotsTotal verifies BuildSnapshot derives Total from devRoster, not a literal 7.
func TestSlotsTotal(t *testing.T) {
	snap := BuildSnapshot(nil, MoPlan{}, OvScan{}, nil)
	if snap.Slots.Total != len(devRoster) {
		t.Errorf("Slots.Total = %d, want %d (len(devRoster))", snap.Slots.Total, len(devRoster))
	}
}

func TestMapJudgmentConflict(t *testing.T) {
	cj := ChJudgment{
		Judges:     []string{"jd-judge-a", "jd-judge-b"},
		Verdict:    "changes",
		Violations: []string{"verify-report missing"},
	}
	rev := MapJudgment("TAL-15", cj)
	if rev.Verdict != "conflict" {
		t.Errorf("verdict = %q, want conflict", rev.Verdict)
	}
	if rev.EscalateTo != "zeus" {
		t.Errorf("escalateTo = %q, want zeus", rev.EscalateTo)
	}
	if len(rev.Judges) != 2 || rev.Gate != "HG5" {
		t.Errorf("judges/gate mapped wrong: %+v", rev)
	}
}

func TestMapJudgmentAgree(t *testing.T) {
	rev := MapJudgment("TAL-15", ChJudgment{Judges: []string{"jd-judge-a"}, Verdict: "approved"})
	if rev.Verdict != "agree" {
		t.Errorf("verdict = %q, want agree", rev.Verdict)
	}
	if rev.EscalateTo != "" {
		t.Errorf("escalateTo = %q, want empty", rev.EscalateTo)
	}
}
