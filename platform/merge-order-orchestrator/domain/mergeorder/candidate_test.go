package mergeorder_test

import (
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
)

func TestCandidate_ZeroValue(t *testing.T) {
	t.Parallel()
	var c mergeorder.Candidate
	if c.Figura != "" || c.Branch != "" || c.Head != "" || c.Path != "" {
		t.Errorf("zero-value Candidate has non-empty string fields: %+v", c)
	}
	if c.CommitsAhead != 0 {
		t.Errorf("zero-value CommitsAhead = %d, want 0", c.CommitsAhead)
	}
	if c.ChangedFiles != nil {
		t.Errorf("zero-value ChangedFiles = %v, want nil", c.ChangedFiles)
	}
	if !c.CreatedAt.IsZero() {
		t.Errorf("zero-value CreatedAt = %v, want zero time", c.CreatedAt)
	}
}

func TestCandidate_AllFields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	c := mergeorder.Candidate{
		Figura:       "hermes",
		Branch:       "agent/hermes/TAL-3",
		Head:         "abc123",
		Path:         "talos.wt/agent-hermes",
		CommitsAhead: 3,
		ChangedFiles: []string{"go.mod", "main.go"},
		CreatedAt:    now,
	}
	if c.Figura != "hermes" {
		t.Errorf("Figura = %q, want %q", c.Figura, "hermes")
	}
	if c.Branch != "agent/hermes/TAL-3" {
		t.Errorf("Branch = %q, want %q", c.Branch, "agent/hermes/TAL-3")
	}
	if c.Head != "abc123" {
		t.Errorf("Head = %q, want %q", c.Head, "abc123")
	}
	if c.Path != "talos.wt/agent-hermes" {
		t.Errorf("Path = %q, want %q", c.Path, "talos.wt/agent-hermes")
	}
	if c.CommitsAhead != 3 {
		t.Errorf("CommitsAhead = %d, want 3", c.CommitsAhead)
	}
	if len(c.ChangedFiles) != 2 {
		t.Errorf("ChangedFiles len = %d, want 2", len(c.ChangedFiles))
	}
	if !c.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", c.CreatedAt, now)
	}
}
