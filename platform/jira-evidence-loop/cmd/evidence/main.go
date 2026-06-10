// Command evidence is the composition root for the jira-evidence-loop module.
// It wires Config + client + service.EvidenceLoop and runs the selected steps
// for a given SDD phase. Credentials are sourced exclusively from environment
// variables (REQ-AUTH, Design §CLI surface).
//
// Usage:
//
//	evidence run-loop --change=<name> --phase=<phase> --agent=<agent> \
//	  --module=<module> --summary=<text> [--jira-key=<KEY>] \
//	  [--pr-url=<url>] [--attach=<path>] [--worklog-seconds=<n>] \
//	  [--dry-run]
//
// Environment variables:
//
//	JIRA_EMAIL        Jira account email (required unless --dry-run)
//	JIRA_API_TOKEN    Jira API token    (required unless --dry-run)
//	JIRA_SITE_URL     Jira site URL     (required)
//	EVIDENCE_DRY_RUN  Set to "1" to activate dry-run mode
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
	"time"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/adapter/dryrun"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/adapter/rest"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/internal/envfile"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/port"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "evidence: %v\n", err)
		os.Exit(1)
	}
}

// run is the testable entry point. It returns a non-nil error on any failure.
func run(args []string) error {
	// R8 / ADR-J3: load env files BEFORE any os.Getenv call or flag default
	// evaluation. Real environment wins (if-unset semantics); CI is unaffected.
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
		return fmt.Errorf("subcommand required: run-loop")
	}
	switch args[0] {
	case "run-loop":
		return cmdRunLoop(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: run-loop", args[0])
	}
}

// repoRoot returns the repository root via git, falling back to the working
// directory on error.
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

// ---------------------------------------------------------------------------
// run-loop subcommand
// ---------------------------------------------------------------------------

// runLoopFlags holds all parsed flags for the run-loop subcommand.
type runLoopFlags struct {
	// Identity / ownership
	Change  string
	Phase   string
	Agent   string
	Module  string
	JiraKey string

	// Issue content
	Summary         string
	DescriptionFile string
	CommentText     string
	PRURL           string
	AttachPath      string
	WorklogSeconds  int

	// Config override
	SiteURL string

	// Modes
	DryRun bool
}

// parseRunLoopFlags parses the run-loop flag set and validates required flags.
// It also checks the EVIDENCE_DRY_RUN environment variable as an alternative
// to --dry-run (Design §D6).
func parseRunLoopFlags(args []string) (runLoopFlags, error) {
	fs := flag.NewFlagSet("run-loop", flag.ContinueOnError)

	// Identity / ownership flags.
	change := fs.String("change", "", "Change identifier (required)")
	phase := fs.String("phase", "", "SDD phase: propose|spec|design|tasks|apply|verify|archive (required)")
	agent := fs.String("agent", "", "Agent name, e.g. hermes (required)")
	module := fs.String("module", "", "Module name, e.g. jira-loop (required)")
	jiraKey := fs.String("jira-key", "", "Jira issue key for phases that operate on an existing issue (e.g. TAL-42)")

	// Issue content flags.
	summary := fs.String("summary", "", "Issue summary / title (required)")
	descriptionFile := fs.String("description-file", "", "Path to ADF JSON description file (optional)")
	commentText := fs.String("comment", "", "Comment text for the comment step (optional)")
	prURL := fs.String("pr-url", "", "Pull-request URL for remote link (verify phase)")
	attachPath := fs.String("attach", "", "Path to file to attach as evidence (verify phase)")
	worklogSeconds := fs.Int("worklog-seconds", 0, "Worklog duration in seconds (0 = use Config default)")

	// Config overrides.
	siteURL := fs.String("site-url", "", "Jira site URL (overrides JIRA_SITE_URL env)")

	// Mode flags.
	dryRun := fs.Bool("dry-run", false, "Plan and log steps without calling Jira (also: EVIDENCE_DRY_RUN=1)")

	if err := fs.Parse(args); err != nil {
		return runLoopFlags{}, err
	}

	// EVIDENCE_DRY_RUN=1 activates dry-run even without the flag.
	if os.Getenv("EVIDENCE_DRY_RUN") == "1" {
		*dryRun = true
	}

	// Validate required flags.
	required := []struct{ name, val string }{
		{"change", *change},
		{"phase", *phase},
		{"agent", *agent},
		{"module", *module},
		{"summary", *summary},
	}
	for _, r := range required {
		if r.val == "" {
			return runLoopFlags{}, fmt.Errorf("flag --%s is required", r.name)
		}
	}

	return runLoopFlags{
		Change:          *change,
		Phase:           *phase,
		Agent:           *agent,
		Module:          *module,
		JiraKey:         *jiraKey,
		Summary:         *summary,
		DescriptionFile: *descriptionFile,
		CommentText:     *commentText,
		PRURL:           *prURL,
		AttachPath:      *attachPath,
		WorklogSeconds:  *worklogSeconds,
		SiteURL:         *siteURL,
		DryRun:          *dryRun,
	}, nil
}

func cmdRunLoop(args []string) error {
	flags, err := parseRunLoopFlags(args)
	if err != nil {
		return err
	}

	// --phase selects the step preset (Design §D3). Unknown phase = fail-loud.
	steps, err := service.PhasePreset(flags.Phase)
	if err != nil {
		return err
	}

	// Resolve site URL (flag overrides env).
	site := flags.SiteURL
	if site == "" {
		site = os.Getenv("JIRA_SITE_URL")
	}

	// Credentials from env only (REQ-AUTH).
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")

	// Credential / dry-run gate (Design §D6):
	//   - --dry-run or EVIDENCE_DRY_RUN=1 → DryRunClient; no token required.
	//   - Missing token without dry-run    → fail-loud (§7).
	var client port.JiraClient
	if flags.DryRun {
		client = dryrun.NewClient(os.Stdout)
	} else {
		if email == "" {
			return errors.New("JIRA_EMAIL environment variable is not set (use --dry-run for token-free execution)")
		}
		if token == "" {
			return errors.New("JIRA_API_TOKEN environment variable is not set (use --dry-run for token-free execution)")
		}
		if site == "" {
			return errors.New("Jira site URL is required: set --site-url or JIRA_SITE_URL env")
		}
	}

	// Build Config — seeded from TAL defaults; overridden by env/flags.
	cfg := service.DefaultTALConfig()
	if site != "" {
		cfg.SiteURL = site
	}
	cfg.Credentials = service.Credentials{Email: email, APIToken: token}

	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" {
		cfg.ProjectKey = v
	}
	if v := os.Getenv("JIRA_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}

	// In dry-run mode skip config validation (credentials are intentionally empty).
	if !flags.DryRun {
		validatedCfg, err := service.NewConfig(cfg)
		if err != nil {
			return fmt.Errorf("config validation: %w", err)
		}
		cfg = validatedCfg
		client = rest.NewClient(cfg)
	}

	// Build description ADF (optional file).
	var description evidence.ADFDocument
	if flags.DescriptionFile != "" {
		raw, err := os.ReadFile(flags.DescriptionFile)
		if err != nil {
			return fmt.Errorf("reading description file: %w", err)
		}
		if err := json.Unmarshal(raw, &description); err != nil {
			return fmt.Errorf("parsing description file as ADF JSON: %w", err)
		}
	} else {
		description = evidence.NewADFDocument(flags.Summary)
	}

	// Build comment ADF.
	var comment evidence.ADFDocument
	if flags.CommentText != "" {
		comment = evidence.NewADFDocument(flags.CommentText)
	} else {
		comment = evidence.NewADFDocument(fmt.Sprintf("Evidence loop executed for %s/%s by %s", flags.Change, flags.Phase, flags.Agent))
	}

	// Build attachment (optional).
	var att evidence.Attachment
	if flags.AttachPath != "" {
		data, err := os.ReadFile(flags.AttachPath)
		if err != nil {
			return fmt.Errorf("reading attachment file: %w", err)
		}
		att = evidence.Attachment{
			Filename:    pathBase(flags.AttachPath),
			ContentType: "application/octet-stream",
			Data:        data,
		}
	}

	// Worklog started = now (ISO-8601 / Jira format).
	started := time.Now().UTC().Format("2006-01-02T15:04:05.000+0000")

	// Ownership map: minimal map for the module→agent pair.
	ownershipMap := map[string]string{flags.Module: flags.Agent}

	in := service.RunInput{
		Module:         flags.Module,
		Agent:          flags.Agent,
		Change:         flags.Change,
		Phase:          flags.Phase,
		JiraKey:        flags.JiraKey,
		Summary:        flags.Summary,
		Description:    description,
		Comment:        comment,
		PRURL:          flags.PRURL,
		Attachment:     att,
		WorklogSeconds: flags.WorklogSeconds,
		WorklogStarted: started,
		OwnershipMap:   ownershipMap,
	}

	ctx := context.Background()
	issueKey, err := service.NewEvidenceLoop(client, cfg).RunSteps(ctx, in, steps)
	if err != nil {
		return fmt.Errorf("evidence loop failed (issue=%s): %w", issueKey, err)
	}

	mode := "live"
	if flags.DryRun {
		mode = "dry-run"
	}
	_, _ = fmt.Fprintf(dryRunOutput(flags.DryRun), "evidence loop complete [%s]: issue=%s phase=%s\n", mode, issueKey, flags.Phase)
	return nil
}

// dryRunOutput returns stdout for dry-run (so the plan is always visible) and
// stdout for live mode too. Kept as a helper for clarity.
func dryRunOutput(_ bool) io.Writer { return os.Stdout }

// pathBase returns the last path component (filename) of p.
func pathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
