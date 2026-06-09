# Design: jira-portability

**Change:** jira-portability · **Jira:** TAL-6 · **Fase:** 2 · **Modules:** `module:devops` (ch/wt) + `module:jira-loop` (evidence) + `module:qa` (ov)
**Owner:** HERMES (lidera) + THEMIS (slice `ov`) · **Store:** hybrid (este archivo + engram `sdd/jira-portability/design`)
**Reads:** proposal (#1893, 9 decisiones resueltas D1–D9, ZEUS) + plan aprobado (`toasty-jellyfish`, file:line + 5-PR breakdown + R1–R8)
**Mirror reference:** `openspec/changes/archive/2026-06-08-merge-order-automation/design.md` (diagrama, package layout, firmas, ADRs numerados)

Este es el **HOW** a nivel arquitectónico. NO enumera los pasos de tareas — `sdd-tasks` convierte estos
boundaries en un backlog strict-TDD ordenado (RED→GREEN por capa L1/L2/L3/L4). Los requisitos funcionales
(el **WHAT**) los posee `sdd-spec`; §12 marca los seams donde ambas fases se tocan.

Este documento FORMALIZA el approach ya aprobado por ZEUS. Las nueve forks se resolvieron en el proposal;
§10 las registra como ADRs (ADR-J1..J9). **Todas las firmas de abajo están aterrizadas contra código real**
(`evidence/cmd/evidence/main.go`, `ci-checks/cmd/ch/main.go` + `domain/cichecks/branch.go` + `service/config.go`,
`overlap-guard/cmd/ov/main.go` + `service/config.go`, `worktree-orchestrator/cmd/wt/main.go` +
`domain/worktree/naming.go` + `service/orchestrator.go` + `domain/worktree/env.go`), con cita `file:línea`.

---

## 1. Architecture approach

La pieza central NO es un módulo hexagonal nuevo: es un **paquete utilitario `internal/envfile`** (sin puertos,
sin adapters — es plumbing del composition root) más **una parametrización quirúrgica del dominio**. El invariante
arquitectónico que este change defiende es:

> **El dominio sigue PURO.** No recibe env, no recibe config, no lee archivos. Recibe `projectKey` **por parámetro**
> y arma su regex con `regexp.QuoteMeta`. El composition root (`cmd/*`) es el ÚNICO lugar que toca `os.Getenv`,
> lee archivos `.env` y resuelve la identidad del proyecto.

El flujo de carga es una **cadena de precedencia** que corre **una sola vez, al tope de `run()`**, antes de cualquier
`flag.NewFlagSet` cuyo default lea `os.Getenv` (esto es **R8**, el bug más probable — ver §3). El `.env` solo **puebla**
el entorno; los `os.Getenv` existentes quedan IGUAL (no-invasivo). El env real (CI/shell) gana siempre.

```
  cmd/<evidence|ch|ov>  (composition root — el ÚNICO que toca os.Getenv / archivos)
  ┌──────────────────────────────────────────────────────────────────────────────┐
  │ run(args):                                                                     │
  │   ① root  := repoRoot()                  // git rev-parse --show-toplevel      │
  │   ② mainR := mainWorktreeRoot()          // git rev-parse --git-common-dir (b) │
  │   ③ envfile.LoadInto(os.Setenv, os.Getenv,                                     │
  │          root+"/.talos/project.env",     // identidad NO-secreta (tracked)     │
  │          root+"/.env",                   // secrets (gitignored)               │
  │          mainR+"/.env")                  // fallback worktree (b) — best-effort │
  │      ▲ ANTES del switch de subcomandos y de cualquier flag default (R8)        │
  │   ④ switch args[0] { case "...": cmdXxx(...) }                                 │
  └───────────────────┬──────────────────────────────┬─────────────────────────────┘
                      │ flag default os.Getenv         │ override 3 escalares
                      │ ("JIRA_SITE_URL") — ya poblado  │ cfg.Project / cfg.ProjectID
                      ▼                                 ▼
        ┌──────────────────────────┐      ┌──────────────────────────────────┐
        │ service.DefaultTALConfig │ ───► │ if v:=os.Getenv("JIRA_PROJECT_KEY")│
        │ (TAL/10099 — NO renombra)│      │   ;v!="" { cfg.Project = v }       │
        └──────────────────────────┘      └─────────────┬────────────────────┘
                                                        │ cfg.Project (projectKey)
                                                        ▼
        ┌────────────────────────────────────────────────────────────────────┐
        │  domain  (PURO — recibe projectKey por parámetro, NO lee env)        │
        │  cichecks.ParseAgentBranch(branch, projectKey)                       │
        │  worktree.ValidateJiraKey(key, projectKey)                           │
        │  worktree.NewWorktreeSpec(figura, jiraKey, base, projectKey)         │
        │  regex := "^...(" + regexp.QuoteMeta(projectKey) + "-[0-9]+)$"       │
        └────────────────────────────────────────────────────────────────────┘
```

**Precedencia (de mayor a menor):**

| # | Fuente | Cuándo gana |
|---|--------|-------------|
| 1 | **Env real** (CI secret / shell `export` / env heredado de ATHENA) | SIEMPRE — `LoadInto` solo setea si `getenv(key)==""` |
| 2 | `root/.talos/project.env` (identidad, tracked) | si la key no está en env — primer arg |
| 3 | `root/.env` (secrets, gitignored) | si la key no está en env ni en project.env — segundo arg |
| 4 | `mainRoot/.env` (fallback worktree, best-effort) | último arg — solo llena gaps en worktrees sin `.env` propio |

Como `LoadInto` aplica **if-unset por orden de args**, la precedencia es exactamente el orden en que se pasan los
paths. CI sin cambios: el env real de los secrets gana, los `.env` ni hace falta que existan.

**Dónde está el valor (honesto):** el corazón puro y exhaustivamente testeable es `Parse` (`[]byte → map`, sin I/O)
y la parametrización de las regex de dominio. `LoadInto` es precedencia + skip-not-exist, testeable con `setenv`/`getenv`
inyectados como closures sobre maps (sin tocar el env del proceso). El wiring en los `cmd/*` es mecánico.

---

## 2. El paquete `envfile`

**Package layout** (reimplementado por módulo Jira — NO cross-import; cero-dep prohíbe `go.mod` cruzados, mismo
precedente que `evidence.ValidateOwnership` en `domain/evidence/ownership.go:35`):

```
platform/jira-evidence-loop/internal/envfile/envfile.go        ← CANÓNICO (PR1, keystone)
platform/jira-evidence-loop/internal/envfile/envfile_test.go   ← tests L1+L2 canónicos
platform/ci-checks/internal/envfile/envfile.go                 ← copia VERBATIM (PR3)
platform/ci-checks/internal/envfile/envfile_test.go
platform/overlap-guard/internal/envfile/envfile.go             ← copia VERBATIM (PR5, THEMIS)
platform/overlap-guard/internal/envfile/envfile_test.go
```

`internal/` garantiza que nadie fuera del módulo lo importe por accidente (no es API pública). La copia de ch/ov
es un **drop verbatim** del canónico de jira-evidence-loop — THEMIS revisa su copia de `ov` contra el de HERMES como
referencia byte-a-byte (ADR-J8).

### Firmas (cada símbolo exportado lleva EXACTAMENTE una línea godoc — convención code-comments)

```go
// Package envfile parses KEY=VALUE files and loads them into the environment
// without overwriting variables already set. Zero third-party deps.
package envfile

// Parse converts KEY=VALUE bytes into a map, skipping blank and #-comment lines.
func Parse(b []byte) map[string]string

// LoadInto sets each parsed key via setenv only when getenv reports it unset,
// reading paths in order so earlier paths win; missing files are skipped.
func LoadInto(setenv func(k, v string) error, getenv func(k string) string, paths ...string) error
```

### Contrato de `Parse` (PURO — L1)

Recibe `[]byte`, devuelve `map[string]string`, **no toca el env, no hace I/O**:

- Itera línea por línea.
- Hace `strings.TrimRight(line, "\r")` (tolerante a CRLF) y luego `strings.TrimSpace`.
- **Skip** si la línea queda vacía o empieza con `#`.
- Split en el **primer** `=` (`strings.SplitN(line, "=", 2)`): `KEY=a=b=c` → key `KEY`, value `a=b=c`.
- Línea sin `=` → se ignora (no es par válido).
- `key = TrimSpace(parts[0])`, `value = TrimSpace(parts[1])`. Key vacía tras trim → skip.
- Entrada vacía → `map` vacío (no `nil` — devolver `map[string]string{}` inicializado).

### Contrato de `LoadInto` (precedencia + if-unset — L2)

```
for _, path := range paths {           // orden de args = orden de precedencia
    b, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) { continue }   // archivo faltante → skip silencioso
        return err                            // primer error NO-not-exist burbujea
    }
    for k, v := range Parse(b) {
        if getenv(k) == "" {                  // if-unset: env real y paths previos ganan
            if err := setenv(k, v); err != nil { return err }
        }
    }
}
return nil
```

- **`setenv`/`getenv` inyectables** → en producción `os.Setenv`/`os.Getenv`; en test, closures sobre un `map`
  (sin tocar el env del proceso, sin I/O salvo `os.ReadFile` de fixtures en tempdir).
- **Precedencia = orden de args + if-unset:** un key seteado por el primer path (o ya en env) NO lo pisa el segundo.
- **Skip not-exist:** `os.IsNotExist(err)` → `continue` (un `.talos/project.env` o `.env` ausente es válido).
- **Error real burbujea:** cualquier otro error de lectura (permiso, etc.) se retorna — el **primero**, sin seguir.

---

## 3. Composition-root wiring (evidence / ch / ov)

**El punto CRÍTICO (R8, ADR-J3):** `LoadInto` DEBE correr ANTES de cualquier `flag.NewFlagSet` cuyo default lea
`os.Getenv`. Hoy esos defaults existen en:

- `ch/main.go:94` → `fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)`
- `ov/main.go:243` → `fs.String("site-url", os.Getenv("JIRA_SITE_URL"), ...)`

El default del flag se evalúa **en el momento de `fs.String(...)`**, dentro de `cmdLabels`/`cmdCheck`. Por eso el loader
**NO puede ir adentro del subcomando** — debe ir al **tope de `run()`, antes del `switch`**, un solo lugar por main.

### Inserción exacta — al tope de cada `run()`

```go
func run(args []string /*, out io.Writer en ch*/) error {
	root := repoRoot()
	mainRoot := mainWorktreeRoot()                              // §4 — fallback (b)
	_ = envfile.LoadInto(os.Setenv, os.Getenv,                 // best-effort: no abortar
		root+"/.talos/project.env",
		root+"/.env",
		mainRoot+"/.env",
	)
	if len(args) == 0 {
		return fmt.Errorf("subcommand required: ...")           // mensaje existente, sin tocar
	}
	switch args[0] {
	// ... cases existentes ...
	}
}
```

- El error de `LoadInto` se **descarta deliberadamente** (`_ =`): el loader es best-effort (D4/ADR-J4 — NO fail-fast).
  La validación de creds la hace cada CLI más abajo, intacta (§8).
- **`evidence` NO tiene `repoRoot()`** (verificado: `cmd/evidence/main.go` no lo define). Hay que **agregarlo**
  (~7 LOC, copia verbatim de `ch/main.go:58-65`):

```go
func repoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	wd, _ := os.Getwd()
	return wd
}
```

  Esto exige agregar `"os/exec"` y `"strings"` a los imports de `evidence/cmd/evidence/main.go`. En `evidence` el
  loader va al tope de `run()` (`main.go:35`), antes del `switch args[0]` (`main.go:39`).

### Override de los 3 escalares de identidad (tras `DefaultTALConfig()`)

**OJO al nombre del field — difiere entre módulos:**

| Módulo | Seam de seeding (file:línea) | Field | Override |
|--------|------------------------------|-------|----------|
| **evidence** | `cmd/evidence/main.go:108-111` (`cfg := service.DefaultTALConfig()`) | `cfg.ProjectKey` / `cfg.ProjectID` | ver abajo |
| **ch** | `cmd/ch/main.go:120` y `:256` (`cfg := service.DefaultTALConfig()`) | `cfg.Project` | ver abajo |
| **ov** | `cmd/ov/main.go:268`, `:302`, `:342` (`cfg := service.DefaultTALConfig()`) | `cfg.Project` | ver abajo |

**evidence** (`service.Config` usa `ProjectKey`/`ProjectID` — `service/config.go:33,36`):

```go
cfg := service.DefaultTALConfig()
if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.ProjectKey = v }
if v := os.Getenv("JIRA_PROJECT_ID");  v != "" { cfg.ProjectID  = v }
cfg.SiteURL = site                            // línea existente :110, sin tocar
cfg.Credentials = service.Credentials{...}    // línea existente :111, sin tocar
```

**ch** (`service.Config` usa `Project` — `service/config.go:6`; **no tiene ProjectID**, solo override de `Project`):

```go
cfg := service.DefaultTALConfig()
if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }
```

**ov** (`service.Config` usa `Project` — `service/config.go:14`; **no tiene ProjectID** en el Config de ov):

```go
cfg := service.DefaultTALConfig()
if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }
```

- `JIRA_SITE_URL` NO necesita override explícito: ya entra por el flag default existente (`ch:94`, `ov:243`) o por la
  resolución de `evidence` (`main.go:100-106`), que ahora ve el env ya poblado por el loader.
- **NO se renombra `DefaultTALConfig`** (R1/ADR-J5): 9 call sites en ch/ov/evidence/wt/mo. Se override en el root.
- En **ch** el `cfg` se construye en DOS lugares (`cmdLabels:120`, `cmdOwnership:256`); el override de `Project` aplica
  en ambos (`ch ownership` no toca regex, pero mantener el patrón es barato y consistente).

---

## 4. Worktree fallback (b) — `mainWorktreeRoot()`

`.talos/project.env` es **tracked** → presente en cada worktree → la identidad resuelve OK. Pero `.env` es **gitignored**
→ NO existe en un worktree nuevo → los secrets no resuelven del `.env` del worktree. Contrato MVP: **(a) env heredado**
(ATHENA exporta antes de spawnear — cero código, ya documentado en `.env.example`; la precedencia "env real gana" es
compatible) **+ (b) fallback** al `.env` del checkout principal. La pieza (b) es esta helper (~8 LOC, best-effort):

```go
// mainWorktreeRoot returns the main checkout's working-tree root via
// git rev-parse --git-common-dir, or "" when it cannot be resolved.
func mainWorktreeRoot() string {
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	common := strings.TrimSpace(string(out)) // ".git" (relativo desde main) o ruta absoluta (desde worktree linkeado)
	if !filepath.IsAbs(common) {
		wd, _ := os.Getwd()
		common = filepath.Join(wd, common)
	}
	return filepath.Dir(common)              // el dir que contiene el .git común = checkout principal
}
```

- **Normalización (R7):** `--git-common-dir` devuelve `.git` **relativo** desde el checkout principal, pero ruta
  **absoluta** desde un worktree linkeado. Por eso el `if !filepath.IsAbs(common)` → `filepath.Join(wd, common)`.
- `filepath.Dir(common)` da el directorio padre del `.git` común = la raíz del checkout principal.
- **Best-effort, lowest precedence:** se appendea **último** en la lista de paths de `LoadInto`. Solo llena gaps —
  en el checkout principal, `mainRoot/.env == root/.env` (mismo path, ya parseado → if-unset lo ignora; sin daño).
- **Repo bare** → no hay working tree → el `.env` no existe en ese path → `LoadInto` lo saltea silencioso (skip not-exist).
- Requiere agregar `"path/filepath"` a los imports de cada `cmd/*` (evidence/ch/ov).

---

## 5. Parametrización de las regex de dominio (matar `TAL-`)

El dominio sigue **PURO**: recibe `projectKey` por parámetro y compila la regex inline con `regexp.QuoteMeta`
(evita inyección de regex si el projectKey trae metacaracteres — ADR-J6).

### `ci-checks/domain/cichecks/branch.go`

**Antes** (`branch.go:5,9`):

```go
var branchRE = regexp.MustCompile(`^agent/([a-z]+)/(TAL-[0-9]+)$`)   // ← se ELIMINA el var package-level
func ParseAgentBranch(branch string) (figura, key string, err error)
```

**Después:**

```go
// ParseAgentBranch extracts the agent figura and JIRA key from a canonical branch name.
func ParseAgentBranch(branch, projectKey string) (figura, key string, err error) {
	re := regexp.MustCompile("^agent/([a-z]+)/(" + regexp.QuoteMeta(projectKey) + "-[0-9]+)$")
	m := re.FindStringSubmatch(branch)
	if m == nil {
		return "", "", ErrNoJiraKey
	}
	return m[1], m[2], nil
}
```

- Se **dropea el `var branchRE` package-level** y se compila inline (el projectKey es runtime, no constante).
- **Caller `ch/main.go:195`** (dentro de `extractBranchParts`): hoy es `cichecks.ParseAgentBranch(branch)`. Pasa a recibir
  el projectKey. Como `extractBranchParts(branch)` (`main.go:194`) no tiene `cfg` en scope, hay que **threadearlo**:
  cambiar la firma a `extractBranchParts(branch, projectKey string)` y en el call de `cmdLabels` (`main.go:127`) pasar
  `extractBranchParts(normalizedBranch, cfg.Project)`. El `cfg` ya existe en `cmdLabels` (`main.go:120`).
- El comentario `// ... TAL-N must remain uppercase` en `normalizeBranchFigura` (`main.go:145`) y en `cmdLabels:106`
  → reword a `<projectKey>-N` (genérico).

### `worktree-orchestrator/domain/worktree/naming.go`

**Antes** (`naming.go:22,33,59`):

```go
var jiraKeyRe = regexp.MustCompile(`^TAL-[1-9][0-9]*$`)             // ← se ELIMINA el var package-level
func ValidateJiraKey(key string) error
func NewWorktreeSpec(figura, jiraKey, worktreeBase string) (WorktreeSpec, error)
```

**Después:**

```go
// ValidateJiraKey checks that key matches <projectKey>-<n> (n >= 1), returning ErrInvalidKey if not.
func ValidateJiraKey(key, projectKey string) error {
	re := regexp.MustCompile("^" + regexp.QuoteMeta(projectKey) + "-[1-9][0-9]*$")
	if !re.MatchString(key) {
		return &ErrInvalidKey{Key: key}
	}
	return nil
}

// NewWorktreeSpec validates figura and jiraKey, then builds a WorktreeSpec with pre-derived Branch and Path.
func NewWorktreeSpec(figura, jiraKey, worktreeBase, projectKey string) (WorktreeSpec, error) {
	f, err := ParseFigura(figura)
	if err != nil {
		return WorktreeSpec{}, err
	}
	if err := ValidateJiraKey(jiraKey, projectKey); err != nil {
		return WorktreeSpec{}, err
	}
	return WorktreeSpec{
		Figura:  f, JiraKey: jiraKey,
		Branch: BranchName(f, jiraKey), Path: WorktreePath(worktreeBase, f),
	}, nil
}
```

### Threading del projectKey en worktree (sin loader — `wt` NO toca creds)

`wt` no carga `.env` (no toca Jira), pero SÍ entra por la regex. El projectKey se thread vía el `Config`:

1. **`worktree/service/orchestrator.go:22-29` — agregar `Project` al `Config`:**

```go
type Config struct {
	RepoRoot     string
	WorktreeBase string
	BaseBranch   string
	Project      string  // ← NUEVO: Jira project key prefix para validar claves; default "TAL".
}
```

   Y en `DefaultTALConfig()` (`orchestrator.go:32-37`) seedear `Project: "TAL"` (mantiene back-compat).

2. **`wt/cmd/wt/main.go:68-74` — seedear desde el env** en el único punto de construcción (`newOrchestrator()`):

```go
func newOrchestrator() *service.Orchestrator {
	root := repoRoot()
	cfg := service.DefaultTALConfig()
	cfg.RepoRoot = root
	if v := os.Getenv("JIRA_PROJECT_KEY"); v != "" { cfg.Project = v }  // ← NUEVO
	runner := gitcli.NewRunner(root)
	return service.NewOrchestrator(runner, cfg)
}
```

   Nota: `wt` NO carga `.env` con `envfile.LoadInto` (no necesita secrets); lee `JIRA_PROJECT_KEY` directo del env.
   Para que el `.talos/project.env` también funcione en `wt`, `newOrchestrator()` puede llamar `envfile.LoadInto`
   (solo `root/.talos/project.env`) **antes** del `os.Getenv("JIRA_PROJECT_KEY")` — decisión menor de tasks; el MVP
   acepta env heredado para `wt`. (Sin loader, `wt` depende del env real o de la futura copia opcional de envfile.)

3. **`orchestrator.go:59,150,202` — threadear `o.cfg.Project` a los 3 call sites de `NewWorktreeSpec`:**

```go
spec, err := worktree.NewWorktreeSpec(figura, jiraKey, o.cfg.WorktreeBase, o.cfg.Project)  // Create:59, Teardown:150, Env:202
```

### Lo que se queda igual (load-bearing)

- **`env_test.go:107` golden** (asserta que el `.env` por-worktree NO contiene `JIRA_`) → **INTACTO**. `RenderEnv`
  (`domain/worktree/env.go:40-45`) sigue emitiendo SOLO `PORT` + `DB_SCHEMA`. El `.env`/`.talos/project.env` son archivos
  separados; el invariante "el `.env` del worktree no lleva secrets de Jira" se mantiene (ADR-J... — §9).

---

## 6. El archivo `.talos/project.env`

**Formato:** `KEY=VALUE`, leído por el MISMO `envfile.Parse` (cero parser nuevo — ADR-J2). 3 keys:

```
# .talos/project.env — identidad del proyecto (runtime de las CLIs Go). NO-secreto, committear.
# mirror de openspec/config.yaml:jira — mantener en sync (project_key / project_id / site).
JIRA_SITE_URL=https://tablex.atlassian.net
JIRA_PROJECT_KEY=TAL
JIRA_PROJECT_ID=10099
```

- **`.gitignore` safety (verificado, ADR-J9):** `.gitignore:3` es `.env.*` con `!.env.example` (`.gitignore:4`).
  `.talos/project.env` **NO matchea `.env.*`** (no empieza con `.env`). **NO nombrarlo `.env.project`** — eso SÍ
  lo atraparía el ignore y nunca se commitearía. El path `.talos/project.env` es seguro de trackear.
- **Comentario espejo en `openspec/config.yaml:46-49`** (bloque `jira:`): agregar `# mirror de .talos/project.env —
  mantener en sync` para que la duplicación deliberada de 3 escalares quede señalada en AMBOS lados (el lint automático
  de sync es follow-up — proposal §Out of scope).
- `openspec/config.yaml:jira` queda como fuente para el harness SDD/humanos; `.talos/project.env` es la fuente **runtime**
  de las CLIs Go.

---

## 7. Enmienda REQ-AUTH (spec + comentario de config.go)

Refinamiento de la convención para distinguir **secret** (nunca committeado) de **identidad no-secreta** (committeable).

### `openspec/specs/jira-loop/functional.md` — REQ-AUTH-1 (`functional.md:23-27`)

**Antes:**

> **Then** it MUST read credentials exclusively from the `JIRA_EMAIL` and `JIRA_API_TOKEN` environment
> variables and MUST NOT read or write any other credential store.

**Después (texto exacto a aplicar):**

> **Then** it MUST read **secret** credentials (`JIRA_EMAIL`, `JIRA_API_TOKEN`) exclusively from the process
> environment and MUST NOT read or write any other **secret** credential store. Secrets MUST originate only from a
> gitignored `.env` or inherited environment — never committed, never passed as flags, never logged.
> **Non-secret project identity** (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) MAY be sourced from a
> committed config file (`.talos/project.env`), loaded into the environment at startup; the precedence is
> "real environment wins over file" (`envfile.LoadInto`, if-unset).

### `evidence/service/config.go:15-16` — comentario de `Credentials`

**Antes:**

```go
// Credentials holds the Jira authentication data sourced exclusively from
// environment variables. They must never appear in flags, logs, or config files.
```

**Después:**

```go
// Credentials holds the Jira secret auth data sourced exclusively from the
// environment (gitignored .env or inherited env). Secrets must never appear in
// flags, logs, or committed config; non-secret project identity (site/key/id) may.
```

---

## 8. Validación de creds por-CLI — UNCHANGED (el loader NO agrega fail-fast)

El loader **solo PUEBLA**. Cada CLI mantiene SU validación existente (D4/ADR-J4):

| CLI / subcomando | ¿Necesita creds? | Validación actual (file:línea) | Cambio |
|------------------|------------------|-------------------------------|--------|
| `evidence run-loop` | **SÍ** — falla fast | `main.go:92-97` (EMAIL/TOKEN obligatorios) | **NINGUNO** — queda igual |
| `ov check` | **SÍ** | usa `os.Getenv` (`ov:263-264`); guard valida T0 | **NINGUNO** |
| `ov scan` / `ov metric` | **NO** — git/wt local | no leen creds | **NINGUNO** |
| `ch labels` (sin `--json`) | **NO** — no toca Jira | creds-opcional (`ch:109-110`) | **NINGUNO** |
| `ch labels --json` | sí lee labels | best-effort (`extractLabels` nil-on-error, `ch:202-214`) | **NINGUNO** |
| `ch ownership` / `ch changed-modules` | **NO** | local | **NINGUNO** |
| `wt *` | **NO** | no toca Jira | **NINGUNO** |

El loader corriendo al tope de `run()` NO debe abortar si falta `.env` (`_ = LoadInto(...)`); romper acá rompería
`ch labels` y `ov scan`, que son creds-opcional. **Fail-fast es responsabilidad de cada subcomando, no del loader.**

---

## 9. Lo que se queda igual

- **`RenderEnv` golden** (`env.go:40-45` + `env_test.go:107`): el `.env` por-worktree sigue siendo SOLO `PORT`+`DB_SCHEMA`.
  Sin `JIRA_`. Invariante load-bearing, intacto.
- **`DefaultTALConfig` NO se renombra** en ninguno de los 4 módulos (ch/ov/evidence/wt) — override en el root (R1).
- **Los `os.Getenv` existentes** (`evidence:90-91,102`, `ch:94,109-110`, `ov:243,263-264`) quedan IGUAL — el loader solo
  puebla el env ANTES; no se reemplaza ninguna lectura.
- **Back-compat:** sin `.talos/project.env` y sin env override, `DefaultTALConfig` devuelve `TAL`/`10099` y las regex
  matchean `TAL-N`. CI sin cambios (env real de secrets gana). Additive, cero flag-day (R2).

---

## 10. ADRs (Architecture Decision Records)

| ADR | Decisión | Por qué (forzado por) |
|-----|----------|----------------------|
| **ADR-J1** | **Desacople TOTAL** (secrets locales + matar hardcode TAL/site/project_key, **incluida la regex de dominio**), no solo secrets | ZEUS (D1): portabilidad real a cualquier Jira; con la regex hardcodeada `FOO-N` ni parsea |
| **ADR-J2** | `.talos/project.env` en **`KEY=VALUE`, mismo `envfile.Parse`** (no YAML de config.yaml, no JSON nuevo) | D2: cero-dep, cero parser nuevo; parsear YAML es frágil, un 2do formato duplica superficie |
| **ADR-J3** | **Hoist `LoadInto` al tope de `run()`**, antes del `switch` y de cualquier flag default `os.Getenv` | D3/R8 (el bug más probable): el flag default se evalúa en `fs.String(...)` (`ch:94`,`ov:243`); el env debe estar poblado antes |
| **ADR-J4** | **NO fail-fast en el loader** (`_ = LoadInto(...)`); cada CLI mantiene su validación | D4: `ch labels`/`ov scan` son creds-opcional; romper en el loader los rompería |
| **ADR-J5** | **NO renombrar `DefaultTALConfig`** → override de los escalares de identidad en el composition root | D5/R1: 9 call sites; rename = blast-radius gratis |
| **ADR-J6** | **`projectKey` por parámetro + `regexp.QuoteMeta`**, dominio sigue PURO (regex compilada inline, drop del `var` package-level) | D6: no inyectar env/config en el dominio; `QuoteMeta` evita inyección de regex |
| **ADR-J7** | **Creds en worktree = (a) env heredado + (b) fallback** al `.env` principal vía `--git-common-dir` (best-effort) | D7/R7: `.env` gitignored no existe en worktree nuevo; `.talos/project.env` (tracked) sí; (b) normaliza relativo/absoluto, skip bare |
| **ADR-J8** | **`envfile` canónico en jira-evidence-loop, copia VERBATIM a ch/ov** (`internal/`) | D8: cero-dep prohíbe cross-import (go.mod separados); mismo patrón que `ValidateOwnership`; THEMIS revisa su copia vs HERMES |
| **ADR-J9** | **`.talos/project.env`, NO `.env.project`** | D9: `.gitignore:3` (`.env.*`) atraparía `.env.project`; `.talos/project.env` no matchea (verificado) |
| **ADR-J10** | **`env_test.go:107` golden se mantiene** (`.env` por-worktree sin `JIRA_`); `.env` y `.talos/project.env` son archivos separados | El invariante "el `.env` del worktree no lleva secrets" es ortogonal a este change y sigue válido |

---

## 11. Ownership / PR map (cruza §2 — HERMES + THEMIS)

| Archivos | Owner |
|----------|-------|
| `jira-evidence-loop/**`, `ci-checks/**`, `worktree-orchestrator/**`, `.talos/project.env`, `.env.example`, `openspec/*` | **HERMES** (lidera) |
| `overlap-guard/**` (loader copy + override) | **THEMIS** (1 PR, **HERMES sign-off**) |

### 5 PRs encadenados (~630 LOC, cada PR <400 — sin `size:exception`)

| PR | Owner | Scope | Archivos | ~LOC |
|----|-------|-------|----------|------|
| **PR1** | HERMES | **`envfile` canónico + tests (keystone)** | `jira-evidence-loop/internal/envfile/{envfile,envfile_test}.go` | ~110 |
| **PR2** | HERMES | wire loader + override en `evidence`; `repoRoot()`+`mainWorktreeRoot()`; enmienda REQ-AUTH; crear `.talos/project.env`; comments `.env.example`/`config.yaml` | `evidence/cmd/evidence/main.go`, `evidence/service/config.go`, `specs/jira-loop/functional.md`, `.talos/project.env`, `.env.example`, `openspec/config.yaml` | ~120 |
| **PR3** | HERMES | `ch`: copy loader + `branch.go` param + override + thread `extractBranchParts` + tests | `ci-checks/internal/envfile/*`, `ci-checks/domain/cichecks/branch.go`, `ci-checks/cmd/ch/main.go`, tests | ~150 |
| **PR4** | HERMES | `wt`: `naming.go` param + `Config.Project` + threading 3 call sites + tests (SIN loader) | `worktree-orchestrator/domain/worktree/naming.go`, `service/orchestrator.go`, `cmd/wt/main.go`, tests | ~140 |
| **PR5** | **THEMIS** | `ov`: copy loader (verbatim de PR1) + override (**HERMES sign-off**) | `overlap-guard/internal/envfile/*`, `overlap-guard/cmd/ov/main.go` | ~110 |

- **PR1 es keystone** (fija el patrón `envfile`). **PR3/PR4/PR5 paralelizables** una vez mergeado PR1.
- **Cross-module §2:** el slice de `ov` (PR5) es de THEMIS (`module:qa`) → su **propio PR** con **HERMES sign-off**
  registrado en `tasks.md` (regla reviewer-distinto, `config.yaml:34`). `envfile.go` canónico de HERMES (PR1) es la
  referencia de revisión byte-a-byte (ADR-J8).

---

## 12. Seams con sdd-spec (el WHAT)

- **REQ-AUTH** (enmienda §7) la posee `sdd-spec` como texto de spec; este design fija la **redacción exacta**.
- Los criterios de aceptación de portabilidad (`JIRA_PROJECT_KEY=FOO ... agent/hermes/FOO-7` parsea; `TAL-7` bajo `FOO`
  se rechaza) son requisitos funcionales — los formaliza `sdd-spec`. Este design fija la **mecánica** (regex param + QuoteMeta).
- La precedencia env > project.env > .env (>.env principal) es comportamiento observable → puede mapear a un REQ de spec.

## 13. Strict TDD — boundaries que este design lockea (para tasks)

- **L1 `Parse` PURO** (table, sin I/O): blank/`#` skip; `KEY=a=b=c`→`a=b=c`; trim; sin-`=` ignorado; vacío→`{}`; CRLF.
- **L2 `LoadInto`** (setenv/getenv inyectados como maps): key ya-seteada NO se pisa; key unset SÍ; primer path gana;
  archivo faltante saltea sin error; error no-not-exist burbujea (el primero).
- **L3 regex param** (extender tests existentes): `branch_test.go` + columna `projectKey` (fila `FOO-7` válida bajo `FOO`,
  mantener `JIRA-7` rechazada bajo `TAL`); `naming_test.go` threadea `"TAL"` + sub-test `FOO`-válido.
- **L4 composition root:** mains finos; los tests `run()` existentes quedan verdes (back-compat).

> **Constraint heredada (apply):** código auto-explicativo, SIN comentarios salvo **una línea godoc en símbolos
> EXPORTADOS** (`Parse`, `LoadInto`, `ParseAgentBranch`, `ValidateJiraKey`, `NewWorktreeSpec`, `Config.Project`,
> `mainWorktreeRoot`, `repoRoot` de evidence). Hexagonal, cero-dep (stdlib only, cero `require`), go 1.26, strict TDD.
