# Blackboard — Ownership

> **Fuente de verdad de las colisiones.** Un módulo = un único agente escritor.
> Todos leen, **solo el dueño escribe**. ATHENA mantiene este archivo durante la fase `tasks`.

## Mapa módulo → agente

| `module:*` | Dueño (`agent:*`) | Dominio | Estado |
|---|---|---|---|
| `module:workspaces` | ATLAS | Proyectos orquestables (binding repo↔Jira) + vault local de credenciales. Local por máquina, sin multi-usuario. | definido |
| `module:orchestration` | HEPHAESTUS | Motor de despacho: issue → worktree → CLIs (`wt`/`mo`/`ov`/`ch`) → loop de evidencia. API que la consola consume. | definido |
| `module:runs` | CRONOS | Runs & observabilidad: historial de corridas, actividad de agentes, resultados de judgment, métricas (conflict rate, DoD). | definido |
| `module:frontend` | IRIS | UI / interacción (consola Go ahora; web a futuro) | active |
| `module:data` | GAIA | esquema / persistencia / migraciones | slot |
| `module:qa` | THEMIS | tests / harness / métricas | slot |
| `module:devops` | HERMES | CI / release / entornos | active |
| `module:jira-loop` | HERMES | Jira lifecycle / evidence plumbing | active |

> **Dominios de plataforma, no de negocio.** Talos es una consola de orquestación opensource que
> corre local en la máquina de cada usuario; cada instalación es un operador con sus propios
> proyectos (sin servidor central, sin multi-usuario, sin whitelist). Por eso los tres contextos
> backend son `workspaces` (qué se orquesta), `orchestration` (cómo se despacha) y `runs` (qué pasó),
> no auth/billing/catálogo. Son genuinamente independientes → admiten 3 worktrees backend en paralelo.
> Nota de persistencia: `workspaces` y `runs` probablemente requieran un store local (p.ej. SQLite)
> en vez del git-filesystem actual — decisión de GAIA (`module:data`) al construirlos.

## Reglas

1. **Invariante:** todo issue de dev tiene exactamente un `agent:*` y un `module:*`, y
   `ownership[module] == agent`. Si no, ATHENA lo rechaza.
2. **Escritura única:** ningún dev modifica archivos fuera de su `module:*` sin pasar por ATHENA.
3. **Declaración de archivos:** cada dev lista en el body del issue los archivos que va a tocar
   (`files:` checklist). ATHENA cruza esas listas entre issues que comparten `module:*` para detectar
   solape a nivel archivo (que JQL no puede ver).
4. **Mapa archivo → módulo:** cuando un archivo es compartido por naturaleza (config raíz, schema
   global), se anota acá su dueño explícito antes de asignar trabajo que lo toque.

## Mapa archivo → módulo (compartidos)

| Archivo / path | Dueño | Nota |
|---|---|---|
| `.env.example` | HERMES | Template raíz para vars de worktree; sin credenciales Jira reales |
| `platform/worktree-orchestrator/**` | HERMES | CLI `wt`, dominio, servicio, adapter gitcli |
| `platform/merge-order-orchestrator/**` | HERMES | CLI `mo`, orden/integración de merge |
| `platform/overlap-guard/**` | THEMIS | CLI `ov`, guard de solapamiento (module:qa) |
| `platform/ci-checks/**` | HERMES | CLI `ch`, invariante §4 |
| `platform/console/**` | IRIS | CLI/TUI `talos` (`module:frontend`); shell-out read-only a `wt`/`mo`/`ov`/`ch` + lectura de `ownership.md`/`openspec/`. NO importa cross-module. |
