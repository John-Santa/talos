// Package runscli implements port.RunRecorder by shelling out to the runs binary.
// All methods return errors to the caller; the Dispatcher wraps every call in a
// fail-soft helper (log + ignore), so these errors never abort a dispatch.
package runscli

import (
	"context"
	"fmt"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// Recorder implements port.RunRecorder via shell-out to the `runs` binary.
type Recorder struct {
	binary string
	runner runner.Runner
}

var _ port.RunRecorder = (*Recorder)(nil)

// NewRecorder constructs a Recorder using the given binary name/path and runner.
// binary defaults to "runs" in the PATH when the empty string is passed.
func NewRecorder(runsBinary string, r runner.Runner) *Recorder {
	if runsBinary == "" {
		runsBinary = "runs"
	}
	return &Recorder{binary: runsBinary, runner: r}
}

// RecordDispatch shells out to `runs record --kind dispatch --jira-key ... --status ...`.
func (r *Recorder) RecordDispatch(ctx context.Context, item dispatch.WorkItem, phase, status string) error {
	args := []string{
		"record",
		"--kind", "dispatch",
		"--jira-key", item.JiraKey,
		"--agent", item.Agent,
		"--change", item.Change,
		"--module", item.Module,
		"--phase", phase,
		"--status", status,
	}
	_, err := r.runner.Run(ctx, r.binary, args...)
	if err != nil {
		return fmt.Errorf("runscli: RecordDispatch: %w", err)
	}
	return nil
}

// RecordActivity shells out to `runs record --kind activity --jira-key ... --text ...`.
func (r *Recorder) RecordActivity(ctx context.Context, jiraKey, agent, text string) error {
	args := []string{
		"record",
		"--kind", "activity",
		"--jira-key", jiraKey,
		"--agent", agent,
		"--text", text,
	}
	_, err := r.runner.Run(ctx, r.binary, args...)
	if err != nil {
		return fmt.Errorf("runscli: RecordActivity: %w", err)
	}
	return nil
}

// RecordMetric shells out to `runs record --kind metric --jira-key ... --metric ... --value ...`.
func (r *Recorder) RecordMetric(ctx context.Context, jiraKey, name string, value float64) error {
	args := []string{
		"record",
		"--kind", "metric",
		"--jira-key", jiraKey,
		"--metric", name,
		"--value", fmt.Sprintf("%.6f", value),
	}
	_, err := r.runner.Run(ctx, r.binary, args...)
	if err != nil {
		return fmt.Errorf("runscli: RecordMetric: %w", err)
	}
	return nil
}
