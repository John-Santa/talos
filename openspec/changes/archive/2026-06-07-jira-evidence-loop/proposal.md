# Proposal — jira-evidence-loop

> **Change:** `jira-evidence-loop` · **Module:** `module:jira-loop` · **Owner:** HERMES
> **Branch:** `agent/hermes/TAL-<n>` · **Store:** hybrid (openspec + engram)
> **Phase:** propose · **Status:** decided (4 architectural decisions LOCKED by ZEUS)

---

## Why / Intent

**Problem.** The Definition of Done (CONSTITUTION §6 — "linkeá lo vivo, adjuntá lo muerto") currently
depends on the **community MCP server** (`mcp__atlassian__*`) for the two evidence-critical operations:
attachments and remote issue links. That server is a single point of failure on the DoD critical path —
it has documented reliability bugs (e.g. worklog ADF mismatches silently dropped), it carries no
unit-testable boundary, and it cannot be extracted to the Kit. The evidence loop — the very thing that
proves our work is done — runs on infrastructure we neither control nor test.

**Why now.** The repo is in Fase 0 with **zero Go code**. This change is the first Go artifact in the
monorepo. Building the evidence loop as a real, tested Go module accomplishes three things at once:

1. **Dogfood the evidence loop.** The platform proves its own DoD discipline before any product code
   exists. The loop that emits evidence is itself built under strict TDD with full evidence.
2. **First Kit-extractable Go artifact.** A self-contained, config-injected module designed from line one
   to lift cleanly into the Kit in Fase 5 — no hardcoded site, project, or transition names.
3. **Remove community-MCP from the DoD critical path.** Attachments and remote links move to our own
   hand-written client under our own token. The MCP servers remain available for ATHENA's interactive
   JQL reads, but they no longer gate "done."

**Success looks like.** A `go test ./...`-green Go module exposing a callable 7-step evidence loop
(create → transition In Progress → comment → worklog → remote link → attachment → transition Done),
unit-tested against a mocked port with zero live calls, plus a build-tagged integration smoke that
exercises a real TAL issue. The ownership blackboard records `module:jira-loop → HERMES`. HG4 (Fase 1
close) can then validate "loop + evidencia válidos" against a real artifact, not a procedure.

---

## Scope

### In scope

- **New Go module** at `platform/jira-evidence-loop/` implementing the full 7-step evidence loop over
  the Jira REST API v3, with a **hexagonal layout** (domain core + port + adapter, dependency inversion).
- **The port** (`port.JiraClient`) — the mockable boundary: `CreateIssue`, `GetTransitions`,
  `DoTransition`, `AddComment`, `AddWorklog`, `CreateRemoteLink`, `AddAttachment` (+ JQL search for the
  idempotency check).
- **The service** (`service/evidence_loop.go`) — orchestrates the 7 steps, enforces the ownership
  invariant before any write, knows nothing about HTTP. This is the primary unit under test.
- **The adapter** (`adapter/rest/`) — a hand-written `net/http` client implementing the port, with a
  dedicated ADF v3 body builder. All instance config injected via a `Config` struct at construction.
- **Mock + unit tests** — hand-written test double for the port; table-driven tests for the happy path,
  ownership/label validation, transition resolution, idempotency skip, and fail-fast attachment errors.
- **Thin CLI** (`cmd/evidence/`) — a minimal shim that wires `Config` and calls the service. Not heavily
  tested (integration-tagged only); the library API is the real surface.
- **Integration smoke** — `//go:build integration` test that creates, transitions, comments, worklogs,
  and tears down a real TAL issue, gated on `JIRA_EMAIL` + `JIRA_API_TOKEN`.
- **Ownership registry update** — add `module:jira-loop | HERMES | Jira evidence loop | active` to
  `team-context/ownership.md`.

### Out of scope

- **Per-agent Jira token accounts.** Identity stays shared-account-plus-agent-label (CONSTITUTION §5).
  Migrating to per-agent accounts is a future ZEUS decision (No-goal, §13).
- **Worktrees / parallelism / process isolation** beyond `.env`-per-worktree (CONSTITUTION §11, §13).
- **Kit extraction itself** (Fase 5 / HG7). This change *designs for* extraction (config injection, zero
  transitive deps) but does not perform it.
- **Visual Tier-2 tooling, file-level collision detection inside Jira, automatic judge-contradiction
  resolution** — all explicit No-goals (§13).

---

## Approach

A standalone Go module hitting `https://tablex.atlassian.net/rest/api/3/` directly, under **hexagonal /
Clean architecture**: the `service` (use case) depends only on the `port.JiraClient` interface; the REST
`adapter` is plugged in at the edge; the `domain` holds pure value objects (Issue, Label, Transition,
Worklog, RemoteLink) and the ownership invariant. This is dependency inversion — the use case never sees
HTTP, so it is trivially unit-tested with a mock, and the adapter is swappable for Kit extraction.

**Proposed layout** (module path `github.com/John-Santa/talos/platform/jira-evidence-loop`):

```
platform/jira-evidence-loop/
├── go.mod / go.sum / README.md
├── domain/evidence/        # Issue, Label, Transition, Worklog, RemoteLink + ownership invariant (pure, unit-tested)
├── port/jira_client.go     # JiraClient interface — the mockable boundary
├── adapter/rest/           # hand-written net/http client + adf.go (ADF v3 builder) + integration smoke
├── service/evidence_loop.go# orchestrates the 7-step loop; depends only on port.JiraClient (unit under test)
├── mock/jira_client_mock.go# hand-written test double for the port
└── cmd/evidence/main.go    # thin CLI shim (wires Config, calls service)
```

The 7-step loop, the labels emitted on create (`agent:hermes`, `module:jira-loop`,
`change:jira-evidence-loop`, `phase:*`), and the ownership invariant enforced before any write are as
mapped in the exploration. The four architectural decisions below are **LOCKED by ZEUS** — encoded here
as decided, not reopened.

### Decision 1 — API token: Basic auth from `.env` (Fase 1) + CI secret (DECIDED)

Authenticate with **Basic auth** (`Authorization: Basic base64(email:apiToken)`) on every request, with
`JIRA_EMAIL` + `JIRA_API_TOKEN` sourced from the **`.env` per worktree** for Fase 1 local/agent runs, and
from a **GitHub Actions secret** for the integration smoke in CI. **Prerequisite:** ZEUS provisions the
API token before `apply` can run the integration smoke. (This supersedes the exploration's open question
on token location.)

### Decision 2 — Idempotency: pre-create JQL check on the label tuple + globalId upsert (DECIDED)

Jira has no native idempotent create. The contract is **one issue per `(change:*, phase:*, agent:*)`
tuple**, enforced by a **pre-create JQL check** keyed on that label tuple: before `CreateIssue`, query
for a matching issue; if found, return its key and **skip create**. Remote links use a stable **`globalId`**
(`pr=<url>`) for **native upsert** — re-emitting the same link mutates rather than duplicates.

### Decision 3 — Transition matching by `to.statusCategory.key`, disambiguated by ordered list (DECIDED)

Resolve transitions **dynamically** via `GET /issue/{key}/transitions` and match by
**`to.statusCategory.key`** (`new` / `indeterminate` / `done`) — **NOT** by localized name. The TAL site
is Spanish; name strings are locale-dependent and team-managed projects can return stale/renamed names.
Status-category keys are locale-independent and rename-resilient. Because "In Progress" and "In Review"
**both** map to `indeterminate`, disambiguate them via a **configurable ordered state list** carried in
the `Config` struct (the ordered progression of target states), so the loop picks the right transition
deterministically. (This supersedes the exploration's open question; the configurable-name-map fallback
is dropped in favour of category-key + ordered list.)

### Decision 4 — Hand-written `net/http` client, NOT go-jira (DECIDED)

The adapter is a **hand-written `net/http` client** (~200 LOC) using `encoding/json`, giving **full ADF
v3 control** (comment/worklog bodies, attachment multipart with the required `X-Atlassian-Token: no-check`
header) and **zero transitive dependencies** for a clean Kit extraction. **Do NOT use go-jira** — its v1
targets API v2 and would require ADF shims while dragging in transitive deps the Kit must not carry.
(This supersedes the exploration's Approach-A go-jira recommendation.)

**Instance config is injected, never hardcoded.** Site URL, project key/id (`TAL` / `10099`), issue type
names (`Tarea`, etc.), and the ordered state list all live in a `Config` struct passed at construction —
the precondition for Kit extraction in Fase 5.

---

## Affected areas

| Path | Change |
|---|---|
| `platform/jira-evidence-loop/**` | **New** Go module (domain, port, adapter, service, mock, cmd, tests) |
| `team-context/ownership.md` | Add row `module:jira-loop \| HERMES \| Jira evidence loop \| active` |

No changes to `openspec/config.yaml`, `.mcp/routing.md`, or `CONSTITUTION.md`. The MCP servers remain
available for ATHENA's interactive JQL reads; the Go adapter takes over the programmatic emit path.

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| **Spanish-locale transition names** — name-matching breaks on the TAL site and on team-managed renames | Match by `to.statusCategory.key` (`new`/`indeterminate`/`done`), never by name (Decision 3) |
| **`indeterminate` ambiguity** — "In Progress" and "In Review" share the same category key | Disambiguate via the configurable ordered state list in `Config` (Decision 3) |
| **ADF v3 bodies** — comment/worklog `body` must be ADF docs, not plain strings; mismatches are silently dropped | Dedicated `adapter/rest/adf.go` builder, unit-tested against fixtures; hand-written client gives full body control (Decision 4) |
| **Secret coordination at parallel scale** — `.env`-per-worktree + CI secret works for Fase 1, but per-agent token sprawl looms when scaling 2→5+ agents | Out of scope now (No-goal §13); flagged for HG6. Shared account + `agent:*` label is the Fase 0–2 contract (§5) |
| **Idempotency contract gaps** — a partially-failed loop could leave an issue mid-progress and re-run could duplicate | Pre-create JQL check on the `(change, phase, agent)` tuple + `globalId` upsert on remote links (Decision 2); fail-fast on attachment errors (§7 — "el helper falla fuerte") |
| **Token provisioning is a hard prerequisite** — integration smoke and any live run block on ZEUS | Unit suite (mocked port) runs with **no token**; integration smoke is build-tagged and gated on env. `apply` proceeds on units; smoke waits for ZEUS-provisioned token |

---

## Open questions for spec / design

1. **Ordered-state list exact mapping.** What is the precise ordered progression of TAL target states
   (and their `statusCategory.key` grouping) the `Config` must encode to disambiguate `indeterminate`
   transitions? Needs the real TAL board columns — resolve in spec/design against the live project.
2. **ADF builder scope.** How much of ADF v3 must `adf.go` support for Fase 1 — paragraphs + text + links
   only, or also code blocks / mentions / panels for richer evidence comments? Define the minimal ADF
   surface in spec.
3. **CLI surface vs. library API.** What exactly does `cmd/evidence` expose (one `run-loop` command, or
   per-step subcommands), and where is the boundary between the thin CLI and the callable library API
   that other phases import? Pin the API contract in design.
