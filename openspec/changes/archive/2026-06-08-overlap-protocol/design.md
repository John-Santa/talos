# Design — overlap-protocol (TAL-4)

> Change #3 · Fase 2 · `module:qa` · owner THEMIS · branch `agent/themis/TAL-4` (base `develop`)
> **Store:** hybrid (this file + engram `sdd/overlap-protocol/design`)
> **Reads:** proposal `[[sdd/overlap-protocol/proposal]]` (7 resolved decisions, FULL scope, 3 open Qs)
> **Module path:** `github.com/John-Santa/talos/platform/overlap-guard`
> **Mirror references (verified against real code):** `platform/merge-order-orchestrator/` (hexagonal, zero deps, read-only `Planner`, `exitCodeFor`), `platform/jira-evidence-loop/adapter/rest/client.go` (`Search`+auth+`HTTPError`).

This is the HOW at the architectural level — it locks ports/adapters/domain so strict TDD can start. The WHAT (checklist grammar, fallback semantics) is owned by `sdd-spec` (running in parallel). Signatures below were verified compile-plausibly against the real `mo`/`evidence` code.

---

## 1. Architecture approach

Hexagonal (ports & adapters), the **same skeleton as `merge-order-orchestrator`**, zero deps, go 1.26. Dependency arrow points inward: `cmd → service → port ← adapter`; everything depends on `domain`; domain is pure (no `os/exec`, no `http`, no `time.Now()`).

This module inherits `mo`'s central invariant (ADR-M1): the use-case holds **only read ports**. `Guard` depends on `IssueSearcher` + `WorktreeLister` + `GitInspector` (all read) and **cannot compile a mutating call** — there is no write port in this module at all. "`ov` never touches the working dir / never mutates Jira" is enforced by the type system, not promised in prose (ADR-OV4).

```
            ┌─────────────────────────────────────────────────────┐
            │              domain/overlap                          │  pure, no deps
            │  Claim · ClaimSource · FileCollision · ModuleOverlap │  value objects +
            │  Verdict · Report · NewReport(claims,threshold)      │  collision algos
            │  FileCollisions · ModuleOverlaps                     │  + parsers
            │  ParseFilesChecklist · BuildPreAssignmentJQL · errs  │  (NO I/O)
            └─────────────────────────────────────────────────────┘
                  ▲                ▲                   ▲
        imports   │                │                   │   imports
   ┌──────────────┴────────────────┴───────────────────┴──────────┐
   │ service/Guard  (READ ONLY — IssueSearcher + WorktreeLister + │
   │                 GitInspector; NO write port exists)          │
   │   CheckPreAssignment · ScanInFlight · Metric                 │
   └───────────────────────────┬──────────────────────────────────┘
                               │ depends on
                               ▼
   ┌──────────────────────────────────────────────────────────────┐
   │  port/  IssueSearcher · WorktreeLister · GitInspector         │  mockable seam
   └──────────────────────────────────────────────────────────────┘
        ▲              ▲              ▲                    ▲
 adapter/jirarest  adapter/wtcli  adapter/gitcli    cmd/ov (composition root)
   (http, +desc,    (shell wt      (own ~50ln        switch · FlagSet · --json
    ADF flatten)     list --json)   Fetch/RevParse/   · exitCodeFor)
                                    ChangedFiles)
```

**Boundary rules:** `service` imports `port`+`domain`, never `adapter/*`. Adapters import `port`+`domain`, never `service`. `cmd/ov` is the only place concrete adapters are constructed. Every adapter and mock carries `var _ port.X = (*T)(nil)`.

## 2. Package layout

```
platform/overlap-guard/
├── go.mod                         module github.com/John-Santa/talos/platform/overlap-guard
├── domain/overlap/  claim.go · collision.go · report.go · parse.go · jql.go · errors.go (+ _test)
├── port/            issue_searcher.go · worktree_lister.go · git_inspector.go
├── service/         config.go · guard.go (+ _test)
├── adapter/jirarest/ client.go (+ integration _test)
├── adapter/wtcli/    lister.go
├── adapter/gitcli/   inspector.go (own, mirror not import)
├── mock/            issue_searcher_mock.go · worktree_lister_mock.go · git_inspector_mock.go
└── cmd/ov/          main.go
```

## 3. Domain signatures (godoc only on exported)

```go
// domain/overlap

// ClaimSource distinguishes a declared (T0 checklist) claim from an actual (T1 git) claim.
type ClaimSource int
const ( SourceDeclared ClaimSource = iota; SourceActual )

// Claim is one agent's set of files in play, from a Jira checklist (T0) or a worktree diff (T1).
type Claim struct { Agent, Module, Ref string; Files []string; Source ClaimSource }

// NewClaim builds a Claim with files normalized (trimmed, deduped, sorted) for deterministic comparison.
func NewClaim(agent, module, ref string, files []string, src ClaimSource) Claim

// FileCollision is the same file claimed by two distinct agents — the hard BLOCK case.
type FileCollision struct { File string; A, B Claim }

// FileCollisions returns every same-file collision across distinct-agent claims (pairwise).
func FileCollisions(claims []Claim) []FileCollision

// ModuleOverlap is two distinct agents touching the same module without a file collision — the soft SERIALIZE case.
type ModuleOverlap struct { Module string; A, B Claim }

// ModuleOverlaps returns module-level overlaps among distinct agents, excluding pairs already in FileCollisions.
func ModuleOverlaps(claims []Claim) []ModuleOverlap

// Verdict is the precedence-ordered overlap outcome: BLOCK > SERIALIZE > OK.
type Verdict int
const ( VerdictOK Verdict = iota; VerdictSerialize; VerdictBlock )

// Report is the full overlap evaluation: verdict, the two collision kinds, and the HG6 collision rate.
type Report struct {
	Verdict        Verdict
	FileCollisions []FileCollision
	ModuleOverlaps []ModuleOverlap
	ClaimCount     int
	CollisionRate  float64
	OverThreshold  bool
}

// NewReport evaluates claims, derives the Verdict by precedence, and computes CollisionRate over distinct-agent pairs.
// OverThreshold is true when CollisionRate is STRICTLY greater than threshold (mirrors mo's 0.15 boundary).
func NewReport(claims []Claim, threshold float64) Report

// ParseFilesChecklist extracts file paths from a Jira body's `files:` checklist (grammar fixed by spec).
func ParseFilesChecklist(body string) []string

// BuildPreAssignmentJQL builds the overlap-protocol.md JQL for the T0 gate, excluding the owner's own agent label.
func BuildPreAssignmentJQL(project, module, owner string) string
```

Typed errors (`errors.go`, mirror `mo`): `ErrSameFileParallel{A,B Claim; File string}`, `ErrEscalateZeus{Reason string}`, `ErrChecklistMissing{IssueKey string}`, `ErrNoClaims{}`. `ErrChecklistMissing` is **advisory** — never silently treated as "no overlap" (spec invariant).

## 4. Port signatures

```go
// port

// IssueResult is one issue row from the T0 Jira search: key, labels, and flattened-ADF body.
type IssueResult struct { Key string; Labels []string; Body string }

// IssueSearcher is the read-only outbound port for the T0 pre-assignment Jira search.
type IssueSearcher interface { Search(ctx context.Context, jql string, maxResults int) ([]IssueResult, error) }

// WorktreeEntry mirrors the JSON shape emitted by `wt list --json` (change #1).
type WorktreeEntry struct { Figura, Branch, Path, Head, Status string `json:...` }

// WorktreeLister is the outbound port for listing agent worktrees via the wt binary.
type WorktreeLister interface { List(ctx context.Context) ([]WorktreeEntry, error) }

// GitInspector is the read-only outbound port for git inspection (own, not mo's).
type GitInspector interface {
	Fetch(ctx context.Context) error
	RevParse(ctx context.Context, ref string) (string, error)
	ChangedFiles(ctx context.Context, base, branch string) ([]string, error)
}
```

## 5. Service signatures

```go
// service

// Config holds runtime config for ov; mirrors mo's DefaultTALConfig seeding.
type Config struct { RepoRoot, BaseBranch, WtBinary, SiteURL, Project string; Threshold float64; NoFetch bool }

// DefaultTALConfig returns Config seeded with Talos defaults (base develop, wt, project TAL, threshold 0.15).
func DefaultTALConfig() Config

// Guard runs the three overlap use-cases using ONLY read ports (no write port exists — ADR-OV4).
type Guard struct { searcher port.IssueSearcher; lister port.WorktreeLister; inspector port.GitInspector; cfg Config }

// NewGuard wires the read ports and config.
func NewGuard(s port.IssueSearcher, l port.WorktreeLister, g port.GitInspector, cfg Config) *Guard

// CheckPreAssignment runs the T0 gate: JQL search → parse each checklist → cross with the owner's declared files.
func (g *Guard) CheckPreAssignment(ctx context.Context, module, owner string, ownerFiles []string) (overlap.Report, error)

// ScanInFlight runs the T1 scan: wt list → per-branch ChangedFiles(base...branch) → pairwise file collisions across siblings.
func (g *Guard) ScanInFlight(ctx context.Context) (overlap.Report, error)

// Metric runs ScanInFlight and returns its Report for the HG6 collision-rate gate.
func (g *Guard) Metric(ctx context.Context) (overlap.Report, error)
```

## 6. CLI surface (`cmd/ov/main.go`)

| Subcommand | FlagSet | Verdict→exit |
|---|---|---|
| `ov check --module X --agent owner [--files-file f] [--site-url u] [--json]` | `check` | BLOCK→1, OK/SERIALIZE→0 |
| `ov scan [--base develop] [--wt-bin wt] [--no-fetch] [--ownership-file f] [--json]` | `scan` | BLOCK→1, else→0 |
| `ov metric [--threshold 0.15] [--strict] [scan flags] [--json]` | `metric` | OverThreshold && `--strict`→1, else→0 |

`run([]string) error` + manual `switch` (`check`/`scan`/`metric`), one `flag.NewFlagSet` per subcommand, `--json` via `json.NewEncoder(os.Stdout)`+`SetIndent("", "  ")`. `exitCodeFor(err)`: `ErrNoClaims`→0 (empty is success, mirror of `ErrNoCandidates`), everything else→1. `repoRoot()` via `git rev-parse --show-toplevel` (mirror `mo`). Composition root constructs `jirarest`/`wtcli`/`gitcli` and wires `Guard`.

## 7. Architecture decisions (ADRs)

| ID | Decision | Alternatives rejected | Rationale |
|---|---|---|---|
| **OV1** | `gitcli` self-contained: own `Fetch`/`RevParse`/`ChangedFiles` (~50 ln, mirror of `mo`'s `inspector.go`) | import `merge-order-orchestrator/adapter/gitcli` | `platform/merge-order-orchestrator/**` is single-writer HERMES; cross-CLI = seam, **never import Go**. Only the 3 verbs `ov` needs are re-implemented — no `MergeTree`/`CommitsAhead`/`BranchCreatedAt`. |
| **OV2** | `jirarest` requests `["summary","labels","description"]` and flattens ADF locally via a minimal `adfNode{Type,Text string; Content []adfNode}` walk collecting `content[].text` | reuse `evidence.ADFDocument`; ask Jira for plain-text rendering | `evidence`'s `Search` asks only `[summary,labels]` (client.go ~L174) — verified risk #3. Importing `evidence` couples go.mod and pulls write methods; a local `adfNode` keeps `ov` zero-coupled. Walk is depth-first, joins text with `\n` so `ParseFilesChecklist` sees line structure. |
| **OV3** | T1 module attribution is **optional**. `scan`/`metric` need NO module: same-file→BLOCK is pairwise over files. `--ownership-file` (figura→module map) only enriches SERIALIZE classification in the report | derive module from branch; make module mandatory | Resolves risk #2: the branch (`agent/<figura>/TAL-N`) yields figura, not module. The hard rule never depends on module, so the MVP is correct at file level alone; module is reporting sugar. `figuraFromBranch` (mirror `mo`) extracts the agent for pairing. |
| **OV4** | `Guard` is read-only by **construction**: it holds only `IssueSearcher`/`WorktreeLister`/`GitInspector`; the module ships **no write port** | document "read-only" in prose; add a guarded write port | Mirror of `mo`'s `Planner` (ADR-M1). Type system proves `ov` cannot mutate Jira or the working dir — `scan` cannot rebase, `check` cannot transition. Out-of-scope mutations are unreachable, not merely undone. |
| **OV5** | `CollisionRate` over-threshold = **strictly >** `threshold` | `>=` | Coherent with `mo`'s `NewPlanReport` (`rate > maxConflictRate`) and overlap-protocol.md "> ~15%". |

## 8. Data flow

```
T0  ov check ─ BuildPreAssignmentJQL ─→ IssueSearcher.Search ─→ [IssueResult]
            ─ per body ─ ParseFilesChecklist ─→ declared Claims (+ owner Claim from --files-file)
            ─ FileCollisions/ModuleOverlaps ─→ NewReport ─→ Verdict ─→ exit

T1  ov scan ─ WorktreeLister.List ─ filter status=active ─ [GitInspector.Fetch] ─→ RevParse(base)
            ─ per sibling ─ ChangedFiles(base...branch) ─→ actual Claims (agent=figura)
            ─ FileCollisions (pairwise) ─→ NewReport ─→ Verdict ─→ exit

HG6 ov metric ─ (== scan) ─→ Report.CollisionRate/OverThreshold ─ informative unless --strict
```

## 9. PR slicing (~2060 ln → 2 chained PRs, mirror of #2 PR#7+#8)

| PR | Targets | Files | Lines (est) | Verification |
|---|---|---|---|---|
| **PR1** domain+service+mocks (pure, no I/O) | `agent/themis/TAL-4` → base | `domain/overlap/*.go`, `port/*.go`, `mock/*.go`, `service/config.go`+`guard.go`, all `_test.go` | ~1250 | `go test ./...` green, table-driven, all collision/verdict/parse/JQL paths |
| **PR2** adapters+cmd+I/O+gated edits | PR1 branch | `adapter/{jirarest,wtcli,gitcli}/*.go`, `cmd/ov/main.go`, `go.mod`, gated `ownership.md`+`.gitignore` | ~810 | `go test ./...`; integration tests `-short`-skippable |

PR1 stands alone (pure domain+service compiles and tests against mocks with no adapters). PR2 adds the real I/O. Both under the 400-line reviewer budget individually is **not** met (each PR exceeds 400) → chained PRs is the delivery mechanism the proposal already locked; `sdd-tasks` carries the budget forecast.

## 10. Testing strategy

| Layer | What | Approach |
|---|---|---|
| Domain | `FileCollisions`/`ModuleOverlaps` (distinct-agent only, pairwise, dedupe), `NewReport` precedence BLOCK>SERIALIZE>OK, `CollisionRate` boundary (strictly-greater), `ParseFilesChecklist` (grammar + missing→empty), `BuildPreAssignmentJQL` string, typed errors | pure table-driven, zero I/O |
| Service | `Guard.CheckPreAssignment`/`ScanInFlight`/`Metric` flows: call order, status=active filter, `ErrChecklistMissing` advisory excluded from file-level but kept at module-level, `--no-fetch` skips `Fetch`, empty→`ErrNoClaims` | hand-written mocks (`Calls []Call`, per-key result maps), `AssertMethodOrder`/`AssertNotCalled` |
| Adapters | `jirarest` ADF flatten + `description` in fields; `wtcli` shell `wt list --json` decode + malformed; `gitcli` `ChangedFiles`/`RevParse`/`Fetch` against real git | `httptest.Server` for jirarest; real `git`/`wt` integration gated by `if testing.Short() { t.Skip() }` |
| CLI | `run(args)` subcommand switch, `exitCodeFor` mapping, `--json` shape | `run`-level tests with mocked ports where wiring allows |

## 11. Migration / rollout

No migration. New module, additive. Gated shared edits (`team-context/ownership.md` row + `.gitignore` entry) require ATHENA sign-off and land in PR2. `ov` binary git-ignored.

## 12. Open questions (for spec / non-blocking)

- [ ] **Checklist grammar** (`- [ ] path`) and `ErrChecklistMissing` advisory semantics — owned by `sdd-spec` (open Q #1). Design assumes `ParseFilesChecklist` returns `[]string` and empty ≠ "no overlap".
- [ ] None block the architecture: ports/adapters/domain are fully closed for strict TDD.
