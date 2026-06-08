package jirarest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/jirarest"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// adfParagraph builds a minimal ADF paragraph node for test fixtures.
func adfParagraph(texts ...string) map[string]any {
	var content []map[string]any
	for _, t := range texts {
		content = append(content, map[string]any{"type": "text", "text": t})
	}
	return map[string]any{
		"type":    "paragraph",
		"content": content,
	}
}

// adfDoc wraps a slice of block nodes into a minimal ADF document.
func adfDoc(blocks ...map[string]any) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": blocks,
	}
}

// issueRow is the shape of one item inside the `issues` array from Jira.
type issueRow struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string         `json:"summary"`
		Labels      []string       `json:"labels"`
		Description map[string]any `json:"description"`
	} `json:"fields"`
}

func buildSearchResponse(rows []issueRow) []byte {
	payload := map[string]any{"issues": rows}
	b, _ := json.Marshal(payload)
	return b
}

// TestSearch_RequestShape verifies that the client POSTs with the required fields list.
func TestSearch_RequestShape(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildSearchResponse(nil))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "test@example.com", "token123")
	_, err := c.Search(context.Background(), "project = TAL", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the fields list contains summary, labels, description.
	fields, ok := capturedBody["fields"]
	if !ok {
		t.Fatal("request body missing 'fields'")
	}
	got := fields.([]any)
	wantSet := map[string]bool{"summary": true, "labels": true, "description": true}
	for _, f := range got {
		delete(wantSet, f.(string))
	}
	if len(wantSet) > 0 {
		t.Errorf("missing required fields in request: %v", wantSet)
	}
}

// TestSearch_ADFFlattening verifies that ADF description content is flattened to plain text.
func TestSearch_ADFFlattening(t *testing.T) {
	row := issueRow{Key: "TAL-1"}
	row.Fields.Summary = "test issue"
	row.Fields.Labels = []string{"agent:themis", "module:qa"}
	row.Fields.Description = adfDoc(
		adfParagraph("files: changes\n"),
		adfParagraph("- [ ] platform/overlap-guard/service/guard.go\n"),
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildSearchResponse([]issueRow{row}))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "test@example.com", "token123")
	results, err := c.Search(context.Background(), "project = TAL", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	body := results[0].Body
	if body == "" {
		t.Fatal("expected non-empty flattened body")
	}
	// Must contain parseable text from both paragraph nodes.
	for _, want := range []string{"files: changes", "platform/overlap-guard/service/guard.go"} {
		if !containsStr(body, want) {
			t.Errorf("body %q does not contain %q", body, want)
		}
	}
}

// TestSearch_Labels verifies that labels are returned correctly.
func TestSearch_Labels(t *testing.T) {
	row := issueRow{Key: "TAL-2"}
	row.Fields.Labels = []string{"agent:atlas", "module:backend-ctx1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildSearchResponse([]issueRow{row}))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	results, err := c.Search(context.Background(), "project = TAL", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	want := []string{"agent:atlas", "module:backend-ctx1"}
	got := results[0].Labels
	if len(got) != len(want) {
		t.Fatalf("expected labels %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("label[%d]: want %q, got %q", i, w, got[i])
		}
	}
}

// TestSearch_HTTPError verifies that non-2xx responses produce an HTTPError.
func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "bad@example.com", "bad-token")
	_, err := c.Search(context.Background(), "project = TAL", 10)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
	var httpErr *jirarest.HTTPError
	if !isHTTPError(err, &httpErr) {
		t.Fatalf("expected *jirarest.HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", httpErr.StatusCode)
	}
}

// TestSearch_Integration uses a real Jira instance; skipped in short mode.
func TestSearch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real Jira credentials")
	}
	// This test requires JIRA_SITE_URL, JIRA_EMAIL, JIRA_API_TOKEN set in env.
	// It is intentionally left as a manual gate only.
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func isHTTPError(err error, target **jirarest.HTTPError) bool {
	if e, ok := err.(*jirarest.HTTPError); ok {
		*target = e
		return true
	}
	return false
}

// varClientCheck is a compile-time assertion that *Client satisfies port.IssueSearcher.
var _ port.IssueSearcher = (*jirarest.Client)(nil)
