package run_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/domain/run"
)

// ---- helpers ----

func makeActivity(jiraKey, agent, text string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindActivity, At: at, JiraKey: jiraKey, Agent: agent, Text: text}
}

func makeJudgment(jiraKey, verdict, fixAgent string, judges []string, at time.Time) run.RunEvent {
	return run.RunEvent{
		V: 1, Kind: run.KindJudgment, At: at, JiraKey: jiraKey,
		Verdict: verdict, Judges: judges, FixAgent: fixAgent,
	}
}

func makeDoDMetric(jiraKey, label, state string, at time.Time) run.RunEvent {
	return run.RunEvent{
		V: 1, Kind: run.KindMetric, At: at, JiraKey: jiraKey,
		Metric: "dod", Label: label, DoDState: state,
	}
}

func makeDispatch(jiraKey, agent, phase, status string, at time.Time) run.RunEvent {
	return run.RunEvent{
		V: 1, Kind: run.KindDispatch, At: at, JiraKey: jiraKey,
		Agent: agent, Phase: run.Phase(phase), Status: status,
	}
}

// ---- ProjectActivity ----

func TestProjectActivity_OrderedByAt(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	evs := []run.RunEvent{
		makeActivity("TAL-1", "IRIS", "second", t2),
		makeActivity("TAL-1", "IRIS", "first", t1),
	}
	got := run.ProjectActivity(evs)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].Text != "first" {
		t.Errorf("got[0].Text=%q, want %q", got[0].Text, "first")
	}
	if got[1].Text != "second" {
		t.Errorf("got[1].Text=%q, want %q", got[1].Text, "second")
	}
}

func TestProjectActivity_FiltersNonActivity(t *testing.T) {
	t.Parallel()
	evs := []run.RunEvent{
		makeActivity("TAL-1", "IRIS", "act", fixedAt),
		makeDispatch("TAL-1", "IRIS", "apply", "done", fixedAt),
	}
	got := run.ProjectActivity(evs)
	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
}

func TestProjectActivity_AtISORFC3339(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 6, 10, 15, 30, 0, 0, time.UTC)
	evs := []run.RunEvent{makeActivity("TAL-1", "", "text", at)}
	got := run.ProjectActivity(evs)
	want := "2026-06-10T15:30:00Z"
	if got[0].At != want {
		t.Errorf("At=%q, want %q", got[0].At, want)
	}
}

func TestProjectActivity_EmptyInput(t *testing.T) {
	t.Parallel()
	got := run.ProjectActivity(nil)
	if got == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("len=%d, want 0", len(got))
	}
}

// ---- ActivityEntry json tags match webapi/domain ----

func TestActivityEntry_JsonTags(t *testing.T) {
	t.Parallel()
	// The json tags in runs must exactly match webapi/domain.ActivityEntry:
	//   At   string `json:"at"`
	//   Text string `json:"text"`
	entry := run.ActivityEntry{At: "2026-06-10T00:00:00Z", Text: "hello"}
	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if m["at"] != "2026-06-10T00:00:00Z" {
		t.Errorf("expected json key 'at', got keys %v", m)
	}
	if m["text"] != "hello" {
		t.Errorf("expected json key 'text', got keys %v", m)
	}
}

// ---- DoDItem json tags match webapi/domain ----

func TestDoDItem_JsonTags(t *testing.T) {
	t.Parallel()
	// webapi/domain.DoDItem: Label string `json:"label"`, State string `json:"state"`, Kind string `json:"kind"`
	item := run.DoDItem{Label: "PR linked", State: "done", Kind: "pr"}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, k := range []string{"label", "state", "kind"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q in DoDItem output: %s", k, string(b))
		}
	}
}

// ---- JudgmentReview json tags match webapi/domain ----

func TestJudgmentReview_JsonTags(t *testing.T) {
	t.Parallel()
	// webapi/domain.JudgmentReview: jiraKey, gate, judges, fixAgent, verdict, escalateTo, pending
	review := run.JudgmentReview{
		JiraKey:  "TAL-1",
		Gate:     "HG5",
		Judges:   []run.Judge{{ID: "ARGOS", Verdict: "APPROVED", Note: ""}},
		FixAgent: "IRIS",
		Verdict:  "APPROVED",
		Pending:  false,
	}
	b, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, k := range []string{"jiraKey", "gate", "judges", "fixAgent", "verdict"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q in JudgmentReview output: %s", k, string(b))
		}
	}
	// pending omitempty when false
	if _, ok := m["pending"]; ok {
		t.Errorf("pending should be omitted when false, but got key in JSON: %s", string(b))
	}
}

// ---- Judge json tags match webapi/domain ----

func TestJudge_JsonTags(t *testing.T) {
	t.Parallel()
	// webapi/domain.Judge: ID string `json:"id"`, Verdict string `json:"verdict"`, Note string `json:"note"`
	j := run.Judge{ID: "ARGOS", Verdict: "APPROVED", Note: "looks good"}
	b, _ := json.Marshal(j)
	var m map[string]string
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"id", "verdict", "note"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q in Judge output: %s", k, string(b))
		}
	}
}

// ---- ProjectDoD ----

func TestProjectDoD_LastWinsByLabel(t *testing.T) {
	t.Parallel()
	t1 := fixedAt
	t2 := t1.Add(time.Hour)
	evs := []run.RunEvent{
		makeDoDMetric("TAL-1", "PR linked", "pending", t1),
		makeDoDMetric("TAL-1", "PR linked", "done", t2), // last-wins
		makeDoDMetric("TAL-1", "CI green", "done", t1),
	}
	got := run.ProjectDoD(evs)
	// should have 2 unique labels
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2; got %+v", len(got), got)
	}
	byLabel := make(map[string]run.DoDItem)
	for _, d := range got {
		byLabel[d.Label] = d
	}
	if byLabel["PR linked"].State != "done" {
		t.Errorf("PR linked state=%q, want done", byLabel["PR linked"].State)
	}
}

func TestProjectDoD_EmptyInput(t *testing.T) {
	t.Parallel()
	got := run.ProjectDoD(nil)
	if got == nil {
		t.Error("expected non-nil slice")
	}
}

// ---- ProjectJudgment ----

func TestProjectJudgment_LastByAt(t *testing.T) {
	t.Parallel()
	t1 := fixedAt
	t2 := t1.Add(time.Hour)
	evs := []run.RunEvent{
		makeJudgment("TAL-1", "REJECTED", "IRIS", []string{"ARGOS"}, t1),
		makeJudgment("TAL-1", "APPROVED", "HEPH", []string{"ARGOS", "ZEUS"}, t2),
	}
	got := run.ProjectJudgment("TAL-1", evs)
	if got.Verdict != "APPROVED" {
		t.Errorf("Verdict=%q, want APPROVED", got.Verdict)
	}
	if got.JiraKey != "TAL-1" {
		t.Errorf("JiraKey=%q, want TAL-1", got.JiraKey)
	}
}

func TestProjectJudgment_NoEvents_ReturnsPending(t *testing.T) {
	t.Parallel()
	got := run.ProjectJudgment("TAL-99", nil)
	if !got.Pending {
		t.Errorf("expected Pending=true when no events")
	}
}

func TestProjectJudgment_JudgesPopulated(t *testing.T) {
	t.Parallel()
	evs := []run.RunEvent{
		makeJudgment("TAL-1", "APPROVED", "IRIS", []string{"ARGOS", "ZEUS"}, fixedAt),
	}
	got := run.ProjectJudgment("TAL-1", evs)
	if len(got.Judges) != 2 {
		t.Fatalf("len(Judges)=%d, want 2", len(got.Judges))
	}
}

// ---- ProjectRuns ----

func TestProjectRuns_FilterByJiraKey(t *testing.T) {
	t.Parallel()
	evs := []run.RunEvent{
		makeDispatch("TAL-1", "IRIS", "apply", "done", fixedAt),
		makeDispatch("TAL-2", "HEPH", "spec", "started", fixedAt),
	}
	f := run.Filter{JiraKey: "TAL-1"}
	got := run.ProjectRuns(evs, f)
	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
	if got[0].JiraKey != "TAL-1" {
		t.Errorf("JiraKey=%q, want TAL-1", got[0].JiraKey)
	}
}

func TestProjectRuns_FilterByAgent(t *testing.T) {
	t.Parallel()
	evs := []run.RunEvent{
		makeDispatch("TAL-1", "IRIS", "apply", "done", fixedAt),
		makeDispatch("TAL-2", "HEPH", "spec", "done", fixedAt),
	}
	f := run.Filter{Agent: "IRIS"}
	got := run.ProjectRuns(evs, f)
	if len(got) != 1 || got[0].Agent != "IRIS" {
		t.Errorf("got %+v, want [{Agent:IRIS ...}]", got)
	}
}

func TestProjectRuns_EmptyInput(t *testing.T) {
	t.Parallel()
	got := run.ProjectRuns(nil, run.Filter{})
	if got == nil {
		t.Error("expected non-nil slice")
	}
}

// ---- golden JSON round-trip: ActivityEntry slice ----

func TestProjectActivity_GoldenJSON(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	evs := []run.RunEvent{makeActivity("TAL-1", "IRIS", "applied patch", at)}
	got := run.ProjectActivity(evs)

	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	// Deserialize back and check
	var decoded []run.ActivityEntry
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, decoded) {
		t.Errorf("round-trip mismatch:\ngot    %+v\ndecoded %+v", got, decoded)
	}
}
