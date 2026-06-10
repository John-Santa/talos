// Command runs is the composition root for the runs module.
//
// Usage:
//
//	runs record --kind <dispatch|activity|judgment|metric> --jira-key K [--agent A]
//	            [--change C] [--phase P] [--module M] [--status S] [--outcome O]
//	            [--text TEXT] [--verdict V] [--judges "a,b"] [--fix-agent F]
//	            [--escalate-to E] [--metric M] [--value F] [--label L]
//	            [--dod-state S] [--at TS]
//	runs list   [--agent A] [--jira-key K] [--since DATE] [--json]
//	runs show   <jiraKey>
//	runs timeline --jira-key K [--agent A] [--json]
//	runs dod    --jira-key K [--json]
//	runs judgment --jira-key K [--json]
//
// TALOS_HOME overrides the default ~/.talos for all store operations.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/John-Santa/talos/platform/runs/adapter/jsonlstore"
	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/internal/taloshome"
	"github.com/John-Santa/talos/platform/runs/service"
)

// app holds the wired services for the CLI.
type app struct {
	recorder *service.Recorder
	querier  *service.Querier
	store    *jsonlstore.Store
}

func main() {
	a, err := wireApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "runs: %v\n", err)
		os.Exit(1)
	}
	if err := run2(context.Background(), os.Args[1:], a, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "runs: %v\n", err)
		os.Exit(1)
	}
}

// wireApp builds the app from the real adapters.
func wireApp() (*app, error) {
	home, err := taloshome.Dir()
	if err != nil {
		return nil, fmt.Errorf("resolving TALOS_HOME: %w", err)
	}
	store, err := jsonlstore.New(home)
	if err != nil {
		return nil, fmt.Errorf("initializing run store: %w", err)
	}
	return &app{
		recorder: service.NewRecorder(store),
		querier:  service.NewQuerier(store),
		store:    store,
	}, nil
}

// run2 is the testable entry point.
func run2(ctx context.Context, args []string, a *app, _ io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: record | list | show | timeline | dod | judgment")
	}
	switch args[0] {
	case "record":
		return cmdRecord(ctx, args[1:], a)
	case "list":
		return cmdList(ctx, args[1:], a, stdout)
	case "show":
		return cmdShow(ctx, args[1:], a, stdout)
	case "timeline":
		return cmdTimeline(ctx, args[1:], a, stdout)
	case "dod":
		return cmdDoD(ctx, args[1:], a, stdout)
	case "judgment":
		return cmdJudgment(ctx, args[1:], a, stdout)
	default:
		return fmt.Errorf("unknown subcommand %q; available: record, list, show, timeline, dod, judgment", args[0])
	}
}

// ---- record ----

func cmdRecord(ctx context.Context, args []string, a *app) error {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	kindFlag := fs.String("kind", "", "event kind: dispatch|activity|judgment|metric")
	jiraKey := fs.String("jira-key", "", "Jira issue key (e.g. TAL-42)")
	agent := fs.String("agent", "", "agent figura")
	change := fs.String("change", "", "change slug")
	phase := fs.String("phase", "", "SDD phase")
	module := fs.String("module", "", "module name")
	status := fs.String("status", "", "dispatch status (started|done|failed|queued)")
	outcome := fs.String("outcome", "", "outcome note")
	text := fs.String("text", "", "activity text")
	verdict := fs.String("verdict", "", "judgment verdict (APPROVED|REJECTED)")
	judges := fs.String("judges", "", "comma-separated judge IDs")
	fixAgent := fs.String("fix-agent", "", "agent assigned to fix after rejection")
	escalateTo := fs.String("escalate-to", "", "escalation target")
	metric := fs.String("metric", "", "metric name (conflict_rate|dod)")
	value := fs.Float64("value", 0, "metric value")
	label := fs.String("label", "", "DoD item label")
	dodState := fs.String("dod-state", "", "DoD item state (done|pending)")
	atFlag := fs.String("at", "", "event timestamp ISO-8601 (default: now)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	at := time.Now().UTC()
	if *atFlag != "" {
		var err error
		at, err = time.Parse(time.RFC3339, *atFlag)
		if err != nil {
			return fmt.Errorf("parsing --at %q: %w", *atFlag, err)
		}
	}

	e := run.RunEvent{
		V:          1,
		Kind:       run.Kind(*kindFlag),
		At:         at,
		JiraKey:    *jiraKey,
		Agent:      *agent,
		Change:     *change,
		Phase:      run.Phase(*phase),
		Module:     *module,
		Status:     *status,
		Outcome:    *outcome,
		Text:       *text,
		Verdict:    *verdict,
		FixAgent:   *fixAgent,
		EscalateTo: *escalateTo,
		Metric:     *metric,
		Value:      *value,
		Label:      *label,
		DoDState:   *dodState,
	}

	if *judges != "" {
		e.Judges = splitComma(*judges)
	}

	return a.recorder.Record(ctx, e)
}

// ---- list ----

func cmdList(ctx context.Context, args []string, a *app, stdout io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jiraKey := fs.String("jira-key", "", "filter by Jira issue key")
	agent := fs.String("agent", "", "filter by agent")
	since := fs.String("since", "", "filter events since date (RFC3339)")
	jsonOut := fs.Bool("json", false, "output as JSON array")

	if err := fs.Parse(args); err != nil {
		return err
	}

	f := run.Filter{JiraKey: *jiraKey, Agent: *agent}
	if *since != "" {
		t, err := time.Parse(time.RFC3339, *since)
		if err != nil {
			return fmt.Errorf("parsing --since %q: %w", *since, err)
		}
		f.Since = t
	}

	views, err := a.querier.Runs(ctx, f)
	if err != nil {
		return err
	}

	if *jsonOut {
		return json.NewEncoder(stdout).Encode(views)
	}

	if len(views) == 0 {
		fmt.Fprintln(stdout, "no runs found")
		return nil
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AT\tJIRA\tAGENT\tPHASE\tSTATUS")
	for _, v := range views {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			v.At.Format(time.RFC3339), v.JiraKey, v.Agent, v.Phase, v.Status)
	}
	return w.Flush()
}

// ---- show ----

func cmdShow(ctx context.Context, args []string, a *app, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("show requires <jiraKey>")
	}
	jiraKey := args[0]

	views, err := a.querier.Runs(ctx, run.Filter{JiraKey: jiraKey})
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "JiraKey: %s\n", jiraKey)
	if len(views) == 0 {
		fmt.Fprintln(stdout, "  (no dispatch events)")
		return nil
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  AT\tAGENT\tPHASE\tSTATUS\tOUTCOME")
	for _, v := range views {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
			v.At.Format(time.RFC3339), v.Agent, v.Phase, v.Status, v.Outcome)
	}
	return w.Flush()
}

// ---- timeline ----

func cmdTimeline(ctx context.Context, args []string, a *app, stdout io.Writer) error {
	fs := flag.NewFlagSet("timeline", flag.ContinueOnError)
	jiraKey := fs.String("jira-key", "", "Jira issue key (required)")
	agent := fs.String("agent", "", "filter by agent")
	jsonOut := fs.Bool("json", false, "output as JSON array of ActivityEntry")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jiraKey == "" {
		return fmt.Errorf("timeline requires --jira-key")
	}

	entries, err := a.querier.Timeline(ctx, *jiraKey, *agent)
	if err != nil {
		return err
	}

	if *jsonOut {
		return json.NewEncoder(stdout).Encode(entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(stdout, "no activity")
		return nil
	}
	for _, e := range entries {
		fmt.Fprintf(stdout, "%s  %s\n", e.At, e.Text)
	}
	return nil
}

// ---- dod ----

func cmdDoD(ctx context.Context, args []string, a *app, stdout io.Writer) error {
	fs := flag.NewFlagSet("dod", flag.ContinueOnError)
	jiraKey := fs.String("jira-key", "", "Jira issue key (required)")
	jsonOut := fs.Bool("json", false, "output as JSON array of DoDItem")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jiraKey == "" {
		return fmt.Errorf("dod requires --jira-key")
	}

	items, err := a.querier.DoD(ctx, *jiraKey)
	if err != nil {
		return err
	}

	if *jsonOut {
		return json.NewEncoder(stdout).Encode(items)
	}

	if len(items) == 0 {
		fmt.Fprintln(stdout, "no DoD items")
		return nil
	}
	for _, d := range items {
		mark := "[ ]"
		if d.State == "done" {
			mark = "[x]"
		}
		fmt.Fprintf(stdout, "%s %s (%s)\n", mark, d.Label, d.Kind)
	}
	return nil
}

// ---- judgment ----

func cmdJudgment(ctx context.Context, args []string, a *app, stdout io.Writer) error {
	fs := flag.NewFlagSet("judgment", flag.ContinueOnError)
	jiraKey := fs.String("jira-key", "", "Jira issue key (required)")
	jsonOut := fs.Bool("json", false, "output as JSON JudgmentReview")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jiraKey == "" {
		return fmt.Errorf("judgment requires --jira-key")
	}

	review, err := a.querier.Judgment(ctx, *jiraKey)
	if err != nil {
		return err
	}

	if *jsonOut {
		return json.NewEncoder(stdout).Encode(review)
	}

	if review.Pending {
		fmt.Fprintln(stdout, "Pending: no judgment recorded")
		return nil
	}
	fmt.Fprintf(stdout, "JiraKey:    %s\n", review.JiraKey)
	fmt.Fprintf(stdout, "Gate:       %s\n", review.Gate)
	fmt.Fprintf(stdout, "Verdict:    %s\n", review.Verdict)
	fmt.Fprintf(stdout, "FixAgent:   %s\n", review.FixAgent)
	if review.EscalateTo != "" {
		fmt.Fprintf(stdout, "EscalateTo: %s\n", review.EscalateTo)
	}
	for _, j := range review.Judges {
		fmt.Fprintf(stdout, "  Judge %s: %s  %s\n", j.ID, j.Verdict, j.Note)
	}
	return nil
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
