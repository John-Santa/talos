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

// portBase is the base port for agent resource assignment. Each figura gets
// an offset from this base.
// DEBT(kit-extraction): move to config file in Fase 5.
const portBase = 8100 // DEBT(kit-extraction)

// assignMap maps each figura to its port offset.
// The offset is added to portBase to derive the final port.
// Offsets are fixed and collision-free by construction.
// DEBT(kit-extraction): move to config file in Fase 5.
var assignMap = map[Figura]int{ // DEBT(kit-extraction)
	"atlas":      0,
	"hephaestus": 1,
	"cronos":     2,
	"iris":       3,
	"gaia":       4,
	"themis":     5,
	"hermes":     6,
	"argos":      7,
}

// AgentResources returns the deterministic Resources (Port + DBSchema) for the
// given figura. Returns ErrInvalidFigure if figura is not in the assign map.
//
// The port assignment is fixed (portBase + offset), disjoint across all figuras,
// and not runtime-configurable in Fase 2 (ADR-D5).
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

// RenderEnv produces the content for a .env file for the given worktree.
// Output contains ONLY PORT and DB_SCHEMA lines — no JIRA_* credentials
// (Decision 5 LOCKED). The output is pure and deterministic: calling
// RenderEnv with the same inputs always produces byte-identical output
// (ADR-D6 + REQ-ENV-3). No runtime timestamp in the variable block.
func RenderEnv(spec WorktreeSpec, res Resources) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "PORT=%d\n", res.Port)
	fmt.Fprintf(&sb, "DB_SCHEMA=%s\n", res.DBSchema)
	return sb.String()
}
