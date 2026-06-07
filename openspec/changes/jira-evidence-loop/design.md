# Design: jira-evidence-loop

**Change:** jira-evidence-loop · **Module:** module:jira-loop · **Owner:** HERMES
**Store:** hybrid (this file + engram `sdd/jira-evidence-loop/design`)
**Reads:** proposal (#1814, 4 locked decisions) · exploration (#1813, REST call map)
**Module path:** `github.com/John-Santa/talos/platform/jira-evidence-loop`

This is the HOW at the architectural level. It does not enumerate task steps — `sdd-tasks`
turns these boundaries into a TDD-ordered backlog.

---

## 1. Architecture approach

Hexagonal (ports & adapters) with strict dependency inversion. The dependency arrow points
**inward**: `cmd → service → port ← adapter`, and everything depends on `domain`. The domain
core has zero I/O and zero third-party imports; the adapter is the only package that knows
HTTP exists; the service knows only the port interface. This is the precondition for two
non-negotiables: (a) strict TDD — the service is unit-tested against a mocked port with zero
live calls, and (b) Kit extraction in Fase 5 — the module is self-contained with all
instance config injected, no hardcoded site/IDs anywhere.

```
            ┌─────────────────────────────────────────────────┐
            │                    domain/evidence              │  pure, no deps
            │  Issue · Label · Transition · Worklog ·         │  value objects +
            │  RemoteLink · Attachment + ownership invariant  │  validation only
            └─────────────────────────────────────────────────┘
                       ▲                         ▲
          imports      │                         │      imports
            ┌──────────┴──────────┐   ┌──────────┴───────────┐
            │  service/           │   │  adapter/rest/       │
            │  evidence_loop      │──▶│  Client (net/http)   │
            │  (orchestrates loop)│   │  adf.go · transitions│
            │  depends ONLY on    │   │  IMPLEMENTS the port │
            │  port.JiraClient    │   └──────────────────────┘
            └─────────┬───────────┘              ▲
                      │ depends on               │ satisfies
                      ▼                           │
            ┌─────────────────────────────────────────────────┐
            │            port/JiraClient (interface)          │  the mockable boundary
            └─────────────────────────────────────────────────┘
                      ▲                           ▲
              mock/  ─┘ (test double)    cmd/evidence ─┘ (wires real adapter)
```

**Boundary rule:** `service` imports `port` + `domain`, never `adapter/rest`. `adapter/rest`
imports `port` + `domain` (to satisfy the interface), never `service`. `cmd/evidence` is the
only place where the concrete `rest.Client` is constructed and handed to the service — this
is the composition root.

---

## 2. Package layout

```
platform/jira-evidence-loop/
├── go.mod                        # module github.com/John-Santa/talos/platform/jira-evidence-loop
├── go.sum                        # empty/minimal — ZERO transitive deps (Kit goal)
├── README.md
├── domain/
│   └── evidence/
│       ├── issue.go              # Issue, Label, LabelSet, Transition, Worklog,
│       │                         #   RemoteLink, Attachment value objects
│       ├── ownership.go          # ownership invariant validation (pure)
│       ├── issue_test.go
│       └── ownership_test.go
├── port/
│   └── jira_client.go            # JiraClient interface — the 7 methods (+ Search)
├── service/
│   ├── config.go                 # Config struct (instance config, injected)
│   ├── evidence_loop.go          # Loop use case; depends only on port.JiraClient
│   └── evidence_loop_test.go     # unit, table-driven, mocked port — ZERO live calls
├── adapter/
│   └── rest/
│       ├── client.go             # net/http Client implementing port.JiraClient
│       ├── auth.go               # Basic auth header builder
│       ├── adf.go                # ADF v3 document builder (interface + impl)
│       ├── transitions.go        # dynamic transition resolution by statusCategory.key
│       ├── adf_test.go           # pure unit — ADF JSON shape vs fixtures
│       ├── transitions_test.go   # pure unit — resolution algorithm
│       └── client_integration_test.go  # //go:build integration — env-gated smoke
├── mock/
│   └── jira_client_mock.go       # hand-written test double for port.JiraClient
└── cmd/
    └── evidence/
        └── main.go               # thin CLI: parses flags, builds Config, wires adapter
```

**Why `config.go` lives in `service/`, not a separate package:** the Config is the input to
the use case. Putting it next to the service keeps the construction contract (`NewEvidenceLoop(client, cfg)`)
in one place. The adapter takes a *narrower* construction config (base URL + credentials),
derived from the same env, but does not import `service` — see §4 for the split.

---

## 3. The Config struct

Injected at construction. NEVER hardcoded — this is the Kit-extraction precondition. The
`cmd` layer builds it from flags + env; tests build literals.

```go
package service

// Config carries all instance-specific Jira configuration. No values are
// hardcoded in domain, service, or adapter — they all flow from here.
type Config struct {
	// --- Site / project identity ---
	SiteURL       string   // e.g. "https://tablex.atlassian.net" (no trailing slash)
	CloudID       string   // Atlassian cloudId; reserved for v3 cloud-scoped routes
	ProjectKey    string   // "TAL"
	ProjectID     string   // "10099" (create uses id, not key — stable across renames)
	IssueTypeName string   // "Tarea" (Spanish on this site — config, never literal)

	// --- Transition disambiguation (Decision 3) ---
	// Maps a statusCategory.key to the ORDERED list of target state names that
	// belong to that category, in board-progression order. Two indeterminate
	// states ("In Progress" before "In Review") are disambiguated by their
	// index in this slice. Matching is by to.statusCategory.key first, then by
	// position; the name strings here are the disambiguator of LAST resort and
	// are compared case-insensitively against to.name only within a category.
	//
	// ⚠️ LIVE-DATA SLOT: the exact TAL column names + order MUST be filled from
	// the real TAL board (GET /project/TAL/statuses or the board UI) before the
	// integration smoke runs. Placeholder below documents the SHAPE, not the
	// final values. This is the one design-level open question that needs the
	// live board — sdd-tasks must include a task to capture it.
	OrderedStates map[StatusCategory][]string

	// --- Worklog / loop tuning ---
	DefaultWorklogSeconds int    // fallback when caller omits a duration
	RemoteLinkRelationship string // "is implemented by" (Jira relationship string)

	// --- Credentials source (Decision 1) ---
	// The adapter reads Basic-auth creds; Config only declares WHERE they come
	// from so the composition root resolves them. Values themselves are NOT
	// stored on Config that gets logged — see Credentials below.
	Credentials Credentials
}

// StatusCategory mirrors Jira's locale-independent status grouping (Decision 3).
type StatusCategory string

const (
	CategoryNew           StatusCategory = "new"           // To Do
	CategoryIndeterminate StatusCategory = "indeterminate" // In Progress / In Review
	CategoryDone          StatusCategory = "done"          // Done
)

// Credentials sources Basic-auth material. Empty fields ⇒ read from env
// (JIRA_EMAIL / JIRA_API_TOKEN). Never logged; String() is redacted.
type Credentials struct {
	Email      string // JIRA_EMAIL
	APIToken   string // JIRA_API_TOKEN (Basic auth password half) — secret
	FromEnv    bool   // if true, resolve Email/APIToken from env at build time
}
```

**Ordered-state mapping — resolution of open question #1.** The proposal flagged that the
exact TAL state names/order need the live board. This design resolves the *mechanism* (the
`OrderedStates map[StatusCategory][]string` shape and the index-based disambiguation
algorithm in §6) and leaves a **clearly-marked LIVE-DATA SLOT** for the values. The
placeholder the code ships with:

```go
OrderedStates: map[StatusCategory][]string{
	CategoryNew:           {"To Do"},                       // ⚠️ confirm TAL name
	CategoryIndeterminate: {"In Progress", "In Review"},    // ⚠️ confirm names + order
	CategoryDone:          {"Done"},                        // ⚠️ confirm TAL name
},
```

`sdd-tasks` MUST emit a task: "Capture live TAL board column names + order; replace the
OrderedStates placeholder before the integration smoke." Unit tests do not depend on the
real names (they assert the *algorithm* against synthetic transitions), so apply can proceed
on units while this slot is pending.

---

## 4. The port interface

```go
package port

import "github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"

// JiraClient is the driven port. The service depends on this and nothing else.
// adapter/rest implements it over net/http; mock/ implements it for tests.
type JiraClient interface {
	// CreateIssue creates an issue and returns its key (e.g. "TAL-142").
	CreateIssue(ctx context.Context, req evidence.CreateIssueRequest) (issueKey string, err error)

	// Search runs a JQL query and returns matching issue keys. Used for the
	// idempotency pre-create check (Decision 2).
	Search(ctx context.Context, jql string) (keys []string, err error)

	// GetTransitions returns the transitions available FROM the issue's current
	// state, each carrying its target statusCategory.key (Decision 3).
	GetTransitions(ctx context.Context, issueKey string) ([]evidence.Transition, error)

	// DoTransition moves the issue using a resolved transition id.
	DoTransition(ctx context.Context, issueKey, transitionID string) error

	// AddComment posts an ADF v3 comment body.
	AddComment(ctx context.Context, issueKey string, body evidence.ADFDocument) error

	// AddWorklog posts a worklog with seconds + optional ADF comment.
	AddWorklog(ctx context.Context, issueKey string, w evidence.Worklog) error

	// CreateRemoteLink upserts a remote link by globalId (Decision 2 idempotency).
	CreateRemoteLink(ctx context.Context, issueKey string, link evidence.RemoteLink) error

	// AddAttachment uploads a file (multipart, X-Atlassian-Token: no-check).
	AddAttachment(ctx context.Context, issueKey string, att evidence.Attachment) error
}
```

The proposal locks "the 7-method interface" (CreateIssue, GetTransitions, DoTransition,
AddComment, AddWorklog, CreateRemoteLink, AddAttachment). `Search` is the 8th method and is
**required** to implement Decision 2 (pre-create JQL idempotency) — it is a read, not part of
the write loop, and the exploration already listed it. Keeping it on the same port avoids a
second adapter and keeps the composition root single.

---

## 5. Domain value objects (pure)

`domain/evidence` holds only data + validation. No `context`, no `http`, no JSON tags that
imply transport (the adapter owns wire mapping). Key types:

- `Label` — a `namespace:value` pair; parses/validates `agent:*`, `module:*`, `change:*`,
  `phase:*`. `LabelSet` enforces the §4 invariant: exactly one `agent:*`, exactly one
  `module:*`.
- `CreateIssueRequest` — ProjectID, IssueTypeName, Summary, Description (ADFDocument), Labels.
- `Transition` — `{ID, ToName string, ToCategory StatusCategory}`. The category is the
  locale-independent match key (Decision 3).
- `Worklog` — `{Seconds int, Started time.Time, Comment ADFDocument}`.
- `RemoteLink` — `{GlobalID, Relationship, URL, Title, IconURL}`. GlobalID is `pr=<url>` for
  upsert (Decision 2).
- `Attachment` — `{FileName, ContentType string, Data []byte}` (or `io.Reader` — adapter
  reads it). Loud error if empty.
- `ADFDocument` — the doc-node tree (see §8). Lives in domain because both comment and
  worklog bodies are ADF and the service builds them.

**Ownership invariant (`ownership.go`)** — pure function:

```go
// ValidateOwnership checks the §4 invariant before any write.
// ownership is the module→agent map sourced from team-context/ownership.md
// and injected (not read from disk here — keeps domain pure).
func ValidateOwnership(labels LabelSet, ownership map[string]string) error
```

Rules: exactly one `agent:*`, exactly one `module:*`, and `ownership[module] == agent`.
Returns a typed error on violation. Unit-tested with zero mocking. The service calls this
**before** `CreateIssue` and refuses to touch the API on violation.

---

## 6. The 7-step flow

Orchestrated by `service.EvidenceLoop.Run(ctx, input)`. Each step names where a **domain
validation** (V) and where an **idempotency check** (I) sits.

```
INPUT: change, phase, agent, module, summary, descriptionADF, prURL, attachment, worklogSecs

  0. PRE-WRITE GUARD (V)  ── domain.ValidateOwnership(labels, ownership)
     │  exactly-one agent/module + ownership[module]==agent. Fail ⇒ STOP, no API call.
     │
  1. IDEMPOTENT CREATE (I) ── Search(jql) on the (change:*, phase:*, agent:*) tuple
     │  jql = project=<key> AND labels=change:<c> AND labels=phase:<p> AND labels=agent:<a>
     │  ├─ match found  ⇒ reuse existing issueKey, SKIP create  (Decision 2: 1 issue/tuple)
     │  └─ no match     ⇒ CreateIssue(req) ⇒ issueKey
     │
  2. TRANSITION → In Progress ── GetTransitions(key) → resolve(indeterminate, ordinal=0)
     │  DoTransition(key, id). (resolution algorithm below.)
     │  Idempotent: if already in target category at the right ordinal, resolve() returns
     │  a no-op signal ⇒ skip DoTransition.
     │
  3. ADD COMMENT (ADF) ── AddComment(key, adf.Build(text, links))
     │
  4. ADD WORKLOG (ADF) ── AddWorklog(key, Worklog{Seconds, Started:now, Comment})
     │
  5. CREATE REMOTE LINK (I) ── CreateRemoteLink(key, RemoteLink{GlobalID:"pr="+prURL, ...})
     │  globalId upsert ⇒ re-run is safe natively (Decision 2). FAIL-LOUD on error (§6 DoD).
     │
  6. ADD ATTACHMENT ── AddAttachment(key, Attachment{verify-report, ...})
     │  FAIL-LOUD on error (CONSTITUTION §7 "falla fuerte"; no swallow).
     │
  7. TRANSITION → Done ── GetTransitions(key) → resolve(done, ordinal=0) → DoTransition
     │  Idempotent: already Done ⇒ no-op.

OUTPUT: issueKey, plus a per-step result record (created? reused? skipped? ).
```

**Where validations sit:** all label/ownership validation is step 0, *before* the network.
ADF well-formedness is enforced at build time in `adf.go` (steps 3–4). Attachment
non-emptiness is validated in the domain before step 6.

**Where idempotency sits:** step 1 (JQL pre-create) and step 5 (globalId upsert) are the two
native idempotency points. Steps 2 and 7 are made idempotent by the resolution algorithm
returning a no-op when already in the target state. Steps 3, 4, 6 are *additive* and NOT
idempotent by themselves — see §9 partial-progress semantics.

---

## 7. Transition resolution algorithm

`adapter/rest/transitions.go` (pure, no I/O — takes the slice `GetTransitions` returned).
Disambiguates the two `indeterminate` states.

```
resolve(available []Transition, targetCat StatusCategory, ordinal int, ordered Config.OrderedStates)
   → (transitionID string, isNoOp bool, err error)

1. Filter `available` to those whose `ToCategory == targetCat`.
2. If zero candidates:
   - the target may be unreachable from the current state (Jira only returns transitions
     valid FROM the current status). Check if the issue is ALREADY in targetCat at `ordinal`
     (caller passes current state) ⇒ return isNoOp=true.
   - else ⇒ ERROR "target state <cat>[<ordinal>] not reachable from current state"
     (fail-loud; do not silently skip a required transition).
3. If exactly one candidate ⇒ return its id.
4. If MULTIPLE candidates (the In Progress vs In Review case, both indeterminate):
   - take ordered[targetCat] = the board-ordered name list for that category.
   - the desired state name = ordered[targetCat][ordinal]  (ordinal 0 = In Progress).
   - match candidates by case-insensitive `ToName == desiredName`.
     ├─ match ⇒ return its id.
     └─ no name match (board renamed / locale drift) ⇒ fall back to ORDINAL position:
        sort candidates by their index in ordered[targetCat]; return the one at `ordinal`.
        If still ambiguous ⇒ ERROR (loud; never guess between two writes).
```

**Why ordinal + name, not name alone:** Decision 3 says match by `statusCategory.key`, never
by localized name. But two `indeterminate` states share a category, so category alone can't
pick. The ordered list provides a stable, config-driven tiebreak: primary key is category
(locale-independent), secondary is position in the configured order (board-truth), name is
only a within-category convenience match. If the board was renamed and names drift, ordinal
position still resolves it. If even that is ambiguous, we fail loud rather than transition to
the wrong column.

**Unreachable target:** step 2 above. The loop never silently skips a required transition; an
unreachable In Progress or Done is a hard error surfaced to the caller (the agent then
inspects the board / config).

---

## 8. ADF builder

`adapter/rest/adf.go`. Resolution of open question #2: **minimal v3 node set for Fase 1** =
`doc` / `paragraph` / `text` / `text-with-link-mark`. No code blocks, panels, mentions, or
media nodes yet — they are not needed for comment/worklog bodies in Fase 1 and adding them
now is speculative surface.

The builder is an **interface** so it can grow without touching call sites:

```go
package rest

// ADFBuilder constructs ADF v3 documents. Interface so the node set can expand
// (code blocks, panels, mentions) in later phases without changing the service.
type ADFBuilder interface {
	Doc() *ADFDoc
}

// Fluent builder for the Fase-1 node set:
//   Paragraph(text)            → { type:"paragraph", content:[{type:"text", text}] }
//   Link(text, href)           → text node with a link mark
// b.Doc().Paragraph("Done.").ParagraphWithLink("See PR", "https://...").Build()
```

Emitted shape (the contract `adf_test.go` asserts against fixtures):

```json
{ "type": "doc", "version": 1,
  "content": [
    { "type": "paragraph",
      "content": [
        { "type": "text", "text": "See PR ", },
        { "type": "text", "text": "#142",
          "marks": [ { "type": "link", "attrs": { "href": "https://github.com/..." } } ] }
      ] } ] }
```

`evidence.ADFDocument` (domain) is the value type; `rest.ADFBuilder` (adapter) is the
construction helper. The service builds bodies via the builder and passes the resulting
`ADFDocument` to the port — the port signature stays transport-agnostic.

---

## 9. Error handling & partial-progress semantics

**Fail-loud (CONSTITUTION §7 "falla fuerte"):** attachment (step 6) and remote-link (step 5)
errors are **never swallowed**. They abort `Run` and propagate. The DoD (§6) lives in
evidence — a missing attachment or remote link means the loop did NOT satisfy DoD, so it must
be a hard failure, not a warning.

**Partial-progress semantics — what's left if it dies mid-loop:**

| Dies after step | State left behind | Re-run behaviour |
|---|---|---|
| 0 (guard fails) | nothing — no API call | clean |
| 1 (create) | issue exists, To Do | step 1 JQL finds it ⇒ reuses, no dup |
| 2 (In Progress) | issue In Progress | step 2 resolve() ⇒ no-op, continues |
| 3 (comment) | comment posted | re-run **re-posts** comment (additive — see note) |
| 4 (worklog) | worklog logged | re-run **re-logs** worklog (additive) |
| 5 (remote link) | link upserted | globalId ⇒ upsert, no dup |
| 6 (attachment) | file attached | re-run **re-attaches** (additive) |
| 7 (Done) | issue Done | step 7 resolve() ⇒ no-op |

**The idempotency guarantee:** re-running a failed loop never creates a duplicate *issue*
(step 1) or duplicate *remote link* (step 5). The additive steps (comment, worklog,
attachment) CAN duplicate on re-run. For Fase 1 this is accepted: the contract is "one issue
per (change, phase, agent) tuple," not "exactly-once side effects." A duplicate comment is
noise, not corruption. (If later phases need exactly-once additive steps, the mechanism would
be a marker comment / attachment-name dedup check — explicitly OUT of scope now.) `sdd-tasks`
should NOT add dedup logic for steps 3/4/6.

**Error typing:** the service returns typed errors (`ErrOwnershipViolation`,
`ErrTransitionUnreachable`, `ErrAttachmentFailed`, wrapped transport errors) so the CLI can
exit with distinct codes and the caller agent can branch.

---

## 10. Testing strategy

**Unit (zero live calls, runs with NO token):**
- `service/evidence_loop_test.go` — table-driven against `mock.JiraClient`. Cases: happy path
  (7 steps, correct args, correct order); idempotent create (Search returns a key ⇒ no
  CreateIssue); ownership violations (missing agent, wrong owner, extra label ⇒ STOP before
  any call); transition resolution wired (mock returns 3 transitions ⇒ correct id chosen);
  attachment error propagates (fail-loud, loop aborts); remote-link error propagates.
- `domain/evidence/*_test.go` — pure value-object + invariant tests (label parse, LabelSet
  invariant, ownership validation, attachment non-empty). No mocks.
- `adapter/rest/adf_test.go` — builder emits exact ADF JSON vs golden fixtures.
- `adapter/rest/transitions_test.go` — resolution algorithm against synthetic transition
  slices, including the two-indeterminate disambiguation and the unreachable-target error.
  **Does not need the live TAL names** — asserts the algorithm, not the board.

**Integration smoke (`//go:build integration`, env-gated):**
- `adapter/rest/client_integration_test.go` — gated by `JIRA_EMAIL` + `JIRA_API_TOKEN`. Runs
  the real loop against a throwaway TAL issue (create → transitions → comment → worklog →
  remote link → attachment → done), then cleans up. Run via `go test -tags=integration ./...`.
  Skipped automatically when env is absent (`t.Skip`).

**What is covered without a live token:** the entire service orchestration, every domain
rule, the ADF wire shape, and the transition algorithm — i.e. all decision logic. **What is
NOT covered:** real Basic-auth handshake, real ADF acceptance by Jira, multipart attachment
upload, and the live transition IDs/names. Those need the integration smoke and ZEUS's token
(hard prerequisite, Decision 1). Apply proceeds on the unit suite; the smoke waits for the
token.

**Strict TDD:** every service/domain/adapter-pure file gets a failing test first. The adapter
HTTP wiring (`client.go` request construction) is unit-tested with an `httptest.Server` (a
local fake, not live Jira) to assert request shape — method, path, headers (Basic auth,
`X-Atlassian-Token: no-check`), body JSON — without the integration tag.

---

## 11. CLI surface

Resolution of open question #3: `cmd/evidence` is a **thin shim** over a callable library
API. Other phases import `service` directly; the CLI exists for the agent (HERMES) to invoke
from a shell / CI step.

**Library boundary:** `service.NewEvidenceLoop(client port.JiraClient, cfg Config)` +
`loop.Run(ctx, RunInput)` is the public API. Importers wire their own client. The CLI is just
one importer.

**CLI subcommands** (hand-rolled `flag` package — zero deps, Kit-clean):

```
evidence run-loop      # the full 7-step loop (primary entry; what HERMES calls)
evidence create        # step 1 only (idempotent create) — debugging / per-step
evidence transition    # steps 2/7 — --to=in-progress|done
evidence comment       # step 3
evidence worklog       # step 4
evidence link          # step 5 (remote link)
evidence attach        # step 6
```

`run-loop` is the main path; per-step subcommands exist for debugging and for agents that
need finer control. Shared flags:

```
--change, --phase, --agent, --module   # identity → labels (drive ownership guard)
--summary, --description-file          # issue fields
--pr-url                               # remote link target
--attach                               # attachment file path (verify-report)
--worklog-seconds
--config                               # path to a config file/env for Config (optional)
# credentials: JIRA_EMAIL / JIRA_API_TOKEN from env (Decision 1) — never flags
```

**How HERMES invokes it:** in the `apply`/`verify` CI step,
`evidence run-loop --change=jira-evidence-loop --phase=apply --agent=hermes
--module=jira-loop --summary="..." --pr-url=<PR> --attach=verify-report.md`. Credentials come
from the worktree `.env` (Fase 1) or the GitHub Actions secret (CI smoke). Exit code is
non-zero on any fail-loud error.

---

## 12. ADR-style decisions (this phase)

The 4 proposal decisions are LOCKED and not re-opened. This design adds these
architecture-level decisions:

**ADR-D1 — Config lives in `service/`, adapter takes a narrower construction struct.**
*Rationale:* Config is the use-case input; co-locating keeps the construction contract
single. The adapter needs only base URL + credentials, so it takes its own small struct
derived from the same env — avoids the adapter importing `service` (would invert the
dependency arrow). *Rejected:* a shared top-level `config` package (extra package, no
benefit at this size); putting Config in `domain` (domain must stay I/O- and
instance-agnostic).

**ADR-D2 — `Search` is the 8th port method, on the same interface.**
*Rationale:* Decision 2's pre-create JQL needs a read; a single port = single adapter =
single composition root. *Rejected:* a separate `JiraReader` port (premature split for one
read method); doing the JQL outside the port (would put HTTP in the service).

**ADR-D3 — Transition resolution = category-primary, ordinal-secondary, name-tiebreak.**
*Rationale:* honours Decision 3 (category is the locale-independent key) while solving the
two-indeterminate ambiguity via the configured ordered list; degrades to ordinal position if
names drift; fails loud rather than guessing. *Rejected:* name-only matching (locale-fragile,
Decision 3 forbids it); hardcoded transition IDs (CONSTITUTION §7 forbids for team-managed
projects).

**ADR-D4 — Additive steps (comment/worklog/attachment) are not made idempotent in Fase 1.**
*Rationale:* the contract is one-issue-per-tuple, not exactly-once side effects; re-run dup
is noise, not corruption; dedup adds complexity for marginal value now. *Rejected:* marker-
comment dedup, attachment-name dedup — explicitly deferred (proposal No-goals).

**ADR-D5 — `OrderedStates` is the config slot for the live TAL board; ships with a
documented placeholder.** *Rationale:* the algorithm is testable without live names; the
values are a deploy-time concern gated on the live board. *Rejected:* hardcoding guessed
names (would break silently on the real board); blocking apply on the live data (units don't
need it).

---

## 13. Decisions tasks must respect

1. **Strict TDD order:** domain + pure adapter helpers (ADF, transitions) and the service
   (against the mock) are unit-tested FIRST. The HTTP wiring uses `httptest.Server`, not live
   Jira. Integration smoke is build-tagged + env-gated and does NOT block the unit suite.
2. **Dependency arrows:** `service` never imports `adapter/rest`. `cmd/evidence` is the only
   composition root. Any task that wires the concrete client elsewhere is wrong.
3. **Zero transitive deps:** no go-jira, no cobra, no testify-with-deps. Hand-written client
   (`net/http`), hand-rolled CLI (`flag`), hand-written mock, std-lib `testing`. `go.sum`
   stays minimal. This is the Kit-extraction gate — do not relax it.
4. **No hardcoded instance values** anywhere outside Config construction in `cmd`/tests.
   Site URL, project key/id, issue type "Tarea", state names — all via Config.
5. **`OrderedStates` placeholder + a task to capture live TAL columns** before the integration
   smoke. Mark it clearly; unit tests must not depend on the real names.
6. **Fail-loud on attachment (step 6) and remote link (step 5)** — no error swallowing.
7. **Idempotency:** step 1 JQL pre-create on `(change, phase, agent)`; step 5 globalId
   `pr=<url>` upsert. Steps 2/7 no-op when already in target state. Do NOT add dedup for
   steps 3/4/6.
8. **`X-Atlassian-Token: no-check`** header is mandatory on the multipart attachment request
   — the adapter HTTP test must assert it.
9. **Basic auth from env only** (`JIRA_EMAIL`/`JIRA_API_TOKEN`), never CLI flags, never logged.
10. **8-method port** (7 writes + `Search`); the mock implements all 8.
11. **Ownership map is injected**, not read from `team-context/ownership.md` inside the domain
    — domain stays pure; the CLI/composition root loads the map and passes it in.
12. **`team-context/ownership.md`** gets the `module:jira-loop | HERMES | ... | active` row
    (from the proposal scope) — a task outside the Go module.
