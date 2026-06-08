package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

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
