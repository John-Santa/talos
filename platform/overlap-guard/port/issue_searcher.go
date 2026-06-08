// Package port defines the outbound interfaces (ports) that the overlap-guard service depends on.
package port

import "context"

// IssueResult is one issue row from the T0 Jira search: key, labels, and flattened-ADF body.
type IssueResult struct {
	Key    string
	Labels []string
	Body   string
}

// IssueSearcher is the read-only outbound port for the T0 pre-assignment Jira search.
type IssueSearcher interface {
	Search(ctx context.Context, jql string, maxResults int) ([]IssueResult, error)
}
