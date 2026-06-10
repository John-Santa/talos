package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
)

// Guard runs the three overlap use-cases using ONLY read ports (no write port exists — ADR-OV4).
type Guard struct {
	searcher  port.IssueSearcher
	lister    port.WorktreeLister
	inspector port.GitInspector
	cfg       Config
}

// NewGuard wires the read ports and config.
func NewGuard(s port.IssueSearcher, l port.WorktreeLister, g port.GitInspector, cfg Config) *Guard {
	return &Guard{
		searcher:  s,
		lister:    l,
		inspector: g,
		cfg:       cfg,
	}
}

// ScanInFlight runs the T1 scan: wt list → per-branch ChangedFiles(base...branch) → pairwise file collisions across siblings.
func (g *Guard) ScanInFlight(ctx context.Context) (overlap.Report, error) {
	entries, err := g.lister.List(ctx)
	if err != nil {
		return overlap.Report{}, fmt.Errorf("listing worktrees: %w", err)
	}

	// No active in-flight branches → nothing to compare. Short-circuit BEFORE fetch/base-resolution:
	// a zero-PR scan must exit 0 (ErrNoClaims) regardless of whether the base ref resolves on a CI
	// checkout, and there is no reason to shell out to git when there is nothing to diff.
	if !hasActive(entries) {
		return overlap.Report{}, &overlap.ErrNoClaims{}
	}

	if !g.cfg.NoFetch {
		if err := g.inspector.Fetch(ctx); err != nil {
			return overlap.Report{}, fmt.Errorf("fetching origin: %w", err)
		}
	}

	baseTip, err := g.inspector.RevParse(ctx, g.cfg.BaseBranch)
	if err != nil {
		return overlap.Report{}, fmt.Errorf("resolving base branch %s: %w", g.cfg.BaseBranch, err)
	}

	var claims []overlap.Claim
	for _, e := range entries {
		if e.Status != "active" {
			continue
		}
		files, err := g.inspector.ChangedFiles(ctx, baseTip, e.Branch)
		if err != nil {
			return overlap.Report{}, fmt.Errorf("getting changed files for %s: %w", e.Branch, err)
		}
		agent := figuraFromBranch(e.Branch, e.Figura)
		claims = append(claims, overlap.NewClaim(agent, "", e.Branch, files, overlap.SourceActual))
	}

	// Defensive: the hasActive short-circuit above already returns ErrNoClaims for an empty in-flight
	// set, and every active entry appends a claim — so today this is belt-and-suspenders. Kept so a
	// future loop that skips entries can't silently produce an empty (false-OK) report on a hard gate.
	if len(claims) == 0 {
		return overlap.Report{}, &overlap.ErrNoClaims{}
	}

	return overlap.NewReport(claims, g.cfg.Threshold), nil
}

// CheckPreAssignment runs the T0 gate: JQL search → parse each checklist → cross with the owner's declared files.
func (g *Guard) CheckPreAssignment(ctx context.Context, module, owner string, ownerFiles []string) (overlap.Report, error) {
	jql := overlap.BuildPreAssignmentJQL(g.cfg.Project, module, owner)
	maxResults := g.cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}
	issues, err := g.searcher.Search(ctx, jql, maxResults)
	if err != nil {
		return overlap.Report{}, fmt.Errorf("searching Jira: %w", err)
	}

	ownerClaim := overlap.NewClaim(owner, module, "owner", ownerFiles, overlap.SourceDeclared)
	claims := []overlap.Claim{ownerClaim}

	var advisories []string
	for _, issue := range issues {
		issueAgent := agentFromLabels(issue.Labels)
		issueModule := moduleFromLabels(issue.Labels)

		files, parseErr := overlap.ParseFilesChecklist(issue.Body)
		if parseErr != nil {
			// ErrChecklistMissing: advisory — exclude from file-level, keep for module-level (REQ-CHECKLIST-4).
			advisories = append(advisories, fmt.Sprintf("%s sin checklist files: — solape a nivel-archivo no verificable", issue.Key))
			claims = append(claims, overlap.NewClaim(issueAgent, issueModule, issue.Key, []string{}, overlap.SourceDeclared))
			continue
		}

		claims = append(claims, overlap.NewClaim(issueAgent, issueModule, issue.Key, files, overlap.SourceDeclared))
	}

	report := overlap.NewReport(claims, g.cfg.Threshold)
	report.Advisories = advisories
	return report, nil
}

// Metric runs ScanInFlight and returns its Report for the HG6 collision-rate gate.
func (g *Guard) Metric(ctx context.Context) (overlap.Report, error) {
	return g.ScanInFlight(ctx)
}

// hasActive reports whether any entry is in-flight ("active"). Used to skip fetch/base-resolution
// when there is nothing to compare.
func hasActive(entries []port.WorktreeEntry) bool {
	for _, e := range entries {
		if e.Status == "active" {
			return true
		}
	}
	return false
}

func figuraFromBranch(branch, fallback string) string {
	// [origin/]agent/<figura>/TAL-N → figura. Remote-mode entries (gitremote) are "origin/"-prefixed,
	// so strip it before parsing — otherwise the agent identity would silently depend on the fallback.
	b := strings.TrimPrefix(branch, "origin/")
	parts := strings.Split(b, "/")
	if len(parts) >= 2 && parts[0] == "agent" {
		return parts[1]
	}
	return fallback
}

func agentFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, "agent:") {
			return strings.TrimPrefix(l, "agent:")
		}
	}
	return ""
}

func moduleFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, "module:") {
			return strings.TrimPrefix(l, "module:")
		}
	}
	return ""
}
