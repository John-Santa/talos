package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/adapter/jsonlstore"
	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/service"
)

var fixedAt = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

// wireTestApp builds the full app wired against a temp dir store.
func wireTestApp(t *testing.T) (*app, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := jsonlstore.New(dir)
	if err != nil {
		t.Fatalf("jsonlstore.New: %v", err)
	}
	a := &app{
		recorder: service.NewRecorder(store),
		querier:  service.NewQuerier(store),
		store:    store,
	}
	return a, dir
}

// ---- record ----

func TestCmd_Record_ActivityEvent(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	var out bytes.Buffer

	args := []string{
		"record",
		"--kind", "activity",
		"--jira-key", "TAL-1",
		"--agent", "IRIS",
		"--text", "applied patch",
		"--at", fixedAt.Format(time.RFC3339),
	}
	if err := run2(context.Background(), args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("run() error: %v", err)
	}
}

func TestCmd_Record_InvalidKind_ReturnsError(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	var out bytes.Buffer

	args := []string{"record", "--kind", "bogus", "--jira-key", "TAL-1"}
	err := run2(context.Background(), args, a, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

// ---- timeline --json ----

func TestCmd_Timeline_JSON(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	// Seed an activity event
	e := run.RunEvent{V: 1, Kind: run.KindActivity, At: fixedAt, JiraKey: "TAL-1", Text: "hello"}
	_ = a.recorder.Record(ctx, e)

	var out bytes.Buffer
	args := []string{"timeline", "--jira-key", "TAL-1", "--json"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("timeline --json error: %v", err)
	}

	var entries []run.ActivityEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("json.Unmarshal timeline output: %v\nraw: %s", err, out.String())
	}
	if len(entries) != 1 || entries[0].Text != "hello" {
		t.Errorf("got %+v, want [{Text:hello ...}]", entries)
	}
}

// ---- judgment --json ----

func TestCmd_Judgment_JSON_NoPendingRecord(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	var out bytes.Buffer
	args := []string{"judgment", "--jira-key", "TAL-99", "--json"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("judgment --json error: %v", err)
	}

	var review run.JudgmentReview
	if err := json.Unmarshal(out.Bytes(), &review); err != nil {
		t.Fatalf("json.Unmarshal judgment output: %v\nraw: %s", err, out.String())
	}
	if !review.Pending {
		t.Error("expected Pending=true when no judgment events")
	}
}

func TestCmd_Judgment_JSON_WithRecord(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	e := run.RunEvent{
		V: 1, Kind: run.KindJudgment, At: fixedAt, JiraKey: "TAL-1",
		Verdict: "APPROVED", Judges: []string{"ARGOS"},
	}
	_ = a.recorder.Record(ctx, e)

	var out bytes.Buffer
	args := []string{"judgment", "--jira-key", "TAL-1", "--json"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("judgment --json error: %v", err)
	}

	var review run.JudgmentReview
	if err := json.Unmarshal(out.Bytes(), &review); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw: %s", err, out.String())
	}
	if review.Verdict != "APPROVED" {
		t.Errorf("Verdict=%q, want APPROVED", review.Verdict)
	}
}

// ---- dod --json ----

func TestCmd_DoD_JSON(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	e := run.RunEvent{
		V: 1, Kind: run.KindMetric, At: fixedAt, JiraKey: "TAL-1",
		Metric: "dod", Label: "PR linked", DoDState: "done",
	}
	_ = a.recorder.Record(ctx, e)

	var out bytes.Buffer
	args := []string{"dod", "--jira-key", "TAL-1", "--json"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("dod --json error: %v", err)
	}

	var items []run.DoDItem
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw: %s", err, out.String())
	}
	if len(items) != 1 || items[0].Label != "PR linked" {
		t.Errorf("got %+v, want [{Label:PR linked ...}]", items)
	}
}

// ---- list --json ----

func TestCmd_List_JSON(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	e := run.RunEvent{V: 1, Kind: run.KindDispatch, At: fixedAt, JiraKey: "TAL-1", Agent: "IRIS", Status: "done"}
	_ = a.recorder.Record(ctx, e)

	var out bytes.Buffer
	args := []string{"list", "--jira-key", "TAL-1", "--json"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("list --json error: %v", err)
	}

	var views []run.RunView
	if err := json.Unmarshal(out.Bytes(), &views); err != nil {
		t.Fatalf("json.Unmarshal: %v\nraw: %s", err, out.String())
	}
	if len(views) != 1 || views[0].JiraKey != "TAL-1" {
		t.Errorf("got %+v, want [{JiraKey:TAL-1 ...}]", views)
	}
}

// ---- show (human-readable) ----

func TestCmd_Show_NonJSON(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	ctx := context.Background()

	e := run.RunEvent{V: 1, Kind: run.KindDispatch, At: fixedAt, JiraKey: "TAL-1", Agent: "IRIS", Status: "done"}
	_ = a.recorder.Record(ctx, e)

	var out bytes.Buffer
	args := []string{"show", "TAL-1"}
	if err := run2(ctx, args, a, strings.NewReader(""), &out); err != nil {
		t.Fatalf("show error: %v", err)
	}
	if !strings.Contains(out.String(), "TAL-1") {
		t.Errorf("show output missing 'TAL-1': %q", out.String())
	}
}

// ---- unknown subcommand ----

func TestCmd_UnknownSubcommand_ReturnsError(t *testing.T) {
	t.Parallel()
	a, _ := wireTestApp(t)
	err := run2(context.Background(), []string{"bogus"}, a, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}
