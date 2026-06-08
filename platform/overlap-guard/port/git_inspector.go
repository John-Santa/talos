package port

import "context"

// GitInspector is the read-only outbound port for git inspection (own implementation, not mo's).
type GitInspector interface {
	Fetch(ctx context.Context) error
	RevParse(ctx context.Context, ref string) (string, error)
	ChangedFiles(ctx context.Context, base, branch string) ([]string, error)
}
