# Skill Registry — talos

> Registry de skills del proyecto. El orquestador (ATHENA) inyecta los **Compact Rules** que matcheen
> en el prompt de cada sub-agente (no los SKILL.md). Generado a mano (skill-registry skill bloqueado).

## Compact Rules (auto-resueltos, inyectar en sub-agentes)

### Convenciones del proyecto (de CONSTITUTION.md)
- **Ownership:** un módulo = un único agente escritor. No tocar archivos fuera del propio `module:*`
  sin pasar por ATHENA. Declarar archivos tocados en el body del issue de Jira.
- **Backend (regla dura):** 3 bounded contexts verticales independientes (ATLAS/HEPHAESTUS/CRONOS);
  corte por dominio, nunca por capa. Si no son independientes → serializar.
- **Identidad:** por label `agent:*` (no por assignee). Branch: `agent/<figura>/<JIRA-KEY>`.
- **Labels:** 1 `agent:*` + 1 `module:*` por issue; `change:*`; `phase:*` opcional.
- **DoD:** PR linkeado + CI verde + verify-report. Linkear lo vivo, adjuntar lo muerto.
- **MCP routing:** Rovo (lifecycle) vs community (adjuntos/remote-links) — ver `.mcp/routing.md`.

### Stack
- **Go (backend):** TDD estricto con `go test ./...`. Table-driven tests; golden files cuando aplique.
  Aplicar la skill **go-testing** (teatest/Bubbletea, golden, coverage).
- **React + Vite (frontend):** TypeScript; tests con `vitest run`. Para UI/UX aplicar **ui-ux-pro-max**.

### Testing / TDD
- **Strict TDD activo.** Rojo → verde → refactor. No saltear el test que falla primero.

## User Skills — tabla de triggers

| Skill | Cuándo aplicarla |
|---|---|
| `go-testing` | Tests Go, coverage, teatest, golden files |
| `ui-ux-pro-max` | Diseño/implementación/review de UI React |
| `branch-pr` | Crear/abrir PRs (issue-first) |
| `chained-pr` | PRs > 400 líneas → slices encadenados |
| `work-unit-commits` | Planificar commits como work units reviewables |
| `judgment-day` | Revisión adversarial (ARGOS) — gate pre-archive Fase 3+ |
| `issue-creation` | Crear issues de GitHub/Jira |
| `comment-writer` | Feedback en PRs/issues/reviews |
| `cognitive-doc-design` | Docs (guías, RFCs, onboarding, READMEs) |
| `sdd-explore/propose/spec/design/tasks/apply/verify` | Fases SDD (vía Agent subagent_type, NO Skill tool) |

## Nota de entorno

Los skills SDD invocados por el **Skill tool** fallan con un error de permisos del sandbox
(`!`echo -n "$(pwd)"``). Correr las fases SDD vía **Agent `subagent_type`** (`sdd-explore`,
`sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`).
