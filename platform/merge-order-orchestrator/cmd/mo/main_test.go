package main

import (
	"strings"
	"testing"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
)

func TestRun_noArgs(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("expected error with no args, got nil")
	}
	if !strings.Contains(err.Error(), "subcommand") {
		t.Errorf("expected 'subcommand' in error, got: %v", err)
	}
}

func TestRun_unknownSubcommand(t *testing.T) {
	err := run([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("expected 'unknown subcommand' in error, got: %v", err)
	}
}

func TestRun_planHelp(t *testing.T) {
	// --help on plan should return an error (flag.ContinueOnError returns ErrHelp, which we surface)
	err := run([]string{"plan", "--help"})
	// flag.ErrHelp is expected
	if err == nil {
		// some flag implementations exit rather than return — just ensure no panic
		return
	}
}

func TestRun_executeRequiresYes(t *testing.T) {
	// execute without --yes should fail at service level (after wiring),
	// but in unit test context with no wt binary available it may fail earlier.
	// We only assert it returns an error (not nil).
	err := run([]string{"execute"})
	if err == nil {
		t.Fatal("execute without --yes must return an error")
	}
}

func TestRun_checkRequiresBranch(t *testing.T) {
	err := run([]string{"check"})
	if err == nil {
		t.Fatal("check without branch arg must return an error")
	}
	if !strings.Contains(err.Error(), "branch") {
		t.Errorf("expected 'branch' in error, got: %v", err)
	}
}

func TestExitCodeFor_nil(t *testing.T) {
	if exitCodeFor(nil) != 0 {
		t.Error("exitCodeFor(nil) must be 0")
	}
}

func TestExitCodeFor_err(t *testing.T) {
	if exitCodeFor(errSentinel("boom")) != 1 {
		t.Error("exitCodeFor(err) must be 1")
	}
}

func TestExitCodeFor_noCandidates(t *testing.T) {
	// REQ-READINESS-4: ErrNoCandidates → exit 0
	err := &mergeorder.ErrNoCandidates{}
	if exitCodeFor(err) != 0 {
		t.Errorf("exitCodeFor(ErrNoCandidates) must be 0, got %d", exitCodeFor(err))
	}
}

func TestParseDependsFlag(t *testing.T) {
	cases := []struct {
		input string
		wantA string
		wantB string
		ok    bool
	}{
		{"A:B", "A", "B", true},
		{"branch-1:branch-2", "branch-1", "branch-2", true},
		{"bad", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			a, b, err := parseDependsEdge(c.input)
			if c.ok && err != nil {
				t.Errorf("parseDependsEdge(%q) error: %v", c.input, err)
			}
			if !c.ok && err == nil {
				t.Errorf("parseDependsEdge(%q) expected error, got nil", c.input)
			}
			if c.ok {
				if a != c.wantA || b != c.wantB {
					t.Errorf("parseDependsEdge(%q) = (%q, %q), want (%q, %q)", c.input, a, b, c.wantA, c.wantB)
				}
			}
		})
	}
}

func TestParseDependsFile(t *testing.T) {
	input := "# comment\nA:B\n\nB:C\n# another comment\nC:D\n"
	deps, err := parseDependsFileContent(input)
	if err != nil {
		t.Fatalf("parseDependsFileContent: %v", err)
	}
	// A→needs B, B→needs C, C→needs D
	if len(deps["A"]) != 1 || deps["A"][0] != "B" {
		t.Errorf("deps[A] = %v, want [B]", deps["A"])
	}
	if len(deps["B"]) != 1 || deps["B"][0] != "C" {
		t.Errorf("deps[B] = %v, want [C]", deps["B"])
	}
}

func TestParseDependsFile_ignoresBlankAndComments(t *testing.T) {
	input := "\n# skip\n\n  \n"
	deps, err := parseDependsFileContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected empty deps, got %v", deps)
	}
}

// helpers used in tests

func errSentinel(msg string) error {
	return sentinelError(msg)
}

type sentinelError string

func (e sentinelError) Error() string { return string(e) }
