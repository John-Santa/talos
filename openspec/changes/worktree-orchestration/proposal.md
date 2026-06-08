# Proposal — worktree-orchestration

> **Change:** `worktree-orchestration` · **Module:** `module:devops` · **Owner:** HERMES
> **Branch:** `agent/hermes/TAL-2` · **Jira:** TAL-2 · **Store:** hybrid (openspec + engram)
> **Phase:** propose · **Status:** decided (3 architectural decisions LOCKED by ZEUS + 6 resolved here)

---

## Why / Intent

**Problem.** Fase 2 promises physical isolation per agent — one worktree, one branch, one disjoint
`.env` (CONSTITUTION §11) — so ATHENA can dispatch work units in parallel without writers colliding on
disk. Today none of that machinery exists. The agent frontmatter carries `isolation: worktree`, but
nothing *materializes* it: in Fase 1 ATHENA never spawned real worktrees. The platform claims parallel
isolation as a capability while having zero tooling to create, list, or tear down the isolated
environments it depends on. Parallelism is blocked at the foundation.

**Why now.** This is the **first change of Fase 2** and the literal cimiento for everything that follows
(merge-order automation, overlap detection, parallel dispatch). Until `wt` exists, every later Fase 2
change has no isolated ground to stand on. It is also the natural second Go artifact in the monorepo —
it lets us prove the hexagonal pattern from `jira-evidence-loop` generalizes to a *git-orchestration*
domain, not just an HTTP one.

**Success looks like.** A `go test ./...`-green Go CLI `wt` at `platform/worktree-orchestrator/`
exposing four subcommands — `create`, `list`, `teardown`, `env` — that:

1. Branch deterministically from `develop` as `agent/<figura>/<TAL-N>` into `talos.wt/agent-<figura>/`.
2. Generate a disjoint, collision-free `.env` per agent (deterministic port + DB schema, NO Jira
   credentials).
3. Report worktree state in human-readable tables **and** `--json` for ATHENA to consume programmatically.
4. Fail loud on every ambiguous edge (branch already checked out, worktree exists, dirty teardown) with
   typed errors — never a silent reuse, never a silent `--force`.

All pure logic (assign-map, `.env` generation, porcelain parsing) is unit-tested with zero git; the git
adapter is exercised by an integration-tagged test against a real temp repo. The ownership blackboard
flips `module:devops` from `slot` to `active` under HERMES, and the shared-file map records HERMES as
owner of `.env.example` and `talos.wt/`.

---

## What changes

### In scope

- **New Go module** at `platform/worktree-orchestrator/` — hexagonal layout mirroring
  `jira-evidence-loop` (cero deps, `flag`-package CLI, typed errors in the `ErrOwnershipViolation`
  style). Module path `github.com/John-Santa/talos/platform/worktree-orchestrator`, `go 1.26`.
- **Pure domain** (`domain/worktree/`): the figura→(port, schema) assign-map, `WorktreeSpec` value
  object, `.env` generation, and `git worktree list --porcelain` parsing — all I/O-free and unit-tested.
- **The port** (`port/git.go`): a single `GitRunner` interface (`Run(ctx, args...) (stdout, err)`) — the
  mockable boundary that keeps git out of the use-case layer.
- **The git adapter** (`adapter/gitcli/`): a thin `os/exec` implementation of `GitRunner` plus an
  integration-tagged test running real git in a temp repo.
- **The service** (`service/`): the orchestrator wiring config + port to implement the four use cases
  (create / list / teardown / env), enforcing fail-loud semantics. Primary unit under test (mocked port).
- **Mock** (`mock/`): hand-written `GitRunner` double, same pattern as `JiraClientMock`.
- **Thin CLI** (`cmd/wt/main.go`): testable `run(args)` entry point + subcommand switch
  (`create | list | teardown | env`), each with its own `flag.FlagSet`.
- **Root `.env.example`** (new file — does not exist yet; `.gitignore` already whitelists it): the
  template the generated per-worktree `.env` mirrors. Owned by HERMES.
- **Ownership registry update** (`team-context/ownership.md`): flip `module:devops` row `slot → active`,
  and populate the shared-file map with `.env.example → HERMES` and `talos.wt/ → HERMES`.
- **`.gitignore`**: ignore the compiled CLI binary `/platform/worktree-orchestrator/wt`.

### Out of scope (explicit boundary)

- **All CI work.** Running the module's `go test ./...` in CI and adding any DoD/label invariant to
  `ci/pr-checks.yml` is **change #4 (ci-hardening)** — a ZEUS-locked boundary. **This change does NOT
  touch `ci/pr-checks.yml`.** The explore's "Riesgo 7" (advance the pr-checks TODO) is deliberately
  deferred there.
- **Merge-order automation** (which branch merges first) → change #2.
- **Overlap / file-collision detection across issues** → change #3.
- **`develop` drift handling during a long task** — rebase-before-PR is merge-order's job
  (`merge-order.md`); `wt` does not auto-sync an open worktree. Documented as a risk, not solved.
- **Process / server orchestration** (actually binding the disjoint ports, booting servers) → Fase 3+.
- **DB schema provisioning** (creating `wt_<figura>` schemas) → GAIA, Fase 3+. `wt` only *names* the
  schema in `.env`; it never touches a database.
- **Kit extraction** (Fase 5). This change *designs toward* it (own `go.mod`, zero deps) but the
  hardcoded assign-map is an accepted, documented debt until then (Decision 6).
- **`go.work` workspace** — each module stays an independent binary; no workspace until Fase 5+.

---

## Locked decisions (ZEUS) — encoded, not reopened

### Decision 1 — Tooling is a Go CLI, hexagonal, mirroring `jira-evidence-loop` (LOCKED)

`wt` is a standalone Go module under hexagonal architecture: pure `domain/worktree/`, a `GitRunner`
port, an `os/exec` `adapter/gitcli/`, a `service/` orchestrator, a hand-written `mock/`, and a thin
`cmd/wt/main.go`. Zero transitive dependencies, `flag`-package CLI, typed errors in the
`ErrOwnershipViolation` style. Shell scripts and Makefiles are rejected: neither offers a test framework
coherent with the Go stack nor typed errors.

**Honest note on where the value lives.** Unlike `jira-evidence-loop`, the port here does **not** buy
swapability (there is no second git backend). Its genuine value is making the *pure* logic — assign-map,
`.env` generation, porcelain parsing — unit-testable with zero git, and keeping `os/exec` out of the use
case. The git adapter is intentionally thin; its real test is the integration-tagged one against real
git. The architecture is justified by testability and stack homogeneity, not by a fictional second adapter.

### Decision 2 — Do NOT use the harness `EnterWorktree` / `ExitWorktree` primitive (LOCKED)

The Claude Code `isolation: worktree` harness has four confirmed, non-configurable gaps that each violate
a Talos invariant:

| Harness behavior | Talos requires | Constitution |
|---|---|---|
| Branches from `origin/main` (hardcoded) | Branch from `develop` | §11 (PRs → `develop`) |
| Random / generic branch name | `agent/<figura>/<TAL-N>` | §3 (branch naming) |
| Places in `.claude/worktrees/<random>` | `talos.wt/agent-<figura>/` | §11 (worktree layout) |
| No per-worktree `.env` | `PORT` + `DB_SCHEMA` disjoint per figura | §11 (disjoint env) |

None of the four is configurable. **This is coexistence, not coupling:** agents keep the
`isolation: worktree` frontmatter tag (it *declares intent*), but the `wt` CLI is what actually
materializes a correct worktree before ATHENA dispatches the agent. We do not call, wrap, or depend on
the harness primitive — we replace its mechanism for Talos worktrees.

### Decision 3 — Strict boundary: this change is ONLY the `wt` CLI; all CI is change #4 (LOCKED)

Everything related to CI — running the module's tests in CI, the labels/DoD invariant in
`ci/pr-checks.yml` — is out of scope and belongs to **change #4 (ci-hardening)**. This change writes
**no** changes to `ci/pr-checks.yml`. The boundary is enforced explicitly so the two changes can land as
independent, reviewable PRs.

---

## Resolved decisions (decided here in propose)

### Decision 4 — `create` idempotency: fail-loud by default, no silent reuse

`wt create <figura> <TAL-N>` is **fail-loud**. If the target worktree path exists, or the branch already
exists (locally or checked out in another worktree), the CLI returns a typed error
(`ErrWorktreeExists` / `ErrBranchExists`) — it never silently reuses. Rationale: in Talos, an existing
`agent/<figura>/<TAL-N>` branch *means* an active issue already owns that worktree; silently reusing it
would mask a double-dispatch bug from ATHENA. There is **no `--reuse` flag** in this change — reuse, if
ever needed, is a deliberate future decision, not a quiet default. (Supersedes the explore's tentative
`--reuse` suggestion.)

### Decision 5 — Generated `.env`: disjoint port + schema, NO Jira credentials

The per-worktree `.env` contains **only** the deterministic, collision-free resources `wt` actually owns:
`PORT` and `DB_SCHEMA`. It does **not** contain Jira credentials. The CLI has no access to secrets, and
ATHENA propagates `JIRA_*` from the parent environment when dispatching an agent. Keeping credentials out
of a generated file also avoids ever writing an empty-placeholder secret that could be mistaken for real
config. (Supersedes the explore's open question #2 — credentials are excluded, not placeholdered.)

Generated `.env` (illustrative):

```env
# Generated by `wt create` — DO NOT EDIT MANUALLY
# Agent: atlas | Issue: TAL-2 | Created: 2026-06-07
PORT=8100
DB_SCHEMA=wt_atlas
```

This change also **creates the root `.env.example`** (it does not exist yet; `.gitignore` already
whitelists it via `!.env.example`). It documents the full shape an agent's `.env` may take — including
the `JIRA_*` keys ATHENA injects — so the template is the human-facing contract, while the *generated*
file stays narrow:

```env
# Talos worktree .env — copy to .env and fill values
# PORT and DB_SCHEMA are generated per-worktree by `wt create`.
PORT=8100
DB_SCHEMA=wt_<figura>
# Injected by ATHENA from the parent environment — never generated by `wt`:
JIRA_EMAIL=
JIRA_API_TOKEN=
JIRA_SITE_URL=https://tablex.atlassian.net
```

### Decision 6 — assign-map figura→(port, schema): hardcoded with documented debt

The assign-map is a **hardcoded pure function** for the eight Constitution §1 figuras, with the port base
(`8100`) exposed in `Config` for test/override but the figura→offset mapping fixed in code:

| Figura | Port | Schema |
|---|---|---|
| atlas | 8100 | wt_atlas |
| hephaestus | 8101 | wt_hephaestus |
| cronos | 8102 | wt_cronos |
| iris | 8103 | wt_iris |
| gaia | 8104 | wt_gaia |
| themis | 8105 | wt_themis |
| hermes | 8106 | wt_hermes |
| argos | 8107 | wt_argos |

An unknown figura returns `ErrUnknownFigura` (fail-loud — no silent default port). `8108+` is reserved.

**Why hardcoded, not config-injectable now.** The eight figuras are fixed in CONSTITUTION §1 for the
entire life of *this instance*; there is no Fase 2 scenario where they change. Making the map injectable
today would add a config surface with zero current consumer — speculative generality. The map is written
as a **single pure function with an explicit `// DEBT(kit-extraction):` comment**: when Talos becomes a
per-instance Kit in Fase 5, this map must move to instance config. Isolating it in one pure function
makes that future extraction a one-file change. This is an accepted, *documented* debt, not an oversight.

### Decision 7 — `wt list`: tabular by default, `--json` for ATHENA

`wt list` prints a human-readable table by default and a machine-parseable array under `--json`. ATHENA
consumes `--json` to reason about live worktrees (which figura, which branch, dirty state) without
scraping formatted text. The JSON shape is derived from the pure `ParseWorktreeList` porcelain parser, so
both renderings share one source of truth.

### Decision 8 — `wt create` fetches `develop` by default, `--no-fetch` to opt out

`create` runs `git fetch origin develop` before `git worktree add ... origin/develop`, so a stale or
missing local `develop` never silently branches from old code. A `--no-fetch` flag lets the caller skip
the network call when `develop` is known-fresh (e.g. ATHENA just fetched). Default-fetch is the safe
choice; the flag is the explicit escape hatch. (Resolves explore open question #1 — network is on by
default but opt-out, never implicit-and-unstoppable.)

### Decision 9 — Subcommand surface: `create`, `list`, `teardown`, `env`

Four subcommands:

- **`create <figura> <TAL-N>`** — fetch (Decision 8) → `git worktree add -b agent/<figura>/<TAL-N>
  talos.wt/agent-<figura> origin/develop` → write `.env`. Fail-loud on existing worktree/branch.
- **`list [--json]`** — parse `git worktree list --porcelain`; tabular or JSON (Decision 7).
- **`teardown <figura> [--force]`** — verify exists → check dirty (uncommitted/untracked →
  `ErrDirtyWorktree` unless `--force`) → `git worktree remove` → `git worktree prune`. Never
  auto-`--force`; the caller (ATHENA) must opt in explicitly.
- **`env <figura> <TAL-N>`** — idempotently (re-)derive and write the `.env` for an existing worktree
  from the assign-map. Safe to re-run; used to repair or refresh a `.env` without recreating the worktree.

---

## Edge cases promoted to requirements (for spec)

These explore-identified edges are **requirements**, not afterthoughts — spec must cover each:

1. **Branch already checked out** in another worktree → pre-check via `git worktree list --porcelain` +
   `git branch --list`; return `ErrBranchExists`, do not let raw git error leak.
2. **`develop` not local** → `git fetch origin develop` and branch from `origin/develop` (Decision 8);
   never fail with a raw "invalid reference".
3. **Dirty teardown** → any uncommitted or untracked file yields `ErrDirtyWorktree{Figura,
   UncommittedFiles}`; `--force` is the only way to override, and only the caller may set it.
4. **`git worktree prune` after teardown** → always run prune after `remove` to clear stale registry
   entries (including those left by an external `rm -rf`).
5. **Worktree path already exists** → pre-check the path; return `ErrWorktreeExists` before attempting
   `git worktree add`.

---

## Affected areas

| Path | Change | Owner |
|---|---|---|
| `platform/worktree-orchestrator/**` | **New** Go module (domain, port, adapter, service, mock, cmd, tests) | HERMES (`module:devops`) |
| `.env.example` (repo root) | **New** — worktree `.env` template (whitelisted by `.gitignore`) | HERMES |
| `team-context/ownership.md` | Flip `module:devops` `slot → active`; add `.env.example → HERMES` and `talos.wt/ → HERMES` to the shared-file map | ATHENA-maintained; declared by this change (ownership rule 3) |
| `.gitignore` | Ignore compiled binary `/platform/worktree-orchestrator/wt` | HERMES |

**Out-of-module files declared per ownership.md rule 3:** `team-context/ownership.md` and `.gitignore`
are touched but are shared/config files — declared here explicitly. **`ci/pr-checks.yml` is NOT touched**
(Decision 3).

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| **Thin-port honesty** — the `GitRunner` port buys no swapability; risk of architecture-for-its-own-sake | Justify the port by *testability* (pure logic unit-tested with zero git) and keep the adapter deliberately thin; real adapter coverage is the integration-tagged test (Decision 1) |
| **`develop` drift on long tasks** — an open worktree desyncs as `develop` advances | Explicitly **out of scope**; rebase-before-PR is merge-order's job. `wt list --json` surfaces state so ATHENA can decide; `wt` never auto-syncs |
| **Orphaned worktrees** — ATHENA dies mid-task without `teardown` | Out of scope to auto-reap, but `wt list --json` returns full state (figura, branch, dirty) so ATHENA can reconcile; `git worktree prune` in teardown clears stale entries |
| **Silent destructive teardown** — `--force` could nuke useful uncommitted work | `wt` is "dumb": always `ErrDirtyWorktree` on dirty; only the explicit caller may pass `--force`. The CLI never decides "failure vs pause" (CONSTITUTION §11 leaves that to ATHENA) |
| **Hardcoded assign-map breaks at Kit extraction** | Accepted, *documented* debt: single pure function + `// DEBT(kit-extraction):` marker, so Fase 5 extraction is a one-file change (Decision 6) |
| **Cross-change boundary leak** — temptation to "just add the CI test while here" | Decision 3 is a hard wall: `ci/pr-checks.yml` is untouched; CI is change #4. Verify must flag any pr-checks diff as a boundary violation |
| **Empty-placeholder secret confusion** — a generated `.env` with blank `JIRA_*` could read as real config | Generated `.env` excludes `JIRA_*` entirely (Decision 5); only `.env.example` documents them, clearly marked as ATHENA-injected |

---

## Open questions for spec / design

1. **Branch cleanup on teardown.** Should `teardown` also `git branch -d agent/<figura>/<TAL-N>`
   (safe-delete, only if merged) for failed/discarded agents, or leave the branch for merge-order to
   reap? The explore flags both paths; pin the exact teardown contract in design. **Recommendation:**
   `teardown` removes only the worktree by default; branch deletion is a separate explicit concern.
2. **`--lock` on `git worktree add`.** Should created worktrees be `--lock`ed to prevent any external
   automatic pruning while an agent is active? Explore in design — tradeoff is manual-unlock overhead vs.
   accidental-prune safety.
3. **`env` vs `create` overlap.** `env` re-derives the `.env` for an existing worktree. Design must pin
   whether `env` errors if the worktree does not exist (recommended: yes, `ErrWorktreeNotFound`) and
   whether `create` internally reuses the same `.env`-writing code path as `env` (recommended: yes — one
   generator, two entry points).
4. **Port-base override surface.** `Config` exposes the `8100` base for tests. Design must decide if the
   CLI surfaces a `--port-base` flag or keeps the base internal-only for Fase 2 (recommended:
   internal-only; no flag until a real consumer needs it).
