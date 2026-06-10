// Package mocli implements the MergeCoordinator port by shelling out to the mo binary.
package mocli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// moPlanOutput mirrors the JSON emitted by `mo plan --json`.
type moPlanOutput struct {
	ConflictRate    float64 `json:"conflict_rate"`
	Threshold       float64 `json:"threshold"`
	SegmentationBad bool    `json:"segmentation_bad"`
}

// Coordinator implements port.MergeCoordinator.
type Coordinator struct {
	binary string
	runner runner.Runner
}

var _ port.MergeCoordinator = (*Coordinator)(nil)

// NewCoordinator constructs a Coordinator using the given mo binary name and runner.
func NewCoordinator(moBinary string, r runner.Runner) *Coordinator {
	return &Coordinator{binary: moBinary, runner: r}
}

// Plan shells out to `mo plan --json` and returns a MergePlan.
// Clean is true when conflict_rate == 0 and segmentation_bad is false.
func (c *Coordinator) Plan(ctx context.Context) (port.MergePlan, error) {
	out, err := c.runner.Run(ctx, c.binary, "plan", "--json")
	if err != nil {
		return port.MergePlan{}, fmt.Errorf("mocli: mo plan failed: %w", err)
	}

	var result moPlanOutput
	if err := json.Unmarshal(out, &result); err != nil {
		fragment := string(out)
		if len(fragment) > 80 {
			fragment = fragment[:80]
		}
		return port.MergePlan{}, fmt.Errorf("mocli: malformed JSON from mo plan (fragment: %q): %w", fragment, err)
	}

	clean := result.ConflictRate == 0 && !result.SegmentationBad
	return port.MergePlan{
		ConflictRate: result.ConflictRate,
		Threshold:    result.Threshold,
		Clean:        clean,
	}, nil
}

// Execute shells out to `mo execute --yes` to integrate branches in merge order.
func (c *Coordinator) Execute(ctx context.Context) error {
	_, err := c.runner.Run(ctx, c.binary, "execute", "--yes")
	if err != nil {
		return fmt.Errorf("mocli: mo execute failed: %w", err)
	}
	return nil
}
