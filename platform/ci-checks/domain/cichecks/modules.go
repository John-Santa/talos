package cichecks

import (
	"regexp"
	"sort"
)

var moduleRE = regexp.MustCompile(`^(platform/[^/]+)(/.*)?$`)

// ModulesFromChangedPaths returns the unique platform/<module> roots touched by the given
// changed file paths, sorted for determinism. Paths not under platform/ are ignored.
// A bare "platform/<m>" path with no trailing segment is included as a module root.
func ModulesFromChangedPaths(paths []string) []string {
	seen := make(map[string]struct{})
	for _, p := range paths {
		m := moduleRE.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		seen[m[1]] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{}
	}
	return out
}
