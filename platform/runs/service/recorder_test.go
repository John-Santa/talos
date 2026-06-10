package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/mock"
	"github.com/John-Santa/talos/platform/runs/service"
)

var fixedAt = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

func validEvent(kind run.Kind) run.RunEvent {
	return run.RunEvent{V: 1, Kind: kind, At: fixedAt, JiraKey: "TAL-1"}
}

func TestRecorder_Record_ValidEvent_Appended(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{}
	rec := service.NewRecorder(st)

	e := validEvent(run.KindActivity)
	e.Text = "started"
	if err := rec.Record(context.Background(), e); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	appends := st.CallsFor("Append")
	if len(appends) != 1 {
		t.Fatalf("Append calls=%d, want 1", len(appends))
	}
}

func TestRecorder_Record_InvalidEvent_ReturnsError(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{}
	rec := service.NewRecorder(st)

	e := run.RunEvent{V: 1, Kind: "bogus", At: fixedAt, JiraKey: "TAL-1"}
	err := rec.Record(context.Background(), e)
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
	var target *run.ErrInvalidKind
	if !errors.As(err, &target) {
		t.Errorf("expected ErrInvalidKind, got %T: %v", err, err)
	}
	// store must NOT have been called
	if len(st.CallsFor("Append")) != 0 {
		t.Error("Append should not be called when validation fails")
	}
}

func TestRecorder_Record_StoreError_Propagated(t *testing.T) {
	t.Parallel()
	st := &mock.RunStoreMock{AppendErr: mock.ErrSentinel("disk full")}
	rec := service.NewRecorder(st)

	e := validEvent(run.KindDispatch)
	err := rec.Record(context.Background(), e)
	if err == nil {
		t.Fatal("expected error from store, got nil")
	}
}
