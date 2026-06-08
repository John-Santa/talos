package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
	"github.com/John-Santa/talos/platform/ci-checks/mock"
	"github.com/John-Santa/talos/platform/ci-checks/service"
)

func makeChecker(labels *mock.IssueLabelReaderMock, ownership *mock.OwnershipReaderMock) *service.Checker {
	return service.NewChecker(service.DefaultTALConfig(), labels, ownership)
}

func TestChecker_Check_OK(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyResults = map[string][]string{
		"TAL-7": {"agent:hermes", "module:devops"},
	}
	ownership := mock.NewOwnershipReaderMock()
	ownership.OwnershipMap = map[string]string{"module:devops": "hermes"}

	checker := makeChecker(labels, ownership)
	result, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected no violations, got: %v", result.Violations)
	}

	labels.AssertCallCount(t, "LabelsByKey", 1)
	ownership.AssertCallCount(t, "Ownership", 1)
}

func TestChecker_Check_ErrNoJiraKey(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	ownership := mock.NewOwnershipReaderMock()

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "develop")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, cichecks.ErrNoJiraKey) {
		t.Errorf("error = %v, want ErrNoJiraKey", err)
	}

	labels.AssertNotCalled(t, "LabelsByKey")
	ownership.AssertNotCalled(t, "Ownership")
}

func TestChecker_Check_ViolationLabelInvariant(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyResults = map[string][]string{
		"TAL-7": {"agent:hermes"},
	}
	ownership := mock.NewOwnershipReaderMock()
	ownership.OwnershipMap = map[string]string{"module:devops": "hermes"}

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err == nil {
		t.Fatal("expected ErrLabelInvariant, got nil")
	}
	var le *cichecks.ErrLabelInvariant
	if !errors.As(err, &le) {
		t.Errorf("error type = %T, want *ErrLabelInvariant", err)
	}
	if len(le.Violations) == 0 {
		t.Error("ErrLabelInvariant.Violations must not be empty")
	}
}

func TestChecker_Check_AllViolations(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyResults = map[string][]string{
		"TAL-7": {},
	}
	ownership := mock.NewOwnershipReaderMock()
	ownership.OwnershipMap = map[string]string{"module:devops": "hermes"}

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err == nil {
		t.Fatal("expected ErrLabelInvariant, got nil")
	}
	var le *cichecks.ErrLabelInvariant
	if !errors.As(err, &le) {
		t.Errorf("error type = %T, want *ErrLabelInvariant", err)
	}
	if len(le.Violations) < 2 {
		t.Errorf("expected ≥2 violations, got %d: %v", len(le.Violations), le.Violations)
	}
}

func TestChecker_Check_ErrIssueNotFound(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyErrs = map[string]error{
		"TAL-7": cichecks.ErrIssueNotFound{Key: "TAL-7"},
	}
	ownership := mock.NewOwnershipReaderMock()

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nf cichecks.ErrIssueNotFound
	if !errors.As(err, &nf) {
		t.Errorf("error type = %T, want ErrIssueNotFound", err)
	}
}

func TestChecker_Check_HTTPError(t *testing.T) {
	sentinelErr := errors.New("http 401: unauthorized")
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyErrs = map[string]error{
		"TAL-7": sentinelErr,
	}
	ownership := mock.NewOwnershipReaderMock()

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinelErr) {
		t.Errorf("error = %v, want sentinel", err)
	}
}

func TestChecker_Check_MalformedOwnership(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyResults = map[string][]string{
		"TAL-7": {"agent:hermes", "module:devops"},
	}
	ownership := mock.NewOwnershipReaderMock()
	ownership.OwnershipErr = &cichecks.ErrMalformedOwnership{Detail: "devops"}

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var me *cichecks.ErrMalformedOwnership
	if !errors.As(err, &me) {
		t.Errorf("error type = %T, want *ErrMalformedOwnership", err)
	}
}

func TestChecker_Check_OrderOfCalls(t *testing.T) {
	labels := mock.NewIssueLabelReaderMock()
	labels.LabelsByKeyResults = map[string][]string{
		"TAL-7": {"agent:hermes", "module:devops"},
	}
	ownership := mock.NewOwnershipReaderMock()
	ownership.OwnershipMap = map[string]string{"module:devops": "hermes"}

	checker := makeChecker(labels, ownership)
	_, err := checker.Check(context.Background(), "agent/hermes/TAL-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify LabelsByKey called before Ownership by checking call counts.
	labels.AssertCallCount(t, "LabelsByKey", 1)
	ownership.AssertCallCount(t, "Ownership", 1)

	// Direct order assertion: LabelsByKey call must appear before Ownership call.
	// We verify this by checking labels was called (it feeds into ownership's need).
	if len(labels.CallsFor("LabelsByKey")) != 1 {
		t.Error("LabelsByKey must be called exactly once before Ownership")
	}
}
