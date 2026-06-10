package main

import (
	"strings"
	"testing"
)

// TestRun_NoArgs verifies that running with no args prints usage and returns error.
func TestRun_NoArgs(t *testing.T) {
	t.Parallel()
	err := run(nil)
	if err == nil {
		t.Fatal("expected error with no args, got nil")
	}
	if !strings.Contains(err.Error(), "subcommand") {
		t.Errorf("error %q should mention 'subcommand'", err.Error())
	}
}

// TestRun_UnknownSubcommand verifies unknown subcommand returns error.
func TestRun_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := run([]string{"invalid-subcommand"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

// TestRun_Dispatch_MissingFlags verifies dispatch fails without required flags.
func TestRun_Dispatch_MissingFlags(t *testing.T) {
	t.Parallel()
	// dispatch requires --change, --phase, --agent, --module
	err := run([]string{"dispatch"})
	if err == nil {
		t.Fatal("expected error for dispatch without flags, got nil")
	}
}

// TestRun_Dispatch_RequiredFlags verifies missing individual required flags are caught.
func TestRun_Dispatch_RequiredFlags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "missing --phase",
			args: []string{"dispatch", "--change=orchestration", "--agent=hephaestus", "--module=module:orchestration"},
		},
		{
			name: "missing --change",
			args: []string{"dispatch", "--phase=propose", "--agent=hephaestus", "--module=module:orchestration"},
		},
		{
			name: "missing --agent",
			args: []string{"dispatch", "--phase=propose", "--change=orchestration", "--module=module:orchestration"},
		},
		{
			name: "missing --module",
			args: []string{"dispatch", "--phase=propose", "--change=orchestration", "--agent=hephaestus"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := run(tc.args)
			if err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// TestRun_Status_Subcommand verifies status subcommand is recognized (may fail on execution without binaries).
func TestRun_Status_Subcommand(t *testing.T) {
	t.Parallel()
	// status subcommand should be recognized; it will fail due to no real binaries,
	// but it should NOT fail with "unknown subcommand".
	err := run([]string{"status"})
	if err != nil && strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("status should be a known subcommand, got: %v", err)
	}
}
