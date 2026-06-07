//go:build integration

package rest

import (
	"context"
	"os"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// TestIntegration_CreateAndSearchIssue is a smoke test that exercises the real
// Jira REST API. It is guarded by the `integration` build tag and by an env
// check so it never runs in the normal suite (go test ./...) without a token.
//
// Prerequisites (ZEUS must supply):
//   - JIRA_EMAIL set to a valid Jira user e-mail
//   - JIRA_API_TOKEN set to a valid Jira API token
//   - JIRA_SITE_URL set to the target Jira Cloud instance base URL
//
// Run manually:
//
//	JIRA_EMAIL=... JIRA_API_TOKEN=... JIRA_SITE_URL=https://org.atlassian.net \
//	  go test -tags=integration ./adapter/rest/ -run TestIntegration -v
func TestIntegration_CreateAndSearchIssue(t *testing.T) {
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	siteURL := os.Getenv("JIRA_SITE_URL")

	if email == "" || token == "" || siteURL == "" {
		t.Skip("integration test skipped: JIRA_EMAIL, JIRA_API_TOKEN, and JIRA_SITE_URL must be set")
	}

	cfg := service.Config{
		SiteURL:       siteURL,
		ProjectKey:    "TAL",
		ProjectID:     "10099",
		IssueTypeName: "Tarea",
		OrderedStates: map[string][]string{
			service.StatusCategoryNew:           {"Por hacer"},
			service.StatusCategoryIndeterminate: {"En curso", "En revisión", "Bloqueado"},
			service.StatusCategoryDone:          {"Listo"},
		},
		DefaultWorklogSeconds:  3600,
		RemoteLinkRelationship: "is implemented by",
		Credentials: service.Credentials{
			Email:    email,
			APIToken: token,
		},
	}

	client := NewClient(cfg)
	ctx := context.Background()

	// Step 1: Create a smoke-test issue.
	req := evidence.CreateIssueRequest{
		ProjectKey:    cfg.ProjectKey,
		ProjectID:     cfg.ProjectID,
		IssueTypeName: cfg.IssueTypeName,
		Summary:       "[integration-smoke] jira-evidence-loop adapter test",
		Description:   evidence.NewADFDocument("Smoke test created by the integration test suite."),
		Labels: evidence.LabelSet{
			{Key: "change", Value: "smoke"},
			{Key: "phase", Value: "integration"},
			{Key: "agent", Value: "hermes"},
			{Key: "module", Value: "jira-loop"},
		},
	}
	issue, err := client.CreateIssue(ctx, req)
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	t.Logf("created issue: %s", issue.Key)

	// Step 2: Search for the issue by label to confirm it exists.
	jql := `project = "TAL" AND labels = "change:smoke" AND labels = "phase:integration"`
	issues, err := client.Search(ctx, jql, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	found := false
	for _, iss := range issues {
		if iss.Key == issue.Key {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Search did not return newly created issue %s", issue.Key)
	}

	// Step 3: GetTransitions to verify the adapter can read transition metadata.
	transitions, err := client.GetTransitions(ctx, issue.Key)
	if err != nil {
		t.Fatalf("GetTransitions(%s): %v", issue.Key, err)
	}
	if len(transitions) == 0 {
		t.Errorf("GetTransitions returned 0 transitions for %s", issue.Key)
	}
	t.Logf("transitions for %s: %v", issue.Key, transitions)

	// Step 4: AddComment to verify ADF posting works.
	comment := evidence.NewADFDocument("Smoke test comment from integration test.")
	if err := client.AddComment(ctx, issue.Key, comment); err != nil {
		t.Fatalf("AddComment(%s): %v", issue.Key, err)
	}

	t.Logf("integration smoke passed for issue %s", issue.Key)
}
