# Verify Report — jira-evidence-loop · PR1 (domain + service)

**Veredicto: PR1 LISTO.** 0 CRITICAL · 1 WARNING (cerrada) · 2 SUGGESTIONS.
Branch: `agent/hermes/TAL-1-domain-service` → `develop`. Issue: TAL-1.

## Tests
- `go test ./... -count=1` → **32 PASS, 0 fallos** (domain/evidence 17+4 · service 11).
- `go vet ./...` → limpio.

## Cobertura de requisitos (scope PR1)
Satisfechos: REQ-CFG-1/2/3 · REQ-LABEL-1/2/3 · REQ-IDEM-1/2/3/4 · REQ-TRANS-3 · REQ-ERR-1/2/3 · REQ-TEST-1/2/3/4.
Diferido a PR2 (no es falla): REQ-AUTH-3 (header HTTP), REQ-ATTACH multipart, ADF wire format, transitions REST, CLI, smoke de integración.

## Integridad hexagonal
- `domain/evidence`: importa solo `fmt`. `service`: depende solo de `port` (cero refs a adapter). `go.mod`: **cero deps transitivas**. ✅

## OrderedStates (valores reales TAL)
`new:["Por hacer"]` · `indeterminate:["En curso","En revisión","Bloqueado"]` · `done:["Listo"]`. ✅

## Hallazgos
- **WARNING (CERRADA):** la rama de desempate multi-candidato de transición no tenía test → se agregaron 4 tests (`TestDisambiguation_*`); lógica confirmada correcta, sin bug. Commit `da38b3c`.
- **SUGGESTION-1:** `mock` usa un check de interfaz anónimo en vez de `port.JiraClient` directo (trade-off para mantener `mock` libre de dep a `port`). Documentado.
- **SUGGESTION-2:** `.env.example` (resuelto — renombrado del `env.example` inicial).

## Loop de evidencia (manual, ATHENA — dogfooding Fase 1)
- Paso 1 create + labels ✅ (TAL-1, labels con dos puntos OK)
- Paso 2 → En curso ✅ (transición 21)
- Pasos 4-7 (PR link, adjunto, comentario, → En revisión): este PR.

---

# Verify Report — jira-evidence-loop · PR2 (adapter/rest + cmd + smoke)

**Veredicto: PR2 LISTO.** 0 CRITICAL · 2 WARNINGs · 2 SUGGESTIONS.
Branch: `agent/hermes/TAL-1-adapter-cmd` (chained from TAL-1-domain-service). Issue: TAL-1.

## Tests

- `go test ./... -count=1` → **ALL PASS** (adapter/rest: 27 · domain/evidence: 21 · service: 11)
- `go vet ./...` → CLEAN
- `go build -tags=integration ./...` → COMPILES
- `go test -tags=integration -run TestIntegration ./adapter/rest/ -v` → SKIP (env absent, as required by REQ-TEST-5)

## REQ Coverage (PR2 Scope)

| REQ | Status | Key Evidence |
|-----|--------|--------------|
| REQ-AUTH-1 | PASS | cmd: `os.Getenv("JIRA_EMAIL/JIRA_API_TOKEN")`; fail-fast before any API call |
| REQ-AUTH-2 | PASS | `NewConfig()` rejects empty `Credentials.Email`/`APIToken` |
| REQ-AUTH-3 | PASS | `setBasicAuth()` called in `newJSONRequest()` + `AddAttachment()`; `assertBasicAuth()` in all 8 HTTP tests |
| REQ-COMMENT-1 | PASS | `AddComment` sends `{"body": <ADFDocument>}` to `/rest/api/3/issue/{key}/comment` |
| REQ-COMMENT-2 | PASS | `adf.go` paragraph+text+link marks; `adf_test.go` exact golden JSON fixtures |
| REQ-WORKLOG-1 | PASS | `AddWorklog` sends `comment: <ADFDocument>`; client_test asserts non-nil |
| REQ-WORKLOG-2 | PASS | `DurationString()` → "Nh Mm" / "Nh" / "Mm"; 6 test cases in domain |
| REQ-REMOTELINK-1 | PASS | `RemoteLink.GlobalID()` = "pr="+PRURL; domain test asserts exact "pr=..." string |
| REQ-REMOTELINK-2 | PASS | `CreateRemoteLink` sends `relationship`; `NewConfig()` validates non-empty |
| REQ-ATTACH-1 | PASS | `AddAttachment` uses `mime/multipart`; test asserts `mediaType == "multipart/form-data"` |
| REQ-ATTACH-2 | PASS | `req.Header.Set("X-Atlassian-Token", "no-check")`; test asserts exact value |
| REQ-ATTACH-3 | PASS | `checkStatus()` returns typed `HTTPError`; `TestClient_AddAttachment_NonSuccess` (403) asserts error |
| REQ-TRANS-1 | PASS | `GetTransitions` calls `GET /rest/api/3/issue/{key}/transitions`; path asserted |
| REQ-TRANS-2 | PASS | `resolveTransition` filters by `ToCategory` (locale-independent) |
| REQ-TRANS-3 | PASS | category-primary/ordinal-secondary/name-tiebreak; 9 tests incl. 3 ordinals for 3-indeterminate TAL board |
| REQ-TRANS-4 | PASS | `TestResolveTransition_OrdinalOutOfBounds` asserts error on ambiguous |
| REQ-CREATE-1/2/3 | PASS | `CreateIssue` sends issuetype.name + project.key/id; test asserts "Tarea" and "TAL" |
| REQ-IDEM-5 | PASS | `CreateRemoteLink` sends `globalId = link.GlobalID()` = "pr=<url>" |
| REQ-TEST-5 | PASS | `//go:build integration` on line 1; `t.Skip` when env absent; excluded from normal suite |
| REQ-OWNER-1 | PASS | `team-context/ownership.md`: `module:jira-loop \| HERMES \| active` |

## Hexagonal Integrity

- `var _ port.JiraClient = (*Client)(nil)` — compile-time assertion in `adapter/rest/client.go:20`. ✅
- `service/evidence_loop.go` imports: `context`, `fmt`, `strings`, `domain/evidence`, `port`. Zero refs to `adapter`. ✅
- `cmd/evidence/main.go` is the sole composition root (wires `adapter/rest` + `service`). ✅
- `go.mod` has zero transitive deps (no go.sum, `go 1.26` only). ✅

## Security-Sensitive Assertions

| Assertion | Result |
|-----------|--------|
| Basic auth header on every request | `assertBasicAuth()` called in all 8 method tests. ✅ |
| `X-Atlassian-Token: no-check` on attachment | `TestClient_AddAttachment_HappyPath` asserts exact header value. ✅ |
| `multipart/form-data` on attachment | `mime.ParseMediaType` asserts mediaType. ✅ |
| Creds never in flags | `cmd`: env-only; no `-email` / `-token` flags defined. ✅ |

## Findings

**WARNING-1 (wire test depth — globalId + relationship):** `TestClient_CreateRemoteLink_HappyPath` checks `payload["globalId"] != nil` and `payload["relationship"] != nil` but does not assert the actual string values (`"pr=https://..."` and `"is implemented by"`). The `pr=` format is covered at domain level (`TestRemoteLink_GlobalID`). Non-blocking but a defense-in-depth gap at the HTTP wire layer.

**WARNING-2 (integration smoke coverage):** The smoke test covers CreateIssue + Search + GetTransitions + AddComment only. It does NOT smoke `AddWorklog`, `CreateRemoteLink` (step 5 — fail-loud), or `AddAttachment` (step 6 — fail-loud). Acceptable for Fase 1 given token availability. Carry to Fase 2.

**SUGGESTION-1 (DurationString edge):** `DurationString(0)` returns `"0m"`. Jira may reject this. `DefaultWorklogSeconds = 3600` prevents it in practice; add a guard if zero-duration worklogs become possible.

**SUGGESTION-2 (smoke test cleanup):** Each integration run creates a real TAL issue with labels `change:smoke`/`phase:integration`. No cleanup step. Accumulates noise on the board. Recommend a `t.Cleanup` that transitions to Done.

## Full Change Verdict

PR1 + PR2 together cover all 31 REQ IDs from the spec. **Change is ready for `sdd-archive` after both PRs merge.** The 2 WARNINGs are non-blocking for merge; document and carry to Fase 2.
