// Command orch is the composition root for the orchestration module.
// It wires all adapters and dispatches SDD work-items via the Dispatcher service.
//
// Usage:
//
//	orch dispatch --change=<name> --phase=<phase> --agent=<figura> --module=<module>
//	              [--jira-key=<KEY>] [--summary=<text>] [--pr-url=<url>]
//	              [--attach=<path>] [--confirm-merge] [--dry-run]
//	              [--ov-bin=ov] [--wt-bin=wt] [--mo-bin=mo] [--evidence-bin=evidence]
//
//	orch status   [--ov-bin=ov] [--wt-bin=wt] [--mo-bin=mo]
//
// Environment variables (loaded from .talos/project.env + .env):
//
//	JIRA_EMAIL, JIRA_API_TOKEN, JIRA_SITE_URL — forwarded to evidence binary
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/John-Santa/talos/platform/orchestration/adapter/evidencecli"
	"github.com/John-Santa/talos/platform/orchestration/adapter/mocli"
	"github.com/John-Santa/talos/platform/orchestration/adapter/ovcli"
	"github.com/John-Santa/talos/platform/orchestration/adapter/runscli"
	"github.com/John-Santa/talos/platform/orchestration/adapter/wtcli"
	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
	"github.com/John-Santa/talos/platform/orchestration/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "orch: %v\n", err)
		os.Exit(1)
	}
}

// run is the testable entry point.
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: dispatch | status")
	}
	switch args[0] {
	case "dispatch":
		return cmdDispatch(args[1:])
	case "status":
		return cmdStatus(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: dispatch, status", args[0])
	}
}

// cmdDispatch implements the `orch dispatch` subcommand.
func cmdDispatch(args []string) error {
	fs := flag.NewFlagSet("dispatch", flag.ContinueOnError)

	change := fs.String("change", "", "SDD change name (required)")
	phase := fs.String("phase", "", "SDD phase: propose|spec|design|tasks|apply|verify|archive (required)")
	agent := fs.String("agent", "", "Agent figura (required)")
	module := fs.String("module", "", "Module name e.g. module:orchestration (required)")
	jiraKey := fs.String("jira-key", "", "Jira issue key (e.g. TAL-42); optional for propose")
	summary := fs.String("summary", "", "Summary text forwarded to evidence")
	prURL := fs.String("pr-url", "", "PR URL forwarded to evidence")
	attach := fs.String("attach", "", "File path to attach (forwarded to evidence)")
	confirmMerge := fs.Bool("confirm-merge", false, "Execute mo merge after clean gate (destructive)")
	dryRun := fs.Bool("dry-run", false, "Plan without executing (forwarded to evidence)")
	ovBin := fs.String("ov-bin", "ov", "Path or name of the ov binary")
	wtBin := fs.String("wt-bin", "wt", "Path or name of the wt binary")
	moBin := fs.String("mo-bin", "mo", "Path or name of the mo binary")
	evidenceBin := fs.String("evidence-bin", "evidence", "Path or name of the evidence binary")
	runsBin := fs.String("runs-bin", "runs", "Path or name of the runs binary (empty = disable recorder)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if *change == "" {
		return fmt.Errorf("dispatch: --change is required")
	}
	if *phase == "" {
		return fmt.Errorf("dispatch: --phase is required")
	}
	if *agent == "" {
		return fmt.Errorf("dispatch: --agent is required")
	}
	if *module == "" {
		return fmt.Errorf("dispatch: --module is required")
	}

	r := runner.New()

	wtMgr := wtcli.NewManager(*wtBin, r)
	ovChecker := ovcli.NewChecker(*ovBin, r)
	evRunner := evidencecli.NewRunner(*evidenceBin, r)
	moCoord := mocli.NewCoordinator(*moBin, r)
	rbCoord := evidencecli.NewRollbacker(*evidenceBin, *wtBin, r)

	// RunRecorder: wire real adapter; in --dry-run use nil (no-op).
	var rec port.RunRecorder
	if !*dryRun && *runsBin != "" {
		rec = runscli.NewRecorder(*runsBin, r)
	}

	cfg := service.DefaultConfig()
	cfg.ConfirmMerge = *confirmMerge

	d := service.NewDispatcher(wtMgr, ovChecker, evRunner, moCoord, rbCoord, rec, cfg)

	item := dispatch.WorkItem{
		JiraKey: *jiraKey,
		Change:  *change,
		Agent:   *agent,
		Module:  *module,
	}
	evArgs := port.EvidenceArgs{
		Summary:    *summary,
		PRUrl:      *prURL,
		AttachPath: *attach,
		DryRun:     *dryRun,
	}

	ctx := context.Background()
	result, err := d.Dispatch(ctx, item, dispatch.Phase(*phase), evArgs)
	if err != nil {
		return err
	}

	fmt.Printf("orch dispatch: done — issue=%s worktree=%s\n", result.IssueKey, result.WorktreePath)
	return nil
}

// cmdStatus implements the `orch status` subcommand.
// Runs ov scan, wt list, and mo plan to produce a composite status view.
func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	ovBin := fs.String("ov-bin", "ov", "Path or name of the ov binary")
	wtBin := fs.String("wt-bin", "wt", "Path or name of the wt binary")
	moBin := fs.String("mo-bin", "mo", "Path or name of the mo binary")

	if err := fs.Parse(args); err != nil {
		return err
	}

	r := runner.New()
	var errs []string

	// wt list
	wtOut, err := r.Run(context.Background(), *wtBin, "list")
	if err != nil {
		errs = append(errs, fmt.Sprintf("wt list: %v", err))
	} else {
		fmt.Printf("=== Worktrees ===\n%s\n", strings.TrimSpace(string(wtOut)))
	}

	// ov scan
	ovOut, err := r.Run(context.Background(), *ovBin, "scan", "--json")
	if err != nil {
		errs = append(errs, fmt.Sprintf("ov scan: %v", err))
	} else {
		fmt.Printf("=== Overlap ===\n%s\n", strings.TrimSpace(string(ovOut)))
	}

	// mo plan
	moOut, err := r.Run(context.Background(), *moBin, "plan", "--json")
	if err != nil {
		errs = append(errs, fmt.Sprintf("mo plan: %v", err))
	} else {
		fmt.Printf("=== Merge Order ===\n%s\n", strings.TrimSpace(string(moOut)))
	}

	if len(errs) > 0 {
		return fmt.Errorf("orch status: %s", strings.Join(errs, "; "))
	}
	return nil
}

// repoRoot returns the git repository root, falling back to the working directory.
func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}

// envFilePaths returns the standard env file paths for this repo.
func envFilePaths() []string {
	root := repoRoot()
	return []string{
		filepath.Join(root, ".talos", "project.env"),
		filepath.Join(root, ".env"),
	}
}
