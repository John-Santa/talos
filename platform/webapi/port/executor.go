// Package port declares the outbound interfaces the gateway depends on.
package port

import "context"

// CLIExecutor runs a platform CLI (wt/mo/ov/ch) and returns its stdout.
type CLIExecutor interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
