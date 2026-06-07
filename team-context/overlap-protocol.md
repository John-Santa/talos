# Blackboard — Protocolo de solape

> Qué hacer cuando dos agentes deben tocar el mismo dominio. ATHENA arbitra.

## Detección (antes de asignar)

ATHENA, antes de despachar un issue, corre JQL para ver si otro issue activo toca el mismo módulo:

```sql
-- ¿Otro agente (≠ dueño) tiene trabajo activo en este módulo?
project = TAL AND statusCategory != Done
  AND labels = "module:<X>" AND labels NOT IN ("agent:<dueño>")
```

## Resolución (por severidad)

1. **Mismo módulo, distinto archivo →** los dos agentes anotan en el blackboard su intención y la
   sección/archivos que tocan. ATHENA decide el **orden**; el `transition → In Progress` marca el
   módulo como tomado. Pueden ir en serie cercana, no en paralelo ciego.
2. **Mismo archivo →** **nunca en paralelo.** Se **serializa** (uno espera) o se **re-segmenta** el
   trabajo para que cada agente tenga archivos disjuntos.
3. **Cross-change (dos changes SDD activos sobre el mismo módulo) →** se serializa por
   `team-context/merge-order.md`.

## Señales de coordinación

- `transition → In Progress` = "módulo tomado".
- Label `agent:*` = quién lo tomó.
- El blackboard (este directorio) = dónde se anota la intención antes de tocar código compartido.

## Cuándo NO paralelizar (escala a ZEUS si hay duda)

- Dos issues sobre el mismo archivo.
- El codebase no se segmenta limpio en el dominio en cuestión.
- Tasa de conflictos de merge **> ~15%** → la segmentación está mal; re-segmentar antes de sumar agentes.
