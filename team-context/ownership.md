# Blackboard — Ownership

> **Fuente de verdad de las colisiones.** Un módulo = un único agente escritor.
> Todos leen, **solo el dueño escribe**. ATHENA mantiene este archivo durante la fase `tasks`.

## Mapa módulo → agente

| `module:*` | Dueño (`agent:*`) | Dominio | Estado |
|---|---|---|---|
| `module:backend-ctx1` | ATLAS | backend — bounded context #1 (nombre real ← `explore`) | slot |
| `module:backend-ctx2` | HEPHAESTUS | backend — bounded context #2 (nombre real ← `explore`) | slot |
| `module:backend-ctx3` | CRONOS | backend — bounded context #3 (nombre real ← `explore`) | slot |
| `module:frontend` | IRIS | UI / interacción | slot |
| `module:data` | GAIA | esquema / persistencia / migraciones | slot |
| `module:qa` | THEMIS | tests / harness / métricas | slot |
| `module:devops` | HERMES | CI / release / entornos | active |
| `module:jira-loop` | HERMES | Jira lifecycle / evidence plumbing | active |

> Los `backend-ctxN` son **slots**: la fase `explore` les asigna el nombre del bounded context real
> (auth, billing, catálogo…). Si no salen 3 contextos backend genuinamente independientes,
> **se serializan dos de ellos** — no se fuerzan 3 worktrees backend en paralelo.

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
