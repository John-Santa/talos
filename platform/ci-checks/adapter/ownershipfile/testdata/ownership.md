# Blackboard — Ownership

> Test fixture for ownershipfile adapter tests.

## Mapa módulo → agente

| `module:*` | Dueño (`agent:*`) | Dominio | Estado |
|---|---|---|---|
| `module:devops` | HERMES | CI / release / entornos | active |
| `module:qa` | THEMIS | tests / harness / métricas | active |
| `module:frontend` | IRIS | UI / interacción | slot |
| `module:data` | GAIA | esquema / persistencia | slot |
| `module:backend-ctx1` | ATLAS | backend — bounded context #1 | slot |

## Mapa archivo → módulo (compartidos)

| Archivo / path | Dueño | Nota |
|---|---|---|
| `platform/ci-checks/**` | HERMES | CLI `ch`, invariante §4 |
| `platform/overlap-guard/**` | THEMIS | CLI `ov`, guard de solapamiento |
