package overlap

import (
	"sort"
	"strings"
)

// ClaimSource distinguishes a declared (T0 checklist) claim from an actual (T1 git) claim.
type ClaimSource int

const (
	// SourceDeclared is a claim derived from a Jira checklist (T0 pre-assignment).
	SourceDeclared ClaimSource = iota
	// SourceActual is a claim derived from a git worktree diff (T1 in-flight scan).
	SourceActual
)

// Claim is one agent's set of files in play, from a Jira checklist (T0) or a worktree diff (T1).
type Claim struct {
	Agent  string
	Module string
	Ref    string
	Files  []string
	Source ClaimSource
}

// NewClaim builds a Claim with files normalized (trimmed, deduped, sorted) for deterministic comparison.
func NewClaim(agent, module, ref string, files []string, src ClaimSource) Claim {
	normalized := normalizeFiles(files)
	return Claim{
		Agent:  agent,
		Module: module,
		Ref:    ref,
		Files:  normalized,
		Source: src,
	}
}

func normalizeFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	out := make([]string, 0, len(files))
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, exists := seen[f]; exists {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
