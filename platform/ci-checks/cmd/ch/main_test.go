package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/adapter/jirarest"
	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

// TestRun_NoArgs verifies that invoking with no args returns an error.
func TestRun_NoArgs(t *testing.T) {
	err := run([]string{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for no args, got nil")
	}
}

// TestRun_UnknownSubcommand verifies that an unknown subcommand returns an error.
func TestRun_UnknownSubcommand(t *testing.T) {
	err := run([]string{"bogus"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

// TestRun_Labels_NoBranch verifies that labels without --branch returns an error mentioning branch.
func TestRun_Labels_NoBranch(t *testing.T) {
	err := run([]string{"labels"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing --branch, got nil")
	}
	if !strings.Contains(err.Error(), "branch") {
		t.Errorf("error should mention 'branch', got: %v", err)
	}
}

// TestExitCodeFor_Table is a table-driven test for all error→exit-code mappings.
func TestExitCodeFor_Table(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"ErrNoJiraKey", cichecks.ErrNoJiraKey, 1},
		{"ErrLabelInvariant", &cichecks.ErrLabelInvariant{Violations: []string{"x"}}, 1},
		{"ErrMalformedOwnership", &cichecks.ErrMalformedOwnership{Detail: "d"}, 1},
		{"ErrIssueNotFound", cichecks.ErrIssueNotFound{Key: "TAL-9"}, 1},
		{"HTTPError", &jirarest.HTTPError{StatusCode: 401}, 1},
		{"generic", errors.New("unexpected"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exitCodeFor(tc.err)
			if got != tc.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// labelsJSONShape is the expected JSON shape for REQ-JSON-1.
type labelsJSONShape struct {
	Branch     string   `json:"branch"`
	JiraKey    string   `json:"jira_key"`
	Figura     string   `json:"figura"`
	Verdict    string   `json:"verdict"`
	Labels     []string `json:"labels"`
	Agent      string   `json:"agent"`
	Module     string   `json:"module"`
	Violations []string `json:"violations"`
}

func fixtureOwnershipPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "adapter", "ownershipfile", "testdata", "ownership.md")
}

// TestLabelsJSON_Shape_OK verifies all required JSON fields are present and correct on success.
func TestLabelsJSON_Shape_OK(t *testing.T) {
	// httptest server: returns OK response for TAL-7
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(map[string]any{
			"key": "TAL-7",
			"fields": map[string]any{
				"labels": []string{"agent:hermes", "module:devops"},
			},
		})
		w.Write(b)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	err := run([]string{
		"labels",
		"--branch", "agent/hermes/TAL-7",
		"--site-url", srv.URL,
		"--ownership-file", fixtureOwnershipPath(),
		"--json",
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out labelsJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, buf.String())
	}
	if out.Branch != "agent/hermes/TAL-7" {
		t.Errorf("branch = %q, want agent/hermes/TAL-7", out.Branch)
	}
	if out.JiraKey != "TAL-7" {
		t.Errorf("jira_key = %q, want TAL-7", out.JiraKey)
	}
	if out.Figura != "hermes" {
		t.Errorf("figura = %q, want hermes", out.Figura)
	}
	if out.Verdict != "OK" {
		t.Errorf("verdict = %q, want OK", out.Verdict)
	}
	if out.Agent != "hermes" {
		t.Errorf("agent = %q, want hermes", out.Agent)
	}
	if out.Module != "devops" {
		t.Errorf("module = %q, want devops", out.Module)
	}
	if out.Violations == nil {
		t.Error("violations must not be nil (use empty slice)")
	}
}

// TestLabelsJSON_Shape_Violation verifies verdict=VIOLATION and non-empty violations.
func TestLabelsJSON_Shape_Violation(t *testing.T) {
	// httptest server: returns labels that violate the invariant (wrong agent)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(map[string]any{
			"key": "TAL-7",
			"fields": map[string]any{
				"labels": []string{"agent:atlas", "module:devops"},
			},
		})
		w.Write(b)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	// run returns an error (ErrLabelInvariant) but still writes JSON when --json is set
	_ = run([]string{
		"labels",
		"--branch", "agent/hermes/TAL-7",
		"--site-url", srv.URL,
		"--ownership-file", fixtureOwnershipPath(),
		"--json",
	}, &buf)

	var out labelsJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, buf.String())
	}
	if out.Verdict != "VIOLATION" {
		t.Errorf("verdict = %q, want VIOLATION", out.Verdict)
	}
	if len(out.Violations) == 0 {
		t.Error("violations must be non-empty on VIOLATION verdict")
	}
}

// TestLabelsJSON_ErrNoJiraKey verifies that branch=develop with --json produces JSON with VIOLATION.
func TestLabelsJSON_ErrNoJiraKey(t *testing.T) {
	var buf bytes.Buffer
	_ = run([]string{
		"labels",
		"--branch", "develop",
		"--site-url", "http://unused",
		"--ownership-file", fixtureOwnershipPath(),
		"--json",
	}, &buf)

	var out labelsJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, buf.String())
	}
	if out.Verdict != "VIOLATION" {
		t.Errorf("verdict = %q, want VIOLATION", out.Verdict)
	}
	if len(out.Violations) == 0 {
		t.Error("violations must be non-empty for ErrNoJiraKey")
	}
}

// TestOwnership_Offline verifies the ownership subcommand exits 0 with a valid file.
func TestOwnership_Offline(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{
		"ownership",
		"--ownership-file", fixtureOwnershipPath(),
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBranchCaseNormalization verifies that GITHUB_HEAD_REF with uppercase figura is normalized.
func TestBranchCaseNormalization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(map[string]any{
			"key": "TAL-7",
			"fields": map[string]any{
				"labels": []string{"agent:hermes", "module:devops"},
			},
		})
		w.Write(b)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	// GITHUB_HEAD_REF can deliver "agent/HERMES/TAL-7" — must be normalized before domain call
	err := run([]string{
		"labels",
		"--branch", "agent/HERMES/TAL-7",
		"--site-url", srv.URL,
		"--ownership-file", fixtureOwnershipPath(),
		"--json",
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error after branch normalization: %v", err)
	}

	var out labelsJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if out.Figura != "hermes" {
		t.Errorf("figura = %q after normalization, want hermes", out.Figura)
	}
}

// noopContext is a helper for in-process test calls that don't need a real context.
func noopContext() context.Context {
	return context.Background()
}

var _ = noopContext

// TestChangedModules_StdinPaths verifies that changed-modules reads paths from a reader and prints module roots.
func TestChangedModules_StdinPaths(t *testing.T) {
	input := strings.NewReader("platform/ci-checks/domain/cichecks/label.go\nplatform/ci-checks/go.mod\n")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "platform/ci-checks" {
		t.Errorf("output = %q, want %q", out, "platform/ci-checks")
	}
}

// TestChangedModules_TwoModules verifies two modules are printed one per line, sorted.
func TestChangedModules_TwoModules(t *testing.T) {
	input := strings.NewReader("platform/foo/a.go\nplatform/bar/b.go\n")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{"platform/bar", "platform/foo"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %v, want %v", lines, want)
	}
}

// TestChangedModules_NonPlatformIgnored verifies that non-platform paths produce empty output.
func TestChangedModules_NonPlatformIgnored(t *testing.T) {
	input := strings.NewReader("openspec/changes/x.md\nREADME.md\n")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// TestChangedModules_EmptyInput verifies empty input produces empty output.
func TestChangedModules_EmptyInput(t *testing.T) {
	input := strings.NewReader("")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// TestChangedModules_JSONFlag verifies --json outputs a JSON array of modules.
func TestChangedModules_JSONFlag(t *testing.T) {
	input := strings.NewReader("platform/ci-checks/main.go\n")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out []string
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	want := []string{"platform/ci-checks"}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("json output = %v, want %v", out, want)
	}
}

// TestChangedModules_JSONFlag_Empty verifies --json with no platform paths outputs an empty JSON array.
func TestChangedModules_JSONFlag_Empty(t *testing.T) {
	input := strings.NewReader("")
	var buf bytes.Buffer
	err := cmdChangedModules(input, &buf, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out []string
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if len(out) != 0 {
		t.Errorf("expected empty JSON array, got %v", out)
	}
}

// TestRun_ChangedModules_DispatchWorks verifies the run() dispatcher routes changed-modules correctly.
func TestRun_ChangedModules_DispatchWorks(t *testing.T) {
	err := run([]string{"changed-modules"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error dispatching changed-modules: %v", err)
	}
}

// ── judgment subcommand tests ────────────────────────────────────────────────

// judgmentReportContent builds a well-formed judgment-report.md for a given
// change slug and verdict. Used by temp-dir fixtures in the tests below.
func judgmentReportContent(change, verdict string) string {
	return "**Change:** " + change + "\n" +
		"**Round:** 1\n" +
		"**Judges:** ARGOS-1, ARGOS-2\n" +
		"**Implementor:** HERMES\n" +
		"**Date:** 2026-06-09\n\n" +
		"JUDGMENT: " + verdict + "\n"
}

// writeJudgmentFixture writes judgment-report.md under changesDir/slug/ and
// returns the path to changesDir. It creates the directory if necessary.
func writeJudgmentFixture(t *testing.T, changesDir, slug, content string) {
	t.Helper()
	dir := filepath.Join(changesDir, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "judgment-report.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestJudgment_Table covers all exit-code scenarios for the judgment subcommand.
func TestJudgment_Table(t *testing.T) {
	cases := []struct {
		name     string
		slug     string
		setup    func(dir string) // writes fixtures into dir (the changes base dir)
		wantErr  bool
		wantCode int
	}{
		{
			name: "APPROVED exits 0",
			slug: "my-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "my-change", judgmentReportContent("my-change", "APPROVED"))
			},
			wantErr:  false,
			wantCode: 0,
		},
		{
			name: "APPROVED with emoji exits 0",
			slug: "my-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "my-change", judgmentReportContent("my-change", "APPROVED ✅"))
			},
			wantErr:  false,
			wantCode: 0,
		},
		{
			name: "ESCALATED exits 1",
			slug: "my-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "my-change", judgmentReportContent("my-change", "ESCALATED"))
			},
			wantErr:  true,
			wantCode: 1,
		},
		{
			name:    "missing report file exits 1",
			slug:    "my-change",
			setup:   func(dir string) { /* no file written */ },
			wantErr: true,
			wantCode: 1,
		},
		{
			name: "no terminal JUDGMENT line exits 1",
			slug: "my-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "my-change",
					"**Change:** my-change\n**Round:** 1\n**Judges:** ARGOS-1, ARGOS-2\n**Implementor:** HERMES\n**Date:** 2026-06-09\n\nNo verdict here.\n")
			},
			wantErr:  true,
			wantCode: 1,
		},
		{
			name: "Round 0 exits 1",
			slug: "my-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "my-change",
					"**Change:** my-change\n**Round:** 0\n**Judges:** ARGOS-1, ARGOS-2\n**Implementor:** HERMES\n**Date:** 2026-06-09\n\nJUDGMENT: APPROVED\n")
			},
			wantErr:  true,
			wantCode: 1,
		},
		{
			name: "change mismatch exits 1",
			slug: "other-change",
			setup: func(dir string) {
				writeJudgmentFixture(t, dir, "other-change", judgmentReportContent("my-change", "APPROVED"))
			},
			wantErr:  true,
			wantCode: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changesDir := t.TempDir()
			tc.setup(changesDir)

			var buf bytes.Buffer
			err := run([]string{
				"judgment",
				"--change", tc.slug,
				"--changes-dir", changesDir,
			}, &buf)

			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			code := exitCodeFor(err)
			if code != tc.wantCode {
				t.Errorf("exitCodeFor(err) = %d, want %d (err=%v)", code, tc.wantCode, err)
			}
		})
	}
}

// judgmentJSONShape mirrors the --json output shape for ch judgment (REQ-VALIDATOR-9).
type judgmentJSONShape struct {
	Change     string   `json:"change"`
	Round      int      `json:"round"`
	Judges     []string `json:"judges"`
	Implementor string  `json:"implementor"`
	Verdict    string   `json:"verdict"`
	Violations []string `json:"violations"`
}

// TestJudgment_JSON_APPROVED verifies --json output for an APPROVED report.
func TestJudgment_JSON_APPROVED(t *testing.T) {
	changesDir := t.TempDir()
	writeJudgmentFixture(t, changesDir, "my-change", judgmentReportContent("my-change", "APPROVED"))

	var buf bytes.Buffer
	err := run([]string{
		"judgment",
		"--change", "my-change",
		"--changes-dir", changesDir,
		"--json",
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out judgmentJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if out.Change != "my-change" {
		t.Errorf("change = %q, want my-change", out.Change)
	}
	if out.Verdict != "APPROVED" {
		t.Errorf("verdict = %q, want APPROVED", out.Verdict)
	}
	if out.Violations == nil {
		t.Error("violations must not be nil; want empty slice")
	}
}

// TestJudgment_JSON_MISSING verifies --json output when the report file is absent.
func TestJudgment_JSON_MISSING(t *testing.T) {
	changesDir := t.TempDir()
	// No file written.

	var buf bytes.Buffer
	_ = run([]string{
		"judgment",
		"--change", "my-change",
		"--changes-dir", changesDir,
		"--json",
	}, &buf)

	var out judgmentJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if out.Verdict != "MISSING" {
		t.Errorf("verdict = %q, want MISSING", out.Verdict)
	}
}

// TestJudgment_JSON_ESCALATED verifies --json output shape when verdict is ESCALATED.
func TestJudgment_JSON_ESCALATED(t *testing.T) {
	changesDir := t.TempDir()
	writeJudgmentFixture(t, changesDir, "my-change", judgmentReportContent("my-change", "ESCALATED"))

	var buf bytes.Buffer
	// run returns an error (ErrJudgmentNotApproved) but still writes JSON when --json is set
	_ = run([]string{
		"judgment",
		"--change", "my-change",
		"--changes-dir", changesDir,
		"--json",
	}, &buf)

	var out judgmentJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if out.Verdict != "ESCALATED" {
		t.Errorf("verdict = %q, want ESCALATED", out.Verdict)
	}
	if out.Change != "my-change" {
		t.Errorf("change = %q, want my-change", out.Change)
	}
	if out.Violations == nil {
		t.Error("violations must not be nil; want empty slice")
	}
}

// TestJudgment_JSON_MALFORMED verifies --json output shape when the report is malformed.
func TestJudgment_JSON_MALFORMED(t *testing.T) {
	changesDir := t.TempDir()
	// Write a report with no JUDGMENT: line (malformed).
	writeJudgmentFixture(t, changesDir, "my-change",
		"**Change:** my-change\n**Round:** 1\n**Judges:** ARGOS-1, ARGOS-2\n**Implementor:** HERMES\n**Date:** 2026-06-09\n\nNo verdict line.\n")

	var buf bytes.Buffer
	// run returns an error but still writes JSON when --json is set
	_ = run([]string{
		"judgment",
		"--change", "my-change",
		"--changes-dir", changesDir,
		"--json",
	}, &buf)

	var out judgmentJSONShape
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}
	if out.Verdict != "MALFORMED" {
		t.Errorf("verdict = %q, want MALFORMED", out.Verdict)
	}
}

// TestExitCodeFor_Judgment_Rows verifies new error types map to exit code 1.
func TestExitCodeFor_Judgment_Rows(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"ErrNoJudgmentReport", cichecks.ErrNoJudgmentReport, 1},
		{"ErrJudgmentNotApproved", &cichecks.ErrJudgmentNotApproved{Change: "x", Verdict: "ESCALATED"}, 1},
		{"ErrMalformedJudgment", &cichecks.ErrMalformedJudgment{Detail: "bad"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exitCodeFor(tc.err)
			if got != tc.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
