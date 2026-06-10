package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/workspaces/adapter/envmaterializer"
	"github.com/John-Santa/talos/platform/workspaces/adapter/filevault"
	"github.com/John-Santa/talos/platform/workspaces/adapter/gitprobe"
	"github.com/John-Santa/talos/platform/workspaces/adapter/tomlstore"
	"github.com/John-Santa/talos/platform/workspaces/service"
)

// initGitRepo creates a temporary git repo and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := newCmd("git", "init", dir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "HOME="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

// testManager creates a fully wired Manager using t.TempDir for TALOS_HOME.
func testManager(t *testing.T) (*service.Manager, string) {
	t.Helper()
	talosHome := t.TempDir()

	store, err := tomlstore.New(talosHome)
	if err != nil {
		t.Fatalf("tomlstore.New: %v", err)
	}
	vault, err := filevault.New(talosHome)
	if err != nil {
		t.Fatalf("filevault.New: %v", err)
	}
	probe := gitprobe.New()
	mat := envmaterializer.New()

	return service.NewManager(store, vault, probe, mat), talosHome
}

// runCmd calls run() with the given args and returns the error.
func runCmd(t *testing.T, mgr *service.Manager, args []string, stdin string) error {
	t.Helper()
	var r io.Reader
	if stdin != "" {
		r = strings.NewReader(stdin)
	} else {
		r = strings.NewReader("")
	}
	return run(context.Background(), args, mgr, r, io.Discard)
}

func TestCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"frobnicate"}, "")
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestCmd_NoSubcommand(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{}, "")
	if err == nil {
		t.Error("expected error when no subcommand given")
	}
}

func TestCmd_List_Empty(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"list"}, "")
	if err != nil {
		t.Errorf("list with empty store: %v", err)
	}
}

func TestCmd_Add_And_List(t *testing.T) {
	// No t.Parallel() — uses git init which can be flaky in parallel with env mutation
	repoDir := initGitRepo(t)
	mgr, _ := testManager(t)

	// stdin provides email\napitoken\n
	stdin := "user@example.com\nsecret-token\n"
	err := runCmd(t, mgr, []string{
		"add", "my-project",
		"--repo", repoDir,
		"--site", "https://example.atlassian.net",
		"--project-key", "EX",
		"--project-id", "10001",
		"--issue-type", "Story",
		"--state-new", "To Do",
		"--state-doing", "In Progress",
		"--state-done", "Done",
	}, stdin)
	if err != nil {
		t.Fatalf("add error: %v", err)
	}

	err = runCmd(t, mgr, []string{"list"}, "")
	if err != nil {
		t.Errorf("list after add: %v", err)
	}
}

func TestCmd_Current_Empty(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"current"}, "")
	if err != nil {
		t.Errorf("current with no active: %v", err)
	}
}

func TestCmd_Show_NotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"show", "nonexistent"}, "")
	if err == nil {
		t.Error("expected error for show nonexistent")
	}
}

func TestCmd_Remove_NotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"remove", "nonexistent"}, "")
	if err == nil {
		t.Error("expected error for remove nonexistent")
	}
}

func TestCmd_Use_NotFound(t *testing.T) {
	t.Parallel()
	mgr, _ := testManager(t)
	err := runCmd(t, mgr, []string{"use", "nonexistent"}, "")
	if err == nil {
		t.Error("expected error for use nonexistent")
	}
}

func TestCmd_Add_And_Use_MaterializesFiles(t *testing.T) {
	repoDir := initGitRepo(t)
	mgr, _ := testManager(t)

	stdin := "user@example.com\nsecret-token\n"
	_ = runCmd(t, mgr, []string{
		"add", "proj",
		"--repo", repoDir,
		"--site", "https://example.atlassian.net",
		"--project-key", "EX",
		"--project-id", "10001",
		"--issue-type", "Story",
		"--state-new", "To Do",
		"--state-doing", "In Progress",
		"--state-done", "Done",
	}, stdin)

	err := runCmd(t, mgr, []string{"use", "proj"}, "")
	if err != nil {
		t.Fatalf("use error: %v", err)
	}

	projEnvPath := filepath.Join(repoDir, ".talos", "project.env")
	if _, err := os.Stat(projEnvPath); os.IsNotExist(err) {
		t.Errorf("project.env not created at %q", projEnvPath)
	}
	dotEnvPath := filepath.Join(repoDir, ".env")
	if _, err := os.Stat(dotEnvPath); os.IsNotExist(err) {
		t.Errorf(".env not created at %q", dotEnvPath)
	}
}

// ---- helpers (internal to test file) ----

func newCmd(name string, args ...string) *cmdHelper { return &cmdHelper{name: name, args: args} }

type cmdHelper struct {
	name string
	args []string
	Env  []string
}

func (c *cmdHelper) CombinedOutput() ([]byte, error) {
	cmd := makeOSCmd(c.name, c.args...)
	if len(c.Env) > 0 {
		cmd.Env = c.Env
	}
	return cmd.CombinedOutput()
}

// readLineFromReader reads one non-empty line (trims whitespace and \n).
func readLineFromReader(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
