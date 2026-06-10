package service

import (
	"context"

	"github.com/John-Santa/talos/platform/runs/domain/run"
	"github.com/John-Santa/talos/platform/runs/port"
)

// Querier is the use-case service for querying projected views of run events.
type Querier struct {
	store port.RunStore
}

// NewQuerier constructs a Querier backed by the given store.
func NewQuerier(store port.RunStore) *Querier {
	return &Querier{store: store}
}

// Timeline returns the ordered activity entries for a jiraKey (and optionally agent).
func (q *Querier) Timeline(ctx context.Context, jiraKey, agent string) ([]run.ActivityEntry, error) {
	evs, err := q.store.Query(ctx, run.Filter{JiraKey: jiraKey, Agent: agent, Kind: run.KindActivity})
	if err != nil {
		return nil, err
	}
	return run.ProjectActivity(evs), nil
}

// DoD returns the DoD checklist for a jiraKey.
func (q *Querier) DoD(ctx context.Context, jiraKey string) ([]run.DoDItem, error) {
	evs, err := q.store.Query(ctx, run.Filter{JiraKey: jiraKey, Kind: run.KindMetric})
	if err != nil {
		return nil, err
	}
	return run.ProjectDoD(evs), nil
}

// Judgment returns the latest JudgmentReview for a jiraKey.
// Returns Pending:true if no judgment events exist.
func (q *Querier) Judgment(ctx context.Context, jiraKey string) (run.JudgmentReview, error) {
	evs, err := q.store.Query(ctx, run.Filter{JiraKey: jiraKey, Kind: run.KindJudgment})
	if err != nil {
		return run.JudgmentReview{}, err
	}
	return run.ProjectJudgment(jiraKey, evs), nil
}

// Runs returns dispatch run views for the given filter.
func (q *Querier) Runs(ctx context.Context, f run.Filter) ([]run.RunView, error) {
	evs, err := q.store.Query(ctx, run.Filter{
		JiraKey: f.JiraKey,
		Agent:   f.Agent,
		Kind:    run.KindDispatch,
		Since:   f.Since,
		Limit:   f.Limit,
	})
	if err != nil {
		return nil, err
	}
	return run.ProjectRuns(evs, f), nil
}
