package main

import (
	"testing"
)

// TestRepoRootDoesNotPanic verifies repoRoot() never panics and always returns
// a non-empty string (falls back to cwd when git is unavailable).
func TestRepoRootDoesNotPanic(t *testing.T) {
	t.Parallel()
	got := repoRoot()
	if got == "" {
		t.Error("repoRoot() returned empty string; expected cwd fallback")
	}
}

// TestMainWorktreeRootDoesNotPanic verifies mainWorktreeRoot() never panics
// and returns either a non-empty path or "" (both are valid).
func TestMainWorktreeRootDoesNotPanic(t *testing.T) {
	t.Parallel()
	// No assertion on value — only that it does not panic.
	_ = mainWorktreeRoot()
}

// TestMainWorktreeRootOutsideRepo verifies mainWorktreeRoot() returns "" when
// git-common-dir is unavailable or points to the same root as repoRoot
// (i.e. not a linked worktree). We cannot force "outside a repo" in the test
// runner, but we can confirm the function handles the normal repo case
// gracefully (returns "" or a valid path, no panic).
func TestMainWorktreeRootGraceful(t *testing.T) {
	t.Parallel()
	result := mainWorktreeRoot()
	// result is either "" (main worktree / error) or a valid dir path.
	// We only verify the invariant: if non-empty, it must be absolute.
	if result != "" {
		if len(result) == 0 || result[0] != '/' {
			t.Errorf("mainWorktreeRoot() returned non-absolute path: %q", result)
		}
	}
}

// ---------------------------------------------------------------------------
// parseRunLoopFlags — new flag parsing tests (Design §D2, §D6)
// ---------------------------------------------------------------------------

func TestParseRunLoopFlags_DryRunFlag(t *testing.T) {
	flags, err := parseRunLoopFlags([]string{
		"--change=TAL-1",
		"--phase=apply",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("parseRunLoopFlags() error = %v", err)
	}
	if !flags.DryRun {
		t.Error("DryRun = false, want true")
	}
}

func TestParseRunLoopFlags_DryRunEnvVar(t *testing.T) {
	t.Setenv("EVIDENCE_DRY_RUN", "1")

	flags, err := parseRunLoopFlags([]string{
		"--change=TAL-1",
		"--phase=apply",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
	})
	if err != nil {
		t.Fatalf("parseRunLoopFlags() error = %v", err)
	}
	if !flags.DryRun {
		t.Error("DryRun = false, want true when EVIDENCE_DRY_RUN=1")
	}
}

func TestParseRunLoopFlags_JiraKeyFlag(t *testing.T) {
	flags, err := parseRunLoopFlags([]string{
		"--change=TAL-1",
		"--phase=spec",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
		"--jira-key=TAL-42",
	})
	if err != nil {
		t.Fatalf("parseRunLoopFlags() error = %v", err)
	}
	if flags.JiraKey != "TAL-42" {
		t.Errorf("JiraKey = %q, want \"TAL-42\"", flags.JiraKey)
	}
}

func TestParseRunLoopFlags_PhaseFlag(t *testing.T) {
	flags, err := parseRunLoopFlags([]string{
		"--change=TAL-1",
		"--phase=verify",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
	})
	if err != nil {
		t.Fatalf("parseRunLoopFlags() error = %v", err)
	}
	if flags.Phase != "verify" {
		t.Errorf("Phase = %q, want \"verify\"", flags.Phase)
	}
}

func TestParseRunLoopFlags_MissingRequiredFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing change", []string{"--phase=apply", "--agent=hermes", "--module=m", "--summary=s"}},
		{"missing phase", []string{"--change=TAL-1", "--agent=hermes", "--module=m", "--summary=s"}},
		{"missing agent", []string{"--change=TAL-1", "--phase=apply", "--module=m", "--summary=s"}},
		{"missing module", []string{"--change=TAL-1", "--phase=apply", "--agent=hermes", "--summary=s"}},
		{"missing summary", []string{"--change=TAL-1", "--phase=apply", "--agent=hermes", "--module=m"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRunLoopFlags(tc.args)
			if err == nil {
				t.Errorf("parseRunLoopFlags() expected error for %q, got nil", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cmdRunLoop — credential and dry-run gate (Design §D6 fail-loud)
// ---------------------------------------------------------------------------

// TestCmdRunLoop_NoCreds_NoDryRun_FailsLoud verifies that missing credentials
// without --dry-run causes a hard error (§7 fail-loud, NOT silent).
func TestCmdRunLoop_NoCreds_NoDryRun_FailsLoud(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_SITE_URL", "")
	t.Setenv("EVIDENCE_DRY_RUN", "")

	err := cmdRunLoop([]string{
		"--change=TAL-1",
		"--phase=apply",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
	})
	if err == nil {
		t.Fatal("cmdRunLoop() expected error when credentials missing and no dry-run, got nil")
	}
}

// TestCmdRunLoop_NoCreds_WithDryRun_Succeeds verifies that missing credentials
// are tolerated when --dry-run is set (DryRunClient used, no network call).
func TestCmdRunLoop_NoCreds_WithDryRun_Succeeds(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_SITE_URL", "https://dry.atlassian.net")
	t.Setenv("EVIDENCE_DRY_RUN", "")

	err := cmdRunLoop([]string{
		"--change=TAL-1",
		"--phase=apply",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
		"--jira-key=TAL-99",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("cmdRunLoop(--dry-run) with no creds error = %v", err)
	}
}

// TestCmdRunLoop_DryRunEnvVar_Succeeds verifies EVIDENCE_DRY_RUN=1 activates
// dry-run mode even without the --dry-run flag.
func TestCmdRunLoop_DryRunEnvVar_Succeeds(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_SITE_URL", "https://dry.atlassian.net")
	t.Setenv("EVIDENCE_DRY_RUN", "1")

	err := cmdRunLoop([]string{
		"--change=TAL-1",
		"--phase=spec",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
		"--jira-key=TAL-55",
	})
	if err != nil {
		t.Fatalf("cmdRunLoop(EVIDENCE_DRY_RUN=1) error = %v", err)
	}
}

// TestCmdRunLoop_PhasePreset_UsesStepSet verifies that --phase selects the
// correct step preset (e.g. spec/design call only AddComment, not CreateIssue).
// In dry-run mode the DryRunClient records nothing real, but we confirm no error.
func TestCmdRunLoop_PhasePreset_AllValidPhases(t *testing.T) {
	phases := []struct {
		phase   string
		jiraKey string // empty = propose (create is included)
	}{
		{"propose", ""},
		{"spec", "TAL-10"},
		{"design", "TAL-11"},
		{"tasks", "TAL-12"},
		{"apply", "TAL-13"},
		{"verify", "TAL-14"},
		{"archive", "TAL-15"},
	}

	for _, tc := range phases {
		t.Run(tc.phase, func(t *testing.T) {
			t.Setenv("JIRA_EMAIL", "")
			t.Setenv("JIRA_API_TOKEN", "")
			t.Setenv("JIRA_SITE_URL", "https://dry.atlassian.net")
			t.Setenv("EVIDENCE_DRY_RUN", "1")

			args := []string{
				"--change=TAL-1",
				"--phase=" + tc.phase,
				"--agent=hermes",
				"--module=jira-loop",
				"--summary=test summary",
				"--dry-run",
			}
			if tc.jiraKey != "" {
				args = append(args, "--jira-key="+tc.jiraKey)
			}

			if err := cmdRunLoop(args); err != nil {
				t.Errorf("cmdRunLoop(phase=%s) error = %v", tc.phase, err)
			}
		})
	}
}

// TestCmdRunLoop_UnknownPhase_FailsLoud verifies that an unknown --phase value
// causes a hard error (fail-loud §7).
func TestCmdRunLoop_UnknownPhase_FailsLoud(t *testing.T) {
	t.Setenv("JIRA_EMAIL", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_SITE_URL", "https://dry.atlassian.net")
	t.Setenv("EVIDENCE_DRY_RUN", "1")

	err := cmdRunLoop([]string{
		"--change=TAL-1",
		"--phase=unknownphase",
		"--agent=hermes",
		"--module=jira-loop",
		"--summary=test",
		"--dry-run",
	})
	if err == nil {
		t.Fatal("cmdRunLoop(--phase=unknownphase) expected error, got nil")
	}
}
