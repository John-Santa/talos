// Package port defines the inbound/outbound ports for the jira-evidence-loop
// hexagonal architecture. Adapters implement these interfaces; the service
// depends only on them, never on concrete adapters.
package port

import (
	"context"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// JiraClient is the outbound port for all Jira REST API operations required
// by the evidence loop. Exactly 8 methods: 7 write operations + Search for
// idempotency pre-check (REQ-IDEM, REQ-TRANS, REQ-COMMENT, REQ-WORKLOG,
// REQ-REMOTELINK, REQ-ATTACH).
//
// All implementations must:
//   - Return non-nil errors on any non-2xx HTTP response.
//   - Never swallow errors silently.
//   - Not perform retries (caller decides retry policy).
type JiraClient interface {
	// CreateIssue creates a new Jira issue and returns the created Issue
	// (including its assigned key, e.g. "TAL-1"). Fails loud on error.
	CreateIssue(ctx context.Context, req evidence.CreateIssueRequest) (evidence.Issue, error)

	// Search executes a JQL query and returns matching issues. Used by the
	// service for idempotent pre-create checks (REQ-IDEM). maxResults
	// limits the number of results returned (use a small value like 2 for
	// duplicate detection).
	Search(ctx context.Context, jql string, maxResults int) ([]evidence.Issue, error)

	// GetTransitions returns the list of transitions currently available
	// for the issue identified by issueKey. The returned Transition values
	// include the statusCategory.key for locale-independent matching
	// (REQ-TRANS, Decision 3).
	GetTransitions(ctx context.Context, issueKey string) ([]evidence.Transition, error)

	// DoTransition applies the transition with the given transitionID to
	// the issue identified by issueKey.
	DoTransition(ctx context.Context, issueKey, transitionID string) error

	// AddComment adds an ADF v3 comment body to the specified issue.
	AddComment(ctx context.Context, issueKey string, body evidence.ADFDocument) error

	// AddWorklog appends a worklog entry to the specified issue.
	AddWorklog(ctx context.Context, issueKey string, worklog evidence.Worklog) error

	// CreateRemoteLink upserts a remote link on the specified issue using
	// the globalId field for idempotency (REQ-IDEM step 5). Fails loud —
	// the link must be recorded for evidence DoD.
	CreateRemoteLink(ctx context.Context, issueKey string, link evidence.RemoteLink) error

	// AddAttachment uploads a file attachment to the specified issue via
	// multipart/form-data. Requires the X-Atlassian-Token: no-check header
	// (REQ-ATTACH). Fails loud — the attachment is evidence DoD.
	AddAttachment(ctx context.Context, issueKey string, attachment evidence.Attachment) error
}
