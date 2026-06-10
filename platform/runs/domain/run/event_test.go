package run_test

import (
	"errors"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/domain/run"
)

var fixedAt = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func baseEvent(kind run.Kind) run.RunEvent {
	return run.RunEvent{
		V:       1,
		Kind:    kind,
		At:      fixedAt,
		JiraKey: "TAL-42",
	}
}

func TestValidate_ValidKinds(t *testing.T) {
	t.Parallel()
	kinds := []run.Kind{run.KindDispatch, run.KindActivity, run.KindJudgment, run.KindMetric}
	for _, k := range kinds {
		k := k
		t.Run(string(k), func(t *testing.T) {
			t.Parallel()
			e := baseEvent(k)
			if err := e.Validate(); err != nil {
				t.Errorf("Validate(%q) unexpected error: %v", k, err)
			}
		})
	}
}

func TestValidate_InvalidKind(t *testing.T) {
	t.Parallel()
	e := baseEvent("bogus")
	err := e.Validate()
	var target *run.ErrInvalidKind
	if !errors.As(err, &target) {
		t.Errorf("expected ErrInvalidKind, got %T: %v", err, err)
	}
}

func TestValidate_MissingAt(t *testing.T) {
	t.Parallel()
	e := baseEvent(run.KindActivity)
	e.At = time.Time{} // zero
	err := e.Validate()
	var target *run.ErrInvalidEvent
	if !errors.As(err, &target) {
		t.Errorf("expected ErrInvalidEvent, got %T: %v", err, err)
	}
}

func TestValidate_MissingJiraKey_NonMetric(t *testing.T) {
	t.Parallel()
	for _, k := range []run.Kind{run.KindDispatch, run.KindActivity, run.KindJudgment} {
		k := k
		t.Run(string(k), func(t *testing.T) {
			t.Parallel()
			e := baseEvent(k)
			e.JiraKey = ""
			err := e.Validate()
			var target *run.ErrMissingJiraKey
			if !errors.As(err, &target) {
				t.Errorf("kind=%q: expected ErrMissingJiraKey, got %T: %v", k, err, err)
			}
		})
	}
}

func TestValidate_MetricAllowsEmptyJiraKey(t *testing.T) {
	t.Parallel()
	e := baseEvent(run.KindMetric)
	e.JiraKey = ""
	if err := e.Validate(); err != nil {
		t.Errorf("metric with empty jiraKey: unexpected error %v", err)
	}
}
