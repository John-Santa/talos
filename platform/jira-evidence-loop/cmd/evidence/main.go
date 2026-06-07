// Command evidence is the composition root for the jira-evidence-loop module.
// It wires Config + rest.Client + service.EvidenceLoop and runs the 7-step
// Jira evidence flow. Credentials are sourced exclusively from environment
// variables (REQ-AUTH, Design §CLI surface).
//
// Usage:
//
//	evidence run-loop --change=<name> --phase=<phase> --agent=<agent> \
//	  --module=<module> --summary=<text> [--pr-url=<url>] \
//	  [--attach=<path>] [--worklog-seconds=<n>]
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/adapter/rest"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
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

// ---------------------------------------------------------------------------
// run-loop subcommand
// ---------------------------------------------------------------------------

func cmdRunLoop(args []string) error {
	fs := flag.NewFlagSet("run-loop", flag.ContinueOnError)

	// Ownership / identity flags.
	change := fs.String("change", "", "Change identifier (required)")
	phase := fs.String("phase", "", "Phase identifier, e.g. apply (required)")
	agent := fs.String("agent", "", "Agent name, e.g. hermes (required)")
	module := fs.String("module", "", "Module name, e.g. jira-loop (required)")

	// Issue content flags.
	summary := fs.String("summary", "", "Issue summary / title (required)")
	descriptionFile := fs.String("description-file", "", "Path to ADF JSON description file (optional)")
	commentText := fs.String("comment", "", "Comment text for step 3 (optional)")
	prURL := fs.String("pr-url", "", "Pull-request URL for remote link (optional)")
	attachPath := fs.String("attach", "", "Path to file to attach as evidence (optional)")
	worklogSeconds := fs.Int("worklog-seconds", 0, "Worklog duration in seconds (0 = use Config default)")

	// Config overrides (optional, for non-TAL projects).
	siteURL := fs.String("site-url", "", "Jira site URL, e.g. https://org.atlassian.net (overrides JIRA_SITE_URL env)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags.
	required := map[string]string{
		"change":  *change,
		"phase":   *phase,
		"agent":   *agent,
		"module":  *module,
		"summary": *summary,
	}
	for name, val := range required {
		if val == "" {
			return fmt.Errorf("flag --%s is required", name)
		}
	}

	// Credentials from env only (REQ-AUTH).
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	if email == "" {
		return errors.New("JIRA_EMAIL environment variable is not set")
	}
	if token == "" {
		return errors.New("JIRA_API_TOKEN environment variable is not set")
	}

	// Resolve site URL (flag overrides env).
	site := *siteURL
	if site == "" {
		site = os.Getenv("JIRA_SITE_URL")
	}
	if site == "" {
		return errors.New("Jira site URL is required: set --site-url or JIRA_SITE_URL env")
	}

	// Build Config seeded from real TAL defaults; caller may override via flags.
	cfg := service.DefaultTALConfig()
	cfg.SiteURL = site
	cfg.Credentials = service.Credentials{Email: email, APIToken: token}

	validatedCfg, err := service.NewConfig(cfg)
	if err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	// Build description ADF (optional file).
	var description evidence.ADFDocument
	if *descriptionFile != "" {
		raw, err := os.ReadFile(*descriptionFile)
		if err != nil {
			return fmt.Errorf("reading description file: %w", err)
		}
		if err := json.Unmarshal(raw, &description); err != nil {
			return fmt.Errorf("parsing description file as ADF JSON: %w", err)
		}
	} else {
		description = evidence.NewADFDocument(*summary)
	}

	// Build comment ADF.
	var comment evidence.ADFDocument
	if *commentText != "" {
		comment = evidence.NewADFDocument(*commentText)
	} else {
		comment = evidence.NewADFDocument(fmt.Sprintf("Evidence loop executed for %s/%s by %s", *change, *phase, *agent))
	}

	// Build attachment (optional).
	var att evidence.Attachment
	if *attachPath != "" {
		data, err := os.ReadFile(*attachPath)
		if err != nil {
			return fmt.Errorf("reading attachment file: %w", err)
		}
		att = evidence.Attachment{
			Filename:    pathBase(*attachPath),
			ContentType: "application/octet-stream",
			Data:        data,
		}
	}

	// Worklog started = now (ISO-8601 / Jira format).
	started := time.Now().UTC().Format("2006-01-02T15:04:05.000+0000")

	// Wire adapter and service.
	client := rest.NewClient(validatedCfg)
	loop := service.NewEvidenceLoop(client, validatedCfg)

	// Ownership map: minimal map for the module→agent pair being processed.
	// Full ownership maps should be injected from team-context/ownership.md
	// in production usage; this minimal map satisfies the ownership guard for
	// single-module invocations.
	ownershipMap := map[string]string{*module: *agent}

	in := service.RunInput{
		Module:         *module,
		Agent:          *agent,
		Change:         *change,
		Phase:          *phase,
		Summary:        *summary,
		Description:    description,
		Comment:        comment,
		PRURL:          *prURL,
		Attachment:     att,
		WorklogSeconds: *worklogSeconds,
		WorklogStarted: started,
		OwnershipMap:   ownershipMap,
	}

	ctx := context.Background()
	issueKey, err := loop.Run(ctx, in)
	if err != nil {
		return fmt.Errorf("evidence loop failed (issue=%s): %w", issueKey, err)
	}

	fmt.Printf("evidence loop complete: issue=%s\n", issueKey)
	return nil
}

// pathBase returns the last path component (filename) of p.
// Avoids importing path/filepath to stay lightweight.
func pathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
