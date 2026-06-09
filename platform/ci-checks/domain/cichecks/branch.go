package cichecks

import "regexp"

// ParseAgentBranch extracts the agent figura and Jira key from a canonical branch name.
// The branch must follow the pattern agent/<figura>/<projectKey>-N where projectKey
// is the caller-supplied Jira project key (e.g. "TAL", "FOO").
// Returns ErrNoJiraKey for any branch that does not match agent/<figura>/<projectKey>-N.
func ParseAgentBranch(branch, projectKey string) (figura, key string, err error) {
	re := regexp.MustCompile(`^agent/([a-z]+)/(` + regexp.QuoteMeta(projectKey) + `-[0-9]+)$`)
	m := re.FindStringSubmatch(branch)
	if m == nil {
		return "", "", ErrNoJiraKey
	}
	return m[1], m[2], nil
}
