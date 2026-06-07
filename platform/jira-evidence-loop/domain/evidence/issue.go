// Package evidence contains pure domain value objects for the jira-evidence-loop.
// Zero I/O, zero third-party imports.
package evidence

import "fmt"

// ---------------------------------------------------------------------------
// Label
// ---------------------------------------------------------------------------

// Label is a typed key:value pair attached to a Jira issue.
type Label struct {
	Key   string
	Value string
}

// String returns the "key:value" representation required by Jira labels.
func (l Label) String() string {
	return l.Key + ":" + l.Value
}

// ---------------------------------------------------------------------------
// LabelSet
// ---------------------------------------------------------------------------

// LabelSet is an ordered collection of Labels.
type LabelSet []Label

// Get returns the value for the first Label whose Key matches k, plus a found bool.
func (ls LabelSet) Get(k string) (string, bool) {
	for _, l := range ls {
		if l.Key == k {
			return l.Value, true
		}
	}
	return "", false
}

// Strings returns all labels formatted as "key:value" strings.
func (ls LabelSet) Strings() []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.String()
	}
	return out
}

// Validate checks that each key in requiredKeys appears exactly once in ls.
func (ls LabelSet) Validate(requiredKeys []string) error {
	counts := make(map[string]int, len(ls))
	for _, l := range ls {
		counts[l.Key]++
	}
	for _, k := range requiredKeys {
		n := counts[k]
		if n == 0 {
			return fmt.Errorf("evidence: label %q is required but missing", k)
		}
		if n > 1 {
			return fmt.Errorf("evidence: label %q appears %d times, must be exactly 1", k, n)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ADFDocument — ADF v3 value object
// ---------------------------------------------------------------------------

// ADFNode is a single node in an Atlassian Document Format tree.
type ADFNode struct {
	Type    string             `json:"type"`
	Text    string             `json:"text,omitempty"`
	Content []ADFNode          `json:"content,omitempty"`
	Marks   []ADFMark          `json:"marks,omitempty"`
	Attrs   map[string]any     `json:"attrs,omitempty"`
}

// ADFMark decorates a text node (e.g., a link).
type ADFMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// ADFDocument is the top-level ADF v3 document value.
type ADFDocument struct {
	Version int       `json:"version"`
	Type    string    `json:"type"`
	Content []ADFNode `json:"content"`
}

// NewADFDocument creates a minimal ADF v3 document containing a single
// paragraph with the given plain text. This satisfies REQ-COMMENT (minimum
// node set: doc/paragraph/text).
func NewADFDocument(text string) ADFDocument {
	return ADFDocument{
		Version: 1,
		Type:    "doc",
		Content: []ADFNode{
			{
				Type: "paragraph",
				Content: []ADFNode{
					{Type: "text", Text: text},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Issue
// ---------------------------------------------------------------------------

// Issue represents a Jira issue (value object — no behaviour beyond data).
type Issue struct {
	Key     string
	Summary string
	Labels  LabelSet
}

// ---------------------------------------------------------------------------
// CreateIssueRequest
// ---------------------------------------------------------------------------

// CreateIssueRequest carries the data needed to create a new Jira issue.
type CreateIssueRequest struct {
	ProjectKey    string
	ProjectID     string
	IssueTypeName string
	Summary       string
	Description   ADFDocument
	Labels        LabelSet
}

// ---------------------------------------------------------------------------
// Transition
// ---------------------------------------------------------------------------

// Transition represents a single workflow transition available for an issue.
// ToCategory matches Jira's statusCategory.key ("new", "indeterminate", "done").
type Transition struct {
	ID         string
	ToName     string
	ToCategory string
}

// ---------------------------------------------------------------------------
// Worklog
// ---------------------------------------------------------------------------

// Worklog records time spent on an issue.
type Worklog struct {
	TimeSpentSeconds int
	Comment          ADFDocument
	// Started is the ISO-8601 datetime string (e.g. "2006-01-02T15:04:05.000+0000").
	Started string
}

// DurationString formats TimeSpentSeconds as Jira's duration string ("1h 30m").
func (w Worklog) DurationString() string {
	h := w.TimeSpentSeconds / 3600
	m := (w.TimeSpentSeconds % 3600) / 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// ---------------------------------------------------------------------------
// RemoteLink
// ---------------------------------------------------------------------------

// RemoteLink represents an external link (e.g., a pull request) attached to
// a Jira issue via the remote-link REST endpoint.
type RemoteLink struct {
	PRURL        string
	Relationship string
}

// GlobalID returns the canonical globalId used for upsert idempotency:
// "pr=<url>".
func (rl RemoteLink) GlobalID() string {
	return "pr=" + rl.PRURL
}

// ---------------------------------------------------------------------------
// Attachment
// ---------------------------------------------------------------------------

// Attachment represents a file to be uploaded as a Jira issue attachment.
type Attachment struct {
	Filename    string
	ContentType string
	// Data holds the raw bytes to be sent as multipart/form-data.
	Data []byte
}
