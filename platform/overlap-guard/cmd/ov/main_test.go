package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitremote"
	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
	"github.com/John-Santa/talos/platform/overlap-guard/mock"
	"github.com/John-Santa/talos/platform/overlap-guard/service"
)

// fakeScanner/fakeChecker/fakeMetricer inject a canned report+error into the run* seams so the
// command-logic wiring (source → emit → verdict error) is regression-locked without real git/wt.
type fakeScanner struct {
	report overlap.Report
	err    error
}

func (f fakeScanner) ScanInFlight(context.Context) (overlap.Report, error) {
	return f.report, f.err
}

type fakeChecker struct {
	report overlap.Report
	err    error
}

func (f fakeChecker) CheckPreAssignment(context.Context, string, string, []string) (overlap.Report, error) {
	return f.report, f.err
}

type fakeMetricer struct {
	report overlap.Report
	err    error
}

func (f fakeMetricer) Metric(context.Context) (overlap.Report, error) {
	return f.report, f.err
}

// blockReport builds a report whose verdict is BLOCK (two distinct agents, same file).
func blockReport(t *testing.T) overlap.Report {
	t.Helper()
	a := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"shared.go"}, overlap.SourceActual)
	b := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"shared.go"}, overlap.SourceActual)
	r := overlap.NewReport([]overlap.Claim{a, b}, 0.15)
	if r.Verdict != overlap.VerdictBlock {
		t.Fatalf("precondition: want BLOCK, got %v", r.Verdict)
	}
	return r
}

// okReport builds a report whose verdict is OK (single claim, no pairs).
func okReport(t *testing.T) overlap.Report {
	t.Helper()
	a := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	r := overlap.NewReport([]overlap.Claim{a}, 0.15)
	if r.Verdict != overlap.VerdictOK {
		t.Fatalf("precondition: want OK, got %v", r.Verdict)
	}
	return r
}

// TestRun_UnknownSubcommand verifies that an unknown subcommand returns an error.
func TestRun_UnknownSubcommand(t *testing.T) {
	err := run([]string{"unknown-cmd"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

// TestRun_NoSubcommand verifies that invoking with no args returns an error.
func TestRun_NoSubcommand(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand, got nil")
	}
}

// TestExitCodeFor_NoClaims verifies that ErrNoClaims maps to exit code 0.
func TestExitCodeFor_NoClaims(t *testing.T) {
	err := &overlap.ErrNoClaims{}
	code := exitCodeFor(err)
	if code != 0 {
		t.Errorf("ErrNoClaims should exit 0, got %d", code)
	}
}

// TestExitCodeFor_SameFileParallel verifies that ErrSameFileParallel maps to exit code 1.
func TestExitCodeFor_SameFileParallel(t *testing.T) {
	err := &overlap.ErrSameFileParallel{File: "foo.go"}
	code := exitCodeFor(err)
	if code != 1 {
		t.Errorf("ErrSameFileParallel should exit 1, got %d", code)
	}
}

// TestExitCodeFor_GenericError verifies that arbitrary errors map to exit code 1.
func TestExitCodeFor_GenericError(t *testing.T) {
	err := errors.New("some error")
	code := exitCodeFor(err)
	if code != 1 {
		t.Errorf("generic error should exit 1, got %d", code)
	}
}

// TestExitCodeFor_Nil verifies that nil maps to exit code 0.
func TestExitCodeFor_Nil(t *testing.T) {
	code := exitCodeFor(nil)
	if code != 0 {
		t.Errorf("nil error should exit 0, got %d", code)
	}
}

// TestRun_CheckMissingFlags verifies that check without required flags returns an error.
func TestRun_CheckMissingFlags(t *testing.T) {
	// check requires --module and --agent at minimum
	err := run([]string{"check"})
	if err == nil {
		t.Fatal("expected error for check with no flags, got nil")
	}
}

// TestRun_ScanNoError verifies that scan subcommand parses its flags without panicking.
// This is a flag-parse-only test (no real git/wt available in unit context).
func TestRun_ScanNoError(t *testing.T) {
	// Just verify the scan subcommand recognises its flags — no real I/O.
	// Passing --no-fetch --base develop would try to call wt which may not exist.
	// The error is expected (binary not found or git error), but no panic.
	err := run([]string{"scan", "--no-fetch", "--base", "develop"})
	// err is expected here (no real wt/git); we just check no panic occurred.
	_ = err
}

// W-03 RED: file_collisions[] must use agents[] not agent_a/agent_b (REQ-OUTPUT-1).
func TestReportToCheckJSON_FileCollisions_AgentsArray(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"shared.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"shared.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)

	out := reportToCheckJSON(report)

	if len(out.FileCollisions) == 0 {
		t.Fatal("expected at least one file collision")
	}
	fc := out.FileCollisions[0]
	if fc.File != "shared.go" {
		t.Errorf("file_collisions[0].file = %q, want %q", fc.File, "shared.go")
	}
	if len(fc.Agents) != 2 {
		t.Fatalf("file_collisions[0].agents length = %d, want 2", len(fc.Agents))
	}
	// agents[] must be sorted (atlas < hermes)
	if fc.Agents[0] != "atlas" || fc.Agents[1] != "hermes" {
		t.Errorf("file_collisions[0].agents = %v, want [atlas hermes]", fc.Agents)
	}
}

// W-03 RED: module_overlaps[] must use agents[] not agent_a/agent_b (REQ-OUTPUT-1).
func TestReportToCheckJSON_ModuleOverlaps_AgentsArray(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"b.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)

	out := reportToCheckJSON(report)

	if len(out.ModuleOverlaps) == 0 {
		t.Fatal("expected at least one module overlap")
	}
	mo := out.ModuleOverlaps[0]
	if len(mo.Agents) != 2 {
		t.Fatalf("module_overlaps[0].agents length = %d, want 2", len(mo.Agents))
	}
	if mo.Agents[0] != "atlas" || mo.Agents[1] != "hermes" {
		t.Errorf("module_overlaps[0].agents = %v, want [atlas hermes]", mo.Agents)
	}
}

// W-02 RED: check --json must NOT emit collision_rate, threshold, over_threshold, pairs_evaluated, colliding_pairs.
func TestReportToCheckJSON_OmitsScanOnlyFields(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas}, 0.15)

	out := reportToCheckJSON(report)
	// checkOnlyJSON must not have scan-only fields — verified via encoding
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(out); err != nil {
		t.Fatalf("encode error: %v", err)
	}
	encoded := buf.String()
	for _, forbidden := range []string{"collision_rate", "threshold", "over_threshold", "pairs_evaluated", "colliding_pairs"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("check JSON must not contain field %q but got: %s", forbidden, encoded)
		}
	}
}

// W-02 RED: metric --json must emit ONLY metric fields (no verdict, file_collisions, module_overlaps, advisories).
func TestReportToMetricJSON_OnlyMetricFields(t *testing.T) {
	t.Parallel()

	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"a.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)

	out := reportToMetricJSON(report, 0.15)
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(out); err != nil {
		t.Fatalf("encode error: %v", err)
	}
	encoded := buf.String()
	for _, forbidden := range []string{"verdict", "file_collisions", "module_overlaps", "advisories"} {
		if strings.Contains(encoded, forbidden) {
			t.Errorf("metric JSON must not contain field %q but got: %s", forbidden, encoded)
		}
	}
	for _, required := range []string{"collision_rate", "threshold", "over_threshold", "pairs_evaluated", "colliding_pairs"} {
		if !strings.Contains(encoded, required) {
			t.Errorf("metric JSON must contain field %q but got: %s", required, encoded)
		}
	}
}

// W-02 RED: cmdMetric unit test — composition root wiring.
func TestCmdMetric_RequiresNoArgsForHelp(t *testing.T) {
	t.Parallel()
	// cmdMetric with --help returns a flag-parse error, not a panic; validates composition root is wired.
	err := run([]string{"metric", "--threshold", "0.15", "--no-fetch"})
	// Error is expected (no real wt/git available in unit tests); no panic is the guarantee.
	_ = err
}

// TAL-16: scan --remote flag must be recognised — if the flag is not declared,
// flag.Parse returns an error containing "flag provided but not defined".
// This test FAILS (RED) until --remote is wired into cmdScan.
func TestRun_ScanRemote_FlagMustBeDeclared(t *testing.T) {
	// --no-fetch is set to avoid real network; --remote must not return a flag-parse error.
	err := run([]string{"scan", "--remote", "--no-fetch", "--base", "develop"})
	// If --remote is undeclared, flag.Parse returns an error containing "flag provided but not defined".
	if err != nil && strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("--remote flag not declared in cmdScan: %v", err)
	}
	// Any other error (e.g. git not available) is acceptable — only flag-parse failure is prohibited.
}

// TAL-16: scan --remote --json flag combination is recognised without flag-parse error.
func TestRun_ScanRemote_JSON_FlagRecognised(t *testing.T) {
	err := run([]string{"scan", "--remote", "--no-fetch", "--json"})
	if err != nil && strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("--remote or --json flag not declared in cmdScan: %v", err)
	}
}

// TAL-16: scan --remote does not accept unknown flags.
func TestRun_ScanRemote_UnknownFlag_ReturnsError(t *testing.T) {
	err := run([]string{"scan", "--remote", "--unknown-flag-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}

// W-01 RED: advisory emitted when issue has no checklist — visible in JSON output.
func TestReportToCheckJSON_AdvisoriesFromReport(t *testing.T) {
	t.Parallel()

	// A report with an advisory (populated by service, tested at service layer).
	// Here we verify that reportToCheckJSON passes report.Advisories through.
	report := overlap.NewReport([]overlap.Claim{}, 0.15)
	report.Advisories = []string{"TAL-99 sin checklist files: — solape a nivel-archivo no verificable"}

	out := reportToCheckJSON(report)
	if len(out.Advisories) != 1 {
		t.Fatalf("advisories length = %d, want 1", len(out.Advisories))
	}
	if out.Advisories[0] != report.Advisories[0] {
		t.Errorf("advisories[0] = %q, want %q", out.Advisories[0], report.Advisories[0])
	}
}

// --- TAL-16 fix B (--branches): an EXPLICIT --branches must never fall back to ls-remote. ---
// CI runs `gh pr list --state open` and passes the result to --branches. When there are zero
// open PRs the value is empty — but it was still provided, and must mean "scan nothing", NOT
// "fall back to ls-remote" (which would resurface stale squash-merged branches: the original bug).

// TestScanListerMode_ExplicitBranchesEvenWhenEmpty — branchesSet wins even with an empty value.
func TestScanListerMode_ExplicitBranchesEvenWhenEmpty(t *testing.T) {
	t.Parallel()
	if got := scanListerMode(true, true); got != modeRemoteExplicit {
		t.Errorf("scanListerMode(remote=true, branchesSet=true) = %v, want modeRemoteExplicit", got)
	}
}

// TestScanListerMode_RemoteWithoutBranches_UsesLsRemote — --remote alone keeps ls-remote discovery.
func TestScanListerMode_RemoteWithoutBranches_UsesLsRemote(t *testing.T) {
	t.Parallel()
	if got := scanListerMode(true, false); got != modeRemoteLsRemote {
		t.Errorf("scanListerMode(remote=true, branchesSet=false) = %v, want modeRemoteLsRemote", got)
	}
}

// TestScanListerMode_NoRemote_UsesWorktree — without --remote the wt lister is used; --branches is ignored.
func TestScanListerMode_NoRemote_UsesWorktree(t *testing.T) {
	t.Parallel()
	if got := scanListerMode(false, false); got != modeWorktree {
		t.Errorf("scanListerMode(remote=false, branchesSet=false) = %v, want modeWorktree", got)
	}
	if got := scanListerMode(false, true); got != modeWorktree {
		t.Errorf("scanListerMode(remote=false, branchesSet=true) = %v, want modeWorktree (branches ignored without --remote)", got)
	}
}

// --- TAL-16 fix A (exit code): the --json branch must NOT swallow the verdict. ---
// errForBlock/errForStrict are the pure decision functions shared by the JSON and text paths
// so `ov scan/check/metric --json` exit codes match the text path (BLOCK → 1, over+strict → 1).

// TestErrForBlock_Block_MapsToExit1 — a BLOCK report yields an error that exitCodeFor maps to 1.
func TestErrForBlock_Block_MapsToExit1(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"shared.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"shared.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)
	if report.Verdict != overlap.VerdictBlock {
		t.Fatalf("precondition: want BLOCK verdict, got %v", report.Verdict)
	}
	err := errForBlock(report)
	if err == nil {
		t.Fatal("errForBlock(BLOCK) = nil, want non-nil error")
	}
	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exitCodeFor(errForBlock(BLOCK)) = %d, want 1", got)
	}
}

// TestErrForBlock_OK_ReturnsNil — an OK report yields no error (exit 0).
func TestErrForBlock_OK_ReturnsNil(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas}, 0.15)
	if report.Verdict != overlap.VerdictOK {
		t.Fatalf("precondition: want OK verdict, got %v", report.Verdict)
	}
	if err := errForBlock(report); err != nil {
		t.Errorf("errForBlock(OK) = %v, want nil", err)
	}
}

// TestErrForBlock_Serialize_ReturnsNil — SERIALIZE is not a hard block (exit 0).
func TestErrForBlock_Serialize_ReturnsNil(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"b.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)
	if report.Verdict != overlap.VerdictSerialize {
		t.Fatalf("precondition: want SERIALIZE verdict, got %v", report.Verdict)
	}
	if err := errForBlock(report); err != nil {
		t.Errorf("errForBlock(SERIALIZE) = %v, want nil", err)
	}
}

// TestErrForStrict_OverThresholdStrict_ReturnsError — metric over threshold with --strict exits 1.
func TestErrForStrict_OverThresholdStrict_ReturnsError(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"a.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)
	if !report.OverThreshold {
		t.Fatalf("precondition: want OverThreshold true, got false (rate=%v)", report.CollisionRate)
	}
	if err := errForStrict(report, true, 0.15); err == nil {
		t.Error("errForStrict(over, strict=true) = nil, want error")
	}
}

// TestErrForStrict_OverThresholdNotStrict_ReturnsNil — without --strict, metric stays informational.
func TestErrForStrict_OverThresholdNotStrict_ReturnsNil(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:core", "branch-b", []string{"a.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)
	if err := errForStrict(report, false, 0.15); err != nil {
		t.Errorf("errForStrict(over, strict=false) = %v, want nil", err)
	}
}

// TestErrForStrict_UnderThresholdStrict_ReturnsNil — under threshold never errors, even with --strict.
func TestErrForStrict_UnderThresholdStrict_ReturnsNil(t *testing.T) {
	t.Parallel()
	atlas := overlap.NewClaim("atlas", "mod:core", "branch-a", []string{"a.go"}, overlap.SourceActual)
	hermes := overlap.NewClaim("hermes", "mod:other", "branch-b", []string{"b.go"}, overlap.SourceActual)
	report := overlap.NewReport([]overlap.Claim{atlas, hermes}, 0.15)
	if report.OverThreshold {
		t.Fatalf("precondition: want OverThreshold false, got true (rate=%v)", report.CollisionRate)
	}
	if err := errForStrict(report, true, 0.15); err != nil {
		t.Errorf("errForStrict(under, strict=true) = %v, want nil", err)
	}
}

// --- TAL-16 fix A (exit code) WIRING: lock the call sites, not just the helpers. ---
// ARGOS Round 1: the original bug lived in the --json branch of the command handlers, not in a
// helper. These tests exercise emit{Check,Scan,Metric}Result directly, so a future revert of a
// --json branch to "just writeJSON()" (dropping the verdict error) goes RED. stdout must also
// stay valid JSON (a CI consumer parses it) while the error still propagates to exit 1.

// TestEmitScanResult_JSONBlock_ErrorsButEmitsValidJSON — the headline regression lock for scan.
func TestEmitScanResult_JSONBlock_ErrorsButEmitsValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := emitScanResult(&buf, blockReport(t), true)
	if err == nil {
		t.Fatal("emitScanResult(json, BLOCK) = nil, want non-nil error (exit 1)")
	}
	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exitCodeFor = %d, want 1", got)
	}
	var got scanOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", e, buf.String())
	}
	if got.Verdict != "BLOCK" {
		t.Errorf("json verdict = %q, want BLOCK", got.Verdict)
	}
}

// TestEmitScanResult_JSONOK_NilAndValidJSON — OK verdict under --json: no error, valid JSON.
func TestEmitScanResult_JSONOK_NilAndValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := emitScanResult(&buf, okReport(t), true); err != nil {
		t.Errorf("emitScanResult(json, OK) = %v, want nil", err)
	}
	var got scanOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v", e)
	}
	if got.Verdict != "OK" {
		t.Errorf("json verdict = %q, want OK", got.Verdict)
	}
}

// TestEmitScanResult_TextBlock_Errors — the text path also returns the verdict error.
func TestEmitScanResult_TextBlock_Errors(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := emitScanResult(&buf, blockReport(t), false); err == nil {
		t.Fatal("emitScanResult(text, BLOCK) = nil, want error")
	}
	if !strings.Contains(buf.String(), "BLOCK") {
		t.Errorf("text output = %q, want it to mention BLOCK", buf.String())
	}
}

// TestEmitCheckResult_JSONBlock_ErrorsButEmitsValidJSON — same wiring lock for check.
func TestEmitCheckResult_JSONBlock_ErrorsButEmitsValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := emitCheckResult(&buf, blockReport(t), true); err == nil {
		t.Fatal("emitCheckResult(json, BLOCK) = nil, want non-nil error (exit 1)")
	}
	var got checkOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v", e)
	}
	if got.Verdict != "BLOCK" {
		t.Errorf("json verdict = %q, want BLOCK", got.Verdict)
	}
}

// TestEmitMetricResult_JSONOverStrict_Errors — metric --json --strict over threshold exits 1.
func TestEmitMetricResult_JSONOverStrict_Errors(t *testing.T) {
	t.Parallel()
	report := blockReport(t) // collision rate 1.0 > 0.15 → OverThreshold
	if !report.OverThreshold {
		t.Fatalf("precondition: want OverThreshold true, got false")
	}
	var buf bytes.Buffer
	if err := emitMetricResult(&buf, report, true, true, 0.15); err == nil {
		t.Fatal("emitMetricResult(json, over, strict) = nil, want error")
	}
	var got metricOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v", e)
	}
	if !got.OverThreshold {
		t.Errorf("json over_threshold = false, want true")
	}
}

// TestEmitMetricResult_JSONOverNotStrict_NilButValidJSON — without --strict, metric stays informational.
func TestEmitMetricResult_JSONOverNotStrict_NilButValidJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := emitMetricResult(&buf, blockReport(t), true, false, 0.15); err != nil {
		t.Errorf("emitMetricResult(json, over, not-strict) = %v, want nil", err)
	}
	var got metricOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout is not valid JSON: %v", e)
	}
}

// TestRun_ScanBranches_RequiresRemote — positively asserts --branches is declared AND wired: without
// --remote it must fail loud ("requires --remote"), never a silent worktree scan / false all-clear.
// (Reaches the validation before any git I/O, so it is deterministic.) Replaces the weaker
// flag-declared-only guard ARGOS flagged as one-sided.
func TestRun_ScanBranches_RequiresRemote(t *testing.T) {
	err := run([]string{"scan", "--branches", "agent/x/TAL-1"})
	if err == nil || !strings.Contains(err.Error(), "requires --remote") {
		t.Fatalf("scan --branches without --remote must error with 'requires --remote', got: %v", err)
	}
}

// --- ARGOS Round 2: lock the command-logic wiring (source → emit → verdict error) end-to-end. ---
// The run* seams are what cmdCheck/cmdScan/cmdMetric delegate to; testing them with a fake source
// catches a revert that drops the verdict error from the --json path AT THE CALL SITE (the exact
// place the original gate-defeating bug lived) — which the helper-only tests could not catch.

// TestRunScan_JSONBlock_PropagatesErrorAndEmitsJSON — BLOCK report through scan seam → exit 1 + valid JSON.
func TestRunScan_JSONBlock_PropagatesErrorAndEmitsJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := runScan(context.Background(), &buf, fakeScanner{report: blockReport(t)}, true)
	if err == nil {
		t.Fatal("runScan(json, BLOCK) = nil, want non-nil error")
	}
	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exitCodeFor = %d, want 1", got)
	}
	var got scanOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("stdout not valid JSON: %v", e)
	}
	if got.Verdict != "BLOCK" {
		t.Errorf("json verdict = %q, want BLOCK", got.Verdict)
	}
}

// TestRunScan_NoClaims_Exit0 — ErrNoClaims from the source still maps to exit 0 (no in-flight PRs).
func TestRunScan_NoClaims_Exit0(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := runScan(context.Background(), &buf, fakeScanner{err: &overlap.ErrNoClaims{}}, true)
	if err == nil {
		t.Fatal("runScan should propagate ErrNoClaims")
	}
	if got := exitCodeFor(err); got != 0 {
		t.Errorf("exitCodeFor(ErrNoClaims) = %d, want 0", got)
	}
}

// TestRunCheck_JSONBlock_PropagatesError — BLOCK report through check seam → non-nil error.
func TestRunCheck_JSONBlock_PropagatesError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := runCheck(context.Background(), &buf, fakeChecker{report: blockReport(t)}, "mod:core", "atlas", nil, true)
	if err == nil {
		t.Fatal("runCheck(json, BLOCK) = nil, want non-nil error")
	}
	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exitCodeFor = %d, want 1", got)
	}
}

// TestRunMetric_JSONOverStrict_PropagatesError — over threshold + --strict through metric seam → error.
func TestRunMetric_JSONOverStrict_PropagatesError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := runMetric(context.Background(), &buf, fakeMetricer{report: blockReport(t)}, true, true, 0.15)
	if err == nil {
		t.Fatal("runMetric(json, over, strict) = nil, want non-nil error")
	}
}

// TestRunMetric_JSONOverNotStrict_Nil — without --strict, metric stays informational (exit 0).
func TestRunMetric_JSONOverNotStrict_Nil(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := runMetric(context.Background(), &buf, fakeMetricer{report: blockReport(t)}, true, false, 0.15); err != nil {
		t.Errorf("runMetric(json, over, not-strict) = %v, want nil", err)
	}
}

// TestRunScan_SourceError_Propagates — an underlying source error surfaces unchanged (exit 1).
func TestRunScan_SourceError_Propagates(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	sentinel := errors.New("boom")
	err := runScan(context.Background(), &buf, fakeScanner{err: sentinel}, true)
	if !errors.Is(err, sentinel) {
		t.Fatalf("runScan should propagate source error, got %v", err)
	}
}

// TestRunScan_RealGuardExplicitBranches_BlockExit1 is the full-gate integration lock: the explicit
// open-PR set flows NewListerFromBranches → Guard.ScanInFlight → emit → exit code, with only the git
// inspector mocked. Two open PRs touching shared.go → BLOCK → exit 1 + valid JSON. (figuraFromBranch's
// origin/-stripping is locked separately by TestGuard_ScanInFlight_RemoteBranches_DeriveAgentFromBranch,
// which uses an empty Figura so the agent identity must be derived from the branch.)
func TestRunScan_RealGuardExplicitBranches_BlockExit1(t *testing.T) {
	t.Parallel()

	lister := gitremote.NewListerFromBranches([]string{"agent/themis/TAL-16", "agent/atlas/TAL-5"})
	inspector := mock.NewGitInspectorMock()
	inspector.RevParseResults = map[string]string{"develop": "abc123"}
	inspector.ChangedFilesByBranch = map[string][]string{
		"origin/agent/themis/TAL-16": {"shared.go"},
		"origin/agent/atlas/TAL-5":   {"shared.go"},
	}
	cfg := service.DefaultTALConfig()
	cfg.NoFetch = true
	guard := service.NewGuard(nil, lister, inspector, cfg)

	var buf bytes.Buffer
	err := runScan(context.Background(), &buf, guard, true)
	if err == nil {
		t.Fatal("real-guard BLOCK through runScan = nil, want exit-1 error")
	}
	if got := exitCodeFor(err); got != 1 {
		t.Errorf("exitCodeFor = %d, want 1", got)
	}
	var out scanOnlyJSON
	if e := json.Unmarshal(buf.Bytes(), &out); e != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", e, buf.String())
	}
	if out.Verdict != "BLOCK" {
		t.Errorf("verdict = %q, want BLOCK", out.Verdict)
	}
}
