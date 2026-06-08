# Design: merge-order-automation

**Change:** merge-order-automation · **Jira:** TAL-3 · **Module:** module:devops · **Owner:** HERMES
**Store:** hybrid (this file + engram `sdd/merge-order-automation/design`)
**Reads:** proposal (#1853, 5 resolved decisions, HYBRID execution posture locked by ZEUS)
**Module path:** `github.com/John-Santa/talos/platform/merge-order-orchestrator`
**Mirror reference:** `platform/worktree-orchestrator/` (hexagonal, zero deps, hand-written mock, `run(args) error` composition root)

This is the HOW at the architectural level. It does not enumerate task steps — `sdd-tasks`
turns these boundaries into a strict-TDD-ordered backlog. Functional requirements (the WHAT)
are owned by `sdd-spec`; see §12 for the seams where the two phases meet.

This design FORMALIZES the approach already approved by ZEUS. The five forks were resolved in
the proposal; §11 records them as ADRs. Signatures below were verified compile-plausibly against
the real `wt` code (`port/git.go`, `service/orchestrator.go`, `cmd/wt/main.go`,
`mock/git_runner_mock.go`).

---

## 1. Architecture approach

Hexagonal (ports & adapters), the **same skeleton as `worktree-orchestrator`**, with the same
honest caveat that module carried: the real work here is *sequencing and inspecting external git
commands plus shelling out to `wt`*, not pure computation. The dependency arrow still points
inward — `cmd → service → port ← adapter`, everything depends on `domain` — and the domain core
has zero I/O and zero third-party imports.

But this module adds a structural invariant the worktree module did not need: **the read-only
path NEVER holds a write port.** `Planner` depends on `GitInspector` (read) + `WorktreeLister`
(read) and CANNOT compile a call to a mutating git command, because the mutating verb
(`RebaseOnto`) lives on a *separate* `GitIntegrator` interface that only `IntegrationRunner`
holds. "`plan` is side-effect-free" is therefore not a discipline we promise in prose — it is
enforced by the type system. This is the central architectural decision of the change (ADR-M1).

```
            ┌─────────────────────────────────────────────────┐
            │              domain/mergeorder                  │  pure, no deps
            │  Candidate · Step · MergePlan · PlanReport      │  value objects +
            │  Order(candidates, deps) ([]Candidate, error)   │  the deterministic
            │  typed errors                                   │  ordering algorithm
            └─────────────────────────────────────────────────┘  (NO os/exec, NO git)
                  ▲                ▲                   ▲
        imports   │                │                   │   imports
   ┌──────────────┴───┐  ┌─────────┴────────┐  ┌───────┴──────────────┐
   │ service/Planner  │  │ service/         │  │ adapter/gitcli       │
   │ (READ ONLY)      │  │ IntegrationRunner│  │  Inspector (read)    │
   │ depends on       │  │ (WRITE PATH)     │  │  Integrator (write)  │
   │  GitInspector +  │  │ depends on       │  │ adapter/wtcli        │
   │  WorktreeLister  │  │  GitInspector +  │  │  Lister              │
   │  — NEVER on      │  │  GitIntegrator + │  │ IMPLEMENT the ports  │
   │  GitIntegrator   │  │  WorktreeLister  │  └──────────────────────┘
   └────────┬─────────┘  └────────┬─────────┘            ▲
            │ depends on          │ depends on           │ satisfies
            ▼                     ▼                       │
   ┌─────────────────────────────────────────────────────────────────┐
   │  port/  GitInspector (read) · GitIntegrator (write) ·            │
   │         WorktreeLister (wt seam)        — the mockable boundary   │
   └─────────────────────────────────────────────────────────────────┘
            ▲                                              ▲
     mock/ ─┘ (hand-written, 3 mocks)        cmd/mo ──────┘ (composition root)
```

**Boundary rules:**
- `service` imports `port` + `domain`, never any `adapter/*`.
- `adapter/gitcli` and `adapter/wtcli` import `port` + `domain` (to satisfy the interface), never `service`.
- `cmd/mo` is the only place where concrete adapters are constructed and wired — the composition root.
- **Read/write split:** `Planner` is constructed with only the read ports. There is no code path
  in `plan`/`check` that can reach `GitIntegrator.RebaseOnto`.

**Where the value actually is (honest):** the deterministic ordering algorithm (`Order`) is the
pure, exhaustively-unit-tested heart. The `service` layer is sequencing + conflict-rate
arithmetic + typed-error translation, tested against the three mocks. The adapters are thin
(build argv / shell `wt`, run, map exit codes) and only their behaviour against real git / real
`wt` is worth a `testing.Short()`-gated integration test.

---

## 2. Package layout

```
platform/merge-order-orchestrator/
├── go.mod                          # module github.com/John-Santa/talos/platform/merge-order-orchestrator
│                                   # go 1.26 — ZERO transitive deps (Kit goal, mirrors wt/evidence)
├── domain/
│   └── mergeorder/
│       ├── errors.go               # typed errors: ErrDependencyCycle, ErrNoCandidates,
│       │                           #   ErrBranchBehind, ErrMergeConflict
│       ├── candidate.go            # Candidate value object
│       ├── order.go                # Order(candidates, deps) — the deterministic algorithm (pure)
│       ├── plan.go                 # Step, MergePlan, PlanReport + ConflictRate/SegmentationBad
│       ├── errors_test.go
│       ├── candidate_test.go
│       ├── order_test.go           # exhaustive: topo, cycle, tiebreaks, determinism
│       └── plan_test.go            # ConflictRate / SegmentationBad arithmetic
├── port/
│   ├── git_inspector.go            # GitInspector interface (read-only, 6 methods)
│   ├── git_integrator.go           # GitIntegrator interface (write, 1 method)
│   └── worktree_lister.go          # WorktreeLister interface + WorktreeEntry DTO (wt seam)
├── adapter/
│   ├── gitcli/
│   │   ├── inspector.go            # GitInspector impl over os/exec
│   │   ├── integrator.go           # GitIntegrator impl over os/exec
│   │   ├── inspector_test.go       # testing.Short()-gated, real git in t.TempDir()
│   │   └── integrator_test.go      # testing.Short()-gated, real git in t.TempDir()
│   └── wtcli/
│       ├── lister.go               # WorktreeLister impl: shells `wt list --json`, decodes
│       └── lister_test.go          # unit: golden JSON fixture decode; integration: real `wt`
├── service/
│   ├── config.go                   # Config + DefaultTALConfig
│   ├── planner.go                  # Planner (READ ONLY): Plan(ctx) (PlanReport, error)
│   ├── integrator.go               # IntegrationRunner (WRITE PATH): Execute(ctx) error
│   ├── planner_test.go             # mocks; happy + typed errors + call-order asserts
│   └── integrator_test.go          # mocks; asserts merge-to-develop is NEVER called
├── mock/
│   ├── git_inspector_mock.go       # hand-written, mirrors GitRunnerMock pattern
│   ├── git_integrator_mock.go      # hand-written
│   └── worktree_lister_mock.go     # hand-written
└── cmd/
    └── mo/
        ├── main.go                 # composition root: run(args) error + subcommand switch (flag pkg)
        └── main_test.go            # run(args) dispatch + flag parsing
```

**Why `domain/mergeorder` is one package, not several:** `Candidate`, `Step`, `MergePlan`,
`PlanReport`, the `Order` algorithm, and the typed errors are all pure value-logic over the same
vocabulary. Splitting adds import ceremony without a boundary worth enforcing. Same reasoning as
`domain/worktree` being a single package.

**Why `Config` lives in `service/`:** identical reasoning to the wt module — `Config` is the
use-case input (`NewPlanner(...,cfg)` / `NewIntegrationRunner(...,cfg)`); co-locating keeps the
construction contract in one place. The adapters take no config (they run git/`wt` in a given
working directory passed at construction).

**Why ports are split into three files, one interface each:** the read/write separation (ADR-M1)
is the whole point — putting `GitInspector` and `GitIntegrator` in separate files makes the
boundary visually obvious and prevents an accidental merge of the two during maintenance. The wt
module had one `git.go` because it had one port; here the *number* of ports is the architecture.

---

## 3. The Config struct

`service/config.go`. Injected at construction. Mirrors `service.DefaultTALConfig()` from the wt
module.

```go
package service

// Config carries the instance-specific merge-order knobs.
type Config struct {
	// RepoRoot is the absolute path to the main repository checkout. git and wt commands run with this as the working directory.
	RepoRoot string

	// BaseBranch is the integration target branch (CONSTITUTION §11); defaults to "develop".
	BaseBranch string

	// WtBinary is the name or path of the worktree-orchestrator binary to shell out to; defaults to "wt".
	WtBinary string

	// MaxConflictRate is the segmentation-bad threshold (gate HG6); a plan whose predicted ConflictRate exceeds this refuses execution. Defaults to 0.15.
	MaxConflictRate float64
}

// DefaultTALConfig returns a Config seeded with the Talos platform defaults; cmd/mo overrides RepoRoot at runtime.
func DefaultTALConfig() Config {
	return Config{
		BaseBranch:      "develop",
		WtBinary:        "wt",
		MaxConflictRate: 0.15,
	}
}
```

`RepoRoot` is resolved at the composition root via `git rev-parse --show-toplevel` (verbatim the
`repoRoot()` helper from `cmd/wt/main.go`). `MaxConflictRate` is overridable from the CLI
(`--max-conflict-rate`); the others have flags too (`--base`, `--wt-bin`).

---

## 4. The port interfaces

Three interfaces, one per file, named for behavior. The read/write split is structural (ADR-M1).

### 4.1 `port/git_inspector.go` — read-only

```go
// Package port defines the outbound ports for the merge-order-orchestrator.
package port

import "context"

// GitInspector is the read-only outbound port for the git inspection the planner needs; it CANNOT mutate the repository.
type GitInspector interface {
	// Fetch updates remote tracking refs from origin before inspection.
	Fetch(ctx context.Context) error

	// RevParse resolves ref (e.g. "develop") to its commit SHA.
	RevParse(ctx context.Context, ref string) (string, error)

	// MergeBase returns the best common ancestor SHA of a and b.
	MergeBase(ctx context.Context, a, b string) (string, error)

	// CommitsAhead returns how many commits branch is ahead of base (readiness: > 0 means work to merge).
	CommitsAhead(ctx context.Context, base, branch string) (int, error)

	// MergeTreeConflicts predicts whether merging branch into base is clean, returning the conflicting paths when it is not; it never touches the working tree.
	MergeTreeConflicts(ctx context.Context, base, branch string) (conflicts []string, clean bool, err error)

	// ChangedFiles returns the files branch changes relative to base (conflict-surface ranking input).
	ChangedFiles(ctx context.Context, base, branch string) ([]string, error)
}
```

`MergeTreeConflicts` maps to `git merge-tree --write-tree --name-only <base> <branch>`:
- exit 0 → `(nil, true, nil)` — clean.
- exit 1 → `(paths, false, nil)` — conflicting; stdout lists the conflicting file paths.
- any other failure → `(nil, false, err)` — a real error (bad ref, not a repo), not a prediction.

`CommitsAhead` and `ChangedFiles` take `base` **explicitly** (not implicitly "develop") so the
execute path can re-check against the *advanced* develop tip after each rebase, while plan checks
against the static snapshot tip. This is what makes the one-at-a-time re-check correct.

### 4.2 `port/git_integrator.go` — write (execute path only)

```go
package port

import "context"

// GitIntegrator is the write outbound port; only the execution path holds it, and it can ONLY rebase — never merge to the base branch (HG3 is structural).
type GitIntegrator interface {
	// RebaseOnto rebases branch onto base, returning the conflicting paths if the rebase stops with conflicts (no merge to base is ever performed).
	RebaseOnto(ctx context.Context, branch, base string) (conflicts []string, err error)
}
```

There is deliberately **no** `Merge`, `Push`, or `OpenPR` method anywhere in the port surface.
`mo` cannot merge to `develop` because no code can call a verb that does — HG3 (§10 human gate)
is honored by the *absence* of the capability, not by a runtime guard.

### 4.3 `port/worktree_lister.go` — the `wt` seam

```go
package port

import "context"

// WorktreeEntry is one row of `wt list --json`; the JSON tags mirror cmd/wt's listEntry exactly so the decode is a stable seam.
type WorktreeEntry struct {
	Figura string `json:"figura"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Head   string `json:"head"`
	Status string `json:"status"`
}

// WorktreeLister is the outbound port over the worktree inventory; adapter/wtcli implements it by shelling out to `wt list --json`.
type WorktreeLister interface {
	// List returns the current agent worktree inventory.
	List(ctx context.Context) ([]WorktreeEntry, error)
}
```

**Seam verified against the real producer.** `cmd/wt/main.go` defines `listEntry` with exactly
these five JSON tags (`figura`, `branch`, `path`, `head`, `status`) and emits them via
`json.NewEncoder(...).SetIndent("", "  ").Encode(entries)` — a JSON array. `WorktreeEntry`
mirrors it field-for-field. The proposal's Decision 1 (shell-out + local DTO, no go.mod
coupling) is honored: this struct is the local copy of the seam, not an import of the wt module.

**Why operation-shaped ports, not a generic `Run(args...)`:** same instinct the wt module's
ADR-D4 recorded — operation-shaped methods keep argv construction in the adapter, let the mocks
record *semantic* calls (e.g. `MergeTreeConflicts(base, branch)`) for clean order/argument
assertions, and are the natural place to map raw git exit codes into typed domain outcomes.

---

## 5. Domain model (pure)

`domain/mergeorder/`. Zero I/O. **Conflict PREDICTION is NOT here** — that is git I/O
(`MergeTreeConflicts`, a port call). The domain only *orders* candidates and *computes* report
arithmetic over already-collected facts.

### 5.1 Value objects

```go
// candidate.go
// Candidate is one ready-to-merge agent branch with the facts the planner collected about it.
type Candidate struct {
	Figura       string
	Branch       string
	Head         string
	Path         string
	CommitsAhead int
	ChangedFiles []string
	CreatedAt    time.Time
}

// plan.go
// Step is one ordered merge in the plan, carrying the read-only conflict prediction for that step.
type Step struct {
	Position       int
	Candidate      Candidate
	PredictedClean bool
	ConflictFiles  []string
}

// MergePlan is the ordered, side-effect-free plan against a snapshot of the base branch tip.
type MergePlan struct {
	BaseBranch string
	BaseTip    string
	Steps      []Step
}

// PlanReport wraps a MergePlan with the gate arithmetic ATHENA/ZEUS read.
type PlanReport struct {
	Plan             MergePlan
	ConflictingCount int
	ConflictRate     float64
	SegmentationBad  bool
}
```

`ConflictRate = ConflictingCount / len(Steps)` (0 when there are no steps). `SegmentationBad` is
`ConflictRate > cfg.MaxConflictRate` — the gate HG6 signal. These are computed by a pure
constructor over a built `MergePlan` (`plan_test.go` covers the arithmetic, including the
empty-plan division guard).

### 5.2 Typed errors

`errors.go` — typed structs implementing `error`, mirroring the wt module's `errors.go`.

```go
// ErrDependencyCycle is returned when the dependency edges contain a cycle; ordering is impossible.
type ErrDependencyCycle struct{ Branches []string }

// ErrNoCandidates is returned when no branch is ready to merge.
type ErrNoCandidates struct{}

// ErrBranchBehind is returned when a branch has no commits ahead of base at execute time (it advanced past it).
type ErrBranchBehind struct{ Branch string }

// ErrMergeConflict is returned when an execute-time rebase stops with conflicts.
type ErrMergeConflict struct {
	Branch string
	Files  []string
}
```

The CLI maps each to a distinct exit code; ATHENA branches on `errors.As`.

### 5.3 The deterministic ordering algorithm

`order.go` — the heart. Pure, total, deterministic.

```go
// Order returns candidates in a deterministic, dependency-respecting merge order, or a typed error; it performs no I/O and predicts no conflicts.
func Order(candidates []Candidate, deps map[string][]string) ([]Candidate, error)
```

`deps` is `branch → branches it depends on` (must merge AFTER its dependencies). Algorithm:

```
Order(candidates, deps):
  if len(candidates) == 0:
      return ErrNoCandidates

  # 1. TOPOLOGICAL LAYERS over the dependency DAG (§11 criterion #1: deps first)
  #    Edge b -> d means "b depends on d" => d must come before b.
  #    Detect back-edges with a colored DFS (white/gray/black).
  #    On a gray-node revisit => cycle => ErrDependencyCycle{the gray-stack branches}.
  #    Kahn-style: layer L0 = nodes with no unsatisfied deps; remove; repeat.
  layers = topoLayers(candidates, deps)        # []​[]Candidate, or ErrDependencyCycle

  # 2. WITHIN EACH LAYER, tiebreak by CONFLICT SURFACE (§11 criterion #2)
  #    Smaller blast radius first: fewer ChangedFiles, and prefer candidates whose
  #    ChangedFiles are DISJOINT from already-placed candidates in this ordering.
  #    Scoring is pure over the ChangedFiles sets already collected — NO git here.
  for layer in layers:
      stableSortWithin(layer, by: conflictSurfaceScore ascending)

  # 3. FINAL TIEBREAK: FIFO (§11 criterion #3) => total order => deterministic
  #    Equal conflict score => older CreatedAt first; equal CreatedAt => Branch asc.
  #    This guarantees a TOTAL order: two runs over identical input yield identical output.
      stableSortWithin(layer, by: (CreatedAt asc, Branch asc))   # applied as the last key

  return flatten(layers)
```

**Determinism is a hard contract** (`order_test.go` asserts byte-identical output across shuffled
inputs): the three keys (topo layer, conflict-surface, then `CreatedAt`+`Branch`) compose into a
**total** order with no ties, so there is exactly one correct output per input. Sorts are stable
and the final `Branch`-ascending key breaks any remaining tie.

**Conflict-surface is a heuristic, not enforcement.** Per the proposal (out-of-scope #3 /
overlap-protocol), `ChangedFiles` only *ranks* — it never serializes or rejects. Smaller / more
disjoint change sets merge earlier to minimize the chance that a later rebase hits a moved tip.

---

## 6. Service flows

`service/planner.go` (read-only) and `service/integrator.go` (write path). Constructors mirror
`NewOrchestrator(runner, cfg)`:

```go
func NewPlanner(inspector port.GitInspector, lister port.WorktreeLister, cfg Config) *Planner
func NewIntegrationRunner(inspector port.GitInspector, integrator port.GitIntegrator, lister port.WorktreeLister, cfg Config) *IntegrationRunner
```

Note `Planner` is constructed WITHOUT `GitIntegrator` — it cannot mutate (ADR-M1).

### 6.1 `Planner.Plan(ctx) (PlanReport, error)` — read-only

```
INPUT: ctx (deps come from Config-adjacent input wired at cmd; see §7)

  1. LIST        ── lister.List(ctx)                      ⇒ []WorktreeEntry
  2. FILTER      ── keep entries with Status == "active"
  3. FETCH       ── inspector.Fetch(ctx)                  (refresh tracking refs)
  4. SNAPSHOT    ── developTip = inspector.RevParse(ctx, cfg.BaseBranch)
  5. PER BRANCH  ── for each active entry:
        a. ahead = inspector.CommitsAhead(ctx, developTip, branch)
        b. if ahead == 0: DROP (readiness MVP: nothing to merge — Decision 4)
        c. files = inspector.ChangedFiles(ctx, developTip, branch)
        d. createdAt = inspector.<branch tip commit date>   (see §8: git log -1 --format=%cI)
        e. build Candidate{...}
  6. EMPTY GUARD ── no candidates ⇒ ErrNoCandidates
  7. ORDER       ── ordered = mergeorder.Order(candidates, deps)   (pure; cycle ⇒ ErrDependencyCycle)
  8. PREDICT     ── for each ordered candidate, in order:
        conflicts, clean, _ = inspector.MergeTreeConflicts(ctx, developTip, branch)
        build Step{Position, Candidate, PredictedClean: clean, ConflictFiles: conflicts}
  9. REPORT      ── build MergePlan{BaseBranch, BaseTip: developTip, Steps}
                    then PlanReport{Plan, ConflictingCount, ConflictRate, SegmentationBad}
OUTPUT: PlanReport
```

Step 8 predicts every step against the **same static `developTip` snapshot** — `plan` is a
photograph, not a simulation. It never advances the tip and never holds `GitIntegrator`. The
`createdAt` source is resolved in §8.

### 6.2 `IntegrationRunner.Execute(ctx) error` — write path, halts at the PR boundary

```
INPUT: ctx

  1. PLAN        ── compute the same PlanReport as §6.1 (reuse Planner internally, or a shared
                    collect+order helper) so refusal uses the SAME numbers humans saw.
  2. GATE HG6    ── if report.SegmentationBad: REFUSE
                    print "conflict rate X% exceeds max Y%; refusing to integrate" ⇒ return (no mutation).
  3. PER STEP (one at a time, in plan order):
        a. RE-SNAPSHOT ── currentTip = inspector.RevParse(ctx, cfg.BaseBranch)  (develop ADVANCED)
        b. RE-CHECK    ── ahead = inspector.CommitsAhead(ctx, currentTip, branch)
                          if ahead == 0 ⇒ ErrBranchBehind{branch} (already integrated; skip/halt)
        c. RE-PREDICT  ── conflicts, clean, _ = inspector.MergeTreeConflicts(ctx, currentTip, branch)
        d. IF clean:
             integrator.RebaseOnto(ctx, branch, cfg.BaseBranch)
             ── HALT ── print the gh command for ZEUS to open/merge the PR; STOP the loop.
        e. IF NOT clean:
             ── HALT ── print the §11 conflict recipe (rebase locally, resolve, re-run);
                        return ErrMergeConflict{branch, conflicts}.
OUTPUT: nil on a successful single integration up to the PR boundary; typed error otherwise.
```

**The HALT is the architecture.** `Execute` integrates **at most one branch's rebase** and then
stops at the PR boundary — it NEVER loops on to a second branch autonomously and NEVER merges to
`develop`. `integrator_test.go` asserts (a) `RebaseOnto` is called for the clean head step, and
(b) there is **no port method that merges to develop** and none is ever invoked — the mock for
`GitIntegrator` exposes only `RebaseOnto`, so "merge to develop is never called" is provable by
the mock's `AssertNotCalled` having no merge method to assert against plus the type-level absence.

Step b re-checks against the **advanced** tip (the explicit-`base` signatures from §4.1 are what
make this possible). Re-predicting in step c is why a plan computed against a stale snapshot
cannot cause a bad rebase: execute always recomputes against the live tip.

---

## 7. CLI surface

`cmd/mo/main.go` — composition root, the `run(args) error` + subcommand-switch pattern lifted
from `cmd/wt/main.go` (testable entry point separate from `main()`; `main_test.go` drives `run`).

```
mo plan    [--json] [--base develop] [--wt-bin wt] [--depends A:B ...] [--depends-file f]
mo execute [--base develop] [--max-conflict-rate 0.15] [--yes]
mo check   <branch> [--base develop]
```

```go
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mo: %v\n", err)
		os.Exit(exitCodeFor(err))
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: plan | execute | check")
	}
	switch args[0] {
	case "plan":
		return cmdPlan(args[1:])
	case "execute":
		return cmdExecute(args[1:])
	case "check":
		return cmdCheck(args[1:])
	default:
		return fmt.Errorf("unknown subcommand %q; available: plan, execute, check", args[0])
	}
}
```

Each `cmd*` uses its own `flag.NewFlagSet(name, flag.ContinueOnError)`. The composition root
resolves `RepoRoot` (`git rev-parse --show-toplevel`, falling back to `os.Getwd`, verbatim from
the wt module), builds `DefaultTALConfig()`, constructs the adapters (`gitcli.NewInspector(root)`,
`gitcli.NewIntegrator(root)`, `wtcli.NewLister(root, cfg.WtBinary)`), wires `NewPlanner(...)` for
`plan`/`check` and `NewIntegrationRunner(...)` for `execute`, and dispatches.

**`--depends` / `--depends-file` (OPEN QUESTION resolved):** dependency edges are optional (MVP
runs conflict-surface + FIFO always; deps only when supplied — proposal Decision 5).
- `--depends A:B` (repeatable) means "branch A depends on B" → B merges before A.
- `--depends-file <f>` is a **simple one-edge-per-line text file**, format `branch:needs-branch`,
  `#` comment lines and blank lines ignored. No YAML/JSON — zero-deps, trivially diffable, mirrors
  the project's "stdlib only" rule. Both sources merge into the `deps map[string][]string` passed
  to `Order`.

**`execute --yes` (HYBRID posture, Decision 2):** `--yes` confirms intent to mutate (rebase);
without it `execute` prints the plan and refuses to touch anything (dry preview). Even with
`--yes`, it HALTS at the PR boundary — there is no flag that makes it merge to `develop`.

### Sample `mo plan` tabular output

```
MERGE PLAN — base develop @ a1b2c3d  (4 ready, conflict rate 25%  ⚠ exceeds 15%)

#  FIGURA      BRANCH                 AHEAD  PREDICTED  CONFLICTS
1  hephaestus  agent/hephaestus/TAL-7   3    clean      —
2  cronos      agent/cronos/TAL-9       1    clean      —
3  atlas       agent/atlas/TAL-5        5    CONFLICT   platform/api/handler.go
4  iris        agent/iris/TAL-6         2    clean      —

⚠ SegmentationBad: conflict rate 25% > 15% — `mo execute` will refuse until conflicts shrink.
```

### Sample `mo plan --json` shape

```json
{
  "baseBranch": "develop",
  "baseTip": "a1b2c3d",
  "conflictingCount": 1,
  "conflictRate": 0.25,
  "segmentationBad": true,
  "steps": [
    {
      "position": 1,
      "figura": "hephaestus",
      "branch": "agent/hephaestus/TAL-7",
      "head": "deadbee",
      "commitsAhead": 3,
      "predictedClean": true,
      "conflictFiles": []
    },
    {
      "position": 3,
      "figura": "atlas",
      "branch": "agent/atlas/TAL-5",
      "head": "cafef00",
      "commitsAhead": 5,
      "predictedClean": false,
      "conflictFiles": ["platform/api/handler.go"]
    }
  ]
}
```

`check <branch>` is the single-branch slice of `plan`: it runs `MergeTreeConflicts(developTip,
branch)` read-only and prints clean / the conflicting files, no ordering.

---

## 8. Adapter strategy

Two adapters, both thin, both with compile-time interface assertions mirroring the wt mock's
`var _ ... = (*X)(nil)` pattern.

### 8.1 `adapter/gitcli` — over `os/exec`

```go
var _ port.GitInspector  = (*Inspector)(nil)
var _ port.GitIntegrator = (*Integrator)(nil)
```

`Inspector` and `Integrator` each hold `repoRoot` and run git with that working directory
(`exec.CommandContext`, `cmd.Dir = repoRoot`). Argv lives here, never in the service. Exit-code
mapping is the adapter's job:
- `MergeTreeConflicts`: run `git merge-tree --write-tree --name-only <base> <branch>`; inspect
  `*exec.ExitError`.ExitCode() — 0 → clean, 1 → parse stdout lines into `conflicts`, else → err.
- `CommitsAhead`: `git rev-list --count <base>..<branch>`, parse int.
- `ChangedFiles`: `git diff --name-only <base>...<branch>` (three-dot: changes on branch since
  merge-base).
- **`CreatedAt` source (OPEN QUESTION resolved):** `git log -1 --format=%cI <branch>` → the
  committer date of the branch tip, RFC-3339, parsed with `time.Parse`. Committer date (not
  author date) reflects when the branch last advanced, which is the right FIFO signal for "ready
  to merge." This is collected inside the `gitcli` adapter and surfaced to the service as the
  `Candidate.CreatedAt` field (the service does not shell git directly).
- `RebaseOnto`: `git rebase <base> <branch>`; on conflict (`git` exit non-zero with rebase-stop
  signature) parse `git diff --name-only --diff-filter=U` for the unmerged paths → `conflicts`.

**Integration tests** (`inspector_test.go`, `integrator_test.go`): gated by
`if testing.Short() { t.Skip("integration: real git") }`. Setup: `t.TempDir()` → `git init` →
seed `develop` + a couple of agent branches with deterministic commits. Cases: a clean
`MergeTreeConflicts`, a conflicting one (assert exit-1 → paths), `CommitsAhead` counts,
`ChangedFiles` set, `RebaseOnto` clean + conflicting (assert unmerged-path extraction). This is
the ONLY place real git runs for these adapters. `go test -short ./...` skips them; `go test
./...` runs them.

### 8.2 `adapter/wtcli` — shells `wt list --json`

```go
var _ port.WorktreeLister = (*Lister)(nil)
```

`Lister` holds `repoRoot` + `wtBinary`; `List` runs `<wtBinary> list --json` with `cmd.Dir =
repoRoot`, then `json.Unmarshal` the stdout into `[]port.WorktreeEntry`.

**Tests:**
- **Unit (no `wt`, no git):** decode a **golden JSON fixture** (the exact indented array
  `wt list --json` produces — verified against `renderListJSON` in `cmd/wt/main.go`) into
  `[]WorktreeEntry`; assert every field maps. This is the seam-contract test: if `wt`'s JSON
  drifts, this golden fixture is where it's caught.
- **Integration (`testing.Short()`-gated):** invoke a real `wt` binary if present on PATH; skip
  in `-short`. Guards against argv / invocation drift, not field mapping.

---

## 9. Mocks

`mock/` — three hand-written test doubles, each mirroring `GitRunnerMock` exactly:
`Calls []Call` (ordered history), per-method `*Result`/`*Err` programmable fields, `record(...)`,
`CallsFor`, `AssertCallCount`, `AssertMethodOrder`, `AssertNotCalled`, and a `var _ port.X =
(*XMock)(nil)` compile assertion at the bottom.

- `git_inspector_mock.go` — `GitInspectorMock` with programmable results keyed where it matters
  (e.g. `MergeTreeConflictsByBranch map[string](...)`, `CommitsAheadByBranch map[string]int`) so
  a test can make one branch conflict and another clean.
- `git_integrator_mock.go` — `GitIntegratorMock` exposing ONLY `RebaseOnto` (records the call,
  returns programmable conflicts). **It has no merge method — that is the structural proof that
  `mo` cannot merge to develop** (§6.2).
- `worktree_lister_mock.go` — `WorktreeListerMock` returning a programmable `[]WorktreeEntry`.

The `Call`/`record` machinery is copied verbatim (same `Call{Method string; Args []any}` shape)
so service tests assert order with `AssertMethodOrder("Fetch","RevParse","CommitsAhead",...)`.

---

## 10. Cleanup tasks (in-scope, HERMES-owned shared files)

Enumerated for `sdd-tasks`; all within `module:devops`:
1. **`team-context/merge-order.md`** — align the WHOLE file to `develop`. It references `main` on
   **two lines**; `CONSTITUTION §11` (the source of truth) says `develop`. Fix both.
2. **`team-context/ownership.md`** — add a row `platform/merge-order-orchestrator/**` → HERMES,
   `module:devops`, mirroring the existing `wt` row pattern (ownership.md:39).
3. **`.gitignore`** — add `/platform/merge-order-orchestrator/mo` (compiled binary), mirroring the
   `wt` binary ignore.

These touch no file outside HERMES ownership and do NOT touch `ci/pr-checks.yml`.

---

## 11. ADR-style decisions (this phase)

The five proposal decisions are not re-opened; they are formalized here with structural
consequences.

**ADR-M1 — Read/write port split is structural; `plan` cannot mutate (proposal Decision 2,
HYBRID posture, locked by ZEUS).** *Decision:* split the git boundary into `GitInspector`
(read-only) and `GitIntegrator` (a single `RebaseOnto`), and construct `Planner` with ONLY the
read ports. *Rationale:* "`plan`/`check` are side-effect-free" and "`mo` never merges to develop"
(HG3, §10 human gate) become **type-system invariants**, not prose promises — there is no
compilable path from the read use case to a mutating verb, and there is no merge/push/PR method
anywhere in the port surface at all. *Rejected — one `GitRunner` with a `dryRun bool` flag:* a
runtime flag is a guard you can forget to check; a missing method is a guard the compiler enforces.
*Rejected — keep merge-to-develop behind a `--auto-merge` flag:* explicitly out of scope and would
breach HG3; the absence is the feature.

**ADR-M2 — Consume `wt list --json` via shell-out + a local DTO, not a go.mod import (proposal
Decision 1).** *Decision:* `WorktreeEntry` is a local struct in `port/`, field-for-field mirroring
`cmd/wt`'s `listEntry`; `adapter/wtcli` shells `wt list --json` and decodes. *Rationale:* honors
the module boundary (no cross-module Go coupling), and the JSON array is a stable, already-shipped
seam from change #1 (archived). A golden-fixture unit test pins the contract so drift is caught at
the seam. *Rejected — `import` the wt module's types:* couples two independently-owned modules'
`go.mod` and build graphs for one struct; *Rejected — parse `wt list` tabular output:* brittle,
the `--json` flag exists precisely for machine consumption.

**ADR-M3 — Collision prediction via `git merge-tree --write-tree --name-only`, exit-code-mapped
(proposal Decision 3).** *Decision:* `MergeTreeConflicts` runs that command and maps exit 0 →
clean, exit 1 → conflicting paths, other → error. *Rationale:* it is **pure-read** — it computes
the merge in git's object store and writes a throwaway tree, never touching the working directory
or index, so `plan`/`check` are genuinely non-mutating (git ≥ 2.50.1, confirmed available).
*Rejected — trial `git merge --no-commit` then `--abort`:* mutates the working tree/index and
leaves a window where an abort failure corrupts state; *Rejected — diff-overlap heuristic only:*
that's the conflict-surface *ranking* input, not a real conflict prediction — they serve different
roles (ranking vs prediction) and we keep both.

**ADR-M4 — Readiness MVP is git-only: `active` in `wt list` AND `CommitsAhead(develop) > 0`
(proposal Decision 4).** *Decision:* a branch is a merge candidate iff `wt` reports it `active`
and it has commits ahead of the base tip. *Rationale:* CI-green readiness and Jira-link discovery
are deferred to change #4 / future work; the MVP needs only git facts to produce a deterministic
plan. *Rejected — gate on CI status now:* out of scope (#4 ci-hardening, no CI edits in this
change); accepted risk: a branch may be "ready" by git but red in CI — closed by #4.

**ADR-M5 — Ordering is §11 deps → conflict-surface → FIFO, with deps optional via
`--depends`/`--depends-file` (proposal Decision 5).** *Decision:* `Order` always applies
conflict-surface (#2) and FIFO (#3); dependency edges (#1) apply only when supplied. The
`--depends-file` format is a simple one-edge-per-line `branch:needs-branch` text file (comments
with `#`). *Rationale:* the three §11 criteria compose into a **total, deterministic** order;
Jira-link auto-discovery of dependencies is deferred, so explicit edges are the MVP input. A
plain text edge list keeps zero-deps and is trivially diffable/reviewable. *Rejected — YAML/JSON
deps file:* adds a parser/dep or stdlib `encoding/json` ceremony for what is a list of pairs;
*Rejected — auto-discover deps from Jira links now:* explicitly deferred in the proposal.

---

## 12. Testing strategy (strict TDD — `go test ./...`)

Strict TDD is active: every domain/service file and every adapter behavior gets a **failing test
first (RED) → minimal implementation (GREEN) → refactor**. The split mirrors where the value is
(§1): the bulk is the pure `Order` algorithm and report arithmetic, fast and exhaustive.

**Unit — `domain/mergeorder/` (the bulk; zero git, table-driven):**
- `order_test.go` — the algorithm exhaustively: (a) topo correctness (deps merge first);
  (b) **cycle → `ErrDependencyCycle`** with the right branches; (c) conflict-surface tiebreak
  (fewer/disjoint `ChangedFiles` first within a layer); (d) FIFO tiebreak by `CreatedAt` then
  `Branch`; (e) **determinism: shuffle the input N times, assert byte-identical output every
  time** (the total-order proof); (f) empty input → `ErrNoCandidates`.
- `plan_test.go` — `ConflictRate`/`SegmentationBad` arithmetic: 0 steps (no divide-by-zero, rate
  0), all clean (rate 0, not bad), mixed (correct fraction), exactly-at-threshold vs over (the
  `> MaxConflictRate` boundary).
- `candidate_test.go` / `errors_test.go` — value-object construction + each error's `Error()`.

**Unit — `service/planner_test.go` + `service/integrator_test.go` (3 mocks, zero git):**
- `Plan` happy: assert call ORDER `List → Fetch → RevParse → (CommitsAhead, ChangedFiles per
  branch) → MergeTreeConflicts per step`; assert `active` filtering and `ahead==0` drop; assert
  the `PlanReport` numbers.
- `Plan` errors: empty inventory → `ErrNoCandidates`; cycle in deps → `ErrDependencyCycle`
  propagated from `Order`.
- `Execute` refusal: a mock inventory that makes `SegmentationBad` true ⇒ assert
  `GitIntegratorMock.AssertNotCalled` for `RebaseOnto` (refused, ZERO mutation).
- `Execute` clean head: re-`RevParse` against advanced tip → clean → `RebaseOnto` called once →
  HALT (assert it does NOT proceed to step 2). **Assert no merge-to-develop method exists/was
  called** — structurally guaranteed because the integrator mock exposes only `RebaseOnto`.
- `Execute` conflict: `MergeTreeConflicts` returns conflicts ⇒ `ErrMergeConflict`, `RebaseOnto`
  NOT called.
- `Execute` branch-behind: `CommitsAhead==0` at re-check ⇒ `ErrBranchBehind`.

**Integration — `adapter/gitcli/*_test.go` + `adapter/wtcli/lister_test.go` (`testing.Short()`-gated):**
- Guard top of each: `if testing.Short() { t.Skip("integration: real git/wt") }`.
- `gitcli`: `t.TempDir()` real repo; clean vs conflicting `MergeTreeConflicts` (exit 0/1
  mapping), `CommitsAhead`, `ChangedFiles`, `CreatedAt` via `git log -1 --format=%cI`,
  `RebaseOnto` clean + conflicting (unmerged-path extraction).
- `wtcli`: **unit** golden-JSON decode (no `wt`, runs in `-short`); **integration** real `wt`
  invocation (gated).

**What's covered without git/wt:** the entire ordering algorithm + determinism contract, all
report arithmetic, the full plan/execute orchestration including the refusal gate, the
one-at-a-time re-check, the HALT, and the structural "never merges to develop" proof — all via
mocks. **What needs real git/wt:** argv correctness, exit-code mapping, `CreatedAt` extraction,
real rebase side effects, and the `wt list --json` invocation — all in `testing.Short()`-gated
files. `go test -short ./...` = fast suite; `go test ./...` = full.

---

## 13. Decisions tasks must respect (and spec-parallelism seams)

1. **Strict-TDD order:** `domain/mergeorder` FIRST and exhaustively (errors, candidate, the
   `Order` algorithm + determinism, plan arithmetic), then the two services against the 3 mocks,
   then the adapters against real git/`wt` (short-gated). `cmd/mo` last.
2. **Dependency arrows:** `service` never imports `adapter/*`; `cmd/mo` is the only composition
   root; both adapters import `port`+`domain`, never `service`. Any task wiring git/`wt` into the
   service is wrong.
3. **Read/write split (ADR-M1):** `Planner` is constructed with read ports ONLY. Do NOT give it
   `GitIntegrator`. There is NO merge/push/PR method anywhere — do not add one.
4. **Zero transitive deps:** hand-written adapters (`os/exec`), hand-rolled CLI (`flag`), three
   hand-written mocks, stdlib `testing` + `encoding/json` + `time`. `go.sum` stays empty.
5. **Operation-shaped ports:** semantic methods, no generic `Run(args...)`. The 3 mocks implement
   all methods.
6. **`MergeTreeConflicts` exit mapping (ADR-M3):** 0→clean, 1→paths, other→err. Pure-read; never
   `git merge --no-commit`.
7. **Explicit `base` params:** `CommitsAhead`/`ChangedFiles`/`MergeTreeConflicts` take `base` so
   execute re-checks against the ADVANCED tip. Do not hardcode "develop" inside the adapter.
8. **HYBRID execute (ADR-M1/Decision 2):** `execute` HALTS at the PR boundary after at most one
   rebase, printing the `gh` command; it NEVER loops to a second branch and NEVER merges. The
   test MUST assert merge-to-develop is never called.
9. **`--depends-file` format:** one edge per line `branch:needs-branch`, `#` comments, blank lines
   ignored. No YAML/JSON.
10. **`CreatedAt` source:** `git log -1 --format=%cI <branch>` (committer date), parsed in the
    `gitcli` adapter; the service receives it as `Candidate.CreatedAt`.
11. **Cleanup (§10):** fix BOTH `main` lines in `team-context/merge-order.md`; add the
    `ownership.md` row; add the `.gitignore` binary line. Do NOT touch `ci/pr-checks.yml`.

**Seams with `sdd-spec`:**
- Spec owns the **functional requirements** (REQ-* for each subcommand's observable behaviour,
  exit codes, the `--json` shape, error surfaces, the gate-HG6 refusal message). This design owns
  the **structure** (the three ports, the read/write split, the `Order` algorithm placement, the
  test split).
- **Potential conflict to watch:** if spec writes a REQ that `execute` auto-merges to `develop`,
  or that `plan` mutates anything, or that a single `GitRunner` port is used, it contradicts
  ADR-M1 here — `sdd-tasks` (which reads BOTH) must reconcile in favour of these locked design
  decisions, or escalate. This design treats the proposal's 5 decisions as RESOLVED; spec should
  formalize the *requirements* of those resolutions, not re-open them.
