# Spec: worktree-orchestration — Functional Requirements

**Change:** worktree-orchestration
**Jira:** TAL-2
**Module:** module:devops
**Owner:** HERMES
**RFC 2119 keywords apply throughout.**

---

## Overview

This spec describes the observable behaviour that MUST be true after the `worktree-orchestration`
change is fully applied. It does NOT prescribe implementation or package structure; that is the
design phase's responsibility.

The system under specification is a Go CLI binary `wt` residing in
`platform/worktree-orchestrator/`. It manages git worktrees for Talos agent figures, producing
per-worktree branches and `.env` files with deterministic, disjoint port and DB-schema
assignments. The CLI also creates `.env.example` at the repository root.

Four subcommands are in scope: `create`, `list`, `teardown`, `env`.
CI integration (`pr-checks.yml`, `go test` in CI) is explicitly out of scope (change #4).

---

## REQ-NAMING — Input validation

### REQ-NAMING-1: Valid figura values
**Given** any invocation of `wt` that takes a `<figura>` argument,
**When** the argument is parsed,
**Then** the CLI MUST accept exactly the eight values defined in CONSTITUTION §1:
`atlas`, `hephaestus`, `cronos`, `iris`, `gaia`, `themis`, `hermes`, `argos`.
Any other value MUST be rejected immediately with `ErrUnknownFigura` before any git operation.

### REQ-NAMING-2: Valid Jira key format
**Given** any invocation of `wt` that takes a `<jira-key>` argument (e.g. `TAL-2`),
**When** the argument is parsed,
**Then** the CLI MUST accept only values matching the pattern `TAL-<positive-integer>`
(uppercase, no leading zeros beyond a single digit, no extra characters).
Any non-matching value MUST be rejected with `ErrInvalidJiraKey` before any git operation.

### REQ-NAMING-3: Branch name derivation
**Given** valid `<figura>` and `<jira-key>` inputs,
**When** the CLI derives the branch name,
**Then** the resulting branch name MUST be exactly `agent/<figura>/<jira-key>`
(e.g. `agent/atlas/TAL-5`).
No other format is acceptable.

### REQ-NAMING-4: Worktree path derivation
**Given** a valid `<figura>` input,
**When** the CLI derives the worktree directory path,
**Then** the path MUST be `talos.wt/agent-<figura>` relative to the repository root
(e.g. `talos.wt/agent-atlas`).
No other path format is acceptable.

---

## REQ-CREATE — Worktree creation

### REQ-CREATE-1: Base branch
**Given** a `wt create <figura> <jira-key>` invocation,
**When** the worktree is created,
**Then** it MUST branch from `develop` (local tracking branch of `origin/develop`).
Branching from `main`, `origin/main`, or any other ref is PROHIBITED.

### REQ-CREATE-2: Fetch develop by default
**Given** a `wt create` invocation without the `--no-fetch` flag,
**When** the CLI executes,
**Then** it MUST run `git fetch origin develop` before creating the worktree, ensuring
`origin/develop` is up-to-date.

### REQ-CREATE-3: Opt-out of fetch
**Given** a `wt create` invocation with the `--no-fetch` flag,
**When** the CLI executes,
**Then** it MUST skip the fetch step and use whatever `origin/develop` ref is already present
locally. The caller assumes responsibility for staleness.

### REQ-CREATE-4: develop not local — fallback
**Given** `develop` does not exist as a local branch,
**When** `git worktree add` is invoked,
**Then** the CLI MUST use `origin/develop` as the base ref directly rather than failing.
The resulting branch MUST still be named `agent/<figura>/<jira-key>`.

### REQ-CREATE-5: Fail-loud on existing worktree path
**Given** the target worktree path (`talos.wt/agent-<figura>`) already exists on disk,
**When** `wt create` is invoked,
**Then** the CLI MUST return `ErrWorktreeExists` and MUST NOT attempt `git worktree add`.
The error message MUST include the conflicting path.

### REQ-CREATE-6: Fail-loud on existing branch
**Given** the branch `agent/<figura>/<jira-key>` already exists locally OR is checked out
in another worktree,
**When** `wt create` is invoked,
**Then** the CLI MUST return `ErrBranchExists` and MUST NOT create a new worktree.
The error message MUST include the conflicting branch name and, if applicable, the path
of the worktree where it is currently checked out.

### REQ-CREATE-7: .env generation on create
**Given** a successful `git worktree add`,
**When** the CLI completes the create step,
**Then** it MUST write a `.env` file at `talos.wt/agent-<figura>/.env` containing at minimum
the `PORT` and `DB_SCHEMA` values derived from REQ-ASSIGN.
See REQ-ENV for the exact content contract.

### REQ-CREATE-8: Atomicity guarantee
**Given** any failure after `git worktree add` but before `.env` creation (e.g. I/O error),
**When** the CLI surfaces the error,
**Then** it MUST report the partial state clearly: the worktree was created but `.env` was not
written. The CLI MUST NOT silently leave an incomplete worktree without reporting it.
Automatic rollback of the git worktree on `.env` failure is a design-time decision (see DC-1).

---

## REQ-ASSIGN — Deterministic resource assignment

### REQ-ASSIGN-1: Port assignment table
**Given** any valid figura,
**When** the CLI derives the port,
**Then** the assignment MUST follow this fixed table (CONSTITUTION §1 ordering):

| Figura | Port |
|---|---|
| atlas | 8100 |
| hephaestus | 8101 |
| cronos | 8102 |
| iris | 8103 |
| gaia | 8104 |
| themis | 8105 |
| hermes | 8106 |
| argos | 8107 |

Port range 8108+ is reserved for future figuras. The table is NOT configurable at runtime in
Fase 2; making it configurable is deferred to Fase 5 kit-extraction.

### REQ-ASSIGN-2: DB schema assignment
**Given** any valid figura,
**When** the CLI derives the DB schema name,
**Then** it MUST be `wt_<figura>` (e.g. `wt_atlas`, `wt_hephaestus`).

### REQ-ASSIGN-3: Disjoint guarantee
**Given** any two distinct figuras,
**When** their resources are derived,
**Then** they MUST have different ports AND different DB schema names.
No two figuras may share a port or schema under any invocation sequence.

### REQ-ASSIGN-4: Determinism across invocations
**Given** the same figura,
**When** `wt create` or `wt env` is called at any time (including after teardown and re-create),
**Then** the derived port and DB schema MUST be identical to all prior derivations for that
figura. The assignment is stateless and does not depend on creation order.

---

## REQ-ENV — .env file generation

### REQ-ENV-1: .env content
**Given** a worktree for `<figura>` associated with `<jira-key>`,
**When** the `.env` file is written (by `create` or `env` subcommand),
**Then** it MUST contain exactly these variables and no others:

```
PORT=<port>
DB_SCHEMA=<schema>
```

Where `<port>` and `<schema>` are derived from REQ-ASSIGN.
Jira credentials (`JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_SITE_URL`) MUST NOT appear in the
generated `.env`; they are propagated by ATHENA from the host environment.

### REQ-ENV-2: .env header comment
**Given** a generated `.env` file,
**When** it is read,
**Then** it MUST begin with a comment block identifying: the generator (`wt create`), the figura,
the jira key, and the creation timestamp in ISO 8601 UTC format (YYYY-MM-DDTHH:MM:SSZ).
This comment is informational and MUST NOT be parsed by the CLI itself.

### REQ-ENV-3: env subcommand re-derives idempotently
**Given** `wt env <figura>` is invoked when the worktree `talos.wt/agent-<figura>` exists,
**When** the subcommand runs,
**Then** it MUST overwrite `.env` with freshly derived content that is byte-for-byte identical
to what `create` would have produced (excluding the timestamp comment).
The operation MUST be idempotent: calling it N times produces the same file each time.

### REQ-ENV-4: env subcommand fails when worktree absent
**Given** `wt env <figura>` is invoked but the worktree directory does not exist,
**When** the subcommand runs,
**Then** it MUST return `ErrWorktreeNotFound` and MUST NOT write any file.

### REQ-ENV-5: .env.example at repository root
**Given** the change is applied,
**When** the repository root is inspected,
**Then** a file `.env.example` MUST exist documenting all variables that a worktree `.env` may
contain. It MUST include:

```
# Talos worktree .env — copy to .env and fill values
PORT=8100
DB_SCHEMA=wt_<figura>
JIRA_EMAIL=
JIRA_API_TOKEN=
JIRA_SITE_URL=https://tablex.atlassian.net
```

The Jira variables appear as empty placeholders to document that ATHENA injects them; they are
not generated by the CLI. The `.env.example` is owned by HERMES (`module:devops`).

---

## REQ-LIST — Listing active worktrees

### REQ-LIST-1: Source of truth
**Given** `wt list` is invoked,
**When** it enumerates worktrees,
**Then** it MUST use `git worktree list --porcelain` as the source of truth and MUST filter
to entries whose branch matches `refs/heads/agent/*`.
The main worktree (repo root) MUST NOT appear in the output.

### REQ-LIST-2: Tabular default output
**Given** `wt list` is invoked without `--json`,
**When** it prints results,
**Then** it MUST emit a human-readable table with at minimum these columns:
`FIGURA`, `BRANCH`, `PATH`, `HEAD` (short SHA), `STATUS`.
Column headers MUST be present.

### REQ-LIST-3: JSON output
**Given** `wt list --json` is invoked,
**When** it prints results,
**Then** it MUST emit a JSON array where each element contains at minimum:
`figura` (string), `branch` (string), `path` (string), `head` (string, full SHA),
`status` (string — see REQ-LIST-4).
The output MUST be valid JSON parseable by standard tools.

### REQ-LIST-4: Status field values
**Given** any listed worktree entry,
**When** its `STATUS` field is populated,
**Then** it MUST be one of:
- `active` — worktree directory exists and branch is present locally
- `orphan` — worktree directory exists but its branch has been deleted locally
- `stale` — worktree is registered in git's internal state but directory no longer exists on disk

This status set is sufficient for ATHENA to detect orphaned or stale worktrees.

### REQ-LIST-5: Empty result
**Given** no `agent/*` worktrees are active,
**When** `wt list` runs,
**Then** the tabular output MUST print the header row and a message "no active agent worktrees".
The JSON output MUST emit an empty array `[]`.

---

## REQ-TEARDOWN — Worktree removal

### REQ-TEARDOWN-1: Normal teardown sequence
**Given** `wt teardown <figura>` is invoked and the worktree is clean,
**When** the command executes,
**Then** it MUST run in order:
1. `git worktree remove talos.wt/agent-<figura>`
2. `git worktree prune`

The worktree directory MUST NOT exist on disk after successful completion.

### REQ-TEARDOWN-2: Reject dirty worktree without --force
**Given** the target worktree has uncommitted changes or untracked files,
**When** `wt teardown <figura>` is invoked without `--force`,
**Then** the CLI MUST return `ErrDirtyWorktree` immediately, perform NO git operations,
and report the number of modified/untracked files.
It MUST NOT silently pass `--force` to git.

### REQ-TEARDOWN-3: Force teardown of dirty worktree
**Given** the target worktree has uncommitted changes,
**When** `wt teardown <figura> --force` is invoked,
**Then** the CLI MUST proceed with removal, discarding uncommitted changes.
A warning MUST be printed to stderr identifying the figura and number of files discarded.

### REQ-TEARDOWN-4: Teardown of non-existent worktree
**Given** no worktree exists at `talos.wt/agent-<figura>` and git has no record of it,
**When** `wt teardown <figura>` is invoked,
**Then** the CLI MUST return `ErrWorktreeNotFound`.

### REQ-TEARDOWN-5: git worktree prune always runs
**Given** `git worktree remove` completes (successfully or was skipped due to already-missing
directory),
**When** teardown finishes,
**Then** `git worktree prune` MUST be executed to clean up stale git-internal state.

### REQ-TEARDOWN-6: Branch deletion is a design-time decision
Whether `wt teardown` also deletes the local branch `agent/<figura>/<jira-key>` is NOT
prescribed by this spec. Design MUST make this decision explicit (see DC-2).
Observable requirement: if branch deletion is implemented, it MUST only occur AFTER
`git worktree remove` succeeds, and MUST NOT fail the overall teardown if the branch has
already been deleted.

---

## REQ-ERR — Typed errors

### REQ-ERR-1: Error catalogue
**Given** any operation that fails for a known reason,
**When** the CLI returns an error,
**Then** it MUST use a typed error from this catalogue (names are behavioural contracts,
not implementation names):

| Error identifier | Trigger condition |
|---|---|
| `ErrUnknownFigura` | `<figura>` not in the eight-element roster |
| `ErrInvalidJiraKey` | `<jira-key>` does not match `TAL-<n>` |
| `ErrWorktreeExists` | Path `talos.wt/agent-<figura>` already exists |
| `ErrBranchExists` | Branch `agent/<figura>/<jira-key>` already exists locally or in another worktree |
| `ErrWorktreeNotFound` | Operation requires an existing worktree but none is found |
| `ErrDirtyWorktree` | Teardown attempted on worktree with uncommitted/untracked files without `--force` |
| `ErrDevelopNotAvailable` | Neither local `develop` nor `origin/develop` is reachable |

### REQ-ERR-2: No raw git error surfacing
**Given** any git command that fails,
**When** the CLI surfaces the error,
**Then** it MUST wrap the raw git stderr with a human-readable message identifying the
operation, the figura, and the jira key (when applicable).
Raw git error text MAY be included as additional context but MUST NOT be the primary message.

### REQ-ERR-3: No panic
**Given** any error condition including unexpected git output shapes,
**When** the CLI handles it,
**Then** it MUST NOT panic. It MUST exit with a non-zero status code and print to stderr.

### REQ-ERR-4: Exit code contract
**Given** a successful invocation,
**When** the CLI exits,
**Then** exit code MUST be 0.
Any error from the typed catalogue or an unexpected failure MUST produce exit code 1.

---

## REQ-TEST — Testability

### REQ-TEST-1: Pure-function unit tests
**Given** the assign-map (REQ-ASSIGN), .env generation (REQ-ENV), porcelain parser (REQ-LIST-1),
and input validation (REQ-NAMING),
**When** unit tests run,
**Then** each MUST be tested with table-driven test cases that cover: valid inputs, invalid inputs,
and boundary values. These tests MUST have no dependency on the filesystem or any git binary.

### REQ-TEST-2: Service-layer tests via GitRunner mock
**Given** the create, teardown, and env use cases,
**When** unit tests for service-layer logic run,
**Then** they MUST depend only on a `GitRunner` interface (or equivalent port) and MUST NOT
invoke a real git binary. The mock MUST support recording calls and returning programmed outputs.

### REQ-TEST-3: Adapter integration tests skippable with -short
**Given** the git adapter (the concrete GitRunner implementation),
**When** integration tests run,
**Then** tests that require a real git binary and a temporary repository MUST be skipped when
`go test -short` is passed. They MUST NOT be skipped in CI full-run mode.

### REQ-TEST-4: Coverage of error catalogue
**Given** REQ-ERR-1,
**When** unit tests are written,
**Then** there MUST be at least one test case per error identifier verifying that the correct
typed error is returned under the described trigger condition.

### REQ-TEST-5: Assign-map disjoint property test
**Given** REQ-ASSIGN-3,
**When** the assign-map unit tests run,
**Then** there MUST be a test that iterates ALL eight figuras, collects their ports and schemas,
and asserts no duplicates exist. This test acts as a regression guard against accidental
collision if the map is modified.

### REQ-TEST-6: Porcelain parser round-trip test
**Given** REQ-LIST-1,
**When** unit tests run,
**Then** there MUST be table-driven test cases covering: empty output, a single main worktree
only (no agent entries), one agent worktree, multiple agent worktrees, and a detached HEAD entry.

---

## REQ-OWNER — Ownership registration

### REQ-OWNER-1: ownership.md entry
**Given** the change is applied,
**When** `team-context/ownership.md` is read,
**Then** it MUST contain a row mapping `module:devops` → `HERMES` with status `active` and
listing `platform/worktree-orchestrator/**` and `.env.example` in the shared-file map.

---

## Design-Time Decisions (open — MUST be closed by design phase before apply)

The following items affect observable behaviour but are intentionally left open at spec level
to avoid over-constraining the design phase.

| ID | Question | Spec-level constraint |
|----|----------|-----------------------|
| DC-1 | If `.env` write fails after `git worktree add`, should the CLI automatically roll back the worktree? | REQ-CREATE-8 requires reporting partial state; rollback policy is design's call. |
| DC-2 | Should `wt teardown` delete the local branch after removing the worktree? | REQ-TEARDOWN-6 requires any deletion to happen post-remove and not fail if already gone. |
| DC-3 | Should `create` and `env` share a single internal env-generator, or are they separate code paths? | REQ-ENV-3 requires byte-for-byte identical output; sharing a generator is the obvious path but not mandated. |
| DC-4 | Should `--lock` be passed to `git worktree add` to prevent accidental pruning? | Not prescribed. Design must decide and document the default behaviour. |
| DC-5 | Should the port base (8100) be overridable via a flag or env var for test isolation? | REQ-ASSIGN-1 locks the table for Fase 2. A `--port-base` flag for test use is acceptable if it does not change production defaults. |
