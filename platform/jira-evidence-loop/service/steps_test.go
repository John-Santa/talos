package service_test

import (
	"context"
	"testing"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/mock"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// ---------------------------------------------------------------------------
// AllSteps
// ---------------------------------------------------------------------------

func TestAllSteps_ContainsAll(t *testing.T) {
	all := service.AllSteps()
	allConstants := []service.Step{
		service.StepOwnership,
		service.StepCreate,
		service.StepTransitionInProgress,
		service.StepComment,
		service.StepWorklog,
		service.StepRemoteLink,
		service.StepAttach,
		service.StepTransitionDone,
	}
	for _, s := range allConstants {
		if !all[s] {
			t.Errorf("AllSteps() missing step %d", s)
		}
	}
	if len(all) != len(allConstants) {
		t.Errorf("AllSteps() len = %d, want %d", len(all), len(allConstants))
	}
}

// ---------------------------------------------------------------------------
// PhasePreset — Design §D3 table
// ---------------------------------------------------------------------------

type phasePresetCase struct {
	phase       string
	mustHave    []service.Step
	mustNotHave []service.Step
}

func TestPhasePreset_Table(t *testing.T) {
	tests := []phasePresetCase{
		{
			// propose: 0,1,2,3 → ownership, create, →InProgress, comment
			phase: "propose",
			mustHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepComment,
			},
			mustNotHave: []service.Step{
				service.StepWorklog,
				service.StepRemoteLink,
				service.StepAttach,
				service.StepTransitionDone,
			},
		},
		{
			// spec: 3 (comment only)
			phase: "spec",
			mustHave: []service.Step{
				service.StepComment,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepWorklog,
				service.StepRemoteLink,
				service.StepAttach,
				service.StepTransitionDone,
			},
		},
		{
			// design: 3 (comment only, same as spec)
			phase: "design",
			mustHave: []service.Step{
				service.StepComment,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepWorklog,
				service.StepRemoteLink,
				service.StepAttach,
				service.StepTransitionDone,
			},
		},
		{
			// tasks: 3,4 (comment + worklog)
			phase: "tasks",
			mustHave: []service.Step{
				service.StepComment,
				service.StepWorklog,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepRemoteLink,
				service.StepAttach,
				service.StepTransitionDone,
			},
		},
		{
			// apply: 3,4 (comment + worklog)
			phase: "apply",
			mustHave: []service.Step{
				service.StepComment,
				service.StepWorklog,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepRemoteLink,
				service.StepAttach,
				service.StepTransitionDone,
			},
		},
		{
			// verify: 3,5,6 (comment, remote-link, attach)
			phase: "verify",
			mustHave: []service.Step{
				service.StepComment,
				service.StepRemoteLink,
				service.StepAttach,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepWorklog,
				service.StepTransitionDone,
			},
		},
		{
			// archive: 7 (→Done only)
			phase: "archive",
			mustHave: []service.Step{
				service.StepTransitionDone,
			},
			mustNotHave: []service.Step{
				service.StepOwnership,
				service.StepCreate,
				service.StepTransitionInProgress,
				service.StepComment,
				service.StepWorklog,
				service.StepRemoteLink,
				service.StepAttach,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.phase, func(t *testing.T) {
			ss, err := service.PhasePreset(tc.phase)
			if err != nil {
				t.Fatalf("PhasePreset(%q) error = %v, want nil", tc.phase, err)
			}
			for _, s := range tc.mustHave {
				if !ss[s] {
					t.Errorf("PhasePreset(%q): step %d should be present", tc.phase, s)
				}
			}
			for _, s := range tc.mustNotHave {
				if ss[s] {
					t.Errorf("PhasePreset(%q): step %d should be absent", tc.phase, s)
				}
			}
		})
	}
}

func TestPhasePreset_UnknownPhase_Error(t *testing.T) {
	_, err := service.PhasePreset("unknownphase")
	if err == nil {
		t.Error("PhasePreset(unknownphase) expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// RunSteps — step-skip behaviour
//
// These tests exercise RunSteps with each phase preset and assert that only
// the expected Jira API methods are called. They reuse the helpers from
// evidence_loop_test.go (validConfig, validInput, transitionsForCategories).
// ---------------------------------------------------------------------------

// newStepsMock returns a mock pre-wired for RunSteps tests involving the
// propose preset (which includes search+create+transitions).
func newStepsMock() *mock.JiraClientMock {
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-42"}
	m.GetTransitionsResult = transitionsForCategories()
	return m
}

func TestRunSteps_ProposePreset_OnlyExpectedCalls(t *testing.T) {
	m := newStepsMock()
	preset, err := service.PhasePreset("propose")
	if err != nil {
		t.Fatalf("PhasePreset: %v", err)
	}

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.RunSteps(context.Background(), validInput(), preset)
	if err != nil {
		t.Fatalf("RunSteps(propose) error = %v", err)
	}
	if key == "" {
		t.Error("RunSteps(propose) returned empty key")
	}

	// propose: search+create+InProgress+comment — no worklog, remote-link, attach, done.
	m.AssertCallCount(t, "Search", 1)
	m.AssertCallCount(t, "CreateIssue", 1)
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "AddWorklog", 0)
	m.AssertCallCount(t, "CreateRemoteLink", 0)
	m.AssertCallCount(t, "AddAttachment", 0)
	// InProgress transition fires once; Done does NOT fire.
	assertTransitionCategoryCount(t, m, "indeterminate", 1)
	assertTransitionCategoryCount(t, m, "done", 0)
}

func TestRunSteps_SpecPreset_OnlyComment(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("spec")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-77"

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(spec) error = %v", err)
	}

	m.AssertCallCount(t, "Search", 0)
	m.AssertCallCount(t, "CreateIssue", 0)
	m.AssertCallCount(t, "GetTransitions", 0)
	m.AssertCallCount(t, "DoTransition", 0)
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "AddWorklog", 0)
	m.AssertCallCount(t, "CreateRemoteLink", 0)
	m.AssertCallCount(t, "AddAttachment", 0)
}

func TestRunSteps_DesignPreset_OnlyComment(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("design")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-88"

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(design) error = %v", err)
	}
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "Search", 0)
}

func TestRunSteps_TasksPreset_CommentAndWorklog(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("tasks")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-99"

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(tasks) error = %v", err)
	}
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "AddWorklog", 1)
	m.AssertCallCount(t, "Search", 0)
	m.AssertCallCount(t, "CreateRemoteLink", 0)
	m.AssertCallCount(t, "AddAttachment", 0)
}

func TestRunSteps_ApplyPreset_CommentAndWorklog(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("apply")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-100"

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(apply) error = %v", err)
	}
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "AddWorklog", 1)
	m.AssertCallCount(t, "Search", 0)
}

func TestRunSteps_VerifyPreset_CommentRemoteLinkAttach(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("verify")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-55"

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(verify) error = %v", err)
	}
	m.AssertCallCount(t, "AddComment", 1)
	m.AssertCallCount(t, "CreateRemoteLink", 1)
	m.AssertCallCount(t, "AddAttachment", 1)
	m.AssertCallCount(t, "AddWorklog", 0)
	m.AssertCallCount(t, "Search", 0)
	m.AssertCallCount(t, "GetTransitions", 0)
	m.AssertCallCount(t, "DoTransition", 0)
}

func TestRunSteps_ArchivePreset_OnlyDoneTransition(t *testing.T) {
	m := mock.NewJiraClientMock()
	m.GetTransitionsResult = transitionsForCategories()
	preset, _ := service.PhasePreset("archive")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "TAL-42"

	key, err := loop.RunSteps(context.Background(), in, preset)
	if err != nil {
		t.Fatalf("RunSteps(archive) error = %v", err)
	}
	if key != "TAL-42" {
		t.Errorf("RunSteps(archive) key = %q, want \"TAL-42\"", key)
	}
	m.AssertCallCount(t, "Search", 0)
	m.AssertCallCount(t, "CreateIssue", 0)
	m.AssertCallCount(t, "AddComment", 0)
	m.AssertCallCount(t, "AddWorklog", 0)
	m.AssertCallCount(t, "CreateRemoteLink", 0)
	m.AssertCallCount(t, "AddAttachment", 0)
	m.AssertCallCount(t, "GetTransitions", 1)
	m.AssertCallCount(t, "DoTransition", 1)
}

func TestRunSteps_AllSteps_BackCompat(t *testing.T) {
	// RunSteps(ctx, in, AllSteps()) must produce an identical call sequence to Run(ctx, in).
	m := mock.NewJiraClientMock()
	m.SearchResults = nil
	m.CreateIssueResult = evidence.Issue{Key: "TAL-42"}
	m.GetTransitionsResult = transitionsForCategories()

	loop := service.NewEvidenceLoop(m, validConfig())
	key, err := loop.RunSteps(context.Background(), validInput(), service.AllSteps())
	if err != nil {
		t.Fatalf("RunSteps(AllSteps) error = %v", err)
	}
	if key != "TAL-42" {
		t.Errorf("RunSteps(AllSteps) key = %q, want \"TAL-42\"", key)
	}

	wantOrder := []string{
		"Search",
		"CreateIssue",
		"GetTransitions",
		"DoTransition",
		"AddComment",
		"AddWorklog",
		"CreateRemoteLink",
		"AddAttachment",
		"GetTransitions",
		"DoTransition",
	}
	m.AssertMethodOrder(t, wantOrder)
}

func TestRunSteps_ArchivePreset_MissingJiraKey_Error(t *testing.T) {
	m := mock.NewJiraClientMock()
	preset, _ := service.PhasePreset("archive")
	loop := service.NewEvidenceLoop(m, validConfig())

	in := validInput()
	in.JiraKey = "" // no key — must fail loud

	_, err := loop.RunSteps(context.Background(), in, preset)
	if err == nil {
		t.Fatal("RunSteps(archive) without JiraKey expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Test helpers local to steps_test.go
// ---------------------------------------------------------------------------

// assertTransitionCategoryCount verifies how many DoTransition calls targeted
// the given status category, using the mock's programmed GetTransitions data
// to map transition IDs → categories.
func assertTransitionCategoryCount(
	t *testing.T,
	m *mock.JiraClientMock,
	category string,
	wantCount int,
) {
	t.Helper()
	catByID := make(map[string]string)
	for _, tr := range m.GetTransitionsResult {
		catByID[tr.ID] = tr.ToCategory
	}
	got := 0
	for _, call := range m.CallsFor("DoTransition") {
		if len(call.Args) < 2 {
			continue
		}
		id, ok := call.Args[1].(string)
		if !ok {
			continue
		}
		if catByID[id] == category {
			got++
		}
	}
	if got != wantCount {
		t.Errorf("DoTransition to category %q: count = %d, want %d", category, got, wantCount)
	}
}
