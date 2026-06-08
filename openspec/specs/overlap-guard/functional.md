# Spec: overlap-protocol — Functional Requirements

**Change:** overlap-protocol
**Jira:** TAL-4
**Module:** module:qa
**Owner:** THEMIS
**RFC 2119 keywords apply throughout.**

---

## Overview

This spec describes the observable behaviour that MUST be true after the `overlap-protocol`
change is fully applied. It does NOT prescribe implementation or package structure; that is the
design phase's responsibility.

The system under specification is a Go CLI binary `ov` residing in
`platform/overlap-guard/`. It automates the overlap discipline defined in
`team-context/overlap-protocol.md`: detecting before assignment whether another agent touches
the same module (T0, `ov check`), scanning active worktrees for file-level collisions between
agents (T1, `ov scan`), and reporting the collision rate that gate HG6 needs (`ov metric`).

`ov` is read-only and read-detect only: it does NOT mutate Jira, git, or any file. It does NOT
compute merge order (that is `mo`'s domain, TAL-3, archived). It does NOT auto-serialize,
re-segment, or escalate — it reports; ATHENA/ZEUS acts.

Three subcommands are in scope: `check`, `scan`, `metric`.

---

## REQ-READINESS — Worktree source (shared by `scan` and `metric`)

### REQ-READINESS-1: Source of worktree inventory
**Given** any invocation of `ov scan` or `ov metric` that requires a worktree inventory,
**When** the CLI populates that inventory,
**Then** it MUST obtain it by shelling out to `wt list --json` and parsing the resulting JSON
array using a local DTO. Direct coupling to the `wt` module's Go packages is PROHIBITED.
The JSON schema from TAL-2 is the stable seam.

### REQ-READINESS-2: Only `active` worktrees are candidates
**Given** the parsed `wt list --json` output,
**When** the CLI filters for scan candidates,
**Then** it MUST include only entries whose `status` field equals `"active"`.
Entries with any other status MUST be silently excluded from the candidate set.

### REQ-READINESS-3: Empty active set is not an error
**Given** the active worktree set is empty after filtering,
**When** `ov scan` or `ov metric` runs,
**Then** the CLI MUST exit with code 0, produce output with an empty `file_collisions[]` and
`module_overlaps[]`, and a `collision_rate` of 0.
It MUST NOT print an error or exit with a non-zero code.
This mirrors the `ErrNoCandidates`→0 behaviour of `mo`.

### REQ-READINESS-4: `wt` binary missing → loud typed error
**Given** the `wt` binary is not found on PATH when `ov` attempts to shell out,
**When** the shell-out fails,
**Then** the CLI MUST return `ErrWtBinaryNotFound` with a message that names the missing binary
and advises how to obtain it. It MUST NOT swallow the error or exit silently.

### REQ-READINESS-5: Malformed JSON from `wt list --json` → loud typed error
**Given** `wt list --json` produces output that is not valid JSON or does not match the
expected array schema,
**When** the CLI parses it,
**Then** it MUST return `ErrWtOutputMalformed` with a message that includes the raw output
fragment (truncated if necessary) and the parse error. It MUST NOT silently return an empty
candidate set.

---

## REQ-CHECK — `ov check` subcommand (T0, pre-assignment gate)

### REQ-CHECK-1: JQL verbatim
**Given** `ov check --module X --agent owner` is invoked,
**When** the CLI queries Jira,
**Then** it MUST issue exactly the following JQL, substituting `X` and `owner` literally:

```
project = TAL AND statusCategory != Done AND labels = "module:X" AND labels NOT IN ("agent:owner")
```

No additional filters, orderings, or field projections beyond those needed for the response
body are permitted. The query is case-sensitive as written.

### REQ-CHECK-2: File-level cross check
**Given** the list of issues returned by REQ-CHECK-1,
**When** the CLI performs the overlap check,
**Then** it MUST parse the `files:` checklist from each issue body (per REQ-CHECKLIST),
compute the intersection with the files declared by `--agent owner` (supplied via
`--files-file`), and classify the result as either a file collision (intersection non-empty,
agent is different) or disjoint (intersection empty).

### REQ-CHECK-3: BLOCK on same-file collision (exit 1)
**Given** at least one file appears in both the owner's declared files and a different agent's
declared files,
**When** the CLI evaluates the result,
**Then** it MUST emit verdict `BLOCK`, record the colliding paths in `file_collisions[]`, and
exit with code **1** (`ErrSameFileParallel`).
BLOCK MUST propagate as an error even when `--json` output is also printed.

### REQ-CHECK-4: OK or SERIALIZE on disjoint (exit 0)
**Given** no file-level collision is found,
**When** the CLI evaluates the result,
**Then** it MUST exit with code **0**.
If `module_overlaps[]` is non-empty (another agent has an active issue in the same module),
the verdict MUST be `SERIALIZE`; otherwise `OK`.

### REQ-CHECK-5: Read-only guarantee
**Given** `ov check` is invoked with any flags,
**When** it executes,
**Then** it MUST NOT create, transition, comment on, or otherwise mutate any Jira issue or
any git state. It is always safe to run multiple times.

### REQ-CHECK-6: Fetch is not applicable
**Given** `ov check` operates on Jira data, not git branches,
**When** it executes,
**Then** it MUST NOT invoke `git fetch` or any git command. No `--no-fetch` flag is needed.

---

## REQ-SCAN — `ov scan` subcommand (T1, active-branch cross-agent scan)

### REQ-SCAN-1: Changed files per branch
**Given** a non-empty active worktree set (REQ-READINESS-2),
**When** the CLI computes the scan,
**Then** for each candidate worktree it MUST compute `ChangedFiles` as the set of file paths
produced by `git diff --name-only <base>...<branch>` where `<base>` is the base tip resolved
at invocation time. The base defaults to `develop` and MAY be overridden via `--base`.

### REQ-SCAN-2: Cross-agent pairwise collision
**Given** the `ChangedFiles` sets for all active worktrees,
**When** the CLI evaluates collisions,
**Then** it MUST perform a pairwise comparison across all (worktreeA, worktreeB) pairs where
the owning agents are **different**. A same-agent pair (same `agent:*` label on both
worktrees) MUST NOT be classified as a file collision regardless of file overlap.

### REQ-SCAN-3: BLOCK on file collision (exit 1)
**Given** at least one file appears in the `ChangedFiles` of two worktrees owned by different
agents,
**When** the CLI evaluates the result,
**Then** it MUST emit verdict `BLOCK`, populate `file_collisions[]` with all colliding paths
(each entry including both agent identifiers and the file path), and exit with code **1**
(`ErrSameFileParallel`).

### REQ-SCAN-4: SERIALIZE on module overlap without file collision (exit 0)
**Given** no file-level collision is found but two worktrees from different agents share the
same `module:*` label,
**When** the CLI evaluates the result,
**Then** it MUST emit verdict `SERIALIZE`, populate `module_overlaps[]`, and exit with code 0.
`ov scan` MUST NOT compute merge order — it defers that to `mo`.

### REQ-SCAN-5: OK on no overlap (exit 0)
**Given** no file collisions and no module overlaps exist,
**When** the CLI evaluates the result,
**Then** it MUST emit verdict `OK` and exit with code 0.

### REQ-SCAN-6: Fetch opt-in
**Given** `ov scan` is invoked without `--no-fetch`,
**When** the CLI begins,
**Then** it MUST run `git fetch origin <base>` before resolving base tips, ensuring refs are
current.

### REQ-SCAN-7: Fetch opt-out
**Given** `ov scan --no-fetch` is invoked,
**When** the CLI begins,
**Then** it MUST skip the fetch step. The caller assumes responsibility for staleness.

### REQ-SCAN-8: Read-only guarantee
**Given** `ov scan` is invoked with any flags,
**When** it executes,
**Then** it MUST NOT mutate the working directory, any branch ref, or any tracked file. It is
always safe to run multiple times.

---

## REQ-METRIC — `ov metric` subcommand (HG6 collision rate)

### REQ-METRIC-1: Collision rate definition
**Given** a scan over P pairs of distinct-agent worktrees of which C pairs share at least one
file,
**When** the metric is computed,
**Then** `CollisionRate` MUST be defined as `C / P` (a value in [0.0, 1.0]).
`CollisionRate` MUST be 0 when P = 0 (no pairs to evaluate; no division by zero).
`PairsEvaluated` MUST equal P; `CollidingPairs` MUST equal C.

### REQ-METRIC-2: Over-threshold boundary (HG6)
**Given** a `CollisionRate` strictly greater than the threshold (default 0.15),
**When** the metric is evaluated,
**Then** `over_threshold` MUST be `true` and the output MUST include a recommendation to
re-segment before adding more agents.
A `CollisionRate` of exactly 0.15 MUST NOT set `over_threshold` to `true`
(strictly-greater-than boundary, consistent with REQ-HEALTH-2 of `mo`).

### REQ-METRIC-3: Configurable threshold
**Given** `--threshold T` is supplied with a value in (0.0, 1.0],
**When** the metric is evaluated,
**Then** the supplied value MUST override the default 0.15. Omitting the flag MUST always
apply the default.

### REQ-METRIC-4: Informative by default (exit 0)
**Given** `ov metric` is invoked without `--strict`,
**When** it completes,
**Then** it MUST exit with code 0 regardless of whether `over_threshold` is true or false.
Gate HG6 is a human gate; `ov metric` informs, it does not decide alone.

### REQ-METRIC-5: Strict mode (exit 1 when over threshold)
**Given** `ov metric --strict` is invoked and `over_threshold` is true,
**When** it completes,
**Then** it MUST exit with code 1. When `over_threshold` is false, it MUST exit with code 0
even in `--strict` mode.

### REQ-METRIC-6: Scan flags forwarded
**Given** `ov metric` is invoked with scan flags (`--base`, `--wt-bin`, `--no-fetch`,
`--ownership-file`),
**When** it performs the underlying scan,
**Then** all scan flags MUST be forwarded to the scan logic. `ov metric` MUST NOT run an
independent worktree discovery; it reuses the scan T1 logic.

---

## REQ-VERDICT — Verdict domain model

### REQ-VERDICT-1: Verdict precedence
**Given** the results of a check or scan,
**When** the CLI assigns the top-level verdict,
**Then** it MUST apply the following precedence, in order:
1. If `len(file_collisions) > 0` → verdict is `BLOCK`.
2. Else if `len(module_overlaps) > 0` → verdict is `SERIALIZE`.
3. Else → verdict is `OK`.

No other criterion may influence the verdict. `BLOCK` always wins over `SERIALIZE`; `SERIALIZE`
always wins over `OK`.

### REQ-VERDICT-2: Same-agent files never collide
**Given** two files touched by the same agent (same `agent:*` label on both worktrees or both
issue declarations),
**When** the CLI evaluates file collisions,
**Then** that pair MUST NOT be counted as a `FileCollision`. A single agent working across
multiple worktrees on the same file is not a collision by definition.

### REQ-VERDICT-3: Deterministic output
**Given** identical inputs on any two separate invocations,
**When** the CLI produces output,
**Then** `file_collisions[]` and `module_overlaps[]` MUST be sorted in a stable, deterministic
order (e.g. lexicographic by file path then by agent pair). Stochastic or environment-dependent
orderings are PROHIBITED.

### REQ-VERDICT-4: Cross-change module overlap → SERIALIZE, not BLOCK
**Given** two active worktrees from different agents share the same `module:*` but do NOT share
any changed file,
**When** the CLI evaluates the result,
**Then** the verdict MUST be `SERIALIZE`, NOT `BLOCK`. `ov` MUST NOT compute merge order for
cross-change serialization — it defers to `mo`.

---

## REQ-CHECKLIST — `files:` checklist grammar and fallback

### REQ-CHECKLIST-1: Section start
**Given** an issue body (plain text, ADF-flattened per REQ-JIRA-2),
**When** the parser scans for the `files:` section,
**Then** a line matching `^\s*files:\s*$` (case-insensitive) MUST start the checklist section.
All lines before this marker are ignored for checklist purposes.

### REQ-CHECKLIST-2: Item recognition
**Given** a line within the `files:` section,
**When** the parser evaluates it,
**Then** a line matching `^\s*[-*]\s*\[[ xX]\]\s+(.+)$` MUST be recognized as a checklist
item. The captured group is the file path, with any surrounding backticks stripped. Both
checked (`[x]`, `[X]`) and unchecked (`[ ]`) items MUST be included — check state is
irrelevant for overlap detection.

### REQ-CHECKLIST-3: Section end
**Given** a line within the `files:` section that does not match the item pattern and is not
blank,
**When** the parser encounters it,
**Then** that line MUST terminate the `files:` section. Lines after that point MUST NOT be
parsed as checklist items unless a new `files:` header restarts the section.

### REQ-CHECKLIST-4: Fallback — missing checklist → advisory, not silence
**Given** an issue body that contains no `files:` section (the marker is absent or the section
yields zero items after parsing),
**When** the CLI processes that issue,
**Then** it MUST emit `ErrChecklistMissing` as an advisory entry in `advisories[]`.
The issue MUST be excluded from file-level collision checks (it contributes no paths to
`file_collisions[]`).
The issue MUST NOT be excluded from module-level overlap checks — it MUST still contribute to
`module_overlaps[]` if its `module:*` label matches.
**"No checklist" MUST NEVER be treated as "no overlap".**

### REQ-CHECKLIST-5: Backtick stripping
**Given** a checklist item path enclosed in backticks (e.g. `` `platform/foo/bar.go` ``),
**When** the parser extracts the path,
**Then** it MUST strip the leading and trailing backtick characters. The resulting path MUST
be the bare file path without backtick delimiters.

---

## REQ-JIRA — Jira adapter (T0 only)

### REQ-JIRA-1: Search scope
**Given** `ov check` runs the JQL from REQ-CHECK-1,
**When** the Jira REST call is made,
**Then** the response MUST include at minimum: issue key, labels, and `description` (body
field). The adapter MUST request `description` explicitly — the `jira-evidence-loop` client
does not request it and MUST NOT be imported.

### REQ-JIRA-2: ADF body flattening
**Given** the `description` field returned by Jira REST contains an ADF (Atlassian Document
Format) JSON structure,
**When** the adapter processes the response,
**Then** it MUST flatten the ADF to plain text before passing the body to the checklist parser.
The flattening MUST preserve line structure sufficient for the `files:` section to be
detectable (REQ-CHECKLIST-1 through REQ-CHECKLIST-3).

### REQ-JIRA-3: Auth via environment
**Given** the Jira REST adapter requires credentials,
**When** it initialises,
**Then** it MUST read the site URL and credentials from environment variables or CLI flags
(`--site-url`, `--token`/env equivalent). Credentials MUST NOT be hard-coded or embedded in
compiled output.

### REQ-JIRA-4: Read-only — no mutation
**Given** any invocation of `ov check`,
**When** the Jira adapter executes,
**Then** it MUST issue only HTTP GET / Search (POST to search endpoint is acceptable if
read-only). It MUST NOT call any create, update, transition, or comment endpoint.

---

## REQ-OUTPUT — Output format

### REQ-OUTPUT-1: JSON schema (`--json`)
**Given** any `ov` subcommand is invoked with `--json`,
**When** it prints results,
**Then** it MUST emit a valid JSON object (to stdout) containing at minimum the following
fields in snake_case:

| Field | Type | Present in |
|---|---|---|
| `verdict` | `"OK"` \| `"SERIALIZE"` \| `"BLOCK"` | all subcommands |
| `collision_rate` | number [0,1] | `scan`, `metric` |
| `threshold` | number | `metric` |
| `over_threshold` | bool | `metric` |
| `pairs_evaluated` | integer | `scan`, `metric` |
| `colliding_pairs` | integer | `scan`, `metric` |
| `file_collisions` | array of objects | all subcommands |
| `module_overlaps` | array of objects | all subcommands |
| `advisories` | array of strings | all subcommands |

`file_collisions` entries MUST contain at minimum: `file` (string), `agents` (array of two
agent identifiers).
`module_overlaps` entries MUST contain at minimum: `module` (string), `agents` (array of
agent identifiers).
Absent fields for inapplicable subcommands MUST be omitted, not set to null.

### REQ-OUTPUT-2: Human-readable default (no `--json`)
**Given** any `ov` subcommand is invoked without `--json`,
**When** it prints results,
**Then** it MUST emit a human-readable summary to stdout that includes: the verdict, a list of
file collisions (if any), module overlaps (if any), and advisories (if any). The format is not
byte-for-byte prescribed but MUST be sufficient for a human operator to act without reading
`--json` output.

### REQ-OUTPUT-3: Errors to stderr
**Given** any error condition (typed or untyped),
**When** the CLI surfaces the error,
**Then** the error message MUST be written to stderr. Normal output (report, JSON) MUST be
written to stdout. Mixing output and errors on the same stream is PROHIBITED.

### REQ-OUTPUT-4: BLOCK propagates as error regardless of `--json`
**Given** verdict is `BLOCK`,
**When** the CLI exits,
**Then** it MUST exit with code 1 and the error `ErrSameFileParallel` MUST be reflected in
the error stream (stderr). Printing the JSON report to stdout in addition is permitted, but
MUST NOT suppress the non-zero exit or the stderr error message.

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
| `ErrSameFileParallel` | File collision detected between two distinct agents — verdict BLOCK |
| `ErrChecklistMissing` | Issue body has no parseable `files:` section; emitted as advisory |
| `ErrEscalateZeus` | Overlap condition requiring human judgement beyond ATHENA's authority |
| `ErrNoClaims` | `ov check --files-file` provided a file with zero parseable paths |

### REQ-ERR-2: No raw git or HTTP error surfacing
**Given** any git command or HTTP call that fails,
**When** the CLI surfaces the error,
**Then** it MUST wrap the raw error with a human-readable message identifying the operation
and context. Raw git stderr or HTTP response bodies MAY be included as additional context
but MUST NOT be the primary message.

### REQ-ERR-3: No panic
**Given** any error condition including unexpected git output, malformed ADF, or missing
environment variables,
**When** the CLI handles it,
**Then** it MUST NOT panic. It MUST exit with a non-zero status code and print to stderr.

### REQ-ERR-4: Exit code contract
**Given** any invocation,
**When** the CLI exits,
**Then**:
- Exit 0: verdict OK, SERIALIZE, or vacío (empty active set).
- Exit 1: verdict BLOCK (`ErrSameFileParallel`), any typed error from the catalogue, or
  `ov metric --strict` with `over_threshold == true`.
- No other exit codes are defined.

---

## REQ-FETCH — Base branch staleness (scan and metric)

### REQ-FETCH-1: Default fetch for scan/metric
**Given** `ov scan` or `ov metric` is invoked without `--no-fetch`,
**When** the CLI begins,
**Then** it MUST run `git fetch origin <base>` before resolving base tips.

### REQ-FETCH-2: `--no-fetch` escape hatch
**Given** `ov scan` or `ov metric` is invoked with `--no-fetch`,
**When** the CLI begins,
**Then** it MUST skip the fetch step. This flag exists for offline use and test isolation;
it MUST NOT be the default.

---

## REQ-TEST — Testability

### REQ-TEST-1: Pure-function unit tests — domain layer
**Given** the verdict logic (REQ-VERDICT), collision rate (REQ-METRIC-1), threshold boundary
(REQ-METRIC-2), and checklist parser (REQ-CHECKLIST),
**When** unit tests run,
**Then** each MUST be tested with table-driven test cases covering: valid inputs, empty inputs,
and boundary values (exactly 0.15 vs. 0.16 collision rate; same-agent vs. different-agent pairs;
checked vs. unchecked items; backtick-wrapped paths).
These tests MUST have no dependency on the filesystem, any git binary, the `wt` binary, or any
HTTP client.

### REQ-TEST-2: Checklist parser boundary tests
**Given** REQ-CHECKLIST,
**When** unit tests run,
**Then** there MUST be named table entries covering:
- Section starts (marker present, absent, case-insensitive variants).
- Item recognition (unchecked `[ ]`, checked `[x]`, `[X]`, bare path, backtick path).
- Section termination (non-item line ends section; subsequent items after non-item are ignored).
- Fallback: body with no `files:` section → `ErrChecklistMissing` advisory, zero paths.
- Fallback: body with `files:` section but zero valid items → `ErrChecklistMissing` advisory.

### REQ-TEST-3: Verdict precedence tests
**Given** REQ-VERDICT-1,
**When** unit tests run,
**Then** there MUST be test cases for:
- Only file collisions → BLOCK.
- Only module overlaps (no file collisions) → SERIALIZE.
- Neither → OK.
- Both file collisions and module overlaps → BLOCK (BLOCK wins).

### REQ-TEST-4: Collision rate boundary tests
**Given** REQ-METRIC-2,
**When** unit tests run,
**Then** there MUST be named table entries for: rate = 0 (P=0), rate = 0.0 (P>0, C=0),
rate = 0.15 (expect `over_threshold=false`), rate = 0.150001 (expect `over_threshold=true`),
rate = 1.0 (all pairs collide).

### REQ-TEST-5: Service-layer tests via port mocks
**Given** the check, scan, and metric use cases,
**When** unit tests for service-layer logic run,
**Then** they MUST depend only on interfaces (ports) for git operations, `wt` shell-out, and
Jira REST; they MUST NOT invoke a real git binary, real `wt` binary, or real Jira API.
Mocks MUST be hand-written (no mock generation frameworks), support recording calls, and
return programmed outputs.

### REQ-TEST-6: Adapter integration tests skippable with `-short`
**Given** the git adapter, `wt` shell-out adapter, and `jirarest` adapter,
**When** integration tests run,
**Then** tests that require a real git binary, real worktrees, or a real Jira tenant MUST be
skipped when `go test -short` is passed. They MUST NOT be skipped in CI full-run mode.

### REQ-TEST-7: Coverage of error catalogue
**Given** REQ-ERR-1,
**When** unit tests are written,
**Then** there MUST be at least one test case per error identifier (excluding `ErrEscalateZeus`,
which is an advisory escalation signal) verifying that the correct typed error is returned
under the described trigger condition.

### REQ-TEST-8: Same-agent non-collision test
**Given** REQ-VERDICT-2,
**When** unit tests run,
**Then** there MUST be at least one test case where two worktrees belonging to the same agent
share a file, and the expected result is verdict OK (no FileCollision created).

### REQ-TEST-9: Read-only guarantee tests
**Given** REQ-CHECK-5 and REQ-SCAN-8,
**When** unit tests verify service-layer behaviour,
**Then** the mocks for the git adapter and the Jira adapter MUST record all calls issued, and
tests MUST assert that no mutating calls (write, transition, merge, rebase, commit) were issued
in any code path.

---

## REQ-CLEANUP — Shared-file additions

### REQ-CLEANUP-1: Ownership entry for new module
**Given** the change is applied,
**When** `team-context/ownership.md` is read,
**Then** it MUST contain a row mapping `module:qa` → `THEMIS` with status `active` and listing
`platform/overlap-guard/**` among its owned paths in the "Mapa archivo → módulo" table,
mirroring the pattern of the `worktree-orchestrator` and `merge-order-orchestrator` rows.

### REQ-CLEANUP-2: `.gitignore` entry for compiled binary
**Given** the change is applied,
**When** `.gitignore` is read,
**Then** it MUST contain the entry `/platform/overlap-guard/ov` so the compiled binary is
never committed.

---

## REQ-OUT-OF-SCOPE — Explicit deferrals

The following items are NOT requirements of this change and MUST NOT be implemented:

| Item | Deferred to |
|---|---|
| Merge order computation or ranking | `mo` (TAL-3, archived) |
| Auto-serialization, rebase, or worktree teardown | ATHENA / human action |
| Jira issue transition, comment, or creation | never in scope for `ov` |
| Git write operations of any kind | never in scope for `ov` |
| Auto-fix or codebase re-segmentation | never in scope for `ov` |
| CI-green readiness | change #4 (ci-hardening) |
| Cross-change merge ordering | `mo` via `merge-order.md` |

---

## Requirements Traceability

| REQ-ID | Source | Gate / ref |
|---|---|---|
| REQ-READINESS-1 | Proposal decision #3 (shell-out seam) | TAL-2 seam |
| REQ-READINESS-2–3 | Proposal §Scope (scan T1); parity with `mo` `ErrNoCandidates`→0 | — |
| REQ-READINESS-4–5 | Proposal §Scope edge cases | — |
| REQ-CHECK-1 | `overlap-protocol.md` §Detección; proposal §What changes | — |
| REQ-CHECK-2 | `ownership.md §3` (files: checklist cross-check) | — |
| REQ-CHECK-3–4 | Proposal decision #5 (BLOCK > SERIALIZE > OK) | — |
| REQ-CHECK-5–6 | Proposal §Out of scope (read-only) | — |
| REQ-SCAN-1 | Proposal §What changes (`git diff --name-only base...branch`) | — |
| REQ-SCAN-2 | Proposal decision #5; proposal §Domain model (same-agent no collision) | — |
| REQ-SCAN-3–5 | Proposal decision #5 | — |
| REQ-SCAN-6–7 | Proposal §Subcommands (`--no-fetch`) | — |
| REQ-SCAN-8 | Proposal §Out of scope (read-only) | — |
| REQ-METRIC-1 | Proposal §Domain model (`CollisionRate = CollidingPairs / PairsEvaluated`) | HG6 |
| REQ-METRIC-2 | Proposal decision #6 (strictly > 0.15); `overlap-protocol.md §"Cuándo NO paralelizar"` | HG6 |
| REQ-METRIC-3 | Proposal §Subcommands (`--threshold`) | — |
| REQ-METRIC-4 | Proposal decision #7 (informative by default) | HG6 human gate |
| REQ-METRIC-5 | Proposal §Subcommands (`--strict`) | — |
| REQ-METRIC-6 | Proposal §Subcommands (scan flags forwarded) | — |
| REQ-VERDICT-1 | Proposal decision #5 (BLOCK > SERIALIZE > OK) | — |
| REQ-VERDICT-2 | Proposal §Domain model (same-agent no collision) | — |
| REQ-VERDICT-3 | Proposal §Domain model (deterministic output) | — |
| REQ-VERDICT-4 | Proposal §Complementariedad; §Out of scope | — |
| REQ-CHECKLIST-1–5 | Proposal §Risks #1 (open, resolved here); `ownership.md §3` | — |
| REQ-JIRA-1 | Proposal §What changes (jirarest with description) | — |
| REQ-JIRA-2 | Proposal §Risks #3 (ADF flattening, partially resolved here) | — |
| REQ-JIRA-3–4 | Proposal §Out of scope (read-only); security baseline | — |
| REQ-OUTPUT-1 | Proposal §Subcommands (`--json` snake_case schema) | — |
| REQ-OUTPUT-2–4 | Proposal §Subcommands; BLOCK exit 1 invariant | — |
| REQ-ERR-1 | Proposal §Domain model (typed errors) | — |
| REQ-ERR-2–4 | Project convention; parity with `mo` error discipline | — |
| REQ-FETCH-1–2 | Proposal §Subcommands (`--no-fetch`) | — |
| REQ-TEST-1–9 | CONSTITUTION; project Strict TDD convention | Strict TDD |
| REQ-CLEANUP-1 | Proposal §Edits compartidos gated | — |
| REQ-CLEANUP-2 | Proposal §Edits compartidos gated | — |
| REQ-OUT-OF-SCOPE | Proposal §Out of scope; §Complementariedad | — |

---

## Design-Time Decisions (open — MUST be closed by design phase before apply)

| ID | Question | Spec-level constraint |
|----|----------|-----------------------|
| DC-1 | How does `ov scan` resolve the owning agent from a worktree entry when the worktree JSON does not carry an `agent:*` label directly? | REQ-SCAN-2 requires agent identity; design decides whether agent is derived from branch name pattern, `--ownership-file`, or both. REQ-VERDICT-2 (same-agent no-collision) depends on this. |
| DC-2 | What is the exact ADF flattening strategy (depth-first text extraction vs. paragraph join)? | REQ-JIRA-2 requires line-structure preservation sufficient for `files:` detection; design specifies the algorithm. |
| DC-3 | Does `ov check --files-file` accept a plain list (one path per line) or a checklist-formatted file? | REQ-CHECK-2 presupposes a file exists; design decides the format and documents it. |
| DC-4 | Should `ov metric` accept a pre-computed scan result (e.g. from stdin/`--scan-file`) or must it always run a live scan? | REQ-METRIC-6 requires scan flags be forwarded; design may also allow a cached scan result for CI pipelines without changing the spec boundary. |
| DC-5 | When `--ownership-file` is absent in `ov scan`, can module-level overlaps (`module_overlaps[]`) still be populated, or are they only available when ownership data is provided? | REQ-SCAN-4 requires SERIALIZE on module overlap; design decides if module resolution is mandatory or enrichment-only. |
