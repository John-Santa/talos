package mergeorder

import "time"

// Candidate represents an agent branch that is eligible for integration.
type Candidate struct {
	Figura       string
	Branch       string
	Head         string
	Path         string
	CommitsAhead int
	ChangedFiles []string
	CreatedAt    time.Time
}
