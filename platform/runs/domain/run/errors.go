// Package run contains the pure domain logic for the runs module.
package run

import "fmt"

// ErrInvalidKind is returned when the event Kind is not one of the allowed values.
type ErrInvalidKind struct {
	Kind Kind
}

func (e *ErrInvalidKind) Error() string {
	return fmt.Sprintf("invalid run event kind %q: must be one of dispatch, activity, judgment, metric", e.Kind)
}

// ErrMissingJiraKey is returned when JiraKey is required but absent.
type ErrMissingJiraKey struct{}

func (e *ErrMissingJiraKey) Error() string {
	return "run event requires a non-empty jiraKey"
}

// ErrInvalidEvent is returned when an event fails validation for a reason other than kind or jiraKey.
type ErrInvalidEvent struct {
	Reason string
}

func (e *ErrInvalidEvent) Error() string {
	return fmt.Sprintf("invalid run event: %s", e.Reason)
}
