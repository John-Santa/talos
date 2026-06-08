package overlap

import (
	"strings"
)

// ParseFilesChecklist extracts file paths from a Jira body's files: checklist (grammar fixed by spec).
// Returns ErrChecklistMissing when the marker is absent or the section yields zero items.
func ParseFilesChecklist(body string) ([]string, error) {
	lines := strings.Split(body, "\n")

	inSection := false
	var files []string

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")

		if !inSection {
			if isFilesMarker(line) {
				inSection = true
			}
			continue
		}

		// Inside the section.
		if strings.TrimSpace(line) == "" {
			continue
		}

		path, ok := parseChecklistItem(line)
		if !ok {
			break
		}
		files = append(files, path)
	}

	if !inSection || len(files) == 0 {
		return nil, &ErrChecklistMissing{}
	}

	return files, nil
}

func isFilesMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.EqualFold(trimmed, "files:")
}

func parseChecklistItem(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// Must start with - or *
	if len(trimmed) == 0 {
		return "", false
	}
	if trimmed[0] != '-' && trimmed[0] != '*' {
		return "", false
	}

	rest := strings.TrimSpace(trimmed[1:])

	// Must have [ ], [x], or [X]
	if len(rest) < 3 {
		return "", false
	}
	if rest[0] != '[' {
		return "", false
	}
	ch := rest[1]
	if ch != ' ' && ch != 'x' && ch != 'X' {
		return "", false
	}
	if rest[2] != ']' {
		return "", false
	}

	path := strings.TrimSpace(rest[3:])
	path = strings.Trim(path, "`")
	path = strings.TrimSpace(path)

	if path == "" {
		return "", false
	}
	return path, true
}
