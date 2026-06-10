# Proposal — judgment-day-gate (TAL-A · por crear)

> Change · Fase 3 · `module:devops` · owner HERMES · branch `agent/hermes/<JIRA-KEY>` (base `develop`)
> Store: hybrid (`openspec/` + engram). Status: proposed. Slug: `change:judgment-day-gate`.
> Thread A de la pareja HG5. El cableado `ov`-en-CI vive en el change hermano `collision-guard-ci` (fuera de scope).

## Why / Context

`CONSTITUTION.md §10/§12` manda que **desde Fase 3** el veredicto **APPROVED** de ARGOS / Judgment Day es
**gate duro pre-`sdd-archive`** (HG5). `openspec/config.yaml:34` ya declara `review.require_judgment_day: true`.
Pero ese gate hoy es **convención, no sistema** — verificado contra el repo y el Kit global:

- La skill global `judgment-day` es **human-invoked** y **devuelve texto** (`JUDGMENT: APPROVED ✅`); **no persiste
  un artefacto committeable**.
- `sdd-archive` (skill global) gatea **sólo** por CRITICAL en `verify-report` (línea 138); **no lee el veredicto**.
- El DAG de ATHENA es `verify→archive`, **sin nodo judgment-day**.
- **No hay job de CI** que exija el veredicto.

**Insight (de explore, fijo):** ARGOS son 2 jueces LLM **ciegos** → la review no se puede enforcer *dentro* de un
CLI Go. Lo enforceable es el **CONTRATO DE EVIDENCIA**: que CI exija un `judgment-report.md` **APPROVED committeado**,
exactamente como hoy el job `dod` exige `verify-report.md` presente. **El gate duro es CI (repo-owned, agente-
independiente), no la disciplina del agente.**

**Success looks like:** ningún PR a `develop` puede mergear (y por tanto ningún change puede archivarse) sin un
`judgment-report.md` con línea terminal `JUDGMENT: APPROVED` parseada por un binario Go testeable, gateado en CI real
— espejo exacto del backstop `verify-report` que ya existe.

## What changes

Espeja el patrón `verify-report → job dod`, aplicado al veredicto de Judgment Day. Tres entregables:

- **A.1 — Contrato de evidencia + validador `ch judgment`.** Nuevo artefacto `openspec/changes/{change}/judgment-report.md`
  (hermano de `verify-report.md`): header (Change, Round, Judges, Implementor, Date) + línea terminal parseable
  `JUDGMENT: APPROVED` | `JUDGMENT: ESCALATED`. Se **extiende el CLI `ch` existente** (`platform/ci-checks/`, hexagonal,
  cero-deps): dominio puro `ParseJudgmentReport` (estilo `ParseOwnershipTable`) + errores `ErrNoJudgmentReport` /
  `ErrJudgmentNotApproved`; `cmd/ch` gana `case "judgment"`. Exit **0 = APPROVED**, **1 = falta | malformado | ESCALATED**.
- **A.2 — Job de CI `judgment` + `rules.archive`.** Nuevo job en `.github/workflows/pr-checks.yml`, **espejo del job `dod`**
  (reusa `ch changed-modules` para derivar el change slug del diff) → corre `ch judgment --change <slug>`. Rojo si falta o
  no-APPROVED. Más bloque `rules.archive` en `openspec/config.yaml` (honrado por `sdd-archive` línea 145, **sin tocar el skill**).
- **A.3 — Governance.** Nuevo `team-context/judgment-day.md`: protocolo de cuándo corre ARGOS, qué artefacto deja, y la
  escalación `ESCALATED → ZEUS`. Doc independiente, mantenido por ATHENA.

## Scope

### In scope
- Artefacto `judgment-report.md` (formato + línea terminal parseable + header).
- Extensión del CLI `ch`: dominio `ParseJudgmentReport` + errores, subcomando `case "judgment"` (exit 0/1).
- Job `judgment` en `pr-checks.yml` (espejo de `dod`, deriva slug del diff vía `ch changed-modules`).
- Bloque `rules.archive` en `config.yaml` (honrado por `sdd-archive` sin editar el skill).
- `team-context/judgment-day.md` (protocolo + escalación).
- Fila en `team-context/ownership.md` si A.3 introduce un path nuevo bajo otro dueño (gated por ATHENA).

### Out of scope (deferrals explícitos)
- **Thread B — cableado `ov`-en-CI:** vive en el change hermano `collision-guard-ci`. **No** es parte de este change.
- **Kit global (`~/.claude`):** **no** se editan las skills `judgment-day` / `sdd-archive` ni las instrucciones de ATHENA.
- **Auto-trigger del nodo `verify→judgment-day→archive`** en el DAG de ATHENA: follow-up de Kit/Fase 5.
- **`ch` ejecutando la review:** ARGOS (2 jueces LLM) corre fuera; `ch` **sólo valida el artefacto**, no juzga.
- **Generación del `judgment-report.md`:** CI **asserta presencia + APPROVED**, no lo genera (igual que `dod` con verify-report).

## Approach

Contrato de evidencia + CI backstop, calcando el mecanismo ya probado de `verify-report`/`dod`:
1. ARGOS deja un `judgment-report.md` committeado en la rama del implementor (ver O-3).
2. `ch judgment` parsea ese artefacto (dominio puro, table-driven) y devuelve exit 0/1.
3. El job `judgment` de CI lo corre fail-closed → **gate duro repo-owned**, independiente del agente.
4. `rules.archive` declara el requisito para `sdd-archive` **sin tocar el Kit global** (lectura de config, no de skill).

## Capabilities

### New Capabilities
- `judgment-gate`: contrato de evidencia del veredicto de Judgment Day (artefacto `judgment-report.md` + validador
  `ch judgment` + job de CI `judgment` + `rules.archive`) como gate duro pre-archive (HG5).

### Modified Capabilities
- None. (Reusa el patrón de `ci-hardening` por extensión del CLI `ch`; no cambia requisitos de capabilities existentes.)

## Resolved decisions (por ZEUS — no re-litigar)

| # | Decisión | Resolución | Por qué |
|---|----------|------------|---------|
| 1 | Alcance del change | **Sólo Thread A** (gate de Judgment Day) | El cableado `ov`-en-CI es el change hermano `collision-guard-ci` |
| 2 | Dónde enforcear | **CI backstop** (job espejo de `dod`), no la disciplina del agente | ARGOS son jueces LLM ciegos → enforceable = contrato de evidencia, repo-owned |
| 3 | Dónde vive el validador | **Extender el CLI `ch`** (`platform/ci-checks/`), no un binario nuevo | `ch` ya es el motor de gates de CI (`labels`/`dod`); `judgment` es otro gate del mismo módulo (HERMES) |
| 4 | Touch del Kit global | **Repo-only**; `rules.archive` en config, no editar skills/ATHENA | Honra config sin acoplar el repo al Kit; auto-trigger del nodo = Fase 5 |

## Open questions (los resuelve spec/design — no bloquean el proposal)

- **O-1:** ¿hard-fail si `judges ≠ 2` o `implementor ∈ judges`? **Reco: surface-only en v1** (warn, no rompe). → spec/design.
- **O-2:** ¿1 PR = 1 change slug? (deriva del diff). → spec.
- **O-3:** ¿el `judgment-report.md` se commitea en la **rama del implementor**, dado que ARGOS no es dueño de módulo?
  **Reco: sí** (el implementor lo adjunta a su rama). → design.
- **O-4:** label del issue del doc A.3 (`module:*` del governance doc). → tasks/dispatch.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `platform/ci-checks/domain/cichecks/` | New | `ParseJudgmentReport` + errores `ErrNoJudgmentReport` / `ErrJudgmentNotApproved` |
| `platform/ci-checks/cmd/ch/` | Modified | `case "judgment"` (exit 0=APPROVED / 1=falta\|malformado\|ESCALATED) |
| `.github/workflows/pr-checks.yml` | Modified | Nuevo job `judgment` (espejo de `dod`, deriva slug vía `ch changed-modules`) |
| `openspec/config.yaml` | Modified | Bloque `rules.archive` (honrado por `sdd-archive` línea 145) |
| `team-context/judgment-day.md` | New | Protocolo de governance + escalación `ESCALATED → ZEUS` |
| `team-context/ownership.md` | Modified | Fila para `team-context/judgment-day.md` si aplica (gated por ATHENA) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dog-food: el propio PR de A.2 introduce y es gateado por el job `judgment` nuevo | Med | El PR que introduce el job adjunta su propio `judgment-report.md` APPROVED; confirmar antes de abrir (estilo HG4) |
| `judgment-report.md` malformado pasa como APPROVED | Med | Parser table-driven en dominio puro; fail-closed ante malformación; tests rojo→verde→refactor |
| Drift entre formato del artefacto y lo que ARGOS escribe | Med | `team-context/judgment-day.md` (A.3) fija el formato como fuente de verdad del protocolo |
| `rules.archive` no honrado por `sdd-archive` (skill no lo lee) | Low | Verificado: `sdd-archive` lee config en línea 145; sin tocar el skill |

## Rollback Plan

Cada PR es independiente y revertible solo: revertir el commit del job `judgment` desactiva el gate sin afectar
`labels`/`dod`; revertir A.1 quita el subcomando sin romper `ch labels`/`ch dod`; revertir A.3 elimina sólo el doc.
`rules.archive` es aditivo en config → quitar el bloque restaura el comportamiento previo (gate sólo por convención).

## Dependencies

- **Reusa** `platform/ci-checks/` (`ch`) y el patrón `verify-report → dod` de `ci-hardening` (TAL-5, archivado) por
  **extensión**, no por nuevo módulo.
- **Hermano (paralelo, no solapado):** `collision-guard-ci` (Thread B). Dueño de módulo distinto por slice; mergeables en paralelo.
- **Repo-only:** no depende de cambios en el Kit global.

## Forecast / delivery

**PRs encadenados ≤ ~400 líneas**, work-unit commits. Delivery strategy: **auto-chain**.

- **A.1** = `ch judgment` (dominio `ParseJudgmentReport` + errores + `cmd/ch case`), **sólo `platform/ci-checks/**`**, cero red.
- **A.2** = job `judgment` en `pr-checks.yml` + `rules.archive` en `config.yaml`. **Stacked sobre A.1** (necesita el subcomando).
- **A.3** = `team-context/judgment-day.md` (governance). **Independiente** (puede ir en paralelo a A.1/A.2).

`Chained PRs recommended: Yes` · `400-line budget risk: Low–Medium` (A.1 es la slice más pesada; A.2/A.3 son chicas).

## Success Criteria

- [ ] `judgment-report.md` con formato fijo + línea terminal `JUDGMENT: APPROVED|ESCALATED` parseable.
- [ ] `ch judgment --change <slug>` devuelve exit 0 sólo ante APPROVED; 1 ante falta / malformado / ESCALATED.
- [ ] Job `judgment` corriendo en CI real, fail-closed, derivando el slug del diff.
- [ ] `rules.archive` en `config.yaml` honrado por `sdd-archive` sin editar el skill.
- [ ] `team-context/judgment-day.md` documenta protocolo + escalación.

> **Constraint heredada (design/apply):** código auto-explicativo, SIN comentarios salvo una línea godoc en
> símbolos **exportados** (convención de code-comments del proyecto). Strict TDD: `go test ./...`, rojo→verde→refactor.
