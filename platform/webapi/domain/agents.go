package domain

// Roster is the canonical figura order.
var Roster = []string{
	"athena", "atlas", "hephaestus", "cronos", "iris", "gaia", "themis", "hermes", "argos", "zeus",
}

// devRoster are the figuras that take worktrees (used to derive the idle bench).
var devRoster = []string{"atlas", "hephaestus", "cronos", "iris", "gaia", "themis", "hermes"}

var registry = map[string]Agent{
	"athena":     {ID: "athena", Name: "Athena", Role: "Orquestador · PM", Model: "Opus", HueToken: "--ag-athena"},
	"atlas":      {ID: "atlas", Name: "Atlas", Role: "Backend · ctx #1", Model: "Sonnet", HueToken: "--ag-atlas"},
	"hephaestus": {ID: "hephaestus", Name: "Hephaestus", Role: "Backend · ctx #2 · forja", Model: "Sonnet", HueToken: "--ag-hephaestus"},
	"cronos":     {ID: "cronos", Name: "Cronos", Role: "Backend · ctx #3", Model: "Sonnet", HueToken: "--ag-cronos"},
	"iris":       {ID: "iris", Name: "Iris", Role: "Frontend", Model: "Sonnet", HueToken: "--ag-iris"},
	"gaia":       {ID: "gaia", Name: "Gaia", Role: "Datos", Model: "Sonnet", HueToken: "--ag-gaia"},
	"themis":     {ID: "themis", Name: "Themis", Role: "QA · testing", Model: "Sonnet", HueToken: "--ag-themis"},
	"hermes":     {ID: "hermes", Name: "Hermes", Role: "DevOps · entrega", Model: "Sonnet", HueToken: "--ag-hermes"},
	"argos":      {ID: "argos", Name: "Argos", Role: "Revisión adversarial", Model: "jueces", HueToken: "--ag-argos"},
	"zeus":       {ID: "zeus", Name: "Zeus", Role: "TL humano · gates", Model: "humano", HueToken: "--ag-zeus"},
}

// AllAgents returns every agent in roster order.
func AllAgents() []Agent {
	out := make([]Agent, 0, len(Roster))
	for _, id := range Roster {
		out = append(out, registry[id])
	}
	return out
}

// AgentByID looks up an agent by figura id.
func AgentByID(id string) (Agent, bool) {
	a, ok := registry[id]
	return a, ok
}
