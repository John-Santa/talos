# Archive Report — worktree-orchestration (TAL-2)

**Change:** worktree-orchestration · **Jira:** TAL-2 · **Module:** module:devops · **Owner:** HERMES
**Fase 2 · change #1** · **Archived:** 2026-06-08 · **Store:** hybrid (openspec/ + engram)

## Resultado

Primer change de la Fase 2. CLI Go `wt` (`platform/worktree-orchestrator/`, hexagonal, cero deps transitivas) que opera el aislamiento por git worktree para el paralelismo multi-agente: `create` / `list` / `teardown` / `env` con ramas `agent/<figura>/<TAL-N>` base `develop`, worktrees `talos.wt/agent-<figura>/` y `.env` con puerto + schema DB **disjuntos y deterministas** por figura.

## Entrega

| PR | Alcance | Estado |
|----|---------|--------|
| #3 | PR1 — domain + port + mock + service (lógica pura, cero git) | MERGED |
| #4 | PR2 — adapter/gitcli (git real) + cmd/wt + .env.example + ownership + .gitignore | MERGED |
| #5 | refactor — convención de comentarios (godoc solo en exportados) | MERGED |

`go test ./...` → **44 PASS / 0 FAIL** · short 37 PASS / 7 SKIP · `go vet` limpio · go.mod cero deps.
Verify final: **0 CRITICAL · 3 WARNING · 3 SUGGESTION**.

## Decisiones clave (rationale completo en engram)

- **No se usó la primitiva `EnterWorktree`/`ExitWorktree` del harness** — branchea de `origin/main`, naming random, ubica en `.claude/worktrees/`, no genera `.env`. No cumple la constitución §3/§11. Coexistencia, no acople.
- **Tooling = CLI Go hexagonal** (no shell/Makefile): testabilidad, consistencia de stack, portabilidad macOS/Ubuntu, strict TDD.
- **assign-map hardcoded** (atlas→8100 … argos→8107, schema `wt_<figura>`) con `// DEBT(kit-extraction)` — re-evaluar en Fase 5.
- **`RenderEnv` puro/determinista** sin `time.Now()` → `env` re-deriva byte-idéntico.
- **`GitRunner` como test-seam port** (no swapability) — habilita testear el service con errores git inyectados.
- **W-2/W-4 corregidos pre-merge**: `wt list` → `active/orphan/stale` (contrato que consume change #2); `ErrDirtyWorktree` con figura real (patrón sentinel) + file count.

## Follow-up documentado (bajo riesgo, no bloqueante)

- **W-1**: el `.env` generado no lleva comentario header estático (REQ-ENV-2).
- **W-3**: sin fallback a `origin/develop` si `develop` local no existe.
- **W-5**: `ErrDevelopNotAvailable` ausente del catálogo de errores.

## Gotcha de entorno

El sandbox bloquea escribir cualquier `.env*` en la raíz del repo (Write y Bash redirect). `.env.example` fue creado manualmente por ZEUS. Futuros changes que necesiten archivos `.env*` en raíz: crear a mano o ajustar permisos.

## Estado del store

- Spec promovido a canónico: `openspec/specs/worktree-orchestrator/functional.md`.
- Artefactos del change archivados en este directorio.
- Engram: topics `sdd/worktree-orchestration/*` + `architecture/talos-fase2-roadmap` + `convention/code-comments`.

## Qué desbloquea

Cimiento físico del paralelismo de Fase 2. Habilita el **change #2 `merge-order-automation`** (consume `wt list` con la taxonomía `active/orphan/stale` ya correcta) y, en cadena, #3 overlap-protocol → #4 ci-hardening → #5 demo 2-devs (gate colisión < 15%).
