# Proposal — merge-order-automation (TAL-3)

> Change #2 · Fase 2 · `module:devops` · owner HERMES · branch `agent/hermes/TAL-3`
> Store: hybrid (`openspec/` + engram). Status: proposed.

## Why / Context

`CONSTITUTION.md §11` y `team-context/merge-order.md` definen una **disciplina de merge** —
de a un worktree a la vez contra `develop`, rebase-antes-del-PR, nunca dos merges concurrentes sin
rebase previo, orden por dependencias → superficie de conflicto → FIFO— pero hoy esa disciplina es
**totalmente manual**. ATHENA la ejecuta a mano: rebasear contra un `develop` que avanza, re-chequear,
prevenir colisiones, secuenciar. Ese toil no escala y no es determinista.

Fase 2 change #1 (`worktree-orchestration`, TAL-2, **archivado**) ya entregó el aislamiento físico: el
CLI `wt` con `wt list --json`. Lo que falta es la capa que **consume** ese inventario y computa/ejecuta
el orden de merge seguro. Este change es el #2 del roadmap: depende solo de #1, es paralelizable con #3
(`overlap-protocol`), y es **prerequisito del demo de 2 devs (#5)**, donde se ejercita el gate HG6
(tasa de conflictos < ~15%). Sin esta automatización, ese demo no se puede correr de forma repetible.

**Success looks like:** un CLI `mo` que, dado el inventario de `wt`, produce un plan de merge
determinista y reproducible, detecta colisiones sin tocar el working dir, e integra ramas listas una a
una contra `develop` re-chequeando contra el tip que avanza — **deteniéndose siempre en el borde del PR**
para que el merge final a `develop` lo apruebe un humano (HG3).

## What changes

Un nuevo módulo Go `platform/merge-order-orchestrator/` con el CLI `mo`, que **espeja** la arquitectura
hexagonal de los módulos existentes (`wt`, `evidence`): go 1.26, **cero dependencias** (stdlib only),
dominio / puertos / servicio testeados sin git, adapters aislados. Tres subcomandos a alto nivel:

- `mo plan` — **read-only siempre.** Consume `wt list --json`, filtra las ramas listas, computa el
  orden de merge seguro (criterios §11) y lo imprime. No muta nada.
- `mo check` — detección de colisiones **pura-lectura** entre ramas / contra `develop`, sin tocar el
  working dir.
- `mo execute --yes` — rebasea y secuencia las ramas listas **una a la vez**, re-chequeando contra el
  tip de `develop` ya avanzado, y **HALTA en el borde del PR**: imprime el comando `gh` para que ZEUS
  abra/mergee el PR. **`mo` nunca mergea a `develop`.**

`mo` **consume** `wt list --json` vía shell-out + un DTO local (no acopla go.mod entre módulos; el JSON
es el seam estable). La detección de colisiones usa `git merge-tree --write-tree --name-only`
(pura-lectura, git ≥ 2.50.1).

**Cleanup in-scope (mínimo, dentro del módulo o compartidos de HERMES):**
- Corregir `team-context/merge-order.md`: hoy dice `main`; `§11` es la fuente de verdad → `develop`.
- Agregar fila en `team-context/ownership.md` para `platform/merge-order-orchestrator/**` (dueño HERMES).
- Agregar `/platform/merge-order-orchestrator/mo` a `.gitignore` (binario compilado).

## Scope

### In scope
- Módulo Go `platform/merge-order-orchestrator/` con CLI `mo` (subcomandos `plan` / `check` / `execute`).
- Consumo de `wt list --json` vía shell-out + DTO local.
- Cómputo determinista del orden de merge (criterios §11 #2 conflict-surface + #3 FIFO siempre;
  dependencias vía `--depends` / `--depends-file` opcional).
- Readiness MVP **git-only**: `active` en `wt list` **y** commits-ahead(`develop`) > 0.
- Detección de colisiones con `git merge-tree --write-tree --name-only`.
- Posture de ejecución **HYBRID**: `plan`/`check` read-only; `execute` rebasea+secuencia una a la vez
  pero **HALTA en el borde del PR**.
- Cleanup listado arriba.

### Out of scope (deferrals explícitos)
- **Enforcement de overlap-protocol (#3):** serialización por mismo-archivo. `mo` usa `ChangedFiles`
  **solo como heurística de ranking**, no la enforcea.
- **ci-hardening (#4):** readiness por CI-green, `--require-ci-green`, edición de `ci/pr-checks.yml`.
  **Este change NO toca CI.**
- **Aprobación humana HG3:** nunca se codea; está garantizada estructuralmente por el HALT en el PR.
- **Auto-discovery de dependencias por link de Jira.**
- **Un executor real `--auto-merge`** (que mergee a `develop`).

## Resolved decisions

| # | Decisión | Resolución | Por qué |
|---|----------|------------|---------|
| 1 | Cómo leer el inventario de `wt` | Shell-out a `wt list --json` + DTO local | Honra el límite de módulo; sin acople de go.mod; el JSON es el seam estable |
| 2 | Posture de ejecución | **HYBRID (decidido por ZEUS):** `plan` read-only; `execute --yes` secuencia una a una pero HALTA en el PR | Honra HG3 (§10, gate humano duro) de forma **estructural** — `mo` nunca mergea a `develop` |
| 3 | Detección de colisiones | `git merge-tree --write-tree --name-only` | Pura-lectura, no muta el working dir; git 2.50.1 lo soporta |
| 4 | Definición de readiness | MVP git-only: `active` + commits-ahead(`develop`) > 0 | CI-green se difiere a #4; approval es HG3 (humano) |
| 5 | Ordenamiento | §11: deps → conflict-surface → FIFO. MVP: #2+#3 siempre; deps vía `--depends`/`--depends-file` opcional | Auto-discovery de deps por Jira se difiere |

## Impact / dependencies

- **Consume** change #1 `worktree-orchestration` (TAL-2, archivado) vía `wt list --json` — dependencia única.
- **Paralelizable** con #3 `overlap-protocol` (no se pisan: `mo` solo usa `ChangedFiles` como heurística).
- **Desbloquea** #5 (demo de 2 devs / gate HG6 conflict < ~15%): habilita ejercitar el loop de merge repetible.
- **Ownership:** todo el cambio cae en `module:devops` (HERMES). El cleanup toca archivos compartidos
  ya bajo HERMES (`ownership.md`, `.gitignore`, `merge-order.md`). Sin tocar nada fuera del módulo.

## Risks

- **Bypass de HG3 (merge automático a `develop`):** mitigado **estructuralmente** — `mo execute` HALTA
  en el borde del PR e imprime el `gh`; no existe ruta de código que mergee a `develop`.
- **Colisión con #3 (overlap):** evitada por diseño — `mo` no enforcea serialización por archivo, solo
  rankea; #3 mantiene su dominio.
- **Deriva del seam JSON:** si `wt list --json` cambia su forma, el DTO local debe seguir. El JSON
  estable de #1 (archivado) es el contrato; cambios requieren re-coordinación HERMES.
- **Falsos negativos de readiness:** MVP git-only puede marcar lista una rama sin CI verde; aceptado en
  MVP, cerrado por #4.

## Forecast

~700–950 LOC · **2 PRs encadenados** — PR1: go.mod + dominio + puertos + mocks + servicio + tests
unitarios (cero git); PR2: adapters (gitcli, wt-shell-out) + cmd + cleanup.
