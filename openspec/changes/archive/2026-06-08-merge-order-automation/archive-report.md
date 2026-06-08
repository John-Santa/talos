# Archive Report — merge-order-automation (TAL-3)

**Change:** merge-order-automation · **Jira:** TAL-3 · **Module:** module:devops · **Owner:** HERMES
**Fase 2 · change #2** · **Archived:** 2026-06-08 · **Store:** hybrid (openspec/ + engram)

## Resultado

Segunda entrega de la Fase 2. CLI Go `mo` (`platform/merge-order-orchestrator/`, hexagonal, cero deps) que automatiza la disciplina de merge §11 de `CONSTITUTION.md`: orden determinista seguro (deps → conflict-surface → FIFO), detección de colisiones sin mutar el working dir vía `git merge-tree --write-tree --name-only`, y secuencia one-at-a-time contra `develop` con re-chequeos del tip avanzado, **DETENIENDO siempre en el borde del PR** (HG3: aprobación humana para merge final, no puede ocurrir en código).

Consume `wt list --json` de change #1 (TAL-2, archivado) vía shell-out + DTO local; **posture HYBRID** decidida por ZEUS: `plan`/`check` read-only; `execute --yes` rebasea+secuencia pero nunca auto-mergea a `develop`.

## Entrega

| PR | Alcance | Estado |
|----|---------|--------|
| #7 | PR1 — domain + service (logic puro, cero git) | MERGED |
| #8 | PR2 — adapters + cmd + fixtures + cleanup (git real, wt shell-out) | MERGED |

`go test ./... -count=1` → **43 PASS / 0 FAIL** · short 5 PASS / 1 SKIP · integration 9 PASS · `go vet` limpio · go.mod cero deps.
Verify final: **0 CRITICAL · 4 WARNING · 3 SUGGESTION**.

## Decisiones clave (rationale completo en engram)

- **ADR-M1: Structural port split** — `Planner` (read-only) depende SOLO de `GitInspector` + `WorktreeLister` y NUNCA puede compilar un `RebaseOnto`. `IntegrationRunner` (write) es quien tiene `GitIntegrator`. "Plan es side-effect-free" es enforzado por tipos, no promesa. Honra HG3.
- **`mo` consume `wt list --json` vía shell-out** — no acopla go.mod; DTO local; JSON es el seam estable. Coexistencia, no acople.
- **Detección de colisiones = `git merge-tree --write-tree --name-only`** — pura lectura, no muta working dir. Git 2.50.1+.
- **Readiness MVP = git-only:** `active` en `wt list` + commits-ahead(`develop`) > 0. CI-green deferred a #4.
- **Posture HYBRID (ZEUS)** — `plan` read-only, `execute` rebasea but HALTA siempre en PR. Ruta de código impossible que auto-mergee a `develop`.
- **FIFO wired:** `BranchCreatedAt(ctx, branch)` → `git log -1 --format=%cI <branch>` → RFC3339. Resuelto en fix batch C-1.
- **Threshold field en JSON output** — REQ-PLAN-3, C-2 RESOLVED.
- **§11 mid-task-failure recipe printed** — REQ-EXECUTE-7/8, C-3 RESOLVED.

## Follow-up documentado (bajo riesgo, no bloqueante)

- **W-1**: REQ-PLAN-4 advisory para ramas behind-develop no implementado (sin crítico, seguimiento).
- **W-2**: `ErrNoCandidates` routing a stderr (REQ-READINESS-4 spirit — empty set should print friendly message, no error).
- **W-3**: `ErrWtBinaryNotFound` no incluye advice de cómo obtener el binary (spec require advice).
- **W-4**: health metric display en header, no footer (REQ-PLAN-2 dice end-of-output); falta recommendation text de segmentation re-balance.

## Estado del store

- Spec promovido a canónico: `openspec/specs/merge-order-orchestrator/functional.md`.
- Artefactos del change archivados en este directorio.
- Engram: topics `sdd/merge-order-automation/*` (#1853 proposal, #1854 spec, #1855 design, #1856 tasks, #1859 verify-report) + `architecture/talos-fase2-roadmap` + evolving.

## Gotchas de entorno / decisiones de implementación

- **Strict TDD:** todas las decisiones de arquitectura (ports, adapters, domain) cerradas en design pre-apply. 44 tests, cero skip en full mode (integration skip bajo -short by design). 
- **cero deps:** incluso stdlib-only. No time.Now() en los value objects; env derivation es determinista.
- **Coexistencia con #3 overlap-protocol:** `mo` rankea por `ChangedFiles` como heurística de surface-minima, no enforcea serialization. #3 mantiene su dominio.
- **Mock de GitRunner como test-seam:** inyectable en `service/Integrator` para test de rebase conflicts sin git real.

## Qué desbloquea

Cimiento de la automatización de merge. Habilita el **change #3 `overlap-protocol`** (refinar la serialización por archivo; `mo` ya rankea por surface), **change #4 `ci-hardening`** (agregar CI-green readiness, tocar `.yml`), y crucialmente el **change #5 `demo 2-devs`** donde se ejercita gate HG6 (tasa de conflictos < ~15%). La secuencia completa Fase 2: `#1 wt` → `#2 mo` → `#3 overlap` → `#4 ci` → `#5 demo`.

## Artefactos de especificación

| Artifact | Location | Observability |
|----------|----------|---|
| Proposal | `openspec/changes/archive/2026-06-08-merge-order-automation/proposal.md` | Engram #1853 |
| Spec (delta → canonical) | `openspec/specs/merge-order-orchestrator/functional.md` (newly promoted) | Engram #1854 |
| Design | `openspec/changes/archive/2026-06-08-merge-order-automation/design.md` | Engram #1855 |
| Tasks | `openspec/changes/archive/2026-06-08-merge-order-automation/tasks.md` | Engram #1856 |
| Verify Report | `openspec/changes/archive/2026-06-08-merge-order-automation/verify-report.md` | Engram #1859 |
| Archive Report | this file + Engram `sdd/merge-order-automation/archive-report` | traceability |

## Roadmap Alignment

Fase 2 roadmap (`[[architecture/talos-fase2-roadmap]]`):
1. **change #1 `worktree-orchestration` (TAL-2)** — archivado 2026-06-08. ✅
2. **change #2 `merge-order-automation` (TAL-3)** — archivado 2026-06-08 (THIS). ✅
3. **change #3 `overlap-protocol` (TAL-4)** — ready to start (blocked on #2 merge, now unblocked).
4. **change #4 `ci-hardening` (TAL-5)** — blocked on #3 integration + #2 readiness finalization (now unblocked).
5. **change #5 `demo 2-devs` (TAL-6)** — blocked on #4 CI green gate. Proof-of-concept 2 agents in parallel, gate HG6 collision < 15%.
