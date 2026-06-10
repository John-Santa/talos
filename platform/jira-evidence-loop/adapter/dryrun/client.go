// Package dryrun provides a no-op implementation of port.JiraClient that logs
// planned operations to an io.Writer without making any network calls. It is
// used when --dry-run is passed to the evidence CLI or when EVIDENCE_DRY_RUN=1
// is set (Design §D6, fail-soft without token).
package dryrun

import (
	"context"
	"fmt"
	"io"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/port"
)

// Compile-time assertion: Client must satisfy port.JiraClient.
var _ port.JiraClient = (*Client)(nil)

// Client implements port.JiraClient as a planning/logging no-op. Every method
// logs the intended operation to the writer and returns a zero value without
// error. CreateIssue returns a deterministic placeholder key so callers can
// continue without panicking.
type Client struct {
	w io.Writer
}

// NewClient constructs a DryRunClient that writes its plan log to w.
// Pass os.Stdout for CLI usage or a bytes.Buffer in tests.
func NewClient(w io.Writer) *Client {
	return &Client{w: w}
}

func (c *Client) log(format string, args ...any) {
	fmt.Fprintf(c.w, "[dry-run] "+format+"\n", args...)
}

// CreateIssue logs the planned create and returns a placeholder key.
func (c *Client) CreateIssue(_ context.Context, req evidence.CreateIssueRequest) (evidence.Issue, error) {
	c.log("CreateIssue: project=%s summary=%q", req.ProjectKey, req.Summary)
	return evidence.Issue{Key: "DRY-0"}, nil
}

// Search logs the planned search and returns an empty result (dry-run never
// finds existing issues — callers that include StepCreate will always reach
// the create path).
func (c *Client) Search(_ context.Context, jql string, maxResults int) ([]evidence.Issue, error) {
	c.log("Search: jql=%q maxResults=%d → [] (dry-run)", jql, maxResults)
	return nil, nil
}

// GetTransitions logs the planned call and returns an empty slice (no
// transitions available means doTransitionToCategory is a no-op).
func (c *Client) GetTransitions(_ context.Context, issueKey string) ([]evidence.Transition, error) {
	c.log("GetTransitions: issue=%s → [] (dry-run)", issueKey)
	return nil, nil
}

// DoTransition logs the planned transition without calling Jira.
func (c *Client) DoTransition(_ context.Context, issueKey, transitionID string) error {
	c.log("DoTransition: issue=%s transitionID=%s", issueKey, transitionID)
	return nil
}

// AddComment logs the planned comment without calling Jira.
func (c *Client) AddComment(_ context.Context, issueKey string, body evidence.ADFDocument) error {
	c.log("AddComment: issue=%s bodyType=%s", issueKey, body.Type)
	return nil
}

// AddWorklog logs the planned worklog without calling Jira.
func (c *Client) AddWorklog(_ context.Context, issueKey string, worklog evidence.Worklog) error {
	c.log("AddWorklog: issue=%s seconds=%d", issueKey, worklog.TimeSpentSeconds)
	return nil
}

// CreateRemoteLink logs the planned remote link without calling Jira.
func (c *Client) CreateRemoteLink(_ context.Context, issueKey string, link evidence.RemoteLink) error {
	c.log("CreateRemoteLink: issue=%s url=%s", issueKey, link.PRURL)
	return nil
}

// AddAttachment logs the planned attachment without calling Jira.
func (c *Client) AddAttachment(_ context.Context, issueKey string, attachment evidence.Attachment) error {
	c.log("AddAttachment: issue=%s filename=%s bytes=%d", issueKey, attachment.Filename, len(attachment.Data))
	return nil
}
