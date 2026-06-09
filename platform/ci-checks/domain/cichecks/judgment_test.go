package cichecks

import (
	"errors"
	"testing"
)

// validHeader is a complete well-formed judgment report header used as baseline.
const validHeader = `**Change:** my-change
**Round:** 1
**Judges:** ARGOS-1, ARGOS-2
**Implementor:** HERMES
**Date:** 2026-06-09
`

func buildReport(header, terminal string) string {
	return header + "\n" + terminal + "\n"
}

// TestParseJudgmentReport_Table covers all spec cases for ParseJudgmentReport + ApprovedFor.
func TestParseJudgmentReport_Table(t *testing.T) {
	cases := []struct {
		name         string
		md           string
		change       string  // slug passed to ApprovedFor
		wantParseErr bool    // true if ParseJudgmentReport itself should return an error
		wantApproved bool    // true if ApprovedFor should return nil
		wantErrType  string  // "malformed" | "notapproved" | ""
	}{
		{
			name:         "APPROVED no emoji",
			md:           buildReport(validHeader, "JUDGMENT: APPROVED"),
			change:       "my-change",
			wantParseErr: false,
			wantApproved: true,
		},
		{
			name:         "APPROVED with emoji",
			md:           buildReport(validHeader, "JUDGMENT: APPROVED ✅"),
			change:       "my-change",
			wantParseErr: false,
			wantApproved: true,
		},
		{
			name:         "ESCALATED no emoji",
			md:           buildReport(validHeader, "JUDGMENT: ESCALATED"),
			change:       "my-change",
			wantParseErr: false,
			wantApproved: false,
			wantErrType:  "notapproved",
		},
		{
			name:         "ESCALATED with emoji",
			md:           buildReport(validHeader, "JUDGMENT: ESCALATED ⚠️"),
			change:       "my-change",
			wantParseErr: false,
			wantApproved: false,
			wantErrType:  "notapproved",
		},
		{
			name: "no terminal JUDGMENT line",
			md: `**Change:** my-change
**Round:** 1
**Judges:** ARGOS-1, ARGOS-2
**Implementor:** HERMES
**Date:** 2026-06-09

Some body text without a terminal line.
`,
			change:       "my-change",
			wantParseErr: true,
			wantErrType:  "malformed",
		},
		{
			name: "Round 0 is malformed",
			md: `**Change:** my-change
**Round:** 0
**Judges:** ARGOS-1, ARGOS-2
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`,
			change:       "my-change",
			wantParseErr: true,
			wantErrType:  "malformed",
		},
		{
			name: "change mismatch",
			md:   buildReport(validHeader, "JUDGMENT: APPROVED"),
			change:       "other-change",
			wantParseErr: false,
			wantApproved: false,
			wantErrType:  "notapproved",
		},
		{
			name: "missing Judges field",
			md: `**Change:** my-change
**Round:** 1
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`,
			change:       "my-change",
			wantParseErr: true,
			wantErrType:  "malformed",
		},
		{
			name: "missing Implementor field",
			md: `**Change:** my-change
**Round:** 1
**Judges:** ARGOS-1, ARGOS-2
**Date:** 2026-06-09

JUDGMENT: APPROVED
`,
			change:       "my-change",
			wantParseErr: true,
			wantErrType:  "malformed",
		},
		{
			name: "O-1 judges not 2 is surface-only violation",
			md: `**Change:** my-change
**Round:** 1
**Judges:** ARGOS-1
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`,
			change:       "my-change",
			wantParseErr: false,
			wantApproved: true,
		},
		{
			name: "O-1 implementor in judges is surface-only violation",
			md: `**Change:** my-change
**Round:** 1
**Judges:** HERMES, ARGOS-1
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`,
			change:       "my-change",
			wantParseErr: false,
			wantApproved: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := ParseJudgmentReport(tc.md)

			if tc.wantParseErr {
				if err == nil {
					t.Fatalf("ParseJudgmentReport: expected parse error, got nil (report=%+v)", report)
				}
				// Verify error type matches expectation.
				if tc.wantErrType == "malformed" {
					var malformed *ErrMalformedJudgment
					if !errors.As(err, &malformed) {
						t.Errorf("expected *ErrMalformedJudgment, got %T: %v", err, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseJudgmentReport: unexpected error: %v", err)
			}

			approvedErr := report.ApprovedFor(tc.change)

			if tc.wantApproved {
				if approvedErr != nil {
					t.Errorf("ApprovedFor(%q): expected nil, got %v", tc.change, approvedErr)
				}
				return
			}

			// Expect a non-nil error from ApprovedFor.
			if approvedErr == nil {
				t.Fatalf("ApprovedFor(%q): expected error, got nil", tc.change)
			}
			if tc.wantErrType == "notapproved" {
				var notApproved *ErrJudgmentNotApproved
				if !errors.As(approvedErr, &notApproved) {
					t.Errorf("expected *ErrJudgmentNotApproved, got %T: %v", approvedErr, approvedErr)
				}
			}
		})
	}
}

// TestJudgmentReport_Violations_O1 verifies O-1 surface violations are collected without failing.
func TestJudgmentReport_Violations_O1(t *testing.T) {
	// Single judge (not 2) — surface warning only.
	md := `**Change:** my-change
**Round:** 1
**Judges:** ARGOS-1
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`
	report, err := ParseJudgmentReport(md)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(report.Violations) == 0 {
		t.Error("expected at least one O-1 violation for single judge, got none")
	}

	// Implementor in judges — surface warning only.
	md2 := `**Change:** my-change
**Round:** 1
**Judges:** HERMES, ARGOS-1
**Implementor:** HERMES
**Date:** 2026-06-09

JUDGMENT: APPROVED
`
	report2, err := ParseJudgmentReport(md2)
	if err != nil {
		t.Fatalf("unexpected parse error for implementor-in-judges: %v", err)
	}
	if len(report2.Violations) == 0 {
		t.Error("expected at least one O-1 violation for implementor in judges, got none")
	}
}
