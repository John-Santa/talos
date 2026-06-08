// Command ov is the composition root for the overlap-guard CLI.
//
// Usage:
//
//	ov check  --module M --agent A [--files-file f] [--site-url url] [--max-results N] [--json]
//	ov scan   [--base develop] [--wt-bin wt] [--no-fetch] [--ownership-file f] [--json]
//	ov metric [--threshold 0.15] [--strict] [--base develop] [--wt-bin wt] [--no-fetch] [--json]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/John-Santa/talos/platform/overlap-guard/adapter/gitcli"
	"github.com/John-Santa/talos/platform/overlap-guard/adapter/jirarest"
	"github.com/John-Santa/talos/platform/overlap-guard/adapter/wtcli"
	"github.com/John-Santa/talos/platform/overlap-guard/domain/overlap"
	"github.com/John-Santa/talos/platform/overlap-guard/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ov: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string) error {
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

func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}

// checkJSON is the --json output shape for ov check and ov scan.
type checkJSON struct {
	Verdict         string              `json:"verdict"`
	FileCollisions  []fileCollisionJSON `json:"file_collisions"`
	ModuleOverlaps  []moduleOverlapJSON `json:"module_overlaps"`
	Advisories      []string            `json:"advisories"`
	CollisionRate   float64             `json:"collision_rate"`
	Threshold       float64             `json:"threshold"`
	OverThreshold   bool                `json:"over_threshold"`
	PairsEvaluated  int                 `json:"pairs_evaluated"`
	CollidingPairs  int                 `json:"colliding_pairs"`
}

type fileCollisionJSON struct {
	File   string `json:"file"`
	AgentA string `json:"agent_a"`
	AgentB string `json:"agent_b"`
}

type moduleOverlapJSON struct {
	Module string `json:"module"`
	AgentA string `json:"agent_a"`
	AgentB string `json:"agent_b"`
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

func reportToJSON(report overlap.Report, threshold float64) checkJSON {
	out := checkJSON{
		Verdict:        verdictString(report.Verdict),
		CollisionRate:  report.CollisionRate,
		Threshold:      threshold,
		OverThreshold:  report.OverThreshold,
		PairsEvaluated: pairsFromReport(report),
		CollidingPairs: collidingPairsFromReport(report),
		Advisories:     []string{},
	}
	for _, fc := range report.FileCollisions {
		out.FileCollisions = append(out.FileCollisions, fileCollisionJSON{
			File:   fc.File,
			AgentA: fc.A.Agent,
			AgentB: fc.B.Agent,
		})
	}
	for _, mo := range report.ModuleOverlaps {
		out.ModuleOverlaps = append(out.ModuleOverlaps, moduleOverlapJSON{
			Module: mo.Module,
			AgentA: mo.A.Agent,
			AgentB: mo.B.Agent,
		})
	}
	if out.FileCollisions == nil {
		out.FileCollisions = []fileCollisionJSON{}
	}
	if out.ModuleOverlaps == nil {
		out.ModuleOverlaps = []moduleOverlapJSON{}
	}
	return out
}

func pairsFromReport(r overlap.Report) int {
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

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
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

	cfg.MaxResults = *maxResults
	guard := service.NewGuard(searcher, nil, nil, cfg)
	ctx := context.Background()
	report, err := guard.CheckPreAssignment(ctx, *module, *agent, ownerFiles)
	if err != nil {
		return err
	}

	if *jsonOut {
		return writeJSON(reportToJSON(report, cfg.Threshold))
	}

	fmt.Printf("verdict: %s\n", verdictString(report.Verdict))
	if report.Verdict == overlap.VerdictBlock {
		return &overlap.ErrSameFileParallel{}
	}
	return nil
}

func cmdScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
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

	inspector := gitcli.NewInspector(root)
	lister := wtcli.NewLister(*wtBin)
	guard := service.NewGuard(nil, lister, inspector, cfg)

	ctx := context.Background()
	report, err := guard.ScanInFlight(ctx)
	if err != nil {
		return err
	}

	if *jsonOut {
		return writeJSON(reportToJSON(report, cfg.Threshold))
	}

	fmt.Printf("verdict: %s\n", verdictString(report.Verdict))
	if report.Verdict == overlap.VerdictBlock {
		return &overlap.ErrSameFileParallel{}
	}
	return nil
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

	inspector := gitcli.NewInspector(root)
	lister := wtcli.NewLister(*wtBin)
	guard := service.NewGuard(nil, lister, inspector, cfg)

	ctx := context.Background()
	report, err := guard.Metric(ctx)
	if err != nil {
		return err
	}

	if *jsonOut {
		return writeJSON(reportToJSON(report, *threshold))
	}

	fmt.Printf("collision_rate: %.4f  threshold: %.4f  over: %v\n",
		report.CollisionRate, *threshold, report.OverThreshold)

	if *strict && report.OverThreshold {
		return fmt.Errorf("collision rate %.4f exceeds threshold %.4f (--strict)", report.CollisionRate, *threshold)
	}
	return nil
}
