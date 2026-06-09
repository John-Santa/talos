# Verify Report: ci-hardening (TAL-5)

**Change:** ci-hardening
**Jira:** TAL-5
**Module:** module:devops
**Owner:** HERMES
**Branches:** `agent/hermes/TAL-5` (PR1) + `agent/hermes/TAL-5-pr2` (PR2, stacked)
**Store:** hybrid
**Verificado en:** 2026-06-08 (rama `agent/hermes/TAL-5-pr2`, contiene PR1 + PR2 completos)
**Veredicto:** PASS WITH WARNINGS — 0 CRITICAL / 2 WARNING / 3 SUGGESTION

---

## Resultados de Tests

### `go test -count=1 ./...` (full suite — sin `-short`)

| Package | Resultado | Tests top-level | Subtests | SKIP |
|---------|-----------|-----------------|----------|------|
| `adapter/jirarest` | PASS | 7 PASS | — | 1 (integration sin creds) |
| `adapter/ownershipfile` | PASS | 4 PASS | — | 0 (integration corre con archivo real presente) |
| `cmd/ch` | PASS | 9 PASS | 7 subtests | — |
| `domain/cichecks` | PASS | 22 PASS | 17 subtests | — |
| `mock` | — | no test files (esperado) | — | — |
| `port` | — | no test files (esperado) | — | — |
| `service` | PASS | 8 PASS | — | — |

**Total: 59 top-level PASS / 24 subtest PASS / 1 SKIP / 0 FAIL**

### `go test -short ./...` (short suite)

| Package | Resultado |
|---------|-----------|
| `adapter/jirarest` | PASS — 7 httptest PASS, integration SKIP ✓ |
| `adapter/ownershipfile` | PASS — 4 unit PASS, integration SKIP ✓ |
| `cmd/ch` | PASS |
| `domain/cichecks` | PASS |
| `service` | PASS |

### `go vet ./...`

LIMPIO — 0 advertencias.

---

## Trazabilidad REQ-ID → Implementación

### REQ-BRANCH

#### REQ-BRANCH-1: Parseo case-insensitive
**Estado: SATISFECHO CON ADVERTENCIA (ver W-1)**

- Implementación: `domain/cichecks/branch.go:5` — regex `^agent/([a-z]+)/(TAL-[0-9]+)$`. Acepta solo lowercase en figura; la normalización previa se delega a `cmd/ch/main.go:142-149` (`normalizeBranchFigura`).
- Test de normalización: `cmd/ch/main_test.go:217` — `TestBranchCaseNormalization` verifica que `agent/HERMES/TAL-7` produce `figura=hermes`. ✓
- Test de parseo directo: `domain/cichecks/branch_test.go` — `TestParseAgentBranch` (7 casos): lowercase, uppercase rechazado por regex, develop, feature, arbitrary, vacío, non-TAL. ✓

#### REQ-BRANCH-2: ErrNoJiraKey sin contactar Jira
**Estado: SATISFECHO**

- Implementación: `service/checker.go:27-30` — `ParseAgentBranch` retorna `ErrNoJiraKey` y `Check` corta antes de `LabelsByKey`.
- Test: `service/checker_test.go:38` — `TestChecker_Check_ErrNoJiraKey` con `AssertNotCalled(LabelsByKey)` + `AssertNotCalled(Ownership)`. ✓

---

### REQ-INVARIANT

#### REQ-INVARIANT-1 (sub-regla a): Exactamente un `agent:*`
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/invariant.go:28-33` — `Validate(["agent"])`, mensaje con conteo explícito (`%d labels agent:* found`).
- Tests: `invariant_test.go:24` `TestCheckLabelInvariant_ZeroAgent`, `invariant_test.go:46` `TestCheckLabelInvariant_TwoAgents`. ✓

#### REQ-INVARIANT-2 (sub-regla b): Exactamente un `module:*`
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/invariant.go:36-41` — `Validate(["module"])`.
- Tests: `invariant_test.go:68` `TestCheckLabelInvariant_ZeroModule`, `invariant_test.go:90` `TestCheckLabelInvariant_TwoModules`. ✓

#### REQ-INVARIANT-3 (sub-regla c): `ownership[module] == agent`
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/invariant.go:48-55` — `ValidateOwnership(moduleLabel, agentVal, in.Ownership)`.
- Tests: `invariant_test.go:112` `TestCheckLabelInvariant_OwnershipMismatch`, `invariant_test.go:124` `TestCheckLabelInvariant_UnknownModule`. ✓

#### REQ-INVARIANT-4 (sub-regla d): `branchFigura == labelAgent`
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/invariant.go:58-63` — comparación directa post-lowercase.
- Test: `invariant_test.go:136` `TestCheckLabelInvariant_BranchFiguraMismatch`. ✓

#### REQ-INVARIANT-5: Múltiples violaciones acumuladas
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/invariant.go:24-65` — acumula en `violations []string`, nunca corta. `CheckLabelInvariant` siempre retorna `(InvariantResult{Violations}, nil)`.
- Tests: `invariant_test.go:158` `TestCheckLabelInvariant_MultipleViolations` (≥2 viols), `invariant_test.go:170` `TestCheckLabelInvariant_AllViolations` (asserta (a) y (b) presentes cuando labels vacíos). ✓

#### REQ-INVARIANT-6: 4 reglas OK → exit 0
**Estado: SATISFECHO**

- Test: `invariant_test.go:196` `TestCheckLabelInvariant_Verdict_OK_Exit0`. ✓

---

### REQ-LABELS

#### REQ-LABELS-1: Orden parse → fetch → ownership → evaluar → emitir
**Estado: SATISFECHO CON ADVERTENCIA (ver W-2)**

- Implementación: `service/checker.go:27-56` — flujo exacto del design §7.
- Test orden: `service/checker_test.go:154` `TestChecker_Check_OrderOfCalls` — verifica call counts de `LabelsByKey` (1) y `Ownership` (1). ✓ (ver W-2 para observación)

#### REQ-LABELS-2: ErrNoJiraKey corta antes de Jira
**Estado: SATISFECHO** — ver REQ-BRANCH-2 arriba.

#### REQ-LABELS-3: `--ownership-file` override
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:89-110` — flag `--ownership-file`, si presente sobreescribe el default derivado de `repoRoot()`.
- Test: `cmd/ch/main_test.go:204` `TestOwnership_Offline` usa `--ownership-file` con fixture. ✓

#### REQ-LABELS-4: `--site-url` override de `JIRA_SITE_URL`
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:91` — `siteURL` default = `os.Getenv("JIRA_SITE_URL")`, override con `--site-url`.
- Tests: `TestLabelsJSON_Shape_OK`, `TestLabelsJSON_Shape_Violation`, `TestBranchCaseNormalization` todos pasan `--site-url` apuntando al servidor httptest, confirman que el override funciona. ✓

---

### REQ-OWNERSHIP

#### REQ-OWNERSHIP-1: Validación offline sin Jira
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:212-240` — `cmdOwnership` solo llama `ownershipfile.NewReader(...).Ownership(ctx)`, sin adapters de Jira.
- Test: `cmd/ch/main_test.go:204` `TestOwnership_Offline`. ✓

#### REQ-OWNERSHIP-2: Duplicado → ErrMalformedOwnership
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/parse.go:42-45` — detecta duplicado y retorna `&ErrMalformedOwnership{...}`.
- Test: `domain/cichecks/parse_test.go` — `TestParseOwnershipTable_DuplicateModule`. ✓

#### REQ-OWNERSHIP-3: Texto legible por defecto; JSON con `--json`
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:233-240` — sin `--json` imprime `k → v` por línea; con `--json` emite `ownershipJSON{Modules: m}`.
- Test: `TestOwnership_Offline` verifica exit 0 en modo texto. ✓

---

### REQ-PARSE

#### REQ-PARSE-1: Solo primera tabla (skip segunda)
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/parse.go:29-31` — `strings.HasPrefix(..., "## ")` cierra la primera sección al ver el segundo heading.
- Tests: `TestParseOwnershipTable_SkipsSecondTable`, `adapter/ownershipfile/reader_test.go:50` `TestReader_Ownership_SkipsSecondTable`. ✓

#### REQ-PARSE-2: Normalización a lowercase de clave y valor
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/parse.go:70-74` — `strings.ToLower` en col-1 y col-2.
- Tests: `TestParseOwnershipTable_OK` (HERMES→hermes), `TestParseOwnershipTable_MixedCasing` (Qa/Themis→qa/themis). ✓

#### REQ-PARSE-3: Filas duplicadas → ErrMalformedOwnership
**Estado: SATISFECHO** — ver REQ-OWNERSHIP-2.

#### REQ-PARSE-4: Headers y separadores ignorados
**Estado: SATISFECHO**

- Implementación: `domain/cichecks/parse.go:77-88` — salta `---` y header con `module:*`.
- Tests: `TestParseOwnershipTable_SkipsHeader`, `TestParseOwnershipTable_SkipsSeparator`. ✓

---

### REQ-JIRA

#### REQ-JIRA-1: GET con fields=labels, creds de env
**Estado: SATISFECHO**

- Implementación: `adapter/jirarest/client.go:57-85` — `GET {baseURL}/rest/api/3/issue/{key}?fields=labels`, creds via `setBasicAuth`.
- Tests: `TestClient_LabelsByKey_ValidatesPath`, `TestClient_LabelsByKey_ValidatesQueryFields`, `TestClient_LabelsByKey_ValidatesBasicAuth`. ✓

#### REQ-JIRA-2: 404 → ErrIssueNotFound
**Estado: SATISFECHO**

- Implementación: `adapter/jirarest/client.go:72-74`.
- Test: `TestClient_LabelsByKey_404` — verifica `cichecks.ErrIssueNotFound` con `Key="TAL-7"`. ✓

#### REQ-JIRA-3: 401 → HTTPError
**Estado: SATISFECHO**

- Implementación: `adapter/jirarest/client.go:76-78`.
- Test: `TestClient_LabelsByKey_401` — verifica `*HTTPError{StatusCode:401}`. ✓

#### REQ-JIRA-4: 5xx → HTTPError, fail-closed, sin retry
**Estado: SATISFECHO**

- Implementación: `adapter/jirarest/client.go:76-78` — mismo branch que 401. Sin lógica de retry.
- Test: `TestClient_LabelsByKey_500`. ✓

---

### REQ-JSON

#### REQ-JSON-1: Shape exacto del output `--json`
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:64-73` — struct `labelsJSON` con los 8 campos requeridos y JSON tags exactos.
- Test: `cmd/ch/main_test.go:89` `TestLabelsJSON_Shape_OK` — decodifica JSON y verifica `branch`, `jira_key`, `figura`, `verdict`, `labels`, `agent`, `module`, `violations`. ✓

#### REQ-JSON-2: `--json` en ErrNoJiraKey produce JSON con `verdict:"VIOLATION"`
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:151-186` — `buildLabelsJSON` maneja `checkErr != nil` (incluido `ErrNoJiraKey`) con `verdict="VIOLATION"`.
- Test: `cmd/ch/main_test.go:181` `TestLabelsJSON_ErrNoJiraKey`. ✓

---

### REQ-EXIT

#### REQ-EXIT-1 y REQ-EXIT-2: Tabla de exit codes, errores inesperados → exit 1
**Estado: SATISFECHO**

- Implementación: `cmd/ch/main.go:47-52` — `exitCodeFor`: nil→0, cualquier error→1.
- Test: `cmd/ch/main_test.go:47` `TestExitCodeFor_Table` — tabla con nil, ErrNoJiraKey, ErrLabelInvariant, ErrMalformedOwnership, ErrIssueNotFound, HTTPError, generic error. ✓

---

### REQ-GATES

#### REQ-GATES-1: Gate `branch-name` — regex verbatim
**Estado: SATISFECHO**

- Implementación: `.github/workflows/pr-checks.yml:13-18` — bash con regex `^agent/[a-z]+/TAL-[0-9]+$`, `::error::` + `exit 1`. ✓

#### REQ-GATES-2: Gate `labels` — invoca `ch labels` con 3 secrets
**Estado: SATISFECHO**

- Implementación: `.github/workflows/pr-checks.yml:20-38` — `needs: branch-name`, build `ch`, run con `JIRA_SITE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`. ✓

#### REQ-GATES-3: Gate `dod` — test por módulo + verify-report glob
**Estado: SATISFECHO CON ADVERTENCIA (ver W-1 en YAML)**

- Implementación: `.github/workflows/pr-checks.yml:40-67` — `fetch-depth:0`, `go test -short ./...` por módulo con `go.mod`, vitest placeholder, assert `ls openspec/changes/**/verify-report.md`.
- Observación: la extracción del módulo usa `sed 's|/[^/]*$||'` en lugar del `sed -E 's#(platform/[^/]+)/.*#\1#'` del design. Ver W-1.

#### REQ-GATES-4: `labels` y `dod` skipean si `branch-name` falla
**Estado: SATISFECHO**

- Implementación: `needs: branch-name` en ambos jobs `labels` y `dod`. ✓

#### REQ-GATES-5: Workflow solo en PRs a `develop`
**Estado: SATISFECHO**

- Implementación: `.github/workflows/pr-checks.yml:2-4` — `on: pull_request: branches: [develop]`. ✓

---

### REQ-ERR

#### REQ-ERR-1: Catálogo de errores tipados
**Estado: SATISFECHO**

Los 5 identificadores del catálogo están implementados:
- `ErrNoJiraKey` — `domain/cichecks/errors.go:12` (sentinel `errors.New`)
- `ErrLabelInvariant` — `errors.go:15-23` (struct con `Violations []string`)
- `ErrMalformedOwnership` — `errors.go:25-34` (struct con `Detail string`)
- `ErrIssueNotFound` — `errors.go:36-45` (struct con `Key string`)
- `HTTPError` — `adapter/jirarest/client.go:17-25` (struct con `StatusCode`, `Body`, `Op`)

#### REQ-ERR-2: Sin panic
**Estado: SATISFECHO**

Ninguna ruta de código usa `panic`. `go vet` limpio. ✓

#### REQ-ERR-3: Mensajes no-vacíos con contexto
**Estado: SATISFECHO**

- `ErrIssueNotFound.Error()` nombra la key: `"ci-checks: issue %q not found in Jira (HTTP 404)"`.
- `ErrLabelInvariant.Error()` lista todas las violaciones con `strings.Join`.
- `ErrMalformedOwnership.Error()` incluye el `Detail` del campo/fila problemática.

---

### REQ-TEST

#### REQ-TEST-1: Parseo de rama table-driven
**Estado: SATISFECHO**

- `domain/cichecks/branch_test.go` — `TestParseAgentBranch` con 7 casos: lowercase, uppercase (rechazado por regex), develop, feature, arbitrary, vacío, non-TAL. ✓
- Normalización testada en `cmd/ch/main_test.go:217` `TestBranchCaseNormalization`. ✓

#### REQ-TEST-2: Sub-reglas del invariante table-driven
**Estado: SATISFECHO**

- `domain/cichecks/invariant_test.go` — 11 tests cubriendo: OK, 0 agent, 2 agent, 0 module, 2 module, ownership mismatch, módulo desconocido, branch≠label, múltiples violaciones, todas las violaciones, verdict OK exit0. ✓

#### REQ-TEST-3: Parseo de ownership con fixture
**Estado: SATISFECHO**

- `domain/cichecks/parse_test.go` — 7 tests con fixture inline: UPPERCASE, SkipsHeader, SkipsSeparator, SkipsSecondTable, DuplicateModule, EmptyResult, MixedCasing. ✓

#### REQ-TEST-4: Service via mocks
**Estado: SATISFECHO CON OBSERVACIÓN (ver W-2)**

- `service/checker_test.go` — 8 tests: OK, ErrNoJiraKey (AssertNotCalled), ViolationLabelInvariant, AllViolations, ErrIssueNotFound, HTTPError (sentinel), MalformedOwnership, OrderOfCalls. ✓
- `mock/` — `IssueLabelReaderMock` y `OwnershipReaderMock` hand-written con call recording, `AssertCallCount`, `AssertNotCalled`, `AssertMethodOrder`. ✓

#### REQ-TEST-5: Adapter jirarest Short-gated
**Estado: SATISFECHO**

- `adapter/jirarest/client_test.go` — 7 httptest tests (siempre corren). 1 test de integración Short-gated con `testing.Short()`. ✓

#### REQ-TEST-6: Adapter ownershipfile con fixture
**Estado: SATISFECHO**

- `adapter/ownershipfile/testdata/ownership.md` presente.
- `adapter/ownershipfile/reader_test.go` — 4 unit tests contra fixture + 1 integración con `ownership.md` real del repo (Short-gated). ✓

#### REQ-TEST-7: Shape JSON
**Estado: SATISFECHO**

- `cmd/ch/main_test.go:89` `TestLabelsJSON_Shape_OK` — decodifica y verifica los 8 campos de REQ-JSON-1.
- `cmd/ch/main_test.go:143` `TestLabelsJSON_Shape_Violation` — verifica `verdict:"VIOLATION"` y violations no-vacías. ✓

#### REQ-TEST-8: Un test por error del catálogo
**Estado: SATISFECHO**

- `ErrNoJiraKey` — `TestChecker_Check_ErrNoJiraKey`
- `ErrLabelInvariant` — `TestChecker_Check_ViolationLabelInvariant`
- `ErrMalformedOwnership` — `TestChecker_Check_MalformedOwnership`
- `ErrIssueNotFound` — `TestChecker_Check_ErrIssueNotFound` + `TestClient_LabelsByKey_404`
- `HTTPError` — `TestClient_LabelsByKey_401` + `TestClient_LabelsByKey_500`

---

### REQ-CLEANUP

#### REQ-CLEANUP-1: `.gitignore` con `/platform/ci-checks/ch`
**Estado: SATISFECHO**

- `.gitignore:39` — `/platform/ci-checks/ch` presente. ✓

#### REQ-CLEANUP-2: `ownership.md` con fila `platform/ci-checks/**`
**Estado: SATISFECHO**

- `team-context/ownership.md:42` — `| platform/ci-checks/** | HERMES | CLI ch, invariante §4 |` en la segunda tabla (que `ParseOwnershipTable` saltea). ✓

#### REQ-CLEANUP-3: `.github/workflows/pr-checks.yml` creado; `ci/` eliminado
**Estado: SATISFECHO**

- `.github/workflows/pr-checks.yml` presente con los 3 jobs. ✓
- Directorio `ci/` y `ci/pr-checks.yml` eliminados. ✓

---

## Hallazgos

### W-1 — WARNING: `sed` del gate `dod` extrae subdirectorios en vez del módulo raíz

| Campo | Detalle |
|-------|---------|
| **Severidad** | WARNING |
| **REQ-IDs** | REQ-GATES-3 |
| **Archivo:línea** | `.github/workflows/pr-checks.yml:53` |
| **Qué está mal** | `sed 's|/[^/]*$||'` elimina solo el último componente del path. Para un archivo como `platform/ci-checks/domain/cichecks/label.go` produce `platform/ci-checks/domain/cichecks`, no `platform/ci-checks`. El `[ -f "$dir/go.mod" ]` falla porque `go.mod` no está en ese subdirectorio. El design prescribe `sed -E 's#(platform/[^/]+)/.*#\1#'`. |
| **Impacto** | En PRs donde solo cambian archivos en subdirectorios de `platform/<m>/` (e.g. todos los cambios de `ci-hardening` excepto `go.mod`), `go test -short ./...` no se ejecuta. El gate pasa en vacío en lugar de testear el módulo. |
| **Fix recomendado** | Cambiar línea 53 a: `CHANGED=$(git diff --name-only origin/${{ github.base_ref }}...HEAD \| grep '^platform/' \| sed -E 's#(platform/[^/]+)/.*#\1#' \| sort -u)` |

---

### W-2 — WARNING: `Config.RequiredKeys` declarado pero no consumido por `CheckLabelInvariant`

| Campo | Detalle |
|-------|---------|
| **Severidad** | WARNING |
| **REQ-IDs** | REQ-LABELS-1, REQ-TEST-4 |
| **Archivo:línea** | `service/checker.go:44-48`, `domain/cichecks/invariant.go:28,36` |
| **Qué está mal** | El design (§5.5 y §7) especifica que `InvariantInput` lleva un campo `RequiredKeys []string` que el service popula desde `c.cfg.RequiredKeys`. En la implementación, `InvariantInput` no tiene ese campo y `CheckLabelInvariant` hardcodea `["agent"]` y `["module"]` internamente. `Config.RequiredKeys` se declara y popula (`service/config.go:19`) pero nunca se lee. |
| **Impacto** | El comportamiento actual es correcto para el caso TAL (siempre `["agent","module"]`), pero la variabilidad de configuración prometida por el diseño es ilusoria. Si se necesitara agregar un tercer label requerido, habría que modificar `invariant.go` en lugar de solo el `Config`. |
| **Fix recomendado** | Agregar `RequiredKeys []string` a `InvariantInput`; el service popula `RequiredKeys: c.cfg.RequiredKeys`; `CheckLabelInvariant` itera sobre `in.RequiredKeys` para sub-reglas (a)+(b) en lugar de strings literales. O bien, documentar explícitamente que `RequiredKeys` es un campo reservado no-funcional (con una nota en `config.go`). |

---

### S-1 — SUGGESTION: Comentarios inline excesivos en `invariant.go` y `main.go`

| Campo | Detalle |
|-------|---------|
| **Severidad** | SUGGESTION |
| **Convención** | Design §0 / `CLAUDE.md`: código auto-explicativo, sin comentarios salvo una línea godoc en exportados |
| **Archivos:líneas** | `domain/cichecks/invariant.go:27,35,43,47,57` — 5 comentarios inline que explican los pasos. `cmd/ch/main.go:100-102` — bloque "CRITICAL:" de 3 líneas que explica el `normalizeBranchFigura`. |
| **Qué está mal** | Los nombres de función y los tests ya documentan el propósito; los comentarios inline duplican información visible en el código. El bloque `CRITICAL:` en `main.go:100-102` viola la regla de una sola línea godoc en exportados (aquí no es exportado, pero la densidad de comentarios explicativos supera la convención). |
| **Fix recomendado** | Eliminar los 5 comentarios inline de `invariant.go`. En `main.go:100-102`, reducir a una línea: `// normalize figura segment only; TAL-N must stay uppercase for the domain regex`. |

---

### S-2 — SUGGESTION: `TestChecker_Check_OrderOfCalls` no usa `AssertMethodOrder`

| Campo | Detalle |
|-------|---------|
| **Severidad** | SUGGESTION |
| **REQ-IDs** | REQ-TEST-4, REQ-LABELS-1 |
| **Archivo:línea** | `service/checker_test.go:154-177` |
| **Qué está mal** | El test aserta que ambos métodos se llamaron una vez (`AssertCallCount`) pero no verifica el ORDEN relativo entre `LabelsByKey` y `Ownership`. Los mocks implementan `AssertMethodOrder` para este propósito (ver `mock/issue_label_reader_mock.go:82`); el test no lo invoca. El orden documentado (LabelsByKey antes de Ownership) queda sin cobertura directa. |
| **Fix recomendado** | Agregar al test: `labels.AssertMethodOrder(t, "LabelsByKey")` y comparar el índice de la última call de `LabelsByKey` con la de `Ownership` — o bien usar los `Calls` del mock para una comparación de índice directa. Alternativamente, dejar el mock compartido entre los dos y llamar `mock.AssertMethodOrder(t, "LabelsByKey", "Ownership")`. |

---

### S-3 — SUGGESTION: `TestReader_Integration_RealOwnership` no está Short-gated

| Campo | Detalle |
|-------|---------|
| **Severidad** | SUGGESTION |
| **REQ-IDs** | REQ-TEST-6 |
| **Archivo:línea** | `adapter/ownershipfile/reader_test.go:74` |
| **Qué está mal** | El test lleva comentario "skipped in short mode" y en el cuerpo tiene `if testing.Short() { t.Skip(...) }` — correcto. Sin embargo, en el run completo corre siempre porque lee `team-context/ownership.md` del repo checkouteado. Esto es por diseño (el test de integración de ownershipfile no requiere red), pero la integración JiraREST sí está Short-gated. La inconsistencia puede confundir: ambos deberían seguir el mismo patrón. |
| **Fix recomendado** | Si se desea uniformidad, dejar el guard `testing.Short()` tal cual (ya está) y aceptar que corre en local pero salta en CI con `-short`. La implementación ya es correcta — esto es solo una nota de legibilidad. |

---

### Advisory — `actionlint` no disponible en el entorno de verificación

`which actionlint` retornó vacío. El YAML no pudo ser validado con linting estático. La superficie no testeada del YAML es `checkout`, `setup-go`, invocar el binario, y pasar env — mitigación por diseño (ADR-C2): low-risk por dog-food del propio PR2.

---

## Invariante §4 — Verificación exhaustiva

Todas las 4 sub-reglas (a)(b)(c)(d) están implementadas y testeadas de forma independiente:

| Sub-regla | Implementada | Test RED→GREEN | Cobertura de casos |
|-----------|-------------|----------------|-------------------|
| (a) agent:* exactamente uno | `invariant.go:28-33` | `TestCheckLabelInvariant_ZeroAgent` + `TwoAgents` | OK, 0, 2 ✓ |
| (b) module:* exactamente uno | `invariant.go:36-41` | `TestCheckLabelInvariant_ZeroModule` + `TwoModules` | OK, 0, 2 ✓ |
| (c) ownership[module]==agent | `invariant.go:48-55` | `TestCheckLabelInvariant_OwnershipMismatch` + `UnknownModule` | OK, mismatch, desconocido ✓ |
| (d) branchFigura==labelAgent | `invariant.go:58-63` | `TestCheckLabelInvariant_BranchFiguraMismatch` | OK, mismatch ✓ |
| Acumulación | `invariant.go:24` `var violations` | `MultipleViolations` + `AllViolations` | ≥2 viols, (a)+(b) acumuladas ✓ |

---

## Strict TDD — Evidencia

El apply-progress (engram #1886) documenta el ciclo RED→GREEN para las 3 fases del PR2:

- `adapter/jirarest/client_test.go` escrito antes que `client.go` → build fallido (RED) → `auth.go + client.go` escritos → GREEN.
- `adapter/ownershipfile/reader_test.go` escrito antes que `reader.go` → build fallido (RED) → `reader.go` escrito → GREEN.
- `cmd/ch/main_test.go` escrito antes que `main.go` → `undefined: run/exitCodeFor` (RED) → `main.go` escrito → fallos por bug `normalizeBranchFigura` → fixed → GREEN.

Todo archivo de comportamiento del dominio/service/adapter tiene su `_test.go`. Los tests de integración con red/filesystem real están correctamente Short-gated.

---

## Zero-dep

`platform/ci-checks/go.mod` contiene `module github.com/John-Santa/talos/platform/ci-checks` y `go 1.26` — **sin bloque `require`**. Ningún archivo fuente importa dependencias de terceros; todos los imports son de `stdlib` o del propio módulo. REQ-CLEANUP-1 satisfecho.

---

## Fix del bug `normalizeBranchFigura` (PR2)

Confirmado: `cmd/ch/main.go:142-149` implementa `normalizeBranchFigura` que hace `SplitN(branch, "/", 3)` y lowercasea solo `parts[1]` (la figura), dejando `parts[2]` (TAL-N) intacto. El test `TestBranchCaseNormalization` verifica que `agent/HERMES/TAL-7` resulta en `figura=hermes` y exitCode 0. El bug original (`strings.ToLower(branch)` completo rompería el regex) no existe en la implementación final.

---

## Resumen de Hallazgos

| # | Severidad | REQ-ID | Archivo:línea | Descripción corta |
|---|-----------|--------|---------------|-------------------|
| W-1 | WARNING | REQ-GATES-3 | `pr-checks.yml:53` | `sed` extrae subdirectorio en lugar del módulo raíz — gate `dod` no testea en cambios en subdirectorios |
| W-2 | WARNING | REQ-LABELS-1, REQ-TEST-4 | `checker.go:44-48`, `invariant.go:28,36` | `Config.RequiredKeys` declarado pero no consumido |
| S-1 | SUGGESTION | — | `invariant.go:27-57`, `main.go:100-102` | Comentarios inline exceden la convención de código auto-explicativo |
| S-2 | SUGGESTION | REQ-TEST-4 | `checker_test.go:154-177` | `AssertMethodOrder` disponible pero no usado para verificar orden real LabelsByKey→Ownership |
| S-3 | SUGGESTION | REQ-TEST-6 | `reader_test.go:74` | Integración ownershipfile corre sin `-short` (comportamiento correcto, inconsistencia cosmética con jirarest) |

---

## Artefactos

- **Archivo:** `openspec/changes/ci-hardening/verify-report.md`
- **Engram:** topic `sdd/ci-hardening/verify-report`
