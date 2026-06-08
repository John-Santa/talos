# Spec: merge-order-automation — Functional Requirements

**Change:** merge-order-automation
**Jira:** TAL-3
**Module:** module:devops
**Owner:** HERMES
**RFC 2119 keywords apply throughout.**

---

## Overview

This spec describes the observable behaviour that MUST be true after the `merge-order-automation`
change is fully applied. It does NOT prescribe implementation or package structure; that is the
design phase's responsibility.

The system under specification is a Go CLI binary `mo` residing in
`platform/merge-order-orchestrator/`. It consumes the `wt list --json` output produced by the
`wt` CLI (change #1, TAL-2, archived) and automates the §11 merge discipline defined in
`CONSTITUTION.md`: deterministic safe-merge ordering, read-only collision detection, and
one-at-a-time rebase+sequencing that halts at the PR boundary for human approval (HG3, §10).

Three subcommands are in scope: `plan`, `execute`, `check`.
CI integration, human approval (HG3), Jira-link auto-discovery, and auto-merge to `develop`
are explicitly out of scope.

---

## REQ-READINESS — Candidate selection

### REQ-READINESS-1: Source of candidates
**Given** any invocation of `mo` that requires a candidate set,
**When** the CLI populates that set,
**Then** it MUST obtain it by shelling out to `wt list --json` and parsing the resulting JSON
array using a local DTO. Direct coupling to the `wt` module's Go packages is PROHIBITED.
The JSON schema from TAL-2 is the stable seam.

### REQ-READINESS-2: Only `active` entries are candidates
**Given** the parsed `wt list --json` output,
**When** the CLI filters for candidates,
**Then** it MUST include only entries whose `status` field equals `"active"`.
Entries with `status` `"orphan"` or `"stale"` MUST be silently excluded from the candidate set.

### REQ-READINESS-3: Zero-ahead branches are silently dropped
**Given** a candidate with `status == "active"`,
**When** the CLI checks its readiness,
**Then** it MUST verify that the branch has at least one commit ahead of `develop`.
A branch with zero commits ahead of `develop` MUST be silently excluded from the candidate set;
it is NOT an error condition (worktrees without changes are expected to be cleaned up separately,
per `team-context/merge-order.md` §"El worktree sin cambios se limpia solo").

### REQ-READINESS-4: Empty candidate set is not an error
**Given** the candidate set is empty (no `active` entries with commits ahead),
**When** any subcommand that requires candidates runs,
**Then** the CLI MUST print a human-readable "nothing to merge" message and exit with code 0.
It MUST NOT print an error or exit with a non-zero code.

### REQ-READINESS-5: `wt` binary missing → loud typed error
**Given** the `wt` binary is not found on PATH when `mo` attempts to shell out,
**When** the shell-out fails,
**Then** the CLI MUST return `ErrWtBinaryNotFound` with a message that names the missing binary
and advises how to obtain it. It MUST NOT swallow the error or exit silently.

### REQ-READINESS-6: Malformed JSON from `wt list --json` → loud typed error
**Given** `wt list --json` produces output that is not valid JSON or does not match the
expected array schema,
**When** the CLI parses it,
**Then** it MUST return `ErrWtOutputMalformed` with a message that includes the raw output
fragment (truncated if necessary) and the parse error. It MUST NOT silently return an
empty candidate set.

---

## REQ-ORDER — Merge ordering

### REQ-ORDER-1: Priority sequence (§11)
**Given** a non-empty candidate set,
**When** the CLI computes the merge order,
**Then** it MUST apply the following priority sequence exactly, in order:

1. **Dependencies first** — topological sort (see REQ-ORDER-2).
2. **Lower conflict surface first** — fewer changed files (smaller `ChangedFiles` count) ranks
   first; this is a ranking heuristic, NOT an enforcement of the overlap protocol (#3).
3. **FIFO** — `ORDER BY created ASC` using the issue creation timestamp embedded in the branch
   name or provided via flags; fully deterministic given equal inputs.

No other ordering criterion may influence the sequence.

### REQ-ORDER-2: Dependency edges — topological sort
**Given** dependency edges supplied via `--depends A:B` (meaning "B depends on A → A merges
first") or `--depends-file <path>` (one `A:B` pair per line),
**When** the CLI builds the merge order,
**Then** it MUST produce a valid topological sort where all dependencies of a branch appear
earlier in the sequence. If no dependency flags are supplied the CLI MUST order by
criteria (2) and (3) only.

### REQ-ORDER-3: Dependency cycle → fail-loud, no partial plan
**Given** the supplied dependency edges form a cycle (e.g. A→B→A),
**When** the CLI attempts to compute the topological sort,
**Then** it MUST return `ErrDependencyCycle` identifying all branches involved in the cycle
and MUST NOT emit any partial plan. Exit code MUST be 1.

### REQ-ORDER-4: Determinism
**Given** identical candidate sets and dependency edges,
**When** the CLI computes the order on any two separate invocations,
**Then** it MUST produce byte-for-byte identical output. Stochastic or environment-dependent
orderings are PROHIBITED.

---

## REQ-COLLISION — Collision detection

### REQ-COLLISION-1: Read-only simulation
**Given** any collision check,
**When** the CLI simulates mergeability,
**Then** it MUST use a read-only simulation (e.g. `git merge-tree --write-tree --name-only`)
that does NOT mutate the working directory, the index, or any tracked branch ref.
Any tool invocation that creates commits, stages files, or modifies tracked refs is PROHIBITED.

### REQ-COLLISION-2: Per-step conflict reporting
**Given** a computed merge sequence with N steps,
**When** the CLI checks each step,
**Then** it MUST report, for each step, whether it is conflict-free or conflicting, and for
conflicting steps MUST list the conflicting paths.

### REQ-COLLISION-3: Conflict against advanced develop tip (execute)
**Given** `mo execute` has successfully integrated one or more branches,
**When** the CLI evaluates the next step,
**Then** the mergeability simulation for that step MUST be run against the CURRENT `develop` tip
(i.e. the tip after previous steps advanced it), NOT against the tip at plan time.

---

## REQ-HEALTH — Health metric (gate HG6)

### REQ-HEALTH-1: Conflict rate definition
**Given** a computed merge sequence of N steps where C steps are predicted conflicting,
**When** the health metric is computed,
**Then** the conflict rate MUST be defined as `C / N` (a value in [0.0, 1.0]).
Both `plan` and `execute` MUST report this metric.

### REQ-HEALTH-2: Segmentation-bad threshold
**Given** a conflict rate strictly greater than 0.15 (default threshold),
**When** the health metric is evaluated,
**Then** the CLI MUST flag the result as "segmentation bad" and include a recommendation to
re-segment (review `ownership.md`) before adding more agents.
A conflict rate of exactly 0.15 MUST NOT be flagged as bad (strictly-greater-than boundary).

### REQ-HEALTH-3: Configurable threshold
**Given** the `--conflict-threshold` flag is supplied with a value in (0.0, 1.0],
**When** the health metric is evaluated,
**Then** the threshold supplied by the flag MUST override the default 0.15.
Omitting the flag MUST always apply the default.

### REQ-HEALTH-4: `execute` refuses when segmentation bad
**Given** `mo execute --yes` is invoked and the conflict rate (at plan time) exceeds the
threshold,
**When** `execute` evaluates the plan,
**Then** it MUST refuse to proceed, print the conflict rate and threshold, print the
"segmentation bad" advisory, and exit with code 1.
The user MUST re-segment and produce a plan whose rate is at or below the threshold before
`execute` will proceed.

---

## REQ-PLAN — `mo plan` subcommand

### REQ-PLAN-1: Read-only guarantee
**Given** `mo plan` is invoked with any flags,
**When** it executes,
**Then** it MUST NOT mutate the working directory, the index, any branch ref, or any file
outside the CLI's own process. It is always safe to run multiple times.

### REQ-PLAN-2: Output — human-readable default
**Given** `mo plan` is invoked without `--json`,
**When** it prints results,
**Then** it MUST emit a numbered, ordered list of merge steps. Each step MUST include:
branch name, conflict status (clean / conflicting), and conflicting paths if any.
The health metric (conflict rate, good/bad flag) MUST appear at the end of the output.

### REQ-PLAN-3: Output — machine-readable JSON
**Given** `mo plan --json` is invoked,
**When** it prints results,
**Then** it MUST emit a valid JSON object containing at minimum:
- `steps`: array of `{ "branch": string, "conflicting": bool, "conflicting_paths": []string }`
  (empty array when clean)
- `conflict_rate`: number in [0.0, 1.0]
- `segmentation_bad`: bool
- `threshold`: number

The output MUST be valid JSON parseable by standard tools. This format is consumed by ATHENA.

### REQ-PLAN-4: Advisory for branches behind develop
**Given** `mo plan` detects a candidate branch that is behind `develop` (develop has commits
the branch does not have),
**When** it prints the plan,
**Then** it MUST include an advisory for each such branch noting it will require a rebase before
it can be integrated. This advisory is informational and MUST NOT prevent plan output.

---

## REQ-EXECUTE — `mo execute` subcommand

### REQ-EXECUTE-1: `--yes` flag required
**Given** `mo execute` is invoked without the `--yes` flag,
**When** the CLI parses the invocation,
**Then** it MUST print a usage hint explaining that `--yes` is required to confirm intent and
exit with code 1. It MUST NOT execute any merge step.

### REQ-EXECUTE-2: Fetch develop by default
**Given** `mo execute --yes` is invoked without `--no-fetch`,
**When** the CLI begins execution,
**Then** it MUST run `git fetch origin develop` before evaluating any step, ensuring the local
`develop` ref is current before rebase operations.

### REQ-EXECUTE-3: Opt-out of fetch
**Given** `mo execute --yes --no-fetch` is invoked,
**When** the CLI begins execution,
**Then** it MUST skip the fetch step. The caller assumes responsibility for staleness.

### REQ-EXECUTE-4: Per-step re-check against advanced tip
**Given** `mo execute --yes` is processing step N (N > 1),
**When** it evaluates mergeability for step N,
**Then** it MUST re-check mergeability against the CURRENT `develop` tip at that moment in the
execution (not against the tip at plan time), honoring REQ-COLLISION-3.

### REQ-EXECUTE-5: Rebase before PR halt
**Given** step N passes the re-check (no predicted conflict),
**When** the CLI processes that step,
**Then** it MUST rebase the step's branch onto the current `develop` tip before halting.
The rebase MUST be performed in the corresponding worktree directory, not in the main worktree.

### REQ-EXECUTE-6: HALT at the PR boundary — MUST NEVER auto-merge to develop
**Given** a branch has been rebased successfully,
**When** the CLI reaches the PR boundary for that step,
**Then** it MUST halt and print the exact `gh pr create` / `gh pr merge` command that ZEUS
must run to open or merge the PR into `develop`.
The CLI MUST NOT execute that command itself.
The CLI MUST NOT merge the branch into `develop` under any code path, flag, or configuration.
This requirement is the structural enforcement of gate HG3 (§10) and is NON-NEGOTIABLE.

### REQ-EXECUTE-7: Halt on predicted conflict — print mid-task-failure recipe
**Given** `mo execute --yes` detects a predicted conflict on a step (either at initial plan
evaluation or at per-step re-check),
**When** the conflict is detected,
**Then** the CLI MUST halt immediately for that step and print the §11 mid-task-failure recipe:
1. Transition the issue back to "To Do".
2. Comment on the issue with the error details.
3. Discard the worktree via `wt teardown --force <figura>`.

The CLI MUST NOT auto-discard the worktree in the MVP. Printing the recipe is sufficient.
Steps after the conflicting step MUST NOT be processed in the same invocation.

### REQ-EXECUTE-8: Halt on rebase conflict
**Given** `mo execute --yes` encounters a rebase conflict (i.e. the actual rebase fails, not
the prediction),
**When** the rebase conflict is detected,
**Then** the CLI MUST abort the rebase (leaving the repository in its pre-rebase state),
halt, and print the same §11 mid-task-failure recipe as REQ-EXECUTE-7.
The CLI MUST NOT leave the repository in an in-progress rebase state.

### REQ-EXECUTE-9: Single-branch execution is valid
**Given** the candidate set contains exactly one branch,
**When** `mo execute --yes` runs,
**Then** it MUST process that single branch through the full rebase+halt sequence.
A single-branch set is not an error.

---

## REQ-CHECK — `mo check` subcommand

### REQ-CHECK-1: One-off read-only simulation
**Given** `mo check <branch>` is invoked,
**When** the CLI runs,
**Then** it MUST perform a read-only mergeability simulation of `<branch>` against the current
local `develop` tip and print whether the branch is conflict-free or conflicting.
Conflicting paths MUST be listed. No mutation of any git state is permitted.

### REQ-CHECK-2: Fetch develop by default
**Given** `mo check <branch>` is invoked without `--no-fetch`,
**When** the CLI begins,
**Then** it MUST run `git fetch origin develop` first.

### REQ-CHECK-3: Opt-out of fetch
**Given** `mo check <branch> --no-fetch` is invoked,
**When** the CLI begins,
**Then** it MUST skip the fetch step.

### REQ-CHECK-4: Unknown branch → typed error
**Given** `mo check <branch>` is invoked with a branch name that does not exist locally,
**When** the CLI looks up the branch,
**Then** it MUST return `ErrBranchNotFound` and exit with code 1.

---

## REQ-ERR — Typed errors

### REQ-ERR-1: Error catalogue
**Given** any operation that fails for a known reason,
**When** the CLI returns an error,
**Then** it MUST use a typed error from this catalogue (names are behavioural contracts,
not implementation names):

| Error identifier | Trigger condition |
|---|---|
| `ErrWtBinaryNotFound` | `wt` binary not found on PATH when shelling out |
| `ErrWtOutputMalformed` | `wt list --json` output is not valid JSON or does not match expected schema |
| `ErrDependencyCycle` | Supplied dependency edges form a directed cycle |
| `ErrBranchNotFound` | Branch supplied to `mo check` does not exist locally |
| `ErrSegmentationBad` | `execute` refuses because conflict rate > threshold |
| `ErrDevelopNotAvailable` | Neither local `develop` nor `origin/develop` is reachable |
| `ErrRebaseConflict` | Actual rebase conflict encountered during `execute` |

### REQ-ERR-2: No raw git error surfacing
**Given** any git command that fails,
**When** the CLI surfaces the error,
**Then** it MUST wrap the raw git stderr with a human-readable message identifying the
operation and the branch. Raw git error text MAY be included as additional context but
MUST NOT be the primary message.

### REQ-ERR-3: No panic
**Given** any error condition including unexpected git output shapes or malformed dependency
file syntax,
**When** the CLI handles it,
**Then** it MUST NOT panic. It MUST exit with a non-zero status code and print to stderr.

### REQ-ERR-4: Exit code contract
**Given** a successful invocation,
**When** the CLI exits,
**Then** exit code MUST be 0.
Any error from the typed catalogue, a detected conflict halting execute, or an unexpected
failure MUST produce exit code 1.

---

## REQ-FETCH — Develop staleness

### REQ-FETCH-1: Default fetch on all mutating/checking subcommands
**Given** any invocation of `plan`, `execute`, or `check` without `--no-fetch`,
**When** the CLI begins,
**Then** it MUST run `git fetch origin develop` to ensure `origin/develop` is current before
any comparison or rebase operation.

### REQ-FETCH-2: `--no-fetch` escape hatch
**Given** any invocation of `plan`, `execute`, or `check` with `--no-fetch`,
**When** the CLI begins,
**Then** it MUST skip the fetch step. The caller assumes responsibility for staleness.
This flag exists for offline use and test isolation; it MUST NOT be the default.

---

## REQ-TEST — Testability

### REQ-TEST-1: Pure-function unit tests — domain layer
**Given** the candidate filter (REQ-READINESS), ordering logic (REQ-ORDER), health metric
(REQ-HEALTH), and JSON output serialization (REQ-PLAN-3),
**When** unit tests run,
**Then** each MUST be tested with table-driven test cases covering: valid inputs, empty
inputs, and boundary values (e.g. exactly 0.15 vs. 0.16 conflict rate).
These tests MUST have no dependency on the filesystem, any git binary, or the `wt` binary.

### REQ-TEST-2: Topological sort correctness tests
**Given** REQ-ORDER-2 and REQ-ORDER-3,
**When** unit tests run,
**Then** there MUST be table-driven tests covering: no edges (order by heuristic only),
simple chain (A→B→C), diamond dependency, and a cycle (must return `ErrDependencyCycle`
with all cycle members identified).

### REQ-TEST-3: Health metric boundary test
**Given** REQ-HEALTH-2,
**When** unit tests run,
**Then** there MUST be test cases for conflict rates of exactly 0.15 (expect good),
0.150001 (expect bad), 0.0 (all clean), and 1.0 (all conflict).
These cases MUST be present as named table entries, not ad-hoc assertions.

### REQ-TEST-4: Service-layer tests via port mocks
**Given** the plan, execute, and check use cases,
**When** unit tests for service-layer logic run,
**Then** they MUST depend only on interfaces (ports) for git operations and `wt` shell-out,
and MUST NOT invoke a real git binary or the real `wt` binary.
Mocks MUST support recording calls and returning programmed outputs.

### REQ-TEST-5: Adapter integration tests skippable with -short
**Given** the git adapter and `wt` shell-out adapter,
**When** integration tests run,
**Then** tests that require a real git binary or real worktrees MUST be skipped when
`go test -short` is passed. They MUST NOT be skipped in CI full-run mode.

### REQ-TEST-6: Coverage of error catalogue
**Given** REQ-ERR-1,
**When** unit tests are written,
**Then** there MUST be at least one test case per error identifier verifying that the
correct typed error is returned under the described trigger condition.

### REQ-TEST-7: Execute HALT test — no auto-merge
**Given** REQ-EXECUTE-6,
**When** unit tests verify execute behaviour,
**Then** there MUST be a test that confirms `execute` halts and prints the `gh` command
without actually invoking it. The mock for the git adapter MUST record that no merge
command was issued to `develop`.

---

## REQ-CLEANUP — Shared-file corrections

### REQ-CLEANUP-1: Fix `team-context/merge-order.md` — align to `develop`
**Given** the change is applied,
**When** `team-context/merge-order.md` is read,
**Then** it MUST contain zero references to `main` as the integration branch.
All occurrences of `main` as the merge target MUST be replaced with `develop`.
This applies to EVERY line in the file (currently: the header summary and the merge strategy
bullet). The file MUST be consistent with `CONSTITUTION.md §11` which names `develop` as
the integration branch.

### REQ-CLEANUP-2: Ownership entry for new module
**Given** the change is applied,
**When** `team-context/ownership.md` is read,
**Then** it MUST contain a row mapping `module:devops` → `HERMES` with status `active` and
listing `platform/merge-order-orchestrator/**` among its owned paths, mirroring the pattern
of the existing `worktree-orchestrator` row.

### REQ-CLEANUP-3: `.gitignore` entry for compiled binary
**Given** the change is applied,
**When** `.gitignore` is read,
**Then** it MUST contain the entry `/platform/merge-order-orchestrator/mo` so the compiled
binary is never committed.

---

## REQ-OUT-OF-SCOPE — Explicit deferrals

The following items are NOT requirements of this change and MUST NOT be implemented:

| Item | Deferred to |
|---|---|
| CI-green readiness check (`--require-ci-green`) | change #4 (ci-hardening) |
| Same-file overlap enforcement (serialization by file) | change #3 (overlap-protocol) |
| Human approval HG3 — enforced structurally by HALT | never coded; structural guarantee |
| Jira-link auto-discovery of dependency edges | future change |
| `--auto-merge` flag or any path that merges to `develop` | never in scope |

---

## Requirements Traceability

| REQ-ID | Source | Gate / Constitution ref |
|---|---|---|
| REQ-READINESS-1 | Proposal §What changes, decision #1 | — |
| REQ-READINESS-2 | Proposal §Scope (readiness MVP) | — |
| REQ-READINESS-3 | Proposal §Scope; merge-order.md §"limpia solo" | — |
| REQ-READINESS-4 | Spec brief §"Empty candidate set" | — |
| REQ-READINESS-5–6 | Spec brief §"Edge cases" | — |
| REQ-ORDER-1 | CONSTITUTION §11; proposal decision #5 | §11 ordering rule |
| REQ-ORDER-2 | Proposal §Scope (--depends flag) | — |
| REQ-ORDER-3 | Spec brief §"dependency cycle" | — |
| REQ-ORDER-4 | Proposal §Scope (deterministic) | — |
| REQ-COLLISION-1 | Proposal decision #3 (git merge-tree) | — |
| REQ-COLLISION-2–3 | Spec brief §"Collision detection" | — |
| REQ-HEALTH-1–4 | CONSTITUTION §11 / spec brief §"Health metric" | HG6 |
| REQ-PLAN-1–4 | Proposal §Subcommands (plan) | — |
| REQ-EXECUTE-1–9 | Proposal decision #2 (HYBRID posture) | HG3 (§10) |
| REQ-CHECK-1–4 | Proposal §Subcommands (check) | — |
| REQ-ERR-1–4 | Spec brief §"Edge cases as requirements" | — |
| REQ-FETCH-1–2 | Spec brief §"stale local develop" | — |
| REQ-TEST-1–7 | CONSTITUTION; project TDD convention | Strict TDD |
| REQ-CLEANUP-1 | Proposal §Cleanup in-scope; CONSTITUTION §11 | — |
| REQ-CLEANUP-2–3 | Proposal §Cleanup in-scope | — |
| REQ-OUT-OF-SCOPE | Proposal §Out of scope | — |

---

## Design-Time Decisions (open — MUST be closed by design phase before apply)

| ID | Question | Spec-level constraint |
|----|----------|-----------------------|
| DC-1 | Does `execute` abort the entire sequence or skip-and-continue on a per-step conflict? | REQ-EXECUTE-7 requires halting the invocation at the conflicting step; skip-and-continue is NOT compliant. |
| DC-2 | How does `mo` obtain the `ChangedFiles` count for conflict-surface ranking? | Must be read-only (REQ-COLLISION-1); design decides whether to parse `git diff --name-only` or another source. |
| DC-3 | How is FIFO ordering resolved when branch creation timestamps are unavailable? | REQ-ORDER-1 requires full determinism; design must specify a tie-breaking fallback (e.g. lexicographic branch name). |
| DC-4 | Does `--depends-file` support comments or blank lines, and what encoding? | Not prescribed; design decides and documents the format. |
| DC-5 | Should the conflict threshold be readable from a config file in addition to the flag? | REQ-HEALTH-3 only mandates the flag; a config file is acceptable if it does not change the flag-override precedence. |
