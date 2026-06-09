package cichecks

import (
	"fmt"
	"strconv"
	"strings"
)

// JudgmentReport holds the parsed content of a judgment-report.md artifact.
// It is a pure value type with no I/O dependency.
type JudgmentReport struct {
	// Change is the slug parsed from the **Change:** header field.
	Change string
	// Round is the review round number (>= 1).
	Round int
	// Judges is the list of judge identifiers from the **Judges:** field.
	Judges []string
	// Implementor is the value of the **Implementor:** header field.
	Implementor string
	// Verdict is the parsed verdict: "APPROVED" or "ESCALATED".
	Verdict string
	// RawVerdictLine is the raw terminal line before emoji stripping.
	RawVerdictLine string
	// Violations holds O-1 advisory messages (judges≠2, implementor∈judges).
	// These are surface-only in v1 and do NOT cause ApprovedFor to return an error.
	Violations []string
}

// ParseJudgmentReport parses the Markdown content of a judgment-report.md artifact.
// It mirrors the line-scan approach of ParseOwnershipTable: no regex, no tokeniser.
//
// Required fields: Change, Round (>= 1), Judges, Implementor, Date (parsed but
// not stored), and a terminal JUDGMENT: line. Missing any of these returns
// *ErrMalformedJudgment.
//
// O-1 advisory conditions (judges≠2, implementor∈judges) are collected in
// Violations but do NOT cause a parse error.
func ParseJudgmentReport(md string) (JudgmentReport, error) {
	lines := strings.Split(md, "\n")

	var (
		changeVal      string
		roundVal       int
		roundParsed    bool
		judges         []string
		implementor    string
		datePresent    bool
		verdict        string
		rawVerdictLine string
	)

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

		// Terminal verdict line — must start with "JUDGMENT:".
		if strings.HasPrefix(trimmed, "JUDGMENT:") {
			rawVerdictLine = trimmed
			after := strings.TrimSpace(trimmed[len("JUDGMENT:"):])
			// Strip trailing emoji characters (multi-byte; use field split).
			fields := strings.Fields(after)
			if len(fields) == 0 {
				return JudgmentReport{}, &ErrMalformedJudgment{Detail: "JUDGMENT: line has no verdict token"}
			}
			verdict = fields[0] // "APPROVED" or "ESCALATED"
			continue
		}

		// Header field lines: **Field:** value
		if key, val, ok := parseHeaderField(trimmed); ok {
			switch key {
			case "Change":
				changeVal = val
			case "Round":
				n, err := strconv.Atoi(val)
				if err != nil {
					return JudgmentReport{}, &ErrMalformedJudgment{
						Detail: fmt.Sprintf("Round field is not an integer: %q", val),
					}
				}
				roundVal = n
				roundParsed = true
			case "Judges":
				judges = splitAndTrim(val)
			case "Implementor":
				implementor = val
			case "Date":
				datePresent = true
			}
		}
	}

	// Validate required fields.
	if changeVal == "" {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing required field: Change"}
	}
	if !roundParsed {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing required field: Round"}
	}
	if roundVal < 1 {
		return JudgmentReport{}, &ErrMalformedJudgment{
			Detail: fmt.Sprintf("Round must be >= 1, got %d", roundVal),
		}
	}
	if len(judges) == 0 {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing required field: Judges"}
	}
	if implementor == "" {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing required field: Implementor"}
	}
	if !datePresent {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing required field: Date"}
	}
	if verdict == "" {
		return JudgmentReport{}, &ErrMalformedJudgment{Detail: "missing terminal JUDGMENT: line"}
	}
	if verdict != "APPROVED" && verdict != "ESCALATED" {
		return JudgmentReport{}, &ErrMalformedJudgment{
			Detail: fmt.Sprintf("unrecognised verdict %q; must be APPROVED or ESCALATED", verdict),
		}
	}

	// Collect O-1 surface violations (non-fatal in v1).
	var violations []string
	if len(judges) != 2 {
		violations = append(violations, fmt.Sprintf("expected 2 judges, got %d", len(judges)))
	}
	for _, j := range judges {
		if strings.EqualFold(j, implementor) {
			violations = append(violations,
				fmt.Sprintf("implementor %q appears in the judges list", implementor))
			break
		}
	}

	return JudgmentReport{
		Change:         changeVal,
		Round:          roundVal,
		Judges:         judges,
		Implementor:    implementor,
		Verdict:        verdict,
		RawVerdictLine: rawVerdictLine,
		Violations:     violations,
	}, nil
}

// ApprovedFor returns nil if the report verdict is APPROVED and the Change
// field equals the requested slug. Any other condition returns
// *ErrJudgmentNotApproved.
func (r JudgmentReport) ApprovedFor(change string) error {
	if r.Change != change {
		return &ErrJudgmentNotApproved{Change: change, Verdict: r.Verdict}
	}
	if r.Verdict != "APPROVED" {
		return &ErrJudgmentNotApproved{Change: r.Change, Verdict: r.Verdict}
	}
	return nil
}

// parseHeaderField extracts (key, value) from a Markdown bold-field line of the
// form "**Key:** value". Returns ok=false for any other line.
func parseHeaderField(line string) (key, value string, ok bool) {
	if !strings.HasPrefix(line, "**") {
		return "", "", false
	}
	rest := line[2:] // after opening **
	end := strings.Index(rest, ":**")
	if end < 0 {
		return "", "", false
	}
	key = rest[:end]
	value = strings.TrimSpace(rest[end+3:]) // skip ":**"
	return key, value, true
}

// splitAndTrim splits a comma-separated string and trims each token.
func splitAndTrim(s string) []string {
	raw := strings.Split(s, ",")
	result := make([]string, 0, len(raw))
	for _, tok := range raw {
		if t := strings.TrimSpace(tok); t != "" {
			result = append(result, t)
		}
	}
	return result
}
