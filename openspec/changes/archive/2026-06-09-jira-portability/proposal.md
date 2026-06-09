# Proposal — jira-portability (TAL-6)

> Fase 2 · `module:devops` (HERMES lidera) + `module:qa` (THEMIS, slice `ov`) · branch `agent/hermes/TAL-6`
> Store: hybrid (`openspec/` + engram). Status: proposed.

## Why / Context

La plataforma Talos se va a **extraer como Kit reutilizable** y a usar en **muchos proyectos con
Jiras distintos**. Hoy hay **DOS acoplamientos duros a "Talos específicamente"** que rompen ese
objetivo de raíz — clonar Talos a un proyecto `FOO` **no corre el flujo sin editar código Go**:

1. **Secrets sin carga local-first.** Las 3 CLIs que tocan Jira (`evidence`, `ov`, `ch`) leen
   `os.Getenv("JIRA_EMAIL/API_TOKEN/SITE_URL")` pero **ninguna carga un `.env`** — el dev tiene que
   exportar las creds a mano en cada shell. La única historia documentada era "cargá GitHub secrets"
   (solo el backend de CI). **No existe local-first.**
2. **Identidad del proyecto hardcodeada.** `DefaultTALConfig()` clava `"TAL"`/`"10099"` en
   evidence/ov/ch; y peor, las regex de dominio **exigen `TAL-`**: `branch.go:5`
   (`^agent/([a-z]+)/(TAL-[0-9]+)$`) y `naming.go:22` (`^TAL-[1-9][0-9]*$`). Clonar a `FOO` **no
   compila el flujo** sin tocar Go.

**Success looks like:** un dev clona Talos a otro proyecto, llena `.talos/project.env` (3 líneas:
site/project_key/project_id) + `.env` (2 secrets: email/token), y **las CLIs corren contra cualquier
Jira sin tocar una línea de Go.** `JIRA_PROJECT_KEY=FOO ch labels --branch agent/hermes/FOO-7`
parsea; `agent/hermes/TAL-7` bajo `FOO` se rechaza (correcto).

**Encuadre honesto (tensión con §13).** El `CONSTITUTION.md §13` lista la **extracción del Kit** como
**no-goal de Fase 0–2**. Este change **NO extrae el Kit** — lo que hace es **Kit-PREP**: matar los dos
acoplamientos para que las CLIs **ya sean portables hoy**, dentro de Talos. La extracción real (mover
los módulos a un repo/módulo Go propio) sigue siendo Fase 5. Lo declaramos explícito para no
disfrazar de "extracción" lo que es "habilitación".

## What changes

**Config en dos niveles**, con un **único parser zero-dep** compartido entre ambos:

| Nivel | Archivo | Git | Contenido |
|---|---|---|---|
| **No-secreto (identidad)** | `.talos/project.env` (NUEVO, committeado) | tracked | `JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID` |
| **Secreto** | `.env` (ya gitignoreado) | ignored | `JIRA_EMAIL`, `JIRA_API_TOKEN` |

- **Loader `internal/envfile/` (zero-dep, ~30 LOC)** reimplementado por módulo (NO cross-import —
  mismo patrón que `ValidateOwnership`): `Parse` puro (`[]byte → map`: skip blank/`#`, split en el
  **primer** `=`, trim) + `LoadInto(setenv, getenv, paths...)` con **precedencia por orden de args +
  if-unset**. **Env real (CI/shell) gana siempre** → CI sin cambios. **NO-invasivo:** los `os.Getenv`
  existentes quedan IGUAL; el loader solo **puebla** el env antes.
- **Override de 3 escalares en el composition root** tras `DefaultTALConfig()` (**NO renombrar la
  func** — R1, 9 call sites): si `JIRA_PROJECT_KEY`/`JIRA_PROJECT_ID` están seteadas, pisan
  `cfg.Project`/`cfg.ProjectID`. `JIRA_SITE_URL` ya entra por el flag default existente.
- **Regex de dominio parametrizada** (matar `TAL-`). El dominio **sigue PURO**: recibe `projectKey`
  por parámetro y arma la regex con `regexp.QuoteMeta`. `ParseAgentBranch(branch, projectKey)`,
  `ValidateJiraKey(key, projectKey)`, `NewWorktreeSpec(..., projectKey)`. `wt` no toca creds pero SÍ
  entra por la regex: se agrega `Project` a `worktree/service.Config`, seedeado de `JIRA_PROJECT_KEY`,
  threadeado a `NewWorktreeSpec`.
- **Enmienda de spec REQ-AUTH** (`jira-loop/functional.md` + `config.go:16`): secrets EMAIL/TOKEN SOLO
  en `.env` gitignoreado; identidad no-secreta en config committeado.

Toca **tres bounded contexts**: evidence (`jira-evidence-loop`, HERMES), ch/wt (`ci-checks` +
`worktree-orchestrator`, HERMES), `ov` (`overlap-guard`, **THEMIS** — slice cross-module §2).

## Scope

### In scope
- Paquete `internal/envfile/` (`Parse` + `LoadInto`, setenv/getenv inyectables) reimplementado en los
  **3 módulos Jira** (evidence canónico, ch + ov copia verbatim).
- Archivo `.talos/project.env` (NUEVO, committeado) con `JIRA_SITE_URL`/`JIRA_PROJECT_KEY`/`JIRA_PROJECT_ID`.
- Override de `JIRA_PROJECT_KEY`/`JIRA_PROJECT_ID` en el composition root de evidence/ch/ov.
- `ParseAgentBranch` + `ValidateJiraKey` + `NewWorktreeSpec` parametrizados por `projectKey` (con
  `regexp.QuoteMeta`); `worktree/service.Config.Project` agregado y threadeado.
- `evidence` gana un `repoRoot()` (~7 LOC, copia de `ch:58-65`) para resolver los paths del loader.
- Worktree: contrato **(a) env heredado** + **slice (b)** fallback al `.env` del checkout principal
  vía `git rev-parse --git-common-dir` (~6 LOC, best-effort skip si no existe).
- Enmienda de la convención **REQ-AUTH** (jira-loop spec).
- `.env.example` + comentarios "mirror — mantener en sync" en `.talos/project.env` ↔ `openspec/config.yaml:jira`.

### Out of scope (deferrals explícitos)
- **`IssueTypeName:"Tarea"` + `OrderedStates` en español** (`evidence/config.go:107-111`): es un **mapa**,
  no un escalar → estructuralmente más duro. Follow-up (extensión de `project.env` con `JIRA_ISSUE_TYPE`
  + archivo de transiciones, o leerlas de Jira en runtime).
- **Roster de agentes hardcodeado** (`naming.go:11-20`, `assignMap` + `portBase 8100`): es el **modelo
  de agentes** (project-defining), no acople de Jira. **Non-goal.**
- **Extracción real del Kit** (§13): mover módulos a repo/módulo propio = **Fase 5**. Este change es
  Kit-PREP, no extracción.
- **Lint de sync `project.env` ↔ `config.yaml:jira`:** la duplicación de 3 escalares es deliberada;
  el enforcement automático es follow-up (por ahora, comentario en ambos).
- **Renombrar `DefaultTALConfig`:** 9 call sites → se mantiene el nombre, se override en el root (R1).

## Resolved decisions

| # | Decisión | Resolución | Por qué |
|---|----------|------------|---------|
| D1 | Alcance del desacople | **Desacople TOTAL** (secrets locales + matar hardcode TAL/site/project_key, **incluida la regex de dominio**), NO solo secrets | Decidido por ZEUS: el objetivo es portabilidad real a cualquier Jira; con la regex hardcodeada `FOO-N` ni siquiera parsea |
| D2 | Formato del config no-secreto | **`.talos/project.env` en `KEY=VALUE`**, leído por el MISMO `envfile.Parse` | Cero-dep, cero parser nuevo. Parsear YAML de `openspec/config.yaml` es frágil; un 2do formato JSON duplica superficie |
| D3 | Cuándo corre el loader | **Hoist `LoadInto` al tope de `run()`**, antes del `switch` de subcomandos | **R8 — el bug más probable:** el loader DEBE poblar el env ANTES de que `flag` evalúe el default `os.Getenv("JIRA_SITE_URL")` (`ch:94`, `ov:243`) |
| D4 | Fail-fast en el loader | **NO meter fail-fast genérico** — el loader solo PUEBLA; cada CLI mantiene SU validación | `ch` es **creds-opcional** (`ch labels` sin `--json` no toca Jira); `ov scan`/`metric` no necesitan creds, `ov check` sí; `evidence` ya falla fuerte |
| D5 | Renombrar `DefaultTALConfig` | **NO renombrar** → override de 3 escalares en el composition root | 9 call sites en ch/ov/evidence/wt/mo; renombrar es un blast-radius gratis (R1) |
| D6 | Regex param vs dominio puro | **`projectKey` por parámetro + `regexp.QuoteMeta`**, dominio sigue PURO | No inyectar env/config en el dominio; la composición arma el `projectKey` y lo pasa. `QuoteMeta` evita inyección de regex |
| D7 | Creds en worktrees | **(a) env heredado** (ATHENA exporta antes de spawnear) **+ (b) fallback** al `.env` principal vía `--git-common-dir` | `.env` (gitignoreado) NO existe en un worktree nuevo; `.talos/project.env` (tracked) sí. (a) es el intent ya documentado; (b) es ~6 LOC best-effort |
| D8 | Reúso del loader | **`envfile` canónico en `jira-evidence-loop`**, copia **verbatim** a ch/ov | Cero-dep prohíbe cross-import (go.mod separados); mismo patrón ya usado con `ValidateOwnership`. THEMIS revisa su copia contra la de HERMES |
| D9 | Nombre del archivo de secrets | **NO nombrar `.env.project`** — `.gitignore:3` (`.env.*`) lo atraparía | `.talos/project.env` NO matchea `.env.*` (verificado) → seguro de committear |

## Impact / dependencies

- **Consume:** nada nuevo. Reusa el `repoRoot()` existente, el patrón hexagonal canónico de los módulos
  y la convención de rama §3. El acople es por **patrón**, no por `go.mod` (cero-dep obliga a reimplementar).
- **Cross-module (§2):** **HERMES lidera**; el slice de `ov` es de **THEMIS** (`module:qa`) → va en su
  **propio PR** con **HERMES sign-off** registrado en `tasks.md` (regla reviewer-distinto, `config.yaml:34`).
  El `envfile.go` se escribe **canónico una vez** en jira-evidence-loop; la copia de `ov` es un drop
  verbatim que THEMIS revisa contra la referencia de HERMES.
- **Back-compat:** **additive, cero flag-day.** Sin `.talos/project.env` y sin env override →
  `DefaultTALConfig` devuelve `TAL`/`10099` igual. CI sin cambios (env real de secrets gana sobre `.env`).
- **Desbloquea:** la **extracción futura del Kit** (Fase 5) y el **onboarding de proyectos nuevos** (clonar
  → 5 líneas de config → correr).
- **Ownership (detalle):** HERMES toca `jira-evidence-loop/**`, `ci-checks/**`, `worktree-orchestrator/**`,
  `.talos/project.env`, `.env.example`, `openspec/*`. THEMIS toca `overlap-guard/**` (loader copy + override).

## Risks

- **R8 — hoist del loader (el bug más probable):** si `LoadInto` corre DESPUÉS de que `flag` evalúa el
  default `os.Getenv("JIRA_SITE_URL")`, el `.env` se ignora silenciosamente. **Mitigación:** hoist al tope
  de `run()`, un solo lugar por main; marcado explícito en `tasks.md`.
- **R1 — rename break:** renombrar `DefaultTALConfig` rompería 9 call sites. **Mitigación:** NO renombrar;
  override de 3 escalares en el root.
- **Back-compat TAL:** cambio **additive**, cero flag-day — sin config nuevo, el comportamiento actual
  (`TAL`/`10099`) se mantiene; el env real de CI gana siempre.
- **Worktree sin `.env`:** el `.env` gitignoreado no existe en un worktree nuevo. **Mitigación:** env
  heredado (a) + fallback (b) al `.env` principal vía `--git-common-dir` (normalizar relativo/absoluto;
  bare repo → path no existe → `LoadInto` saltea silencioso).
- **Duplicación `project.env` ↔ `config.yaml:jira`:** 3 escalares repetidos pueden derivar. **Mitigación:**
  comentario "mirror — mantener en sync" en ambos; lint automático = follow-up.
- **Cross-module THEMIS (§2):** el slice de `ov` cruza ownership. **Mitigación:** PR propio de THEMIS +
  HERMES sign-off + `envfile` canónico de HERMES como referencia de revisión.

## Forecast

~**630 LOC** · **5 PRs encadenados**:

| PR | Owner | Scope | ~LOC |
|---|---|---|---|
| PR1 | HERMES | `envfile` canónico + tests (en jira-evidence-loop) — **keystone** | ~110 |
| PR2 | HERMES | wire loader + override en `evidence` (+`repoRoot`); enmienda REQ-AUTH; crear `.talos/project.env`; comments `.env.example`/`config.yaml` | ~120 |
| PR3 | HERMES | `ch`: copy loader + `branch.go` param + override + tests | ~150 |
| PR4 | HERMES | `wt`: `naming.go` param + `Config.Project` + threading + tests (sin loader) | ~140 |
| PR5 | THEMIS | `ov`: copy loader + override (**HERMES sign-off**) | ~110 |

**`Chained PRs recommended: Yes`** · **400-line budget:** cada PR **<400** (OK, sin `size:exception`).
**PR1 es keystone** (fija el patrón `envfile`); **PR3/PR4/PR5 paralelizables** una vez mergeado PR1.

> **Constraint heredada (design/apply):** código auto-explicativo, SIN comentarios salvo una línea
> godoc en símbolos **exportados** (convención de code-comments del proyecto). Hexagonal, cero-dep
> (stdlib only, cero `require`), go 1.26, strict TDD.
