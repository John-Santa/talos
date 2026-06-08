package worktree

import (
	"fmt"
	"strings"
)

// Resources holds the per-agent compute resources: port and DB schema.
type Resources struct {
	Port     int
	DBSchema string
}

const portBase = 8100

var assignMap = map[Figura]int{
	"atlas":      0,
	"hephaestus": 1,
	"cronos":     2,
	"iris":       3,
	"gaia":       4,
	"themis":     5,
	"hermes":     6,
	"argos":      7,
}

// AgentResources returns the deterministic Resources for the given figura, or ErrInvalidFigure if unknown.
func AgentResources(f Figura) (Resources, error) {
	offset, ok := assignMap[f]
	if !ok {
		return Resources{}, &ErrInvalidFigure{Figura: string(f)}
	}
	return Resources{
		Port:     portBase + offset,
		DBSchema: fmt.Sprintf("wt_%s", f),
	}, nil
}

// RenderEnv produces the deterministic .env content (PORT + DB_SCHEMA only) for the given worktree.
func RenderEnv(spec WorktreeSpec, res Resources) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "PORT=%d\n", res.Port)
	fmt.Fprintf(&sb, "DB_SCHEMA=%s\n", res.DBSchema)
	return sb.String()
}
