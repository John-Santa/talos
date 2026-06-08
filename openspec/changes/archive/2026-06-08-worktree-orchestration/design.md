# Design: worktree-orchestration

**Change:** worktree-orchestration · **Jira:** TAL-2 · **Module:** module:devops · **Owner:** HERMES
**Store:** hybrid (this file + engram `sdd/worktree-orchestration/design`)
**Reads:** proposal (#1834, 3 LOCKED + 6 resolved decisions) · exploration (#1831, git mechanics + layout)
**Module path:** `github.com/John-Santa/talos/platform/worktree-orchestrator`
**Mirror reference:** `platform/jira-evidence-loop/` (hexagonal, zero deps, hand-written mock)

This is the HOW at the architectural level. It does not enumerate task steps — `sdd-tasks`
turns these boundaries into a strict-TDD-ordered backlog. Functional requirements (the WHAT)
are owned by `sdd-spec` running in parallel; see §13 for the seams where the two phases meet.

---

## 1. Architecture approach

Hexagonal (ports & adapters), same skeleton as `jira-evidence-loop`, but with an **honest
caveat the proposal already flagged**: this module's real work is *sequencing external git
commands*, not pure computation. The dependency arrow still points inward —
`cmd → service → port ← adapter`, everything depends on `domain` — and the domain core has
zero I/O and zero third-party imports. But the port here (`GitRunner`) is a **thin command
boundary**, not a rich API like `JiraClient`. We keep it anyway, and §11 justifies that
decision openly rather than hiding it.

```
            ┌─────────────────────────────────────────────────┐
            │                 domain/worktree                 │  pure, no deps
            │  naming (regex/roster/validate) · env (assign-  │  value objects +
            │  map figura→port/schema, .env generator) ·      │  validation only
            │  porcelain parser · typed errors                │  (NO os/exec, NO disk)
            └─────────────────────────────────────────────────┘
                       ▲                         ▲
          imports      │                         │      imports
            ┌──────────┴──────────┐   ┌──────────┴───────────┐
            │  service/           │   │  adapter/gitcli/     │
            │  orchestrator       │──▶│  Runner (os/exec)    │
            │  create/list/       │   │  + porcelain feeder  │
            │  teardown/env       │   │  IMPLEMENTS the port │
            │  depends ONLY on    │   └──────────────────────┘
            │  port.GitRunner     │              ▲
            └─────────┬───────────┘              │ satisfies
                      │ depends on               │
                      ▼                          │
            ┌─────────────────────────────────────────────────┐
            │           port/GitRunner (interface)            │  the mockable boundary
            └─────────────────────────────────────────────────┘
                      ▲                           ▲
              mock/  ─┘ (hand-written)    cmd/wt ─┘ (wires real gitcli.Runner)
```

**Boundary rule:** `service` imports `port` + `domain`, never `adapter/gitcli`.
`adapter/gitcli` imports `port` + `domain` (to satisfy the interface and to call the pure
porcelain parser), never `service`. `cmd/wt` is the only place where the concrete
`gitcli.Runner` is constructed and handed to the orchestrator — the composition root.

**Where the value actually is (honest):** the *bulk* of the testable logic lives in
`domain/worktree` (naming validation, the assign-map, the `.env` generator, the porcelain
parser) and is unit-tested with zero git. The `service` layer is sequencing + typed-error
translation, tested against the mock. The `adapter` is genuinely thin (build argv, run,
hand stdout to the pure parser) and only its happy/edge behaviour against real git is worth
an integration test. This matches the exploration's verdict: Go CLI wins, but the "pure" core
is where the architecture earns its keep, not in port swapability.

---

## 2. Package layout

```
platform/worktree-orchestrator/
├── go.mod                        # module github.com/John-Santa/talos/platform/worktree-orchestrator
│                                 # go 1.26 — ZERO transitive deps (Kit goal, mirrors jira-loop)
├── README.md
├── domain/
│   └── worktree/
│       ├── naming.go             # figura roster + regex, branch/path builders, validation (pure)
│       ├── env.go                # assign-map figura→(port,schema) + .env generator (pure)
│       ├── porcelain.go          # ParseWorktreeList(stdout) ([]WorktreeInfo, error) (pure)
│       ├── errors.go             # typed errors: ErrInvalidFigure, ErrInvalidKey,
│       │                         #   ErrWorktreeExists, ErrDirtyWorktree, ErrBranchExists,
│       │                         #   ErrWorktreeNotFound
│       ├── naming_test.go        # table-driven, no git
│       ├── env_test.go           # table-driven (assign-map exhaustive + .env golden), no git
│       └── porcelain_test.go     # fixtures, no git
├── port/
│   └── git.go                    # GitRunner interface (6 methods)
├── service/
│   ├── orchestrator.go           # Orchestrator + Config + Create/List/Teardown/Env use cases
│   └── orchestrator_test.go      # mock GitRunner; happy + typed errors + call-order asserts
├── adapter/
│   └── gitcli/
│       ├── runner.go             # GitRunner impl over os/exec; delegates parse to domain.porcelain
│       └── runner_test.go        # //go:build !short gate via testing.Short(); real git in t.TempDir()
├── mock/
│   └── git_runner_mock.go        # hand-written test double (mirrors JiraClientMock pattern)
└── cmd/
    └── wt/
        └── main.go               # composition root: run(args) error + subcommand switch (flag pkg)
```

**Why `domain/worktree` is one package, not three sub-packages:** naming, env, and porcelain
are all pure value-logic over the same `Figura`/`WorktreeSpec` vocabulary. Splitting them adds
import ceremony without a boundary worth enforcing. They share `errors.go`. This mirrors
`domain/evidence` being a single package.

**Why `Config` lives in `service/` (not its own package):** identical reasoning to
jira-evidence-loop ADR-D1 — Config is the use-case input; co-locating keeps the construction
contract (`NewOrchestrator(runner, cfg)`) in one place. The adapter takes no config (it just
runs git in a given working directory passed per-call), so there is no dependency-arrow risk.

---

## 3. The Config struct

Injected at construction. The instance-specific knobs are minimal here — the port base and the
worktree base path — because the figura roster is fixed by CONSTITUTION §1 (see §6 / ADR-D5).

```go
package service

// Config carries the instance-specific worktree knobs. The figura→resource
// assign-map is NOT here — it is a pure domain function (see ADR-D5). Only the
// values that legitimately vary per repo checkout live here.
type Config struct {
	// RepoRoot is the absolute path to the main repository checkout. git
	// worktree commands run with this as the working directory.
	RepoRoot string

	// WorktreeBase is the directory (relative to RepoRoot or absolute) under
	// which agent worktrees are created, e.g. "talos.wt". A worktree for figura
	// "atlas" lands at <WorktreeBase>/agent-atlas. .gitignore already ignores
	// talos.wt/ (exploration confirmed).
	WorktreeBase string

	// BaseBranch is the branch new worktrees are cut from, e.g. "develop"
	// (CONSTITUTION §11 — PRs target develop). Fetched from origin before add
	// unless --no-fetch (Decision 8).
	BaseBranch string
}

// DefaultTALConfig seeds the real Talos values; cmd/wt overrides RepoRoot at runtime.
func DefaultTALConfig() Config {
	return Config{
		WorktreeBase: "talos.wt",
		BaseBranch:   "develop",
	}
}
```

**Resolution of OPEN QUESTION #4 (`--port-base` flag) → NO flag.** The port base stays a
constant inside the domain assign-map, marked `// DEBT(kit-extraction)`, NOT a `Config` field
and NOT a CLI flag. Rationale below (ADR-D5) and aligned with proposal Decision 6: the 8
figuras are fixed in CONSTITUTION §1; exposing `--port-base` now is generality nobody consumes
in Fase 2, and it would let a caller produce a `.env` whose port collides with another agent's
(the whole point of the deterministic map is collision-freedom). Per-instance configurability
is the Fase-5 Kit concern; it is isolated to one pure function so the extraction is a
single-file change.

---

## 4. The port interface

```go
package port

import "context"

// GitRunner is the driven port: the boundary over external git invocations the
// orchestrator needs. The service depends on this and nothing else.
// adapter/gitcli implements it over os/exec; mock/ implements it for tests.
//
// Methods are intentionally git-operation-shaped (not a single Run(args...))
// so the mock can assert INTENT (which operation, with what arguments) rather
// than raw argv strings, and so typed errors are produced at the boundary.
type GitRunner interface {
	// Fetch updates the remote tracking ref for branch (e.g. "develop") from
	// origin. No-op-safe to call repeatedly. Honours --no-fetch by the caller
	// simply not invoking it.
	Fetch(ctx context.Context, repoRoot, branch string) error

	// BranchExists reports whether a local branch named branch exists.
	// Used to fail-loud BEFORE worktree add (Decision 4: branch exists = active
	// issue, do not silently reuse).
	BranchExists(ctx context.Context, repoRoot, branch string) (bool, error)

	// WorktreeAdd creates a worktree at path on a NEW branch cut from base.
	// Maps to: git worktree add -b <branch> <path> <base>.
	WorktreeAdd(ctx context.Context, repoRoot, path, branch, base string) error

	// WorktreeList returns the raw `git worktree list --porcelain` stdout.
	// Parsing is the domain's job (pure) — the adapter does NOT parse.
	WorktreeList(ctx context.Context, repoRoot string) (porcelain string, err error)

	// WorktreeRemove removes the worktree at path. force maps to --force and is
	// ONLY passed when the caller explicitly opted in (dirty teardown, Decision 9).
	WorktreeRemove(ctx context.Context, repoRoot, path string, force bool) error

	// Prune runs `git worktree prune` to clear stale administrative entries
	// after a remove (exploration §git mechanics #6).
	Prune(ctx context.Context, repoRoot string) error
}
```

**Why operation-shaped, not `Run(ctx, args...)`:** the exploration sketched a generic
`Run(ctx, args...) (stdout, error)`. We reject that here. A generic runner pushes argv
construction into the service and makes the mock assert on brittle string slices
(`["worktree","add","-b",...]`). Operation-shaped methods (a) keep argv construction in the
adapter where it belongs, (b) let the mock record *semantic* calls (`WorktreeAdd(path,
branch, base)`) for clean order/argument assertions, and (c) are the natural place to map raw
git exit-text into typed domain errors. This is the same instinct as `JiraClient` having
`CreateIssue`/`DoTransition` rather than `Do(method, path, body)`.

**Note — no `BranchDelete` method in the base interface.** Teardown does NOT delete branches by
default (OPEN QUESTION #1, resolved below). The optional `--delete-branch` path adds a
`BranchDelete(ctx, repoRoot, branch string) error` method; it is included in the port from the
start (the mock implements it) but the orchestrator only calls it when the caller opts in.

---

## 5. Domain: naming (pure)

`domain/worktree/naming.go` — the figura roster, branch/path derivation, and input validation.
No I/O.

```go
// Figura is a validated agent name from the CONSTITUTION §1 roster.
type Figura string

// roster is the fixed set of valid figuras (CONSTITUTION §1). DEBT(kit-extraction):
// per-instance rosters become config in Fase 5; isolated here as one var.
var roster = map[Figura]bool{
	"atlas": true, "hephaestus": true, "cronos": true, "iris": true,
	"gaia": true, "themis": true, "hermes": true, "argos": true,
}

// jiraKeyRe validates the TAL-<n> issue key shape.
var jiraKeyRe = regexp.MustCompile(`^TAL-[1-9][0-9]*$`)

// ParseFigura validates s against the roster, returning ErrInvalidFigure if unknown.
func ParseFigura(s string) (Figura, error)

// ValidateJiraKey checks s matches TAL-<n>, returning ErrInvalidKey otherwise.
func ValidateJiraKey(s string) error

// BranchName returns the canonical branch: agent/<figura>/<KEY>  (CONSTITUTION §3).
func BranchName(f Figura, jiraKey string) string   // "agent/atlas/TAL-5"

// WorktreePath returns <base>/agent-<figura>  (relative to RepoRoot).
func WorktreePath(base string, f Figura) string    // "talos.wt/agent-atlas"
```

`WorktreeSpec` is the value object threaded through the service:

```go
type WorktreeSpec struct {
	Figura  Figura
	JiraKey string // validated TAL-<n>
	Branch  string // agent/<figura>/<key>
	Path    string // <base>/agent-<figura>
}

// NewWorktreeSpec validates figura + key and derives Branch/Path, or returns a typed error.
func NewWorktreeSpec(base, figura, jiraKey string) (WorktreeSpec, error)
```

This is the §3-style "validated value object built once" pattern (mirrors
`NewConfig`/`NewWorktreeConfig` from the exploration). The service never constructs branch/path
strings by hand — it always goes through `NewWorktreeSpec`, so naming rules have exactly one
home.

---

## 6. Domain: env + assign-map (pure)

`domain/worktree/env.go` — the deterministic, collision-free resource map and the `.env`
generator. **This is the heart of OPEN QUESTION #3 and #4.**

```go
// Resources is the disjoint resource allocation for one figura.
type Resources struct {
	Port     int    // 8100..8107, disjoint per figura
	DBSchema string // "wt_<figura>"
}

// portBase is the first allocated port. DEBT(kit-extraction): becomes config in
// Fase 5. NOT a Config field, NOT a CLI flag (ADR-D5 / OPEN QUESTION #4).
const portBase = 8100

// assignMap is the fixed figura→offset allocation. Order is stable and part of
// the contract — changing an offset reassigns a port and is a breaking change.
// DEBT(kit-extraction): per-instance map becomes config in Fase 5; isolated to
// THIS one var + portBase const so extraction is a single-file change.
var assignMap = map[Figura]int{
	"atlas": 0, "hephaestus": 1, "cronos": 2, "iris": 3,
	"gaia": 4, "themis": 5, "hermes": 6, "argos": 7,
}

// AgentResources returns the disjoint Port+DBSchema for figura, or
// ErrInvalidFigure if it is not in the roster. Pure, deterministic, no I/O.
func AgentResources(f Figura) (Resources, error)
```

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

**The shared `.env` generator (resolution of OPEN QUESTION #3 → YES, one pure function):**

```go
// RenderEnv produces the canonical .env file contents for a spec. It is the
// SINGLE source of .env truth: both `create` (writing a new worktree's .env) and
// `env` (re-deriving idempotently into an existing worktree) call this exact
// function. DRY by construction — there is no second code path that can drift.
//
// Per Decision 5: the generated .env carries ONLY PORT + DB_SCHEMA. JIRA_*
// credentials are NOT written here — ATHENA propagates them from the parent
// environment. The .env.example at repo root documents the JIRA_* contract.
func RenderEnv(spec WorktreeSpec, res Resources) string
```

Emitted shape (golden-tested in `env_test.go`):

```env
# Generated by wt — DO NOT EDIT MANUALLY
# Agent: atlas | Issue: TAL-5
PORT=8100
DB_SCHEMA=wt_atlas
```

**Why one function, not two:** `create` and `env` differ only in *when* they write (new vs
re-derive) and *whether the worktree must already exist* (`env` returns `ErrWorktreeNotFound`
if it does not). The *content* is identical and must be byte-for-byte reproducible — that is
the entire point of `env` being "idempotent re-derive." Two generators would be two chances to
drift. So the content is one pure `RenderEnv`; the orchestrator's `Create` and `Env` use cases
both call it and differ only in their surrounding pre-conditions and file-write step.

**No JIRA_* placeholders in the generated file (Decision 5 lock).** The exploration floated
including empty `JIRA_*` placeholders; the proposal LOCKED them out. `RenderEnv` emits PORT +
DB_SCHEMA only. The repo-root `.env.example` (owned by HERMES, created by this change)
documents that ATHENA injects `JIRA_EMAIL` / `JIRA_API_TOKEN` / `JIRA_SITE_URL` from the parent
env — they never live in a generated worktree `.env`.

---

## 7. Domain: porcelain parser (pure)

`domain/worktree/porcelain.go` — `git worktree list --porcelain` stdout → `[]WorktreeInfo`.
100% unit-testable from fixtures; zero git.

```go
type WorktreeInfo struct {
	Path     string // absolute worktree path
	Head     string // commit SHA
	Branch   string // "refs/heads/agent/atlas/TAL-5" → exposed as "agent/atlas/TAL-5"
	Detached bool   // true when the entry is a detached HEAD (no branch line)
	Bare     bool   // true for the bare entry (rare; main repo)
}

// ParseWorktreeList parses porcelain stdout. Blocks are separated by a blank
// line; each block has `worktree <path>`, `HEAD <sha>`, then `branch <ref>` OR
// `detached`. Returns Err* only on malformed input — an empty list is valid.
func ParseWorktreeList(stdout string) ([]WorktreeInfo, error)
```

Porcelain format the parser must handle (fixture cases): a normal branch block, a `detached`
block (HEAD present, no `branch` line, a `detached` line), the `bare` line on the main entry,
and trailing/leading blank lines. The `refs/heads/` prefix is stripped on the way out so the
service can compare against `BranchName(...)` directly.

**Why parsing lives in domain, not adapter:** the adapter's job is to *get bytes from git*; the
*meaning* of those bytes is domain logic and the most error-prone part (block framing, detached
edge case). Keeping it pure makes it the cheapest thing to test exhaustively, and lets the
service reason over `[]WorktreeInfo` without ever touching strings. The adapter's
`WorktreeList` returns raw stdout precisely so the parser can stay pure and fixture-driven.

---

## 8. Service: the four use cases

`service/orchestrator.go`. `Orchestrator` holds `(runner port.GitRunner, cfg Config)` and
exposes four methods. Each names where a **domain validation (V)**, a **state pre-check (P)**,
and a **fail-loud (F)** sits.

```go
func NewOrchestrator(runner port.GitRunner, cfg Config) *Orchestrator

func (o *Orchestrator) Create(ctx context.Context, in CreateInput) (WorktreeSpec, error)
func (o *Orchestrator) List(ctx context.Context) ([]WorktreeInfo, error)
func (o *Orchestrator) Teardown(ctx context.Context, in TeardownInput) error
func (o *Orchestrator) Env(ctx context.Context, in EnvInput) (string, error) // returns rendered .env
```

### Create — sequence

```
INPUT: figura, jiraKey, [noFetch bool]

  0. SPEC BUILD (V) ── NewWorktreeSpec(cfg.WorktreeBase, figura, jiraKey)
     │  invalid figura ⇒ ErrInvalidFigure;  invalid key ⇒ ErrInvalidKey.  STOP, no git.
  1. RESOURCES (V) ── AgentResources(spec.Figura)  (cannot fail post-step-0; defensive)
  2. PRE-CHECK branch (P) ── runner.BranchExists(spec.Branch)
     │  exists ⇒ ErrBranchExists (Decision 4: active issue, fail-loud, NO silent reuse).
  3. PRE-CHECK path (P) ── ParseWorktreeList(runner.WorktreeList()) ; any entry at spec.Path
     │  ⇒ ErrWorktreeExists.   (also catches a path squatted by a non-git dir via the adapter.)
  4. FETCH (unless noFetch, Decision 8) ── runner.Fetch(cfg.BaseBranch)
  5. ADD ── runner.WorktreeAdd(spec.Path, spec.Branch, "origin/"+cfg.BaseBranch)
  6. WRITE .env ── os.WriteFile(spec.Path/".env", RenderEnv(spec, res))   ← composition concern*
OUTPUT: spec
```

\* The actual `os.WriteFile` is a thin filesystem effect. It is NOT in the `GitRunner` port
(it is not git). Two honest placements: keep it in the service guarded behind a tiny
`fileWriter` func value (default `os.WriteFile`, swapped in tests), OR perform it in `cmd/wt`
after `Create` returns the spec+rendered content. **Decision: the service owns it via an
injected `WriteFile func(path string, data []byte, perm fs.FileMode) error` field on the
Orchestrator** (defaults to `os.WriteFile`). Rationale: `.env` generation is the use case's
responsibility (a worktree without its `.env` is half-created), and a one-field func injection
keeps the service unit-testable without disk while not inventing a second port for one stdlib
call. `sdd-tasks` must wire this field, defaulting it in `NewOrchestrator`.

### List — sequence

```
  1. runner.WorktreeList(cfg.RepoRoot) ⇒ raw porcelain
  2. ParseWorktreeList(raw) ⇒ []WorktreeInfo   (pure)
OUTPUT: []WorktreeInfo  (cmd/wt renders tabular default or --json, Decision 7)
```

List is read-only; rendering (tabular vs `--json`) is a `cmd/wt` concern, not service logic.

### Teardown — sequence

```
INPUT: figura, jiraKey, force bool, deleteBranch bool

  0. SPEC BUILD (V) ── NewWorktreeSpec(...)
  1. EXISTS (P) ── ParseWorktreeList(runner.WorktreeList()) ; no entry at spec.Path
     │  ⇒ ErrWorktreeNotFound.
  2. DIRTY GUARD (P/F) ── if worktree has uncommitted changes AND NOT force
     │  ⇒ ErrDirtyWorktree.  (Decision 9: CLI is dumb; only ATHENA decides --force.)
  3. REMOVE ── runner.WorktreeRemove(spec.Path, force)
  4. PRUNE ── runner.Prune()   (exploration #6: clear stale admin entries)
  5. (opt) BRANCH DELETE ── if deleteBranch ⇒ runner.BranchDelete(spec.Branch)
     │  DEFAULT: do NOT delete (OPEN QUESTION #1 resolved → opt-in only).
OUTPUT: nil
```

**Dirty detection placement:** the dirtiness check needs a git query
(`git status --porcelain` in the worktree). That is an external git call, so it belongs behind
the port — but it is a *read* and is teardown-specific. Rather than bloat `GitRunner` with a
`Status` method used in one place, `WorktreeRemove` already fails-loud when dirty
(git's native "contains modified or untracked files, use --force"). **Decision: rely on git's
native dirty refusal.** The adapter maps that specific failure to `ErrDirtyWorktree` (string
match on git's stderr, asserted in the integration test). When `force` is true, the adapter
passes `--force` and the refusal never fires. This avoids a one-use `Status` port method and
keeps the "CLI is dumb, caller decides --force" contract exact. `sdd-tasks` must implement the
stderr→`ErrDirtyWorktree` mapping in the adapter and assert it in the integration test.

### Env — sequence (idempotent re-derive)

```
INPUT: figura, jiraKey

  0. SPEC BUILD (V) ── NewWorktreeSpec(...)
  1. EXISTS (P) ── ParseWorktreeList(runner.WorktreeList()) ; no entry at spec.Path
     │  ⇒ ErrWorktreeNotFound  (Decision 9 / OQ#3: env requires an existing worktree).
  2. RESOURCES ── AgentResources(spec.Figura)
  3. RENDER + WRITE ── o.WriteFile(spec.Path/".env", RenderEnv(spec, res))
OUTPUT: rendered .env string  (for the caller / tests to inspect)
```

`Env` and `Create` step 6 call the **same** `RenderEnv` — that is the OQ#3 DRY guarantee made
structural. `Env` differs only by requiring the worktree to pre-exist (`ErrWorktreeNotFound`)
and by skipping branch/path creation. Re-running `Env` is byte-identical and side-effect-safe.

---

## 9. Typed errors

`domain/worktree/errors.go` — all errors are typed structs implementing `error`, mirroring
`ErrOwnershipViolation`. They carry enough context for the CLI to print a clear message and
for the caller agent (ATHENA) to branch on `errors.As`.

```go
type ErrInvalidFigure  struct{ Figura string }                    // not in roster
type ErrInvalidKey     struct{ Key string }                       // not TAL-<n>
type ErrWorktreeExists struct{ Figura, Path string }              // create: path occupied
type ErrBranchExists   struct{ Branch string }                    // create: branch already cut
type ErrWorktreeNotFound struct{ Figura, Path string }            // teardown/env: nothing there
type ErrDirtyWorktree  struct{ Figura, Path string }              // teardown: uncommitted, no --force
```

`ErrDirtyWorktree` deliberately does NOT carry an uncommitted-file count (the exploration
floated it): getting that count needs a second git call we decided against (§8 dirty guard).
The error states the fact ("dirty, pass --force to discard") — counting is not the CLI's job.

The service returns these directly; the adapter is responsible for translating raw git
failures into the two it can detect at the boundary (`ErrBranchExists` is detected by the
`BranchExists` pre-check, not adapter string-matching; `ErrDirtyWorktree` IS adapter
string-matching on remove failure). The CLI maps each typed error to a distinct exit code.

---

## 10. CLI surface

`cmd/wt/main.go` — composition root, the `run(args) error` + subcommand-switch pattern lifted
verbatim from `cmd/evidence/main.go` (testable entry point separate from `main()`).

```
wt create   <figura> <TAL-N> [--no-fetch]                  # Decision 8: fetches develop by default
wt list     [--json]                                        # Decision 7: tabular default, --json opt-in
wt teardown <figura> <TAL-N> [--force] [--delete-branch]    # Decision 9 + OQ#1
wt env      <figura> <TAL-N>                                # idempotent .env re-derive (OQ#3)
```

```go
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "wt: %v\n", err)
		os.Exit(exitCodeFor(err)) // typed-error → distinct code
	}
}

func run(args []string) error {
	if len(args) == 0 { return fmt.Errorf("subcommand required: create|list|teardown|env") }
	switch args[0] {
	case "create":   return cmdCreate(args[1:])
	case "list":     return cmdList(args[1:])
	case "teardown": return cmdTeardown(args[1:])
	case "env":      return cmdEnv(args[1:])
	default:         return fmt.Errorf("unknown subcommand %q; available: create|list|teardown|env", args[0])
	}
}
```

Each `cmd*` uses its own `flag.NewFlagSet(name, flag.ContinueOnError)`. Positional args
(`figura`, `TAL-N`) are read from `fs.Args()` after parse — flags before positionals or
mixed, standard `flag` behaviour. The composition root resolves `RepoRoot` (via `os.Getwd` /
`git rev-parse --show-toplevel`), builds `DefaultTALConfig()`, constructs `gitcli.NewRunner()`,
wires `service.NewOrchestrator(runner, cfg)`, and dispatches.

**`--no-fetch` (Decision 8):** default behaviour fetches `origin/develop` before add so a stale
checkout still cuts from fresh develop; `--no-fetch` opts out for offline / already-fetched
callers. **`--json` (Decision 7):** `list` prints a tabular table for humans by default; ATHENA
passes `--json` for machine parsing. **`--lock`:** NOT exposed — see OQ#2 resolution (§11).

---

## 11. ADR-style decisions (this phase)

The 3 LOCKED proposal decisions (hexagonal mirror, no harness `EnterWorktree`, boundary with
NO CI touch) are not re-opened. This design adds:

**ADR-D1 — Keep the `GitRunner` port despite no second backend (THE tension, faced openly).**
The proposal correctly flagged that `GitRunner` buys **no swapability** — there is exactly one
git, there will never be a second backend, so the classic "ports enable adapter substitution"
justification does NOT apply here. *Decision: keep the port anyway, justified solely by
testability.* The mock `GitRunner` is what lets `service/orchestrator_test.go` exercise the
full create/teardown/env sequencing — including **injected typed git errors** (`ErrBranchExists`
on pre-check, a `WorktreeAdd` failure, a dirty-remove failure) and **call-order assertions**
(fetch → add → write; list → remove → prune) — **without touching disk or spawning git**. That
is real, recurring value: the service is the orchestration brain and it must be tested against
adverse git outcomes cheaply and deterministically. *Rejected — collapse the port and have the
service call `os/exec` directly:* it would make every service test an integration test (real
git, real temp repos, slow, can't inject a mid-sequence failure), and would put argv
construction + process spawning inside the orchestration logic. The honest framing: this is a
**test-seam port, not a swapability port**, and we name it as such rather than pretending it
abstracts a pluggable backend. The cost (one interface + one hand-written mock) is small and
paid once; the benefit (fast, deterministic, failure-injecting service tests) is paid on every
test run.

**ADR-D2 — Teardown does NOT delete the branch by default; `--delete-branch` is opt-in
(OPEN QUESTION #1).** *Decision: teardown removes only the worktree (+ prune); the branch
`agent/<figura>/<TAL-N>` is left intact unless the caller passes `--delete-branch`.* *Rationale:*
the worktree's whole reason to exist is an open PR against `develop`, and **at teardown time the
PR may not be merged yet** (the agent finished its work, ATHENA reclaims the worktree, but the
branch must survive for review/merge). Deleting the branch by default would destroy unmerged
work and break the merge-order flow (Fase 2 change #2) that operates on those branches.
`git branch -d` (safe delete) even refuses unmerged branches — so a default delete would either
fail loudly (annoying) or need `-D` (dangerous). The CLI stays dumb (CONSTITUTION-aligned,
Decision 9): it deletes a branch only on explicit `--delete-branch`, which maps to safe
`git branch -d` (refuses if unmerged) — never `-D`. *Rejected — delete by default:* destroys
unmerged work; *Rejected — never offer deletion:* leaves orphan branches with no first-class
cleanup path for the merged/abandoned case ATHENA legitimately wants to tidy.

**ADR-D3 — `--lock` on `worktree add` is NOT used (OPEN QUESTION #2).** `git worktree add
--lock` marks a worktree so `prune` won't auto-remove it even if its directory vanishes.
*Decision: do not use `--lock`.* *Rationale:* lock exists to protect worktrees on **removable
media** (USB drive unmounted ⇒ directory gone ⇒ prune would discard the admin entry). Talos
worktrees live under `talos.wt/` on the same local disk as the repo — they never disappear out
from under git. `--lock` would actively HARM the teardown contract: a locked worktree refuses
`git worktree remove` until `git worktree unlock` runs first, adding a step and a failure mode
for zero benefit. Our lifecycle is explicit (`wt teardown`), not media-driven. *Rejected — lock
to prevent accidental prune:* `prune` only removes entries whose **directory is already gone**;
a live Talos worktree is never in that state, so there is nothing to protect against.

**ADR-D4 — `GitRunner` is operation-shaped (6 methods), not generic `Run(args...)`.**
*Rationale (full text §4):* keeps argv in the adapter, lets the mock assert semantic calls and
order, and is the natural typed-error boundary. *Rejected — generic `Run(ctx, args...)`:* pushes
argv into the service and makes mock assertions brittle string-slice comparisons.

**ADR-D5 — The assign-map (figura→port/schema) is a pure domain function with a hardcoded
roster, NOT injected config and NOT a `--port-base` flag (OPEN QUESTION #4, proposal
Decision 6).** *Rationale:* the 8 figuras are fixed in CONSTITUTION §1; the map's entire value
is being **deterministic and collision-free**, which a runtime-overridable base would
undermine (two callers with different `--port-base` produce colliding `.env`s — the exact bug
the map prevents). Injectability now is generality with no Fase-2 consumer. The Kit-extraction
need (per-instance maps in Fase 5) is satisfied by isolating the map + base to **one var + one
const in one file**, each marked `// DEBT(kit-extraction)`, so the future change is a single
mechanical edit. *Rejected — `--port-base` flag / `Config` field:* speculative generality that
weakens the collision guarantee; *Rejected — config-file map in Fase 2:* nothing consumes it
yet and CONSTITUTION fixes the roster.

**ADR-D6 — One shared `RenderEnv` pure function feeds both `create` and `env` (OPEN
QUESTION #3).** *Rationale:* `env`'s contract is "idempotent re-derive," which only holds if the
content is byte-identical to what `create` wrote — guaranteed structurally by a single
generator. *Rejected — separate generators:* two drift surfaces for content that must match
exactly.

**ADR-D7 — `.env` write is a service responsibility via an injected `WriteFile` func, not a
port method and not a `cmd` concern.** *Rationale (full text §8 Create):* a worktree without
its `.env` is half-created, so generation belongs to the use case; a one-field func injection
keeps the service disk-free in tests without inventing a filesystem port for one stdlib call.
*Rejected — `.env` write in `cmd/wt`:* splits a single use case across two layers; *Rejected —
a `FileSystem` port:* over-engineering one `os.WriteFile`.

---

## 12. Testing strategy (strict TDD — skill go-testing)

Strict TDD is active: every domain/service/adapter-pure file gets a **failing test first**.
The split mirrors where the value is (§1): the bulk is pure domain, fast and exhaustive.

**Unit — `domain/worktree/` (the bulk; zero git, table-driven):**
- `naming_test.go` — `ParseFigura` (every roster member + several rejects), `ValidateJiraKey`
  (`TAL-1`, `TAL-42` pass; `TAL-0`, `tal-5`, `FOO-1`, `TAL-`, empty reject), `BranchName` /
  `WorktreePath` exact-string, `NewWorktreeSpec` happy + each typed error.
- `env_test.go` — `AgentResources` **exhaustive over all 8 figuras** (assert exact port +
  schema, the collision-free contract) + invalid figura. `RenderEnv` golden-string assertion
  (PORT + DB_SCHEMA only; **assert NO `JIRA_` substring** — Decision 5 guard).
- `porcelain_test.go` — fixtures: single branch block, multi-worktree, detached HEAD, bare main
  entry, leading/trailing blank lines, empty input (valid → empty slice), malformed (→ error).

**Unit — `service/orchestrator_test.go` (mock `GitRunner`, zero git):**
- `Create` happy: assert call ORDER `BranchExists → WorktreeList → Fetch → WorktreeAdd` and
  that `WriteFile` received `RenderEnv` output at `spec.Path/.env`.
- `Create --no-fetch`: assert `Fetch` is NOT called.
- `Create` typed errors: invalid figura (STOP before any runner call), `ErrBranchExists`
  (BranchExists→true ⇒ no add), `ErrWorktreeExists` (list shows path ⇒ no add),
  `WorktreeAdd` failure propagates.
- `Teardown` happy: order `WorktreeList → WorktreeRemove(force=false) → Prune`; assert
  `BranchDelete` NOT called (default). `--delete-branch`: assert `BranchDelete` IS called.
  `--force`: assert `WorktreeRemove` got `force=true`. `ErrWorktreeNotFound` when list is empty.
- `Env`: `ErrWorktreeNotFound` when absent; happy path writes identical bytes to a
  `create`-then-`env` sequence (the idempotency proof).
- The mock injects errors per method and records ordered calls (mirrors `JiraClientMock`:
  `Calls []Call`, `*Result`/`*Err` fields, `AssertMethodOrder`).

**Integration — `adapter/gitcli/runner_test.go` (real git, `testing.Short()`-gated):**
- Guard top of each test: `if testing.Short() { t.Skip("integration: real git") }`. Run the
  fast suite with `go test -short ./...`; full suite with `go test ./...`.
- Setup: `t.TempDir()` → `git init` → an initial commit → a local `develop` branch (so
  `origin/develop` resolution / `-b ... <base>` has something to cut from; for the no-network
  case use a local base branch instead of a real `origin`).
- Cases: `WorktreeAdd` creates the dir + branch; `WorktreeList` stdout round-trips through
  `ParseWorktreeList`; `WorktreeRemove` on a **dirty** worktree fails and the adapter maps it to
  `ErrDirtyWorktree`; `WorktreeRemove(force=true)` succeeds on dirty; `Prune` clears a stale
  entry; `BranchExists` true/false. This is the ONLY place real git runs.

**What's covered without git:** all naming/key validation, the entire assign-map + collision
contract, the `.env` content + Decision-5 guard, the porcelain parser's every edge, and the
full create/list/teardown/env **orchestration including injected git failures and call order**.
**What needs real git:** argv correctness, the dirty-refusal stderr→`ErrDirtyWorktree` mapping,
and actual worktree/branch/prune side effects — all in the one `testing.Short()`-gated file.

---

## 13. Decisions tasks must respect (and spec-parallelism seams)

1. **Strict-TDD order:** domain (naming, env, porcelain, errors) FIRST and exhaustively, then
   the service against the mock, then the adapter against real git (short-gated). cmd/wt last.
2. **Dependency arrows:** `service` never imports `adapter/gitcli`; `cmd/wt` is the only
   composition root; the porcelain parser lives in `domain` and the adapter calls it (adapter
   imports domain, not vice-versa). Any task wiring git into the service is wrong.
3. **Zero transitive deps:** hand-written runner (`os/exec`), hand-rolled CLI (`flag`),
   hand-written mock, stdlib `testing` + `regexp`. `go.sum` stays minimal — the Kit gate.
4. **Port is a test-seam, kept (ADR-D1):** do NOT collapse `GitRunner` into direct `os/exec` in
   the service "to simplify" — that destroys the failure-injection tests.
5. **Operation-shaped port (ADR-D4):** 6 base methods + opt-in `BranchDelete`; the mock
   implements all of them. No generic `Run(args...)`.
6. **Teardown branch default (ADR-D2):** worktree-only by default; branch survives; only
   `--delete-branch` deletes, mapped to safe `git branch -d` (never `-D`).
7. **No `--lock` (ADR-D3):** `worktree add` is plain; do not add `--lock` / `unlock` handling.
8. **Single `RenderEnv` (ADR-D6):** `create` and `env` MUST call the same generator; no second
   `.env` content path.
9. **`--port-base` is NOT a flag (ADR-D5):** assign-map stays a hardcoded `portBase` const +
   `assignMap` var, both `// DEBT(kit-extraction)` in one file.
10. **`.env` = PORT + DB_SCHEMA only (Decision 5):** no `JIRA_*` in generated files; the
    `env_test.go` golden MUST assert the absence of `JIRA_`.
11. **`.env` write via injected `WriteFile` field (ADR-D7):** defaulted in `NewOrchestrator`,
    overridable in tests; not a port method.
12. **Dirty teardown via git's native refusal (ADR-D1/§8):** adapter maps the remove-failure
    stderr to `ErrDirtyWorktree`; assert it in the integration test. No `Status` port method.
13. **`create` fail-loud on existing branch/path (Decision 4):** `ErrBranchExists` /
    `ErrWorktreeExists`; NO silent reuse, NO `--reuse` flag.

**Seams with `sdd-spec` (running in parallel — flag if spec formalizes differently):**
- Spec owns the **functional requirements** (REQ-* for each subcommand's observable behaviour,
  exit codes, the `.env` contract, error surfaces). This design owns the **structure**
  (packages, the port, the assign-map placement, the test split).
- **Potential conflict to watch:** if spec writes a REQ that teardown deletes the branch by
  default, or that `.env` includes `JIRA_*` placeholders, or that `--port-base` is a user-facing
  flag, it contradicts ADR-D2 / Decision 5 / ADR-D5 here — `sdd-tasks` (which reads BOTH) must
  reconcile in favour of these locked design decisions, or escalate. This design treats OQ#1–4
  as RESOLVED; spec should formalize the *requirements* of those resolutions, not re-open them.
- **Non-Go tasks** (outside this module, for `sdd-tasks` to enumerate): create root
  `.env.example` documenting the `JIRA_*` injection contract (HERMES-owned); flip
  `module:devops` slot → active + add the shared-file map in `team-context/ownership.md`; add
  the `wt` binary to `.gitignore`. **This change does NOT touch `ci/pr-checks.yml`** (LOCKED
  boundary — go-test-in-CI is change #4 ci-hardening).
