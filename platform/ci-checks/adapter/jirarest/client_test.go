package jirarest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/adapter/jirarest"
	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
)

func buildIssueResponse(key string, labels []string) []byte {
	payload := map[string]any{
		"key": key,
		"fields": map[string]any{
			"labels": labels,
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// TestClient_LabelsByKey_OK verifies label decoding from a 200 response.
func TestClient_LabelsByKey_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildIssueResponse("TAL-7", []string{"agent:hermes", "module:devops"}))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "test@example.com", "token123")
	labels, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(labels), labels)
	}
	if labels[0] != "agent:hermes" || labels[1] != "module:devops" {
		t.Errorf("unexpected labels: %v", labels)
	}
}

// TestClient_LabelsByKey_ValidatesPath verifies the request path includes the issue key.
func TestClient_LabelsByKey_ValidatesPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildIssueResponse("TAL-7", nil))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/rest/api/3/issue/TAL-7"
	if capturedPath != want {
		t.Errorf("path = %q, want %q", capturedPath, want)
	}
}

// TestClient_LabelsByKey_ValidatesQueryFields verifies the fields=labels query parameter.
func TestClient_LabelsByKey_ValidatesQueryFields(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildIssueResponse("TAL-7", nil))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedQuery, "fields=labels") {
		t.Errorf("query %q does not contain fields=labels", capturedQuery)
	}
}

// TestClient_LabelsByKey_ValidatesBasicAuth verifies the Authorization header is set.
func TestClient_LabelsByKey_ValidatesBasicAuth(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buildIssueResponse("TAL-7", nil))
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Errorf("Authorization header = %q, want Basic ...", capturedAuth)
	}
}

// TestClient_LabelsByKey_404 verifies that a 404 yields ErrIssueNotFound.
func TestClient_LabelsByKey_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err == nil {
		t.Fatal("expected ErrIssueNotFound, got nil")
	}
	var notFound cichecks.ErrIssueNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected cichecks.ErrIssueNotFound, got %T: %v", err, err)
	}
	if notFound.Key != "TAL-7" {
		t.Errorf("ErrIssueNotFound.Key = %q, want %q", notFound.Key, "TAL-7")
	}
}

// TestClient_LabelsByKey_401 verifies that a 401 yields *HTTPError.
func TestClient_LabelsByKey_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "bad@example.com", "bad-token")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err == nil {
		t.Fatal("expected HTTPError, got nil")
	}
	var httpErr *jirarest.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *jirarest.HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want 401", httpErr.StatusCode)
	}
}

// TestClient_LabelsByKey_500 verifies that a 500 yields *HTTPError.
func TestClient_LabelsByKey_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := jirarest.NewClient(srv.URL, "user@example.com", "tok")
	_, err := c.LabelsByKey(context.Background(), "TAL-7")
	if err == nil {
		t.Fatal("expected HTTPError, got nil")
	}
	var httpErr *jirarest.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *jirarest.HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", httpErr.StatusCode)
	}
}

// TestClient_LabelsByKey_Integration uses a real Jira instance; skipped in short mode.
func TestClient_LabelsByKey_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test — requires real Jira credentials")
	}
	siteURL := os.Getenv("JIRA_SITE_URL")
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	if siteURL == "" || email == "" || token == "" {
		t.Skip("JIRA_SITE_URL/JIRA_EMAIL/JIRA_API_TOKEN not set")
	}
	c := jirarest.NewClient(siteURL, email, token)
	labels, err := c.LabelsByKey(context.Background(), "TAL-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("TAL-5 labels: %v", labels)
}

// compile-time assertion.
var _ interface{ LabelsByKey(context.Context, string) ([]string, error) } = (*jirarest.Client)(nil)
