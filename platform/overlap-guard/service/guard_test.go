package service_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
	"github.com/John-Santa/talos/platform/overlap-guard/mock"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
	"github.com/John-Santa/talos/platform/overlap-guard/service"
)

// --- ScanInFlight ---

func TestGuard_ScanInFlight_FiltersOnlyActive(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Path: "/wt/atlas", Status: "active"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-2", Path: "/wt/hermes", Status: "idle"},
		{Figura: "cronos", Branch: "agent/cronos/TAL-3", Path: "/wt/cronos", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1":  {"a.go"},
		"agent/cronos/TAL-3": {"c.go"},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, err := g.ScanInFlight(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only active worktrees → only 2 claims → 1 distinct-agent pair (atlas, cronos), no shared files → OK
	if report.Verdict != overlap.VerdictOK {
		t.Errorf("Verdict = %v, want VerdictOK", report.Verdict)
	}
	if report.ClaimCount != 2 {
		t.Errorf("ClaimCount = %d, want 2 (only active worktrees)", report.ClaimCount)
	}
	// hermes (idle) must not appear in claims
	lister.AssertCallCount(t, "List", 1)
}

func TestGuard_ScanInFlight_EmptyActive_ReturnsErrNoClaims(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "idle"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, err := g.ScanInFlight(context.Background())

	// Empty active set → ErrNoClaims with exit-0 semantics; report is zero-value
	if err == nil {
		t.Fatal("expected ErrNoClaims, got nil")
	}
	var noClaims *overlap.ErrNoClaims
	if !errors.As(err, &noClaims) {
		t.Errorf("expected *overlap.ErrNoClaims, got %T: %v", err, err)
	}
	_ = report
}

// TestGuard_ScanInFlight_RemoteBranches_DeriveAgentFromBranch is the gate-path lock for `--remote`:
// remote entries are "origin/"-prefixed and (worst case) carry no Figura. The agent identity MUST
// be derived from the branch — otherwise distinct agents collapse to "" and a real BLOCK silently
// downgrades to OK (the gate passes a genuine collision). RED before figuraFromBranch handles origin/.
func TestGuard_ScanInFlight_RemoteBranches_DeriveAgentFromBranch(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "", Branch: "origin/agent/themis/TAL-16", Status: "active"},
		{Figura: "", Branch: "origin/agent/atlas/TAL-5", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"origin/agent/themis/TAL-16": {"shared.go"},
		"origin/agent/atlas/TAL-5":   {"shared.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, err := g.ScanInFlight(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != overlap.VerdictBlock {
		t.Fatalf("Verdict = %v, want VerdictBlock (distinct agents themis/atlas on shared.go)", report.Verdict)
	}
	if len(report.FileCollisions) != 1 {
		t.Fatalf("FileCollisions = %d, want 1", len(report.FileCollisions))
	}
	agents := []string{report.FileCollisions[0].A.Agent, report.FileCollisions[0].B.Agent}
	sort.Strings(agents)
	if agents[0] != "atlas" || agents[1] != "themis" {
		t.Errorf("collision agents = %v, want [atlas themis] derived from branch (not empty Figura fallback)", agents)
	}
}

// TestGuard_ScanInFlight_NoActiveEntries_SkipsFetchAndRevParse pins the ordering: a zero-PR scan must
// short-circuit to ErrNoClaims (exit 0) WITHOUT shelling out to fetch/base-resolution, which can fail
// on a CI checkout where the base ref isn't local. RED before the early-return is added.
func TestGuard_ScanInFlight_NoActiveEntries_SkipsFetchAndRevParse(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock() // empty in-flight set
	inspector := mock.NewGitInspectorMock()
	inspector.FetchErr = errors.New("fetch must not be called on an empty set")
	inspector.RevParseErrs = map[string]error{"develop": errors.New("revparse must not be called on an empty set")}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(nil, lister, inspector, cfg)
	_, err := g.ScanInFlight(context.Background())

	var noClaims *overlap.ErrNoClaims
	if !errors.As(err, &noClaims) {
		t.Fatalf("want *overlap.ErrNoClaims, got %T: %v", err, err)
	}
	inspector.AssertNotCalled(t, "Fetch")
	inspector.AssertNotCalled(t, "RevParse")
}

func TestGuard_ScanInFlight_NoFetch_OmitsFetch(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1": {"a.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true

	g := service.NewGuard(nil, lister, inspector, cfg)
	_, _ = g.ScanInFlight(context.Background())

	// Fetch must NOT be called when NoFetch=true (REQ-FETCH-2, REQ-TEST-9)
	inspector.AssertNotCalled(t, "Fetch")
}

func TestGuard_ScanInFlight_WithFetch_CallsFetch(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1": {"a.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = false

	g := service.NewGuard(nil, lister, inspector, cfg)
	_, _ = g.ScanInFlight(context.Background())

	inspector.AssertCallCount(t, "Fetch", 1)
}

func TestGuard_ScanInFlight_PairwiseDistinctAgent_Block(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-2", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1":  {"shared.go", "a.go"},
		"agent/hermes/TAL-2": {"shared.go", "b.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, err := g.ScanInFlight(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != overlap.VerdictBlock {
		t.Errorf("Verdict = %v, want VerdictBlock", report.Verdict)
	}
	if len(report.FileCollisions) == 0 {
		t.Error("expected file collisions, got none")
	}
}

func TestGuard_ScanInFlight_SameAgent_NoCollision(t *testing.T) {
	t.Parallel()

	// Same agent (atlas) in two branches touching same file → no collision (REQ-SCAN-2)
	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
		{Figura: "atlas", Branch: "agent/atlas/TAL-5", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1": {"shared.go"},
		"agent/atlas/TAL-5": {"shared.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, err := g.ScanInFlight(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Verdict != overlap.VerdictOK {
		t.Errorf("Verdict = %v, want VerdictOK (same-agent, no collision)", report.Verdict)
	}
}

// --- CheckPreAssignment ---

func TestGuard_CheckPreAssignment_JQLVerbatim(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	cfg := service.DefaultTALConfig()
	// Return no issues → clean result
	searcher.DefaultResult = nil

	g := service.NewGuard(searcher, nil, nil, cfg)
	_, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// JQL must match verbatim REQ-CHECK-1
	wantJQL := `project = TAL AND statusCategory != Done AND labels = "module:core" AND labels NOT IN ("agent:atlas")`
	calls := searcher.CallsFor("Search")
	if len(calls) != 1 {
		t.Fatalf("Search call count = %d, want 1", len(calls))
	}
	gotJQL, ok := calls[0].Args[0].(string)
	if !ok || gotJQL != wantJQL {
		t.Errorf("Search JQL = %q, want %q", gotJQL, wantJQL)
	}
}

func TestGuard_CheckPreAssignment_ErrChecklistMissing_Advisory_ModuleKept(t *testing.T) {
	t.Parallel()

	// Issue with no checklist → ErrChecklistMissing advisory, excluded from file-level, kept for module-level.
	searcher := mock.NewIssueSearcherMock()
	// The issue has no files: section in body → ErrChecklistMissing advisory
	searcher.DefaultResult = []port.IssueResult{
		{
			Key:    "TAL-99",
			Labels: []string{"module:core", "agent:hermes"},
			Body:   "No checklist here.",
		},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, nil, cfg)
	// Owner is atlas declaring file "a.go" in module "core"
	report, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File collision must be empty (no checklist → no files to compare)
	if len(report.FileCollisions) != 0 {
		t.Errorf("FileCollisions = %d, want 0 (checklist missing → excluded from file-level)", len(report.FileCollisions))
	}
	// Module overlap must NOT be empty (hermes has same module:core → SERIALIZE)
	// Note: module overlap exists because hermes has module:core same as owner's module "core"
	if len(report.ModuleOverlaps) == 0 {
		t.Errorf("ModuleOverlaps = 0, want >0 (issue with module:core still contributes to module-level despite missing checklist)")
	}
}

func TestGuard_CheckPreAssignment_FileCollision_Block(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	searcher.DefaultResult = []port.IssueResult{
		{
			Key:    "TAL-77",
			Labels: []string{"module:core", "agent:hermes"},
			Body:   "files:\n- [ ] shared.go\n- [ ] b.go\n",
		},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, nil, cfg)
	report, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"shared.go", "a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Verdict != overlap.VerdictBlock {
		t.Errorf("Verdict = %v, want VerdictBlock", report.Verdict)
	}
	if len(report.FileCollisions) == 0 {
		t.Error("expected file collision on shared.go")
	}
}

func TestGuard_CheckPreAssignment_DisjointOverlap_Serialize(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	searcher.DefaultResult = []port.IssueResult{
		{
			Key:    "TAL-88",
			Labels: []string{"module:core", "agent:hermes"},
			Body:   "files:\n- [ ] b.go\n",
		},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, nil, cfg)
	report, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Verdict != overlap.VerdictSerialize {
		t.Errorf("Verdict = %v, want VerdictSerialize (disjoint files, same module)", report.Verdict)
	}
}

func TestGuard_CheckPreAssignment_NoOverlap_OK(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	// Different module entirely
	searcher.DefaultResult = []port.IssueResult{
		{
			Key:    "TAL-55",
			Labels: []string{"module:data", "agent:gaia"},
			Body:   "files:\n- [ ] d.go\n",
		},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, nil, cfg)
	report, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Verdict != overlap.VerdictOK {
		t.Errorf("Verdict = %v, want VerdictOK (different module, no overlap)", report.Verdict)
	}
}

// W-01 RED: issue with missing checklist → report.Advisories non-empty (REQ-CHECKLIST-4).
func TestGuard_CheckPreAssignment_MissingChecklist_PopulatesAdvisories(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	searcher.DefaultResult = []port.IssueResult{
		{
			Key:    "TAL-99",
			Labels: []string{"module:core", "agent:hermes"},
			Body:   "No checklist here.",
		},
	}
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, nil, cfg)
	report, err := g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Advisories) == 0 {
		t.Error("Advisories must be non-empty when at least one issue has no checklist (REQ-CHECKLIST-4)")
	}
}

func TestGuard_CheckPreAssignment_GitNotCalled(t *testing.T) {
	t.Parallel()

	searcher := mock.NewIssueSearcherMock()
	searcher.DefaultResult = nil
	inspector := mock.NewGitInspectorMock()
	cfg := service.DefaultTALConfig()

	g := service.NewGuard(searcher, nil, inspector, cfg)
	_, _ = g.CheckPreAssignment(context.Background(), "core", "atlas", []string{"a.go"})

	// REQ-CHECK-6: no git calls at all
	inspector.AssertNotCalled(t, "Fetch")
	inspector.AssertNotCalled(t, "RevParse")
	inspector.AssertNotCalled(t, "ChangedFiles")
}

// --- Metric ---

func TestGuard_Metric_DelegatesToScanInFlight(t *testing.T) {
	t.Parallel()

	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-2", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1":  {"a.go"},
		"agent/hermes/TAL-2": {"b.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true

	g := service.NewGuard(nil, lister, inspector, cfg)

	scanReport, _ := g.ScanInFlight(context.Background())

	// Reset calls for the lister to count again
	lister.Calls = nil
	inspector.Calls = nil

	metricReport, err := g.Metric(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Metric returns the same report shape as scan
	if metricReport.Verdict != scanReport.Verdict {
		t.Errorf("Metric.Verdict = %v, ScanInFlight.Verdict = %v — should match", metricReport.Verdict, scanReport.Verdict)
	}
	if metricReport.CollisionRate != scanReport.CollisionRate {
		t.Errorf("Metric.CollisionRate = %f, ScanInFlight.CollisionRate = %f", metricReport.CollisionRate, scanReport.CollisionRate)
	}
}

func TestGuard_Metric_ThresholdFromConfig(t *testing.T) {
	t.Parallel()

	// 1 collision pair out of 1 pair → rate=1.0
	lister := mock.NewWorktreeListerMock()
	lister.ListResult = []port.WorktreeEntry{
		{Figura: "atlas", Branch: "agent/atlas/TAL-1", Status: "active"},
		{Figura: "hermes", Branch: "agent/hermes/TAL-2", Status: "active"},
	}
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "sha"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"agent/atlas/TAL-1":  {"shared.go"},
		"agent/hermes/TAL-2": {"shared.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true
	cfg.Threshold = 0.15

	g := service.NewGuard(nil, lister, inspector, cfg)
	report, _ := g.Metric(context.Background())

	if !report.OverThreshold {
		t.Errorf("OverThreshold = false, want true (rate=1.0 > threshold=0.15)")
	}
}
