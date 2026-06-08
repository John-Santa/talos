# Proposal — overlap-protocol (TAL-4)

> Change #3 · Fase 2 · `module:qa` · owner THEMIS · branch `agent/themis/TAL-4` (base `develop`)
> Store: hybrid (`openspec/` + engram). Status: proposed.

## Why / Context

`team-context/overlap-protocol.md` y `team-context/ownership.md §3` definen una **disciplina de solape** —
detectar antes de asignar si otro agente toca el mismo módulo, y aplicar la regla **dura** "mismo
archivo → **nunca en paralelo**" (serializar o re-segmentar)— pero hoy esa disciplina es **totalmente
manual**. ATHENA corre el JQL a mano antes de despachar, lee los checklists `files:` de cada body a ojo,
y decide orden/serialización sin una verificación reproducible. Ese arbitraje no escala y no es
determinista: con 2 agentes es tolerable, con 5+ es la fuente principal de colisión.

Este es el change **#3** del roadmap de Fase 2. Es **prevención**, no integración: va **antes** del demo
de 2 devs (#5) porque alimenta el gate humano **HG6** — la decisión de escalar 2 → 5+ agentes exige
**tasa de colisión < 15%** (`overlap-protocol.md` §"Cuándo NO paralelizar"). Sin una métrica
reproducible, HG6 se decide por intuición.

Su única dependencia es **#1 `worktree-orchestration`** (TAL-2, **archivado**), que entregó el CLI `wt`
con `wt list --json` — el inventario de worktrees activos que el scan T1 consume. **#2
`merge-order-automation`** (TAL-3, **archivado**) ya entregó `mo`; este change es **complementario**, no
solapado (ver "Complementariedad con `mo`").

**Success looks like:** un CLI `ov` que, dado (a) los issues activos de Jira y (b) el inventario de `wt`,
**bloquea** de forma determinista cualquier asignación o par de ramas que toquen el mismo archivo entre
agentes distintos, **serializa** los solapes a nivel módulo, y **reporta** la tasa de colisión que HG6
necesita — todo sin tocar el working dir y sin acoplar Go con `mo`.

## What changes

Un nuevo módulo Go `platform/overlap-guard/` con el CLI **`ov`**, que **espeja** la arquitectura
hexagonal de los módulos existentes (`wt`, `mo`, `evidence`): go 1.26, **cero dependencias** (stdlib
only), dominio / puertos / servicio testeados sin I/O, adapters aislados. **Autocontenido**: cross-CLI es
**seam JSON / shell-out**, **nunca import Go**. Tres subcomandos (`cmd/ov/main.go`):

- `ov check --module X --agent owner [--files-file f] [--site-url u] [--json]` — **T0**, gate Jira
  **pre-asignación**. Corre el JQL de `overlap-protocol.md`
  (`project = TAL AND statusCategory != Done AND labels = "module:X" AND labels NOT IN ("agent:owner")`),
  parsea el checklist `files:` de cada body de issue, y lo cruza con los archivos declarados por el owner.
  Exit **1 = BLOCK** (same-file con otro agente), **0 = OK / SERIALIZE**.
- `ov scan [--base develop] [--wt-bin wt] [--no-fetch] [--ownership-file f] [--json]` — **T1**, scan de
  solape de archivos **cross-agente** sobre los worktrees activos. Consume `wt list --json` (shell-out),
  computa `ChangedFiles` (`base...branch`) por rama, y detecta colisión de archivo pairwise entre
  hermanos. Exit **1 = BLOCK**, **0 = OK / SERIALIZE / vacío**.
- `ov metric [--threshold 0.15] [--strict] [scan flags] [--json]` — **HG6**, tasa de colisión sobre el
  scan T1. **Informativo** (no rompe el pipeline) salvo `--strict`.

`ov` **re-implementa su propio `gitcli`** (un `Fetch` / `RevParse` / `ChangedFiles` de ~50 ln, espejo del
de `mo` pero **propio**) y consume `wt list --json` por shell-out. **NO toca `mo`**:
`platform/merge-order-orchestrator/**` es single-writer de HERMES — `ov` no lo importa ni lo edita. El
adapter `jirarest` espeja `platform/jira-evidence-loop/adapter/rest/client.go`, pero **pide además el
campo `description`** (que `evidence` no pide) y **aplana el ADF** a texto plano antes de parsear el
checklist.

**Edits compartidos (gated, fuera del módulo — requieren visto de ATHENA):**
- `team-context/ownership.md`: nueva fila en el mapa archivo→módulo →
  `platform/overlap-guard/**` → THEMIS / `module:qa`.
- `.gitignore`: `/platform/overlap-guard/ov` (binario compilado).

## Complementariedad con `mo` (no se solapan)

`mo` y `ov` responden preguntas **distintas** y mantienen dominios disjuntos:

| | `mo` (TAL-3, archivado) | `ov` (este change) |
|---|---|---|
| Pregunta | "¿esta rama conflictúa con `develop` **al mergear**?" | "¿dos ramas/issues de agentes **distintos** tocan el **mismo archivo**?" |
| Mecánica | `git merge-tree` vs base; ranking por superficie de conflicto | colisión en el **origen**, pairwise entre hermanos |
| Resultado | heurística de **orden** (no enforcea) | **regla dura enforced** (BLOCK same-file) |
| Momento | integración (post-trabajo) | prevención (pre-asignación / durante) |

El archive-report de `mo` reservó explícitamente este dominio: *"mo rankea `ChangedFiles` como heurística,
no enforcea serialización; #3 mantiene su dominio"*. `ov` es quien **enforcea**.

## Scope

### In scope
- Módulo Go `platform/overlap-guard/` con CLI `ov` (subcomandos `check` / `scan` / `metric`) — **alcance
  FULL**: los tres subcomandos (T0 + T1 + métrica).
- **`gitcli` propio** (`Fetch` / `RevParse` / `ChangedFiles`, ~50 ln) — re-implementado, sin importar `mo`.
- Consumo de `wt list --json` vía **shell-out** + DTO local.
- Adapter `jirarest` (espejo de `evidence`, con `description` + aplanado ADF) para el JQL de T0.
- Modelo de dominio: `Claim` (declared/T0 | actual/T1), `FileCollisions()` (hard), `ModuleOverlaps()`
  (soft), `Verdict{OK|SERIALIZE|BLOCK}` con precedencia BLOCK > SERIALIZE > OK, `Report` con
  `CollisionRate` (boundary `0.15` strictly-greater, igual que `mo`).
- Edits compartidos gated listados arriba (`ownership.md`, `.gitignore`).

### Out of scope (deferrals explícitos)
- **Ranking / orden de merge:** dominio de `mo` (#2, archivado). `ov` enforcea same-file, **no ordena**.
- **Ejecución / serialización automática:** `ov` **decide y reporta** el `Verdict`; **no** rebasea, no
  mueve issues, no abre PRs. La acción (serializar, re-segmentar, escalar a ZEUS) la ejecuta ATHENA/humano.
- **Cross-change vía `mo`:** "dos changes SDD activos sobre el mismo módulo" se serializa **difiriendo a
  `mo`** (`merge-order.md`) — `ov` no re-implementa ese orden.
- **Mutación de Jira:** `ov check` es **read-only** sobre Jira (sólo Search). No transiciona, no comenta.
- **Auto-fix / re-segmentación automática del codebase.**
- **CI-green readiness** (dominio de #4 `ci-hardening`).

## Resolved decisions (por ZEUS — no re-litigar)

| # | Decisión | Resolución | Por qué |
|---|----------|------------|---------|
| 1 | Dónde vive | CLI Go nuevo `ov` en `platform/overlap-guard/`, hexagonal, cero deps | Espeja `mo`/`wt`; autocontenido; honra single-writer por módulo |
| 2 | Cómo lee git | **`gitcli` propio** (~50 ln), NO importa `mo` | `platform/merge-order-orchestrator/**` es exclusivo de HERMES; cross-CLI = seam, nunca import Go |
| 3 | Cómo lee worktrees | Shell-out a `wt list --json` + DTO local | Seam JSON estable de #1; sin acople de go.mod |
| 4 | Alcance | **FULL**: T0 (`check`) + T1 (`scan`) + métrica (`metric`) | #3 cubre la disciplina completa de `overlap-protocol.md` |
| 5 | Regla dura | same-file entre agentes distintos → **BLOCK** (exit 1), precedencia BLOCK > SERIALIZE > OK | "mismo archivo → nunca en paralelo" es invariante (`overlap-protocol.md §2`) |
| 6 | Boundary de métrica | `CollisionRate` over-threshold = **strictly >** `0.15` | Coherente con el `0.15` de `mo` y el "> ~15%" de `overlap-protocol.md` |
| 7 | Posture de `metric` | Informativo por defecto; rompe el pipeline sólo con `--strict` | HG6 es gate **humano**; la métrica informa, no decide sola |

## Impact / dependencies

- **Consume** #1 `worktree-orchestration` (TAL-2, archivado) vía `wt list --json` — **dependencia única**.
- **Consume** Jira REST (Search) para T0 — espeja el adapter de `jira-evidence-loop`, **sin** importarlo.
- **Complementa** #2 `merge-order-automation` (TAL-3, archivado): dominios disjuntos (ver tabla). `ov` NO
  toca `platform/merge-order-orchestrator/**`.
- **Desbloquea / alimenta** el demo de 2 devs (#5) y el gate **HG6** (colisión < 15% para escalar 2→5+).
- **Ownership:** el módulo entero cae en `module:qa` (THEMIS). Los edits compartidos (`ownership.md`,
  `.gitignore`) son gated por ATHENA. **Sin tocar nada de otro `module:*`.**

## Risks / open questions (los resuelve spec/design)

> Decisiones ya tomadas arriba. Estos tres quedan **abiertos para `sdd-spec` / `sdd-design`**:

1. **Convención del checklist `files:` (→ spec).** `ownership.md §3` la manda, pero **ningún issue conocido
   la trae**. El spec debe **fijar la gramática** (`- [ ] path`) y el **fallback**: issue sin checklist →
   **advisory ruidoso** (`ErrChecklistMissing`), se **excluye del nivel-archivo** pero **NO del
   nivel-módulo**. Invariante: **"sin checklist" NUNCA se trata como "sin solape"**.
2. **Atribución de módulo en T1 (→ design).** La rama da la **figura**, no el **módulo**; la regla
   same-file → BLOCK **no necesita módulo** (es pairwise sobre archivos). `--ownership-file` sólo
   **enriquece** (clasificar SERIALIZE de módulo). El design decide si `scan` necesita módulo o lo deja
   opcional.
3. **ADF body (→ design).** La `description` de Jira es **ADF JSON**; el adapter `jirarest` debe
   **aplanarla a texto plano** antes de parsear el checklist. (Confirmado: el `rest.Client` de `evidence`
   hoy **no pide** `description` en su Search — `ov` debe agregarlo.)

## Roadmap alignment

- Roadmap Fase 2: `[[architecture/talos-fase2-roadmap]]`. Este es **#3 (prevención)**, va antes de #5.
- Antecedente complementario: `[[sdd/merge-order-automation/archive-report]]` (reservó el dominio de #3).
- Disciplina fuente: `team-context/overlap-protocol.md`, `team-context/ownership.md §3`.

## Owner / delivery

- **Owner:** THEMIS · `module:qa` · branch `agent/themis/TAL-4` (base `develop`).
- **Forecast:** ~2060 LOC · **2 PRs encadenados** (espeja la entrega de #2):
  - **PR1** = domain + service + mocks (**puro, sin I/O**): `Claim`, `FileCollisions`,
    `ModuleOverlaps`, `Verdict`, `Report`/`CollisionRate`, errores (`ErrSameFileParallel`,
    `ErrEscalateZeus`, `ErrChecklistMissing`, `ErrNoClaims`) — todo table-driven, mocks hand-written.
  - **PR2** = adapters (`jirarest` / `wtcli` / `gitcli` propio) + `cmd/ov` + I/O real (tests de
    integración bajo `-short` skip) + edits compartidos gated.
- **DoD:** PR linkeado + CI verde + verify-report. Strict TDD (`go test ./...`, rojo→verde→refactor).
