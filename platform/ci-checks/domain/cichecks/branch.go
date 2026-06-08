package cichecks

import "regexp"

var branchRE = regexp.MustCompile(`^agent/([a-z]+)/(TAL-[0-9]+)$`)

// ParseAgentBranch extracts the agent figura and JIRA key from a canonical branch name.
// Returns ErrNoJiraKey for any branch that does not match agent/<figura>/<TAL-N>.
func ParseAgentBranch(branch string) (figura, key string, err error) {
	m := branchRE.FindStringSubmatch(branch)
	if m == nil {
		return "", "", ErrNoJiraKey
	}
	return m[1], m[2], nil
}
