# Proposal — ci-hardening (TAL-5)

> Change #4 · Fase 2 · `module:devops` · owner HERMES · branch `agent/hermes/TAL-5`
> Store: hybrid (`openspec/` + engram). Status: proposed.

## Why / Context

`CONSTITUTION.md §4` define un **invariante de identidad por label**: cada PR a `develop` está ligado a
un issue Jira que lleva **exactamente un** `agent:*` y **un** `module:*`, y el par debe cumplir
`ownership[module] == agent` (de `team-context/ownership.md`). Hoy ese invariante **no está enforced en CI**.
`ci/pr-checks.yml` es un **esqueleto** de GitHub Actions con 3 jobs, y dos de ellos son `echo "TODO"`:

1. `branch-name` — **HECHO** (regex `^agent/[a-z]+/TAL-[0-9]+$`, §3). Bash que funciona.
2. `labels` — **TODO.** Enforcear el invariante §4 contra el issue Jira ligado al PR.
3. `dod` — **TODO.** §6: tests verdes por módulo cambiado + verify-report presente.

El proposal de `merge-order-automation` (TAL-3, archivado) **difiere explícitamente a este change**
(`proposal.md` líneas 64 y 77): *"ci-hardening (#4): readiness por CI-green, `--require-ci-green`,
edición de `ci/pr-checks.yml`"*, y *"CI-green se difiere a #4"*. `mo` dejó su readiness git-only por esa
razón. Este change cierra esa deuda.

**Success looks like:** los 3 gates corriendo en CI real bajo `.github/workflows/`, con el invariante §4
enforced por un **binario Go testeable** (`ch`) bajo strict TDD, no por bash. El gate `labels` resuelve la
key desde la rama (§3), corrobora la identidad de la rama contra el label `agent:*` (§5, sub-regla (d)) y
falla cerrado ante violación, ausencia de labels o issue inexistente.

## What changes

Un nuevo módulo Go `platform/ci-checks/` con el CLI `ch`, que **espeja** la arquitectura hexagonal de los
módulos existentes (`wt`, `mo`, `ov`): `go 1.26`, **cero dependencias** (stdlib only), dominio / puertos /
servicio testeados sin red, adapters aislados (`testing.Short()`-gated). El binario es **solo el motor del
invariante de labels** — correr tests es trabajo de CI, no de `ch`.

CLI surface:

- `ch labels --branch <agent/<figura>/TAL-N> [--ownership-file <path>] [--site-url <url>] [--json]` —
  **el gate.** Parsea la rama → figura + key, fetch de labels del issue por key, lee `ownership.md`, evalúa
  el invariante §4 (a)(b)(c)(d). Exit 0 OK · 1 violación / `ErrNoJiraKey` / issue-404 / ownership malformado.
- `ch ownership [--ownership-file <path>] [--json]` — **canario offline**: valida que `ownership.md` esté
  bien-formado, sin red. Pre-flight barato.

Migración del YAML: crear `.github/workflows/pr-checks.yml` (dir net-new, no existe `.github/` en el repo),
**borrar** `ci/pr-checks.yml` + el dir `ci/` vacío. 3 jobs `on: pull_request: branches:[develop]`:

- **`branch-name`**: portar el bash existente **verbatim**. Gate barato primero.
- **`labels`** (`needs: branch-name`): checkout + setup-go 1.26 + `go build -o /tmp/ch ./cmd/ch` +
  `/tmp/ch labels --branch "$GITHUB_HEAD_REF" --json` desde repo-root.
- **`dod`** (`needs: branch-name`): `fetch-depth:0` + detectar `platform/<m>/` cambiados vs base +
  `go test -short ./...` por módulo con `go.mod` + placeholder vitest + assert verify-report presente.

**Cleanup in-scope (todo HERMES-owned, sin cross-module write):**
- `.gitignore`: append `/platform/ci-checks/ch` (binario compilado, patrón de los 4 siblings).
- `team-context/ownership.md`: fila en la tabla archivo→módulo para `platform/ci-checks/**` (dueño HERMES).
- Borrar `ci/pr-checks.yml` + dir `ci/`; crear `.github/workflows/pr-checks.yml`.
- (Opcional) `team-context/ci.md`: doc de los 3 jobs + 3 secrets + contrato de detección del `dod`.

## Scope

### In scope
- Módulo Go `platform/ci-checks/` con CLI `ch` (subcomandos `labels` / `ownership`), hexagonal, cero-deps.
- Dominio puro: `ParseLabels` + `LabelSet.Validate` (exactly-once), `ValidateOwnership`, `CheckLabelInvariant`
  ((a)(b)(c)(d)), `ParseAgentBranch`, `ParseOwnershipTable` (lowercasing de figura y módulo).
- Ports `IssueLabelReader` + `OwnershipReader`; mocks hand-written con call-recording.
- Service `Checker.Check(ctx, branch)` + `DefaultTALConfig()`.
- Adapter `jirarest`: `GET /rest/api/3/issue/{key}?fields=labels`, `auth.go` verbatim, 404 typed.
- Adapter `ownershipfile`: leer-de-path, default `repoRoot/team-context/ownership.md` (NO `go:embed`).
- Migración `ci/pr-checks.yml` → `.github/workflows/pr-checks.yml` con los 3 jobs implementados.
- Cleanup listado arriba.

### Out of scope (deferrals explícitos)
- **vitest real:** no hay frontend todavía → placeholder documentado no-bloqueante en el job `dod`.
- **Generación de verify-report:** el `dod` assert **presencia**, no lo genera.
- **`ch` absorbiendo `dod`:** correr tests vive en el YAML/runner, NO en `ch` (evita el "test-corriendo-tests";
  `ch` es SOLO el motor de labels).
- **`--require-ci-green` en `mo`:** wiring del readiness CI-green de `mo` contra estos gates es concern aparte.
- **`actionlint` como gate duro:** queda como **advisory** (el YAML no es Go-testeable; ver Risks).
- **Retry / rate-limit wrapper sobre Jira:** 1 GET por run con timeout; retry es futuro.
- **Grace para issue mid-creación** (sin labels todavía): se trata como violación real (§4: ATHENA labela al crear).

## Resolved decisions

| # | Decisión | Resolución | Por qué |
|---|----------|------------|---------|
| 1 | Motor del invariante de labels | **CLI Go nuevo `ch`** (`platform/ci-checks/`, hexagonal, cero-deps), no bash, no extender `ov` | Bash no es testeable bajo strict TDD. `ov` es de THEMIS (`module:qa`) → extenderlo violaría §2 (escritura única) |
| 2 | Alcance del gate `dod` | Corre `go test -short ./...` por módulo cambiado en el **YAML/runner**, NO en `ch` | Correr tests es trabajo de CI; mantener `ch` como motor de labels evita el "test-corriendo-tests" |
| 3 | Endpoint Jira | `GET /rest/api/3/issue/{key}?fields=labels`, NO el `POST .../search` de `ov` | "Labels de un issue por key" → GET es el fit exacto: decode más simple (sin `issues[]`, sin ADF), 404 distinguible |
| 4 | Fuente de `ownership.md` | **Leer-de-path** (default `repoRoot/team-context/ownership.md`), NO `go:embed` | CI corre en repo checkouteado → el archivo vivo está presente; embeber bakearía un snapshot stale de la fuente-de-verdad (§2) |
| 5 | Invariante branch-figura == label-agent | **Incluir como sub-regla (d) fuerte** del invariante | Cruza §3 (rama) con §4 (label) y realiza §5 ("la rama como corroboración"); puro, trivial de testear, atrapa el PR mal-labelado vs su rama |
| 6 | Reúso de dominio | **Reimplementar verbatim** `LabelSet.Validate` (de `evidence/issue.go:11-64`) y `ValidateOwnership`+`ErrOwnershipViolation` (de `evidence/ownership.go:35-63`) | No se puede importar — `go.mod` separado, cero-dep. (a)(b)(c) ya están diseñados y testeados; (c) es case-sensitive (`ownership.go:54`) → el parser **lowercasea** figura y módulo |
| 7 | Migración del dir `ci/` | **Borrar** `ci/pr-checks.yml` + dir `ci/`, crear `.github/workflows/pr-checks.yml` desde cero | `ci/` era staging documentado sin pointer-stub; `.github/` no existe en el repo → movimiento limpio a la ubicación canónica de Actions |
| 8 | Secrets de GitHub | `JIRA_SITE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN` (= env vars que lee `cmd/ch/main.go`) | Mismas creds Basic-auth que `ov`; leídas en `cmd` (no en el adapter — los adapters toman args explícitos) |

## Impact / dependencies

- **Reúsa (reimplementa, no importa)** patrones de change #1 `worktree-orchestration` (TAL-2, archivado) y
  #3 `overlap-protocol` (TAL-4, archivado): la convención de rama §3 (`figuraFromBranch` + regex TAL),
  el esqueleto hexagonal canónico de `mo`/`ov`, y el adapter `jirarest` (auth + HTTP a Jira REST v3). El acople
  es por **patrón**, no por `go.mod` (cero-dep obliga a reimplementar).
- **Paralelizable:** nada concurrente ahora mismo.
- **Desbloquea:** el demo #5 (CI-green readiness) — los 3 gates en CI real son prerequisito de ese demo.
- **Ownership:** todo el cambio cae en `module:devops` (HERMES). El cleanup toca archivos compartidos ya bajo
  HERMES (`.gitignore`, `team-context/ownership.md`, `ci/`→`.github/`). **Sin cross-module write.**

## Risks

- **Chicken-egg / dog-food (el PR de ci-hardening pasa sus propios gates):** PR1 lo gatea el esqueleto
  **viejo** (sin enforcement); PR2 introduce y es gateado por los gates **nuevos**. Mitigado: TAL-5 ya está
  creado con `agent:hermes` + `module:devops` (`ownership[devops]==hermes` ✓, branch hermes == label hermes ✓)
  → PR2 pasa su propio `labels`. Confirmar labels antes de abrir PR2 (estilo HG4).
- **El YAML no es Go-testeable:** la superficie no-testeada se reduce por diseño a "checkout, build, invocar
  binario, pasar env" (el punto de D1). Mitigado con `actionlint` **advisory** + dog-food del propio PR2.
- **Flakiness / rate-limit de Jira:** 1 GET por run, `Timeout:30s`, **fail-closed** ante 5xx. Retry = futuro.
- **Drift de `ownership.md` (PR que gatea vs PR gateado):** mitigado leyendo el archivo **vivo** (no `go:embed`),
  que es la fuente-de-verdad §2.
- **Falso-negativo del `dod` (0 módulos cambiados → exit 0):** `fetch-depth:0` mitiga el merge-base; un PR
  docs-only sin Go es aceptable. Assert "≥1 módulo testeable o docs-only" = futuro.

## Forecast

~750–950 LOC · **2 PRs encadenados** — PR1: `go.mod` + `domain/cichecks/*` (+tests) + `port/*` + `mock/*` +
`service/*` (config, checker, checker_test mockeado), **cero red**; PR2 (stacked en PR1): adapters (`jirarest`,
`ownershipfile`) + `cmd/ch/*` + migración del YAML (`.github/workflows/pr-checks.yml`) + borrar `ci/` + cleanup
(`.gitignore`, fila en `ownership.md`). Cada PR verde solo (`go test -short ./...`).
**`Chained PRs recommended: Yes` · `400-line budget risk: High`.**

> **Constraint heredada (design/apply):** código auto-explicativo, SIN comentarios salvo una línea godoc en
> símbolos **exportados** (convención de code-comments del proyecto).
