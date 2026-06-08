// Package jirarest implements the IssueSearcher port via the Jira REST API v3.
package jirarest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/John-Santa/talos/platform/overlap-guard/port"
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

// Client implements port.IssueSearcher against the Jira REST API v3. Zero external deps.
type Client struct {
	http    *http.Client
	baseURL string
	email   string
	token   string
}

var _ port.IssueSearcher = (*Client)(nil)

// NewClient constructs a Client targeting siteURL and authenticating via Basic auth.
func NewClient(siteURL, email, token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: siteURL,
		email:   email,
		token:   token,
	}
}

// adfNode is a minimal representation of an Atlassian Document Format node.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

// flattenADF walks the ADF node tree depth-first, collecting all text leaves.
func flattenADF(node adfNode) string {
	if node.Text != "" {
		return node.Text
	}
	if len(node.Content) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, child := range node.Content {
		sb.WriteString(flattenADF(child))
	}
	return sb.String()
}

// Search implements port.IssueSearcher.Search via POST /rest/api/3/issue/search.
func (c *Client) Search(ctx context.Context, jql string, maxResults int) ([]port.IssueResult, error) {
	payload := map[string]any{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"summary", "labels", "description"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jirarest Search: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/rest/api/3/issue/search",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("jirarest Search: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	setBasicAuth(req, c.email, c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jirarest Search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(raw), Op: "Search"}
	}

	type issueRow struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string          `json:"summary"`
			Labels      []string        `json:"labels"`
			Description json.RawMessage `json:"description"`
		} `json:"fields"`
	}
	type searchResponse struct {
		Issues []issueRow `json:"issues"`
	}

	var res searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("jirarest Search: decode: %w", err)
	}

	out := make([]port.IssueResult, 0, len(res.Issues))
	for _, row := range res.Issues {
		var descText string
		if len(row.Fields.Description) > 0 && string(row.Fields.Description) != "null" {
			var node adfNode
			if err := json.Unmarshal(row.Fields.Description, &node); err == nil {
				descText = flattenADF(node)
			}
		}
		out = append(out, port.IssueResult{
			Key:    row.Key,
			Labels: row.Fields.Labels,
			Body:   descText,
		})
	}
	return out, nil
}
