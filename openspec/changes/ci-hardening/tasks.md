# Tasks: ci-hardening

**Change:** ci-hardening · **Jira:** TAL-5 · **Module:** module:devops · **Owner:** HERMES
**Branch:** `agent/hermes/TAL-5` (PR1) + stacked PR2 · **Store:** hybrid
**TDD mode:** Strict — RED antes que GREEN para todo archivo de comportamiento del dominio/service/adapter.
**Status: PENDING APPLY**

---

## Execution Summary

41 tareas (Fases 0–7) distribuidas en 2 PRs encadenados:

| Fase | Cant. ítems | PR | Status | Notas |
|------|-------------|----|--------|-------|
| 0 | 3 (scaffolding) | PR1 | pending | go.mod, .gitignore entry, ownership.md row |
| 1 | 16 (domain RED→GREEN) | PR1 | pending | label, ownership, branch, parse, errors, invariant + todos sus tests |
| 2 | 2 (ports) | PR1 | pending | IssueLabelReader, OwnershipReader (compile-only) |
| 3 | 2 (mocks) | PR1 | pending | IssueLabelReaderMock, OwnershipReaderMock |
| 4 | 5 (service RED→GREEN) | PR1 | pending | config, checker impl, checker tests (todos los paths) |
| 5 | 7 (adapters Short-gated) | PR2 | pending | jirarest client+auth+test, ownershipfile reader+test+fixture |
| 6 | 4 (cmd RED→GREEN) | PR2 | pending | cmd/ch main.go + main_test.go |
| 7 | 2 (YAML + cleanup) | PR2 | pending | pr-checks.yml, borrar ci/, .gitignore final, ownership.md row |

### Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~800–1000 LOC |
| Chained PRs recommended | Yes |
| 400-line budget risk | High |
| Decision needed before apply | Yes |
| PR1 est. lines | ~440–560 LOC |
| PR2 est. lines | ~360–440 LOC |

> **Guard nota:** el orquestador DEBE resolver la `delivery_strategy` antes de lanzar `sdd-apply`.
> PR1 es la base lógica pura (cero red); PR2 es stacked sobre PR1.

### Strict TDD — Orden de capas

```
domain (puro, cero I/O) → ports (interfaces) → mocks (test doubles) →
service (mocked ports) → adapters (httptest/Short-gated) → cmd (arg/dispatch)
→ YAML + cleanup
```

Dentro de cada capa: el test (`_test.go`) se escribe en rojo ANTES que la implementación.

---

## Key Decisions Locked by Design

1. **ADR-C1 — CLI Go `ch`:** el motor del invariante vive en `platform/ci-checks/`, hexagonal, cero-deps. No bash, no extender `ov` (pertenece a THEMIS).
2. **ADR-C2 — `dod` en YAML:** correr `go test -short ./...` vive en el runner, NO en `ch`. `ch` es SOLO motor de labels.
3. **ADR-C3 — GET-by-key:** `GET /rest/api/3/issue/{key}?fields=labels`, NO el `POST search` de `ov`.
4. **ADR-C4 — sub-regla (d) fuerte:** `branchFigura == labelAgent` es una sub-regla del invariante, no un check opcional.
5. **ADR-C5 — read-from-path:** NO `go:embed`. CI corre en repo checkouteado; embeber bakearía un snapshot stale.
6. **ADR-C6 — reimpl verbatim:** `LabelSet.Validate` (de `evidence/issue.go:49-64`) y `ValidateOwnership`/`ErrOwnershipViolation` (de `evidence/ownership.go:5-63`) se reimplementan. `go.mod` separado, cero `require`.
7. **ADR-C7 — borrar `ci/`:** `ci/pr-checks.yml` + dir `ci/` se eliminan; `.github/workflows/pr-checks.yml` se crea desde cero.
8. **ADR-C8 — creds en `cmd`:** `JIRA_SITE_URL`/`JIRA_EMAIL`/`JIRA_API_TOKEN` se leen en `cmd/ch/main.go`, no en `Config` ni en el adapter.

---

## PR1 — Lógica pura (cero red) · branch `agent/hermes/TAL-5`

### Fase 0 — Scaffolding del módulo

- [ ] **0.1 — `go.mod`** · `platform/ci-checks/go.mod`
  Crear el módulo Go: `module github.com/John-Santa/talos/platform/ci-checks` + `go 1.26`, sin bloque `require`. Espeja `overlap-guard/go.mod`. Sin comportamiento observable → no requiere test RED.
  _REQ-ninguno (scaffolding puro)_

- [ ] **0.2 — Entrada `.gitignore`** · `.gitignore`
  Append `/platform/ci-checks/ch` (binario compilado) al `.gitignore` del repo, después del bloque de los siblings (`wt`/`mo`/`ov`).
  _REQ-CLEANUP-1_

- [ ] **0.3 — Fila en `team-context/ownership.md`** · `team-context/ownership.md`
  Agregar a la **segunda** tabla (archivo→módulo) la fila `| \`platform/ci-checks/**\` | HERMES | CLI \`ch\`, invariante §4 |`. Esta fila vive en la tabla que `ParseOwnershipTable` **saltea** — no afecta el parseo. Esta tarea y la 7.3 son las únicas escrituras en archivos compartidos.
  _REQ-CLEANUP-2_

---

### Fase 1 — Domain `domain/cichecks/` (RED→GREEN por archivo)

Orden de escritura dentro de cada ítem: primero el `_test.go` (RED), luego la implementación (GREEN). El package es `cichecks`.

#### 1.1 — `errors.go` (sin test de behavior, solo compilación)

- [ ] **1.1 — `errors.go`** · `platform/ci-checks/domain/cichecks/errors.go`
  Definir el catálogo de errores tipados:
  - `ErrNoJiraKey` (sentinel o struct; debe satisfacer `error`)
  - `ErrLabelInvariant` (struct con campo `Violations []string`; `.Error()` lista todas las violaciones)
  - `ErrMalformedOwnership` (struct con campo `Detail string`; `.Error()` nombra el módulo o fila problemática)
  - `ErrIssueNotFound` (struct con campo `Key string`; `.Error()` nombra la key)

  Sin comportamiento complejo → compile-check es suficiente aquí. Los tests de catálogo van en **1.6.T** (invariant_test) y en las partes T de cada archivo.
  _REQ-ERR-1, REQ-ERR-3_

#### 1.2 — `label.go` — RED → GREEN

- [ ] **1.2.T — `label_test.go`** (RED) · `platform/ci-checks/domain/cichecks/label_test.go`
  Escribir tests table-driven ANTES de `label.go`. Casos:
  - `TestParseLabels`: strings crudos `"agent:hermes"`, `"module:devops"`, `"nocoIon"`, lista vacía → LabelSet correcta.
  - `TestLabelSet_Get`: key presente → valor; key ausente → `"", false`.
  - `TestLabelSet_Validate_OK`: `["agent", "module"]` con exactamente uno de cada uno → nil.
  - `TestLabelSet_Validate_MissingAgent`: 0 labels `agent:*` → error con texto "required but missing".
  - `TestLabelSet_Validate_DuplicateAgent`: 2 labels `agent:*` → error con texto "appears 2 times".
  - `TestLabelSet_Validate_MissingModule`: 0 labels `module:*` → error.
  - `TestLabelSet_Validate_DuplicateModule`: 2 labels `module:*` → error.
  Todos los tests fallan (RED) hasta que se escribe `label.go`.
  _REQ-INVARIANT-1, REQ-INVARIANT-2, REQ-TEST-1_

- [ ] **1.2.I — `label.go`** (GREEN) · `platform/ci-checks/domain/cichecks/label.go`
  Implementar `Label`, `LabelSet`, `ParseLabels` (verbatim de `evidence/issue.go:11-64`), `Get`, `Validate`. Hacer pasar todos los tests de 1.2.T.
  _REQ-INVARIANT-1, REQ-INVARIANT-2_

#### 1.3 — `ownership.go` — RED → GREEN

- [ ] **1.3.T — `ownership_test.go`** (RED) · `platform/ci-checks/domain/cichecks/ownership_test.go`
  Tests table-driven:
  - `TestValidateOwnership_OK`: `module=devops`, `agent=hermes`, `ownership={"devops":"hermes"}` → nil.
  - `TestValidateOwnership_Mismatch`: `agent=atlas`, expected hermes → `*ErrOwnershipViolation` con ambas figuras.
  - `TestValidateOwnership_UnknownModule`: module no presente en mapa → `*ErrOwnershipViolation` con detail "not registered".
  - `TestErrOwnershipViolation_Error`: `.Error()` contiene módulo y ambas figuras.
  Todos fallan hasta `ownership.go`.
  _REQ-INVARIANT-3, REQ-TEST-8_

- [ ] **1.3.I — `ownership.go`** (GREEN) · `platform/ci-checks/domain/cichecks/ownership.go`
  Reimplementar verbatim `ValidateOwnership` + `ErrOwnershipViolation` de `evidence/ownership.go:5-63`. Case-sensitive (el parser lowercasea aguas arriba).
  _REQ-INVARIANT-3_

#### 1.4 — `branch.go` — RED → GREEN

- [ ] **1.4.T — `branch_test.go`** (RED) · `platform/ci-checks/domain/cichecks/branch_test.go`
  Tests table-driven (REQ-TEST-1 completo):
  - `agent/hermes/TAL-7` → figura=`hermes`, key=`TAL-7`, nil.
  - `agent/Hermes/TAL-7` → `ErrNoJiraKey` (el regex es `[a-z]+`, no acepta mayúsculas — la normalización es del CALLER externo; la sub-regla (d) cubre la diferencia de casing desde `GITHUB_HEAD_REF`).

  > **Nota de diseño:** el regex `^agent/([a-z]+)/(TAL-[0-9]+)$` del spec (REQ-BRANCH-1) acepta solo lowercase en la figura. `GITHUB_HEAD_REF` en GitHub Actions entrega el nombre **tal como se creó**; las ramas se crean lowercase (`agent/hermes/TAL-5`) → el regex funciona. El test de "figura mayúscula" (REQ-BRANCH-1 scenario "OK, figura mayúscula normalizada") refleja que `cmd/ch` puede hacer `strings.ToLower` antes de llamar a `ParseAgentBranch` — ese comportamiento se testea en `main_test.go` (Fase 6), no aquí.

  - `develop` → `ErrNoJiraKey`, nil figura/key.
  - `agent/hermes/feature` → `ErrNoJiraKey`.
  - `fix/typo` → `ErrNoJiraKey`.
  - `""` (vacío) → `ErrNoJiraKey`.
  - `agent/hermes/JIRA-7` (key no-TAL) → `ErrNoJiraKey`.
  Todos fallan hasta `branch.go`.
  _REQ-BRANCH-1, REQ-BRANCH-2, REQ-TEST-1_

- [ ] **1.4.I — `branch.go`** (GREEN) · `platform/ci-checks/domain/cichecks/branch.go`
  Implementar `ParseAgentBranch` con regex `^agent/([a-z]+)/(TAL-[0-9]+)$`. Retorna `(figura, key string, err error)`; error = `ErrNoJiraKey` en cualquier no-match.
  _REQ-BRANCH-1, REQ-BRANCH-2_

#### 1.5 — `parse.go` — RED → GREEN

- [ ] **1.5.T — `parse_test.go`** (RED) · `platform/ci-checks/domain/cichecks/parse_test.go`
  Tests con fixture de string Markdown inline (REQ-TEST-3 completo):
  - `TestParseOwnershipTable_OK`: tabla con filas UPPERCASE (`HERMES`, `ATLAS`) → mapa lowercase (`"module:devops":"hermes"`).
  - `TestParseOwnershipTable_SkipsHeader`: fila encabezado `module:*` → ignorada (no aparece en mapa).
  - `TestParseOwnershipTable_SkipsSeparator`: fila `|---|---|` → ignorada.
  - `TestParseOwnershipTable_SkipsSecondTable`: tabla con segunda tabla (header `## Mapa archivo`) → solo primera tabla en mapa.
  - `TestParseOwnershipTable_DuplicateModule`: dos filas `module:devops` → `ErrMalformedOwnership` nombra `devops`.
  - `TestParseOwnershipTable_EmptyResult`: tabla válida vacía (solo headers) → `ErrMalformedOwnership`.
  - `TestParseOwnershipTable_MixedCasing`: `| \`module:Qa\` | Themis |` → `"module:qa":"themis"`.
  Todos fallan hasta `parse.go`.
  _REQ-PARSE-1, REQ-PARSE-2, REQ-PARSE-3, REQ-PARSE-4, REQ-TEST-3_

- [ ] **1.5.I — `parse.go`** (GREEN) · `platform/ci-checks/domain/cichecks/parse.go`
  Implementar `ParseOwnershipTable(md string) (map[string]string, error)` — line-scanner estilo `overlap/parse.go:9-42`, máquina de estados `inSection`, solo primera tabla, lowercasear ambos col-1 (quitar backticks, quitar prefijo `module:`) y col-2. Clave del mapa: `"module:devops"` (valor completo del label, sin quitar prefijo). Duplicado → `ErrMalformedOwnership`. Mapa vacío → `ErrMalformedOwnership`.
  _REQ-PARSE-1, REQ-PARSE-2, REQ-PARSE-3, REQ-PARSE-4_

#### 1.6 — `invariant.go` — RED → GREEN (el centro de la suite)

- [ ] **1.6.T — `invariant_test.go`** (RED) · `platform/ci-checks/domain/cichecks/invariant_test.go`
  Tests table-driven cubriendo TODAS las combinaciones (REQ-TEST-2 + REQ-INVARIANT-1 a 6):
  - `TestCheckLabelInvariant_OK`: labels `["agent:hermes","module:devops"]`, ownership `{"module:devops":"hermes"}`, branchFigura `hermes` → `Violations` vacío, nil error.
  - `TestCheckLabelInvariant_ZeroAgent` (sub-regla a): labels `["module:devops"]` → violación "0 labels agent:*".
  - `TestCheckLabelInvariant_TwoAgents` (sub-regla a): labels `["agent:hermes","agent:atlas","module:devops"]` → violación "2 labels agent:*".
  - `TestCheckLabelInvariant_ZeroModule` (sub-regla b): labels `["agent:hermes"]` → violación "0 labels module:*".
  - `TestCheckLabelInvariant_TwoModules` (sub-regla b): labels `["agent:hermes","module:devops","module:qa"]` → violación "2 labels module:*".
  - `TestCheckLabelInvariant_OwnershipMismatch` (sub-regla c): labels OK, ownership `{"module:devops":"atlas"}`, figura hermes → violación ownership.
  - `TestCheckLabelInvariant_UnknownModule` (sub-regla c): labels `["agent:hermes","module:unknown"]`, ownership `{"module:devops":"hermes"}` → violación módulo desconocido.
  - `TestCheckLabelInvariant_BranchFiguraMismatch` (sub-regla d): labels `["agent:atlas","module:devops"]`, branchFigura `hermes` → violación branch≠label.
  - `TestCheckLabelInvariant_MultipleViolations`: labels vacíos, branchFigura `hermes` → al menos violaciones (a) y (b) acumuladas.
  - `TestCheckLabelInvariant_AllViolations`: labels `[]`, branchFigura `hermes` → TODAS las violaciones (a, b; c y d salteadas porque no hay agent/module).
  - `TestCheckLabelInvariant_Verdict_OK_Exit0`: asert que `len(Violations)==0` cuando todas las sub-reglas pasan (REQ-INVARIANT-6).
  Todos fallan hasta `invariant.go`.
  _REQ-INVARIANT-1–6, REQ-TEST-2, REQ-TEST-8_

- [ ] **1.6.I — `invariant.go`** (GREEN) · `platform/ci-checks/domain/cichecks/invariant.go`
  Implementar `InvariantInput`, `InvariantResult`, `CheckLabelInvariant`. Flujo: (a)+(b) via `Validate(RequiredKeys)` → append a Violations; extraer `agent`/`module` via `Get`; (c) si ambos presentes → `ValidateOwnership` → append; (d) si `agent` presente → comparar `BranchFigura != agent` → append. Retorna `(InvariantResult, nil)` siempre (errores de negocio van en Violations, no en el segundo return). Acumula todas las violaciones, no corta.
  _REQ-INVARIANT-1–6_

---

### Fase 2 — Ports (compile-only, sin tests propios)

- [ ] **2.1 — `port/issue_label_reader.go`** · `platform/ci-checks/port/issue_label_reader.go`
  Definir `IssueLabelReader` interface: `LabelsByKey(ctx context.Context, key string) ([]string, error)`. Una línea godoc. Package `port`. No importa `domain`.
  _REQ-JIRA-1, REQ-TEST-4_

- [ ] **2.2 — `port/ownership_reader.go`** · `platform/ci-checks/port/ownership_reader.go`
  Definir `OwnershipReader` interface: `Ownership(ctx context.Context) (map[string]string, error)`. Una línea godoc. Package `port`. No importa `domain`.
  _REQ-OWNERSHIP-1, REQ-TEST-4_

---

### Fase 3 — Mocks (hand-written, RED→GREEN implícito: el test del service los usa)

- [ ] **3.1 — `mock/issue_label_reader_mock.go`** · `platform/ci-checks/mock/issue_label_reader_mock.go`
  Implementar `IssueLabelReaderMock` espejando `overlap-guard/mock/issue_searcher_mock.go:11-98`:
  - Tipo `Call{Method string; Args []any}` compartido en el package `mock`.
  - `LabelsByKeyResults map[string][]string` + `LabelsByKeyErrs map[string]error` + `DefaultResult []string` + `DefaultErr error`.
  - `LabelsByKey(ctx, key)` → registra `Call{"LabelsByKey", [key]}`, devuelve el resultado configurado por key o el default.
  - Helpers: `record`, `CallsFor`, `AssertCallCount(t, method, n)`, `AssertNotCalled(t, method)`, `AssertMethodOrder(t, methods ...string)`.
  - `var _ port.IssueLabelReader = (*IssueLabelReaderMock)(nil)` (assertion estática).
  _REQ-TEST-4_

- [ ] **3.2 — `mock/ownership_reader_mock.go`** · `platform/ci-checks/mock/ownership_reader_mock.go`
  Implementar `OwnershipReaderMock`:
  - `OwnershipMap map[string]string` + `OwnershipErr error`.
  - `Ownership(ctx)` → registra `Call{"Ownership", []}`, devuelve `OwnershipMap`/`OwnershipErr`.
  - Mismos helpers (comparte el tipo `Call` del package).
  - `var _ port.OwnershipReader = (*OwnershipReaderMock)(nil)`.
  _REQ-TEST-4_

---

### Fase 4 — Service (RED→GREEN)

- [ ] **4.1 — `service/config.go`** · `platform/ci-checks/service/config.go`
  Implementar `Config{Project, OwnershipPath, RequiredKeys}` + `DefaultTALConfig()` (espeja `merge-order/service/config.go`). Valores default: `Project="TAL"`, `OwnershipPath="team-context/ownership.md"`, `RequiredKeys=["agent","module"]`. Compile-check; no requiere test propio.
  _REQ-LABELS-3, REQ-LABELS-4_

- [ ] **4.2.T — `service/checker_test.go`** (RED) · `platform/ci-checks/service/checker_test.go`
  Tests completos del service usando los dos mocks (REQ-TEST-4):
  - `TestChecker_Check_OK`: branchFigura hermes, labels `["agent:hermes","module:devops"]`, ownership `{"module:devops":"hermes"}` → nil error, InvariantResult.Violations vacío. Verificar orden de calls: `LabelsByKey` llamado antes de `Ownership` (`AssertMethodOrder`).
  - `TestChecker_Check_ErrNoJiraKey`: branch `develop` → `ErrNoJiraKey`, `AssertNotCalled(LabelsByKey)`, `AssertNotCalled(Ownership)`.
  - `TestChecker_Check_ViolationLabelInvariant`: labels `["agent:hermes"]` (sin module) → `*ErrLabelInvariant` con violación en `.Violations`.
  - `TestChecker_Check_AllViolations`: labels `[]` → `*ErrLabelInvariant` con ≥2 violaciones.
  - `TestChecker_Check_ErrIssueNotFound`: `LabelsByKeyErrs["TAL-7"] = cichecks.ErrIssueNotFound{Key:"TAL-7"}` → error burbujea tal cual.
  - `TestChecker_Check_HTTPError`: `LabelsByKeyErrs["TAL-7"] = &jirarest.HTTPError{StatusCode:401}` — nota: dado que `service` no puede importar `adapter/jirarest`, usar el error genérico devuelto por el mock y testear que NO es nil. Alternativamente: el mock devuelve `errors.New("http 401")` y el service simplemente lo burbujea → verificar que `Check` retorna ese error.
  - `TestChecker_Check_MalformedOwnership`: `OwnershipErr = &cichecks.ErrMalformedOwnership{Detail:"devops"}` → error burbujea.
  - `TestChecker_Check_OrderOfCalls`: branch válido → verify order `LabelsByKey` then `Ownership` via `AssertMethodOrder`.
  Todos fallan hasta `checker.go`.
  _REQ-LABELS-1, REQ-LABELS-2, REQ-TEST-4, REQ-TEST-8_

- [ ] **4.2.I — `service/checker.go`** (GREEN) · `platform/ci-checks/service/checker.go`
  Implementar `Checker`, `NewChecker`, `Check(ctx, branch) (cichecks.InvariantResult, error)`. Flujo exacto del design §7: ParseAgentBranch → LabelsByKey → Ownership → ParseLabels → CheckLabelInvariant → len(Violations)>0 → ErrLabelInvariant. Creds NO en Config (ADR-C8). Hacer pasar todos los tests de 4.2.T.
  _REQ-LABELS-1, REQ-LABELS-2, REQ-INVARIANT-1–6_

---

## PR2 — Adapters + cmd + YAML + cleanup (stacked sobre PR1)

### Fase 5 — Adapters (httptest para jirarest; Short-gated para integración real)

#### 5.1 — `adapter/jirarest`

- [ ] **5.1.T — `adapter/jirarest/client_test.go`** (RED) · `platform/ci-checks/adapter/jirarest/client_test.go`
  Tests con `httptest.NewServer` (REQ-TEST-5 — siempre corren, NO son Short-gated):
  - `TestClient_LabelsByKey_OK`: servidor retorna `{"key":"TAL-7","fields":{"labels":["agent:hermes","module:devops"]}}` → labels correctos.
  - `TestClient_LabelsByKey_ValidatesPath`: verificar que el request tiene path `/rest/api/3/issue/TAL-7`.
  - `TestClient_LabelsByKey_ValidatesQueryFields`: request tiene `?fields=labels`.
  - `TestClient_LabelsByKey_ValidatesBasicAuth`: request tiene header `Authorization: Basic …`.
  - `TestClient_LabelsByKey_404`: servidor retorna 404 → `cichecks.ErrIssueNotFound` con Key=`"TAL-7"`.
  - `TestClient_LabelsByKey_401`: servidor retorna 401 → `*HTTPError{StatusCode:401}`.
  - `TestClient_LabelsByKey_500`: servidor retorna 500 → `*HTTPError{StatusCode:500}`.
  Test de integración (Short-gated, skip si `-short`):
  - `TestClient_Integration_RealJira`: requiere `JIRA_SITE_URL`/`JIRA_EMAIL`/`JIRA_API_TOKEN` en entorno. `t.Skip("integration")` si `testing.Short()`.
  Todos los httptest fallan hasta `client.go`.
  _REQ-JIRA-1–4, REQ-TEST-5_

- [ ] **5.1.A — `adapter/jirarest/auth.go`** · `platform/ci-checks/adapter/jirarest/auth.go`
  Reimplementar `setBasicAuth` verbatim de `overlap-guard/adapter/jirarest/auth.go:8-11` (Basic, `base64.StdEncoding`). Sin tests propios (cubierto por 5.1.T via cabecera HTTP).
  _REQ-JIRA-1_

- [ ] **5.1.I — `adapter/jirarest/client.go`** (GREEN) · `platform/ci-checks/adapter/jirarest/client.go`
  Implementar `HTTPError`, `Client`, `NewClient`, `LabelsByKey`, `issueLabelResponse`. Verbatim de `overlap-guard/adapter/jirarest/client.go:18-46` para `NewClient`/`HTTPError`/timeout. GET `{baseURL}/rest/api/3/issue/{key}?fields=labels`. 404 → `cichecks.ErrIssueNotFound`. no-2xx → `*HTTPError`. `var _ port.IssueLabelReader = (*Client)(nil)`.
  _REQ-JIRA-1–4_

#### 5.2 — `adapter/ownershipfile`

- [ ] **5.2.F — `adapter/ownershipfile/testdata/ownership.md`** · `platform/ci-checks/adapter/ownershipfile/testdata/ownership.md`
  Crear fixture de `team-context/ownership.md` con: filas UPPERCASE (`HERMES`, `ATLAS`, etc.), fila de encabezado, separadores `|---|`, segunda tabla (`## Mapa archivo → módulo`), y UNA fila duplicada comentada para el test de error. Debe ser una réplica suficiente del archivo real, con al menos 4 módulos.
  _REQ-TEST-6_

- [ ] **5.2.T — `adapter/ownershipfile/reader_test.go`** (RED) · `platform/ci-checks/adapter/ownershipfile/reader_test.go`
  Tests contra el fixture (REQ-TEST-6, nunca dependen del `ownership.md` real del repo):
  - `TestReader_Ownership_OK`: apunta al fixture via `NewReader(testdataPath)` → mapa lowercase correcto (HERMES→hermes, etc.).
  - `TestReader_Ownership_Lowercase`: assertea explícitamente que todos los valores son lowercase (el hallazgo clave del design §5.4).
  - `TestReader_Ownership_SkipsSecondTable`: verifica que las entradas de la segunda tabla NO aparecen en el mapa.
  - `TestReader_Ownership_FileNotFound`: `NewReader("/no/existe.md")` → error no-nil de `os.ReadFile`.
  Test de integración (Short-gated):
  - `TestReader_Integration_RealOwnership`: `t.Skip("integration")` si `testing.Short()`. Apunta al `ownership.md` real del repo; assert que los módulos conocidos están presentes y lowercase.
  Todos los unit tests fallan hasta `reader.go`.
  _REQ-OWNERSHIP-1, REQ-TEST-6_

- [ ] **5.2.I — `adapter/ownershipfile/reader.go`** (GREEN) · `platform/ci-checks/adapter/ownershipfile/reader.go`
  Implementar `Reader{path string}`, `NewReader(path string) *Reader`, `Ownership(ctx context.Context) (map[string]string, error)`. `os.ReadFile(r.path)` + `cichecks.ParseOwnershipTable(string(data))`. `var _ port.OwnershipReader = (*Reader)(nil)`.
  _REQ-OWNERSHIP-1, REQ-LABELS-3_

---

### Fase 6 — `cmd/ch` (RED→GREEN)

- [ ] **6.1.T — `cmd/ch/main_test.go`** (RED) · `platform/ci-checks/cmd/ch/main_test.go`
  Tests del composition root via `run(args)` (espeja `merge-order/cmd/mo/main_test.go`):
  - `TestRun_NoArgs`: `run([]string{})` → error no-nil (mensaje de uso).
  - `TestRun_UnknownSubcommand`: `run([]string{"bogus"})` → error no-nil.
  - `TestRun_Labels_NoBranch`: `run([]string{"labels"})` → error con "branch".
  - `TestExitCodeFor_Table`: table-driven: nil→0, `ErrNoJiraKey`→1, `&ErrLabelInvariant{}`→1, `&ErrMalformedOwnership{}`→1, `&ErrIssueNotFound{}`→1, `&jirarest.HTTPError{}`→1, `errors.New("unexpected")`→1.
  - `TestLabelsJSON_Shape_OK`: con servidor httptest + fixture ownership → `run(["labels","--branch","agent/hermes/TAL-7","--json"])` decodificar output JSON, verificar que todos los campos de REQ-JSON-1 están presentes (`branch`, `jira_key`, `figura`, `verdict`, `labels`, `agent`, `module`, `violations`) y tienen los tipos correctos.
  - `TestLabelsJSON_Shape_Violation`: mismo shape con `verdict:"VIOLATION"` y `violations` no-vacío.
  - `TestLabelsJSON_ErrNoJiraKey`: `run(["labels","--branch","develop","--json"])` → JSON con `verdict:"VIOLATION"`, `violations` contiene `ErrNoJiraKey` (REQ-JSON-2). NO texto plano mixto.
  - `TestOwnership_Offline`: `run(["ownership","--ownership-file","<fixture>"])` → nil error, exit 0.
  Todos fallan hasta `main.go`.
  _REQ-LABELS-1–4, REQ-OWNERSHIP-1–3, REQ-JSON-1–2, REQ-EXIT-1–2, REQ-TEST-7_

- [ ] **6.1.I — `cmd/ch/main.go`** (GREEN) · `platform/ci-checks/cmd/ch/main.go`
  Implementar `main`, `run(args []string) error`, `exitCodeFor(err) int`, `repoRoot() string` (verbatim de `mo/main.go:63-70`: `git rev-parse --show-toplevel`, fallback `os.Getwd()`). Dispatch: `args[0]` → `labels` | `ownership`. FlagSet con `ContinueOnError`. Subcomando `labels`: flags `--branch` (required), `--ownership-file`, `--site-url`, `--json`. Env reads `JIRA_SITE_URL`/`JIRA_EMAIL`/`JIRA_API_TOKEN` (si falta alguno → error claro antes de Jira). Construir `jirarest.NewClient` + `ownershipfile.NewReader` + `service.NewChecker`. Subcomando `ownership`: solo `ownershipfile.NewReader(...).Ownership(ctx)`. Structs `labelsJSON` + `ownershipJSON` con tags JSON exactos de REQ-JSON-1. `enc.SetIndent("","  ")` (como `mo/main.go:213`). Sin `--json`: salida tabular legible a stdout. Errores → stderr + exit 1 via `exitCodeFor`.
  _REQ-LABELS-1–4, REQ-OWNERSHIP-1–3, REQ-JSON-1–2, REQ-EXIT-1–2_

---

### Fase 7 — YAML migration + cleanup final

- [ ] **7.1 — `.github/workflows/pr-checks.yml`** · `.github/workflows/pr-checks.yml`
  Crear el directorio `.github/workflows/` (net-new; no existe en el repo) y el archivo con los 3 jobs `on: pull_request: branches: [develop]` (REQ-GATES-5):
  - **Job `branch-name`**: portar el bash de `ci/pr-checks.yml:16-27` **verbatim**. Regex `^agent/[a-z]+/TAL-[0-9]+$`, `::error::` + `exit 1`. Gate barato, sin `needs`.
  - **Job `labels`** (`needs: branch-name`): `actions/checkout@v4` + `actions/setup-go@v5 {go-version:'1.26'}` + `go build -o /tmp/ch ./cmd/ch` (en `working-directory: platform/ci-checks`) + `/tmp/ch labels --branch "$GITHUB_HEAD_REF" --json` (en repo-root) con secrets `JIRA_SITE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`.
  - **Job `dod`** (`needs: branch-name`, `fetch-depth: 0`): detectar módulos Go cambiados bajo `platform/` vía `git diff --name-only origin/$GITHUB_BASE_REF...HEAD`; para cada uno con `go.mod` → `go test -short ./...`; placeholder vitest no-bloqueante; assert `ls openspec/changes/**/verify-report.md`.
  _REQ-GATES-1–5_

- [ ] **7.2 — Eliminar `ci/`** · `ci/pr-checks.yml` + directorio `ci/`
  Borrar `ci/pr-checks.yml` y el directorio `ci/` (era staging documentado sin pointer-stub; ADR-C7). Verificar que no quedan referencias rotas.
  _REQ-CLEANUP-3_

- [ ] **7.3 — Verificar `.gitignore` y `team-context/ownership.md`** · `.gitignore`, `team-context/ownership.md`
  Confirmar que las entradas de las tareas 0.2 y 0.3 (agregadas en PR1) están presentes y correctas. Si PR1 y PR2 se desarrollan en el mismo branch, esto es un no-op; si son branches separados, cherry-pick o merge.
  _REQ-CLEANUP-1, REQ-CLEANUP-2_

---

## Cleanup Tasks

- [ ] Confirmar que `go test -short ./...` pasa desde `platform/ci-checks/` (PR1 antes de abrir PR; PR2 antes de abrir PR).
- [ ] Confirmar que `go build ./cmd/ch` compila sin errores.
- [ ] Confirmar que `ch labels --branch agent/hermes/TAL-5 --json` (con creds reales) retorna exit 0 con el issue TAL-5 bien labelado — dog-food del propio PR2.
- [ ] (Opcional) Crear `team-context/ci.md` documentando los 3 jobs, los 3 secrets requeridos (`JIRA_SITE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`) y el contrato de detección de módulos del gate `dod`.

---

## Completion

**Status: PENDING**
- [ ] Todos los tests del domain pasan (cero red, cero filesystem externo)
- [ ] Todos los tests del service pasan (mocks únicamente)
- [ ] Tests httptest del adapter jirarest pasan (sin red real)
- [ ] Tests del adapter ownershipfile pasan contra fixture
- [ ] Tests del cmd/ch pasan (arg validation, exitCodeFor, JSON shape)
- [ ] `go test -short ./...` → PASS (integración Short-skipped)
- [ ] `.github/workflows/pr-checks.yml` presente; `ci/` eliminado
- [ ] Verify-report presente en `openspec/changes/ci-hardening/`
- [ ] PR1 abierto y verde; PR2 stacked y verde
