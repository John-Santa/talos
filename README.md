# TALOS

**TALOS** es una plataforma multi-agente de desarrollo: un orquestador PM (**ATHENA**, Opus)
planifica con el flujo SDD ([gentle-ai](https://github.com/gentleman-programming)) y maneja Jira
vía MCP, y un equipo de sub-agentes dev (Sonnet) implementa tareas SDD en paralelo, aislados en
git worktrees. **ARGOS** (revisión adversarial / Judgment Day) vigila los módulos; **ZEUS** (el TL
humano) es la autoridad final.

El sistema tiene dos capas:

- **El Kit** — la plataforma reutilizable (convenciones, defs de agentes, config SDD, blackboard,
  esquema de Jira, plantillas CI, bootstrap). Se extrae a su propio repo recién cuando la primera
  instancia cierra un change SDD completo.
- **La Instancia** — el monorepo concreto con el Kit aplicado. Este repo (`talos`) es la primera.

## Por dónde empezar

1. [`CONSTITUTION.md`](CONSTITUTION.md) — las convenciones congeladas que todo el sistema asume.
   **Leelo primero.** Es la pieza más reutilizable del Kit.
2. El plan de implementación completo (Fases 0→5) vive fuera del repo, en el plan aprobado de la
   sesión SDD.

## Roster

| Figura | Rol | Modelo |
|---|---|---|
| ATHENA | Orquestador / PM | Opus |
| ATLAS · HEPHAESTUS · CRONOS | Backend (1 bounded context c/u) | Sonnet |
| IRIS | Frontend | Sonnet |
| GAIA | Datos | Sonnet |
| THEMIS | QA / testing | Sonnet |
| HERMES | DevOps / entrega | Sonnet |
| ARGOS | Revisión adversarial (Judgment Day) | — |
| ZEUS | TL humano — autoridad final | humano |

## Estado

🚧 **Fase 0 — Constitución.** El repo se está bootstrapeando. Ver `CONSTITUTION.md`.
