package cichecks

import (
	"fmt"
	"strings"
)

// ParseOwnershipTable parses the first Markdown table containing module:* rows from md.
// Keys in the returned map are full lowercase label strings (e.g. "module:devops").
// Values are agent figures in lowercase (e.g. "hermes").
// Returns ErrMalformedOwnership for duplicate module entries or an empty result.
func ParseOwnershipTable(md string) (map[string]string, error) {
	lines := strings.Split(md, "\n")

	inSection := false
	result := make(map[string]string)

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")

		if !inSection {
			if isModuleTableHeader(line) {
				inSection = true
			}
			continue
		}

		// Once in section, a new ## heading ends the first table.
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			break
		}

		if !strings.Contains(line, "|") {
			continue
		}

		mod, agent, ok := parseOwnershipRow(line)
		if !ok {
			continue
		}

		if _, dup := result[mod]; dup {
			return nil, &ErrMalformedOwnership{
				Detail: fmt.Sprintf("duplicate module %q", strings.TrimPrefix(mod, "module:")),
			}
		}
		result[mod] = agent
	}

	if len(result) == 0 {
		return nil, &ErrMalformedOwnership{Detail: "table is empty or contains no valid module rows"}
	}
	return result, nil
}

// isModuleTableHeader returns true when line is the header of the module→agent table.
func isModuleTableHeader(line string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(trimmed, "module:*") && strings.Contains(trimmed, "|")
}

// parseOwnershipRow extracts (module-label, agent) from a pipe-delimited table row.
// Returns ok=false for separator rows, header rows, or rows without a valid module: prefix.
func parseOwnershipRow(line string) (mod, agent string, ok bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 3 {
		return "", "", false
	}

	col1 := strings.ToLower(strings.TrimSpace(parts[1]))
	col1 = strings.Trim(col1, "`")
	col1 = strings.TrimSpace(col1)

	col2 := strings.ToLower(strings.TrimSpace(parts[2]))

	// Skip separator rows.
	if strings.HasPrefix(col1, "---") || strings.HasPrefix(col2, "---") {
		return "", "", false
	}

	// Must start with module: and have a real suffix (not the wildcard header).
	if !strings.HasPrefix(col1, "module:") {
		return "", "", false
	}
	suffix := col1[len("module:"):]
	if suffix == "" || suffix == "*" {
		return "", "", false
	}

	if col2 == "" {
		return "", "", false
	}

	return col1, col2, true
}
