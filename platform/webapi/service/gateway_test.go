package service

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/webapi/domain"
)

// fakeReader returns canned platform state (no git, no files).
type fakeReader struct{}

func (fakeReader) Worktrees(context.Context) ([]domain.WtEntry, error) {
	return []domain.WtEntry{
		{Figura: "hermes", Branch: "agent/hermes/TAL-15", Head: "2e61d7e", Status: "active"},
		{Figura: "iris", Branch: "agent/iris/TAL-22", Head: "ec3ff1a", Status: "active"},
	}, nil
}

func (fakeReader) MergePlan(context.Context) (domain.MoPlan, error) {
	return domain.MoPlan{
		BaseBranch: "develop",
		Threshold:  0.15,
		Steps: []domain.MoPlanStep{
			{Position: 1, Branch: "agent/hermes/TAL-15", Figura: "hermes", CommitsAhead: 4, PredictedClean: true},
		},
	}, nil
}

func (fakeReader) Overlap(context.Context) (domain.OvScan, error) {
	return domain.OvScan{Verdict: "ok", PairsEvaluated: 1, CollidingPairs: 0}, nil
}

func (fakeReader) Ownership(context.Context) (map[string]string, error) {
	return map[string]string{"module:devops": "hermes", "module:frontend": "iris"}, nil
}

func (fakeReader) Ready(context.Context) error { return nil }

func (fakeReader) Labels(_ context.Context, _ string) (domain.ChLabels, error) {
	return domain.ChLabels{}, nil
}

func (fakeReader) Activity(_ context.Context, _ string) ([]domain.ActivityEntry, error) {
	return []domain.ActivityEntry{}, nil
}

func (fakeReader) RunsJudgment(_ context.Context, jiraKey string) (domain.JudgmentReview, error) {
	return domain.JudgmentReview{JiraKey: jiraKey, Pending: true}, nil
}

func (fakeReader) RunsDoD(_ context.Context, _ string) ([]domain.DoDItem, error) {
	return []domain.DoDItem{}, nil
}

func (fakeReader) CreateWorktree(context.Context, string, string) error { return nil }
func (fakeReader) TeardownWorktree(context.Context, string) error       { return nil }
func (fakeReader) Merge(_ context.Context, figura, jiraKey string) error {
	_ = figura
	_ = jiraKey
	return nil
}

func newGateway() *Gateway { return NewGateway(fakeReader{}, fakeReader{}) }

func TestGatewayOrchestration(t *testing.T) {
	snap, err := newGateway().Orchestration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Worktrees) != 2 {
		t.Errorf("worktrees = %d, want 2", len(snap.Worktrees))
	}
	if snap.MergeOrder.Threshold != 15 {
		t.Errorf("threshold = %v, want 15", snap.MergeOrder.Threshold)
	}
	if snap.Worktrees[0].Module != "devops" {
		t.Errorf("hermes module = %q, want devops", snap.Worktrees[0].Module)
	}
}

func TestGatewayAgentHasWorktree(t *testing.T) {
	detail, err := newGateway().Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Agent.Name != "Hermes" {
		t.Errorf("agent name = %q, want Hermes", detail.Agent.Name)
	}
	if detail.Worktree == nil || detail.Worktree.Branch != "agent/hermes/TAL-15" {
		t.Errorf("worktree mapped wrong: %+v", detail.Worktree)
	}
}

func TestGatewayAgentUnknown(t *testing.T) {
	if _, err := newGateway().Agent(context.Background(), "nobody"); err == nil {
		t.Error("want error for unknown figura")
	}
}

func TestGatewayAgents(t *testing.T) {
	if got := len(newGateway().Agents(context.Background())); got != 10 {
		t.Errorf("agents = %d, want 10", got)
	}
}

func TestGatewayReady(t *testing.T) {
	if err := newGateway().Ready(context.Background()); err != nil {
		t.Errorf("Ready = %v, want nil", err)
	}
}

func TestGatewayWriteDelegation(t *testing.T) {
	g := newGateway()
	ctx := context.Background()
	if err := g.CreateWorktree(ctx, "atlas", "TAL-99"); err != nil {
		t.Errorf("CreateWorktree = %v", err)
	}
	if err := g.TeardownWorktree(ctx, "atlas"); err != nil {
		t.Errorf("TeardownWorktree = %v", err)
	}
	if err := g.MergeWorktree(ctx, "iris", "TAL-15"); err != nil {
		t.Errorf("MergeWorktree = %v", err)
	}
}

func TestGatewayJudgmentMinimal(t *testing.T) {
	rev, err := newGateway().Judgment(context.Background(), "TAL-15")
	if err != nil {
		t.Fatal(err)
	}
	if rev.JiraKey != "TAL-15" {
		t.Errorf("judgment.JiraKey = %q, want TAL-15", rev.JiraKey)
	}
	if !rev.Pending {
		t.Errorf("judgment.Pending = false, want true (no ch source available)")
	}
	if rev.Verdict != "pending" {
		t.Errorf("judgment.Verdict = %q, want \"pending\" (must not fabricate \"agree\")", rev.Verdict)
	}
}

// --- PR3 tests ---------------------------------------------------------------

// fakeReaderWithLabels overrides Labels() to return a canned ChLabels response.
type fakeReaderWithLabels struct {
	fakeReader
	labels domain.ChLabels
}

func (f fakeReaderWithLabels) Labels(_ context.Context, _ string) (domain.ChLabels, error) {
	return f.labels, nil
}

func TestAgentDoDFromLabels(t *testing.T) {
	cl := domain.ChLabels{
		Labels:     []string{"ci:green", "pr:merged"},
		Violations: []string{"verify:missing"},
	}
	r := fakeReaderWithLabels{labels: cl}
	g := NewGateway(r, fakeReader{})
	detail, err := g.Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	// Expect 3 DoD items: 2 done + 1 pending
	if len(detail.DoD) != 3 {
		t.Errorf("DoD len = %d, want 3; items: %+v", len(detail.DoD), detail.DoD)
	}
	doneCount := 0
	for _, d := range detail.DoD {
		if d.State == "done" {
			doneCount++
		}
	}
	if doneCount != 2 {
		t.Errorf("done items = %d, want 2", doneCount)
	}
}

func TestAgentDoDEmptyWhenNoLabels(t *testing.T) {
	// fakeReader returns empty ChLabels — DoD should be empty slice, not nil.
	detail, err := newGateway().Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if detail.DoD == nil {
		t.Error("DoD must be an empty slice, not nil")
	}
	if len(detail.DoD) != 0 {
		t.Errorf("DoD len = %d, want 0 (no labels from ch)", len(detail.DoD))
	}
}

func TestJudgmentReturnsPendingWhenNoChSource(t *testing.T) {
	rev, err := newGateway().Judgment(context.Background(), "TAL-15")
	if err != nil {
		t.Fatal(err)
	}
	// With no ch source, verdict must NOT be a fabricated "agree" — must be pending=true.
	if !rev.Pending {
		t.Errorf("Judgment without ch source: Pending = false, want true")
	}
	if len(rev.Judges) != 0 {
		t.Errorf("Judgment without ch source: Judges = %v, want empty", rev.Judges)
	}
}

// --- PR2 tests ---------------------------------------------------------------

func TestAgentNormalizesCase(t *testing.T) {
	tests := []struct {
		name    string
		figura  string
		wantErr bool
	}{
		{name: "mixed-case Hermes → 200", figura: "Hermes", wantErr: false},
		{name: "all-caps HERMES → 200", figura: "HERMES", wantErr: false},
		{name: "lower hermes → 200", figura: "hermes", wantErr: false},
		{name: "unknown unicorn → error", figura: "unicorn", wantErr: true},
	}
	g := newGateway()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail, err := g.Agent(context.Background(), tt.figura)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for figura %q, got detail %+v", tt.figura, detail)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for figura %q: %v", tt.figura, err)
			}
			if !tt.wantErr && detail.Agent.ID != "hermes" {
				t.Errorf("agent.ID = %q, want hermes", detail.Agent.ID)
			}
		})
	}
}

// --- gateway-runs-wiring: Activity / Judgment wired through runs ---------------

// fakeReaderWithRuns extends fakeReader with Activity / RunsJudgment / RunsDoD.
type fakeReaderWithRuns struct {
	fakeReader
	activity    []domain.ActivityEntry
	actErr      error
	judgment    domain.JudgmentReview
	judgErr     error
	runsDoD     []domain.DoDItem
	runsDoDErr  error
}

func (f fakeReaderWithRuns) Activity(_ context.Context, _ string) ([]domain.ActivityEntry, error) {
	return f.activity, f.actErr
}

func (f fakeReaderWithRuns) RunsJudgment(_ context.Context, _ string) (domain.JudgmentReview, error) {
	return f.judgment, f.judgErr
}

func (f fakeReaderWithRuns) RunsDoD(_ context.Context, _ string) ([]domain.DoDItem, error) {
	return f.runsDoD, f.runsDoDErr
}

// TestAgentActivityFromRuns verifies that Agent() populates Activity from
// reader.Activity() instead of returning the empty placeholder.
func TestAgentActivityFromRuns(t *testing.T) {
	acts := []domain.ActivityEntry{
		{At: "2026-06-10T10:00:00Z", Text: "apply started"},
		{At: "2026-06-10T11:00:00Z", Text: "verify passed"},
	}
	r := fakeReaderWithRuns{activity: acts}
	g := NewGateway(r, fakeReader{})
	detail, err := g.Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Activity) != 2 {
		t.Fatalf("Activity len = %d, want 2; entries: %+v", len(detail.Activity), detail.Activity)
	}
	if detail.Activity[0].Text != "apply started" {
		t.Errorf("Activity[0].Text = %q, want %q", detail.Activity[0].Text, "apply started")
	}
}

// TestAgentActivityDegrades verifies that when runs is unavailable (Activity returns
// error / empty), Agent() still succeeds and returns an empty (non-nil) Activity.
func TestAgentActivityDegrades(t *testing.T) {
	r := fakeReaderWithRuns{activity: nil, actErr: nil}
	g := NewGateway(r, fakeReader{})
	detail, err := g.Agent(context.Background(), "hermes")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Activity == nil {
		t.Error("Activity must be non-nil even when runs returns nothing")
	}
}

// TestJudgmentFromRunsWhenDataPresent verifies that Judgment() uses the real
// JudgmentReview from runs when runs has data (Pending=false).
func TestJudgmentFromRunsWhenDataPresent(t *testing.T) {
	realReview := domain.JudgmentReview{
		JiraKey:  "TAL-42",
		Gate:     "HG5",
		Judges:   []domain.Judge{{ID: "cronos", Verdict: "APPROVED", Note: ""}},
		FixAgent: "idle",
		Verdict:  "agree",
		Pending:  false,
	}
	r := fakeReaderWithRuns{judgment: realReview}
	g := NewGateway(r, fakeReader{})
	rev, err := g.Judgment(context.Background(), "TAL-42")
	if err != nil {
		t.Fatal(err)
	}
	if rev.Pending {
		t.Errorf("Judgment.Pending = true, want false — runs has real data")
	}
	if rev.JiraKey != "TAL-42" {
		t.Errorf("Judgment.JiraKey = %q, want TAL-42", rev.JiraKey)
	}
	if len(rev.Judges) != 1 || rev.Judges[0].ID != "cronos" {
		t.Errorf("Judgment.Judges = %+v, want [{cronos APPROVED}]", rev.Judges)
	}
}

// TestJudgmentFallbackToPendingWhenRunsHasNoData verifies that Judgment() falls
// back to Pending:true when runs returns Pending (no events recorded).
func TestJudgmentFallbackToPendingWhenRunsHasNoData(t *testing.T) {
	pendingReview := domain.JudgmentReview{
		JiraKey: "TAL-42",
		Pending: true,
	}
	r := fakeReaderWithRuns{judgment: pendingReview}
	g := NewGateway(r, fakeReader{})
	rev, err := g.Judgment(context.Background(), "TAL-42")
	if err != nil {
		t.Fatal(err)
	}
	if !rev.Pending {
		t.Errorf("Judgment.Pending = false, want true when runs has no data")
	}
}

// TestJudgmentFallbackWhenRunsErrors verifies that when RunsJudgment() returns
// an error, Judgment() degrades to Pending:true — no 500, no panic.
func TestJudgmentFallbackWhenRunsErrors(t *testing.T) {
	r := fakeReaderWithRuns{
		judgment: domain.JudgmentReview{},
		judgErr:  errors.New("runs: store not found"),
	}
	g := NewGateway(r, fakeReader{})
	rev, err := g.Judgment(context.Background(), "TAL-42")
	if err != nil {
		t.Fatalf("Judgment must not return error when runs degrades, got: %v", err)
	}
	if !rev.Pending {
		t.Errorf("Judgment.Pending = false, want true on runs error")
	}
}

func TestCreateWorktreeRejectsUnknownFigura(t *testing.T) {
	tests := []struct {
		name     string
		figura   string
		wantErr  bool
		wantCode bool // true = expect ErrUnknownFigura
	}{
		{name: "valid figura iris → ok", figura: "iris", wantErr: false},
		{name: "invalid figura bogus → ErrUnknownFigura", figura: "bogus", wantErr: true, wantCode: true},
		{name: "mixed-case Iris → ok (normalized)", figura: "Iris", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGateway()
			err := g.CreateWorktree(context.Background(), tt.figura, "TAL-99")
			if tt.wantErr && err == nil {
				t.Errorf("expected error for figura %q", tt.figura)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for figura %q: %v", tt.figura, err)
			}
			if tt.wantCode && !errors.Is(err, domain.ErrUnknownFigura) {
				t.Errorf("expected ErrUnknownFigura for figura %q, got: %v", tt.figura, err)
			}
		})
	}
}
