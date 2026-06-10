// Package ovcli implements the OverlapChecker port by shelling out to the ov binary.
package ovcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/John-Santa/talos/platform/orchestration/domain/dispatch"
	"github.com/John-Santa/talos/platform/orchestration/internal/runner"
	"github.com/John-Santa/talos/platform/orchestration/port"
)

// ovScanOutput mirrors the JSON emitted by `ov check --json`.
type ovScanOutput struct {
	Verdict       string  `json:"verdict"`
	CollisionRate float64 `json:"collision_rate"`
}

// Checker implements port.OverlapChecker.
type Checker struct {
	binary string
	runner runner.Runner
}

var _ port.OverlapChecker = (*Checker)(nil)

// NewChecker constructs a Checker using the given ov binary name and runner.
func NewChecker(ovBinary string, r runner.Runner) *Checker {
	return &Checker{binary: ovBinary, runner: r}
}

// Check shells out to `ov check --module <m> --agent <a> --json` and returns the verdict.
func (c *Checker) Check(ctx context.Context, module, agent string) (dispatch.OverlapVerdict, error) {
	out, err := c.runner.Run(ctx, c.binary, "check", "--module", module, "--agent", agent, "--json")
	if err != nil {
		return "", fmt.Errorf("ovcli: ov check failed: %w", err)
	}

	var result ovScanOutput
	if err := json.Unmarshal(out, &result); err != nil {
		fragment := string(out)
		if len(fragment) > 80 {
			fragment = fragment[:80]
		}
		return "", fmt.Errorf("ovcli: malformed JSON from ov check (fragment: %q): %w", fragment, err)
	}

	return dispatch.OverlapVerdict(result.Verdict), nil
}
