package service_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/mock"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/service"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func defaultCfg() service.Config {
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = "/repo"
	return cfg
}

// fakeWriteFile captures the path and data passed to WriteFile.
type fakeWriteFile struct {
	path string
	data []byte
	err  error
}

func (f *fakeWriteFile) write(path string, data []byte, perm fs.FileMode) error {
	f.path = path
	f.data = data
	return f.err
}

// newOrchestrator builds an orchestrator using the mock runner and a fake WriteFile.
func newOrchestrator(runner *mock.GitRunnerMock, wf *fakeWriteFile) *service.Orchestrator {
	o := service.NewOrchestrator(runner, defaultCfg())
	o.WriteFile = wf.write
	return o
}

// ---------------------------------------------------------------------------
// Create — happy path
// ---------------------------------------------------------------------------

func TestCreate_HappyPath(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = "" // no existing worktrees
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	ctx := context.Background()
	err := o.Create(ctx, "hermes", "TAL-2", false)
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	// Call order: BranchExists → WorktreeList → Fetch → WorktreeAdd
	runner.AssertMethodOrder(t, "BranchExists", "WorktreeList", "Fetch", "WorktreeAdd")

	// WriteFile must have been called with the .env content at the correct path
	wantPath := "talos.wt/agent-hermes/.env"
	if wf.path != wantPath {
		t.Errorf("WriteFile path = %q, want %q", wf.path, wantPath)
	}

	// .env content must be byte-identical to RenderEnv output
	spec, _ := worktree.NewWorktreeSpec("hermes", "TAL-2", "talos.wt")
	res, _ := worktree.AgentResources(spec.Figura)
	wantContent := worktree.RenderEnv(spec, res)
	if string(wf.data) != wantContent {
		t.Errorf("WriteFile data =\n%q\nwant:\n%q", string(wf.data), wantContent)
	}
}

// ---------------------------------------------------------------------------
// Create — --no-fetch: Fetch must NOT be called
// ---------------------------------------------------------------------------

func TestCreate_NoFetch(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = ""
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	ctx := context.Background()
	err := o.Create(ctx, "atlas", "TAL-1", true /* noFetch */)
	if err != nil {
		t.Fatalf("Create(noFetch=true) unexpected error: %v", err)
	}

	// Fetch must NOT appear in the call log
	runner.AssertNotCalled(t, "Fetch")
	runner.AssertMethodOrder(t, "BranchExists", "WorktreeList", "WorktreeAdd")
}

// ---------------------------------------------------------------------------
// Create — validation failures stop before any runner call
// ---------------------------------------------------------------------------

func TestCreate_InvalidFigura(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "zeus", "TAL-1", false)
	if err == nil {
		t.Fatal("expected error for invalid figura, got nil")
	}
	var e *worktree.ErrInvalidFigure
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrInvalidFigure", err)
	}
	if len(runner.Calls) != 0 {
		t.Errorf("runner should not have been called on validation failure, got calls: %v", runner.Calls)
	}
}

func TestCreate_InvalidJiraKey(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "atlas", "TAL-0", false)
	if err == nil {
		t.Fatal("expected error for invalid jira key, got nil")
	}
	var e *worktree.ErrInvalidKey
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrInvalidKey", err)
	}
	if len(runner.Calls) != 0 {
		t.Errorf("runner should not have been called on validation failure, got calls: %v", runner.Calls)
	}
}

// ---------------------------------------------------------------------------
// Create — pre-check failures
// ---------------------------------------------------------------------------

func TestCreate_BranchExists(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = true // branch already exists
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "hermes", "TAL-2", false)
	if err == nil {
		t.Fatal("expected ErrBranchExists, got nil")
	}
	var e *worktree.ErrBranchExists
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrBranchExists", err)
	}
	runner.AssertNotCalled(t, "WorktreeAdd")
}

func TestCreate_WorktreeAlreadyExists(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	// Existing worktree at the path that would be created for hermes
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-99\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "hermes", "TAL-2", false)
	if err == nil {
		t.Fatal("expected ErrWorktreeExists, got nil")
	}
	var e *worktree.ErrWorktreeExists
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrWorktreeExists", err)
	}
	runner.AssertNotCalled(t, "WorktreeAdd")
}

// ---------------------------------------------------------------------------
// Create — mid-sequence failure
// ---------------------------------------------------------------------------

func TestCreate_WorktreeAddFailure(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = ""
	runner.WorktreeAddErr = mock.ErrSentinel("git worktree add failed")
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "hermes", "TAL-2", false)
	if err == nil {
		t.Fatal("expected error from WorktreeAdd failure, got nil")
	}
	// WriteFile must NOT have been called
	if wf.path != "" {
		t.Errorf("WriteFile was called with path %q after WorktreeAdd failure", wf.path)
	}
}

// ---------------------------------------------------------------------------
// Create — WriteFile failure: partial state reported
// ---------------------------------------------------------------------------

func TestCreate_WriteFileFailure(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = ""
	wf := &fakeWriteFile{err: mock.ErrSentinel("write .env failed")}
	o := newOrchestrator(runner, wf)

	err := o.Create(context.Background(), "hermes", "TAL-2", false)
	if err == nil {
		t.Fatal("expected error from WriteFile failure, got nil")
	}
	// The error must mention partial state or .env write failure
	if !strings.Contains(err.Error(), ".env") && !strings.Contains(err.Error(), "partial") {
		t.Errorf("error %q should mention .env or partial state", err.Error())
	}
}

// ---------------------------------------------------------------------------
// List — REQ-LIST-4: status taxonomy active/orphan/stale
// ---------------------------------------------------------------------------

// TestList_Active verifies that a worktree whose branch still exists and whose
// directory exists on disk is classified as "active".
func TestList_Active(t *testing.T) {
	t.Parallel()
	// Use a real directory so the disk-existence check passes.
	wtDir := t.TempDir()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree " + wtDir + "\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/atlas/TAL-1\n" +
		"\n"
	// Branch exists → active (not orphan)
	runner.BranchExistsResultsByBranch = map[string]bool{
		"agent/atlas/TAL-1": true,
	}
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	statuses, err := o.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(statuses))
	}
	if statuses[0].Info.Branch != "agent/atlas/TAL-1" {
		t.Errorf("Info.Branch = %q, want %q", statuses[0].Info.Branch, "agent/atlas/TAL-1")
	}
	if statuses[0].Status != "active" {
		t.Errorf("Status = %q, want %q", statuses[0].Status, "active")
	}
}

// TestList_Orphan verifies that a worktree whose directory exists but whose
// branch has been deleted (merged/pruned) is classified as "orphan".
func TestList_Orphan(t *testing.T) {
	t.Parallel()
	// Use a real directory so the disk-existence check passes (not stale).
	wtDir := t.TempDir()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree " + wtDir + "\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/atlas/TAL-1\n" +
		"\n"
	// Branch does NOT exist → orphan (merged/deleted)
	runner.BranchExistsResultsByBranch = map[string]bool{
		"agent/atlas/TAL-1": false,
	}
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	statuses, err := o.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(statuses))
	}
	if statuses[0].Status != "orphan" {
		t.Errorf("Status = %q, want %q", statuses[0].Status, "orphan")
	}
}

// TestList_Stale verifies that a worktree whose path does not exist on disk
// is classified as "stale" (prunable) regardless of branch existence.
func TestList_Stale(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	// Use a path guaranteed not to exist on disk
	runner.WorktreeListResult = "worktree /nonexistent-path-stale-9f3d\n" +
		"HEAD abc\n" +
		"branch refs/heads/agent/hermes/TAL-99\n" +
		"\n"
	// Branch may or may not exist for stale — disk check takes priority
	runner.BranchExistsResultsByBranch = map[string]bool{
		"agent/hermes/TAL-99": true,
	}
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	statuses, err := o.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(statuses))
	}
	if statuses[0].Status != "stale" {
		t.Errorf("Status = %q, want %q", statuses[0].Status, "stale")
	}
}

// TestList_NoDetachedStatus verifies that "detached" is NOT a valid output
// status — detached HEAD worktrees are either orphan (no branch ref) or
// active (path exists), never a separate "detached" status value.
func TestList_NoDetachedStatus(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree talos.wt/agent-hermes\n" +
		"HEAD abc\n" +
		"detached\n" +
		"\n"
	runner.BranchExistsResultsByBranch = map[string]bool{}
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	statuses, err := o.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	for _, s := range statuses {
		if s.Status == "detached" {
			t.Errorf("Status = %q: 'detached' is not a valid list status (use orphan or active)", s.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Teardown — happy path (default: no delete-branch)
// ---------------------------------------------------------------------------

func TestTeardown_HappyPath(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, false)
	if err != nil {
		t.Fatalf("Teardown() unexpected error: %v", err)
	}

	// Call order: WorktreeList → WorktreeRemove → Prune
	runner.AssertMethodOrder(t, "WorktreeList", "WorktreeRemove", "Prune")
	runner.AssertNotCalled(t, "BranchDelete")
}

// ---------------------------------------------------------------------------
// Teardown — --delete-branch: BranchDelete IS called after Prune
// ---------------------------------------------------------------------------

func TestTeardown_DeleteBranch(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, true /* deleteBranch */)
	if err != nil {
		t.Fatalf("Teardown(deleteBranch=true) unexpected error: %v", err)
	}

	// BranchDelete MUST appear after Prune
	runner.AssertMethodOrder(t, "WorktreeList", "WorktreeRemove", "Prune", "BranchDelete")
}

// ---------------------------------------------------------------------------
// Teardown — --force: WorktreeRemove receives force=true
// ---------------------------------------------------------------------------

func TestTeardown_Force(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", true /* force */, false)
	if err != nil {
		t.Fatalf("Teardown(force=true) unexpected error: %v", err)
	}

	removeCalls := runner.CallsFor("WorktreeRemove")
	if len(removeCalls) != 1 {
		t.Fatalf("WorktreeRemove called %d times, want 1", len(removeCalls))
	}
	forceArg, ok := removeCalls[0].Args[1].(bool)
	if !ok || !forceArg {
		t.Errorf("WorktreeRemove(force) arg = %v, want true", removeCalls[0].Args[1])
	}
}

// ---------------------------------------------------------------------------
// Teardown — ErrWorktreeNotFound
// ---------------------------------------------------------------------------

func TestTeardown_NotFound(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "" // no worktrees
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, false)
	if err == nil {
		t.Fatal("expected ErrWorktreeNotFound, got nil")
	}
	var e *worktree.ErrWorktreeNotFound
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrWorktreeNotFound", err)
	}
	runner.AssertNotCalled(t, "WorktreeRemove")
}

// ---------------------------------------------------------------------------
// Env — ErrWorktreeNotFound
// ---------------------------------------------------------------------------

func TestEnv_NotFound(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "" // no worktrees
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Env(context.Background(), "hermes", "TAL-2")
	if err == nil {
		t.Fatal("expected ErrWorktreeNotFound, got nil")
	}
	var e *worktree.ErrWorktreeNotFound
	if !errors.As(err, &e) {
		t.Errorf("error type = %T, want *ErrWorktreeNotFound", err)
	}
	if wf.path != "" {
		t.Error("WriteFile should not have been called when worktree not found")
	}
}

// ---------------------------------------------------------------------------
// Env — happy path: WriteFile receives byte-identical RenderEnv output
// ---------------------------------------------------------------------------

func TestEnv_HappyPath(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Env(context.Background(), "hermes", "TAL-2")
	if err != nil {
		t.Fatalf("Env() unexpected error: %v", err)
	}

	// WriteFile must have been called
	if wf.path == "" {
		t.Fatal("WriteFile was not called")
	}

	// Content must be byte-identical to RenderEnv (idempotency proof)
	spec, _ := worktree.NewWorktreeSpec("hermes", "TAL-2", "talos.wt")
	res, _ := worktree.AgentResources(spec.Figura)
	wantContent := worktree.RenderEnv(spec, res)
	if string(wf.data) != wantContent {
		t.Errorf("WriteFile data =\n%q\nwant (RenderEnv output):\n%q", string(wf.data), wantContent)
	}

	// No JIRA_ in content (Decision 5 guard, belt+suspenders)
	if strings.Contains(string(wf.data), "JIRA_") {
		t.Errorf("env content contains JIRA_ — Decision 5 violation: %s", string(wf.data))
	}
}

// ---------------------------------------------------------------------------
// Teardown — W-4: ErrDirtyWorktree must carry the figura (REQ-TEARDOWN-2,3)
// ---------------------------------------------------------------------------

// TestTeardown_Dirty_HasFigura verifies that when the adapter refuses to
// remove a dirty worktree, the service wraps the error into ErrDirtyWorktree
// with the correct figura (not empty string).
func TestTeardown_Dirty_HasFigura(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	// Adapter returns sentinel "dirty" error — no figura in it
	runner.WorktreeRemoveErr = mock.ErrDirtyWorktreeSentinel("talos.wt/agent-hermes")
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, false)
	if err == nil {
		t.Fatal("expected error from dirty worktree, got nil")
	}

	var dirty *worktree.ErrDirtyWorktree
	if !errors.As(err, &dirty) {
		t.Fatalf("error type = %T, want *ErrDirtyWorktree; err = %v", err, err)
	}
	// The service MUST enrich the error with the figura
	if dirty.Figura != "hermes" {
		t.Errorf("ErrDirtyWorktree.Figura = %q, want %q", dirty.Figura, "hermes")
	}
	// Path must also be set
	if dirty.Path == "" {
		t.Errorf("ErrDirtyWorktree.Path is empty, want non-empty")
	}
}

// TestTeardown_Dirty_FileCount verifies that when file-count is available,
// ErrDirtyWorktree.FileCount is > 0. This test uses a mock that sets
// WorktreeRemoveFileCount to simulate the adapter reporting a count.
func TestTeardown_Dirty_FileCount(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /repo\n" +
		"HEAD abc\n" +
		"branch refs/heads/develop\n" +
		"\n" +
		"worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	// Adapter returns sentinel with a file count of 3
	runner.WorktreeRemoveErr = mock.ErrDirtyWorktreeSentinelWithCount("talos.wt/agent-hermes", 3)
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, false)
	if err == nil {
		t.Fatal("expected error from dirty worktree, got nil")
	}

	var dirty *worktree.ErrDirtyWorktree
	if !errors.As(err, &dirty) {
		t.Fatalf("error type = %T, want *ErrDirtyWorktree; err = %v", err, err)
	}
	if dirty.FileCount != 3 {
		t.Errorf("ErrDirtyWorktree.FileCount = %d, want 3", dirty.FileCount)
	}
}
