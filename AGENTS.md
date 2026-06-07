# AGENTS.md — Roster de Talos

Definiciones canónicas en [`agents/`](agents/). Convenciones en [`CONSTITUTION.md`](CONSTITUTION.md).

## El equipo

| Figura | Rol | Modelo | Worktree | Tiene `Task` |
|---|---|---|---|---|
| [ATHENA](agents/athena.md) | Orquestador / PM | Opus | no (thread principal) | **Sí** |
| [ATLAS](agents/atlas.md) | Backend — bounded context #1 | Sonnet | sí | no |
| [HEPHAESTUS](agents/hephaestus.md) | Backend — bounded context #2 | Sonnet | sí | no |
| [CRONOS](agents/cronos.md) | Backend — bounded context #3 | Sonnet | sí | no |
| [IRIS](agents/iris.md) | Frontend | Sonnet | sí | no |
| [GAIA](agents/gaia.md) | Datos / DB | Sonnet | sí | no |
| [THEMIS](agents/themis.md) | QA / testing | Sonnet | sí | no |
| [HERMES](agents/hermes.md) | DevOps / entrega | Sonnet | sí | no |
| [ARGOS](agents/argos.md) | Revisión adversarial (Judgment Day) | routing propio | no | no |

## Patrón de invocación

**Orchestrator-workers vía Task tool.** ATHENA es el único con `Task` en sus tools y despacha a los
devs. **Ningún dev tiene `Task`** — los sub-agentes no spawnean sub-agentes. Cada dev pone
`isolation: worktree` en su frontmatter → worktree automático por invocación, limpiado si no hubo
cambios.

**Regla de escritura única:** un solo agente escritor por módulo. Antes de despachar, ATHENA chequea
colisión por JQL (`module:*` + `agent:*`) y consulta `team-context/ownership.md`.

## Contrato de cada dev (los 4 elementos que no se omiten)

1. **Objetivo** explícito (el issue de Jira + la tarea de `tasks.md`).
2. **Formato de salida** esperado (código + tests + verify + evidencia en Jira).
3. **Herramientas/fuentes** permitidas (su worktree, su módulo, los MCP por su ruta).
4. **Límites de tarea** (no tocar código fuera de su `module:*`; declarar archivos tocados en el issue).
