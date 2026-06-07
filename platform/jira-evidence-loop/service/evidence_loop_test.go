package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/mock"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validConfig() service.Config {
	return service.Config{
		SiteURL:               "https://test.atlassian.net",
		CloudID:               "cloud-id",
		ProjectKey:            "TAL",
		ProjectID:             "10099",
		IssueTypeName:         "Tarea",
		DefaultWorklogSeconds: 3600,
		RemoteLinkRelationship: "is implemented by",
		OrderedStates: map[string][]string{
			service.StatusCategoryNew:           {"Por hacer"},
			service.StatusCategoryIndeterminate: {"En curso", "En revisión", "Bloqueado"},
			service.StatusCategoryDone:          {"Listo"},
		},
		Credentials: service.Credentials{
			Email:    "test@example.com",
			APIToken: "token123",
		},
	}
}

func validInput() service.RunInput {
	return service.RunInput{
		Module:      "jira-loop",
		Agent:       "hermes",
		Change:      "TAL-1",
		Phase:       "apply",
		Summary:     "Evidence for TAL-1 apply",
		Description: evidence.NewADFDocument("Evidence body"),
		Comment:     evidence.NewADFDocument("Step comment"),
		PRURL:       "https://github.com/org/repo/pull/42",
		Attachment: evidence.Attachment{
			Filename:    "verify-report.md",
			ContentType: "text/markdown",
			Data:        []byte("# Report"),
		},
		WorklogSeconds: 3600,
		OwnershipMap: map[string]string{
			"jira-loop": "hermes",
		},
	}
}

// transitionsForCategories returns a minimal transition list covering all
// needed categories with unambiguous names.
func transitionsForCategories() []evidence.Transition {
	return []evidence.Transition{
		{ID: "11", ToName: "Por hacer", ToCategory: "new"},
		{ID: "21", ToName: "En curso", ToCategory: "indeterminate"},
		{ID: "51", ToName: "Listo", ToCategory: "done"},
	}
}

// ---------------------------------------------------------------------------
// Config validation
// ---------------------------------------------------------------------------

func TestNewConfig_Valid(t *testing.T) {
	cfg, err := service.NewConfig(validConfig())
	if err != nil {
		t.Fatalf("NewConfig() error = %v, want nil", err)
	}
	if cfg.ProjectKey != "TAL" {
		t.Errorf("ProjectKey = %q, want \"TAL\"", cfg.ProjectKey)
	}
}

func TestNewConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*service.Config)
	}{
		{"empty SiteURL", func(c *service.Config) { c.SiteURL = "" }},
		{"empty ProjectKey", func(c *service.Config) { c.ProjectKey = "" }},
		{"empty ProjectID", func(c *service.Config) { c.ProjectID = "" }},
		{"empty IssueTypeName", func(c *service.Config) { c.IssueTypeName = "" }},
		{"empty OrderedStates", func(c *service.Config) { c.OrderedStates = nil }},
		{"missing new states", func(c *service.Config) { delete(c.OrderedStates, service.StatusCategoryNew) }},
		{"missing indeterminate states", func(c *service.Config) { delete(c.OrderedStates, service.StatusCategoryIndeterminate) }},
		{"missing done states", func(c *service.Config) { delete(c.OrderedStates, service.StatusCategoryDone) }},
		{"empty RemoteLinkRelationship", func(c *service.Config) { c.RemoteLinkRelationship = "" }},
		{"empty Credentials.Email", func(c *service.Config) { c.Credentials.Email = "" }},
		{"empty Credentials.APIToken", func(c *service.Config) { c.Credentials.APIToken = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			if _, err := service.NewConfig(cfg); err == nil {
				t.Errorf("NewConfig() expected error for %q, got nil", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Happy path: 7-step order
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_HappyPath(t *testing.T) {
	m := mock.NewJiraClientMock()
	// Step 1: Search returns 0 matches → CreateIssue
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-42"}
	// Steps 2 and 7: GetTransitions returns all transitions
	m.GetTransitionsResult = transitionsForCategories()

	cfg := validConfig()
	loop := service.NewEvidenceLoop(m, cfg)
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if key != "TAL-42" {
		t.Errorf("Run() key = %q, want \"TAL-42\"", key)
	}

	// Assert 7-step call order:
	// Search → CreateIssue → GetTransitions → DoTransition →
	// AddComment → AddWorklog → CreateRemoteLink → AddAttachment →
	// GetTransitions → DoTransition
	wantOrder := []string{
		"Search",
		"CreateIssue",
		"GetTransitions",
		"DoTransition",
		"AddComment",
		"AddWorklog",
		"CreateRemoteLink",
		"AddAttachment",
		"GetTransitions",
		"DoTransition",
	}
	m.AssertMethodOrder(t, wantOrder)
}

// ---------------------------------------------------------------------------
// Idempotency: 1 existing match → reuse key, skip CreateIssue
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_IdempotentOneMatch(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = []evidence.Issue{{Key: "TAL-7", Summary: "existing"}}
	m.GetTransitionsResult = transitionsForCategories()

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if key != "TAL-7" {
		t.Errorf("Run() key = %q, want \"TAL-7\"", key)
	}
	m.AssertCallCount(t, "CreateIssue", 0)
	m.AssertCallCount(t, "Search", 1)
}

// ---------------------------------------------------------------------------
// Idempotency: 0 matches → CreateIssue called
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_IdempotentZeroMatches(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-99"}
	m.GetTransitionsResult = transitionsForCategories()

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	m.AssertCallCount(t, "Search", 1)
	m.AssertCallCount(t, "CreateIssue", 1)
}

// ---------------------------------------------------------------------------
// Idempotency: >1 matches → error, no further calls
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_IdempotentMultipleMatches(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = []evidence.Issue{
		{Key: "TAL-1"},
		{Key: "TAL-2"},
	}

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected error for >1 search matches, got nil")
	}
	m.AssertCallCount(t, "CreateIssue", 0)
	// No write operations after the error
	m.AssertCallCount(t, "DoTransition", 0)
}

// ---------------------------------------------------------------------------
// Ownership violation: no API call must be made
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_OwnershipViolation(t *testing.T) {
	m := mock.NewJiraClientMock()

	input := validInput()
	input.OwnershipMap = map[string]string{
		"jira-loop": "atlas", // hermes != atlas
	}

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected ownership error, got nil")
	}
	var ve *evidence.ErrOwnershipViolation
	if !errors.As(err, &ve) {
		t.Errorf("error type = %T, want *ErrOwnershipViolation", err)
	}
	// Zero API calls must have been made.
	if len(m.Calls) != 0 {
		t.Errorf("expected zero API calls on ownership violation, got %d: %v", len(m.Calls), m.Calls)
	}
}

// ---------------------------------------------------------------------------
// Error propagation: CreateIssue fails
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_CreateIssueError(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueErr = mock.ErrSentinel("create failed")

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected error from CreateIssue, got nil")
	}
}

// ---------------------------------------------------------------------------
// Error propagation: AddAttachment fails loud
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_AttachmentFails(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-5"}
	m.GetTransitionsResult = transitionsForCategories()
	m.AddAttachmentErr = mock.ErrSentinel("attachment upload failed")

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected error from AddAttachment, got nil")
	}
	// Attachment must have been attempted
	m.AssertCallCount(t, "AddAttachment", 1)
	// Transition to done must NOT have been attempted after the failure
	m.AssertCallCount(t, "DoTransition", 1) // only the indeterminate transition
}

// ---------------------------------------------------------------------------
// Error propagation: CreateRemoteLink fails loud
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_RemoteLinkFails(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-6"}
	m.GetTransitionsResult = transitionsForCategories()
	m.CreateRemoteLinkErr = mock.ErrSentinel("remote link failed")

	loop := service.NewEvidenceLoop(m, validConfig())
	_, err := loop.Run(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected error from CreateRemoteLink, got nil")
	}
	m.AssertCallCount(t, "CreateRemoteLink", 1)
	// Attachment and final transition must not run after remote-link failure
	m.AssertCallCount(t, "AddAttachment", 0)
	m.AssertCallCount(t, "DoTransition", 1) // only the indeterminate transition
}

// ---------------------------------------------------------------------------
// No panic under adversarial inputs
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Run() panicked: %v", r)
		}
	}()

	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{} // empty key
	m.GetTransitionsResult = nil           // no transitions

	loop := service.NewEvidenceLoop(m, validConfig())
	// Should error gracefully (unreachable transition), not panic.
	loop.Run(context.Background(), validInput()) //nolint:errcheck
}

// ---------------------------------------------------------------------------
// Transition wiring: already in target state (no-op)
// ---------------------------------------------------------------------------

func TestEvidenceLoop_Run_TransitionNoOpWhenAlreadyIndeterminate(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-10"}
	// GetTransitions returns transitions but NOT indeterminate (already there) —
	// simulate by returning only new and done transitions.
	m.GetTransitionsResult = []evidence.Transition{
		{ID: "11", ToName: "Por hacer", ToCategory: "new"},
		{ID: "51", ToName: "Listo", ToCategory: "done"},
	}

	loop := service.NewEvidenceLoop(m, validConfig())
	// Step 2 should be no-op (no indeterminate transition available = already there).
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if key != "TAL-10" {
		t.Errorf("key = %q, want \"TAL-10\"", key)
	}
	// DoTransition called once only (for done at step 7), not for indeterminate.
	m.AssertCallCount(t, "DoTransition", 1)
}

// ---------------------------------------------------------------------------
// Transition disambiguation: multiple candidates in the same category
//
// The real TAL board has THREE indeterminate states: "En curso", "En revisión",
// "Bloqueado". When GetTransitions returns all three, doTransitionToCategory
// must pick the first entry in OrderedStates["indeterminate"] (ordinal=0),
// which is "En curso" (ID "21"). This exercises the multi-candidate branch
// that the happy-path tests never reach.
// ---------------------------------------------------------------------------

// threeIndeterminateTransitions simulates the real TAL board: three transitions
// in the indeterminate category with distinct IDs and names.
func threeIndeterminateTransitions() []evidence.Transition {
	return []evidence.Transition{
		// indeterminate candidates in a board-API order that differs from
		// OrderedStates so we verify name-match, not position-match.
		{ID: "31", ToName: "Bloqueado", ToCategory: service.StatusCategoryIndeterminate},
		{ID: "21", ToName: "En curso", ToCategory: service.StatusCategoryIndeterminate},
		{ID: "41", ToName: "En revisión", ToCategory: service.StatusCategoryIndeterminate},
		// Non-indeterminate transitions always present.
		{ID: "11", ToName: "Por hacer", ToCategory: service.StatusCategoryNew},
		{ID: "51", ToName: "Listo", ToCategory: service.StatusCategoryDone},
	}
}

// doTransitionIDFor is a test helper that returns the transitionID argument
// from the n-th DoTransition call in the mock's call log (0-indexed).
func doTransitionIDFor(t *testing.T, m *mock.JiraClientMock, callIndex int) string {
	t.Helper()
	dt := m.CallsFor("DoTransition")
	if callIndex >= len(dt) {
		t.Fatalf("DoTransition call[%d] does not exist (total=%d)", callIndex, len(dt))
	}
	id, ok := dt[callIndex].Args[1].(string)
	if !ok {
		t.Fatalf("DoTransition Args[1] is not a string: %T", dt[callIndex].Args[1])
	}
	return id
}

// TestDisambiguation_InProgressPicksEnCurso verifies that when GetTransitions
// returns all three indeterminate states, the service selects "En curso" (the
// first entry in OrderedStates["indeterminate"]) for the In-Progress step.
func TestDisambiguation_InProgressPicksEnCurso(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-20"}
	m.GetTransitionsResult = threeIndeterminateTransitions()

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if key != "TAL-20" {
		t.Errorf("key = %q, want \"TAL-20\"", key)
	}

	// Step 2 (indeterminate): must pick "En curso" → ID "21".
	// Step 7 (done): must pick "Listo" → ID "51".
	m.AssertCallCount(t, "DoTransition", 2)

	if got := doTransitionIDFor(t, m, 0); got != "21" {
		t.Errorf("step 2 DoTransition ID = %q, want \"21\" (En curso)", got)
	}
	if got := doTransitionIDFor(t, m, 1); got != "51" {
		t.Errorf("step 7 DoTransition ID = %q, want \"51\" (Listo)", got)
	}
}

// TestDisambiguation_DoneAlwaysPictsListo verifies that the done-category
// disambiguation still resolves correctly when indeterminate has 3 candidates
// (i.e., the done branch is unaffected by indeterminate noise).
func TestDisambiguation_DoneAlwaysPictsListo(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = []evidence.Issue{{Key: "TAL-21"}} // reuse existing
	m.GetTransitionsResult = threeIndeterminateTransitions()

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if key != "TAL-21" {
		t.Errorf("key = %q, want \"TAL-21\"", key)
	}

	// Step 7 must select "Listo" (ID "51"), the sole done candidate.
	if got := doTransitionIDFor(t, m, 1); got != "51" {
		t.Errorf("step 7 DoTransition ID = %q, want \"51\" (Listo)", got)
	}
}

// TestDisambiguation_CandidateNotInOrderedStates verifies the fallback
// behaviour when the OrderedStates name for ordinal=0 does NOT appear among
// the returned candidates. The code falls back to candidates[0] (first in the
// slice returned by GetTransitions). This is explicitly documented behaviour:
// the fallback is deterministic relative to the API's ordering, not random.
// The test asserts the actual behaviour so that any accidental change is caught.
func TestDisambiguation_CandidateNotInOrderedStates(t *testing.T) {
	// Config with an indeterminate name that won't match any returned transition.
	cfgMismatched := validConfig()
	cfgMismatched.OrderedStates[service.StatusCategoryIndeterminate] = []string{"Inexistente"}

	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-22"}
	// Three indeterminate candidates; none is named "Inexistente".
	// Slice order: Bloqueado (31), En curso (21), En revisión (41).
	m.GetTransitionsResult = threeIndeterminateTransitions()

	loop := service.NewEvidenceLoop(m, cfgMismatched)
	_, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v (expected fallback to succeed)", err)
	}

	// The fallback uses candidates[0], which is the first element in the
	// filtered indeterminate slice — iteration order over threeIndeterminateTransitions
	// means the first indeterminate candidate appended is "Bloqueado" (ID "31").
	if got := doTransitionIDFor(t, m, 0); got != "31" {
		t.Errorf("fallback DoTransition ID = %q, want \"31\" (Bloqueado, first indeterminate candidate)", got)
	}
}

// TestDisambiguation_NoCandidatesForCategory verifies that when GetTransitions
// returns transitions for other categories but none for the target category,
// the result is a no-op (nil error), not ErrTransitionUnreachable.
// The issue is assumed to already be in the target state.
func TestDisambiguation_NoCandidatesForCategory(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-23"}
	// No indeterminate transitions — issue is already In Progress.
	m.GetTransitionsResult = []evidence.Transition{
		{ID: "11", ToName: "Por hacer", ToCategory: service.StatusCategoryNew},
		{ID: "51", ToName: "Listo", ToCategory: service.StatusCategoryDone},
	}

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (no-op when already in target state)", err)
	}
	if key != "TAL-23" {
		t.Errorf("key = %q, want \"TAL-23\"", key)
	}
	// Only the done transition fires (step 7); step 2 is a no-op.
	m.AssertCallCount(t, "DoTransition", 1)
	if got := doTransitionIDFor(t, m, 0); got != "51" {
		t.Errorf("DoTransition ID = %q, want \"51\" (Listo, step 7)", got)
	}
}
