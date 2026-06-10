package mock_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/console/mock"
)

func TestPlatformActorMock_TeardownWorktree_RecordsCall(t *testing.T) {
	m := mock.NewPlatformActorMock()

	if err := m.TeardownWorktree(context.Background(), "iris", "TAL-18"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m.AssertCallCount(t, "TeardownWorktree", 1)
	calls := m.CallsFor("TeardownWorktree")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Args[0] != "iris" {
		t.Errorf("Args[0] = %v, want iris", calls[0].Args[0])
	}
	if calls[0].Args[1] != "TAL-18" {
		t.Errorf("Args[1] = %v, want TAL-18", calls[0].Args[1])
	}
}

func TestPlatformActorMock_TeardownWorktree_ReturnsProgrammedError(t *testing.T) {
	m := mock.NewPlatformActorMock()
	want := errors.New("teardown failed")
	m.TeardownErr = want

	err := m.TeardownWorktree(context.Background(), "hermes", "TAL-5")
	if !errors.Is(err, want) {
		t.Errorf("want errors.Is(err, want); got %v", err)
	}

	m.AssertCallCount(t, "TeardownWorktree", 1)
}

func TestPlatformActorMock_AssertNotCalled(t *testing.T) {
	m := mock.NewPlatformActorMock()
	// Should not fail — method was never called.
	m.AssertNotCalled(t, "TeardownWorktree")
}
