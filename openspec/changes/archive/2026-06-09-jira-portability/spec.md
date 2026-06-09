# Spec: jira-portability — Functional Requirements

**Change:** jira-portability
**Jira:** TAL-6
**Module:** module:devops (HERMES lidera) + module:qa (THEMIS, slice `ov`)
**Owner:** HERMES + THEMIS
**RFC 2119 keywords apply throughout.**

---

## Overview

Este spec describe el comportamiento observable que MUST ser verdadero después de aplicar el change
`jira-portability`. No prescribe implementación ni nombres de paquetes — eso es responsabilidad de la
fase de diseño.

El sistema bajo especificación abarca las tres CLIs que interactúan con Jira dentro de la plataforma
Talos: `evidence` (`jira-evidence-loop`), `ch` (`ci-checks`), y `ov` (`overlap-guard`). El objetivo es
que esas CLIs operen contra **cualquier proyecto Jira** sin modificar código Go — alcanzable sólo
llenando archivos de configuración.

Este change es **Kit-PREP** (no extracción del Kit — eso es Fase 5): mata dos acoplamientos duros que
hoy impiden clonar Talos a un proyecto distinto: (1) ausencia de carga local-first de secrets; (2)
identidad de proyecto hardcodeada, incluidas las expresiones regulares de dominio que exigen `TAL-`.

---

## REQ-ENV — Loader de archivos de entorno

### REQ-ENV-1: Carga de variables al arranque

**Given** una CLI que toca Jira (`evidence`, `ch`, `ov`) es invocada,
**When** el proceso inicializa (antes de evaluar cualquier flag o subcomando),
**Then** MUST cargar variables de entorno desde los archivos de configuración locales siguiendo la
precedencia: entorno real (del proceso) > `.env` (gitignoreado) > `.talos/project.env` (committeado).
Una variable ya presente en el entorno del proceso NO DEBE ser sobreescrita por ningún archivo.

### REQ-ENV-2: Clave ya seteada no se pisa

**Given** la variable `K` ya está presente en el entorno del proceso con valor `V_real`,
**When** el loader procesa un archivo que también define `K=V_archivo`,
**Then** el valor del proceso (`V_real`) MUST permanecer sin cambios.
El loader MUST NOT sobreescribir variables ya presentes, independientemente del archivo.

### REQ-ENV-3: Clave no seteada se popula desde archivo

**Given** la variable `K` no está presente en el entorno del proceso,
**When** el loader procesa un archivo que define `K=V`,
**Then** `K` MUST quedar seteada con valor `V` en el entorno del proceso.

### REQ-ENV-4: `.env` tiene precedencia sobre `.talos/project.env` para la misma clave

**Given** ambos archivos (`.env` y `.talos/project.env`) están presentes y definen la misma clave `K`
con valores distintos,
**When** el loader los procesa,
**Then** el valor de `.env` MUST prevalecer sobre el de `.talos/project.env`.
(Esto es un corolario de REQ-ENV-1: `.env` se procesa antes que `project.env`; la primera asignación
gana; la segunda — para la misma clave — la ignora porque ya está seteada.)

### REQ-ENV-5: Archivo ausente se saltea sin error

**Given** uno o ambos archivos de configuración (`.env`, `.talos/project.env`) no existen en el
sistema de archivos,
**When** el loader intenta cargar ese archivo,
**Then** MUST saltearlo silenciosamente, sin producir error y sin afectar las demás variables.
La ausencia de ambos archivos MUST NOT causar fallo de ningún tipo.

### REQ-ENV-6: Error de lectura no "not-exist" se superficie

**Given** un archivo de configuración existe pero no puede leerse (p.ej. permisos insuficientes),
**When** el loader intenta abrirlo,
**Then** MUST retornar o surfacear el error subyacente.
El error MUST distinguirse de la condición "archivo no encontrado" (REQ-ENV-5) y MUST incluir el
path del archivo afectado.

### REQ-ENV-7: Comportamiento en CI no se modifica

**Given** el entorno de CI ya tiene `JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_SITE_URL` y demás variables
seteadas como secrets del runner,
**When** la CLI corre en CI (con o sin archivos `.env` / `.talos/project.env`),
**Then** el comportamiento MUST ser idéntico al estado anterior a este change: el entorno real gana
siempre (REQ-ENV-2), y ningún archivo puede pisar esas variables.

---

## REQ-PARSE — Parser de archivos `KEY=VALUE`

### REQ-PARSE-1: Formato `KEY=VALUE`

**Given** un archivo de configuración en formato texto plano,
**When** el parser lo procesa,
**Then** MUST interpretar cada línea no-blank y no-comentario como un par `KEY=VALUE`, donde el
separador es el **primer** `=` de la línea.

### REQ-PARSE-2: Líneas en blanco y comentarios ignorados

**Given** un archivo que contiene líneas vacías y líneas que comienzan con `#`,
**When** el parser las encuentra,
**Then** MUST saltearse esas líneas silenciosamente.
Una línea con sólo espacios o tabs también se considera en blanco.

### REQ-PARSE-3: Split en el primer `=` (valor puede contener `=`)

**Given** una línea de la forma `K=a=b=c`,
**When** el parser la procesa,
**Then** MUST producir `KEY="K"` y `VALUE="a=b=c"`.
Todo lo que aparece después del primer `=` conforma el valor.

### REQ-PARSE-4: Trim de espacios en clave y valor

**Given** una línea con espacios alrededor de la clave y/o el valor (p.ej. `  K  =  V  `),
**When** el parser la procesa,
**Then** MUST recortar los espacios en blanco al inicio y al final de la clave y del valor.
El resultado MUST ser `KEY="K"` y `VALUE="V"`.

### REQ-PARSE-5: Línea sin `=` ignorada

**Given** una línea que no contiene el carácter `=` (y no comienza con `#` ni es en blanco),
**When** el parser la encuentra,
**Then** MUST ignorarla silenciosamente sin producir error ni par clave-valor.

### REQ-PARSE-6: Tolerancia a CRLF

**Given** un archivo con terminaciones de línea CRLF (Windows),
**When** el parser lo procesa,
**Then** MUST producir los mismos pares `KEY=VALUE` que con terminaciones LF.
El `\r` final de cada valor MUST ser recortado.

### REQ-PARSE-7: Archivo vacío produce resultado vacío

**Given** un archivo vacío (cero bytes),
**When** el parser lo procesa,
**Then** MUST retornar un resultado vacío (sin pares) y sin error.

---

## REQ-IDENT — Resolución de identidad del proyecto

### REQ-IDENT-1: Variables de identidad dinámicas desde el entorno

**Given** las variables `JIRA_SITE_URL`, `JIRA_PROJECT_KEY` y `JIRA_PROJECT_ID` están presentes en
el entorno del proceso (sea por herencia, por `.env`, o por `.talos/project.env`),
**When** cualquiera de las CLIs Jira construye su configuración,
**Then** MUST usar los valores de esas variables para operar: site URL, clave de proyecto e ID
de proyecto, respectivamente.

### REQ-IDENT-2: Fallback a valores por defecto TAL sin configuración

**Given** ninguna de las variables `JIRA_PROJECT_KEY` ni `JIRA_PROJECT_ID` está presente en el
entorno, y no existe `.talos/project.env`,
**When** la CLI construye su configuración,
**Then** MUST comportarse de forma idéntica al estado anterior a este change: operar con la clave
`TAL` y el ID `10099` (back-compat, cero flag-day, sin `.talos/project.env` no hay cambio observable).

### REQ-IDENT-3: Operación contra un proyecto distinto de TAL

**Given** `.talos/project.env` (o el entorno heredado) define `JIRA_PROJECT_KEY=FOO` y
`JIRA_PROJECT_ID=20001`,
**When** cualquiera de las CLIs Jira corre,
**Then** MUST operar contra el proyecto `FOO`/`20001`, sin que ninguna línea de código Go haya sido
modificada.
Toda llamada a la API Jira MUST referenciar el proyecto `FOO`.

### REQ-IDENT-4: Contenido committeado en `.talos/project.env` — sólo identidad no-secreta

**Given** el archivo `.talos/project.env` existe en el repositorio,
**When** su contenido es inspeccionado,
**Then** MUST contener exclusivamente variables de identidad no-secreta: `JIRA_SITE_URL`,
`JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`.
MUST NOT contener `JIRA_EMAIL`, `JIRA_API_TOKEN`, ni ninguna otra credencial.

---

## REQ-BRANCH — Validación de ramas parametrizada

### REQ-BRANCH-1: Patrón de rama agente usa la clave de proyecto configurada

**Given** la clave de proyecto está configurada como `<KEY>` (p.ej. `TAL`, `FOO`),
**When** una rama es validada como rama canónica de agente,
**Then** MUST seguir el patrón `agent/<figura>/<KEY>-<n>` donde `<KEY>` es la clave configurada y
`<n>` es un número entero positivo.
Una rama que use una clave de proyecto distinta a la configurada MUST ser rechazada con `ErrNoJiraKey`.

### REQ-BRANCH-2: Rama válida bajo la clave configurada

**Given** la clave configurada es `TAL` y la rama evaluada es `agent/hermes/TAL-7`,
**When** la validación corre,
**Then** MUST retornar `figura="hermes"`, `key="TAL-7"` sin error.

### REQ-BRANCH-3: Rama de proyecto incorrecto bajo clave TAL

**Given** la clave configurada es `TAL` y la rama evaluada es `agent/hermes/FOO-7`,
**When** la validación corre,
**Then** MUST retornar `ErrNoJiraKey`.

### REQ-BRANCH-4: Rama válida bajo clave alternativa

**Given** la clave configurada es `FOO` y la rama evaluada es `agent/hermes/FOO-7`,
**When** la validación corre,
**Then** MUST retornar `figura="hermes"`, `key="FOO-7"` sin error.

### REQ-BRANCH-5: Rama de proyecto incorrecto bajo clave alternativa

**Given** la clave configurada es `FOO` y la rama evaluada es `agent/hermes/TAL-7`,
**When** la validación corre,
**Then** MUST retornar `ErrNoJiraKey`.

---

## REQ-WTKEY — Validación de clave Jira para worktrees parametrizada

### REQ-WTKEY-1: Clave de worktree usa la clave de proyecto configurada

**Given** la clave de proyecto está configurada como `<KEY>`,
**When** una clave Jira de worktree es validada (p.ej. `TAL-7`, `FOO-7`),
**Then** MUST aceptar únicamente claves que sigan el patrón `<KEY>-<n>` con `n >= 1`.
Una clave con una clave de proyecto distinta a la configurada MUST ser rechazada con `ErrNoJiraKey`.

### REQ-WTKEY-2: Clave válida bajo clave configurada

**Given** la clave configurada es `TAL` y la clave evaluada es `TAL-1`,
**When** la validación corre,
**Then** MUST aceptarla sin error.

### REQ-WTKEY-3: Clave con `n=0` siempre inválida

**Given** cualquier clave configurada y la clave evaluada es `TAL-0` (o `FOO-0`),
**When** la validación corre,
**Then** MUST rechazarla con `ErrNoJiraKey`.
`n` MUST ser `>= 1`.

### REQ-WTKEY-4: Clave de proyecto incorrecto rechazada

**Given** la clave configurada es `FOO` y la clave evaluada es `TAL-5`,
**When** la validación corre,
**Then** MUST retornar `ErrNoJiraKey`.

---

## REQ-AUTH — Autenticación (enmienda)

> Esta sección **reemplaza** el requisito `REQ-AUTH-1` del spec canónico
> `openspec/specs/jira-loop/functional.md`. El resto de REQ-AUTH (REQ-AUTH-2, REQ-AUTH-3) permanece
> sin cambios.

### REQ-AUTH-1: Fuente de credenciales (enmendado)

**Given** el proceso ambiente al inicializar cualquier CLI que toca Jira,
**When** la CLI resuelve las credenciales de autenticación,
**Then** los **secrets** (`JIRA_EMAIL`, `JIRA_API_TOKEN`) MUST provenir exclusivamente del entorno
del proceso — ya sea heredado (p.ej. CI) o cargado desde el archivo **gitignoreado** `.env` por el
loader al arranque.

Los secrets MUST NOT:
- Aparecer en flags de línea de comandos.
- Quedar registrados en logs, stdout, stderr, o trazas.
- Existir en ningún archivo committeado al repositorio (incluido `.talos/project.env`).

La identidad no-secreta del proyecto (`JIRA_SITE_URL`, `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`) MAY
residir en el archivo committeado `.talos/project.env`, dado que no son credenciales.

**Nota de enmienda:** el enunciado anterior decía "must never appear in config files". Este requisito
lo refina: la prohibición aplica a **archivos committeados**; un archivo gitignoreado cargado por el
loader al arranque es el mecanismo previsto para secrets locales.

---

## REQ-WT — Resolución de credenciales en worktrees

### REQ-WT-1: Identidad resuelta en worktree vía archivo tracked

**Given** una CLI corre dentro de un git worktree,
**When** el loader busca `.talos/project.env`,
**Then** MUST encontrarlo (porque está tracked en git) y resolver `JIRA_PROJECT_KEY`, `JIRA_PROJECT_ID`
y `JIRA_SITE_URL` desde él.
La identidad del proyecto MUST estar disponible en cualquier worktree sin acción adicional del usuario.

### REQ-WT-2: Secrets en worktree vía entorno heredado

**Given** una CLI corre dentro de un git worktree y el proceso padre exportó `JIRA_EMAIL` y
`JIRA_API_TOKEN` antes de spawnear el proceso hijo,
**When** la CLI inicializa,
**Then** MUST leer esos secrets del entorno heredado (REQ-ENV-2 garantiza que no serán pisados).
No se requiere que `.env` exista dentro del worktree.

### REQ-WT-3: Fallback a `.env` del checkout principal (best-effort)

**Given** una CLI corre dentro de un git worktree, los secrets NO están en el entorno heredado, y
existe un `.env` en el checkout principal (directorio raíz del repositorio compartido, no del
worktree),
**When** el loader intenta resolver las credenciales,
**Then** SHOULD cargar ese `.env` principal como fallback best-effort.
Si el `.env` principal tampoco existe, el loader MUST saltearlo silenciosamente (REQ-ENV-5).

### REQ-WT-4: Sin creds + sin `.env` principal — validación existente aplica

**Given** una CLI corre dentro de un worktree, los secrets no están en el entorno heredado, y no
existe `.env` en el checkout principal,
**When** la CLI corre,
**Then** MUST aplicar la validación propia de cada CLI (no del loader):
- `evidence` y `ov check` MUST fallar con el error de credenciales faltantes existente.
- `ch labels` sin `--json` (modo no-Jira) MUST continuar sin error.

Este comportamiento es idéntico al estado anterior a este change en ausencia de secrets.

### REQ-WT-5: `.env` generado por worktree no contiene secrets Jira

**Given** el worktree-orchestrator genera un `.env` por-worktree (para `PORT` y `DB_SCHEMA`),
**When** ese archivo es inspeccionado,
**Then** MUST NOT contener `JIRA_EMAIL`, `JIRA_API_TOKEN`, ni ningún otro secret de Jira.
Este invariante preexistente MUST ser preservado por este change.

---

## REQ-COMPAT — Back-compatibilidad y no-invasividad

### REQ-COMPAT-1: Sin `.talos/project.env` y sin override → comportamiento idéntico al actual

**Given** no existe `.talos/project.env`, no hay override de `JIRA_PROJECT_KEY` en el entorno, y
las credenciales están en el entorno del proceso (el caso CI),
**When** cualquiera de las CLIs Jira corre,
**Then** su comportamiento observable MUST ser **idéntico** al estado anterior a este change:
clave de proyecto `TAL`, ID `10099`, sin cambios en el flujo de autenticación ni de validación de ramas.

### REQ-COMPAT-2: El entorno real de CI siempre gana

**Given** el runner de CI tiene `JIRA_EMAIL`, `JIRA_API_TOKEN`, y cualquier otra variable Jira
seteada como environment secrets,
**When** la CLI corre en CI (con o sin archivos locales),
**Then** esas variables MUST NOT ser sobreescritas por ningún archivo de configuración (REQ-ENV-2).
El job CI existente MUST continuar verde sin modificaciones en el pipeline ni en los archivos de
configuración de CI.

### REQ-COMPAT-3: El invariante del `.env` por-worktree se preserva

**Given** el worktree-orchestrator genera un `.env` por-worktree limitado a variables de
infraestructura (`PORT`, `DB_SCHEMA`),
**When** este change es aplicado,
**Then** ese comportamiento MUST continuar sin modificación.
La separación entre "env de infraestructura por-worktree" y "env de Jira global" MUST mantenerse.

---

## REQ-ERR — Errores tipados

### REQ-ERR-1: Catálogo de errores de este change

**Given** una operación falla por una razón conocida de las introducidas por este change,
**When** la CLI retorna el error,
**Then** MUST usar el identificador de error correspondiente de este catálogo:

| Identificador | Condición de disparo |
|---|---|
| `ErrNoJiraKey` | Rama o clave Jira no sigue el patrón `<projectKey>-<n>` configurado; ya existía, ahora su mensaje MUST ser actualizable para reflejar la clave configurada |

No se introducen nuevos tipos de error para el loader: la ausencia de archivo es silenciosa
(REQ-ENV-5); un error de lectura no-"not-exist" se surfacea como error nativo de Go (con el path
incluido), sin wrapper específico.

### REQ-ERR-2: Mensaje de `ErrNoJiraKey` refleja la clave configurada

**Given** la CLI rechaza una rama o clave con `ErrNoJiraKey` bajo la clave de proyecto `FOO`,
**When** el error es formateado para el usuario,
**Then** MUST mencionar la clave esperada (`FOO`) para que el mensaje sea accionable.
Un mensaje genérico como "invalid key" sin mencionar la clave configurada NO cumple este requisito.

---

## REQ-TEST — Testeabilidad

### REQ-TEST-1: Tests de `Parse` — función pura, sin I/O

**Given** el parser (REQ-PARSE-*),
**When** los unit tests corren,
**Then** MUST existir tests table-driven cubriendo: línea en blanco ignorada; línea `#` ignorada;
`K=a=b=c` → `{K: "a=b=c"}`; trim de espacios; línea sin `=` ignorada; CRLF tolerado; archivo vacío.
Estos tests MUST NOT tocar el sistema de archivos ni el entorno del proceso.

### REQ-TEST-2: Tests de `LoadInto` — setenv/getenv inyectables

**Given** el loader (REQ-ENV-*),
**When** los unit tests corren,
**Then** MUST existir tests con `setenv`/`getenv` implementados como maps inyectados que verifiquen:
clave ya-seteada NO se pisa (REQ-ENV-2); clave no-seteada se setea (REQ-ENV-3); `.env` gana sobre
`project.env` para la misma clave (REQ-ENV-4); archivo ausente saltea sin error (REQ-ENV-5);
error de lectura no-"not-exist" burbujea (REQ-ENV-6).
Estos tests MUST NOT tocar el entorno real del proceso.

### REQ-TEST-3: Tests de validación de rama parametrizada

**Given** `ParseAgentBranch` con `projectKey` (REQ-BRANCH-*),
**When** los unit tests corren,
**Then** MUST existir tests table-driven con columna `projectKey` y filas cubriendo:
`TAL`/`agent/hermes/TAL-7` → válida; `TAL`/`agent/hermes/FOO-7` → `ErrNoJiraKey`;
`FOO`/`agent/hermes/FOO-7` → válida; `FOO`/`agent/hermes/TAL-7` → `ErrNoJiraKey`.

### REQ-TEST-4: Tests de validación de clave Jira parametrizada

**Given** `ValidateJiraKey` con `projectKey` (REQ-WTKEY-*),
**When** los unit tests corren,
**Then** MUST existir tests table-driven con filas cubriendo: `TAL`/`TAL-1` → válida;
`TAL`/`TAL-0` → `ErrNoJiraKey`; `FOO`/`FOO-7` → válida; `FOO`/`TAL-5` → `ErrNoJiraKey`.

### REQ-TEST-5: Back-compat — sin archivos, con creds en env → comportamiento invariante

**Given** los tests del composition root de cada CLI (REQ-COMPAT-1),
**When** los tests del `run()` existentes corren tras este change,
**Then** MUST seguir verdes sin modificación de los casos de prueba existentes que no involucran
los nuevos archivos de configuración.

---

## REQ-OUT-OF-SCOPE — Deferrals explícitos

Los siguientes ítems NO son requisitos de este change y MUST NOT ser implementados:

| Ítem | Deferido a |
|---|---|
| `IssueTypeName:"Tarea"` + `OrderedStates` en español | Follow-up (extensión de `project.env` con `JIRA_ISSUE_TYPE` + archivo de transiciones, o runtime) |
| Roster de agentes hardcodeado (`assignMap`, `portBase`) | Non-goal (modelo de agentes, project-defining) |
| Extracción real del Kit a repo/módulo Go propio | Fase 5 |
| Lint automático de sync `project.env` ↔ `config.yaml:jira` | Follow-up |
| Renombrar `DefaultTALConfig` | R1 — 9 call sites, no renombrar |

---

## Trazabilidad de requisitos

| REQ-ID | Fuente | Decisión / riesgo |
|---|---|---|
| REQ-ENV-1 a 7 | Propuesta §"What changes", D3, D4, plan §A | D3 (hoist), D4 (no fail-fast) |
| REQ-PARSE-1 a 7 | Propuesta §"What changes", plan §A | D2 (KEY=VALUE) |
| REQ-IDENT-1 a 4 | Propuesta §"What changes", D1, D5, D9 | D5 (no rename), D9 (nombre archivo) |
| REQ-BRANCH-1 a 5 | Propuesta §"What changes", D1, D6 | D6 (QuoteMeta, dominio puro) |
| REQ-WTKEY-1 a 4 | Propuesta §"What changes", D6 | D6 (QuoteMeta) |
| REQ-AUTH-1 (enmienda) | Propuesta §"What changes", plan §D | Enmienda spec jira-loop/functional.md |
| REQ-WT-1 a 5 | Propuesta §"Scope", D7, plan §E | D7 (env heredado + fallback) |
| REQ-COMPAT-1 a 3 | Propuesta §"Impact", §"Risks" R2 | R2 (back-compat additive) |
| REQ-ERR-1 a 2 | Propuesta §"Resolved decisions" | ErrNoJiraKey preexistente |
| REQ-TEST-1 a 5 | CONSTITUTION; strict TDD; plan §"Strict TDD" | L1/L2/L3/L4 |
| REQ-OUT-OF-SCOPE | Propuesta §"Out of scope" | R1, R5/R6 |

---

## Design-Time Decisions (abiertas — MUST resolverse antes del apply)

| ID | Pregunta | Constraint del spec |
|----|----------|---------------------|
| DC-1 | ¿Cómo se resuelve el `repoRoot()` en `evidence`? | REQ-ENV-1 requiere que el loader corra antes de la evaluación de flags; el path al `.env` depende de la raíz del repo. El diseño debe especificar cómo se obtiene esa raíz (ya existe en `ch`, ~7 LOC). |
| DC-2 | ¿Cómo se normaliza el path de `--git-common-dir` en worktrees? | REQ-WT-3 requiere acceder al `.env` principal; el path puede ser relativo o absoluto. El diseño debe documentar la normalización y el comportamiento en repos bare (path inexistente → silencioso). |
| DC-3 | ¿Dónde se ubica exactamente el hoist de `LoadInto` en cada CLI? | REQ-ENV-1 requiere que sea antes de cualquier evaluación de flag. El diseño debe identificar el punto exacto en `run()` para cada uno de los tres binarios (R8). |
