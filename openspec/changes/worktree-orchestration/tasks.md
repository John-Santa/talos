# Tasks: worktree-orchestration

**Change:** worktree-orchestration · **Jira:** TAL-2 · **Module:** module:devops · **Owner:** HERMES
**Branch:** `agent/hermes/TAL-2` · **Store:** hybrid (this file + engram `sdd/worktree-orchestration/tasks`)
**TDD mode:** Strict — RED before GREEN for every domain/service/adapter file.
**Boundary:** DO NOT touch `ci/pr-checks.yml` (change #4, LOCKED).

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 480–620 LOC |
| 400-line budget risk | Medium-High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → go.mod + domain + port + service + mock + all unit tests; PR 2 → adapter/gitcli + cmd/wt + .env.example + ownership + .gitignore entry |
| Delivery strategy | ask-on-risk |
| Decision needed before apply | **Yes** — confirm single PR (size:exception) vs two chained PRs before sdd-apply starts |

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | go.mod + domain/worktree (naming, env, porcelain, errors) + port + service + mock + all unit tests | PR 1 | Zero git, zero disk, zero live calls; self-contained |
| 2 | adapter/gitcli + cmd/wt + .env.example + .gitignore + ownership.md | PR 2 | Base = PR 1 branch; real git needed for integration tests only |

---

## Phase 0: Repo-level scaffolding (sequential, no RED/GREEN — no logic)

- [x] **0.1** Add `/platform/worktree-orchestrator/wt` to `.gitignore` (mirrors the `evidence` binary entry). No test needed — verified by `git status`. **REQ-ENV-5 (binary hygiene), design §13**
- [ ] **0.2** Create `.env.example` at repo root with the exact content specified in REQ-ENV-5. File must include `PORT=8100`, `DB_SCHEMA=wt_<figura>`, `JIRA_EMAIL=`, `JIRA_API_TOKEN=`, `JIRA_SITE_URL=https://tablex.atlassian.net`, and the comment header. **REQ-ENV-5** — BLOCKED: sandbox write restriction on repo root (`.env` present blocks tool writes; ZEUS to create manually).
- [x] **0.3** Update `team-context/ownership.md`: (a) flip `module:devops` row status from `slot` → `active`, (b) add `platform/worktree-orchestrator/**` and `.env.example` entries to the shared-file map with owner `HERMES`. **REQ-OWNER-1**

---

## Phase 1: Module init + domain (PR 1)

### 1.1 — Module scaffold (sequential, no RED/GREEN)

- [x] **1.1** Create `platform/worktree-orchestrator/go.mod` with module path `github.com/John-Santa/talos/platform/worktree-orchestrator`, go 1.26, zero transitive dependencies. Mirrors `jira-evidence-loop` Kit goal. **design §2**

### 1.2 — Typed errors (prerequisite; no RED/GREEN, pure declarations)

- [x] **1.2** Write `domain/worktree/errors.go`: six typed error structs — `ErrInvalidFigure{Figura string}`, `ErrInvalidKey{Key string}`, `ErrWorktreeExists{Figura, Path string}`, `ErrBranchExists{Branch string}`, `ErrWorktreeNotFound{Figura, Path string}`, `ErrDirtyWorktree{Figura, Path string}`. Each implements `error`. Zero I/O, zero imports beyond `fmt`. **REQ-ERR-1, design §9**

### 1.3 — Domain: naming (RED → GREEN)

- [x] **1.3a [RED]** Write `domain/worktree/naming_test.go`: table-driven tests for `ParseFigura` (all 8 valid values + several invalid → `ErrInvalidFigure`), `ValidateJiraKey` (`TAL-1`, `TAL-42` pass; `TAL-0`, `tal-5`, `FOO-1`, `TAL-`, empty reject → `ErrInvalidKey`), `BranchName`/`WorktreePath` exact-string assertions, `NewWorktreeSpec` happy path + each typed error. All tests must fail (symbols not yet defined). **REQ-NAMING-1..4, REQ-ERR-1, REQ-TEST-1**
- [x] **1.3b [GREEN]** Write `domain/worktree/naming.go`: `Figura` type, `roster` var, `jiraKeyRe` regex, `ParseFigura`, `ValidateJiraKey`, `BranchName`, `WorktreePath`, `WorktreeSpec` struct, `NewWorktreeSpec`. Pure, zero I/O, zero third-party imports. All 1.3a tests must pass. **REQ-NAMING-1..4, design §5**

### 1.4 — Domain: env + assign-map (RED → GREEN)

- [x] **1.4a [RED]** Write `domain/worktree/env_test.go`:
  - Exhaustive `AgentResources` table: assert exact `Port` and `DBSchema` for all 8 figuras per REQ-ASSIGN-1 table; assert `ErrInvalidFigure` for unknown figura.
  - Disjoint regression test: iterate all 8 figuras, collect ports and schemas, assert no duplicates (REQ-ASSIGN-3, REQ-TEST-5).
  - `RenderEnv` golden-string assertion: assert exact byte content (PORT + DB_SCHEMA lines only); assert NO `JIRA_` substring (Decision 5 guard). Content must NOT include a timestamp inside the variable block — timestamp is informational only and excluded from the bytes-identical comparison (ADR-D6 + REQ-ENV-3 reconciliation).
  - All tests must fail. **REQ-ASSIGN-1..4, REQ-ENV-1..3, REQ-TEST-1,5**
- [x] **1.4b [GREEN]** Write `domain/worktree/env.go`: `Resources` struct, `portBase` const (8100, `// DEBT(kit-extraction)`), `assignMap` var (figura→offset, `// DEBT(kit-extraction)`), `AgentResources(f Figura) (Resources, error)`, `RenderEnv(spec WorktreeSpec, res Resources) string`. `RenderEnv` emits ONLY `PORT=<n>` and `DB_SCHEMA=<schema>` in the variable block; any header comment MUST NOT contain a runtime timestamp (header is static/deterministic so the output is byte-identical across calls — this closes ADR-D6 + REQ-ENV-3). All 1.4a tests must pass. **REQ-ASSIGN-1..4, REQ-ENV-1..3, design §6, ADR-D5, ADR-D6**

  > **ADR-D7 reconciliation note (mandatory):** `RenderEnv` is pure. The `.env` write is NOT done here — it is the service's responsibility via the injected `WriteFile` field wired in task 2.3b (`NewOrchestrator`). A worktree without a `.env` is half-created; task 2.3b MUST wire `WriteFile` default to `os.WriteFile` so there is no runtime path where `Create` skips the write.

### 1.5 — Domain: porcelain parser (RED → GREEN)

- [x] **1.5a [RED]** Write `domain/worktree/porcelain_test.go`: fixture-based table-driven tests for `ParseWorktreeList` covering: empty string (valid → empty slice), single main/bare worktree only (no agent entries → empty after filter), one agent worktree, multiple agent worktrees, detached HEAD entry, leading/trailing blank lines, malformed block (→ error). `refs/heads/` prefix stripped in output. All tests must fail. **REQ-LIST-1, REQ-TEST-6**
- [x] **1.5b [GREEN]** Write `domain/worktree/porcelain.go`: `WorktreeInfo` struct (Path, Head, Branch, Detached, Bare), `ParseWorktreeList(stdout string) ([]WorktreeInfo, error)`. Pure, no git. `refs/heads/` stripped on output. All 1.5a tests must pass. **REQ-LIST-1, design §7**

---

## Phase 2: Port (sequential, no RED/GREEN — pure interface declaration)

- [x] **2.1** Write `port/git.go`: `GitRunner` interface with 7 methods: `Fetch`, `BranchExists`, `WorktreeAdd`, `WorktreeList`, `WorktreeRemove`, `Prune`, `BranchDelete` (opt-in; mock implements it; orchestrator calls only when `--delete-branch`). Signatures exactly as specified in design §4. Zero deps beyond `context`. **REQ-CREATE-1..3, REQ-LIST-1, REQ-TEARDOWN-1..5, design §4, ADR-D1, ADR-D4**

---

## Phase 3: Mock (sequential — depends on port 2.1)

- [x] **3.1** Write `mock/git_runner_mock.go`: hand-written test double implementing all 7 `GitRunner` methods. Supports per-method configurable stub returns (`*Result`/`*Err` fields), records ordered call log (`Calls []Call`), and exposes `AssertMethodOrder(t, methods...)`. Pattern mirrors `mock/jira_client_mock.go`. No third-party test libs. **REQ-TEST-2, design §12**

---

## Phase 4: Service (RED → GREEN — depends on domain + port + mock)

- [x] **4.1a [RED]** Write `service/orchestrator_test.go` — mock-backed, zero git, zero disk. Test cases:
  - **Create happy path:** assert call order `BranchExists → WorktreeList → Fetch → WorktreeAdd`; assert `WriteFile` callback received `RenderEnv` output at `spec.Path+"/.env"`.
  - **Create --no-fetch:** assert `Fetch` is NOT called; all other steps proceed.
  - **Create validation failures (each stops before any runner call):** invalid figura → `ErrInvalidFigure`; invalid jira key → `ErrInvalidKey`.
  - **Create pre-check failures:** `BranchExists` returns true → `ErrBranchExists` (WorktreeAdd not called); list shows path occupied → `ErrWorktreeExists` (WorktreeAdd not called).
  - **Create mid-sequence failure:** `WorktreeAdd` returns error → propagated; `WriteFile` not called.
  - **Create WriteFile failure:** `WorktreeAdd` succeeds, `WriteFile` errors → partial state reported (not silent). **REQ-CREATE-8**
  - **List:** `WorktreeList` raw stdout passed through `ParseWorktreeList`; result returned as `[]WorktreeInfo`.
  - **Teardown happy (default):** order `WorktreeList → WorktreeRemove(force=false) → Prune`; `BranchDelete` NOT called.
  - **Teardown --delete-branch:** `BranchDelete` IS called after Prune.
  - **Teardown --force:** `WorktreeRemove` receives `force=true`.
  - **Teardown ErrWorktreeNotFound:** list empty / path absent → return `ErrWorktreeNotFound`, no remove called.
  - **Env ErrWorktreeNotFound:** list shows no matching path → `ErrWorktreeNotFound`, no WriteFile called.
  - **Env happy:** WriteFile receives bytes byte-identical to `RenderEnv(spec, res)` (the idempotency proof; no timestamp in content).
  - All tests must fail. **REQ-CREATE-1..8, REQ-LIST-1, REQ-TEARDOWN-1..5, REQ-ENV-3..4, REQ-ERR-1..4, REQ-TEST-2**

- [x] **4.1b [GREEN]** Write `service/orchestrator.go`: `Config` struct with `RepoRoot`, `WorktreeBase`, `BaseBranch`; `DefaultTALConfig()` returning `{WorktreeBase:"talos.wt", BaseBranch:"develop"}`; `Orchestrator` struct with `runner port.GitRunner`, `cfg Config`, `WriteFile func(path string, data []byte, perm fs.FileMode) error`; `NewOrchestrator(runner port.GitRunner, cfg Config) *Orchestrator` — **MUST default `WriteFile` to `os.WriteFile` here (ADR-D7 wiring)**; methods `Create`, `List`, `Teardown`, `Env` with exact sequences from design §8. All 4.1a tests must pass. **REQ-CREATE-1..8, REQ-LIST-1, REQ-TEARDOWN-1..5, REQ-ENV-3..4, REQ-ERR-1..4, design §3, §8, ADR-D7**

  > **ADR-D7 MUST-DO:** `NewOrchestrator` sets `o.WriteFile = os.WriteFile` as the default. Failure to do this leaves every `Create` and `Env` call silently skipping `.env` write in production (the worktree is left half-created). Tests use the injected stub; production uses the default. This is the reconciliation item from design ADR-D7.

> **PR 1 ends here.** All unit tests pass with `go test -short ./...`. Zero git binary invocations. Zero disk side effects.

---

## Phase 5: Adapter (RED → GREEN — real git, testing.Short()-gated)

- [x] **5.1a [RED]** Write `adapter/gitcli/runner_test.go` with `testing.Short()` guard at top of each test (`if testing.Short() { t.Skip("integration: real git") }`). Test cases using `t.TempDir()` + `git init` + initial commit + local `develop` branch:
  - `WorktreeAdd` creates dir + branch.
  - `WorktreeList` stdout round-trips through `domain.ParseWorktreeList` (adapter returns raw; domain parses).
  - `WorktreeRemove` on dirty worktree fails; adapter maps stderr → `ErrDirtyWorktree` (string-match strategy from design §8).
  - `WorktreeRemove(force=true)` on dirty worktree succeeds.
  - `Prune` clears a stale entry (manually remove dir after add, then prune, verify no admin state).
  - `BranchExists` returns true for existing branch, false for absent.
  - `BranchDelete` removes a merged branch (safe `git branch -d`).
  All tests must fail. **REQ-TEST-3, design §12 integration section, ADR-D1**
- [x] **5.1b [GREEN]** Write `adapter/gitcli/runner.go`: `Runner` struct implementing `GitRunner`. Uses `os/exec`, no third-party deps. Builds argv per-method (operation-shaped, not generic `Run(args...)`). `WorktreeList` returns raw stdout — does NOT parse (parser lives in domain, adapter only feeds bytes). `WorktreeRemove` maps dirty-refusal stderr to `ErrDirtyWorktree` via string match. All 5.1a tests must pass. **REQ-CREATE-1..4, REQ-LIST-1, REQ-TEARDOWN-1..5, design §4, §12, ADR-D4**

---

## Phase 6: CLI (sequential — composition root, depends on all prior phases)

- [x] **6.1** Write `cmd/wt/main.go`: composition root with `main()` → `run(os.Args[1:])` → `os.Exit(exitCodeFor(err))` pattern (mirrors `cmd/evidence/main.go`). Subcommand switch: `create`, `list`, `teardown`, `env`; default → unknown subcommand error. Each `cmd*` function uses its own `flag.NewFlagSet`. CLI flags:
  - `create <figura> <TAL-N> [--no-fetch]`
  - `list [--json]` (tabular default: FIGURA/BRANCH/PATH/HEAD/STATUS columns with header; `--json` emits JSON array per REQ-LIST-3)
  - `teardown <figura> <TAL-N> [--force] [--delete-branch]`
  - `env <figura> <TAL-N>`
  Composition root resolves `RepoRoot` via `git rev-parse --show-toplevel` (or `os.Getwd` fallback), builds `DefaultTALConfig()`, constructs `gitcli.NewRunner()`, wires `service.NewOrchestrator(runner, cfg)` (WriteFile defaults to `os.WriteFile` — already wired in 4.1b). `exitCodeFor` maps typed errors to exit code 1, success to 0. **REQ-NAMING-1..4, REQ-CREATE-1..8, REQ-LIST-1..5, REQ-TEARDOWN-1..6, REQ-ENV-3..4, REQ-ERR-1..4, design §10**

  > **LIST rendering note:** tabular output for `wt list` must print the header row and "no active agent worktrees" when the result is empty (REQ-LIST-5). `--json` emits `[]` for empty. Status derivation (`active`/`orphan`/`stale`) lives in `cmd/wt` (it is a presentation concern, not service logic) using `WorktreeInfo.Detached` + filesystem existence check. **REQ-LIST-4**

> **PR 2 ends here.** Branch: `agent/hermes/TAL-2` (or a stacked branch if chained). Full suite passes with `go test ./...`; short suite passes with `go test -short ./...`.

---

## Execution order and parallelism summary

```
0.1, 0.2, 0.3   — parallel (independent repo-level files)
1.1             — sequential (go.mod must exist first)
1.2             — sequential (errors.go is a prereq for all domain tests)
1.3a → 1.3b     — sequential RED→GREEN pair
1.4a → 1.4b     — sequential RED→GREEN pair
1.5a → 1.5b     — sequential RED→GREEN pair
   1.3, 1.4, 1.5 pairs are PARALLEL with each other after 1.1+1.2
2.1             — sequential after all domain (port imports domain)
3.1             — sequential after 2.1 (mock implements port)
4.1a → 4.1b     — sequential RED→GREEN pair (service depends on domain + port + mock)
5.1a → 5.1b     — sequential RED→GREEN pair (adapter; can start in parallel with 4.1b if needed)
6.1             — sequential last (composition root needs everything)
```

Phases 0 tasks are independent of all Go tasks — they can run first or in parallel with Phase 1.

---

## Constraint reminders (DO NOT violate)

1. `service` NEVER imports `adapter/gitcli`. `cmd/wt` is the sole composition root.
2. `adapter/gitcli` imports `domain/worktree` (to call `ParseWorktreeList`) and `port`; it never imports `service`.
3. Zero transitive deps in `go.mod`. No cobra, no testify, no viper. stdlib only.
4. No `--port-base` flag. No `--lock` flag. No `Status` port method. No `JIRA_*` in generated `.env`.
5. `RenderEnv` output has NO runtime timestamp in the variable block — timestamp is static or omitted. This is non-negotiable for REQ-ENV-3 byte-identical idempotency.
6. `NewOrchestrator` MUST wire `WriteFile = os.WriteFile` as default (ADR-D7). Missing this wire = runtime half-creates.
7. No touch to `ci/pr-checks.yml` (LOCKED boundary — change #4).
8. Integration tests in `adapter/gitcli/runner_test.go` MUST be gated with `testing.Short()` on each test (not a build tag, per design §12).
