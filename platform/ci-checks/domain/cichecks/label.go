package cichecks

import (
	"fmt"
	"strings"
)

// Label is a typed key:value pair attached to a Jira issue.
type Label struct {
	// Key is the part before the colon, e.g. "agent".
	Key string
	// Value is the part after the colon, e.g. "hermes".
	Value string
}

// String returns the "key:value" representation of the label.
func (l Label) String() string {
	return l.Key + ":" + l.Value
}

// LabelSet is an ordered collection of Labels parsed from Jira label strings.
type LabelSet []Label

// ParseLabels parses raw Jira label strings (e.g. "agent:hermes") into a LabelSet.
// Strings without a colon separator are silently skipped.
func ParseLabels(raw []string) LabelSet {
	var ls LabelSet
	for _, s := range raw {
		idx := strings.IndexByte(s, ':')
		if idx < 0 {
			continue
		}
		ls = append(ls, Label{Key: s[:idx], Value: s[idx+1:]})
	}
	return ls
}

// Get returns the value of the first Label whose Key matches k, plus a found bool.
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
			return fmt.Errorf("ci-checks: label %q is required but missing", k)
		}
		if n > 1 {
			return fmt.Errorf("ci-checks: label %q appears %d times, must be exactly 1", k, n)
		}
	}
	return nil
}
