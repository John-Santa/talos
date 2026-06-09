# Verify Report: jira-portability (TAL-6)

**Change:** jira-portability
**Jira:** TAL-6
**Módulos:** module:devops (HERMES lidera — jira-evidence-loop, ci-checks, worktree-orchestrator) + module:qa (THEMIS — overlap-guard)
**PRs integrados:** PR #17 (HERMES, 4 commits) + PR #18 (THEMIS, 1 commit)
**Branch verificado:** `tmp-verify-TAL-6` (integración de `agent/hermes/TAL-6` + `agent/themis/TAL-6` sobre develop)
**Store:** hybrid
**Fecha:** 2026-06-08
**Verificador:** ARGOS (sdd-verify)
**Veredicto inicial:** PASS WITH WARNINGS — 0 CRITICAL / 3 WARNING / 2 SUGGESTION
**Veredicto final (post fix-batch):** PASS — 0 CRITICAL / 0 WARNING / 2 SUGGESTION

---

## Fix-Batch Resolution (post-verify, aprobado por ZEUS)

Los 3 WARNINGs se corrigieron bajo strict TDD antes del archive (el change se trata de portabilidad; dejar `TAL-<n>` en los errores la contradecía):

| ID | Fix | Commit | Rama |
|----|-----|--------|------|
| **W-3** | `ErrInvalidKey` lleva `ProjectKey`; mensaje `must match <projectKey>-<n>` (RED→GREEN: test de mensaje bajo proyecto FOO) | `213f7d6` | agent/hermes/TAL-6 (PR #17) |
| **W-2** | Caso simétrico `TAL-7` rechazado bajo projectKey `FOO` en `branch_test.go` | `213f7d6` | agent/hermes/TAL-6 (PR #17) |
| **W-1** | Override `JIRA_PROJECT_KEY` en `cmdScan`/`cmdMetric` (uniformidad en los 3 subcomandos: 308/347/391) | `1dc8beb` | agent/themis/TAL-6 (PR #18) |

**Re-verificación post-fix:** los 4 módulos PASS (439 tests), `go vet` limpio, en el árbol integrado de ambas ramas actualizadas. Golden no-`JIRA_` del worktree `.env` intacto. Las 2 SUGGESTIONs quedan como follow-ups no-bloqueantes.

---

## Test Results

### `go test ./... -count=1` (full suite)

| Módulo | Paquetes | Tests pasando | FAIL |
|--------|----------|---------------|------|
| `jira-evidence-loop` | 5 (+ 2 sin tests) | 111 | 0 |
| `ci-checks` | 6 (+ 2 sin tests) | 118 | 0 |
| `worktree-orchestrator` | 5 (+ 2 sin tests) | 101 | 0 |
| `overlap-guard` | 7 (+ 2 sin tests) | 109 | 0 |
| **Total** | **23 paquetes con tests** | **439** | **0** |

### `go test -short ./... -count=1`

Idéntico al full suite — 0 tests salteados con `-short`. Todos PASS.

### `go vet ./...`

LIMPIO en los 4 módulos. 0 advertencias.

---

## REQ Coverage — hallazgo por REQ-ID

### ✅ REQ-ENV-1..7 + REQ-PARSE-1..7 — Loader + Parser

**Implementación:** `internal/envfile/envfile.go` (canónico en `jira-evidence-loop`; copia verbatim en `ci-checks`, `overlap-guard`, `worktree-orchestrator`).

- `Parse(b []byte) map[string]string` — puro, sin I/O. Implementado en cada módulo en `internal/envfile/envfile.go:11-31`.
- `LoadInto(setenv, getenv, paths...)` — precedencia if-unset, not-exist silencioso, otro-error burbujea. Implementado en `internal/envfile/envfile.go:38-56`.
- **Byte-identidad entre copias confirmada:** `diff` de los 4 `envfile.go` → IDÉNTICOS. Tests verbatim también (solo difieren en el import path del módulo, lo cual es correcto).

**Tests cubriendo REQ-PARSE-1..7 + REQ-ENV-2..6:**
- `TestParse` (11 casos) en cada módulo: blank, comment, first-= split, trim, no-= ignored, CRLF, empty, multiple, inline hash, leading hash, whitespace-only.
- `TestLoadInto` (5 casos) en cada módulo: key-already-set, unset-gets-populated, first-file-wins, missing-skipped, read-error-surfaces.

**Status: CUBIERTO ✅**

---

### ✅ REQ-IDENT-1..4 — Identidad dinámica desde entorno

**Implementación:**

| Módulo | Archivo | Seam | Campo |
|--------|---------|------|-------|
| `evidence` | `cmd/evidence/main.go:165-169` | `cmdRunLoop` | `cfg.ProjectKey` + `cfg.ProjectID` |
| `ch` | `cmd/ch/main.go:161-163` | `cmdLabels` | `cfg.Project` |
| `ov` | `cmd/ov/main.go:308-310` | `cmdCheck` | `cfg.Project` |
| `wt` | `cmd/wt/main.go:78-80` | `newOrchestrator` | `cfg.Project` |

**REQ-IDENT-2 / REQ-COMPAT-1:** `DefaultTALConfig()` sigue retornando `TAL`/`10099` en los 4 módulos. Confirmado en `service/config.go` de cada módulo.

**REQ-IDENT-4:** `.talos/project.env` contiene únicamente `JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`. Sin secrets. Confirmado por inspección directa del archivo.

**Status: CUBIERTO ✅** (con WARNING W-1 para cmdScan/cmdMetric de ov — ver abajo)

---

### W-1 ⚠️ REQ-IDENT-1 (ov) — Override de `cfg.Project` ausente en `cmdScan` y `cmdMetric`

**Severidad:** WARNING

**Archivo:línea:** `platform/overlap-guard/cmd/ov/main.go:331-366` (cmdScan) y `369-410` (cmdMetric)

**Qué está mal:** Las tareas PR5-T4 exigen el override `if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }` en los **3 seams** de ov: `cmdCheck`, `cmdScan`, `cmdMetric`. Solo `cmdCheck` (línea 308-310) lo tiene. `cmdScan` y `cmdMetric` no.

**Impacto real:** `cfg.Project` solo es consumido en `guard.CheckPreAssignment()` para construir el JQL (línea `guard.go:70`). `ScanInFlight` y `Metric` no usan `cfg.Project`. Por ende, el comportamiento observable en `ov scan` y `ov metric` NO está afectado hoy. Pero la inconsistencia viola el patrón de portabilidad completa y podría convertirse en un bug silencioso si se agrega uso de `cfg.Project` en esos paths.

**Fix recomendado:** Agregar `if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }` en `cmdScan` (tras `cfg := service.DefaultTALConfig()` en línea ~336) y en `cmdMetric` (línea ~374).

**REQ-ID:** REQ-IDENT-1, PR5-T4

---

### ✅ REQ-BRANCH-1..5 — Validación de rama parametrizada

**Implementación:** `platform/ci-checks/domain/cichecks/branch.go:9-16`

```go
func ParseAgentBranch(branch, projectKey string) (figura, key string, err error) {
    re := regexp.MustCompile(`^agent/([a-z]+)/(` + regexp.QuoteMeta(projectKey) + `-[0-9]+)$`)
    ...
}
```

- Firma parametrizada ✅, `regexp.QuoteMeta` ✅, sin `var branchRE` package-level ✅.
- Threading: `extractBranchParts(branch, projectKey string)` en `cmd/ch/main.go:239-245` pasa `cfg.Project` desde `cmdLabels:171`.

**Tests cubriendo REQ-BRANCH-1..5 (`branch_test.go`):**
- `"canonical lowercase figura"` (TAL/TAL-7 → válida) ✅ REQ-BRANCH-2
- `"non-TAL key rejected under TAL projectKey"` (TAL/JIRA-7 → error) ✅ REQ-BRANCH-3 (cubre FOO bajo TAL)
- `"different project key accepted"` (FOO/FOO-7 → válida) ✅ REQ-BRANCH-4
- `"FOO branch rejected under TAL projectKey"` (TAL/FOO-7 → error) ✅ REQ-BRANCH-3

**Status: CUBIERTO ✅** (con WARNING W-2 — test case REQ-BRANCH-5 explícito ausente)

---

### W-2 ⚠️ REQ-BRANCH-5 / REQ-TEST-3 — Test case `FOO/agent/hermes/TAL-7 → ErrNoJiraKey` ausente

**Severidad:** WARNING

**Archivo:línea:** `platform/ci-checks/domain/cichecks/branch_test.go` (falta el caso)

**Qué está mal:** REQ-TEST-3 y las tareas PR3-T3 exigen explícitamente una fila `"FOO / agent/hermes/TAL-7 → ErrNoJiraKey"` en `TestParseAgentBranch`. El test file tiene `"different project key accepted"` (FOO/FOO-7 → valid) y `"FOO branch rejected under TAL projectKey"` (FOO-7/TAL → error), pero NO tiene `projectKey:"FOO", branch:"agent/hermes/TAL-7"` como caso propio. La lógica es correcta (el regex lo rechazaría), pero el caso de test explícito para REQ-BRANCH-5 falta.

**Fix recomendado:** Agregar en `TestParseAgentBranch`:
```go
{
    name:       "TAL branch rejected under FOO projectKey",
    branch:     "agent/hermes/TAL-7",
    projectKey: "FOO",
    wantErr:    true,
},
```

**REQ-ID:** REQ-BRANCH-5, REQ-TEST-3, PR3-T3

---

### ✅ REQ-WTKEY-1..4 — Validación de clave Jira para worktrees

**Implementación:** `platform/worktree-orchestrator/domain/worktree/naming.go:31-37`

```go
func ValidateJiraKey(key, projectKey string) error {
    re := regexp.MustCompile("^" + regexp.QuoteMeta(projectKey) + "-[1-9][0-9]*$")
    ...
}
```

- Firma parametrizada ✅, `regexp.QuoteMeta` ✅, sin `var jiraKeyRe` package-level ✅.
- `NewWorktreeSpec(figura, jiraKey, worktreeBase, projectKey string)` ✅.
- 3 call sites en `service/orchestrator.go:62,153,205` con `o.cfg.Project` ✅.

**Tests cubriendo REQ-WTKEY-1..4 (`naming_test.go`):**
- `TAL/TAL-1` → válida ✅
- `TAL/TAL-0` → error ✅ (REQ-WTKEY-3)
- `FOO/FOO-1` → válida ✅ (REQ-WTKEY-2 under FOO)
- `FOO/TAL-1` → error ✅ (REQ-WTKEY-4)

**Status: CUBIERTO ✅**

---

### W-3 ⚠️ REQ-ERR-2 — Mensaje de error hardcodea "TAL" en lugar del `projectKey` configurado

**Severidad:** WARNING

**Archivo:línea 1:** `platform/worktree-orchestrator/domain/worktree/errors.go:21`
```go
func (e *ErrInvalidKey) Error() string {
    return fmt.Sprintf("invalid jira key %q: must match TAL-<n> (n >= 1)", e.Key)
}
```

**Archivo:línea 2:** `platform/worktree-orchestrator/domain/worktree/errors.go:15` (comentario)
```go
// ErrInvalidKey is returned when a Jira key does not match the TAL-<n> pattern.
```

**Archivo:línea 3:** `platform/ci-checks/domain/cichecks/errors.go:12`
```go
var ErrNoJiraKey = errors.New("ci-checks: branch has no extractable Jira key (expected agent/<figura>/<projectKey>-N)")
```

**Qué está mal:**
1. `ErrInvalidKey.Error()` hardcodea `"TAL-<n>"` — cuando `projectKey=FOO` el mensaje dice "must match TAL-<n>", lo cual es incorrecto e incumple REQ-ERR-2.
2. El comentario en `errors.go:15` también hardcodea el patrón `TAL-<n>` (problema de documentación).
3. `ErrNoJiraKey` en ci-checks es un sentinel (`errors.New`), no un tipo struct, por lo que NO puede transportar el `projectKey` configurado. El mensaje muestra `<projectKey>` como literal, no el valor real (ej: `FOO`).

**Fix recomendado:**
- `ErrInvalidKey`: convertir a struct que almacene `projectKey string`, actualizar `Error()` a `fmt.Sprintf("invalid jira key %q: must match %s-<n> (n >= 1)", e.Key, e.ProjectKey)`.
- `ValidateJiraKey`: pasar `projectKey` al construir el error: `&ErrInvalidKey{Key: key, ProjectKey: projectKey}`.
- `ErrNoJiraKey`: similar refactor para transportar y mostrar el projectKey real.
- Actualizar el comentario godoc de `ErrInvalidKey`.

**REQ-ID:** REQ-ERR-2, REQ-WTKEY-4

---

### ✅ R8 / ADR-J3 — Hoist de `LoadInto` ANTES del switch y flag defaults

**Verificación de ordering en los 3 binarios:**

**`evidence/main.go:40-50` (run()):**
```go
// R8 / ADR-J3: load env files BEFORE any os.Getenv call or flag default evaluation.
root := repoRoot()
paths := []string{ ... }
if mainRoot := mainWorktreeRoot(); mainRoot != "" && mainRoot != root {
    paths = append(paths, ...)
}
_ = envfile.LoadInto(os.Setenv, os.Getenv, paths...)
// ↓ LUEGO: switch args[0] → cmdRunLoop
```
`cmdRunLoop` tiene `siteURL := fs.String("site-url", "", ...)` (default vacío, no `os.Getenv`) — sin riesgo. ✅

**`ch/main.go:39-49` (run()):**
```go
// R8 / ADR-J3: load env files BEFORE os.Getenv call or flag default evaluation
// (e.g. fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)).
root := repoRoot()
...
_ = envfile.LoadInto(os.Setenv, os.Getenv, paths...)
// ↓ LUEGO: switch → cmdLabels
```
`cmdLabels:131` tiene `siteURL := fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)` — este flag default evalúa `os.Getenv` en el momento de `fs.String(...)`, que ocurre DESPUÉS del hoist. ✅

**`ov/main.go:39-48` (run()):**
```go
// R8 / ADR-J3: load env files BEFORE os.Getenv call or flag default evaluation
// (e.g. fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)).
root := repoRoot()
...
_ = envfile.LoadInto(os.Setenv, os.Getenv, paths...)
// ↓ LUEGO: switch → cmdCheck/cmdScan/cmdMetric
```
`cmdCheck:280` tiene `siteURL := fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)` — DESPUÉS del hoist. ✅

**`wt/main.go:37-39` (run()):**
```go
// R8 / ADR-J3: load project.env BEFORE any os.Getenv evaluation.
_ = envfile.LoadInto(os.Setenv, os.Getenv, filepath.Join(repoRoot(), ".talos", "project.env"))
// ↓ LUEGO: switch
```
`wt` solo carga `project.env` (no `.env` — ADR-J7: wt no toca creds). ✅

**Status: CUBIERTO ✅ — ADR-J3 R8 satisfecho en los 4 binarios.**

---

### ✅ REQ-AUTH-1 (enmienda) — `openspec/specs/jira-loop/functional.md`

**Archivo:líneas:** `openspec/specs/jira-loop/functional.md:24-31`

El texto enmendado dice:
> secrets (`JIRA_EMAIL`, `JIRA_API_TOKEN`) MUST be sourced exclusively from the environment (populated by a gitignored `.env` file or an inherited environment) and MUST NOT appear in flags, logs, or any committed file. Non-secret project identity (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) MAY be pre-populated from `.talos/project.env`, which is a committed, non-secret runtime mirror of `openspec/config.yaml:jira`.

**Comentario `config.go:15-18`** (evidence/service):
> Credentials holds the Jira authentication secrets (JIRA_EMAIL, JIRA_API_TOKEN) sourced exclusively from the environment or a gitignored .env file. They must never appear in flags, logs, or committed files. Non-secret project identity (site URL, project key/ID) may live in committed .talos/project.env.

**Status: CUBIERTO ✅**

---

### ✅ REQ-WT-1..5 — Worktrees

- `mainWorktreeRoot()` presente en `evidence/main.go:77-94`, `ch/main.go:84-102`, `ov/main.go:88-105`. Implementa `git rev-parse --git-common-dir` + normalización relativo→absoluto + `filepath.Dir`. Best-effort, retorna `""` en error. ✅
- `wt` carga únicamente `.talos/project.env` (NO `.env`) — correcto per ADR-J7. ✅
- Golden `env_test.go:107` (`worktree-orchestrator/domain/worktree/env_test.go`): `RenderEnv` sigue emitiendo solo `PORT=8106\nDB_SCHEMA=wt_hermes\n`, sin `JIRA_`. Test `TestRenderEnv` pasa. ✅ REQ-WT-5 / REQ-COMPAT-3.

**Status: CUBIERTO ✅**

---

### ✅ REQ-COMPAT-1..3 — Back-compatibilidad

- Sin env/archivos → `DefaultTALConfig()` retorna `TAL`/`10099` en los 4 módulos. Confirmado. ✅
- Tests existentes de los 4 módulos PASS sin modificación. ✅
- Golden `env_test.go:107` intacto. ✅
- `.talos/project.env` confirmado no ignorado: `git check-ignore -v .talos/project.env` → sin output. ✅
- `.gitignore:3` es `.env.*` — `.talos/project.env` NO matchea este patrón (empieza con `.talos/`, no con `.env`). ✅

**Status: CUBIERTO ✅**

---

### ✅ Zero-dep — `go.mod` sin bloque `require`

Los 4 `go.mod` son de 2 líneas cada uno (module path + go version). Sin `require`, sin `go.sum`. ✅

**Módulos confirmados:**
- `github.com/John-Santa/talos/platform/jira-evidence-loop` — go 1.26
- `github.com/John-Santa/talos/platform/ci-checks` — go 1.26
- `github.com/John-Santa/talos/platform/worktree-orchestrator` — go 1.26
- `github.com/John-Santa/talos/platform/overlap-guard` — go 1.26

**Status: CUBIERTO ✅**

---

### ✅ REQ-IDENT hardcode scan — sin "TAL"/"10099"/"tablex" fuera de DefaultTALConfig

Grep exhaustivo de `"TAL"`, `"10099"`, `"tablex"` en todos los archivos `.go` no-test, no-DefaultTALConfig:

- Todos los literales `"TAL"` en non-test code están en `DefaultTALConfig()` seeds (confirmado por grep). ✅
- Únicas excepciones legítimas: comentarios godoc de `Project` field mencionando `"TAL"` como ejemplo (no path de código). ✅
- `"10099"` y `"tablex"` aparecen solo en `jira-evidence-loop/service/config.go` dentro de `DefaultTALConfig()`. ✅

**EXCEPCIÓN encontrada en `errors.go:21`:** `"must match TAL-<n>"` en `ErrInvalidKey.Error()` — este sí es un hardcode en un path de código. Reportado como **W-3**.

---

### ✅ Cross-module §2 — Ownership HERMES/THEMIS

- PR #17 (commits e3da7e2, ec61402, ef79469, 65da73b): HERMES escribió en `jira-evidence-loop/`, `ci-checks/`, `worktree-orchestrator/`. NO tocó `overlap-guard/`. ✅
- PR #18 (commit cd68a32): THEMIS escribió exclusivamente en `overlap-guard/`. ✅
- HERMES sign-off en PR5 registrado en apply-progress (ZEUS aprobó el cruce §2). ✅

**Status: §2 COMPLIANT ✅**

---

## Suggestions

### S-1 — Comentarios inline en `wt/main.go` y `errors.go` con literales `TAL-N`

**Severidad:** SUGGESTION

**Archivo:líneas:** `platform/worktree-orchestrator/cmd/wt/main.go:5,7,8` (doc package header: `<TAL-N>`), líneas `95,189,220` (error messages en strings de usuario: `<TAL-N>`).

Las strings de ayuda como `"create requires <figura> <TAL-N> [--no-fetch]"` sugieren al usuario que debe usar `TAL-N`, cuando el CLI ya acepta cualquier clave. Podría actualizarse a `<KEY-N>` o `<JIRA-KEY>` para consistencia con la portabilidad.

**REQ-ID:** SUGGESTION (no bloquea)

### S-2 — Test `TestParse` en los 4 módulos no incluye caso de clave vacía tras trim

**Severidad:** SUGGESTION

El parser en `envfile.go:26-28` skipea claves vacías tras trim (`if key == "" { continue }`), pero ningún test cubre el caso `"  = value"` (clave vacía después de trim). Este es un edge case relevante definido en REQ-PARSE-4. La cobertura es implícita pero no explícita.

**REQ-ID:** SUGGESTION, REQ-PARSE-4

---

## REQ Coverage Summary

| REQ-ID | Status | Notas |
|--------|--------|-------|
| REQ-ENV-1..7 | ✅ CUBIERTO | `LoadInto` hoisted en los 4 binarios antes del switch |
| REQ-PARSE-1..7 | ✅ CUBIERTO | `Parse` puro, 11 casos table-driven |
| REQ-IDENT-1 | ⚠️ PARCIAL | Override correcto en evidence/ch/cmdCheck/wt. Ausente en ov cmdScan+cmdMetric (W-1, baja severidad funcional) |
| REQ-IDENT-2..4 | ✅ CUBIERTO | DefaultTALConfig seeds, .talos/project.env non-secret |
| REQ-BRANCH-1..5 | ⚠️ PARCIAL | Lógica correcta; test case explícito REQ-BRANCH-5 ausente (W-2) |
| REQ-WTKEY-1..4 | ✅ CUBIERTO | ValidateJiraKey + NewWorktreeSpec parametrizados, QuoteMeta |
| REQ-AUTH-1 (enmienda) | ✅ CUBIERTO | functional.md + config.go comment actualizados |
| REQ-WT-1..5 | ✅ CUBIERTO | mainWorktreeRoot, project.env tracked, golden intacto |
| REQ-COMPAT-1..3 | ✅ CUBIERTO | DefaultTALConfig back-compat, golden invariant |
| REQ-ERR-1 | ✅ CUBIERTO | ErrNoJiraKey (ci-checks), ErrInvalidKey (wt) preexistentes |
| REQ-ERR-2 | ⚠️ WARNING | ErrInvalidKey hardcodea "TAL-<n>"; ErrNoJiraKey no transporta projectKey real (W-3) |
| REQ-TEST-1..2 | ✅ CUBIERTO | Parse + LoadInto con inyectables en los 4 módulos |
| REQ-TEST-3 | ⚠️ PARCIAL | Falta caso explícito FOO/TAL-7 → ErrNoJiraKey (W-2) |
| REQ-TEST-4 | ✅ CUBIERTO | ValidateJiraKey table-driven con sub-tests TAL + FOO |
| REQ-TEST-5 | ✅ CUBIERTO | Tests existentes de los 4 módulos PASS sin cambios |

---

## Task Completion

49 tareas en 5 PRs — todas implementadas según el apply-progress. Gaps reportados como WARNING/SUGGESTION no bloquean el archive.

---

## Resumen

| Criterio | Resultado |
|----------|-----------|
| `go test ./... -count=1` (4 módulos) | ✅ PASS — 439 tests, 0 FAIL |
| `go test -short ./... -count=1` (4 módulos) | ✅ PASS — idéntico |
| `go vet ./...` (4 módulos) | ✅ PASS — 0 advertencias |
| Zero-dep (go.mod sin require) | ✅ PASS — los 4 módulos |
| envfile byte-idéntico (4 copias) | ✅ PASS — diff confirma identidad del `.go` |
| R8/ADR-J3 hoist ANTES del switch | ✅ PASS — verificado en evidence/ch/ov/wt |
| REQ-AUTH-1 enmendado | ✅ PASS — functional.md + config.go |
| .talos/project.env no-ignorado, contenido correcto | ✅ PASS |
| Golden env_test.go:107 (sin JIRA_) intacto | ✅ PASS |
| QuoteMeta sin var package-level (branch/naming) | ✅ PASS |
| REQ-IDENT override en ov cmdScan/cmdMetric | ⚠️ WARNING W-1 |
| REQ-BRANCH-5 test case explícito | ⚠️ WARNING W-2 |
| REQ-ERR-2 mensaje con projectKey configurado | ⚠️ WARNING W-3 |
| Cross-module §2 (HERMES/THEMIS) | ✅ PASS |

**Veredicto: PASS WITH WARNINGS**
0 CRITICAL / 3 WARNING / 2 SUGGESTION.

Los 3 WARNINGs son gaps de completitud (no regresiones ni bloqueos): W-1 es un override ausente en seams que hoy no consumen el campo; W-2 es un test case faltante para un caso que la lógica ya maneja; W-3 es un mensaje de error que hardcodea "TAL" y no informa la clave configurada al usuario. Ninguno bloquea el archive — se recomienda un fix-batch ligero antes de archive o como follow-up. La arquitectura es sólida, el R8 crítico está correctamente implementado, todos los suites están verdes, y el invariante golden del worktree .env está intacto.
