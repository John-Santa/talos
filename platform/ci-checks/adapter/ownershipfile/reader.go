// Package ownershipfile implements the OwnershipReader port by reading a Markdown file.
package ownershipfile

import (
	"context"
	"os"

	"github.com/John-Santa/talos/platform/ci-checks/domain/cichecks"
	"github.com/John-Santa/talos/platform/ci-checks/port"
)

// Reader reads the module→agent ownership table from a Markdown file on disk.
type Reader struct {
	path string
}

var _ port.OwnershipReader = (*Reader)(nil)

// NewReader constructs a Reader that reads from path.
func NewReader(path string) *Reader {
	return &Reader{path: path}
}

// Ownership implements port.OwnershipReader by reading and parsing the ownership file.
func (r *Reader) Ownership(_ context.Context) (map[string]string, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return nil, err
	}
	return cichecks.ParseOwnershipTable(string(data))
}
