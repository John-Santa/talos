# Tasks: jira-portability

**Change:** jira-portability · **Jira:** TAL-6 · **Modules:** module:devops (ch/wt) + module:jira-loop (evidence) + module:qa (ov)
**Owner:** HERMES (lidera) + THEMIS (slice `ov`)
**Branch:** `agent/hermes/TAL-6` (PR1–PR4) + THEMIS branch (PR5)
**Store:** hybrid (`openspec/changes/jira-portability/tasks.md` + engram `sdd/jira-portability/tasks`)
**TDD mode:** Strict — RED before GREEN, L1→L2→L3→L4, every behavioral unit.
**Status: PENDING**

---

## Execution Summary

All tasks across 5 chained PRs:

| PR | Owner | Scope | Task Count | Status |
|----|-------|-------|------------|--------|
| PR1 | HERMES | `envfile` canónico + tests (keystone) | 10 | PENDING |
| PR2 | HERMES | evidence wire + REQ-AUTH + `.talos/project.env` | 9 | PENDING |
| PR3 | HERMES | `ch`: loader copy + `branch.go` param + override + threading | 12 | PENDING |
| PR4 | HERMES | `wt`: `naming.go` param + `Config.Project` + threading + LoadInto | 11 | PENDING |
| PR5 | THEMIS | `ov`: loader copy + override (HERMES sign-off) | 7 | PENDING |
| **Total** | | | **49** | **PENDING** |

### Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~630 LOC |
| Chained PRs recommended | Yes |
| 400-line budget risk | Low (each PR <400) |
| Decision needed before apply | No (auto-chain; cross-module §2 sign-off required for PR5) |
| Per-PR estimates | PR1 ~110 · PR2 ~120 · PR3 ~150 · PR4 ~140 · PR5 ~110 |

### Strict TDD Layers

- **L1** — `Parse` PURO (table-driven, sin I/O): blank/`#`/trim/CRLF/vacío/sin-`=`/primer-`=`.
- **L2** — `LoadInto` (setenv/getenv inyectables como maps): precedencia env-real > archivo; if-unset; primer-path gana; not-exist silencioso; error-no-not-exist burbujea.
- **L3** — regex param (extender tests existentes): `branch_test.go` columna `projectKey`; `naming_test.go` threadea `"TAL"` + sub-test FOO.
- **L4** — composition root: mains finos; tests `run()` existentes quedan verdes.

---

## Key Decisions Locked by Design

1. **ADR-J3 (R8 — bug más probable):** `LoadInto` al TOPE de `run()`, ANTES del `switch` y de cualquier `flag.NewFlagSet` cuyo default lea `os.Getenv("JIRA_SITE_URL")` (`ch:94`, `ov:243`). El default del flag se evalúa en `fs.String(...)`, dentro del subcomando → el loader NO puede ir adentro.
2. **ADR-J4:** Error de `LoadInto` se descarta (`_ =`). El loader SOLO PUEBLA; fail-fast es de cada CLI.
3. **ADR-J5 (R1):** `DefaultTALConfig` NO se renombra (9 call sites). Override de 3 escalares en el composition root.
4. **ADR-J6:** Dominio sigue PURO. `projectKey` por parámetro; regex compilada inline con `regexp.QuoteMeta`. Drop del `var` package-level.
5. **ADR-J8:** `envfile` canónico en `jira-evidence-loop`; copia VERBATIM a `ci-checks/` y `overlap-guard/`. NO cross-import (go.mod separados — mismo patrón que `ValidateOwnership`).
6. **ADR-J9:** Archivo `.talos/project.env` (NO `.env.project` — `.gitignore:3` lo atraparía).
7. **ADR-J10:** Golden de `env_test.go:107` (worktree `.env` sin `JIRA_`) → INTACTO. Load-bearing invariant.
8. **Field override difiere por módulo:** `evidence` usa `cfg.ProjectKey`/`cfg.ProjectID` (`service/config.go:33,36`); `ch` usa `cfg.Project` (`service/config.go:6`, sin ProjectID); `ov` usa `cfg.Project` (`service/config.go:14`, sin ProjectID).
9. **ch threading:** `extractBranchParts(branch)` no tiene `cfg` en scope → cambiar firma a `extractBranchParts(branch, projectKey string)` y pasar `cfg.Project` desde `cmdLabels:127`.
10. **wt loader:** `wt` NO toca creds (no carga `.env`), pero SÍ necesita `JIRA_PROJECT_KEY` para la regex parametrizada. `newOrchestrator()` corre `LoadInto` solo con `root/.talos/project.env` antes de leer `JIRA_PROJECT_KEY`.

---

## PR1 — `envfile` canónico + tests (HERMES, keystone)

> **Archivos:** `platform/jira-evidence-loop/internal/envfile/envfile_test.go` (nuevo), `platform/jira-evidence-loop/internal/envfile/envfile.go` (nuevo)
> **REQ-IDs:** REQ-PARSE-1..7, REQ-ENV-1..6, REQ-TEST-1..2

Este PR es el **keystone**: fija el patrón `envfile` que PR3 y PR5 copian verbatim. No toca ningún `cmd/*`.

### Fase L1 — Tests de `Parse` (RED primero)

- [ ] **PR1-T1.1** `platform/jira-evidence-loop/internal/envfile/envfile_test.go` — crear archivo. Escribir `TestParse` table-driven con casos (REQ-PARSE-1..7, REQ-TEST-1):
  - `"blank line ignored"`: input `"\n"` → `{}`
  - `"comment line ignored"`: input `"# comment\n"` → `{}`
  - `"first-= split"`: input `"K=a=b=c"` → `{"K": "a=b=c"}` (REQ-PARSE-3)
  - `"trim spaces"`: input `"  K  =  V  "` → `{"K": "V"}` (REQ-PARSE-4)
  - `"no-= ignored"`: input `"NOEQUALS"` → `{}` (REQ-PARSE-5)
  - `"CRLF tolerated"`: input `"K=V\r\n"` → `{"K": "V"}` (REQ-PARSE-6)
  - `"empty input"`: input `""` → `{}` (REQ-PARSE-7)
  - Confirmar que `go test ./internal/envfile/...` FALLA (RED — `Parse` no existe todavía).

- [ ] **PR1-T1.2** `platform/jira-evidence-loop/internal/envfile/envfile.go` — implementar `Parse(b []byte) map[string]string`:
  - `strings.TrimRight(line, "\r")` + `strings.TrimSpace` por línea.
  - Skip vacía o comienza con `#`.
  - `strings.SplitN(line, "=", 2)` → key `TrimSpace(parts[0])`, value `TrimSpace(parts[1])`.
  - Línea sin `=` (len(parts) < 2) → skip.
  - Key vacía tras trim → skip.
  - Retornar `map[string]string{}` (no nil) para input vacío.
  - Confirmar `go test ./internal/envfile/...` PASA (GREEN, solo L1).

### Fase L2 — Tests de `LoadInto` (RED primero)

- [ ] **PR1-T2.1** `platform/jira-evidence-loop/internal/envfile/envfile_test.go` — agregar `TestLoadInto` con setenv/getenv inyectables como closures sobre maps (REQ-ENV-1..6, REQ-TEST-2):
  - `"key already set not overwritten"`: env preexistente `K=real`; archivo define `K=file` → env sigue `K=real` (REQ-ENV-2).
  - `"unset key gets populated"`: env vacío; archivo define `K=V` → env tiene `K=V` (REQ-ENV-3).
  - `"first-file-wins"`: dos archivos en tempdir; primero define `K=first`, segundo `K=second` → env tiene `K=first` (REQ-ENV-4, corolario).
  - `"missing file skipped silently"`: path inexistente → `nil` error, env sin cambios (REQ-ENV-5).
  - `"read-error surfaces"`: path a archivo que existe pero con error de lectura (usar `os.WriteFile` + `os.Chmod(0)`) → error no-nil devuelto (REQ-ENV-6). _(Nota: este caso puede requerir `os.TempDir` + fixture real; `getenv`/`setenv` siguen siendo maps inyectados; solo `os.ReadFile` del path toca el filesystem.)_
  - Confirmar FALLA (RED — `LoadInto` no existe todavía).

- [ ] **PR1-T2.2** `platform/jira-evidence-loop/internal/envfile/envfile.go` — agregar `LoadInto(setenv func(k, v string) error, getenv func(k string) string, paths ...string) error`:
  - Iterar paths en orden; `os.ReadFile`; si `os.IsNotExist(err)` → `continue`; otro error → `return err` inmediato.
  - `Parse(b)` → iterar keys; si `getenv(k) == ""` → `setenv(k, v)`; si `setenv` retorna error → `return err`.
  - `return nil`.
  - Confirmar `go test ./internal/envfile/...` PASA (GREEN, L1 + L2).

### Fase complementaria — `mainWorktreeRoot()` helper

- [ ] **PR1-T3.1** `platform/jira-evidence-loop/internal/envfile/envfile_test.go` — agregar test ligero de `mainWorktreeRoot()` (best-effort, no test de worktree real): verificar que en un repo normal (no worktree) retorna un string no-vacío y que la raíz resultante tiene un directorio `.git`. _(Test puede ser skipped con `testing.Short()` si requiere `git`.)_

- [ ] **PR1-T3.2** Ubicar `mainWorktreeRoot()` en `platform/jira-evidence-loop/cmd/evidence/main.go` — se implementará en PR2. Notar aquí para tracking: la helper va en `cmd/evidence/main.go`, no en `internal/envfile/`. No crear todavía en este PR.

> **Criterio de merge PR1:** `go test ./internal/envfile/... -count=1` → PASS (todos los casos L1 + L2). Sin cambios en `cmd/*`.

---

## PR2 — Evidence wire + REQ-AUTH + `.talos/project.env` (HERMES)

> **Archivos:** `platform/jira-evidence-loop/cmd/evidence/main.go`, `platform/jira-evidence-loop/service/config.go`, `openspec/specs/jira-loop/functional.md`, `.talos/project.env` (nuevo), `.env.example`, `openspec/config.yaml`
> **REQ-IDs:** REQ-ENV-1 (evidence), REQ-IDENT-1..4, REQ-AUTH-1 (enmienda), REQ-WT-2..3, REQ-COMPAT-1..2, REQ-TEST-5
> **Depende de:** PR1 mergeado (usa `internal/envfile`)

### R8 — hoist del loader (tarea más crítica de este PR)

- [ ] **PR2-T1** `platform/jira-evidence-loop/cmd/evidence/main.go` — **agregar `repoRoot()` helper** (~7 LOC, copia verbatim de `ci-checks/cmd/ch/main.go:58-65`). Agregar imports `"os/exec"`, `"strings"`, `"path/filepath"` si no están.

- [ ] **PR2-T2** `platform/jira-evidence-loop/cmd/evidence/main.go` — **agregar `mainWorktreeRoot()` helper** (~8 LOC, diseño §4). Usar `git rev-parse --git-common-dir`; normalizar relativo→absoluto con `filepath.Join(wd, common)` si `!filepath.IsAbs(common)`; retornar `filepath.Dir(common)`. Best-effort: retornar `""` si `git` falla.

- [ ] **PR2-T3 [R8 CRÍTICO]** `platform/jira-evidence-loop/cmd/evidence/main.go` — **hoist `LoadInto` al TOPE de `run()`**, antes del `switch args[0]` (`main.go:39`). El bloque completo:
  ```go
  root := repoRoot()
  mainRoot := mainWorktreeRoot()
  _ = envfile.LoadInto(os.Setenv, os.Getenv,
      root+"/.talos/project.env",
      root+"/.env",
      mainRoot+"/.env",
  )
  ```
  Confirmar que está ANTES de cualquier evaluación de flag o subcomando. Error descartado (`_ =`) — ADR-J4.

### Override de identidad en evidence

- [ ] **PR2-T4** `platform/jira-evidence-loop/cmd/evidence/main.go` — **override de `cfg.ProjectKey` y `cfg.ProjectID`** tras `DefaultTALConfig()` (en el seam `main.go:108-111`):
  ```go
  if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.ProjectKey = v }
  if v := os.Getenv("JIRA_PROJECT_ID");  v != "" { cfg.ProjectID  = v }
  ```
  _(Campos: `ProjectKey` y `ProjectID` — NO `Project`; verificar contra `evidence/service/config.go:33,36`.)_

### Enmienda REQ-AUTH

- [ ] **PR2-T5** `openspec/specs/jira-loop/functional.md` — **actualizar REQ-AUTH-1** (~líneas 24-28). Aplicar texto exacto del design §7:
  > **Then** it MUST read **secret** credentials (`JIRA_EMAIL`, `JIRA_API_TOKEN`) exclusively from the process environment and MUST NOT read or write any other **secret** credential store. Secrets MUST originate only from a gitignored `.env` or inherited environment — never committed, never passed as flags, never logged. **Non-secret project identity** (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) MAY be sourced from a committed config file (`.talos/project.env`), loaded into the environment at startup; the precedence is "real environment wins over file" (`envfile.LoadInto`, if-unset).

- [ ] **PR2-T6** `platform/jira-evidence-loop/service/config.go:15-16` — **actualizar comentario de `Credentials`**:
  ```go
  // Credentials holds the Jira secret auth data sourced exclusively from the
  // environment (gitignored .env or inherited env). Secrets must never appear in
  // flags, logs, or committed config; non-secret project identity (site/key/id) may.
  ```

### Archivo de identidad y comentarios mirror

- [ ] **PR2-T7** `.talos/project.env` (NUEVO, committeable) — crear con contenido exacto:
  ```
  # .talos/project.env — identidad del proyecto (runtime de las CLIs Go). NO-secreto, committear.
  # mirror de openspec/config.yaml:jira — mantener en sync (project_key / project_id / site).
  JIRA_SITE_URL=https://tablex.atlassian.net
  JIRA_PROJECT_KEY=TAL
  JIRA_PROJECT_ID=10099
  ```
  Verificar que `.gitignore:3` (`.env.*`) NO lo captura (la línea es `.env.*`, y `.talos/project.env` no empieza con `.env`).

- [ ] **PR2-T8** `.env.example` — agregar comentario `# mirror de .talos/project.env para identidad no-secreta` y asegurar que las 3 variables de identidad (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) tienen nota que también viven en `.talos/project.env`.

- [ ] **PR2-T9** `openspec/config.yaml` — agregar comentario en el bloque `jira:` (~líneas 46-49): `# mirror de .talos/project.env — mantener en sync (project_key / project_id / site)`.

> **Criterio de merge PR2:** `go test ./... -count=1` en `platform/jira-evidence-loop/` → PASS (tests `run()` existentes siguen verdes — REQ-TEST-5). `.talos/project.env` commiteado y visible en `git status`.

---

## PR3 — `ch`: loader copy + `branch.go` param + override + threading (HERMES)

> **Archivos:** `platform/ci-checks/internal/envfile/envfile.go` (nuevo), `platform/ci-checks/internal/envfile/envfile_test.go` (nuevo), `platform/ci-checks/domain/cichecks/branch.go`, `platform/ci-checks/domain/cichecks/branch_test.go`, `platform/ci-checks/cmd/ch/main.go`
> **REQ-IDs:** REQ-ENV-1 (ch), REQ-BRANCH-1..5, REQ-IDENT-1 (ch), REQ-TEST-3, REQ-TEST-5
> **Depende de:** PR1 mergeado (referencia canónica para la copia verbatim)

### Copia verbatim del envfile canónico

- [ ] **PR3-T1** `platform/ci-checks/internal/envfile/envfile.go` — **copiar VERBATIM** desde `platform/jira-evidence-loop/internal/envfile/envfile.go` (PR1). Solo cambiar el `package envfile` → sigue siendo `package envfile`. NO modificar ningún símbolo ni lógica (ADR-J8).

- [ ] **PR3-T2** `platform/ci-checks/internal/envfile/envfile_test.go` — **copiar VERBATIM** desde `platform/jira-evidence-loop/internal/envfile/envfile_test.go`. Verificar: `go test ./internal/envfile/... -count=1` en `ci-checks/` → PASS.

### L3 — Tests de `branch.go` parametrizado (RED primero)

- [ ] **PR3-T3** `platform/ci-checks/domain/cichecks/branch_test.go` — **actualizar** `TestParseAgentBranch` (table-driven) para agregar columna `projectKey` y los casos del spec (REQ-BRANCH-1..5, REQ-TEST-3):
  - Columna `projectKey string` en la struct del test.
  - Fila existente `TAL / agent/hermes/TAL-7 → válida` → ahora `projectKey:"TAL"`.
  - Fila `TAL / agent/hermes/FOO-7 → ErrNoJiraKey` (REQ-BRANCH-3).
  - Fila **nueva** `FOO / agent/hermes/FOO-7 → válida` (REQ-BRANCH-4).
  - Fila `FOO / agent/hermes/TAL-7 → ErrNoJiraKey` (REQ-BRANCH-5).
  - Llamada: `cichecks.ParseAgentBranch(tc.branch, tc.projectKey)`.
  - Confirmar FALLA (RED — firma actual no acepta `projectKey`).

- [ ] **PR3-T4** `platform/ci-checks/domain/cichecks/branch.go` — **parametrizar `ParseAgentBranch`**:
  - **Eliminar** `var branchRE = regexp.MustCompile(...)` (package-level).
  - Nueva firma: `func ParseAgentBranch(branch, projectKey string) (figura, key string, err error)`.
  - Compilar regex inline: `regexp.MustCompile("^agent/([a-z]+)/(" + regexp.QuoteMeta(projectKey) + "-[0-9]+)$")`.
  - Godoc (1 línea): `// ParseAgentBranch extracts the agent figura and JIRA key from a canonical branch name.`
  - Confirmar `go test ./domain/cichecks/... -count=1` PASA (GREEN, L3).

### Threading del projectKey en `ch/main.go`

- [ ] **PR3-T5** `platform/ci-checks/cmd/ch/main.go` — **cambiar firma de `extractBranchParts`** de `extractBranchParts(branch string)` a `extractBranchParts(branch, projectKey string)`:
  - Dentro de `extractBranchParts`: pasar `projectKey` al llamar `cichecks.ParseAgentBranch(branch, projectKey)`.
  - En el call site `cmdLabels:127`: cambiar a `extractBranchParts(normalizedBranch, cfg.Project)`. (`cfg` ya existe en `cmdLabels` scope, `main.go:120`.)

- [ ] **PR3-T6** `platform/ci-checks/cmd/ch/main.go` — **reword de comentarios** con literales `TAL-N` → `<projectKey>-N` (genérico). Revisar: `normalizeBranchFigura` (`main.go:145`), `cmdLabels:106`, y cualquier otro comentario inline que mencione `TAL-N` literalmente.

### Override de identidad y hoist del loader en `ch`

- [ ] **PR3-T7 [R8 CRÍTICO]** `platform/ci-checks/cmd/ch/main.go` — **hoist `LoadInto` al TOPE de `run()`**, ANTES del `switch args[0]` y ANTES del `flag.NewFlagSet` de `cmdLabels` (que contiene `fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)` en `ch:94`):
  ```go
  root := repoRoot()
  mainRoot := mainWorktreeRoot()
  _ = envfile.LoadInto(os.Setenv, os.Getenv,
      root+"/.talos/project.env",
      root+"/.env",
      mainRoot+"/.env",
  )
  ```
  Agregar `mainWorktreeRoot()` helper si no existe en `ch/main.go` (design §4; `repoRoot()` ya existe en `ch:58-65`).

- [ ] **PR3-T8** `platform/ci-checks/cmd/ch/main.go` — **override de `cfg.Project`** tras `DefaultTALConfig()` en los DOS seams (`cmdLabels:120` y `cmdOwnership:256`):
  ```go
  cfg := service.DefaultTALConfig()
  if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }
  ```
  _(Campo: `Project` — NO `ProjectKey`; verificar contra `ci-checks/service/config.go:6`.)_

### Verificación back-compat ch

- [ ] **PR3-T9** Confirmar que los tests existentes de `ch` (`cmd/ch/main_test.go`, `domain/cichecks/branch_test.go`) siguen verdes: `go test ./... -count=1` en `platform/ci-checks/` → PASS (REQ-TEST-5).

- [ ] **PR3-T10** Agregar imports necesarios en `ch/main.go`: `"platform/ci-checks/internal/envfile"` (o path relativo del módulo Go), `"os"` si faltara, `"path/filepath"` para `mainWorktreeRoot()`.

> **Criterio de merge PR3:** `go test ./... -count=1` en `platform/ci-checks/` → PASS. `ParseAgentBranch` acepta `projectKey`. Override de `cfg.Project` presente en ambos seams. Loader hoisted (ANTES de cualquier `fs.String`).

---

## PR4 — `wt`: `naming.go` param + `Config.Project` + threading + LoadInto (HERMES)

> **Archivos:** `platform/worktree-orchestrator/domain/worktree/naming.go`, `platform/worktree-orchestrator/domain/worktree/naming_test.go`, `platform/worktree-orchestrator/service/orchestrator.go`, `platform/worktree-orchestrator/cmd/wt/main.go`
> **REQ-IDs:** REQ-WTKEY-1..4, REQ-IDENT-1 (wt), REQ-WT-1, REQ-COMPAT-1, REQ-TEST-4..5
> **Depende de:** PR1 mergeado (para la copia de `envfile` si se decide incluirla en wt; ver tarea PR4-T5)
> **Invariante:** `env_test.go:107` golden (worktree `.env` sin `JIRA_`) → NO MODIFICAR (ADR-J10)

### L3 — Tests de `naming.go` parametrizado (RED primero)

- [ ] **PR4-T1** `platform/worktree-orchestrator/domain/worktree/naming_test.go` — **actualizar** tests de `ValidateJiraKey` y `NewWorktreeSpec` para threaded `projectKey` (REQ-WTKEY-1..4, REQ-TEST-4):
  - `TestValidateJiraKey`: llamadas existentes pasan `"TAL"` explícito como segundo parámetro. Agregar sub-test `"FOO/FOO-7 valid"` (`ValidateJiraKey("FOO-7", "FOO")` → `nil`) y `"FOO/TAL-5 rejected"` (`ValidateJiraKey("TAL-5", "FOO")` → `ErrNoJiraKey`/`ErrInvalidKey`).
  - `TestNewWorktreeSpec`: llamadas existentes pasan `"TAL"` como cuarto argumento.
  - Confirmar FALLA (RED — firmas actuales no aceptan `projectKey`).

- [ ] **PR4-T2** `platform/worktree-orchestrator/domain/worktree/naming.go` — **parametrizar `ValidateJiraKey`**:
  - **Eliminar** `var jiraKeyRe = regexp.MustCompile(...)` (package-level).
  - Nueva firma: `func ValidateJiraKey(key, projectKey string) error`.
  - Regex inline: `regexp.MustCompile("^" + regexp.QuoteMeta(projectKey) + "-[1-9][0-9]*$")`.
  - Godoc (1 línea): `// ValidateJiraKey checks that key matches <projectKey>-<n> (n >= 1), returning ErrInvalidKey if not.`
  - Confirmar `go test ./domain/worktree/... -count=1` PASA para `ValidateJiraKey` (GREEN, L3 parcial).

- [ ] **PR4-T3** `platform/worktree-orchestrator/domain/worktree/naming.go` — **actualizar `NewWorktreeSpec`** para recibir `projectKey` como cuarto parámetro y pasarlo a `ValidateJiraKey`:
  - Nueva firma: `func NewWorktreeSpec(figura, jiraKey, worktreeBase, projectKey string) (WorktreeSpec, error)`.
  - Godoc (1 línea): `// NewWorktreeSpec validates figura and jiraKey, then builds a WorktreeSpec with pre-derived Branch and Path.`
  - Confirmar `go test ./domain/worktree/... -count=1` PASA (GREEN, L3 completo).

### `Config.Project` — seeding en service y wiring en orchestrator

- [ ] **PR4-T4** `platform/worktree-orchestrator/service/orchestrator.go:22-29` — **agregar campo `Project string` al struct `Config`**:
  ```go
  type Config struct {
      RepoRoot     string
      WorktreeBase string
      BaseBranch   string
      Project      string // Jira project key prefix for key validation; default "TAL".
  }
  ```
  Godoc del campo: `// Project is the Jira project key prefix used to validate worktree keys; default "TAL".`
  En `DefaultTALConfig()` (`orchestrator.go:32-37`): agregar `Project: "TAL"` (back-compat).

- [ ] **PR4-T5** `platform/worktree-orchestrator/cmd/wt/main.go` — **LoadInto solo con project.env** + **seedear `cfg.Project`** en `newOrchestrator()` (`wt/main.go:68-74`):
  - Agregar `internal/envfile/` al módulo de `wt` (copia verbatim de PR1 o inline — ver nota abajo).
  - En `newOrchestrator()`, ANTES de `os.Getenv("JIRA_PROJECT_KEY")`:
    ```go
    root := repoRoot()
    _ = envfile.LoadInto(os.Setenv, os.Getenv, root+"/.talos/project.env")  // solo identidad, sin .env
    cfg := service.DefaultTALConfig()
    cfg.RepoRoot = root
    if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }
    ```
  - **Nota:** `wt` NO carga `.env` (no toca creds Jira, ADR-J7/diseño §5). Solo `project.env` para identidad.
  - Si `worktree-orchestrator` no tiene `internal/envfile/` propio, crearlo copiando verbatim de PR1 (mismo patrón ADR-J8). Sin este loader, `wt` depende del env real — back-compat OK (`TAL` por default), pero el archivo `.talos/project.env` no surtiría efecto para `wt` (degrade de portabilidad).

- [ ] **PR4-T6** `platform/worktree-orchestrator/service/orchestrator.go:59,150,202` — **threadear `o.cfg.Project` a los 3 call sites de `NewWorktreeSpec`**:
  ```go
  spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase, o.cfg.Project)
  ```
  Los 3 call sites son: Create (`orchestrator.go:59`), Teardown (`orchestrator.go:150`), Env (`orchestrator.go:202`).

### Invariante golden (NO tocar)

- [ ] **PR4-T7** `platform/worktree-orchestrator/domain/worktree/env_test.go:107` — **verificar que el golden test sigue intacto**: `RenderEnv` en `env.go:40-45` sigue emitiendo SOLO `PORT` + `DB_SCHEMA`. NO agregar ni quitar líneas del golden. Este test NO debe ser modificado (ADR-J10, load-bearing).

### Verificación back-compat wt

- [ ] **PR4-T8** Confirmar `go test ./... -count=1` en `platform/worktree-orchestrator/` → PASS. Tests existentes de `wt` (`cmd/wt/main_test.go`, `domain/worktree/naming_test.go`, `domain/worktree/env_test.go`) siguen verdes (REQ-TEST-5).

> **Criterio de merge PR4:** `go test ./... -count=1` en `platform/worktree-orchestrator/` → PASS. `ValidateJiraKey` y `NewWorktreeSpec` aceptan `projectKey`. `Config.Project` existe con `"TAL"` por default. Golden de `env_test.go:107` inalterado. `newOrchestrator()` carga `project.env` y seedea `cfg.Project`.

---

## PR5 — `ov`: loader copy + override (THEMIS, HERMES sign-off)

> **Archivos:** `platform/overlap-guard/internal/envfile/envfile.go` (nuevo), `platform/overlap-guard/internal/envfile/envfile_test.go` (nuevo), `platform/overlap-guard/cmd/ov/main.go`
> **REQ-IDs:** REQ-ENV-1 (ov), REQ-IDENT-1 (ov), REQ-TEST-2 (ov copy), REQ-TEST-5
> **Depende de:** PR1 mergeado (referencia canónica para copia verbatim)
> **Cross-module §2:** THEMIS es owner de `overlap-guard/**`; HERMES da sign-off (config.yaml:34, regla reviewer-distinto).
> **Sign-off note:** HERMES revisa la copia de `envfile.go` de ov contra el canónico de PR1 byte-a-byte (ADR-J8).

### Copia verbatim del envfile canónico

- [ ] **PR5-T1** `platform/overlap-guard/internal/envfile/envfile.go` — **copiar VERBATIM** desde `platform/jira-evidence-loop/internal/envfile/envfile.go` (canónico de PR1). Solo cambiar `package envfile` si aplica, pero el nombre del paquete debe ser `envfile`. NO modificar ningún símbolo ni lógica.

- [ ] **PR5-T2** `platform/overlap-guard/internal/envfile/envfile_test.go` — **copiar VERBATIM** desde `platform/jira-evidence-loop/internal/envfile/envfile_test.go`. Confirmar: `go test ./internal/envfile/... -count=1` en `overlap-guard/` → PASS.

### R8 — hoist del loader en `ov/main.go`

- [ ] **PR5-T3 [R8 CRÍTICO]** `platform/overlap-guard/cmd/ov/main.go` — **hoist `LoadInto` al TOPE de `run()`**, ANTES del `switch args[0]` y ANTES de `cmdCheck` que contiene `fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)` en `ov:243`:
  ```go
  root := repoRoot()
  mainRoot := mainWorktreeRoot()
  _ = envfile.LoadInto(os.Setenv, os.Getenv,
      root+"/.talos/project.env",
      root+"/.env",
      mainRoot+"/.env",
  )
  ```
  Agregar `mainWorktreeRoot()` helper (~8 LOC, design §4) si no existe en `ov/main.go` (`repoRoot()` ya existe en `ov/main.go`).

### Override de identidad en `ov`

- [ ] **PR5-T4** `platform/overlap-guard/cmd/ov/main.go` — **override de `cfg.Project`** tras `DefaultTALConfig()` en los 3 seams de ov (`cmdCheck:268`, `cmdScan:302`, `cmdMetric:342`):
  ```go
  cfg := service.DefaultTALConfig()
  if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }
  ```
  _(Campo: `Project` — NO `ProjectKey`; verificar contra `overlap-guard/service/config.go:14`. ov NO tiene ProjectID en su Config.)_
  Agregar imports necesarios: `"platform/overlap-guard/internal/envfile"`, `"path/filepath"`.

### Verificación back-compat ov

- [ ] **PR5-T5** Confirmar `go test ./... -count=1` en `platform/overlap-guard/` → PASS. Tests existentes de `ov` siguen verdes (REQ-TEST-5).

> **Criterio de merge PR5:** `go test ./... -count=1` en `platform/overlap-guard/` → PASS. Loader hoisted (ANTES de `cmdCheck`). Override de `cfg.Project` en los 3 seams. `envfile.go` de ov idéntico byte-a-byte al canónico de PR1 (HERMES verifica antes de sign-off).
> **HERMES sign-off REQUERIDO** antes de merge (cross-module §2, config.yaml:34).

---

## Cleanup y archivos auxiliares

- [ ] **CL-1** `team-context/ownership.md` — verificar que no falta ninguna fila nueva para los archivos introducidos por este change (`.talos/project.env`, `internal/envfile/` en los 3 módulos). Si la convención del proyecto requiere registro en ownership, actualizar.
- [ ] **CL-2** Verificar `.gitignore` respecto a `.talos/project.env`: confirmar que `.gitignore:3` (`.env.*`) NO captura `.talos/project.env`. Ejecutar `git check-ignore -v .talos/project.env` → debe retornar nada (no ignorado).
- [ ] **CL-3** Confirmar que en todos los módulos (`jira-evidence-loop`, `ci-checks`, `overlap-guard`, `worktree-orchestrator`) el `go.mod` no tiene dependencias externas nuevas (cero `require` nuevos — cero-dep invariante).

---

## Completion

**Status: PENDING**

Criterios de cierre del change (todos los PRs mergeados):
- `go test ./... -count=1` en los 4 módulos afectados → PASS (0 FAIL).
- `JIRA_PROJECT_KEY=FOO go run ./cmd/ch/... labels --branch agent/hermes/FOO-7` → parsea (no `ErrNoJiraKey`).
- `JIRA_PROJECT_KEY=FOO go run ./cmd/ch/... labels --branch agent/hermes/TAL-7` → `ErrNoJiraKey` (correcto).
- Sin `.talos/project.env` y sin env override → `DefaultTALConfig` devuelve `TAL`/`10099` (REQ-COMPAT-1).
- CI existente continúa verde sin cambios en el pipeline.
- `.talos/project.env` commiteado y visible en git (`git check-ignore -v .talos/project.env` → nada).
- `env_test.go:107` golden inalterado (worktree `.env` sin `JIRA_`).
- HERMES sign-off registrado en PR5 (cross-module §2).
- verify-report: 0 CRITICAL.
- Archive ejecutado → `openspec/changes/archive/` + engram `sdd/jira-portability/archive-report`.
