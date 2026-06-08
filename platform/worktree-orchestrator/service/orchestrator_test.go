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

func defaultCfg() service.Config {
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = "/repo"
	return cfg
}

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

func newOrchestrator(runner *mock.GitRunnerMock, wf *fakeWriteFile) *service.Orchestrator {
	o := service.NewOrchestrator(runner, defaultCfg())
	o.WriteFile = wf.write
	return o
}

func TestCreate_HappyPath(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = ""
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	ctx := context.Background()
	err := o.Create(ctx, "hermes", "TAL-2", false)
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	runner.AssertMethodOrder(t, "BranchExists", "WorktreeList", "Fetch", "WorktreeAdd")

	wantPath := "talos.wt/agent-hermes/.env"
	if wf.path != wantPath {
		t.Errorf("WriteFile path = %q, want %q", wf.path, wantPath)
	}

	spec, _ := worktree.NewWorktreeSpec("hermes", "TAL-2", "talos.wt")
	res, _ := worktree.AgentResources(spec.Figura)
	wantContent := worktree.RenderEnv(spec, res)
	if string(wf.data) != wantContent {
		t.Errorf("WriteFile data =\n%q\nwant:\n%q", string(wf.data), wantContent)
	}
}

func TestCreate_NoFetch(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = false
	runner.WorktreeListResult = ""
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	ctx := context.Background()
	err := o.Create(ctx, "atlas", "TAL-1", true)
	if err != nil {
		t.Fatalf("Create(noFetch=true) unexpected error: %v", err)
	}

	runner.AssertNotCalled(t, "Fetch")
	runner.AssertMethodOrder(t, "BranchExists", "WorktreeList", "WorktreeAdd")
}

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

func TestCreate_BranchExists(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.BranchExistsResult = true
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
	if wf.path != "" {
		t.Errorf("WriteFile was called with path %q after WorktreeAdd failure", wf.path)
	}
}

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
	if !strings.Contains(err.Error(), ".env") && !strings.Contains(err.Error(), "partial") {
		t.Errorf("error %q should mention .env or partial state", err.Error())
	}
}

func TestList_Active(t *testing.T) {
	t.Parallel()
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

func TestList_Orphan(t *testing.T) {
	t.Parallel()
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

func TestList_Stale(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree /nonexistent-path-stale-9f3d\n" +
		"HEAD abc\n" +
		"branch refs/heads/agent/hermes/TAL-99\n" +
		"\n"
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

	runner.AssertMethodOrder(t, "WorktreeList", "WorktreeRemove", "Prune")
	runner.AssertNotCalled(t, "BranchDelete")
}

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

	err := o.Teardown(context.Background(), "hermes", "TAL-2", false, true)
	if err != nil {
		t.Fatalf("Teardown(deleteBranch=true) unexpected error: %v", err)
	}

	runner.AssertMethodOrder(t, "WorktreeList", "WorktreeRemove", "Prune", "BranchDelete")
}

func TestTeardown_Force(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = "worktree talos.wt/agent-hermes\n" +
		"HEAD def\n" +
		"branch refs/heads/agent/hermes/TAL-2\n" +
		"\n"
	wf := &fakeWriteFile{}
	o := newOrchestrator(runner, wf)

	err := o.Teardown(context.Background(), "hermes", "TAL-2", true, false)
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

func TestTeardown_NotFound(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = ""
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

func TestEnv_NotFound(t *testing.T) {
	t.Parallel()
	runner := mock.NewGitRunnerMock()
	runner.WorktreeListResult = ""
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

	if wf.path == "" {
		t.Fatal("WriteFile was not called")
	}

	spec, _ := worktree.NewWorktreeSpec("hermes", "TAL-2", "talos.wt")
	res, _ := worktree.AgentResources(spec.Figura)
	wantContent := worktree.RenderEnv(spec, res)
	if string(wf.data) != wantContent {
		t.Errorf("WriteFile data =\n%q\nwant (RenderEnv output):\n%q", string(wf.data), wantContent)
	}

	if strings.Contains(string(wf.data), "JIRA_") {
		t.Errorf("env content contains JIRA_ — Decision 5 violation: %s", string(wf.data))
	}
}

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
	if dirty.Figura != "hermes" {
		t.Errorf("ErrDirtyWorktree.Figura = %q, want %q", dirty.Figura, "hermes")
	}
	if dirty.Path == "" {
		t.Errorf("ErrDirtyWorktree.Path is empty, want non-empty")
	}
}

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
