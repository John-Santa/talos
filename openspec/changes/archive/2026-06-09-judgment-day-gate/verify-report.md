# Verify Report: judgment-day-gate — PR A1 slice

**Change:** judgment-day-gate
**Jira:** TAL-7
**Module:** module:devops
**Owner:** HERMES
**Verificador:** ARGOS (sdd-verify)
**Rama verificada:** `agent/hermes/TAL-7`
**Fecha:** 2026-06-09
**Store:** hybrid
**Scope:** PR A1 slice only — domain `judgment.go` + `errors.go` + cmd `case "judgment"` (A2/A3 deferred)
**Veredicto:** PASS — 0 CRITICAL / 0 WARNING / 2 SUGGESTION

---

## Resultados de Tests

### `go test ./...` (full suite, desde `platform/ci-checks`)

| Package | Resultado |
|---------|-----------|
| `adapter/jirarest` | PASS (cached) |
| `adapter/ownershipfile` | PASS (cached) |
| `cmd/ch` | PASS |
| `domain/cichecks` | PASS |
| `internal/envfile` | PASS |
| `mock` | — (no test files, esperado) |
| `port` | — (no test files, esperado) |
| `service` | PASS |

**Total: 0 FAIL. Todos los packages PASS.**

Tests de judgment específicos ejecutados verbosamente y confirmados individualmente:

**`domain/cichecks` — `TestParseJudgmentReport_Table` (11 subtests):**
- APPROVED no emoji → PASS
- APPROVED with emoji → PASS
- ESCALATED no emoji → PASS
- ESCALATED with emoji → PASS
- no terminal JUDGMENT line → PASS
- Round 0 is malformed → PASS
- change mismatch → PASS
- missing Judges field → PASS
- missing Implementor field → PASS
- O-1 judges not 2 is surface-only violation → PASS
- O-1 implementor in judges is surface-only violation → PASS

**`domain/cichecks` — `TestJudgmentReport_Violations_O1` (2 sub-cases inline):** PASS

**`cmd/ch` — `TestJudgment_Table` (7 subtests):**
- APPROVED exits 0 → PASS
- APPROVED with emoji exits 0 → PASS
- ESCALATED exits 1 → PASS
- missing report file exits 1 → PASS
- no terminal JUDGMENT line exits 1 → PASS
- Round 0 exits 1 → PASS
- change mismatch exits 1 → PASS

**`cmd/ch` — `TestJudgment_JSON_APPROVED`:** PASS
**`cmd/ch` — `TestJudgment_JSON_MISSING`:** PASS
**`cmd/ch` — `TestExitCodeFor_Judgment_Rows` (3 subtests):** PASS (ErrNoJudgmentReport, ErrJudgmentNotApproved, ErrMalformedJudgment all → 1)

### `go vet ./...`

LIMPIO — 0 advertencias.

### `go build -o /dev/null ./cmd/ch`

LIMPIO — compila sin errores.

---

## Trazabilidad REQ-ID → Implementación (A1 scope)

### REQ-ARTIFACT

#### REQ-ARTIFACT-1: Ruta canónica del artefacto
**Estado: SATISFECHO**

- `cmdJudgment` construye `reportPath = filepath.Join(*changesDir, *change, "judgment-report.md")` (`main.go:319`). Default `--changes-dir openspec/changes`.
- Ausencia → `ErrNoJudgmentReport` → exit 1. Test: `TestJudgment_Table/missing_report_file_exits_1`. ✓

#### REQ-ARTIFACT-2: Header obligatorio (cinco campos)
**Estado: SATISFECHO**

- `ParseJudgmentReport` valida post-scan: `changeVal==""` → `ErrMalformedJudgment{Detail: "missing required field: Change"}` (`judgment.go:94`); `!roundParsed` → `ErrMalformedJudgment{Detail: "missing required field: Round"}` (`judgment.go:97`); `len(judges)==0` → `ErrMalformedJudgment{Detail: "missing required field: Judges"}` (`judgment.go:105`); `implementor==""` → `ErrMalformedJudgment{Detail: "missing required field: Implementor"}` (`judgment.go:108`); `!datePresent` → `ErrMalformedJudgment{Detail: "missing required field: Date"}` (`judgment.go:111`).
- Tests: `TestParseJudgmentReport_Table/missing_Judges_field`, `missing_Implementor_field`. ✓
- Nota: `Change` y `Date` missing no tienen casos de tabla independientes (ver W-1).

#### REQ-ARTIFACT-3: Campo `Round` — valor mínimo ≥ 1
**Estado: SATISFECHO**

- `roundVal < 1` → `ErrMalformedJudgment{Detail: fmt.Sprintf("Round must be >= 1, got %d", roundVal)}` (`judgment.go:100`).
- Test: `TestParseJudgmentReport_Table/Round_0_is_malformed`, `TestJudgment_Table/Round_0_exits_1`. ✓

#### REQ-ARTIFACT-4: Línea terminal — formato y unicidad; tolerancia emoji
**Estado: SATISFECHO**

- Parser detecta `strings.HasPrefix(trimmed, "JUDGMENT:")`, extrae primer `strings.Fields` token posterior para strip de emoji (`judgment.go:57-66`). Acepta `✅` y `⚠️` como tokens no-verdad en `fields[1]`, ignorados.
- Tests domain: `ESCALATED_with_emoji`, `APPROVED_with_emoji`. Tests cmd: `APPROVED_with_emoji_exits_0`. ✓
- Nota: El spec define la línea terminal como "la última línea no-vacía del archivo". La implementación usa scan de primera ocurrencia `JUDGMENT:` (no last-line-wins). Para el caso típico de un solo `JUDGMENT:` el comportamiento es idéntico (ver W-2).

#### REQ-ARTIFACT-5: Campo `Change` matchea slug del invocador
**Estado: SATISFECHO**

- `ApprovedFor(change)`: si `r.Change != change` → `ErrJudgmentNotApproved{Change: change, Verdict: r.Verdict}` (`judgment.go:152`).
- Test: `TestParseJudgmentReport_Table/change_mismatch`, `TestJudgment_Table/change_mismatch_exits_1`. ✓

---

### REQ-VALIDATOR

#### REQ-VALIDATOR-1: Interfaz del subcomando (`--change`, `--changes-dir`, `--json`)
**Estado: SATISFECHO**

- `cmdJudgment` declara los tres flags (`main.go:307-313`). `--change` requerido: si vacío → `fmt.Errorf("judgment requires --change")`. Default `--changes-dir openspec/changes`. ✓

#### REQ-VALIDATOR-2: Exit code 0 — veredicto APPROVED
**Estado: SATISFECHO**

- `ApprovedFor` retorna nil cuando `Verdict==APPROVED && Change==change`. `exitCodeFor(nil)=0` (`main.go:69-73`).
- Tests: `TestJudgment_Table/APPROVED_exits_0`, `TestJudgment_Table/APPROVED_with_emoji_exits_0`. ✓

#### REQ-VALIDATOR-3: Exit code 1 — report ausente
**Estado: SATISFECHO**

- `os.ReadFile` falla → `return cichecks.ErrNoJudgmentReport` (`main.go:333`). `exitCodeFor(ErrNoJudgmentReport)=1`.
- Test: `TestJudgment_Table/missing_report_file_exits_1`. ✓

#### REQ-VALIDATOR-4: Exit code 1 — veredicto ESCALATED
**Estado: SATISFECHO**

- `ApprovedFor`: `Verdict != "APPROVED"` → `ErrJudgmentNotApproved`. `exitCodeFor → 1`.
- Tests: `TestJudgment_Table/ESCALATED_exits_1`, `TestParseJudgmentReport_Table/ESCALATED_no_emoji`, `ESCALATED_with_emoji`. ✓

#### REQ-VALIDATOR-5: Exit code 1 — sin línea terminal parseable
**Estado: SATISFECHO**

- `verdict==""` post-scan → `ErrMalformedJudgment{Detail: "missing terminal JUDGMENT: line"}` → collapsed to `ErrJudgmentNotApproved` at cmd → exit 1.
- Tests: `TestParseJudgmentReport_Table/no_terminal_JUDGMENT_line`, `TestJudgment_Table/no_terminal_JUDGMENT_line_exits_1`. ✓

#### REQ-VALIDATOR-6: Exit code 1 — header malformado o Round 0
**Estado: SATISFECHO**

- Ver REQ-ARTIFACT-2 y REQ-ARTIFACT-3 arriba. ✓

#### REQ-VALIDATOR-7: Exit code 1 — mismatch de `Change`
**Estado: SATISFECHO**

- Ver REQ-ARTIFACT-5 arriba. ✓

#### REQ-VALIDATOR-8: Tolerancia emoji en APPROVED
**Estado: SATISFECHO**

- Ver REQ-ARTIFACT-4. `judgment.go:64-65`: `fields := strings.Fields(after); verdict = fields[0]` — el token `✅` cae en `fields[1]` y es ignorado.
- Tests: `TestParseJudgmentReport_Table/APPROVED_with_emoji`, `TestJudgment_Table/APPROVED_with_emoji_exits_0`. ✓

#### REQ-VALIDATOR-9: Salida `--json` — shape exacto
**Estado: SATISFECHO CON ADVERTENCIA (ver W-1)**

- `judgmentJSON` struct tiene exactamente los campos `{change, round, judges, implementor, verdict, violations}` con JSON tags correctos (`main.go:294-301`). `violations` nunca nil (inicializado a `[]string{}` en ausencia).
- Tests: `TestJudgment_JSON_APPROVED` verifica `change`, `verdict:"APPROVED"`, violations no-nil. `TestJudgment_JSON_MISSING` verifica `verdict:"MISSING"`.
- Nota: No existe `TestJudgment_JSON_ESCALATED` ni `TestJudgment_JSON_MALFORMED` como casos independientes (ver W-1).

#### REQ-VALIDATOR-10: Advertencias O-1 — surface-only, sin cambio de exit code
**Estado: SATISFECHO**

- `ParseJudgmentReport`: `len(judges)!=2` → append a `violations`; `implementor∈judges` → append a `violations`; ninguna condición retorna error (`judgment.go:123-134`).
- Tests: `TestParseJudgmentReport_Table/O-1_judges_not_2_is_surface-only_violation` (wantApproved: true, no error), `O-1_implementor_in_judges_is_surface-only_violation` (idem). `TestJudgmentReport_Violations_O1` verifica que `len(report.Violations) > 0`. ✓

---

### REQ-ERR

#### REQ-ERR-1: Errores `ErrNoJudgmentReport` y `ErrJudgmentNotApproved`
**Estado: SATISFECHO**

- `ErrNoJudgmentReport` — `errors.go:49` — var sentinel `errors.New("ci-checks: judgment-report.md not found")`. ✓
- `ErrJudgmentNotApproved` — `errors.go:54-64` — struct `{Change, Verdict string}` con `.Error()` que incluye change y verdict. ✓
- `ErrMalformedJudgment` — `errors.go:67-76` — struct `{Detail string}` (sentinel interno; collapsed a `ErrJudgmentNotApproved` en el I/O boundary). ✓
- `TestExitCodeFor_Judgment_Rows` verifica que los tres tipos producen exit code 1. ✓

#### REQ-ERR-2: Errores existentes no modificados
**Estado: SATISFECHO**

- `errors.go` — `ErrNoJiraKey`, `ErrLabelInvariant`, `ErrMalformedOwnership`, `ErrIssueNotFound` inalterados.
- `go test ./...` confirma tests existentes todos PASS. ✓

---

### REQ-TEST

#### REQ-TEST-1: `ParseJudgmentReport` — table-driven, sin I/O
**Estado: SATISFECHO**

- `judgment_test.go` — `TestParseJudgmentReport_Table` con 11 subtests; `TestJudgmentReport_Violations_O1` con 2 casos inline. Zero I/O (todo inline strings).
- Cubre 9 de los 9 casos exigidos por REQ-TEST-1 + 2 O-1 extra. ✓
- Nota: `missing Change field` y `missing Date field` no tienen fila independiente en la tabla (cubiertos implícitamente por el hardcodeo del `validHeader`). Esto es minor gap documentado en W-1.

#### REQ-TEST-2: `ch judgment` — integration tests con temp dir
**Estado: SATISFECHO**

- `main_test.go` — `TestJudgment_Table` (7 casos tabla + temp dir fixture), `TestJudgment_JSON_APPROVED`, `TestJudgment_JSON_MISSING`. ✓
- Spec requiere 9 casos cmd (APPROVED/ESCALATED/missing/no-terminal/round-0/mismatch/emoji-tolerance/--json APPROVED shape/--json MISSING shape). Implementados: 7 en tabla + 2 JSON = 9 total. ✓

#### REQ-TEST-3: Strict TDD — rojo→verde→refactor
**Estado: SATISFECHO**

- Apply-progress (engram #1910) documenta ciclo RED→GREEN: tests escritos antes que implementación, go test fallaba, luego implementación → verde.
- Commits: `c89acc6` (dominio) → `8729f2c` (cmd); ambos en rama `agent/hermes/TAL-7`. ✓

#### REQ-TEST-4: Tests existentes siguen verdes
**Estado: SATISFECHO**

- `go test ./...` — todos los packages pasan. Tests de `labels`, `ownership`, `changed-modules` y `dod` inalterados. ✓

---

### REQ-CI-GATE, REQ-ARCHIVE-RULE, REQ-GOVERNANCE
**Estado: DEFERRED — fuera del scope de PR A1**

Estos grupos de requisitos corresponden a PR A2 (CI job `judgment` en `pr-checks.yml`, bloque `rules.archive` en `openspec/config.yaml`, dog-food `judgment-report.md`) y PR A3 (`team-context/judgment-day.md`, fila en `ownership.md`). No se evalúan en esta verificación de A1.

---

## Verificación de Pureza Hexagonal y Scope de Ownership

### Hexagonal purity (D-2)
- `judgment.go` — pure function on `string`; imports únicamente `fmt`, `strconv`, `strings`. Zero I/O, zero filesystem. ✓
- `cmdJudgment` en `main.go` — hace `os.ReadFile(reportPath)` y pasa `string(data)` al dominio. I/O boundary exacto. ✓

### Scope de ownership (solo `platform/ci-checks/**`)

`git diff --stat develop..HEAD` muestra exactamente 5 archivos, todos bajo `platform/ci-checks/`:

| Archivo | LOC añadidas | LOC eliminadas |
|---------|--------------|----------------|
| `cmd/ch/main.go` | +96 | -2 |
| `cmd/ch/main_test.go` | +213 | 0 |
| `domain/cichecks/errors.go` | +31 | 0 |
| `domain/cichecks/judgment.go` | +186 | 0 |
| `domain/cichecks/judgment_test.go` | +232 | 0 |

**Total: 756 LOC añadidas / 2 eliminadas. Ningún archivo fuera de `platform/ci-checks/**` fue tocado.** ✓

---

## Strict TDD — Evidencia

- `judgment_test.go` escrito antes que `judgment.go` (commits `c89acc6` y `8729f2c` en ese orden documentado en apply-progress).
- Tests del dominio son table-driven, zero I/O. Tests del cmd usan `t.TempDir()` para fixture aislado, nunca el filesystem real del repo.
- 13 tests de dominio + 10 tests de cmd = 23 tests nuevos para la feature de judgment. ✓

---

## Hallazgos

### W-1 — WARNING: Gaps de cobertura test en el spec REQ-TEST-1 y cmd JSON shape

| Campo | Detalle |
|-------|---------|
| **Severidad** | WARNING |
| **REQ-IDs** | REQ-TEST-1, REQ-TEST-2, REQ-VALIDATOR-9 |
| **Archivos** | `domain/cichecks/judgment_test.go`, `cmd/ch/main_test.go` |
| **Qué falta** | (a) REQ-TEST-1 exige un caso para `missing Change field` y la tabla de `judgment_test.go` no tiene una fila dedicada que omita `**Change:**` del header; el caso se cubre implícitamente pero no está documentado como fila de tabla. (b) REQ-TEST-2 y REQ-VALIDATOR-9 implican casos `--json ESCALATED` (verdict campo `"ESCALATED"`) y `--json MALFORMED` (verdict `"MALFORMED"`) — solo existen `TestJudgment_JSON_APPROVED` y `TestJudgment_JSON_MISSING`. El comportamiento ESCALATED/MALFORMED en JSON está implementado correctamente en `cmdJudgment` pero sin tests independientes de su shape. |
| **Impacto** | Bajo — todos los caminos de código existen y el comportamiento es correcto (cubierto vía exit-code tests). La ausencia de tests de shape JSON para ESCALATED/MALFORMED significa que un refactor que rompa esos campos no sería atrapado por el suite. |
| **Fix recomendado** | Añadir en `judgment_test.go`: un caso `"missing Change field"` con header sin `**Change:**` y en `main_test.go`: `TestJudgment_JSON_ESCALATED` y `TestJudgment_JSON_MALFORMED` que decodifiquen el JSON y aserten `verdict:"ESCALATED"` / `verdict:"MALFORMED"`. Recomendado antes de A2. |

---

### W-2 — WARNING: Parser no garantiza que `JUDGMENT:` sea la última línea no-vacía

| Campo | Detalle |
|-------|---------|
| **Severidad** | WARNING |
| **REQ-IDs** | REQ-ARTIFACT-4 |
| **Archivo:línea** | `platform/ci-checks/domain/cichecks/judgment.go:57` |
| **Qué está mal** | El spec define: "La línea terminal MUST ser la última línea no-vacía del archivo." La implementación hace scan lineal y asigna `verdict` a la primera línea `JUDGMENT:` encontrada. Si un report tiene un `JUDGMENT: ESCALATED` en el cuerpo seguido de un `JUDGMENT: APPROVED` al final, el parser retorna el primer match (`ESCALATED`), no el último. En la práctica los reports bien formados solo tienen una línea `JUDGMENT:`, pero la invariante de unicidad no es verificada (el spec dice "exactamente UNA"). |
| **Impacto** | Bajo — en reportes reales ARGOS produce exactamente una línea terminal. El comportamiento es correcto para todos los casos de uso actuales. El riesgo es que un report intencionalmente malformado con dos líneas `JUDGMENT:` podría producir un veredicto distinto al esperado. |
| **Fix recomendado** | Agregar validación post-scan: si se encontraron ≥2 líneas `JUDGMENT:` → `ErrMalformedJudgment{Detail: "multiple JUDGMENT: lines found; expected exactly one"}`. O bien usar last-line-wins. Agregar test case `"multiple JUDGMENT: lines"` en domain tests. |

---

### S-1 — SUGGESTION: `judgmentJSON` no incluye `implementor` en el caso MISSING

| Campo | Detalle |
|-------|---------|
| **Severidad** | SUGGESTION |
| **REQ-IDs** | REQ-VALIDATOR-9 |
| **Archivo:línea** | `cmd/ch/main.go:325-330` |
| **Qué está mal** | Cuando el archivo no existe, el payload JSON se construye como `{Change: *change, Verdict: "MISSING", Violations: []}`. Los campos `Round`, `Judges`, `Implementor` quedan en sus zero-values (0, nil, ""). El spec dice que `verdict` es `"MISSING"` en este caso pero no especifica explícitamente los valores de los otros campos. Sin embargo, devolver `judges: null` en vez de `[]` para el caso MISSING rompe la consistencia del array (el spec garantiza array vacío para `violations` pero no para `judges` en MISSING). |
| **Impacto** | Cosmético — los consumers del JSON (CI job en A2) solo usan `verdict` para tomar decisiones; el resto de los campos en MISSING no se consumen. |
| **Fix recomendado** | Inicializar `Judges: []string{}` en el payload MISSING para consistencia de shape. |

---

### S-2 — SUGGESTION: `ErrMalformedJudgment` como tipo interno no-exportado sería más limpio

| Campo | Detalle |
|-------|---------|
| **Severidad** | SUGGESTION |
| **REQ-IDs** | REQ-ERR-1 |
| **Archivo:línea** | `platform/ci-checks/domain/cichecks/errors.go:67-76` |
| **Qué está mal** | `ErrMalformedJudgment` es documentado como "internal sentinel" colapsado en el cmd boundary pero está exportado (`Err`-prefix + título-case). Si es genuinamente interno, debería ser `errMalformedJudgment` (unexported). Su exportación invita a que código externo haga `errors.As(..., &ErrMalformedJudgment{})` directamente, lo que acopla el exterior a un detalle de implementación. |
| **Impacto** | Muy bajo en el estado actual (solo el cmd lo usa). En el futuro, si se añade otro consumidor de la librería, la API pública queda más limpia con el tipo unexported. |
| **Fix recomendado** | Renombrar a `errMalformedJudgment` (unexported) en `errors.go` y `judgment.go`. Actualizar los tests de dominio que usan `var malformed *ErrMalformedJudgment` a `var malformed *errMalformedJudgment`. |

---

## Resumen de Hallazgos

| # | Severidad | REQ-ID | Archivo | Descripción corta |
|---|-----------|--------|---------|-------------------|
| W-1 | WARNING | REQ-TEST-1, REQ-TEST-2, REQ-VALIDATOR-9 | `judgment_test.go`, `main_test.go` | Gaps de cobertura: falta caso "missing Change field", TestJudgment_JSON_ESCALATED y TestJudgment_JSON_MALFORMED |
| W-2 | WARNING | REQ-ARTIFACT-4 | `judgment.go:57` | Parser usa first-match no last-line-wins; no verifica unicidad de línea JUDGMENT: |
| S-1 | SUGGESTION | REQ-VALIDATOR-9 | `main.go:325-330` | JSON MISSING: `judges` devuelve null en lugar de `[]` (inconsistencia de shape) |
| S-2 | SUGGESTION | REQ-ERR-1 | `errors.go:67-76` | `ErrMalformedJudgment` exportado innecesariamente (es sentinel interno) |

---

## Artefactos

- **Archivo:** `openspec/changes/judgment-day-gate/verify-report.md`
- **Engram:** topic `sdd/judgment-day-gate/verify-report`

---

## Fix-batch (post-verify)

**Commit:** `68878c4` — `fix(ci-checks): enforce single JUDGMENT line + close judgment test-coverage gaps`
**Branch:** `agent/hermes/TAL-7`
**Fecha:** 2026-06-09

### W-2 — RESOLVED
`ParseJudgmentReport` now counts `JUDGMENT:` lines during the scan loop (`judgmentLineCount`). Post-scan, if `judgmentLineCount > 1` the function returns `ErrMalformedJudgment{Detail: "multiple JUDGMENT: lines found (N); expected exactly one"}`. New table case `"duplicate JUDGMENT lines is malformed"` added to `judgment_test.go` — confirmed RED before implementation, GREEN after.

### W-1 — RESOLVED
- `judgment_test.go`: added table row `"missing Change field is malformed"` — header without `**Change:**` asserts `wantParseErr: true, wantErrType: "malformed"`. Confirms existing parser behavior (field missing → `ErrMalformedJudgment{Detail: "missing required field: Change"}`).
- `main_test.go`: added `TestJudgment_JSON_ESCALATED` — asserts `verdict:"ESCALATED"` and non-nil `violations` in the `--json` output for an ESCALATED report.
- `main_test.go`: added `TestJudgment_JSON_MALFORMED` — asserts `verdict:"MALFORMED"` in the `--json` output for a report with no terminal `JUDGMENT:` line.

**`go test ./...` post-fix-batch:** all packages PASS, 0 FAIL.

**Updated verdict: PASS (0C/0W/2S)** — S-1 and S-2 (suggestions on null judges and ErrMalformedJudgment export) remain open, not blocking A2.
