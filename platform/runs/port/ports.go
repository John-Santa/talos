// Package port defines the outbound ports for the runs module.
package port

import (
	"context"

	"github.com/John-Santa/talos/platform/runs/domain/run"
)

// RunStore is the persistence port for run event log operations.
type RunStore interface {
	// Append appends a validated run event to the log. O(1).
	Append(ctx context.Context, e run.RunEvent) error
	// Query scans the log and returns events matching the filter.
	Query(ctx context.Context, f run.Filter) ([]run.RunEvent, error)
}
