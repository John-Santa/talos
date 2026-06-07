package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/port"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/service"
)

// Compile-time assertion: *Client must satisfy port.JiraClient.
var _ port.JiraClient = (*Client)(nil)

// Client is the concrete HTTP adapter implementing port.JiraClient against the
// Jira REST API v3. It uses only stdlib net/http — zero transitive deps.
//
// service.Config is accepted at construction to keep the adapter thin;
// it reads SiteURL and Credentials from config but never imports service
// types beyond that.
type Client struct {
	http    *http.Client
	cfg     service.Config
	baseURL string
}

// NewClient constructs a Client wired to the Jira instance described by cfg.
func NewClient(cfg service.Config) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		cfg:     cfg,
		baseURL: cfg.SiteURL,
	}
}

// ---------------------------------------------------------------------------
// HTTPError — typed error for non-2xx responses (REQ-ERR)
// ---------------------------------------------------------------------------

// HTTPError is returned on any non-2xx Jira API response. It includes the
// HTTP status code and the raw response body for diagnosis.
type HTTPError struct {
	StatusCode int
	Body       string
	Op         string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("jira %s: HTTP %d: %s", e.Op, e.StatusCode, e.Body)
}

// checkStatus reads the response body and returns an HTTPError if the status
// code is outside the 2xx range.
func checkStatus(resp *http.Response, op string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(resp.Body)
	return &HTTPError{StatusCode: resp.StatusCode, Body: string(raw), Op: op}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (c *Client) url(path string) string {
	return c.baseURL + path
}

func (c *Client) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("rest: marshal %s: %w", path, err)
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = &bytes.Buffer{}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	setBasicAuth(req, c.cfg.Credentials)
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}

// ---------------------------------------------------------------------------
// CreateIssue — POST /rest/api/3/issue
// ---------------------------------------------------------------------------

func (c *Client) CreateIssue(ctx context.Context, r evidence.CreateIssueRequest) (evidence.Issue, error) {
	type issueTypePayload struct {
		Name string `json:"name"`
	}
	type projectPayload struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	type fieldsPayload struct {
		Project     projectPayload   `json:"project"`
		IssueType   issueTypePayload `json:"issuetype"`
		Summary     string           `json:"summary"`
		Description evidence.ADFDocument `json:"description"`
		Labels      []string         `json:"labels"`
	}
	type createPayload struct {
		Fields fieldsPayload `json:"fields"`
	}

	payload := createPayload{
		Fields: fieldsPayload{
			Project:     projectPayload{Key: r.ProjectKey, ID: r.ProjectID},
			IssueType:   issueTypePayload{Name: r.IssueTypeName},
			Summary:     r.Summary,
			Description: r.Description,
			Labels:      r.Labels.Strings(),
		},
	}

	req, err := c.newJSONRequest(ctx, http.MethodPost, "/rest/api/3/issue", payload)
	if err != nil {
		return evidence.Issue{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return evidence.Issue{}, fmt.Errorf("rest CreateIssue: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "CreateIssue"); err != nil {
		return evidence.Issue{}, err
	}

	type createResponse struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string   `json:"summary"`
			Labels  []string `json:"labels"`
		} `json:"fields"`
	}
	var res createResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return evidence.Issue{}, fmt.Errorf("rest CreateIssue decode: %w", err)
	}
	ls := make(evidence.LabelSet, 0, len(res.Fields.Labels))
	for _, s := range res.Fields.Labels {
		ls = append(ls, labelFromString(s))
	}
	return evidence.Issue{Key: res.Key, Summary: res.Fields.Summary, Labels: ls}, nil
}

// ---------------------------------------------------------------------------
// Search — POST /rest/api/3/issue/search
// ---------------------------------------------------------------------------

func (c *Client) Search(ctx context.Context, jql string, maxResults int) ([]evidence.Issue, error) {
	payload := map[string]any{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"summary", "labels"},
	}
	req, err := c.newJSONRequest(ctx, http.MethodPost, "/rest/api/3/issue/search", payload)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("rest Search: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "Search"); err != nil {
		return nil, err
	}

	type issueRow struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string   `json:"summary"`
			Labels  []string `json:"labels"`
		} `json:"fields"`
	}
	type searchResponse struct {
		Issues []issueRow `json:"issues"`
	}
	var res searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("rest Search decode: %w", err)
	}
	out := make([]evidence.Issue, len(res.Issues))
	for i, row := range res.Issues {
		ls := make(evidence.LabelSet, 0, len(row.Fields.Labels))
		for _, s := range row.Fields.Labels {
			ls = append(ls, labelFromString(s))
		}
		out[i] = evidence.Issue{Key: row.Key, Summary: row.Fields.Summary, Labels: ls}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// GetTransitions — GET /rest/api/3/issue/{key}/transitions
// ---------------------------------------------------------------------------

func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]evidence.Transition, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey)
	req, err := c.newJSONRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("rest GetTransitions: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, "GetTransitions"); err != nil {
		return nil, err
	}

	type toStatus struct {
		StatusCategory struct {
			Key string `json:"key"`
		} `json:"statusCategory"`
	}
	type transRow struct {
		ID   string   `json:"id"`
		Name string   `json:"name"`
		To   toStatus `json:"to"`
	}
	type transResponse struct {
		Transitions []transRow `json:"transitions"`
	}
	var res transResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("rest GetTransitions decode: %w", err)
	}
	out := make([]evidence.Transition, len(res.Transitions))
	for i, t := range res.Transitions {
		out[i] = evidence.Transition{
			ID:         t.ID,
			ToName:     t.Name,
			ToCategory: t.To.StatusCategory.Key,
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// DoTransition — POST /rest/api/3/issue/{key}/transitions
// ---------------------------------------------------------------------------

func (c *Client) DoTransition(ctx context.Context, issueKey, transitionID string) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey)
	payload := map[string]any{
		"transition": map[string]string{"id": transitionID},
	}
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("rest DoTransition: %w", err)
	}
	defer resp.Body.Close()
	return checkStatus(resp, "DoTransition")
}

// ---------------------------------------------------------------------------
// AddComment — POST /rest/api/3/issue/{key}/comment
// ---------------------------------------------------------------------------

func (c *Client) AddComment(ctx context.Context, issueKey string, body evidence.ADFDocument) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/comment", issueKey)
	payload := map[string]any{"body": body}
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("rest AddComment: %w", err)
	}
	defer resp.Body.Close()
	return checkStatus(resp, "AddComment")
}

// ---------------------------------------------------------------------------
// AddWorklog — POST /rest/api/3/issue/{key}/worklog
// ---------------------------------------------------------------------------

func (c *Client) AddWorklog(ctx context.Context, issueKey string, wl evidence.Worklog) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey)
	payload := map[string]any{
		"timeSpent": wl.DurationString(),
		"started":   wl.Started,
		"comment":   wl.Comment,
	}
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("rest AddWorklog: %w", err)
	}
	defer resp.Body.Close()
	return checkStatus(resp, "AddWorklog")
}

// ---------------------------------------------------------------------------
// CreateRemoteLink — POST /rest/api/3/issue/{key}/remotelink
// ---------------------------------------------------------------------------

func (c *Client) CreateRemoteLink(ctx context.Context, issueKey string, link evidence.RemoteLink) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/remotelink", issueKey)
	payload := map[string]any{
		"globalId":     link.GlobalID(),
		"relationship": link.Relationship,
		"object": map[string]any{
			"url":   link.PRURL,
			"title": link.PRURL,
			"icon": map[string]any{
				"url16x16": "https://github.com/favicon.ico",
				"title":    "GitHub",
			},
		},
	}
	req, err := c.newJSONRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("rest CreateRemoteLink: %w", err)
	}
	defer resp.Body.Close()
	return checkStatus(resp, "CreateRemoteLink")
}

// ---------------------------------------------------------------------------
// AddAttachment — POST /rest/api/3/issue/{key}/attachments (multipart)
// ---------------------------------------------------------------------------

func (c *Client) AddAttachment(ctx context.Context, issueKey string, att evidence.Attachment) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/attachments", issueKey)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Create the file part with the correct Content-Type header.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, att.Filename))
	h.Set("Content-Type", att.ContentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("rest AddAttachment create part: %w", err)
	}
	if _, err := part.Write(att.Data); err != nil {
		return fmt.Errorf("rest AddAttachment write part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("rest AddAttachment close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	// X-Atlassian-Token: no-check is REQUIRED for attachment uploads (REQ-ATTACH).
	req.Header.Set("X-Atlassian-Token", "no-check")
	setBasicAuth(req, c.cfg.Credentials)

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("rest AddAttachment: %w", err)
	}
	defer resp.Body.Close()
	return checkStatus(resp, "AddAttachment")
}

// ---------------------------------------------------------------------------
// labelFromString — utility used during response decoding
// ---------------------------------------------------------------------------

// labelFromString parses a "key:value" label string into an evidence.Label.
// If there is no colon, the entire string is used as the key with empty value.
func labelFromString(s string) evidence.Label {
	for i, ch := range s {
		if ch == ':' {
			return evidence.Label{Key: s[:i], Value: s[i+1:]}
		}
	}
	return evidence.Label{Key: s}
}
