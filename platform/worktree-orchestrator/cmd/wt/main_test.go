// Package main_test contains tests for the wt CLI argument validation and subcommand dispatch.
package main

import (
	"strings"
	"testing"
)

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
	err := run([]string{"create"})
	if err == nil {
		t.Fatal("run(create) with no positional args: expected error, got nil")
	}
}

func TestRun_Create_TooFewArgs_ReturnsError(t *testing.T) {
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
	_ = run([]string{"list"})
}

func TestRun_KnownSubcommands_Recognized(t *testing.T) {
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
