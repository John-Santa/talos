// Package service contains the runs use-case services.
package service

import (
	"context"

	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/port"
)

// Recorder is the use-case service for appending run events.
type Recorder struct {
	store port.RunStore
}

// NewRecorder constructs a Recorder backed by the given store.
func NewRecorder(store port.RunStore) *Recorder {
	return &Recorder{store: store}
}

// Record validates and appends a RunEvent to the store.
// Returns domain validation errors or a store error.
func (r *Recorder) Record(ctx context.Context, e run.RunEvent) error {
	if err := e.Validate(); err != nil {
		return err
	}
	return r.store.Append(ctx, e)
}
