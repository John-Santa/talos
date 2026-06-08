package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
