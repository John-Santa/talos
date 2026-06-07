# Routing MCP (híbrido) — referencia

> Los dos servidores Atlassian ya están conectados a nivel Claude Code (ambos autenticados contra
> `tablex.atlassian.net`). Este repo **no** lleva config de conexión propia; este archivo es la
> **referencia de qué llamada va por cuál server**. Convención congelada en `CONSTITUTION.md` §7.

## Servidores

| Alias | Namespace | Identidad | Capacidad clave |
|---|---|---|---|
| **Rovo (oficial)** | `mcp__claude_ai_Atlassian__*` | John (compartida) | lifecycle del issue; **NO** adjuntos |
| **Community** | `mcp__atlassian__*` | John (compartida) | **adjuntos** + **remote issue links** + batch |

## Loop de evidencia — qué server hace cada paso

| # | Paso | Server | Llamada |
|---|---|---|---|
| 1 | Crear issue de la tarea SDD | Rovo | `createJiraIssue` (+labels `agent:*`,`module:*`,`change:*`) |
| 2 | Pasar a In Progress al iniciar apply | Rovo | `transitionJiraIssue` (+`phase:apply`) |
| 3 | Comentar progreso / log de tiempo | Rovo | `addCommentToJiraIssue` / `addWorklogToJiraIssue` |
| 4 | Linkear el PR (dev panel / remote link) | **Community** | `jira_create_remote_issue_link` |
| 5 | Adjuntar verify-report / artefactos | **Community** | `jira_update_issue` (+attachment) |
| 6 | Transicionar a Done tras verify+archive | Rovo | `transitionJiraIssue` |
| 7 | Queries de colisión/identidad | Rovo | `searchJiraIssuesUsingJql` |

## Reglas

- **Linkear lo vivo, adjuntar lo muerto.** Community se usa SOLO en pasos 4–5 (set mínimo).
- El helper **falla fuerte** si un adjunto/remote-link errorea — no se traga el error.
- Proyecto team-managed → **leer transiciones con `getTransitionsForJiraIssue`**, nunca hardcodear IDs.
- Ningún MCP crea proyectos → TALOS se crea manual (gate HG1).
