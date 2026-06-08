// Command mo is the composition root for the merge-order-orchestrator module.
//
// Usage:
//
//	mo plan   [--json] [--base develop] [--wt-bin wt] [--depends A:B] [--depends-file f] [--no-fetch]
//	mo execute [--base develop] [--max-conflict-rate 0.15] [--yes] [--no-fetch]
//	mo check  <branch> [--base develop] [--no-fetch]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/John-Santa/talos/platform/merge-order-orchestrator/adapter/gitcli"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/adapter/wtcli"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/domain/mergeorder"
	"github.com/John-Santa/talos/platform/merge-order-orchestrator/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mo: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: plan | execute | check")
	}
	switch args[0] {
	case "plan":
		return cmdPlan(args[1:])
	case "execute":
		return cmdExecute(args[1:])
	case "check":
		return cmdCheck(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: plan, execute, check", args[0])
	}
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var nc *mergeorder.ErrNoCandidates
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

// dependsValue is a repeatable --depends flag value.
type dependsValue []string

func (d *dependsValue) String() string { return strings.Join(*d, ",") }
func (d *dependsValue) Set(v string) error {
	*d = append(*d, v)
	return nil
}

// parseDependsEdge parses a single "A:B" dependency edge.
func parseDependsEdge(s string) (branch, needs string, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid depends edge %q: expected branch:needs-branch", s)
	}
	return parts[0], parts[1], nil
}

// parseDependsFileContent parses a depends-file: one edge per line "branch:needs-branch".
// Blank lines and lines starting with # are ignored.
func parseDependsFileContent(content string) (map[string][]string, error) {
	deps := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		a, b, err := parseDependsEdge(line)
		if err != nil {
			return nil, err
		}
		deps[a] = append(deps[a], b)
	}
	return deps, scanner.Err()
}

func buildDeps(dependsEdges []string, dependsFile string) (map[string][]string, error) {
	deps := make(map[string][]string)
	for _, edge := range dependsEdges {
		a, b, err := parseDependsEdge(edge)
		if err != nil {
			return nil, err
		}
		deps[a] = append(deps[a], b)
	}
	if dependsFile != "" {
		data, err := os.ReadFile(dependsFile)
		if err != nil {
			return nil, fmt.Errorf("reading depends-file %q: %w", dependsFile, err)
		}
		fileDeps, err := parseDependsFileContent(string(data))
		if err != nil {
			return nil, fmt.Errorf("parsing depends-file %q: %w", dependsFile, err)
		}
		for k, vs := range fileDeps {
			deps[k] = append(deps[k], vs...)
		}
	}
	return deps, nil
}

// planJSON is the --json output shape for mo plan.
type planJSON struct {
	BaseBranch      string         `json:"base_branch"`
	BaseTip         string         `json:"base_tip"`
	ConflictRate    float64        `json:"conflict_rate"`
	Threshold       float64        `json:"threshold"`
	SegmentationBad bool           `json:"segmentation_bad"`
	Steps           []planStepJSON `json:"steps"`
}

type planStepJSON struct {
	Position       int      `json:"position"`
	Branch         string   `json:"branch"`
	Figura         string   `json:"figura"`
	CommitsAhead   int      `json:"commits_ahead"`
	PredictedClean bool     `json:"predicted_clean"`
	ConflictFiles  []string `json:"conflict_files,omitempty"`
}

func cmdPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	base := fs.String("base", "develop", "Integration base branch")
	wtBin := fs.String("wt-bin", "wt", "wt binary name on PATH")
	noFetch := fs.Bool("no-fetch", false, "Skip initial fetch")
	dependsFile := fs.String("depends-file", "", "File with one branch:needs-branch edge per line")
	var dependsEdges dependsValue
	fs.Var(&dependsEdges, "depends", "Dependency edge A:B (repeatable)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	deps, err := buildDeps(dependsEdges, *dependsFile)
	if err != nil {
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
	planner := service.NewPlanner(inspector, lister, cfg)

	ctx := context.Background()
	report, err := planner.Plan(ctx, deps)
	if err != nil {
		return err
	}

	if *jsonOut {
		return renderPlanJSONWithThreshold(report, cfg.MaxConflictRate)
	}
	return renderPlanTabular(report)
}

func renderPlanJSONWithThreshold(report mergeorder.PlanReport, threshold float64) error {
	out := planJSON{
		BaseBranch:      report.Plan.BaseBranch,
		BaseTip:         report.Plan.BaseTip,
		ConflictRate:    report.ConflictRate,
		Threshold:       threshold,
		SegmentationBad: report.SegmentationBad,
	}
	for _, step := range report.Plan.Steps {
		out.Steps = append(out.Steps, planStepJSON{
			Position:       step.Position,
			Branch:         step.Candidate.Branch,
			Figura:         step.Candidate.Figura,
			CommitsAhead:   step.Candidate.CommitsAhead,
			PredictedClean: step.PredictedClean,
			ConflictFiles:  step.ConflictFiles,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderPlanTabular(report mergeorder.PlanReport) error {
	if len(report.Plan.Steps) == 0 {
		fmt.Println("no ready branches to merge")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Base: %s @ %s\n", report.Plan.BaseBranch, report.Plan.BaseTip)
	fmt.Fprintf(w, "Conflict rate: %.4f  Segmentation bad: %v\n\n", report.ConflictRate, report.SegmentationBad)
	fmt.Fprintln(w, "#\tBRANCH\tFIGURA\tAHEAD\tCLEAN\tCONFLICTS")
	for _, step := range report.Plan.Steps {
		clean := "yes"
		if !step.PredictedClean {
			clean = "NO"
		}
		conflicts := strings.Join(step.ConflictFiles, " ")
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n",
			step.Position,
			step.Candidate.Branch,
			step.Candidate.Figura,
			step.Candidate.CommitsAhead,
			clean,
			conflicts,
		)
	}
	return w.Flush()
}

// printConflictRecipe writes the §11 mid-task-failure three-step recipe to w.
func printConflictRecipe(w io.Writer, branch, figura string, conflictFiles []string) {
	fmt.Fprintf(w, "\nMerge halted — conflict detected on branch %q\n", branch)
	if len(conflictFiles) > 0 {
		fmt.Fprintf(w, "Conflicting files: %s\n", strings.Join(conflictFiles, ", "))
	}
	fmt.Fprintln(w, "\n§11 mid-task-failure recipe:")
	fmt.Fprintln(w, "  1. Transition the Jira issue back to To Do")
	fmt.Fprintln(w, "  2. Add a comment on the issue with the conflict details above")
	fmt.Fprintf(w, "  3. Discard the worktree: wt teardown --force %s\n", figura)
}

func cmdExecute(args []string) error {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	base := fs.String("base", "develop", "Integration base branch")
	maxRate := fs.Float64("max-conflict-rate", 0.15, "Max conflict rate threshold")
	yes := fs.Bool("yes", false, "Confirm execution (required)")
	noFetch := fs.Bool("no-fetch", false, "Skip initial fetch")
	dependsFile := fs.String("depends-file", "", "File with one branch:needs-branch edge per line")
	var dependsEdges dependsValue
	fs.Var(&dependsEdges, "depends", "Dependency edge A:B (repeatable)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*yes {
		return fmt.Errorf("--yes is required to execute; use 'mo execute --yes' to confirm")
	}

	deps, err := buildDeps(dependsEdges, *dependsFile)
	if err != nil {
		return err
	}

	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	cfg.BaseBranch = *base
	cfg.MaxConflictRate = *maxRate

	inspector := gitcli.NewInspector(root)
	integrator := gitcli.NewIntegrator(root)
	lister := wtcli.NewLister(cfg.WtBinary)
	runner := service.NewIntegrationRunner(inspector, integrator, lister, cfg)

	ctx := context.Background()
	err = runner.Execute(ctx, service.ExecuteOptions{
		Deps:      deps,
		NoFetch:   *noFetch,
		Confirmed: true,
	})
	if err != nil {
		var mc *mergeorder.ErrMergeConflict
		var rc *mergeorder.ErrRebaseConflict
		switch {
		case errors.As(err, &mc):
			printConflictRecipe(os.Stderr, mc.Branch, figuraFromBranch(mc.Branch), mc.Files)
		case errors.As(err, &rc):
			printConflictRecipe(os.Stderr, rc.Branch, figuraFromBranch(rc.Branch), rc.Files)
		}
	}
	return err
}

// figuraFromBranch extracts the figura segment from an agent branch name.
// For "agent/<figura>/TAL-N" it returns "<figura>"; for other shapes it returns the branch as-is.
func figuraFromBranch(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) == 3 && parts[0] == "agent" {
		return parts[1]
	}
	return branch
}

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	base := fs.String("base", "develop", "Integration base branch")
	noFetch := fs.Bool("no-fetch", false, "Skip initial fetch")

	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) == 0 {
		return fmt.Errorf("check requires a branch argument")
	}
	branch := positional[0]

	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	cfg.BaseBranch = *base
	cfg.NoFetch = *noFetch

	inspector := gitcli.NewInspector(root)
	lister := wtcli.NewLister(cfg.WtBinary)
	planner := service.NewPlanner(inspector, lister, cfg)

	ctx := context.Background()
	step, err := planner.Check(ctx, branch)
	if err != nil {
		return err
	}

	if step.PredictedClean {
		fmt.Printf("branch %q: clean (no predicted conflicts against %s)\n", branch, *base)
	} else {
		fmt.Printf("branch %q: CONFLICTS predicted against %s:\n", branch, *base)
		for _, f := range step.ConflictFiles {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}
