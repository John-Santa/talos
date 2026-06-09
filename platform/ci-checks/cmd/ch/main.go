// Command ch is the composition root for the ci-checks CLI.
//
// Usage:
//
//	ch labels          --branch BRANCH [--ownership-file F] [--site-url URL] [--json]
//	ch ownership       [--ownership-file F] [--json]
//	ch changed-modules [--json]  (reads changed file paths from stdin, one per line)
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
	"path/filepath"
	"strings"

	"github.com/John-Santa/talos/platform/ci-checks/adapter/jirarest"
	"github.com/John-Santa/talos/platform/ci-checks/adapter/ownershipfile"
	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
	"github.com/John-Santa/talos/platform/ci-checks/internal/envfile"
	"github.com/John-Santa/talos/platform/ci-checks/service"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ch: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string, out io.Writer) error {
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
		return fmt.Errorf("subcommand required: labels | ownership | changed-modules")
	}
	switch args[0] {
	case "labels":
		return cmdLabels(args[1:], out)
	case "ownership":
		return cmdOwnership(args[1:], out)
	case "changed-modules":
		return cmdChangedModules(os.Stdin, out, args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: labels, ownership, changed-modules", args[0])
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

// labelsJSON is the --json output shape for ch labels (REQ-JSON-1).
type labelsJSON struct {
	Branch     string   `json:"branch"`
	JiraKey    string   `json:"jira_key"`
	Figura     string   `json:"figura"`
	Verdict    string   `json:"verdict"`
	Labels     []string `json:"labels"`
	Agent      string   `json:"agent"`
	Module     string   `json:"module"`
	Violations []string `json:"violations"`
}

// ownershipJSON is the --json output shape for ch ownership.
type ownershipJSON struct {
	Modules map[string]string `json:"modules"`
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cmdLabels(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("labels", flag.ContinueOnError)
	branch := fs.String("branch", "", "PR branch name (required; GITHUB_HEAD_REF)")
	ownershipFile := fs.String("ownership-file", "", "Path to ownership.md (default: team-context/ownership.md relative to repo root)")
	siteURL := fs.String("site-url", os.Getenv("JIRA_SITE_URL"), "Jira site URL")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *branch == "" {
		return fmt.Errorf("labels requires --branch")
	}

	// CRITICAL: normalize only the figura segment to lowercase before ParseAgentBranch.
	// GITHUB_HEAD_REF may deliver uppercase figura (e.g. agent/HERMES/TAL-5).
	// The regex requires [a-z]+ for figura but <projectKey>-N must remain uppercase.
	normalizedBranch := normalizeBranchFigura(*branch)

	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")

	root := repoRoot()
	ownershipPath := *ownershipFile
	if ownershipPath == "" {
		ownershipPath = root + "/" + service.DefaultTALConfig().OwnershipPath
	}

	labelsClient := jirarest.NewClient(*siteURL, email, token)
	ownershipReader := ownershipfile.NewReader(ownershipPath)
	cfg := service.DefaultTALConfig()

	// Override non-secret project identity from env (populated by
	// .talos/project.env or inherited environment; REQ-IDENT).
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.Project = v
	}

	checker := service.NewChecker(cfg, labelsClient, ownershipReader)

	ctx := context.Background()
	result, checkErr := checker.Check(ctx, normalizedBranch)

	if *jsonOut {
		figura, jiraKey := extractBranchParts(normalizedBranch, cfg.Project)
		rawLabels := extractLabels(ctx, labelsClient, jiraKey)

		payload := buildLabelsJSON(*branch, jiraKey, figura, result, rawLabels, checkErr)
		if err := writeJSON(out, payload); err != nil {
			return err
		}
		return checkErr
	}

	if checkErr != nil {
		return checkErr
	}
	fmt.Fprintf(out, "verdict: OK\n")
	return nil
}

// normalizeBranchFigura lowercases only the figura segment of an agent/<figura>/<projectKey>-N branch.
// The Jira key segment is left unchanged — the domain regex requires uppercase project key.
func normalizeBranchFigura(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) == 3 && parts[0] == "agent" {
		parts[1] = strings.ToLower(parts[1])
		return strings.Join(parts, "/")
	}
	return strings.ToLower(branch)
}

// buildLabelsJSON constructs the JSON output from check results.
func buildLabelsJSON(branch, jiraKey, figura string, result cichecks.InvariantResult, rawLabels []string, checkErr error) labelsJSON {
	verdict := "OK"
	violations := result.Violations
	if violations == nil {
		violations = []string{}
	}

	var labelErr *cichecks.ErrLabelInvariant
	if errors.As(checkErr, &labelErr) {
		verdict = "VIOLATION"
		violations = labelErr.Violations
	} else if checkErr != nil {
		verdict = "VIOLATION"
		violations = []string{checkErr.Error()}
	}

	if rawLabels == nil {
		rawLabels = []string{}
	}

	ls := cichecks.ParseLabels(rawLabels)
	agentVal, _ := ls.Get("agent")
	moduleVal, _ := ls.Get("module")

	return labelsJSON{
		Branch:     branch,
		JiraKey:    jiraKey,
		Figura:     figura,
		Verdict:    verdict,
		Labels:     rawLabels,
		Agent:      agentVal,
		Module:     moduleVal,
		Violations: violations,
	}
}

// extractBranchParts returns (figura, jiraKey) from a normalized branch name
// using the caller-supplied projectKey for the domain regex.
// If parsing fails, returns empty strings.
func extractBranchParts(branch, projectKey string) (figura, key string) {
	f, k, err := cichecks.ParseAgentBranch(branch, projectKey)
	if err != nil {
		return "", ""
	}
	return f, k
}

// extractLabels fetches labels from Jira, returning nil on any error.
func extractLabels(ctx context.Context, reader interface {
	LabelsByKey(context.Context, string) ([]string, error)
}, key string) []string {
	if key == "" {
		return nil
	}
	labels, err := reader.LabelsByKey(ctx, key)
	if err != nil {
		return nil
	}
	return labels
}

// cmdChangedModules reads changed file paths from r (one per line), detects which
// platform/<module> roots were touched, and prints each on its own line.
// With --json it prints a JSON array instead. Exit 0 always.
func cmdChangedModules(r io.Reader, out io.Writer, args []string) error {
	fs := flag.NewFlagSet("changed-modules", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Output as JSON array")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var paths []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			paths = append(paths, line)
		}
	}

	modules := cichecks.ModulesFromChangedPaths(paths)

	if *jsonOut {
		return writeJSON(out, modules)
	}
	for _, m := range modules {
		fmt.Fprintln(out, m)
	}
	return nil
}

func cmdOwnership(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("ownership", flag.ContinueOnError)
	ownershipFile := fs.String("ownership-file", "", "Path to ownership.md (default: team-context/ownership.md relative to repo root)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := repoRoot()
	ownershipPath := *ownershipFile
	if ownershipPath == "" {
		ownershipPath = root + "/" + service.DefaultTALConfig().OwnershipPath
	}

	reader := ownershipfile.NewReader(ownershipPath)
	ctx := context.Background()
	m, err := reader.Ownership(ctx)
	if err != nil {
		return fmt.Errorf("ownership: %w", err)
	}

	if *jsonOut {
		return writeJSON(out, ownershipJSON{Modules: m})
	}

	for k, v := range m {
		fmt.Fprintf(out, "%s → %s\n", k, v)
	}
	return nil
}
