// Package jirarest implements the IssueLabelReader port via the Jira REST API v3.
package jirarest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
	"github.com/John-Santa/talos/platform/ci-checks/port"
)

// HTTPError is returned on any non-2xx Jira API response.
type HTTPError struct {
	StatusCode int
	Body       string
	Op         string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("jira %s: HTTP %d: %s", e.Op, e.StatusCode, e.Body)
}

// Client implements port.IssueLabelReader against the Jira REST API v3. Zero external deps.
type Client struct {
	http    *http.Client
	baseURL string
	email   string
	token   string
}

var _ port.IssueLabelReader = (*Client)(nil)

// NewClient constructs a Client targeting siteURL and authenticating via Basic auth.
func NewClient(siteURL, email, token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: siteURL,
		email:   email,
		token:   token,
	}
}

// issueLabelResponse is the minimal shape of GET /rest/api/3/issue/{key}?fields=labels.
type issueLabelResponse struct {
	Key    string `json:"key"`
	Fields struct {
		Labels []string `json:"labels"`
	} `json:"fields"`
}

// LabelsByKey implements port.IssueLabelReader via GET /rest/api/3/issue/{key}?fields=labels.
func (c *Client) LabelsByKey(ctx context.Context, key string) ([]string, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=labels", c.baseURL, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("jirarest LabelsByKey: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	setBasicAuth(req, c.email, c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jirarest LabelsByKey: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, cichecks.ErrIssueNotFound{Key: key}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(raw), Op: "LabelsByKey"}
	}

	var res issueLabelResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("jirarest LabelsByKey: decode: %w", err)
	}
	return res.Fields.Labels, nil
}
