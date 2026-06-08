# Tasks: overlap-protocol

**Change:** overlap-protocol · **Jira:** TAL-4 · **Module:** module:qa · **Owner:** THEMIS
**Branch:** `agent/themis/TAL-4` (PR1) + `agent/themis/TAL-4-pr2` (PR2, stacked) · **Store:** hybrid
**TDD mode:** Strict — RED before GREEN for every domain/service/adapter behavioral file.

---

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2060 total (PR1 ~1250, PR2 ~810) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 (domain+service+mocks, pure) → PR2 (adapters+cmd+I/O, stacked on PR1) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending (user decision required) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | domain + ports + mocks + service (pure, no I/O) | PR1 | base `develop`; `go test ./...` green in isolation |
| 2 | adapters + cmd/ov + gated edits (I/O) | PR2 | base PR1 branch; integration tests `-short`-skippable |

---

## Phase A — Domain puro (sin I/O) — PR1

> Req: REQ-ERR-1, REQ-VERDICT-1–4, REQ-CHECKLIST-1–5, REQ-METRIC-1–2, REQ-TEST-1–4, REQ-TEST-7–8

- [x] A.1 **RED** `domain/overlap/errors.go` — define `ErrSameFileParallel`, `ErrEscalateZeus`, `ErrChecklistMissing`, `ErrNoClaims`; escribir `errors_test.go` table-driven que verifica cada tipo.
- [x] A.2 **GREEN** Implementar los 4 tipos de error en `errors.go` hasta que `errors_test.go` pase.
- [x] A.3 **RED** `domain/overlap/claim_test.go` — casos: normaliza (trim, dedup, sort), SourceDeclared vs SourceActual, campos obligatorios.
- [x] A.4 **GREEN** `domain/overlap/claim.go` — `ClaimSource`, `Claim`, `NewClaim` (normalización determinista).
- [x] A.5 **RED** `domain/overlap/parse_test.go` — casos nombrados: marker ausente, case-insensitive, `[ ]`/`[x]`/`[X]`, backtick strip, sección termina en non-item, fallback→`ErrChecklistMissing`, cero items→`ErrChecklistMissing`. Cubre REQ-TEST-2.
- [x] A.6 **GREEN** `domain/overlap/parse.go` — `ParseFilesChecklist(body string) []string` con gramática exacta del spec.
- [x] A.7 **RED** `domain/overlap/jql_test.go` — verifica JQL verbatim de REQ-CHECK-1 para distintos valores de project/module/owner.
- [x] A.8 **GREEN** `domain/overlap/jql.go` — `BuildPreAssignmentJQL(project, module, owner string) string`.
- [x] A.9 **RED** `domain/overlap/collision_test.go` — casos: distinto-agente comparte archivo→colisión; mismo-agente→no colisión (REQ-TEST-8); pairwise completo; orden determinista (REQ-VERDICT-3). `ModuleOverlaps` excluye pares ya en FileCollisions.
- [x] A.10 **GREEN** `domain/overlap/collision.go` — `FileCollision`, `FileCollisions([]Claim)`, `ModuleOverlap`, `ModuleOverlaps([]Claim)`.
- [x] A.11 **RED** `domain/overlap/report_test.go` — casos: solo colisiones→BLOCK; solo overlaps→SERIALIZE; ninguno→OK; ambos→BLOCK; CollisionRate 0 (P=0), 0.0 (C=0), 0.15→false, 0.150001→true, 1.0 (REQ-TEST-3–4). Cubre REQ-METRIC-1–2.
- [x] A.12 **GREEN** `domain/overlap/report.go` — `Verdict`, `Report`, `NewReport(claims []Claim, threshold float64) Report` (precedencia BLOCK>SERIALIZE>OK, strictly-greater, CollisionRate seguro P=0).

---

## Phase B — Ports + Mocks — PR1

> Req: REQ-TEST-5, REQ-TEST-9, REQ-READINESS-1, REQ-CHECK-5, REQ-SCAN-8

- [x] B.1 `port/issue_searcher.go` — `IssueResult{Key,Labels,Body string}`, `IssueSearcher` interface con `Search(ctx, jql string, maxResults int)([]IssueResult, error)`.
- [x] B.2 `port/worktree_lister.go` — `WorktreeEntry{Figura,Branch,Path,Head,Status string}` con tags JSON; `WorktreeLister` interface con `List(ctx)([]WorktreeEntry, error)`.
- [x] B.3 `port/git_inspector.go` — `GitInspector` interface con `Fetch(ctx) error`, `RevParse(ctx, ref string)(string, error)`, `ChangedFiles(ctx, base, branch string)([]string, error)`.
- [x] B.4 `mock/issue_searcher_mock.go` — `IssueSearcherMock` con `Calls []Call`, result map; `var _ port.IssueSearcher = (*IssueSearcherMock)(nil)`.
- [x] B.5 `mock/worktree_lister_mock.go` — `WorktreeListerMock` análogo; `var _ port.WorktreeLister = (*WorktreeListerMock)(nil)`.
- [x] B.6 `mock/git_inspector_mock.go` — `GitInspectorMock` con `AssertNotCalled` (Fetch omitido en check, REQ-TEST-9); `var _ port.GitInspector = (*GitInspectorMock)(nil)`.

---

## Phase C — Service (vía mocks, sin I/O real) — PR1

> Req: REQ-CHECK-1–6, REQ-SCAN-1–8, REQ-METRIC-1–6, REQ-READINESS-2–5, REQ-TEST-5–6, REQ-TEST-9

- [x] C.1 `service/config.go` — `Config{RepoRoot,BaseBranch,WtBinary,SiteURL,Project string; Threshold float64; NoFetch bool}`, `DefaultTALConfig()` (base develop, wt, TAL, 0.15).
- [x] C.2 **RED** `service/guard_test.go` bloque `ScanInFlight` — casos: filtra solo `active` (REQ-READINESS-2), vacío→`ErrNoClaims` (REQ-READINESS-3), `--no-fetch` omite `Fetch` (REQ-FETCH-2, REQ-TEST-9), pairwise distintos-agente→BLOCK, mismo-agente→OK (REQ-SCAN-2).
- [x] C.3 **GREEN** `service/guard.go` — `Guard`, `NewGuard`, `ScanInFlight(ctx)(overlap.Report, error)`.
- [x] C.4 **RED** `service/guard_test.go` bloque `CheckPreAssignment` — casos: JQL verbatim (REQ-CHECK-1), `ErrChecklistMissing` advisory+módulo kept (REQ-CHECKLIST-4), file-collision→BLOCK, disjoint+overlap→SERIALIZE, sin solapamiento→OK; git no llamado (REQ-CHECK-6, REQ-TEST-9).
- [x] C.5 **GREEN** `Guard.CheckPreAssignment(ctx, module, owner string, ownerFiles []string)(overlap.Report, error)`.
- [x] C.6 **RED** `service/guard_test.go` bloque `Metric` — verifica que delega a `ScanInFlight` y devuelve el mismo `Report`; `--strict` y threshold se leen de Config.
- [x] C.7 **GREEN** `Guard.Metric(ctx)(overlap.Report, error)`.

> **Cierra PR1** — `go test ./...` sin I/O real. Verificación: tabla-driven, 0 FAIL, todos los paths de REQ-TEST-1–9 cubiertos.

---

## Phase D — Adapters (I/O real, `-short`-skippable) — PR2

> Req: REQ-JIRA-1–4, REQ-READINESS-1–5, REQ-SCAN-1, REQ-FETCH-1–2, REQ-TEST-6

- [x] D.1 **RED** `adapter/jirarest/client_test.go` — `httptest.Server` con respuesta ADF; verifica que el adapter pide `["summary","labels","description"]`, aplana ADF (depth-first, join `\n`), retorna `IssueResult` con `Body` plano; tests de I/O reales gateados con `testing.Short()`.
- [x] D.2 **GREEN** `adapter/jirarest/client.go` — `Client` (auth via env/flags `--site-url`/`--token`), `Search(ctx, jql, max)`, `adfNode{Type,Text string; Content []adfNode}` local (walk DFS), implementa `port.IssueSearcher`; `var _ port.IssueSearcher = (*Client)(nil)`.
- [x] D.3 **RED** `adapter/wtcli/lister_test.go` — mock del ejecutable + JSON bien formado; casos: JSON malformado→`ErrWtOutputMalformed`, binario ausente→`ErrWtBinaryNotFound`; tests reales gateados con `testing.Short()`.
- [x] D.4 **GREEN** `adapter/wtcli/lister.go` — `Lister` que shella `wt list --json`, parsea al DTO `port.WorktreeEntry`; `var _ port.WorktreeLister = (*Lister)(nil)`.
- [x] D.5 **RED** `adapter/gitcli/inspector_test.go` — casos: `ChangedFiles` formato correcto, `RevParse` HEAD, `Fetch` invocado/omitido; tests reales gateados con `testing.Short()`.
- [x] D.6 **GREEN** `adapter/gitcli/inspector.go` — auto-contenido (~50 ln): `Fetch`, `RevParse`, `ChangedFiles`; `var _ port.GitInspector = (*Inspector)(nil)`. NO importar `merge-order-orchestrator` (ADR-OV1).

---

## Phase E — cmd/ov (composition root) — PR2

> Req: REQ-ERR-3–4, REQ-OUTPUT-1–4, REQ-METRIC-4–5, REQ-CHECK-3–4

- [x] E.1 **RED** `cmd/ov/main_test.go` — `run(args)` nivel: switch check/scan/metric, `exitCodeFor` (ErrNoClaims→0, ErrSameFileParallel→1, OverThreshold&&--strict→1), `--json` produce JSON válido a stdout, errores a stderr (REQ-OUTPUT-3–4).
- [x] E.2 **GREEN** `cmd/ov/main.go` — `run([]string) error`, switch manual `check`/`scan`/`metric`, `flag.NewFlagSet` por subcomando, `--json` via `json.NewEncoder(os.Stdout).SetIndent("","  ")`, `exitCodeFor`, `repoRoot()` via `RevParse`, composition root wire `jirarest`/`wtcli`/`gitcli` → `Guard`.

---

## Phase F — Edits compartidos + bootstrap — PR2

> Req: REQ-CLEANUP-1–2, REQ-READINESS-1 (go.mod)

- [x] F.1 `platform/overlap-guard/go.mod` — módulo `github.com/John-Santa/talos/platform/overlap-guard`, `go 1.26`, zero deps externos.
- [x] F.2 `team-context/ownership.md` — agregar fila `platform/overlap-guard/**` → THEMIS / module:qa / active en "Mapa archivo → módulo".
- [x] F.3 `.gitignore` — agregar `/platform/overlap-guard/ov` (binary nunca commiteado; REQ-CLEANUP-2).
- [x] F.4 Verificar `team-context/merge-order.md` referencia `main`→`develop`: si ya está limpio (fijado por #2 / TAL-3), NO modificar.

---

## Dependency Graph

```
A (domain) ──► B (ports+mocks) ──► C (service)  ← cierra PR1
                                        │
                                        ▼
                              D (adapters) ──► E (cmd/ov) ──► F (bootstrap)  ← cierra PR2
```

Fases A→B→C son estrictamente secuenciales (domain antes que mocks, mocks antes que service).
Fases D y E son paralelas entre sí (adapters no dependen de cmd, cmd puede desarrollarse contra mocks).
Fase F es independiente y puede hacerse antes o durante PR2.

---

## Resumen de tasks por fase

| Fase | PR | Items | Secuencia |
|------|----|-------|-----------|
| A — Domain | PR1 | 12 (6 RED+GREEN pairs + errors + parse + jql) | Secuencial |
| B — Ports+Mocks | PR1 | 6 | Secuencial post-A |
| C — Service | PR1 | 7 (3 RED+GREEN pairs + Config) | Secuencial post-B |
| D — Adapters | PR2 | 6 (3 RED+GREEN pairs) | Paralelo con E |
| E — cmd/ov | PR2 | 2 (RED+GREEN) | Paralelo con D |
| F — Bootstrap | PR2 | 4 | Independiente |
| **Total** | | **37** | |
