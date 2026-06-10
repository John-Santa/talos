// Command ov is the composition root for the overlap-guard CLI.
//
// Usage:
//
//	ov check  --module M --agent A [--files-file f] [--site-url url] [--max-results N] [--json]
//	ov scan   [--base develop] [--wt-bin wt] [--no-fetch] [--remote] [--branches csv] [--json]
//	ov metric [--threshold 0.15] [--strict] [--base develop] [--wt-bin wt] [--no-fetch] [--json]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitcli"
	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitremote"
	"github.com/John-Santa/talos/platform/overlap-guard/adapter/jirarest"
	"github.com/John-Santa/talos/platform/overlap-guard/adapter/wtcli"
	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
	"github.com/John-Santa/talos/platform/overlap-guard/internal/envfile"
	"github.com/John-Santa/talos/platform/overlap-guard/port"
	"github.com/John-Santa/talos/platform/overlap-guard/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ov: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string) error {
	// R8 / ADR-J3: load env files BEFORE any os.Getenv call or flag default
	// evaluation (e.g. fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)).
	// Real environment wins (if-unset semantics); CI is unaffected.
	root := repoRoot()
	paths := []string{
		filepath.Join(root, ".talos", "project.env"),
		filepath.Join(root, ".env"),
	}
	if mainRoot := mainWorktreeRoot(); mainRoot != "" && mainRoot != root {
		paths = append(paths, filepath.Join(mainRoot, ".env"))
	}
	_ = envfile.LoadInto(os.Setenv, os.Getenv, paths...)

	if len(args) == 0 {
		return fmt.Errorf("subcommand required: check | scan | metric")
	}
	switch args[0] {
	case "check":
		return cmdCheck(args[1:])
	case "scan":
		return cmdScan(args[1:])
	case "metric":
		return cmdMetric(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: check, scan, metric", args[0])
	}
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var nc *overlap.ErrNoClaims
	if errors.As(err, &nc) {
		return 0
	}
	return 1
}

// errForBlock maps a BLOCK verdict to the sentinel error that exitCodeFor turns into exit 1.
// Both the --json and the text path of check/scan return it, so the JSON branch no longer
// swallows the verdict (a BLOCK with --json used to encode fine and exit 0, defanging the gate).
// OK/SERIALIZE → nil (exit 0).
func errForBlock(report overlap.Report) error {
	if report.Verdict == overlap.VerdictBlock {
		return &overlap.ErrSameFileParallel{}
	}
	return nil
}

// errForStrict maps an over-threshold metric to an error when --strict is set, mirroring the
// text path of cmdMetric so `metric --json --strict` also exits 1 over threshold. Without
// --strict (or under threshold) metric stays purely informational (exit 0).
func errForStrict(report overlap.Report, strict bool, threshold float64) error {
	if strict && report.OverThreshold {
		return fmt.Errorf("collision rate %.4f exceeds threshold %.4f (--strict)", report.CollisionRate, threshold)
	}
	return nil
}

func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}

// mainWorktreeRoot returns the main worktree checkout root when running inside
// a linked worktree, enabling .env fallback from the primary checkout. Returns
// "" on any error (best-effort).
func mainWorktreeRoot() string {
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return ""
	}
	if !filepath.IsAbs(commonDir) {
		wd, err := os.Getwd()
		if err != nil {
			return ""
		}
		commonDir = filepath.Join(wd, commonDir)
	}
	return filepath.Dir(commonDir)
}

// fileCollisionJSON is a single entry in file_collisions[]. agents[] contains both agent identifiers (REQ-OUTPUT-1).
type fileCollisionJSON struct {
	File   string   `json:"file"`
	Agents []string `json:"agents"`
}

// moduleOverlapJSON is a single entry in module_overlaps[]. agents[] contains both agent identifiers (REQ-OUTPUT-1).
type moduleOverlapJSON struct {
	Module string   `json:"module"`
	Agents []string `json:"agents"`
}

// checkOnlyJSON is the --json output shape for ov check (REQ-OUTPUT-1).
// Scan-only fields (collision_rate, threshold, over_threshold, pairs_evaluated, colliding_pairs) are omitted.
type checkOnlyJSON struct {
	Verdict        string              `json:"verdict"`
	FileCollisions []fileCollisionJSON `json:"file_collisions"`
	ModuleOverlaps []moduleOverlapJSON `json:"module_overlaps"`
	Advisories     []string            `json:"advisories"`
}

// scanOnlyJSON is the --json output shape for ov scan (REQ-OUTPUT-1).
type scanOnlyJSON struct {
	Verdict        string              `json:"verdict"`
	CollisionRate  float64             `json:"collision_rate"`
	PairsEvaluated int                 `json:"pairs_evaluated"`
	CollidingPairs int                 `json:"colliding_pairs"`
	FileCollisions []fileCollisionJSON `json:"file_collisions"`
	ModuleOverlaps []moduleOverlapJSON `json:"module_overlaps"`
	Advisories     []string            `json:"advisories"`
}

// metricOnlyJSON is the --json output shape for ov metric (REQ-OUTPUT-1).
// verdict, file_collisions, module_overlaps, advisories are omitted — metric informs HG6 only.
type metricOnlyJSON struct {
	CollisionRate  float64 `json:"collision_rate"`
	Threshold      float64 `json:"threshold"`
	OverThreshold  bool    `json:"over_threshold"`
	PairsEvaluated int     `json:"pairs_evaluated"`
	CollidingPairs int     `json:"colliding_pairs"`
}

func verdictString(v overlap.Verdict) string {
	switch v {
	case overlap.VerdictBlock:
		return "BLOCK"
	case overlap.VerdictSerialize:
		return "SERIALIZE"
	default:
		return "OK"
	}
}

func buildFileCollisions(report overlap.Report) []fileCollisionJSON {
	fcs := make([]fileCollisionJSON, 0, len(report.FileCollisions))
	for _, fc := range report.FileCollisions {
		agents := []string{fc.A.Agent, fc.B.Agent}
		if agents[0] > agents[1] {
			agents[0], agents[1] = agents[1], agents[0]
		}
		fcs = append(fcs, fileCollisionJSON{File: fc.File, Agents: agents})
	}
	return fcs
}

func buildModuleOverlaps(report overlap.Report) []moduleOverlapJSON {
	mos := make([]moduleOverlapJSON, 0, len(report.ModuleOverlaps))
	for _, mo := range report.ModuleOverlaps {
		agents := []string{mo.A.Agent, mo.B.Agent}
		if agents[0] > agents[1] {
			agents[0], agents[1] = agents[1], agents[0]
		}
		mos = append(mos, moduleOverlapJSON{Module: mo.Module, Agents: agents})
	}
	return mos
}

func advisoriesSlice(report overlap.Report) []string {
	if report.Advisories != nil {
		return report.Advisories
	}
	return []string{}
}

// reportToCheckJSON maps a Report to the check subcommand JSON shape (REQ-OUTPUT-1).
func reportToCheckJSON(report overlap.Report) checkOnlyJSON {
	return checkOnlyJSON{
		Verdict:        verdictString(report.Verdict),
		FileCollisions: buildFileCollisions(report),
		ModuleOverlaps: buildModuleOverlaps(report),
		Advisories:     advisoriesSlice(report),
	}
}

// reportToScanJSON maps a Report to the scan subcommand JSON shape (REQ-OUTPUT-1).
func reportToScanJSON(report overlap.Report) scanOnlyJSON {
	return scanOnlyJSON{
		Verdict:        verdictString(report.Verdict),
		CollisionRate:  report.CollisionRate,
		PairsEvaluated: pairsEvaluatedFromReport(report),
		CollidingPairs: collidingPairsFromReport(report),
		FileCollisions: buildFileCollisions(report),
		ModuleOverlaps: buildModuleOverlaps(report),
		Advisories:     advisoriesSlice(report),
	}
}

// reportToMetricJSON maps a Report to the metric subcommand JSON shape (REQ-OUTPUT-1).
func reportToMetricJSON(report overlap.Report, threshold float64) metricOnlyJSON {
	return metricOnlyJSON{
		CollisionRate:  report.CollisionRate,
		Threshold:      threshold,
		OverThreshold:  report.OverThreshold,
		PairsEvaluated: pairsEvaluatedFromReport(report),
		CollidingPairs: collidingPairsFromReport(report),
	}
}

func pairsEvaluatedFromReport(r overlap.Report) int {
	seen := make(map[string]bool)
	for _, fc := range r.FileCollisions {
		key := pairKey(fc.A.Agent, fc.B.Agent)
		seen[key] = true
	}
	for _, mo := range r.ModuleOverlaps {
		key := pairKey(mo.A.Agent, mo.B.Agent)
		seen[key] = true
	}
	return len(seen)
}

func collidingPairsFromReport(r overlap.Report) int {
	seen := make(map[string]bool)
	for _, fc := range r.FileCollisions {
		key := pairKey(fc.A.Agent, fc.B.Agent)
		seen[key] = true
	}
	return len(seen)
}

func pairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

// writeJSONTo encodes v as indented JSON to w. The result-emitting helpers below write through it so
// they are testable against a buffer (the original gate-defeating bug lived in the emit wiring, not the helpers).
func writeJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emitCheckResult writes the check report (JSON or text) to w and returns the verdict error.
// BOTH paths return errForBlock(report), so --json never swallows a BLOCK verdict (the gate-defeating
// bug this change fixes). JSON is written BEFORE the error is returned, so stdout stays valid JSON.
func emitCheckResult(w io.Writer, report overlap.Report, jsonOut bool) error {
	if jsonOut {
		if err := writeJSONTo(w, reportToCheckJSON(report)); err != nil {
			return err
		}
		return errForBlock(report)
	}
	fmt.Fprintf(w, "verdict: %s\n", verdictString(report.Verdict))
	return errForBlock(report)
}

// emitScanResult writes the scan report (JSON or text) to w and returns the verdict error (both paths).
func emitScanResult(w io.Writer, report overlap.Report, jsonOut bool) error {
	if jsonOut {
		if err := writeJSONTo(w, reportToScanJSON(report)); err != nil {
			return err
		}
		return errForBlock(report)
	}
	fmt.Fprintf(w, "verdict: %s\n", verdictString(report.Verdict))
	return errForBlock(report)
}

// emitMetricResult writes the metric report (JSON or text) to w and returns the --strict error (both paths).
func emitMetricResult(w io.Writer, report overlap.Report, jsonOut, strict bool, threshold float64) error {
	if jsonOut {
		if err := writeJSONTo(w, reportToMetricJSON(report, threshold)); err != nil {
			return err
		}
		return errForStrict(report, strict, threshold)
	}
	fmt.Fprintf(w, "collision_rate: %.4f  threshold: %.4f  over: %v\n",
		report.CollisionRate, threshold, report.OverThreshold)
	return errForStrict(report, strict, threshold)
}

// scanner/checker/metricer are the narrow report-producing seams each subcommand delegates to.
// *service.Guard satisfies all three; fakes inject canned reports in tests so the command-logic
// wiring (source → emit → verdict error) is regression-locked without real git/wt.
type scanner interface {
	ScanInFlight(ctx context.Context) (overlap.Report, error)
}

type checker interface {
	CheckPreAssignment(ctx context.Context, module, agent string, ownerFiles []string) (overlap.Report, error)
}

type metricer interface {
	Metric(ctx context.Context) (overlap.Report, error)
}

// runScan produces the scan report and emits it; the verdict error propagates to exit 1.
func runScan(ctx context.Context, w io.Writer, s scanner, jsonOut bool) error {
	report, err := s.ScanInFlight(ctx)
	if err != nil {
		return err
	}
	return emitScanResult(w, report, jsonOut)
}

// runCheck produces the check report and emits it; the verdict error propagates to exit 1.
func runCheck(ctx context.Context, w io.Writer, c checker, module, agent string, ownerFiles []string, jsonOut bool) error {
	report, err := c.CheckPreAssignment(ctx, module, agent, ownerFiles)
	if err != nil {
		return err
	}
	return emitCheckResult(w, report, jsonOut)
}

// runMetric produces the metric report and emits it; the --strict error propagates to exit 1.
func runMetric(ctx context.Context, w io.Writer, m metricer, jsonOut, strict bool, threshold float64) error {
	report, err := m.Metric(ctx)
	if err != nil {
		return err
	}
	return emitMetricResult(w, report, jsonOut, strict, threshold)
}

// listerMode is the worktree-lister strategy chosen by cmdScan.
type listerMode int

const (
	// modeWorktree lists local worktrees via the wt binary (default, no --remote).
	modeWorktree listerMode = iota
	// modeRemoteLsRemote discovers in-flight branches via `git ls-remote` (--remote, no --branches).
	modeRemoteLsRemote
	// modeRemoteExplicit uses the explicit --branches set (the open-PR list from CI).
	modeRemoteExplicit
)

// scanListerMode picks the lister strategy. An explicitly-set --branches (branchesSet) selects the
// explicit set EVEN WHEN EMPTY — CI passing an empty list means "zero open PRs, scan nothing", which
// must NOT fall back to ls-remote and its stale squash-merged branches. --branches without --remote
// is ignored (worktree mode).
func scanListerMode(remote, branchesSet bool) listerMode {
	switch {
	case remote && branchesSet:
		return modeRemoteExplicit
	case remote:
		return modeRemoteLsRemote
	default:
		return modeWorktree
	}
}

// splitCSV splits a comma-separated flag value into trimmed, non-empty items.
func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	module := fs.String("module", "", "Jira module label (required)")
	agent := fs.String("agent", "", "Agent identifier (required)")
	filesFile := fs.String("files-file", "", "Path to file listing owner's declared files (one per line)")
	siteURL := fs.String("site-url", os.Getenv("JIRA_SITE_URL"), "Jira site URL")
	maxResults := fs.Int("max-results", 100, "Max Jira search results")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *module == "" || *agent == "" {
		return fmt.Errorf("check requires --module and --agent")
	}

	var ownerFiles []string
	if *filesFile != "" {
		lines, err := readLines(*filesFile)
		if err != nil {
			return fmt.Errorf("reading files-file: %w", err)
		}
		ownerFiles = lines
	}

	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	searcher := jirarest.NewClient(*siteURL, email, token)

	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	cfg.SiteURL = *siteURL
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.Project = v
	}

	cfg.MaxResults = *maxResults
	guard := service.NewGuard(searcher, nil, nil, cfg)
	return runCheck(context.Background(), os.Stdout, guard, *module, *agent, ownerFiles, *jsonOut)
}

func cmdScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	base := fs.String("base", "develop", "Integration base branch")
	wtBin := fs.String("wt-bin", "wt", "wt binary name on PATH")
	noFetch := fs.Bool("no-fetch", false, "Skip initial fetch")
	remote := fs.Bool("remote", false, "Detect collisions via real diffs of remote agent/* branches (CI-friendly, no worktrees)")
	branches := fs.String("branches", "", "Comma-separated agent/* branches to scan (with --remote: overrides git ls-remote with the open-PR set, e.g. from `gh pr list --state open --json headRefName`)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	cfg.BaseBranch = *base
	cfg.WtBinary = *wtBin
	cfg.NoFetch = *noFetch
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.Project = v
	}

	inspector := gitcli.NewInspector(root)

	// Distinguish "--branches not given" from "--branches given but empty" (CI: zero open PRs).
	branchesSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "branches" {
			branchesSet = true
		}
	})

	// --branches only feeds the remote explicit lister; without --remote it would be silently
	// dropped to a worktree scan (likely a false all-clear on a hard gate). Fail loud instead.
	if branchesSet && !*remote {
		return fmt.Errorf("--branches requires --remote")
	}

	var lister port.WorktreeLister
	switch scanListerMode(*remote, branchesSet) {
	case modeRemoteExplicit:
		// CI supplies the in-flight set (open PRs) via --branches; bypass ls-remote so
		// stale squash-merged branches don't cause false-positive collisions. Empty set → no claims.
		lister = gitremote.NewListerFromBranches(splitCSV(*branches))
	case modeRemoteLsRemote:
		lister = gitremote.NewLister(root)
	default:
		lister = wtcli.NewLister(*wtBin)
	}

	guard := service.NewGuard(nil, lister, inspector, cfg)
	return runScan(context.Background(), os.Stdout, guard, *jsonOut)
}

func cmdMetric(args []string) error {
	fs := flag.NewFlagSet("metric", flag.ContinueOnError)
	threshold := fs.Float64("threshold", 0.15, "CollisionRate threshold for HG6")
	strict := fs.Bool("strict", false, "Exit 1 when over threshold")
	base := fs.String("base", "develop", "Integration base branch")
	wtBin := fs.String("wt-bin", "wt", "wt binary name on PATH")
	noFetch := fs.Bool("no-fetch", false, "Skip initial fetch")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	cfg.BaseBranch = *base
	cfg.WtBinary = *wtBin
	cfg.NoFetch = *noFetch
	cfg.Threshold = *threshold
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.Project = v
	}

	inspector := gitcli.NewInspector(root)
	// metric is the local HG6 collision-rate informer; --remote/--branches are intentionally out of
	// scope here (the hard CI gate is `scan --remote --branches`). metric always uses local worktrees.
	lister := wtcli.NewLister(*wtBin)
	guard := service.NewGuard(nil, lister, inspector, cfg)
	return runMetric(context.Background(), os.Stdout, guard, *jsonOut, *strict, *threshold)
}
