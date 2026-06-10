package rest

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// testServer starts a local httptest.Server and returns a *Client wired to it
// together with a channel that receives the incoming *http.Request for each
// call so the test can inspect it.
//
// The handler sends the provided status code and JSON body back. Tests that
// need to inspect the request pull from reqCh after making the client call.
func testServer(t *testing.T, status int, body string) (*Client, chan *http.Request, func()) {
	t.Helper()
	reqCh := make(chan *http.Request, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body so the client doesn't get broken-pipe.
		_, _ = io.ReadAll(r.Body)
		r.Body = http.NoBody
		reqCh <- r
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}))
	cfg := service.Config{
		SiteURL:    srv.URL,
		ProjectKey: "TAL",
		ProjectID:  "10099",
		Credentials: service.Credentials{
			Email:    "user@example.com",
			APIToken: "secret",
		},
	}
	c := NewClient(cfg)
	return c, reqCh, srv.Close
}

// testServerCapturingBody is like testServer but the handler reads and stores
// the raw request body before forwarding the request to the channel.
func testServerCapturingBody(t *testing.T, status int, respBody string) (*Client, chan capturedRequest, func()) {
	t.Helper()
	reqCh := make(chan capturedRequest, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		reqCh <- capturedRequest{Request: r, Body: raw}
		if respBody != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if respBody != "" {
			_, _ = io.WriteString(w, respBody)
		}
	}))
	cfg := service.Config{
		SiteURL:    srv.URL,
		ProjectKey: "TAL",
		ProjectID:  "10099",
		Credentials: service.Credentials{
			Email:    "user@example.com",
			APIToken: "secret",
		},
	}
	c := NewClient(cfg)
	return c, reqCh, srv.Close
}

type capturedRequest struct {
	*http.Request
	Body []byte
}

func assertBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("Authorization header = %q, want Basic ...", auth)
	}
}

// ---------------------------------------------------------------------------
// CreateIssue
// ---------------------------------------------------------------------------

func TestClient_CreateIssue_HappyPath(t *testing.T) {
	respBody := `{"key":"TAL-1","fields":{"summary":"Test Issue","labels":["change:c1"]}}`
	client, reqCh, stop := testServerCapturingBody(t, http.StatusCreated, respBody)
	defer stop()

	req := evidence.CreateIssueRequest{
		ProjectKey:    "TAL",
		ProjectID:     "10099",
		IssueTypeName: "Tarea",
		Summary:       "Test Issue",
		Labels: evidence.LabelSet{
			{Key: "change", Value: "c1"},
		},
	}
	issue, err := client.CreateIssue(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Key != "TAL-1" {
		t.Errorf("issue.Key = %q, want %q", issue.Key, "TAL-1")
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue" {
		t.Errorf("path = %q, want /rest/api/3/issue", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	// Assert body shape: must have fields.project.key and fields.issuetype.name
	var payload map[string]any
	if err := json.Unmarshal(cr.Body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	fields, _ := payload["fields"].(map[string]any)
	if fields == nil {
		t.Fatal("body.fields is missing")
	}
	proj, _ := fields["project"].(map[string]any)
	if proj["key"] != "TAL" {
		t.Errorf("fields.project.key = %v, want TAL", proj["key"])
	}
	issuetype, _ := fields["issuetype"].(map[string]any)
	if issuetype["name"] != "Tarea" {
		t.Errorf("fields.issuetype.name = %v, want Tarea", issuetype["name"])
	}
}

func TestClient_CreateIssue_NonCreatedStatus(t *testing.T) {
	client, _, stop := testServerCapturingBody(t, http.StatusBadRequest, `{"errorMessages":["bad"]}`)
	defer stop()
	_, err := client.CreateIssue(context.Background(), evidence.CreateIssueRequest{
		ProjectKey: "TAL", ProjectID: "10099", IssueTypeName: "Tarea", Summary: "x",
	})
	if err == nil {
		t.Error("expected error on 400, got nil")
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestClient_Search_HappyPath(t *testing.T) {
	respBody := `{"issues":[{"key":"TAL-1","fields":{"summary":"s","labels":[]}}]}`
	client, reqCh, stop := testServer(t, http.StatusOK, respBody)
	defer stop()

	issues, err := client.Search(context.Background(), `project = "TAL"`, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1", len(issues))
	}
	if issues[0].Key != "TAL-1" {
		t.Errorf("issues[0].Key = %q, want TAL-1", issues[0].Key)
	}

	r := <-reqCh
	if r.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", r.Method)
	}
	if r.URL.Path != "/rest/api/3/search/jql" {
		t.Errorf("path = %q, want /rest/api/3/search/jql", r.URL.Path)
	}
	assertBasicAuth(t, r)
}

func TestClient_Search_Empty(t *testing.T) {
	client, _, stop := testServer(t, http.StatusOK, `{"issues":[]}`)
	defer stop()
	issues, err := client.Search(context.Background(), `project = "TAL"`, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues, want 0", len(issues))
	}
}

// ---------------------------------------------------------------------------
// GetTransitions
// ---------------------------------------------------------------------------

func TestClient_GetTransitions_HappyPath(t *testing.T) {
	respBody := `{"transitions":[{"id":"21","name":"En curso","to":{"statusCategory":{"key":"indeterminate"}}}]}`
	client, reqCh, stop := testServer(t, http.StatusOK, respBody)
	defer stop()

	transitions, err := client.GetTransitions(context.Background(), "TAL-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("got %d transitions, want 1", len(transitions))
	}
	if transitions[0].ID != "21" {
		t.Errorf("transitions[0].ID = %q, want 21", transitions[0].ID)
	}
	if transitions[0].ToCategory != "indeterminate" {
		t.Errorf("ToCategory = %q, want indeterminate", transitions[0].ToCategory)
	}

	r := <-reqCh
	if r.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", r.Method)
	}
	if r.URL.Path != "/rest/api/3/issue/TAL-1/transitions" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/transitions", r.URL.Path)
	}
	assertBasicAuth(t, r)
}

// ---------------------------------------------------------------------------
// DoTransition
// ---------------------------------------------------------------------------

func TestClient_DoTransition_HappyPath(t *testing.T) {
	client, reqCh, stop := testServerCapturingBody(t, http.StatusNoContent, "")
	defer stop()

	if err := client.DoTransition(context.Background(), "TAL-1", "21"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue/TAL-1/transitions" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/transitions", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	var payload map[string]any
	if err := json.Unmarshal(cr.Body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	trans, _ := payload["transition"].(map[string]any)
	if trans["id"] != "21" {
		t.Errorf("transition.id = %v, want 21", trans["id"])
	}
}

func TestClient_DoTransition_NonSuccess(t *testing.T) {
	client, _, stop := testServer(t, http.StatusBadRequest, `{}`)
	defer stop()
	err := client.DoTransition(context.Background(), "TAL-1", "99")
	if err == nil {
		t.Error("expected error on 400, got nil")
	}
}

// ---------------------------------------------------------------------------
// AddComment
// ---------------------------------------------------------------------------

func TestClient_AddComment_HappyPath(t *testing.T) {
	client, reqCh, stop := testServerCapturingBody(t, http.StatusCreated, `{"id":"10001"}`)
	defer stop()

	doc := evidence.NewADFDocument("great work")
	if err := client.AddComment(context.Background(), "TAL-1", doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue/TAL-1/comment" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/comment", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	// Body must have "body" field containing the ADF document.
	var payload map[string]any
	if err := json.Unmarshal(cr.Body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if payload["body"] == nil {
		t.Error("request body.body is missing")
	}
}

// ---------------------------------------------------------------------------
// AddWorklog
// ---------------------------------------------------------------------------

func TestClient_AddWorklog_HappyPath(t *testing.T) {
	client, reqCh, stop := testServerCapturingBody(t, http.StatusCreated, `{"id":"10002"}`)
	defer stop()

	wl := evidence.Worklog{
		TimeSpentSeconds: 3600,
		Comment:          evidence.NewADFDocument("done"),
		Started:          "2026-06-07T09:00:00.000+0000",
	}
	if err := client.AddWorklog(context.Background(), "TAL-1", wl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue/TAL-1/worklog" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/worklog", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	var payload map[string]any
	if err := json.Unmarshal(cr.Body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if payload["timeSpent"] == nil {
		t.Error("request body.timeSpent is missing")
	}
	if payload["started"] == nil {
		t.Error("request body.started is missing")
	}
	// ADF comment body must be present.
	if payload["comment"] == nil {
		t.Error("request body.comment is missing")
	}
}

// ---------------------------------------------------------------------------
// CreateRemoteLink
// ---------------------------------------------------------------------------

func TestClient_CreateRemoteLink_HappyPath(t *testing.T) {
	client, reqCh, stop := testServerCapturingBody(t, http.StatusCreated, `{"id":100}`)
	defer stop()

	link := evidence.RemoteLink{
		PRURL:        "https://github.com/org/repo/pull/42",
		Relationship: "is implemented by",
	}
	if err := client.CreateRemoteLink(context.Background(), "TAL-1", link); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue/TAL-1/remotelink" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/remotelink", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	var payload map[string]any
	if err := json.Unmarshal(cr.Body, &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if payload["globalId"] == nil {
		t.Error("request body.globalId is missing")
	}
	if payload["relationship"] == nil {
		t.Error("request body.relationship is missing")
	}
	obj, _ := payload["object"].(map[string]any)
	if obj == nil || obj["url"] == nil {
		t.Error("request body.object.url is missing")
	}
}

// ---------------------------------------------------------------------------
// AddAttachment — multipart/form-data + X-Atlassian-Token: no-check
// ---------------------------------------------------------------------------

func TestClient_AddAttachment_HappyPath(t *testing.T) {
	respBody := `[{"id":"10003","filename":"report.pdf","size":4}]`
	client, reqCh, stop := testServerCapturingBody(t, http.StatusOK, respBody)
	defer stop()

	att := evidence.Attachment{
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		Data:        []byte("data"),
	}
	if err := client.AddAttachment(context.Background(), "TAL-1", att); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cr := <-reqCh
	if cr.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", cr.Method)
	}
	if cr.URL.Path != "/rest/api/3/issue/TAL-1/attachments" {
		t.Errorf("path = %q, want /rest/api/3/issue/TAL-1/attachments", cr.URL.Path)
	}
	assertBasicAuth(t, cr.Request)

	// X-Atlassian-Token: no-check is REQUIRED (REQ-ATTACH).
	if cr.Header.Get("X-Atlassian-Token") != "no-check" {
		t.Errorf("X-Atlassian-Token = %q, want no-check", cr.Header.Get("X-Atlassian-Token"))
	}

	// Content-Type must be multipart/form-data.
	ct := cr.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "multipart/form-data" {
		t.Errorf("Content-Type media type = %q, want multipart/form-data", mediaType)
	}

	// The body must contain the file data (we stored raw bytes above).
	if !strings.Contains(string(cr.Body), "data") {
		t.Errorf("attachment body does not contain file data")
	}
}

func TestClient_AddAttachment_NonSuccess(t *testing.T) {
	client, _, stop := testServer(t, http.StatusForbidden, `{"errorMessages":["forbidden"]}`)
	defer stop()
	att := evidence.Attachment{Filename: "x.pdf", ContentType: "application/pdf", Data: []byte("x")}
	err := client.AddAttachment(context.Background(), "TAL-1", att)
	if err == nil {
		t.Error("expected error on 403, got nil")
	}
}

// ---------------------------------------------------------------------------
// Typed error on non-2xx
// ---------------------------------------------------------------------------

func TestClient_HTTPError_ContainsStatusAndBody(t *testing.T) {
	client, _, stop := testServer(t, http.StatusUnprocessableEntity, `{"errorMessages":["oops"]}`)
	defer stop()

	err := client.DoTransition(context.Background(), "TAL-1", "99")
	if err == nil {
		t.Fatal("expected error on 422, got nil")
	}
	// Error message should contain the status code and body fragment.
	msg := err.Error()
	if !strings.Contains(msg, "422") {
		t.Errorf("error %q does not mention status 422", msg)
	}
}
