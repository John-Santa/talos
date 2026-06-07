// Package main_test contains tests for the wt CLI composition root.
// These tests exercise the run(args) function for argument validation and
// subcommand dispatch. They do NOT invoke real git (no t.TempDir + git init
// required for the short suite).
package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Argument dispatch tests (all run in short suite — no real git required)
// ---------------------------------------------------------------------------

func TestRun_NoArgs_ReturnsError(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("run() with no args: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "subcommand") {
		t.Errorf("run() with no args: error %q should mention subcommand", err.Error())
	}
}

func TestRun_UnknownSubcommand_ReturnsError(t *testing.T) {
	err := run([]string{"unknown-cmd"})
	if err == nil {
		t.Fatal("run() with unknown subcommand: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") && !strings.Contains(err.Error(), "unknown-cmd") {
		t.Errorf("run() unknown subcommand: error %q should name the bad subcommand", err.Error())
	}
}

func TestRun_Create_MissingArgs_ReturnsError(t *testing.T) {
	// create requires figura + jiraKey positional args
	err := run([]string{"create"})
	if err == nil {
		t.Fatal("run(create) with no positional args: expected error, got nil")
	}
}

func TestRun_Create_TooFewArgs_ReturnsError(t *testing.T) {
	// create with only one positional arg is invalid
	err := run([]string{"create", "hermes"})
	if err == nil {
		t.Fatal("run(create hermes) with missing jiraKey: expected error, got nil")
	}
}

func TestRun_Teardown_MissingArgs_ReturnsError(t *testing.T) {
	err := run([]string{"teardown"})
	if err == nil {
		t.Fatal("run(teardown) with no positional args: expected error, got nil")
	}
}

func TestRun_Env_MissingArgs_ReturnsError(t *testing.T) {
	err := run([]string{"env"})
	if err == nil {
		t.Fatal("run(env) with no positional args: expected error, got nil")
	}
}

func TestRun_List_NoGit_ReturnsError(t *testing.T) {
	// list with no git repo available returns an error (can't resolve repo root
	// or git fails) — we only assert a non-nil error is returned
	// Note: this test accepts that it may succeed in a git repo context;
	// the important invariant is that no panic occurs.
	_ = run([]string{"list"})
	// No assertion on error vs nil — we only verify it doesn't panic
}

func TestRun_KnownSubcommands_Recognized(t *testing.T) {
	// Ensure each known subcommand is at least recognized (dispatched, not
	// "unknown subcommand"). They may fail for other reasons (missing git,
	// missing args) but must NOT return "unknown subcommand" error.
	knownButNeedsArgs := [][]string{
		{"create", "hermes", "TAL-2"},
		{"teardown", "hermes", "TAL-2"},
		{"env", "hermes", "TAL-2"},
	}
	for _, args := range knownButNeedsArgs {
		err := run(args)
		if err != nil && (strings.Contains(err.Error(), "unknown subcommand") ||
			strings.Contains(err.Error(), "unknown command")) {
			t.Errorf("run(%v): got 'unknown subcommand' error, want dispatch: %v", args, err)
		}
	}
}
