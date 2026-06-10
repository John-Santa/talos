// Command ws is the composition root for the workspaces module.
//
// Usage:
//
//	ws list [--json]
//	ws add <name> --repo <path> --site <url> --project-key <KEY> --project-id <id> \
//	         --issue-type <name> --state-new <status,...> --state-doing <status,...> --state-done <status,..>
//	ws use <name>
//	ws show <name>
//	ws current
//	ws remove <name> [--purge-creds]
//
// Secrets (email and API token) are read from stdin on `ws add`, never from flags.
// TALOS_HOME overrides the default ~/.talos for all store and vault operations.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/John-Santa/talos/platform/workspaces/adapter/envmaterializer"
	"github.com/John-Santa/talos/platform/workspaces/adapter/filevault"
	"github.com/John-Santa/talos/platform/workspaces/adapter/gitprobe"
	"github.com/John-Santa/talos/platform/workspaces/adapter/tomlstore"
	"github.com/John-Santa/talos/platform/workspaces/domain/workspace"
	"github.com/John-Santa/talos/platform/workspaces/internal/taloshome"
	"github.com/John-Santa/talos/platform/workspaces/port"
	"github.com/John-Santa/talos/platform/workspaces/service"
)

func main() {
	mgr, err := wireManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ws: %v\n", err)
		os.Exit(1)
	}
	if err := run(context.Background(), os.Args[1:], mgr, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ws: %v\n", err)
		os.Exit(1)
	}
}

// wireManager builds the Manager from the real adapters.
func wireManager() (*service.Manager, error) {
	home, err := taloshome.Dir()
	if err != nil {
		return nil, fmt.Errorf("resolving TALOS_HOME: %w", err)
	}

	store, err := tomlstore.New(home)
	if err != nil {
		return nil, fmt.Errorf("initializing workspace store: %w", err)
	}

	vault, err := filevault.New(home)
	if err != nil {
		return nil, fmt.Errorf("initializing credential vault: %w", err)
	}

	probe := gitprobe.New()
	mat := envmaterializer.New()

	return service.NewManager(store, vault, probe, mat), nil
}

// run is the testable entry point.
func run(ctx context.Context, args []string, mgr *service.Manager, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: list | add | use | show | current | remove")
	}
	switch args[0] {
	case "list":
		return cmdList(ctx, args[1:], mgr, stdout)
	case "add":
		return cmdAdd(ctx, args[1:], mgr, stdin, stdout)
	case "use":
		return cmdUse(ctx, args[1:], mgr)
	case "show":
		return cmdShow(ctx, args[1:], mgr, stdout)
	case "current":
		return cmdCurrent(ctx, args[1:], mgr, stdout)
	case "remove":
		return cmdRemove(ctx, args[1:], mgr)
	default:
		return fmt.Errorf("unknown subcommand %q; available: list, add, use, show, current, remove", args[0])
	}
}

// ---- list ----

func cmdList(ctx context.Context, args []string, mgr *service.Manager, stdout io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "output as JSON array")
	if err := fs.Parse(args); err != nil {
		return err
	}

	views, err := mgr.List(ctx)
	if err != nil {
		return err
	}

	if *jsonOut {
		return json.NewEncoder(stdout).Encode(views)
	}

	if len(views) == 0 {
		fmt.Fprintln(stdout, "no workspaces registered")
		return nil
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tREPO\tSITE\tACTIVE")
	for _, v := range views {
		active := ""
		if v.Active {
			active = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", v.Name, v.RepoPath, v.Jira.SiteURL, active)
	}
	return w.Flush()
}

// ---- add ----

func cmdAdd(ctx context.Context, args []string, mgr *service.Manager, stdin io.Reader, stdout io.Writer) error {
	// The first positional arg is always the workspace name.
	// All flags follow after the name (or before — we skip the name and parse the rest).
	if len(args) == 0 {
		return fmt.Errorf("add requires <name> [--flags...]")
	}
	name := args[0]
	rest := args[1:]

	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	repo := fs.String("repo", "", "absolute path to the git repository")
	site := fs.String("site", "", "Jira site URL (e.g. https://acme.atlassian.net)")
	projKey := fs.String("project-key", "", "Jira project key (e.g. TAL)")
	projID := fs.String("project-id", "", "Jira project ID (numeric)")
	issueType := fs.String("issue-type", "", "Jira issue type name (e.g. Story)")
	stateNew := fs.String("state-new", "", "comma-separated Jira statuses for 'new' category")
	stateDoing := fs.String("state-doing", "", "comma-separated Jira statuses for 'indeterminate' category")
	stateDone := fs.String("state-done", "", "comma-separated Jira statuses for 'done' category")

	if err := fs.Parse(rest); err != nil {
		return err
	}

	binding := workspace.JiraBinding{
		SiteURL:       *site,
		ProjectKey:    *projKey,
		ProjectID:     *projID,
		IssueTypeName: *issueType,
		StateMapping: map[workspace.StatusCategory][]string{
			workspace.CategoryNew:           splitComma(*stateNew),
			workspace.CategoryIndeterminate: splitComma(*stateDoing),
			workspace.CategoryDone:          splitComma(*stateDone),
		},
	}

	// Read secrets from stdin — never from flags (REQ-AUTH: secrets must not appear in argv/history).
	fmt.Fprint(stdout, "Jira email: ")
	email := readLine(stdin)
	fmt.Fprint(stdout, "Jira API token: ")
	apiToken := readLine(stdin)

	creds := port.Credentials{Email: email, APIToken: apiToken}

	if err := mgr.Add(ctx, name, *repo, binding, creds); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "workspace %q registered\n", name)
	return nil
}

// ---- use ----

func cmdUse(ctx context.Context, args []string, mgr *service.Manager) error {
	if len(args) == 0 {
		return fmt.Errorf("use requires <name>")
	}
	name := args[0]
	return mgr.Use(ctx, name)
}

// ---- show ----

func cmdShow(ctx context.Context, args []string, mgr *service.Manager, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("show requires <name>")
	}
	name := args[0]

	view, err := mgr.Show(ctx, name)
	if err != nil {
		return err
	}

	active := ""
	if view.Active {
		active = " (active)"
	}
	creds := "not stored"
	if view.HasCredentials {
		creds = "stored"
	}

	fmt.Fprintf(stdout, "Name:        %s%s\n", view.Name, active)
	fmt.Fprintf(stdout, "Repo:        %s\n", view.RepoPath)
	fmt.Fprintf(stdout, "Site:        %s\n", view.Jira.SiteURL)
	fmt.Fprintf(stdout, "Project:     %s (%s)\n", view.Jira.ProjectKey, view.Jira.ProjectID)
	fmt.Fprintf(stdout, "Issue type:  %s\n", view.Jira.IssueTypeName)
	fmt.Fprintf(stdout, "Credentials: %s\n", creds)
	return nil
}

// ---- current ----

func cmdCurrent(ctx context.Context, _ []string, mgr *service.Manager, stdout io.Writer) error {
	name, err := mgr.Current(ctx)
	if err != nil {
		return err
	}
	if name == "" {
		fmt.Fprintln(stdout, "(none)")
		return nil
	}
	fmt.Fprintln(stdout, name)
	return nil
}

// ---- remove ----

func cmdRemove(ctx context.Context, args []string, mgr *service.Manager) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	purgeCreds := fs.Bool("purge-creds", false, "also delete stored credentials")
	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) == 0 {
		return fmt.Errorf("remove requires <name>")
	}
	name := positional[0]
	return mgr.Remove(ctx, name, *purgeCreds)
}

// ---- helpers ----

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func readLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// makeOSCmd is a thin wrapper around exec.Command to allow test injection.
// In tests, main_test.go's cmdHelper calls this for git init.
func makeOSCmd(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
