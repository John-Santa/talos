package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/mock"
	"github.com/John-Santa/talos/platform/runs/service"
)

func makeActivity(jiraKey, text string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindActivity, At: at, JiraKey: jiraKey, Text: text}
}

func makeJudgment(jiraKey, verdict string, judges []string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindJudgment, At: at, JiraKey: jiraKey, Verdict: verdict, Judges: judges}
}

func makeDoDMetric(jiraKey, label, state string, at time.Time) run.RunEvent {
	return run.RunEvent{
		V: 1, Kind: run.KindMetric, At: at, JiraKey: jiraKey,
		Metric: "dod", Label: label, DoDState: state,
	}
}

func makeDispatch(jiraKey, agent, status string, at time.Time) run.RunEvent {
	return run.RunEvent{V: 1, Kind: run.KindDispatch, At: at, JiraKey: jiraKey, Agent: agent, Status: status}
}

func TestQuerier_Timeline_ReturnsActivityEntries(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{
		Events: []run.RunEvent{
			makeActivity("TAL-1", "step A", fixedAt),
			makeActivity("TAL-1", "step B", fixedAt.Add(time.Minute)),
		},
	}
	q := service.NewQuerier(st)
	got, err := q.Timeline(context.Background(), "TAL-1", "")
	if err != nil {
		t.Fatalf("Timeline() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
}

func TestQuerier_Timeline_QueryCallsStore(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{}
	q := service.NewQuerier(st)
	_, _ = q.Timeline(context.Background(), "TAL-1", "")
	if len(st.CallsFor("Query")) != 1 {
		t.Error("expected 1 Query call")
	}
}

func TestQuerier_DoD_ReturnsDoDItems(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{
		Events: []run.RunEvent{
			makeDoDMetric("TAL-1", "PR linked", "done", fixedAt),
			makeDoDMetric("TAL-1", "CI green", "pending", fixedAt),
		},
	}
	q := service.NewQuerier(st)
	got, err := q.DoD(context.Background(), "TAL-1")
	if err != nil {
		t.Fatalf("DoD() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
}

func TestQuerier_Judgment_ReturnsReview(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{
		Events: []run.RunEvent{
			makeJudgment("TAL-1", "APPROVED", []string{"ARGOS"}, fixedAt),
		},
	}
	q := service.NewQuerier(st)
	got, err := q.Judgment(context.Background(), "TAL-1")
	if err != nil {
		t.Fatalf("Judgment() error: %v", err)
	}
	if got.Verdict != "APPROVED" {
		t.Errorf("Verdict=%q, want APPROVED", got.Verdict)
	}
	if got.Pending {
		t.Error("Pending should be false when a judgment event exists")
	}
}

func TestQuerier_Judgment_NoEvents_ReturnsPending(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{}
	q := service.NewQuerier(st)
	got, err := q.Judgment(context.Background(), "TAL-99")
	if err != nil {
		t.Fatalf("Judgment() error: %v", err)
	}
	if !got.Pending {
		t.Error("Pending should be true when no judgment events")
	}
}

func TestQuerier_Runs_FiltersByJiraKey(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{
		Events: []run.RunEvent{
			makeDispatch("TAL-1", "IRIS", "done", fixedAt),
			makeDispatch("TAL-2", "HEPH", "done", fixedAt),
		},
	}
	q := service.NewQuerier(st)
	got, err := q.Runs(context.Background(), run.Filter{JiraKey: "TAL-1"})
	if err != nil {
		t.Fatalf("Runs() error: %v", err)
	}
	if len(got) != 1 || got[0].JiraKey != "TAL-1" {
		t.Errorf("got %+v, want [{JiraKey:TAL-1 ...}]", got)
	}
}
