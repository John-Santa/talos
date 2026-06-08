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

	if len(claims) == 0 {
		return overlap.Report{}, &overlap.ErrNoClaims{}
	}

	return overlap.NewReport(claims, g.cfg.Threshold), nil
}

// CheckPreAssignment runs the T0 gate: JQL search → parse each checklist → cross with the owner's declared files.
func (g *Guard) CheckPreAssignment(ctx context.Context, module, owner string, ownerFiles []string) (overlap.Report, error) {
	jql := overlap.BuildPreAssignmentJQL(g.cfg.Project, module, owner)
	issues, err := g.searcher.Search(ctx, jql, 100)
	if err != nil {
		return overlap.Report{}, fmt.Errorf("searching Jira: %w", err)
	}

	ownerClaim := overlap.NewClaim(owner, module, "owner", ownerFiles, overlap.SourceDeclared)
	claims := []overlap.Claim{ownerClaim}

	for _, issue := range issues {
		issueAgent := agentFromLabels(issue.Labels)
		issueModule := moduleFromLabels(issue.Labels)

		files, parseErr := overlap.ParseFilesChecklist(issue.Body)
		if parseErr != nil {
			// ErrChecklistMissing: advisory — exclude from file-level, keep for module-level.
			// Create a claim with no files so it contributes to module overlaps only.
			claims = append(claims, overlap.NewClaim(issueAgent, issueModule, issue.Key, []string{}, overlap.SourceDeclared))
			continue
		}

		claims = append(claims, overlap.NewClaim(issueAgent, issueModule, issue.Key, files, overlap.SourceDeclared))
	}

	return overlap.NewReport(claims, g.cfg.Threshold), nil
}

// Metric runs ScanInFlight and returns its Report for the HG6 collision-rate gate.
func (g *Guard) Metric(ctx context.Context) (overlap.Report, error) {
	return g.ScanInFlight(ctx)
}

func figuraFromBranch(branch, fallback string) string {
	// agent/<figura>/TAL-N → figura
	parts := strings.Split(branch, "/")
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
