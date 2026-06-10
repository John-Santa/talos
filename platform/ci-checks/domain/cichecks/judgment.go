package cichecks

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// verdictLineRe matches a strict JUDGMENT verdict line at column 0 (no leading
// whitespace). It captures the verdict token only; emoji suffix validation is
// done separately by isStrictVerdictLine. The pattern is anchored so any
// leading whitespace disqualifies the match.
//
// Accepted forms (all validated together with isStrictVerdictLine):
//
//	JUDGMENT: APPROVED
//	JUDGMENT: APPROVED ✅
//	JUDGMENT: ESCALATED
//	JUDGMENT: ESCALATED ⚠️
var verdictLineRe = regexp.MustCompile(`^JUDGMENT: (APPROVED|ESCALATED)(.*)$`)

// allowedEmojiSuffixes are the only permitted trailing tokens after the verdict
// word (preceded by a single space). ⚠️ is two runes: U+26A0 + U+FE0F.
var allowedEmojiSuffixes = []string{" ✅", " ⚠️"}

// isStrictVerdictLine returns (verdict, ok) if line exactly matches one of the
// four accepted verdict forms. An empty suffix is also valid (no emoji).
// This function must be called on the raw (non-trimmed) line.
func isStrictVerdictLine(line string) (verdict string, ok bool) {
	m := verdictLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	verd := m[1]  // "APPROVED" or "ESCALATED"
	tail := m[2]  // everything after the verdict word
	if tail == "" {
		return verd, true
	}
	for _, sfx := range allowedEmojiSuffixes {
		if tail == sfx {
			return verd, true
		}
	}
	return "", false
}

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
//
// Verdict detection follows the strict last-non-empty-line rule (REQ-ARTIFACT-4,
// C2 hardening): the verdict MUST be the LAST non-empty line of the document, and
// it MUST match exactly ^JUDGMENT: (APPROVED|ESCALATED)( [✅⚠️])?$ at column 0
// (no leading whitespace). Any other form — indented, quoted, in prose, in a
// fenced block, with trailing text — is rejected as malformed.
//
// Header fields are collected via the same line-scan used by ParseOwnershipTable.
// Required: Change, Round (>= 1), Judges, Implementor, Date. Missing any returns
// *ErrMalformedJudgment.
//
// O-1 advisory conditions (judges≠2, implementor∈judges) are collected in
// Violations but do NOT cause a parse error.
func ParseJudgmentReport(md string) (JudgmentReport, error) {
	lines := strings.Split(md, "\n")

	var (
		changeVal   string
		roundVal    int
		roundParsed bool
		judges      []string
		implementor string
		datePresent bool
	)

	// --- Pass 1: collect header fields ----------------------------------------
	// We do NOT look for JUDGMENT: lines here. Header parsing is independent of
	// the verdict line so that prose mentioning "JUDGMENT:" cannot inject a verdict.
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)

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

	// --- Pass 2: locate the last non-empty line and enforce strict verdict rules -
	//
	// Two constraints (REQ-ARTIFACT-4, C2 hardening):
	//   A) The last non-empty line MUST be a strict verdict line.
	//   B) There MUST be exactly ONE strict verdict line in the entire document.
	//      More than one → malformed (prevents decoy+real dual-line bypass).
	//
	// "Non-empty" = raw line (after stripping \r) has at least one non-whitespace
	// character. The regex is applied to the raw line (not TrimSpace) so that
	// any leading whitespace disqualifies the match (column-0 requirement).
	lastNonEmpty := ""
	strictVerdictCount := 0
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) != "" {
			lastNonEmpty = line
		}
		if _, ok := isStrictVerdictLine(line); ok {
			strictVerdictCount++
		}
	}

	// Constraint B: exactly one strict verdict line.
	if strictVerdictCount > 1 {
		return JudgmentReport{}, &ErrMalformedJudgment{
			Detail: fmt.Sprintf(
				"multiple strict JUDGMENT verdict lines found (%d); expected exactly one",
				strictVerdictCount,
			),
		}
	}

	// Constraint A: last non-empty line must be the verdict.
	rawVerdictLine := lastNonEmpty
	verdict, ok := isStrictVerdictLine(rawVerdictLine)
	if !ok {
		return JudgmentReport{}, &ErrMalformedJudgment{
			Detail: fmt.Sprintf(
				"last non-empty line %q is not a valid JUDGMENT verdict; "+
					"must be exactly 'JUDGMENT: APPROVED' or 'JUDGMENT: ESCALATED' (optional emoji suffix, column 0)",
				rawVerdictLine,
			),
		}
	}

	// --- Validate header fields -----------------------------------------------
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

	// --- O-1 surface violations (non-fatal in v1) -----------------------------
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
