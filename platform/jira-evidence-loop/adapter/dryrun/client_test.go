package dryrun_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/adapter/dryrun"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// ---------------------------------------------------------------------------
// DryRunClient — interface compliance + no-op behaviour
// ---------------------------------------------------------------------------

func TestDryRunClient_CreateIssue_ReturnsPlaceholderKey(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)

	issue, err := c.CreateIssue(context.Background(), evidence.CreateIssueRequest{
		Summary: "Test issue",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if issue.Key == "" {
		t.Error("CreateIssue() returned empty Key; want a dry-run placeholder")
	}
}

func TestDryRunClient_Search_ReturnsEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)

	results, err := c.Search(context.Background(), `project = "TAL"`, 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search() len = %d, want 0 (dry-run always returns empty)", len(results))
	}
}

func TestDryRunClient_GetTransitions_ReturnsEmpty(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)

	transitions, err := c.GetTransitions(context.Background(), "TAL-1")
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}
	if len(transitions) != 0 {
		t.Errorf("GetTransitions() len = %d, want 0", len(transitions))
	}
}

func TestDryRunClient_WriteOps_NoError(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)
	ctx := context.Background()

	if err := c.DoTransition(ctx, "TAL-1", "21"); err != nil {
		t.Errorf("DoTransition() error = %v", err)
	}
	if err := c.AddComment(ctx, "TAL-1", evidence.NewADFDocument("hi")); err != nil {
		t.Errorf("AddComment() error = %v", err)
	}
	if err := c.AddWorklog(ctx, "TAL-1", evidence.Worklog{TimeSpentSeconds: 3600}); err != nil {
		t.Errorf("AddWorklog() error = %v", err)
	}
	if err := c.CreateRemoteLink(ctx, "TAL-1", evidence.RemoteLink{PRURL: "https://github.com/pr/1"}); err != nil {
		t.Errorf("CreateRemoteLink() error = %v", err)
	}
	if err := c.AddAttachment(ctx, "TAL-1", evidence.Attachment{Filename: "report.md"}); err != nil {
		t.Errorf("AddAttachment() error = %v", err)
	}
}

func TestDryRunClient_LogsToWriter(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)

	_, _ = c.CreateIssue(context.Background(), evidence.CreateIssueRequest{Summary: "dry-run-issue"})
	_ = c.AddComment(context.Background(), "TAL-1", evidence.NewADFDocument("hello"))

	logged := buf.String()
	if logged == "" {
		t.Error("DryRunClient wrote nothing to the output writer")
	}
	// Log must mention at least one of the called methods.
	if !strings.Contains(logged, "CreateIssue") && !strings.Contains(logged, "AddComment") {
		t.Errorf("DryRunClient log does not mention called methods: %q", logged)
	}
}

// TestDryRunClient_IntegratesWithRunSteps verifies that DryRunClient wires
// cleanly with service.RunSteps for the propose preset, producing no error
// and no network calls.
func TestDryRunClient_IntegratesWithRunSteps(t *testing.T) {
	var buf bytes.Buffer
	c := dryrun.NewClient(&buf)

	cfg := service.Config{
		SiteURL:               "https://dry.atlassian.net",
		CloudID:               "dry",
		ProjectKey:            "DRY",
		ProjectID:             "10001",
		IssueTypeName:         "Task",
		DefaultWorklogSeconds: 3600,
		RemoteLinkRelationship: "is implemented by",
		OrderedStates: map[string][]string{
			service.StatusCategoryNew:           {"To Do"},
			service.StatusCategoryIndeterminate: {"In Progress"},
			service.StatusCategoryDone:          {"Done"},
		},
		Credentials: service.Credentials{
			Email:    "dry@example.com",
			APIToken: "dry-token",
		},
	}

	loop := service.NewEvidenceLoop(c, cfg)
	preset, err := service.PhasePreset("propose")
	if err != nil {
		t.Fatalf("PhasePreset: %v", err)
	}

	in := service.RunInput{
		Module:       "dry-module",
		Agent:        "hermes",
		Change:       "DRY-1",
		Phase:        "propose",
		Summary:      "Dry run propose",
		Description:  evidence.NewADFDocument("desc"),
		Comment:      evidence.NewADFDocument("comment"),
		OwnershipMap: map[string]string{"dry-module": "hermes"},
	}

	key, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps with DryRunClient error = %v", err)
	}
	if key == "" {
		t.Error("RunSteps with DryRunClient returned empty key")
	}
	if buf.Len() == 0 {
		t.Error("DryRunClient produced no log output during RunSteps")
	}
}
