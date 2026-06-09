package service

import (
	"context"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
	"github.com/John-Santa/talos/platform/ci-checks/port"
)

// Checker orchestrates the label invariant check for a single PR branch.
type Checker struct {
	cfg      Config
	labels   port.IssueLabelReader
	ownership port.OwnershipReader
}

// NewChecker creates a Checker with the given configuration and ports.
func NewChecker(cfg Config, labels port.IssueLabelReader, ownership port.OwnershipReader) *Checker {
	return &Checker{cfg: cfg, labels: labels, ownership: ownership}
}

// Check evaluates the label invariant §4 for the issue identified by the given branch name.
// Returns ErrNoJiraKey immediately if the branch does not match the canonical format.
// Returns ErrLabelInvariant when one or more sub-rules are violated.
// Other errors from ports (ErrIssueNotFound, HTTPError, ErrMalformedOwnership) are bubbled as-is.
func (c *Checker) Check(ctx context.Context, branch string) (cichecks.InvariantResult, error) {
	figura, key, err := cichecks.ParseAgentBranch(branch, c.cfg.Project)
	if err != nil {
		return cichecks.InvariantResult{}, err
	}

	rawLabels, err := c.labels.LabelsByKey(ctx, key)
	if err != nil {
		return cichecks.InvariantResult{}, err
	}

	ownershipMap, err := c.ownership.Ownership(ctx)
	if err != nil {
		return cichecks.InvariantResult{}, err
	}

	labelSet := cichecks.ParseLabels(rawLabels)

	result, err := cichecks.CheckLabelInvariant(cichecks.InvariantInput{
		Labels:       labelSet,
		Ownership:    ownershipMap,
		BranchFigura: figura,
	})
	if err != nil {
		return cichecks.InvariantResult{}, err
	}

	if len(result.Violations) > 0 {
		return result, &cichecks.ErrLabelInvariant{Violations: result.Violations}
	}
	return result, nil
}
