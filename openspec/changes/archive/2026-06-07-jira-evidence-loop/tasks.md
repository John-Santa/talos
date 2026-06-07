# Tasks: jira-evidence-loop

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 600–850 LOC |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → domain + service + mock + unit tests; PR 2 → adapter/rest + cmd + integration smoke + ownership row |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | domain + port + service + mock + all unit tests | PR 1 | Base = `agent/hermes/TAL-<n>`; zero live calls; no token needed |
| 2 | adapter/rest + cmd/evidence + integration smoke + ownership row | PR 2 | Base = PR 1 branch; ZEUS Jira token needed for integration smoke only |

---

## Phase 1: Foundation — Module Init + Domain (PR 1)

- [ ] 1.1 Create `platform/jira-evidence-loop/go.mod` with module path `github.com/John-Santa/talos/platform/jira-evidence-loop`, no transitive deps; create `.env.example` with `JIRA_EMAIL=` and `JIRA_API_TOKEN=` entries. **REQ-AUTH, REQ-CFG**
- [ ] 1.2 **[RED]** Write `domain/evidence/issue_test.go`: table-driven tests for `Label`, `LabelSet`, and `ADFDocument` value objects — zero-value guards, required-field presence. **REQ-LABEL, REQ-COMMENT, REQ-TEST**
- [ ] 1.3 **[GREEN]** Write `domain/evidence/issue.go`: `Issue`, `Label`, `LabelSet`, `Transition`, `Worklog`, `RemoteLink`, `Attachment`, `ADFDocument`, `CreateIssueRequest` — pure value objects, zero I/O, zero third-party imports. **REQ-CREATE, REQ-COMMENT, REQ-WORKLOG, REQ-REMOTELINK, REQ-ATTACH**
- [ ] 1.4 **[RED]** Write `domain/evidence/ownership_test.go`: table-driven tests for `ValidateOwnership` — exactly-1 agent, exactly-1 module, ownership[module]==agent, missing/duplicate label, wrong agent. **REQ-LABEL**
- [ ] 1.5 **[GREEN]** Write `domain/evidence/ownership.go`: pure `ValidateOwnership(labels LabelSet, ownership map[string]string) error`; typed error `ErrOwnershipViolation`. **REQ-LABEL**
- [ ] 1.6 Write `port/jira_client.go`: `JiraClient` interface — 8 methods: `CreateIssue`, `Search`, `GetTransitions`, `DoTransition`, `AddComment`, `AddWorklog`, `CreateRemoteLink`, `AddAttachment`. **REQ-IDEM, REQ-TRANS, REQ-COMMENT, REQ-WORKLOG, REQ-REMOTELINK, REQ-ATTACH**

## Phase 2: Service + Mock (PR 1)

- [ ] 2.1 Write `service/config.go`: `Config` struct — `SiteURL`, `CloudID`, `ProjectKey`, `ProjectID`, `IssueTypeName`, `OrderedStates map[StatusCategory][]string`, `DefaultWorklogSeconds`, `RemoteLinkRelationship`, `Credentials`; `StatusCategory` consts (`new`/`indeterminate`/`done`); `NewConfig()` validates on construction. **REQ-CFG**
- [ ] 2.2 Write `mock/jira_client_mock.go`: hand-written test double implementing all 8 `JiraClient` methods; supports configurable stub returns per call. **REQ-TEST**
- [ ] 2.3 **[RED]** Write `service/evidence_loop_test.go`: table-driven, mock-backed — happy path (7-step order verified), idempotent create (0/1/>1 match), ownership violation stops before API call, transition wiring, attachment/link error propagation, typed errors. **REQ-IDEM, REQ-TRANS, REQ-ERR, REQ-TEST**
- [ ] 2.4 **[GREEN]** Write `service/evidence_loop.go`: `EvidenceLoop` struct, `NewEvidenceLoop(client JiraClient, cfg Config)`, `Run(ctx, RunInput)` — 7-step flow: ValidateOwnership(0), IdempotentCreate(1), Transition→indeterminate(2), AddComment(3), AddWorklog(4), CreateRemoteLink(5 fail-loud), AddAttachment(6 fail-loud), Transition→done(7). Typed errors `ErrTransitionUnreachable`, `ErrAttachmentFailed`. **REQ-IDEM, REQ-TRANS, REQ-COMMENT, REQ-WORKLOG, REQ-REMOTELINK, REQ-ATTACH, REQ-ERR**

> **PR 1 ends here.** Branch: `agent/hermes/TAL-<n>-domain-service`. All unit tests pass with zero live calls.

---

## Phase 3: Adapter / REST (PR 2, base = PR 1 branch)

- [ ] 3.1 Write `adapter/rest/auth.go`: `BasicAuth(email, token string) string` — base64 encode, zero deps. **REQ-AUTH**
- [ ] 3.2 **[RED]** `[P]` Write `adapter/rest/adf_test.go`: golden-fixture tests for ADF v3 builder — paragraph/text/link nodes, exact JSON output. **REQ-COMMENT, REQ-WORKLOG, REQ-TEST**
- [ ] 3.3 **[GREEN]** `[P]` Write `adapter/rest/adf.go`: `ADFBuilder` interface + concrete builder — `doc/paragraph/text/text-with-link-mark` node set only; no cobra/third-party deps. **REQ-COMMENT**
- [ ] 3.4 **[RED]** `[P]` Write `adapter/rest/transitions_test.go`: synthetic table-driven tests — single candidate, 2-indeterminate disambiguation (by ordinal), unreachable (zero candidates, not already in category) → error. Uses synthetic names; no live TAL names needed. **REQ-TRANS, REQ-TEST**
- [ ] 3.5 **[GREEN]** `[P]` Write `adapter/rest/transitions.go`: `resolveTransition(transitions []Transition, targetCat StatusCategory, ordered []string, ordinal int) (id string, err error)` — category-primary, ordinal-secondary, name-tiebreak, fail loud on ambiguous. **REQ-TRANS**
- [ ] 3.6 **[RED]** Write `adapter/rest/client_test.go` (uses `httptest.Server`, no build tag, no live calls): assert method/path/headers (`Authorization: Basic ...`, `X-Atlassian-Token: no-check` on attach, `Content-Type` on multipart), body JSON shape for each of the 8 port methods. **REQ-AUTH, REQ-ATTACH, REQ-TEST**
- [ ] 3.7 **[GREEN]** Write `adapter/rest/client.go`: `Client` struct implementing `JiraClient` — `net/http` only, no third-party; Basic auth on every request; multipart `AddAttachment` with `X-Atlassian-Token: no-check`; fail-loud on non-2xx; typed errors from design. **REQ-AUTH, REQ-ATTACH, REQ-ERR**

## Phase 4: CLI + Integration Smoke (PR 2)

- [ ] 4.1 Write `cmd/evidence/main.go`: thin CLI shim over library — `flag` pkg only; subcommands `run-loop`/`create`/`transition`/`comment`/`worklog`/`link`/`attach`; flags: `--change`, `--phase`, `--agent`, `--module`, `--summary`, `--description-file`, `--pr-url`, `--attach`, `--worklog-seconds`, `--config`; credentials from env only (never flags/logs); `service.NewEvidenceLoop` + `loop.Run`; non-zero exit on error. **REQ-AUTH, REQ-CFG, REQ-ERR**
- [ ] 4.2 Capture live TAL board column names — query TAL Jira board and populate the exact ordered state names for each StatusCategory in `Config.OrderedStates`; document in `service/config.go` comment and update placeholder. **REQ-CFG, EC-1** _(blocks integration smoke, not unit tests)_
- [ ] 4.3 Write `adapter/rest/client_integration_test.go` (`//go:build integration`, `t.Skip` when `JIRA_EMAIL`/`JIRA_API_TOKEN` absent): real auth handshake, ADF acceptance, multipart upload, live transition IDs/names on TAL board. **REQ-TEST, REQ-AUTH** _(needs ZEUS Jira token)_
- [ ] 4.4 Add `module:jira-loop → HERMES active` row to `team-context/ownership.md`. **REQ-OWNER**

> **PR 2 ends here.** Branch: `agent/hermes/TAL-<n>-adapter-cmd`. Integration smoke gated behind build tag and env vars.

---

## Notes

- Tasks marked `[P]` (3.2–3.5) are parallel within Phase 3 — ADF builder and transitions resolver are independent.
- Task 4.2 (live TAL column capture) is a prerequisite for task 4.3 only; all unit tasks proceed without it.
- Task 4.3 requires ZEUS's Jira token (`JIRA_EMAIL` + `JIRA_API_TOKEN`). No other task does.
- `service` NEVER imports `adapter/rest`. `cmd/evidence/main.go` is the sole composition root.
- All tests in PR 1 run with zero network calls and no token.
