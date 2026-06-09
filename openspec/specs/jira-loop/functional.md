# Spec: jira-evidence-loop — Functional Requirements

**Change:** jira-evidence-loop
**Module:** module:jira-loop
**Owner:** HERMES
**RFC 2119 keywords apply throughout.**

---

## Overview

This spec describes the observable behaviour that MUST be true after the `jira-evidence-loop` change
is fully applied. It does NOT prescribe implementation; that is the design phase's responsibility.

The system under specification is a Go library that performs the 7-step Jira evidence loop
(create → transition In Progress → comment → worklog → remote link → attachment → transition Done)
deterministically over the Jira REST v3 API.

---

## REQ-AUTH — Authentication

### REQ-AUTH-1: Credential source
**Given** the process environment,
**When** the loop initialises,
**Then** secrets (`JIRA_EMAIL`, `JIRA_API_TOKEN`) MUST be sourced exclusively from the environment
(populated by a gitignored `.env` file or an inherited environment) and MUST NOT appear in flags,
logs, or any committed file.
Non-secret project identity (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) MAY be
pre-populated from `.talos/project.env`, which is a committed, non-secret runtime mirror of
`openspec/config.yaml:jira`. Real environment variables always win (if-unset load semantics).

### REQ-AUTH-2: Missing credentials fail fast
**Given** that either `JIRA_EMAIL` or `JIRA_API_TOKEN` is absent or empty,
**When** the loop is constructed,
**Then** it MUST return a clear, actionable error before performing any network call.
The error message MUST identify which variable is missing.

### REQ-AUTH-3: Auth encoding
**Given** valid credentials,
**When** any HTTP request is issued,
**Then** the `Authorization` header MUST use HTTP Basic auth encoded as
`Basic base64(JIRA_EMAIL:JIRA_API_TOKEN)`.
No other auth scheme is permitted.

---

## REQ-CFG — Configuration injection

### REQ-CFG-1: No hardcoded values
**Given** the loop implementation,
**When** it is read by any reviewer,
**Then** the site URL, project key, project ID, issue type names, and ordered state list
MUST NOT appear as string literals in any non-test Go source file.
All such values MUST be injected via a `Config` struct passed at construction time.

### REQ-CFG-2: Config completeness
**Given** a `Config` struct,
**When** the loop is constructed,
**Then** it MUST validate that `SiteURL`, `ProjectKey`, `ProjectID`, `TaskIssueType`, and
`OrderedStates` are all non-empty, and MUST return an error if any is absent.

### REQ-CFG-3: Ordered state list
**Given** a `Config.OrderedStates` list,
**When** the loop needs to resolve "In Progress" vs "In Review" or any other `indeterminate`
state category,
**Then** it MUST select the state whose name appears EARLIEST in `OrderedStates`
among all candidate transitions in the `indeterminate` category.

---

## REQ-LABEL — Label schema and ownership invariant

### REQ-LABEL-1: Mandatory labels on create
**Given** a create context containing `AgentName`, `ModuleName`, `ChangeName`, and `PhaseName`,
**When** the loop creates a Jira issue,
**Then** the issue MUST carry exactly one label in each of the four namespaces:
- one `agent:<AgentName>`
- one `module:<ModuleName>`
- one `change:<ChangeName>`
- one `phase:<PhaseName>`

No other `agent:*`, `module:*`, `change:*`, or `phase:*` label MAY appear on the same issue.

### REQ-LABEL-2: Ownership invariant enforced pre-write
**Given** a create context where `ownership[ModuleName] != AgentName`,
**When** the loop is asked to create an issue,
**Then** it MUST reject the request with a descriptive error BEFORE making any API call.
No partial state (network side-effects) is acceptable for this failure mode.

### REQ-LABEL-3: Missing or duplicated label detection
**Given** a create context where any of the four label namespaces is absent or appears more
than once,
**When** the loop validates the context,
**Then** it MUST return an error identifying the offending namespace(s) BEFORE making any API call.

---

## REQ-CREATE — Issue creation

### REQ-CREATE-1: Issue type
**Given** a valid create context,
**When** the loop creates an issue in project TAL,
**Then** the issue type MUST be `Config.TaskIssueType` (configured value, e.g. "Tarea").
No other issue type is used by this loop.

### REQ-CREATE-2: Project targeting
**Given** a valid create context,
**When** the loop creates an issue,
**Then** the issue MUST be created in the project identified by `Config.ProjectKey` and
`Config.ProjectID`.

### REQ-CREATE-3: Create response handling
**Given** a successful create API call,
**When** the response is received,
**Then** the loop MUST extract and retain the issue key (e.g. `TAL-42`) for all subsequent steps.
If the key is absent from the response, the loop MUST fail with an error.

---

## REQ-IDEM — Idempotency

### REQ-IDEM-1: Pre-create JQL check
**Given** any create invocation,
**When** the loop begins the create step,
**Then** it MUST first execute a JQL query searching for issues matching the tuple
`(change:<ChangeName>, phase:<PhaseName>, agent:<AgentName>)` within the configured project.

### REQ-IDEM-2: Existing issue reuse
**Given** the JQL check returns exactly one matching issue,
**When** the create step processes the result,
**Then** it MUST reuse that issue's key and MUST NOT create a new issue.

### REQ-IDEM-3: No duplicate on match
**Given** the JQL check returns one or more matching issues,
**When** the create step processes the result,
**Then** zero new issues MUST be created for this invocation.

### REQ-IDEM-4: Ambiguous match is an error
**Given** the JQL check returns more than one matching issue,
**When** the create step processes the result,
**Then** the loop MUST fail with an error listing the conflicting keys.
It MUST NOT silently pick one.

### REQ-IDEM-5: Remote link upsert via globalId
**Given** a remote link (PR) to attach,
**When** the loop creates or updates a remote link,
**Then** it MUST use a stable `globalId` of the form `pr=<url>` so that repeated invocations
upsert the existing link rather than creating duplicates.

---

## REQ-TRANS — Transitions

### REQ-TRANS-1: Dynamic resolution only
**Given** a target status category (new / indeterminate / done),
**When** the loop needs to transition an issue,
**Then** it MUST call the GET transitions endpoint for that specific issue and resolve the
transition ID dynamically from the response.
Hardcoded transition IDs and hardcoded status names MUST NOT appear in non-test source.

### REQ-TRANS-2: Match by statusCategory.key
**Given** the list of available transitions from the API,
**When** selecting the target transition,
**Then** the loop MUST match on `to.statusCategory.key` (values: `new`, `indeterminate`, `done`).
Matching on localized `to.name` (e.g. Spanish names) is PROHIBITED.

### REQ-TRANS-3: Indeterminate disambiguation
**Given** multiple transitions whose `to.statusCategory.key` is `indeterminate`,
**When** the loop selects among them,
**Then** it MUST apply `Config.OrderedStates` and pick the candidate whose `to.name` appears
EARLIEST in the list.

### REQ-TRANS-4: No matching transition is an error
**Given** a dynamic transition lookup for a target status category,
**When** no transition with the required `to.statusCategory.key` exists,
**Then** the loop MUST fail with an error identifying the issue key and the missing category.
It MUST NOT silently skip the transition step.

### REQ-TRANS-5: Loop transition sequence
**Given** a successful create step,
**When** the loop executes steps 2 and 7,
**Then** step 2 MUST transition the issue to an `indeterminate` state (In Progress), and
step 7 MUST transition the issue to the `done` state, in that order, with steps 3–6 between them.

---

## REQ-COMMENT — Comment step

### REQ-COMMENT-1: ADF v3 body
**Given** a comment to add to an issue,
**When** the loop calls the add-comment endpoint,
**Then** the request body MUST be a valid ADF v3 document.
Plain-text payloads are PROHIBITED.

### REQ-COMMENT-2: Minimal ADF surface
**Given** the ADF builder,
**When** it constructs a comment body,
**Then** it MUST support at minimum: `paragraph`, `text`, and `link` node types.
Support for additional node types (code blocks, mentions, panels) is OPTIONAL for Fase 1.

---

## REQ-WORKLOG — Worklog step

### REQ-WORKLOG-1: ADF v3 comment body
**Given** a worklog entry to record,
**When** the loop calls the add-worklog endpoint,
**Then** the `comment` field MUST be a valid ADF v3 document using the same builder as REQ-COMMENT-1.

### REQ-WORKLOG-2: Time spent format
**Given** a time duration to log,
**When** the loop constructs the worklog payload,
**Then** `timeSpent` MUST conform to Jira's duration string format (e.g. `1h`, `30m`).

---

## REQ-REMOTELINK — Remote link (PR) step

### REQ-REMOTELINK-1: Stable globalId
**Given** a PR URL to link,
**When** the loop calls the create-remote-link endpoint,
**Then** the payload MUST include `globalId: "pr=<url>"` where `<url>` is the exact PR URL,
ensuring subsequent calls upsert rather than duplicate (see also REQ-IDEM-5).

### REQ-REMOTELINK-2: Link relationship label
**Given** a remote link payload,
**When** the loop constructs it,
**Then** `relationship` MUST be set to a human-readable string (e.g. `"Pull Request"`) and
MUST NOT be empty.

---

## REQ-ATTACH — Attachment step

### REQ-ATTACH-1: Multipart upload with required header
**Given** a file (e.g. verify-report) to attach to an issue,
**When** the loop calls the add-attachment endpoint,
**Then** the request MUST be a `multipart/form-data` upload and MUST include the header
`X-Atlassian-Token: no-check` (Atlassian XSRF bypass; the call fails without it).

### REQ-ATTACH-2: Fail loud on error
**Given** an attachment upload that returns a non-2xx HTTP status or a network error,
**When** the loop receives that response,
**Then** it MUST surface the error to the caller and MUST abort the loop.
Silent swallowing of attachment errors is PROHIBITED.

### REQ-ATTACH-3: No silent retry
**Given** an attachment failure,
**When** the loop handles it,
**Then** it MUST NOT silently retry. Retry behaviour, if any, is the caller's responsibility.

---

## REQ-ERR — Error handling and partial progress

### REQ-ERR-1: Fail fast, propagate context
**Given** any step in the 7-step loop that returns an error,
**When** that error is received,
**Then** the loop MUST stop and return an error that includes: the step name, the issue key
(if already created), and the underlying API error detail.
Subsequent steps MUST NOT execute after a step failure.

### REQ-ERR-2: Network error mid-loop
**Given** a network error occurring after issue creation but before the final transition,
**When** the loop propagates the error,
**Then** the returned error MUST include the issue key so the caller can record partial progress
(e.g. comment a failure note and transition back to To Do, as per CONSTITUTION §11).

### REQ-ERR-3: No panic
**Given** any error condition including unexpected API response shapes,
**When** the loop handles it,
**Then** the loop MUST NOT panic. It MUST return an error value.

---

## REQ-TEST — Testability

### REQ-TEST-1: Port boundary
**Given** the module's hexagonal layout,
**When** unit tests run,
**Then** all service-layer tests MUST depend only on the `port.JiraClient` interface (or
equivalent port), never on the concrete HTTP adapter.
No live HTTP calls are permitted in unit tests.

### REQ-TEST-2: Unit test coverage of ownership invariant
**Given** REQ-LABEL-2,
**When** unit tests are written,
**Then** there MUST be at least one table-driven test case that verifies rejection when
`ownership[module] != agent` and one case that verifies acceptance when the pair is valid.

### REQ-TEST-3: Unit test coverage of idempotency
**Given** REQ-IDEM-1 through REQ-IDEM-4,
**When** unit tests are written,
**Then** there MUST be table-driven test cases covering: no match (creates), exactly one match
(reuses), and more than one match (error).

### REQ-TEST-4: Unit test coverage of transition matching
**Given** REQ-TRANS-1 through REQ-TRANS-4,
**When** unit tests are written,
**Then** there MUST be test cases covering: single indeterminate match, multiple indeterminate
resolved by ordered list, and no matching transition (error).

### REQ-TEST-5: Integration smoke (build-tagged)
**Given** the `//go:build integration` tag,
**When** the integration smoke test runs,
**Then** it MUST require `JIRA_EMAIL` and `JIRA_API_TOKEN` to be set and MUST be skipped
(not failed) when they are absent.
It MAY run against a real TAL issue.

---

## REQ-OWNER — Ownership registration

### REQ-OWNER-1: ownership.md entry
**Given** the change is applied,
**When** `team-context/ownership.md` is read,
**Then** it MUST contain a row mapping `module:jira-loop` → `HERMES` with status `active`.

---

## Edge Cases and Design-Time Ambiguities

The following items were identified during spec writing. They are NOT blocking for the spec but
MUST be resolved by the design phase or flagged to ZEUS before apply.

| ID | Description | Impact |
|----|-------------|--------|
| EC-1 | `OrderedStates` exact initial values — the actual TAL board column names and their `statusCategory.key` grouping are unknown without a live GET transitions call. The spec requires Config injection but cannot validate the initial list. | Design must document the expected initial `OrderedStates` value and how it is discovered (e.g. one-time manual GET, CI fixture). |
| EC-2 | ADF builder scope for Fase 1 — the spec requires `paragraph`, `text`, `link` as minimum. Whether `code`, `mention`, or `panel` nodes are needed for real comment/worklog bodies in Fase 1 is unspecified. | Design or apply phase must decide and document the supported node set. |
| EC-3 | CLI surface vs library API — `cmd/evidence/main.go` shape (single run-loop command, per-step subcommands, flags) is unspecified. The spec only requires the library be callable. | Design must define the CLI contract so apply has a clear target. |
| EC-4 | Remote link endpoint routing — CONSTITUTION §7 assigns `jira_create_remote_issue_link` to the community MCP server, but this module uses its own hand-written HTTP client. The spec requires the correct REST v3 endpoint path for remote links. Design must confirm the endpoint and whether any capability flag differs from the MCP server. |
| EC-5 | Worklog `startedAt` field — Jira's add-worklog endpoint requires a `started` timestamp in a specific ISO 8601 variant. The format and whether it should be the loop's wall-clock time or injected is unspecified. | Design must decide and document. |
