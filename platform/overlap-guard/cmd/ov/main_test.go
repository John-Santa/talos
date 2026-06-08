package main

import (
	"errors"
	"testing"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
)

// TestRun_UnknownSubcommand verifies that an unknown subcommand returns an error.
func TestRun_UnknownSubcommand(t *testing.T) {
	err := run([]string{"unknown-cmd"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

// TestRun_NoSubcommand verifies that invoking with no args returns an error.
func TestRun_NoSubcommand(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand, got nil")
	}
}

// TestExitCodeFor_NoClaims verifies that ErrNoClaims maps to exit code 0.
func TestExitCodeFor_NoClaims(t *testing.T) {
	err := &overlap.ErrNoClaims{}
	code := exitCodeFor(err)
	if code != 0 {
		t.Errorf("ErrNoClaims should exit 0, got %d", code)
	}
}

// TestExitCodeFor_SameFileParallel verifies that ErrSameFileParallel maps to exit code 1.
func TestExitCodeFor_SameFileParallel(t *testing.T) {
	err := &overlap.ErrSameFileParallel{File: "foo.go"}
	code := exitCodeFor(err)
	if code != 1 {
		t.Errorf("ErrSameFileParallel should exit 1, got %d", code)
	}
}

// TestExitCodeFor_GenericError verifies that arbitrary errors map to exit code 1.
func TestExitCodeFor_GenericError(t *testing.T) {
	err := errors.New("some error")
	code := exitCodeFor(err)
	if code != 1 {
		t.Errorf("generic error should exit 1, got %d", code)
	}
}

// TestExitCodeFor_Nil verifies that nil maps to exit code 0.
func TestExitCodeFor_Nil(t *testing.T) {
	code := exitCodeFor(nil)
	if code != 0 {
		t.Errorf("nil error should exit 0, got %d", code)
	}
}

// TestRun_CheckMissingFlags verifies that check without required flags returns an error.
func TestRun_CheckMissingFlags(t *testing.T) {
	// check requires --module and --agent at minimum
	err := run([]string{"check"})
	if err == nil {
		t.Fatal("expected error for check with no flags, got nil")
	}
}

// TestRun_ScanNoError verifies that scan subcommand parses its flags without panicking.
// This is a flag-parse-only test (no real git/wt available in unit context).
func TestRun_ScanNoError(t *testing.T) {
	// Just verify the scan subcommand recognises its flags — no real I/O.
	// Passing --no-fetch --base develop would try to call wt which may not exist.
	// The error is expected (binary not found or git error), but no panic.
	err := run([]string{"scan", "--no-fetch", "--base", "develop"})
	// err is expected here (no real wt/git); we just check no panic occurred.
	_ = err
}
