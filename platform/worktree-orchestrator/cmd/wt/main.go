// Command wt is the composition root for the worktree-orchestrator module.
//
// Usage:
//
//	wt create <figura> <TAL-N> [--no-fetch]
//	wt list [--json]
//	wt teardown <figura> <TAL-N> [--force] [--delete-branch]
//	wt env <figura> <TAL-N>
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/John-Santa/talos/platform/worktree-orchestrator/adapter/gitcli"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/domain/worktree"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/internal/envfile"
	"github.com/John-Santa/talos/platform/worktree-orchestrator/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "wt: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string) error {
	// R8 / ADR-J3: load project.env BEFORE any os.Getenv evaluation.
	// wt touches no secrets — only project.env (tracked), not .env.
	_ = envfile.LoadInto(os.Setenv, os.Getenv, filepath.Join(repoRoot(), ".talos", "project.env"))

	if len(args) == 0 {
		return fmt.Errorf("subcommand required: create | list | teardown | env")
	}
	switch args[0] {
	case "create":
		return cmdCreate(args[1:])
	case "list":
		return cmdList(args[1:])
	case "teardown":
		return cmdTeardown(args[1:])
	case "env":
		return cmdEnv(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: create, list, teardown, env", args[0])
	}
}

func exitCodeFor(err error) int {
	if err == nil {
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

func newOrchestrator() *service.Orchestrator {
	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.Project = v
	}
	runner := gitcli.NewRunner(root)
	return service.NewOrchestrator(runner, cfg)
}

func cmdCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	noFetch := fs.Bool("no-fetch", false, "Skip git fetch before creating worktree")

	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) < 2 {
		return fmt.Errorf("create requires <figura> <TAL-N> [--no-fetch]; got %d positional arg(s)", len(positional))
	}

	figura := positional[0]
	jiraKey := positional[1]

	o := newOrchestrator()
	ctx := context.Background()
	return o.Create(ctx, figura, jiraKey, *noFetch)
}

type listEntry struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}

func parseFiguraFromBranch(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) == 3 && parts[0] == "agent" {
		return parts[1]
	}
	return branch
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON array")

	if err := fs.Parse(args); err != nil {
		return err
	}

	o := newOrchestrator()
	ctx := context.Background()
	statuses, err := o.List(ctx)
	if err != nil {
		return err
	}

	if *jsonOut {
		return renderListJSON(statuses)
	}
	return renderListTabular(statuses)
}

func renderListJSON(statuses []service.WorktreeStatus) error {
	entries := make([]listEntry, 0, len(statuses))
	for _, ws := range statuses {
		entries = append(entries, listEntry{
			Figura: parseFiguraFromBranch(ws.Info.Branch),
			Branch: ws.Info.Branch,
			Path:   ws.Info.Path,
			Head:   ws.Info.Head,
			Status: ws.Status,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func renderListTabular(statuses []service.WorktreeStatus) error {
	if len(statuses) == 0 {
		fmt.Println("no active agent worktrees")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIGURA\tBRANCH\tPATH\tHEAD\tSTATUS")
	for _, ws := range statuses {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			parseFiguraFromBranch(ws.Info.Branch),
			ws.Info.Branch,
			ws.Info.Path,
			ws.Info.Head,
			ws.Status,
		)
	}
	return w.Flush()
}

func cmdTeardown(args []string) error {
	fs := flag.NewFlagSet("teardown", flag.ContinueOnError)
	force := fs.Bool("force", false, "Remove dirty worktree (warns to stderr)")
	deleteBranch := fs.Bool("delete-branch", false, "Delete the branch after teardown (safe git branch -d)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) < 2 {
		return fmt.Errorf("teardown requires <figura> <TAL-N>; got %d positional arg(s)", len(positional))
	}

	figura := positional[0]
	jiraKey := positional[1]

	if *force {
		fmt.Fprintf(os.Stderr, "wt: --force: removing worktree even if dirty\n")
	}

	o := newOrchestrator()
	ctx := context.Background()
	err := o.Teardown(ctx, figura, jiraKey, *force, *deleteBranch)
	if err != nil {
		var dirty *worktree.ErrDirtyWorktree
		if errors.As(err, &dirty) {
			fmt.Fprintf(os.Stderr, "wt: worktree has uncommitted changes; re-run with --force to override\n")
		}
		return err
	}
	return nil
}

func cmdEnv(args []string) error {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) < 2 {
		return fmt.Errorf("env requires <figura> <TAL-N>; got %d positional arg(s)", len(positional))
	}

	figura := positional[0]
	jiraKey := positional[1]

	o := newOrchestrator()
	ctx := context.Background()
	return o.Env(ctx, figura, jiraKey)
}
