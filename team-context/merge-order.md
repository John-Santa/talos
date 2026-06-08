# Blackboard — Disciplina de merge

> Cómo se reconcilian las ramas de los agentes contra `develop`. ATHENA coordina (Fase 2+).

## Estrategia

- **Merge ordenado:** de a **un worktree a la vez** contra `develop`, **o** rebase-sobre-`develop`-antes-del-PR.
- Nunca dos merges concurrentes sin rebase previo.
- El worktree sin cambios se limpia solo.

## Orden de merge

Cuando hay varias ramas listas, ATHENA ordena por:
1. **Dependencias** primero (si B depende de A, A mergea antes).
2. Menor superficie de conflicto (módulos más aislados primero).
3. FIFO entre iguales (`ORDER BY created ASC` del issue).

## Manejo de fallo mid-task

Si un dev falla a mitad de la tarea:
1. `transition → To Do` (de vuelta).
2. Comentario en el issue con el error.
3. El worktree se **descarta** (no se mergea trabajo a medias).

## Métrica de salud

- **Tasa de conflictos de merge > ~15% → segmentación mala.** Re-segmentar (revisar `ownership.md`)
  antes de sumar más agentes. Registrar la métrica por sprint para decidir el escalado (gate HG6).
