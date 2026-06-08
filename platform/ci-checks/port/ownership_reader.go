package port

import "context"

// OwnershipReader reads the module→agent ownership table.
type OwnershipReader interface {
	Ownership(ctx context.Context) (map[string]string, error)
}
