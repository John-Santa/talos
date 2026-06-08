# Tasks: merge-order-automation

**Change:** merge-order-automation · **Jira:** TAL-3 · **Module:** module:devops · **Owner:** HERMES
**Branch:** `agent/hermes/TAL-3` · **Store:** hybrid (this file + engram `sdd/merge-order-automation/tasks`)
**TDD mode:** Strict — RED before GREEN for every domain/service/adapter behavioral file.
**Boundary:** DO NOT touch `ci/pr-checks.yml` (LOCKED). DO NOT add merge/push/PR methods to any port.

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–950 LOC |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → go.mod + domain/mergeorder (all 4 files + tests) + port (3 files) + mock (3 files) + service (config, planner, integrator) + all unit tests; PR 2 → adapter/gitcli (inspector, integrator + tests) + adapter/wtcli (lister + tests) + cmd/mo/main.go + cleanup (.gitignore, ownership.md, merge-order.md) |
| Delivery strategy | ask-on-risk |
| Decision needed before apply | **Yes** — confirm single PR (size:exception) vs two chained PRs before sdd-apply starts |

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | go.mod + domain/mergeorder + port + mock + service + all unit tests | PR 1 | Zero git, zero disk, zero live calls; `go test -short ./...` green |
| 2 | adapter/gitcli + adapter/wtcli + cmd/mo + cleanup files | PR 2 | Base = PR 1 branch; real git needed for integration tests only |

---

## Reconciliation notes (spec ↔ design conflicts, resolved in favor of design ADRs)

- **REQ-READINESS-1** uses the phrase "local DTO" — design clarifies this DTO is `port.WorktreeEntry`, not a domain type. Resolved: task 2.3 places `WorktreeEntry` in `port/worktree_lister.go`.
- **Spec error catalogue (REQ-ERR-1)** lists `ErrNoCandidates`, `ErrBranchBehind`, `ErrMergeConflict` as domain errors and `ErrWtBinaryNotFound`, `ErrWtOutputMalformed`, `ErrSegmentationBad`, `ErrDevelopNotAvailable`, `ErrBranchNotFound`, `ErrRebaseConflict` as adapter/service errors. Design places all typed structs in `domain/mergeorder/errors.go`. Resolved: all live in `domain/mergeorder/errors.go` — the domain package is the error catalogue for all layers (mirrors `wt` pattern). Adapter/service errors not in the design struct list (`ErrWtBinaryNotFound`, `ErrWtOutputMalformed`, `ErrDevelopNotAvailable`, `ErrBranchNotFound`, `ErrRebaseConflict`) are added as typed structs in the same file.
- **ADR-M1 (locked):** `Planner` holds ONLY read ports. No path exists from plan/check to `GitIntegrator.RebaseOnto`. No merge/push/PR method anywhere in the port surface. Any spec text that implies a single `GitRunner` is reconciled to three split ports per design.

---

## Phase 0: Repo-level scaffolding (sequential, no RED/GREEN — no logic)

- [ ] **0.1** Add `/platform/merge-order-orchestrator/mo` to `.gitignore` after the `/platform/worktree-orchestrator/wt` line. No test needed — verified by `git status`. **REQ-CLEANUP-3, design §10**

- [ ] **0.2** Fix `team-context/merge-order.md`: replace ALL occurrences of `main` as the integration branch with `develop`. Two lines require editing — line 3 (header: `contra \`main\``) and line 7 (bullet: `contra \`main\`, **o** rebase-sobre-\`main\``). Zero occurrences of `main` as a merge target MUST remain after this edit. **REQ-CLEANUP-1, design §10**

- [ ] **0.3** Update `team-context/ownership.md`: add row `platform/merge-order-orchestrator/**` → HERMES, module:devops, note `CLI \`mo\`, dominio, servicio, adapters gitcli/wtcli` to the shared-file map, mirroring the existing `platform/worktree-orchestrator/**` pattern (line 40). **REQ-CLEANUP-2, design §10**

---

## Phase 1: Module init + domain (PR 1)

### 1.1 — Module scaffold (sequential, no RED/GREEN)

- [x] **1.1** Create `platform/merge-order-orchestrator/go.mod` with module path `github.com/John-Santa/talos/platform/merge-order-orchestrator`, go 1.26, zero transitive dependencies. Mirrors `worktree-orchestrator` Kit goal — `go.sum` stays empty. **design §2**

### 1.2 — Typed errors (prerequisite; no RED/GREEN, pure declarations)

- [x] **1.2** Write `domain/mergeorder/errors.go`: ten typed error structs implementing `error`, zero I/O, zero imports beyond `fmt`. Domain-layer structs (per design §5.2): `ErrDependencyCycle{Branches []string}`, `ErrNoCandidates{}`, `ErrBranchBehind{Branch string}`, `ErrMergeConflict{Branch string; Files []string}`. Additional typed errors from the full spec catalogue (REQ-ERR-1, reconciled into this file per `wt` pattern): `ErrWtBinaryNotFound{Binary string}`, `ErrWtOutputMalformed{Fragment string; Cause error}`, `ErrSegmentationBad{Rate float64; Threshold float64}`, `ErrDevelopNotAvailable{}`, `ErrBranchNotFound{Branch string}`, `ErrRebaseConflict{Branch string; Files []string}`. **REQ-ERR-1, design §5.2**

### 1.3 — Domain: candidate value object (RED → GREEN)

- [x] **1.3a [RED]** Write `domain/mergeorder/candidate_test.go`: table-driven tests for `Candidate` construction — zero-value defaults, all fields populated, `Error()` on each error type from 1.2 (string contains the key field). All tests must fail. **REQ-ERR-1, REQ-TEST-6, design §5.1**

- [x] **1.3b [GREEN]** Write `domain/mergeorder/candidate.go`: `Candidate` struct with fields `Figura, Branch, Head, Path string`, `CommitsAhead int`, `ChangedFiles []string`, `CreatedAt time.Time`. Also write `domain/mergeorder/errors_test.go` (each error's `Error()` returns a non-empty string containing its key field; all 10 types). All 1.3a tests must pass. **REQ-ERR-1, REQ-TEST-6, design §5.1**

### 1.4 — Domain: ordering algorithm (RED → GREEN)

- [x] **1.4a [RED]** Write `domain/mergeorder/order_test.go`: the exhaustive table-driven algorithm tests (no git, no I/O). Named cases required:
  - `empty input → ErrNoCandidates`
  - `single candidate, no deps → returned as-is`
  - `no deps, tiebreak by ChangedFiles count ascending`
  - `no deps, conflict-surface tie → FIFO by CreatedAt ascending`
  - `no deps, CreatedAt tie → Branch lexicographic ascending`
  - `simple chain A→B deps (B depends on A, A merges first) → A before B`
  - `diamond dependency (A→C, B→C → C before A and B)`
  - `cycle A→B→A → ErrDependencyCycle{Branches includes A and B}`
  - `cycle longer chain A→B→C→A → ErrDependencyCycle`
  - `determinism: shuffle input 10 times, assert identical output every run`
  - `disjoint ChangedFiles ranked before overlapping within same topo layer`
  All tests must fail. **REQ-ORDER-1..4, REQ-TEST-1, REQ-TEST-2, design §5.3**

- [x] **1.4b [GREEN]** Write `domain/mergeorder/order.go`: `Order(candidates []Candidate, deps map[string][]string) ([]Candidate, error)` with the three-criterion algorithm (topo layers via colored DFS → conflict-surface score → FIFO `CreatedAt`+`Branch`). Pure, zero I/O, zero third-party imports. All 1.4a tests must pass. **REQ-ORDER-1..4, REQ-TEST-1, REQ-TEST-2, design §5.3**

### 1.5 — Domain: plan report arithmetic (RED → GREEN)

- [x] **1.5a [RED]** Write `domain/mergeorder/plan_test.go`: table-driven tests for `Step`, `MergePlan`, `PlanReport` value objects and `NewPlanReport` constructor. Named cases required:
  - `zero steps → ConflictRate 0.0, SegmentationBad false (no divide-by-zero)`
  - `all steps clean → rate 0.0, bad false`
  - `one conflict of two → rate 0.5, bad true (0.5 > 0.15)`
  - `one conflict of four → rate 0.25, bad true`
  - `exactly at threshold 0.15 (1 conflict of ~6.66 steps is not exact; use 15 conflicts of 100) → bad false`
  - `threshold 0.1501 → bad true` (strict greater-than; REQ-HEALTH-2)
  - `zero conflicts → rate 0.0, bad false`
  - `100% conflicts → rate 1.0, bad true`
  All tests must fail. **REQ-HEALTH-1..3, REQ-TEST-1, REQ-TEST-3, design §5.1**

- [x] **1.5b [GREEN]** Write `domain/mergeorder/plan.go`: `Step{Position int; Candidate Candidate; PredictedClean bool; ConflictFiles []string}`, `MergePlan{BaseBranch, BaseTip string; Steps []Step}`, `PlanReport{Plan MergePlan; ConflictingCount int; ConflictRate float64; SegmentationBad bool}`, and `NewPlanReport(plan MergePlan, maxConflictRate float64) PlanReport` (computes `ConflictingCount`, `ConflictRate = ConflictingCount/len(Steps)` guarded on 0, `SegmentationBad = ConflictRate > maxConflictRate`). All 1.5a tests must pass. **REQ-HEALTH-1..3, REQ-TEST-1, REQ-TEST-3, design §5.1**

---

## Phase 2: Ports (sequential — depends on domain 1.2; pure interface declarations, no RED/GREEN)

- [x] **2.1** Write `port/git_inspector.go`: `GitInspector` interface with 6 methods: `Fetch(ctx) error`, `RevParse(ctx, ref string) (string, error)`, `MergeBase(ctx, a, b string) (string, error)`, `CommitsAhead(ctx, base, branch string) (int, error)`, `MergeTreeConflicts(ctx, base, branch string) (conflicts []string, clean bool, err error)`, `ChangedFiles(ctx, base, branch string) ([]string, error)`. Godoc on each method exactly as in design §4.1. **REQ-COLLISION-1, REQ-ORDER-1, ADR-M1, design §4.1**

- [x] **2.2** Write `port/git_integrator.go`: `GitIntegrator` interface with exactly ONE method: `RebaseOnto(ctx, branch, base string) (conflicts []string, err error)`. Godoc notes no merge/push/PR method exists by design (HG3 structural enforcement). **REQ-EXECUTE-6, ADR-M1, design §4.2**

- [x] **2.3** Write `port/worktree_lister.go`: `WorktreeEntry` struct with five JSON-tagged fields mirroring `wt`'s `listEntry` exactly (`figura`, `branch`, `path`, `head`, `status`), and `WorktreeLister` interface with `List(ctx) ([]WorktreeEntry, error)`. Add `var _ WorktreeLister = (*struct{})(nil)` compile stub comment. **REQ-READINESS-1, ADR-M2, design §4.3**

---

## Phase 3: Mocks (sequential — depends on port 2.1–2.3)

- [x] **3.1** Write `mock/git_inspector_mock.go`: `GitInspectorMock` implementing `port.GitInspector`. Fields: `Calls []Call`, `FetchErr error`, `RevParseResults map[string]string`, `RevParseErrs map[string]error`, `MergeBaseResult string`, `MergeBaseErr error`, `CommitsAheadByBranch map[string]int`, `CommitsAheadErrs map[string]error`, `MergeTreeConflictsByBranch map[string][]string` (non-nil slice = conflicting), `ChangedFilesByBranch map[string][]string`. Exposes `record(method, args...)`, `CallsFor(method)`, `AssertCallCount`, `AssertMethodOrder`, `AssertNotCalled`. Compile assertion `var _ port.GitInspector = (*GitInspectorMock)(nil)`. Mirrors `GitRunnerMock` pattern exactly. **REQ-TEST-4, design §9**

- [x] **3.2** Write `mock/git_integrator_mock.go`: `GitIntegratorMock` implementing `port.GitIntegrator`. ONE method: `RebaseOnto` (records call, returns programmable `RebaseConflicts []string`, `RebaseErr error`). Same `Calls []Call` + `AssertMethodOrder` + `AssertNotCalled` pattern. `var _ port.GitIntegrator = (*GitIntegratorMock)(nil)`. This mock has NO merge method — its absence is the structural proof that `mo` cannot merge to develop. **REQ-TEST-7, ADR-M1, design §9**

- [x] **3.3** Write `mock/worktree_lister_mock.go`: `WorktreeListerMock` implementing `port.WorktreeLister`. Fields: `Calls []Call`, `ListResult []port.WorktreeEntry`, `ListErr error`. `var _ port.WorktreeLister = (*WorktreeListerMock)(nil)`. **REQ-TEST-4, design §9**

---

## Phase 4: Service (RED → GREEN — depends on domain + port + mock)

### 4.1 — Config (no RED/GREEN, pure declaration)

- [x] **4.1** Write `service/config.go`: `Config` struct with fields `RepoRoot, BaseBranch, WtBinary string` and `MaxConflictRate float64`; `DefaultTALConfig() Config` returning `{BaseBranch:"develop", WtBinary:"wt", MaxConflictRate:0.15}`. Constructors `NewPlanner` and `NewIntegrationRunner` accept this config. **design §3**

### 4.2 — Planner service (RED → GREEN)

- [x] **4.2a [RED]** Write `service/planner_test.go`: mock-backed, zero git, zero disk. Test cases:
  - **Plan happy path:** mock lister returns 3 active entries (1 orphan dropped), mock CommitsAhead returns 0 for one (dropped), 2 remain → assert call ORDER `List → Fetch → RevParse → CommitsAhead (× active branches) → ChangedFiles (× candidates ahead) → MergeTreeConflicts (× ordered steps)`; assert `PlanReport` fields.
  - **Plan --no-fetch:** assert `Fetch` NOT called when `NoFetch` config option is set.
  - **Plan empty inventory → ErrNoCandidates** (all orphan or 0-ahead).
  - **Plan cycle in deps → ErrDependencyCycle** propagated from `Order`.
  - **Plan SegmentationBad flag true** when mocked conflicts exceed threshold.
  - **Check single branch:** `RevParse → MergeTreeConflicts(base, branch)` only; no lister, no ordering. Returns clean or conflicting.
  - **Check unknown branch → ErrBranchNotFound** (mock RevParse returns error).
  All tests must fail. **REQ-PLAN-1..4, REQ-CHECK-1..4, REQ-HEALTH-1..3, REQ-FETCH-1..2, REQ-TEST-1, REQ-TEST-4, design §6.1**

- [x] **4.2b [GREEN]** Write `service/planner.go`: `Planner` struct holding `port.GitInspector`, `port.WorktreeLister`, `Config`; `NewPlanner(inspector, lister, cfg) *Planner`; `Plan(ctx, deps map[string][]string) (PlanReport, error)` executing the full flow from design §6.1 steps 1–9 (List → filter → Fetch → RevParse → CommitsAhead/ChangedFiles/CreatedAt per branch → Order → MergeTreeConflicts per step → NewPlanReport); `Check(ctx, branch string) (Step, error)` for the single-branch slice. `Planner` MUST NOT hold or accept `port.GitIntegrator` — compile-time enforced. All 4.2a tests must pass. **REQ-PLAN-1..4, REQ-CHECK-1..4, REQ-FETCH-1..2, ADR-M1, design §6.1**

### 4.3 — Integration runner service (RED → GREEN)

- [x] **4.3a [RED]** Write `service/integrator_test.go`: mock-backed, zero git, zero disk. Test cases:
  - **Execute refused — SegmentationBad:** mock data makes ConflictRate > 0.15 → assert `RebaseOnto` is NEVER called (`GitIntegratorMock.AssertNotCalled("RebaseOnto")`); assert return is `ErrSegmentationBad`. **REQ-HEALTH-4, REQ-TEST-7**
  - **Execute clean head (1 candidate):** mock CommitsAhead > 0, MergeTreeConflicts returns clean → assert call order `List → Fetch → RevParse (plan) → CommitsAhead → ChangedFiles → MergeTreeConflicts (plan) → RevParse (re-snapshot) → CommitsAhead (re-check) → MergeTreeConflicts (re-check) → RebaseOnto`; assert HALT after first step (no second iteration). **REQ-EXECUTE-4..6, REQ-TEST-7**
  - **Execute conflict at plan time:** MergeTreeConflicts returns dirty → assert `RebaseOnto` NOT called; assert `ErrMergeConflict` returned. **REQ-EXECUTE-7**
  - **Execute conflict at re-check (advanced tip):** plan clean, re-check dirty → assert `RebaseOnto` NOT called; assert `ErrMergeConflict`. **REQ-EXECUTE-7**
  - **Execute branch-behind at re-check:** CommitsAhead returns 0 at re-check → assert `ErrBranchBehind`. **REQ-EXECUTE-4**
  - **Execute --no-fetch:** Fetch NOT called. **REQ-EXECUTE-3, REQ-FETCH-2**
  - **Execute without --yes → refused before any port call** (CLI-level guard, tested in cmd/mo; service receives a boolean `Confirmed` field on config or call param → assert returns usage error).
  - **No merge-to-develop structural test:** assert `GitIntegratorMock` exposes only `RebaseOnto` via `AssertNotCalled` for any "merge"/"push" name (none registered → the absence IS the test). **HG3, REQ-EXECUTE-6, REQ-TEST-7**
  All tests must fail. **REQ-EXECUTE-1..9, REQ-TEST-7, ADR-M1, design §6.2**

- [x] **4.3b [GREEN]** Write `service/integrator.go`: `IntegrationRunner` struct holding `port.GitInspector`, `port.GitIntegrator`, `port.WorktreeLister`, `Config`; `NewIntegrationRunner(inspector, integrator, lister, cfg) *IntegrationRunner`; `Execute(ctx context.Context, opts ExecuteOptions) error` where `ExecuteOptions` carries `Deps`, `NoFetch`, `Confirmed bool`. Implementation follows design §6.2 exactly: Plan → Gate HG6 → per step Re-Snapshot → Re-check CommitsAhead → Re-predict MergeTreeConflicts → if clean: RebaseOnto then HALT printing gh command → if dirty: HALT with §11 recipe + ErrMergeConflict. Halts after AT MOST ONE rebase; NEVER loops to step 2; NEVER merges. All 4.3a tests must pass. **REQ-EXECUTE-1..9, ADR-M1, design §6.2**

> **PR 1 ends here.** All unit tests pass with `go test -short ./...`. Zero git binary invocations. Zero disk side effects beyond the module directory itself.

---

## Phase 5: Adapters (RED → GREEN — real git/wt; `testing.Short()`-gated integration tests)

### 5.1 — `adapter/gitcli` Inspector (RED → GREEN)

- [ ] **5.1a [RED]** Write `adapter/gitcli/inspector_test.go` with `testing.Short()` guard at top of each test (`if testing.Short() { t.Skip("integration: real git") }`). Setup: `t.TempDir()` → `git init` → seed `develop` branch with an initial commit → create an agent branch `agent/test/TAL-X` with 2 commits, and a separate `agent/conflict/TAL-Y` branch that edits the same file. Cases:
  - `Fetch` invokes without error on a valid repo.
  - `RevParse("develop")` returns a non-empty SHA.
  - `MergeBase(develop, agentBranch)` returns correct ancestor.
  - `CommitsAhead(developTip, agentBranch)` returns 2 (seeded count).
  - `CommitsAhead(developTip, developTip)` returns 0.
  - `ChangedFiles(developTip, agentBranch)` returns the seeded filenames.
  - `MergeTreeConflicts(developTip, agentBranch)` clean branch → exit 0 → `(nil, true, nil)`.
  - `MergeTreeConflicts(developTip, conflictBranch)` conflicting branch → exit 1 → `(paths, false, nil)` with correct paths.
  All tests must fail. **REQ-COLLISION-1, REQ-TEST-5, ADR-M3, design §8.1**

- [ ] **5.1b [GREEN]** Write `adapter/gitcli/inspector.go`: `Inspector` struct implementing `port.GitInspector`. `NewInspector(repoRoot string) *Inspector`. Each method builds its own argv (operation-shaped, not generic `Run`), sets `cmd.Dir = repoRoot`, maps exit codes as per design §8.1. `MergeTreeConflicts` maps exit 0 → clean, exit 1 → parse stdout lines → conflicting, other → error. `CommitsAhead` runs `git rev-list --count <base>..<branch>`. `ChangedFiles` runs `git diff --name-only <base>...<branch>` (three-dot). `CreatedAt` via `git log -1 --format=%cI <branch>` returned as `time.Time` (NOTE: this method is NOT on the `GitInspector` interface — it's a package-internal helper called by the adapter during candidate building, exposed as a helper that service tests do not need to mock). Compile assertion: `var _ port.GitInspector = (*Inspector)(nil)`. All 5.1a tests must pass. **REQ-COLLISION-1, ADR-M3, design §8.1**

### 5.2 — `adapter/gitcli` Integrator (RED → GREEN)

- [ ] **5.2a [RED]** Write `adapter/gitcli/integrator_test.go` with `testing.Short()` guard. Setup: same real git repo structure as 5.1a, plus a clean rebase target and a conflicting one. Cases:
  - `RebaseOnto(agentBranch, "develop")` clean branch → `(nil, nil)`.
  - `RebaseOnto(conflictBranch, "develop")` conflicting branch → returns non-empty `conflicts` slice with the conflicting paths (unmerged files from `git diff --name-only --diff-filter=U`), and a non-nil error.
  All tests must fail. **REQ-EXECUTE-5, REQ-EXECUTE-8, design §8.1**

- [ ] **5.2b [GREEN]** Write `adapter/gitcli/integrator.go`: `Integrator` struct implementing `port.GitIntegrator`. `NewIntegrator(repoRoot string) *Integrator`. `RebaseOnto` runs `git rebase <base> <branch>` with `cmd.Dir = worktreePath` (NOTE: `worktreePath` is passed through `RebaseOnto`'s context or as a field — design §6.2 step 3d says "rebase in the worktree directory"; reconcile by adding `WorktreePath` to `ExecuteOptions` and passing it when calling `RebaseOnto`, or adjusting the `Integrator` to accept `worktreePath` alongside `branch` in the call — choose the cleanest option consistent with zero extra port methods). On conflict (non-zero exit + "CONFLICT" in stderr), runs `git diff --name-only --diff-filter=U` for unmerged paths, runs `git rebase --abort`, returns `(conflicts, ErrRebaseConflict{...})`. Compile assertion: `var _ port.GitIntegrator = (*Integrator)(nil)`. All 5.2a tests must pass. **REQ-EXECUTE-5, REQ-EXECUTE-8, ADR-M1, design §8.1**

### 5.3 — `adapter/wtcli` Lister (RED → GREEN)

- [ ] **5.3a [RED]** Write `adapter/wtcli/lister_test.go` with two test sections:
  - **Unit (no `wt`, runs under `-short`):** golden JSON fixture decode — embed the exact indented JSON array that `wt list --json` produces (verified against `cmd/wt/main.go`'s `renderListJSON`); decode into `[]port.WorktreeEntry`; assert every field maps correctly (seam-contract test). Does NOT have `t.Skip` guard — it MUST run under `-short` as a pure unit test.
  - **Integration (real `wt`):** guarded with `if testing.Short() { t.Skip("integration: real wt") }`. Invokes `wt list --json` on PATH; validates non-error decode. Skipped in `-short`.
  All tests must fail. **REQ-READINESS-1, REQ-TEST-5, ADR-M2, design §8.2**

- [ ] **5.3b [GREEN]** Write `adapter/wtcli/lister.go`: `Lister` struct implementing `port.WorktreeLister`. `NewLister(repoRoot, wtBinary string) *Lister`. `List` runs `<wtBinary> list --json` with `cmd.Dir = repoRoot`; on `exec.ErrNotFound` or path-not-found error wraps as `ErrWtBinaryNotFound{Binary: wtBinary}`; on successful run decodes stdout into `[]port.WorktreeEntry`; on JSON decode failure wraps as `ErrWtOutputMalformed{Fragment: truncated, Cause: err}`. Compile assertion: `var _ port.WorktreeLister = (*Lister)(nil)`. All 5.3a tests must pass. **REQ-READINESS-1, REQ-READINESS-5..6, ADR-M2, design §8.2**

---

## Phase 6: CLI composition root (sequential — depends on all prior phases)

- [ ] **6.1** Write `cmd/mo/main.go` and `cmd/mo/main_test.go` (composition root). Implementation:
  - `main() → run(os.Args[1:]) → os.Exit(exitCodeFor(err))` pattern (mirrors `cmd/wt/main.go`).
  - `run(args []string) error` subcommand switch: `plan`, `execute`, `check`; default → usage error.
  - `cmdPlan(args)`: flags `--json`, `--base` (default "develop"), `--wt-bin` (default "wt"), `--depends` (repeatable, `A:B` format), `--depends-file <f>`, `--no-fetch`. Builds deps map. Resolves `repoRoot` via `git rev-parse --show-toplevel` (or `os.Getwd` fallback). Wires `gitcli.NewInspector(root)`, `wtcli.NewLister(root, cfg.WtBinary)`, `NewPlanner(inspector, lister, cfg)`. Calls `Plan(ctx, deps)`. Renders tabular output (numbered steps, AHEAD, PREDICTED, CONFLICTS columns, health metric footer) or `--json` shape from design §7. Prints "nothing to merge" message on `ErrNoCandidates` + exit 0.
  - `cmdExecute(args)`: flags `--base`, `--max-conflict-rate`, `--yes`, `--no-fetch`. Requires `--yes` — without it, prints usage hint and returns error (exit 1). Wires all three adapters + `NewIntegrationRunner`. Calls `Execute(ctx, opts)`.
  - `cmdCheck(args)`: `<branch>` positional arg required. Flags `--base`, `--no-fetch`. Wires inspector + `NewPlanner`. Calls `Check(ctx, branch)`. Prints clean or conflict + paths.
  - `exitCodeFor(err)`: success → 0; `ErrNoCandidates` → 0; all other typed errors and unexpected errors → 1.
  - `main_test.go`: drives `run(args)` — tests subcommand dispatch, `--yes` gate for execute, unknown subcommand error, `ErrNoCandidates` exit-0 path (mocked via injectable adapters or flag parsing alone).
  **REQ-PLAN-2..3, REQ-EXECUTE-1..9, REQ-CHECK-1..4, REQ-ERR-3..4, REQ-READINESS-4, REQ-FETCH-1..2, design §7**

> **PR 2 ends here.** Branch: `agent/hermes/TAL-3` (or stacked branch base PR 1). Full suite passes with `go test ./...`; short suite passes with `go test -short ./...`.

---

## Execution order and parallelism summary

```
0.1, 0.2, 0.3   — parallel (independent repo-level files; no Go deps)
1.1             — sequential (go.mod must exist first)
1.2             — sequential (errors.go is a prereq for all domain tests)
1.3a → 1.3b     — sequential RED→GREEN pair
1.4a → 1.4b     — sequential RED→GREEN pair
1.5a → 1.5b     — sequential RED→GREEN pair
   1.3, 1.4, 1.5 pairs are PARALLEL with each other after 1.1+1.2
2.1, 2.2, 2.3   — parallel after all domain is green (ports import domain; each port file is independent)
3.1, 3.2, 3.3   — parallel after 2.1–2.3 (each mock implements one port)
4.1             — sequential after 3.1–3.3 (config, no test dep)
4.2a → 4.2b     — sequential RED→GREEN pair (planner; depends on 3.1, 3.3, 4.1)
4.3a → 4.3b     — sequential RED→GREEN pair (integrator; depends on 3.1, 3.2, 3.3, 4.1; can start in parallel with 4.2b)
5.1a → 5.1b     — sequential RED→GREEN pair (gitcli inspector; no service dep)
5.2a → 5.2b     — sequential RED→GREEN pair (gitcli integrator; no service dep; can start in parallel with 5.1)
5.3a → 5.3b     — sequential RED→GREEN pair (wtcli lister; independent)
   5.1, 5.2, 5.3 pairs are PARALLEL with each other after ports are defined
6.1             — sequential last (composition root needs everything wired)
```

Phase 0 tasks are independent of all Go tasks — they can run concurrently with Phase 1 scaffolding.

---

## Constraint reminders (DO NOT violate)

1. `service` NEVER imports `adapter/gitcli` or `adapter/wtcli`. `cmd/mo` is the sole composition root.
2. `adapter/gitcli` and `adapter/wtcli` import `port` + `domain/mergeorder`; they NEVER import `service`.
3. `Planner` is constructed with ONLY `GitInspector` + `WorktreeLister` + `Config`. Do NOT pass `GitIntegrator` to `Planner`. This is the ADR-M1 structural enforcement.
4. There is NO `Merge`, `Push`, `OpenPR`, or any develop-mutating method anywhere in the port surface. Do NOT add one under any flag or configuration.
5. Zero transitive deps in `go.mod`. No cobra, no testify, no viper. stdlib only (`os/exec`, `flag`, `encoding/json`, `time`, `testing`). `go.sum` stays empty.
6. `MergeTreeConflicts` maps git exit code 0 → clean, 1 → conflicting paths, other → error. NEVER use `git merge --no-commit` (ADR-M3).
7. `CommitsAhead`, `ChangedFiles`, `MergeTreeConflicts` take explicit `base` param (not hardcoded "develop") so execute can re-check against the advanced tip.
8. `execute` HALTS after at most ONE rebase at the PR boundary, printing the `gh` command. It NEVER loops to a second branch and NEVER merges. This is non-negotiable.
9. `--depends-file` format: one edge per line `branch:needs-branch`, `#` comment lines, blank lines ignored. No YAML/JSON parser.
10. Integration tests in `adapter/gitcli/*_test.go` and the integration section of `adapter/wtcli/lister_test.go` MUST be gated with `if testing.Short() { t.Skip(...) }` per test (not a build tag). The unit golden-decode test in `lister_test.go` MUST run under `-short`.
11. Do NOT touch `ci/pr-checks.yml` (LOCKED).
