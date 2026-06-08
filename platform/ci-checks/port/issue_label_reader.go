// Package port defines the outbound interfaces that the ci-checks service depends on.
package port

import "context"

// IssueLabelReader fetches the Jira labels for a given issue key.
type IssueLabelReader interface {
	LabelsByKey(ctx context.Context, key string) ([]string, error)
}
