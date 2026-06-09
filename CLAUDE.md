# CLAUDE.md — Talos (contexto de instancia)

> Contexto global del proyecto para cualquier agente que trabaje en este repo.
> La **fuente de verdad de las convenciones** es [`CONSTITUTION.md`](CONSTITUTION.md). Leela primero.

## Qué es este repo

`talos` es la **primera instancia** de la plataforma multi-agente TALOS. En esta etapa, el producto
que se construye **es la propia plataforma** (dogfooding): el primer entregable real es el helper del
loop de evidencia de Jira (ver `platform/`).

## Cómo se opera

- El orquestador es **ATHENA**, que es la persona/configuración del `sdd-orchestrator` global
  (vive en `~/.claude/`), no un engine nuevo. ATHENA planifica con el flujo SDD de gentle-ai y
  reparte el trabajo entre los devs.
- Los devs (ATLAS·HEPHAESTUS·CRONOS·IRIS·GAIA·THEMIS·HERMES) implementan tareas SDD, cada uno
  **dueño de un módulo** (ver `team-context/ownership.md`), aislados en git worktrees (Fase 2+).
- **ARGOS** corre la revisión adversarial (Judgment Day) — no implementa.
- **ZEUS** (el TL humano) aprueba los gates (ver `CONSTITUTION.md` §10).

## Convenciones que NO se negocian (resumen — detalle en la constitución)

- **Ownership:** un módulo = un único agente escritor. Backend = 3 bounded contexts verticales (§2).
- **Identidad por label** `agent:*`, no por assignee (§5).
- **DoD** = PR linkeado + CI verde + verify-report; linkear lo vivo, adjuntar lo muerto (§6).
- **Routing MCP híbrido:** Rovo para lifecycle, community para adjuntos/remote-links (§7).
- **Branch naming:** `agent/<figura>/<JIRA-KEY>` (§3).
- **Store SDD híbrido:** `openspec/` + engram; sin `docs/sdd/` hasta Fase 5 (§9).

## Stack técnico

- **Backend: Go** (`go test ./...`, strict TDD; skill `go-testing` disponible).
  Devs backend: ATLAS · HEPHAESTUS · CRONOS. También GAIA (datos: Go + SQL/migraciones).
- **Frontend (IRIS): consola primero.** La capa visible arranca como **TUI Go + Bubbletea**
  (módulo `platform/console`, binario `talos`; `go test ./...`, strict TDD, skill `go-testing`).
  **React + Vite** (TS, vitest) queda para una eventual promoción a web — todavía no se construye.

> Nota de diseño para Fase 1: con backend Go, el helper `jira-evidence-loop` puede ser un módulo/CLI
> Go contra el **REST de Jira** (token propio) — eso habilita adjuntos sin depender del MCP community.
> Go-REST-directo vs MCP lo resuelve el `explore` del primer change; no está prejuzgado.

## Memoria

Engram activo. Las decisiones de arquitectura están en el topic `architecture/talos-system`.
