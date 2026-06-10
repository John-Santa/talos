package jsonlstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/adapter/jsonlstore"
	"github.com/John-Santa/talos/platform/runs/domain/run"
)

var fixedAt = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

// newStore creates a Store backed by t.TempDir; does NOT call t.Setenv because
// Store.New accepts talosHome directly and does not read TALOS_HOME.
// Tests that use this helper can safely call t.Parallel().
func newStore(t *testing.T) *jsonlstore.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := jsonlstore.New(dir)
	if err != nil {
		t.Fatalf("jsonlstore.New: %v", err)
	}
	return s
}

func activityEvent(jiraKey, text string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindActivity, At: at, JiraKey: jiraKey, Text: text}
}

func dispatchEvent(jiraKey, agent, status string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindDispatch, At: at, JiraKey: jiraKey, Agent: agent, Status: status}
}

// ---- Append ----

func TestStore_Append_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := jsonlstore.New(dir)
	if err != nil {
		t.Fatalf("jsonlstore.New: %v", err)
	}

	e := activityEvent("TAL-1", "hello", fixedAt)
	if err := s.Append(context.Background(), e); err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	runsPath := filepath.Join(dir, "runs", "runs.jsonl")
	if _, err := os.Stat(runsPath); os.IsNotExist(err) {
		t.Errorf("runs.jsonl not created at %q", runsPath)
	}
}

func TestStore_Append_MultipleEvents(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	events := []run.RunEvent{
		activityEvent("TAL-1", "first", fixedAt),
		activityEvent("TAL-1", "second", fixedAt.Add(time.Minute)),
		dispatchEvent("TAL-2", "IRIS", "started", fixedAt),
	}
	for _, e := range events {
		if err := s.Append(ctx, e); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	got, err := s.Query(ctx, run.Filter{})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}
}

// ---- Query: filter ----

func TestStore_Query_FilterByJiraKey(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_ = s.Append(ctx, activityEvent("TAL-1", "a", fixedAt))
	_ = s.Append(ctx, activityEvent("TAL-2", "b", fixedAt))

	got, err := s.Query(ctx, run.Filter{JiraKey: "TAL-1"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(got) != 1 || got[0].JiraKey != "TAL-1" {
		t.Errorf("got %+v, want [{JiraKey:TAL-1 ...}]", got)
	}
}

func TestStore_Query_FilterByKind(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_ = s.Append(ctx, activityEvent("TAL-1", "act", fixedAt))
	_ = s.Append(ctx, dispatchEvent("TAL-1", "IRIS", "done", fixedAt))

	got, err := s.Query(ctx, run.Filter{Kind: run.KindDispatch})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(got) != 1 || got[0].Kind != run.KindDispatch {
		t.Errorf("expected 1 dispatch event, got %+v", got)
	}
}

func TestStore_Query_FilterBySince(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	ctx := context.Background()

	_ = s.Append(ctx, activityEvent("TAL-1", "old", fixedAt))
	_ = s.Append(ctx, activityEvent("TAL-1", "new", fixedAt.Add(2*time.Hour)))

	got, err := s.Query(ctx, run.Filter{Since: fixedAt.Add(time.Hour)})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(got) != 1 || got[0].Text != "new" {
		t.Errorf("got %+v, want [{Text:new ...}]", got)
	}
}

// ---- Corrupt line tolerance ----

func TestStore_Query_CorruptLineDiscarded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, _ := jsonlstore.New(dir)
	ctx := context.Background()

	// Write one good event
	_ = s.Append(ctx, activityEvent("TAL-1", "good", fixedAt))

	// Inject a corrupt line directly into the file
	runsDir := filepath.Join(dir, "runs")
	f, _ := os.OpenFile(filepath.Join(runsDir, "runs.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString("{CORRUPT JSON\n")
	_ = f.Close()

	// Write another good event
	_ = s.Append(ctx, activityEvent("TAL-1", "also good", fixedAt.Add(time.Minute)))

	got, err := s.Query(ctx, run.Filter{})
	if err != nil {
		t.Fatalf("Query() error on corrupt file: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 good events, got %d: %+v", len(got), got)
	}
}

// ---- Persistence across instances ----

func TestStore_Persistence_AcrossInstances(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	s1, _ := jsonlstore.New(dir)
	ctx := context.Background()
	_ = s1.Append(ctx, activityEvent("TAL-1", "persisted", fixedAt))

	s2, _ := jsonlstore.New(dir)
	got, err := s2.Query(ctx, run.Filter{})
	if err != nil {
		t.Fatalf("Query() via second instance: %v", err)
	}
	if len(got) != 1 || got[0].Text != "persisted" {
		t.Errorf("second instance got %+v, want [{Text:persisted ...}]", got)
	}
}

// ---- Empty store ----

func TestStore_Query_EmptyStore(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	got, err := s.Query(context.Background(), run.Filter{})
	if err != nil {
		t.Fatalf("Query() on empty store: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}
