// Package service contains the application-layer use cases for the console module.
package service

import (
	"context"
	"sync"

	"github.com/John-Santa/talos/platform/console/domain/platform"
	"github.com/John-Santa/talos/platform/console/port"
)

// Snapshot is the aggregate view of the platform state at a single point in time.
// Sources that fail contribute their error to Errors; successful sources are
// always populated regardless of other failures (degrade-on-fail).
// Labels is intentionally excluded: it is per-branch and fetched on demand.
type Snapshot struct {
	Worktrees []platform.Worktree
	MergePlan platform.MergePlan
	Overlap   platform.Overlap
	Errors    map[string]error
}

// Aggregator fans out concurrent reads from a PlatformReader and assembles a Snapshot.
type Aggregator struct {
	reader port.PlatformReader
}

// NewAggregator constructs an Aggregator backed by the given reader.
func NewAggregator(reader port.PlatformReader) *Aggregator {
	return &Aggregator{reader: reader}
}

// Snapshot fetches Worktrees, MergePlan, and Overlap concurrently.
// Any source that errors has its error recorded in Snapshot.Errors keyed by source name;
// the remaining sources are still returned (degrade-on-fail).
func (a *Aggregator) Snapshot(ctx context.Context) Snapshot {
	var (
		snap Snapshot
		mu   sync.Mutex
		wg   sync.WaitGroup
	)

	setErr := func(key string, err error) {
		mu.Lock()
		if snap.Errors == nil {
			snap.Errors = make(map[string]error)
		}
		snap.Errors[key] = err
		mu.Unlock()
	}

	wg.Add(3)

	go func() {
		defer wg.Done()
		wts, err := a.reader.Worktrees(ctx)
		if err != nil {
			setErr("worktrees", err)
			return
		}
		mu.Lock()
		snap.Worktrees = wts
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		plan, err := a.reader.MergePlan(ctx)
		if err != nil {
			setErr("mergeplan", err)
			return
		}
		mu.Lock()
		snap.MergePlan = plan
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		ov, err := a.reader.Overlap(ctx)
		if err != nil {
			setErr("overlap", err)
			return
		}
		mu.Lock()
		snap.Overlap = ov
		mu.Unlock()
	}()

	wg.Wait()
	return snap
}
