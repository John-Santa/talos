# Spec: ci-hardening — Contrato de Comportamiento Observable

**Change:** ci-hardening
**Jira:** TAL-5
**Module:** module:devops
**Owner:** HERMES
**Branch:** `agent/hermes/TAL-5`
**RFC 2119 keywords apply throughout.**

---

## Overview

Este spec describe el comportamiento observable que DEBE ser verdadero una vez aplicado el change
`ci-hardening`. No prescribe implementación ni estructura de paquetes; eso es responsabilidad de la
fase de diseño.

El sistema bajo especificación es:

1. **`ch`** — un CLI Go que reside en `platform/ci-checks/`. Tiene dos subcomandos: `ch labels` y
   `ch ownership`. Es el motor del invariante §4 (CONSTITUTION.md): exactamente un `agent:*` y un
   `module:*` por issue Jira, cuyo par cumple `ownership[module] == agent` y cuya figura de rama
   coincide con el label `agent:*`.

2. **Los tres gates de CI** — jobs en `.github/workflows/pr-checks.yml` que bloquean un PR a
   `develop` si cualquier gate falla (exit ≠ 0). Los gates son: `branch-name`, `labels`, `dod`.

El spec especifica el COMPORTAMIENTO OBSERVABLE (qué hace, con qué entradas y salidas). El cómo lo
hace es dominio del diseño.

---

## REQ-BRANCH — Parseo de rama

### REQ-BRANCH-1: Formato canónico de rama agente
**Given** una cadena de rama con formato `agent/<figura>/<JIRA-KEY>`,
**When** `ch` parsea esa cadena,
**Then** MUST extraer exactamente dos valores: la figura (en lowercase) y la JIRA-KEY (en uppercase).
El parseo es case-insensitive en la figura: `agent/Hermes/TAL-7` y `agent/hermes/TAL-7` son
equivalentes y MUST producir figura `hermes`, key `TAL-7`.

**Scenario — OK, figura minúscula:**
- Given: rama `agent/hermes/TAL-7`
- When: se parsea
- Then: figura=`hermes`, key=`TAL-7`, sin error.

**Scenario — OK, figura mayúscula normalizada:**
- Given: rama `agent/Hermes/TAL-7`
- When: se parsea
- Then: figura=`hermes`, key=`TAL-7`, sin error.

### REQ-BRANCH-2: Ramas sin JIRA-KEY → ErrNoJiraKey
**Given** una cadena de rama que no sigue el formato `agent/<figura>/<JIRA-KEY>` (porque es
`develop`, una feature sin key, o cualquier otra forma no-agente),
**When** `ch` intenta parsearla,
**Then** MUST retornar `ErrNoJiraKey` y MUST NOT intentar contactar Jira ni leer ownership.
Exit code MUST ser 1.

**Scenario — rama develop:**
- Given: rama `develop`
- When: se parsea
- Then: `ErrNoJiraKey`, exit 1.

**Scenario — rama agent sin key:**
- Given: rama `agent/hermes/feature`
- When: se parsea
- Then: `ErrNoJiraKey`, exit 1.

**Scenario — rama arbitraria:**
- Given: rama `fix/typo`
- When: se parsea
- Then: `ErrNoJiraKey`, exit 1.

---

## REQ-INVARIANT — Invariante de label §4

El invariante §4 tiene 4 sub-reglas (a)(b)(c)(d), cada una independientemente testeable.

### REQ-INVARIANT-1 (sub-regla a): Exactamente un label `agent:*`
**Given** el conjunto de labels del issue Jira,
**When** se evalúa el invariante,
**Then** MUST existir exactamente UN label cuya clave sea `agent`.
Cero labels `agent:*` o dos o más labels `agent:*` son AMBAS violaciones distintas.

**Scenario — OK:**
- Given: labels `["agent:hermes", "module:devops"]`
- When: sub-regla (a) evalúa
- Then: sin violación.

**Scenario — cero labels agent:**
- Given: labels `["module:devops"]`
- When: sub-regla (a) evalúa
- Then: violación; mensaje identifica "0 labels agent:*".

**Scenario — dos labels agent:**
- Given: labels `["agent:hermes", "agent:atlas", "module:devops"]`
- When: sub-regla (a) evalúa
- Then: violación; mensaje identifica "2 labels agent:*".

### REQ-INVARIANT-2 (sub-regla b): Exactamente un label `module:*`
**Given** el conjunto de labels del issue Jira,
**When** se evalúa el invariante,
**Then** MUST existir exactamente UN label cuya clave sea `module`.
Cero labels `module:*` o dos o más labels `module:*` son AMBAS violaciones distintas.

**Scenario — OK:**
- Given: labels `["agent:hermes", "module:devops"]`
- When: sub-regla (b) evalúa
- Then: sin violación.

**Scenario — cero labels module:**
- Given: labels `["agent:hermes"]`
- When: sub-regla (b) evalúa
- Then: violación; mensaje identifica "0 labels module:*".

**Scenario — dos labels module:**
- Given: labels `["agent:hermes", "module:devops", "module:qa"]`
- When: sub-regla (b) evalúa
- Then: violación; mensaje identifica "2 labels module:*".

### REQ-INVARIANT-3 (sub-regla c): El par `ownership[module] == agent`
**Given** el label `agent:<figura>` y el label `module:<módulo>` extraídos del issue,
**and** la tabla de ownership parseada de `team-context/ownership.md`,
**When** se evalúa el invariante,
**Then** MUST verificar que `ownership[<módulo>] == <figura>` (comparación case-insensitive
después de normalización a lowercase).
Si `<módulo>` no existe en la tabla, es también una violación de esta sub-regla.

**Scenario — OK:**
- Given: labels `["agent:hermes", "module:devops"]`, ownership: `devops→hermes`
- When: sub-regla (c) evalúa
- Then: sin violación.

**Scenario — mismatch agent/module:**
- Given: labels `["agent:atlas", "module:devops"]`, ownership: `devops→hermes`
- When: sub-regla (c) evalúa
- Then: violación; mensaje identifica el par obtenido y el par esperado.

**Scenario — módulo desconocido:**
- Given: labels `["agent:hermes", "module:unknown"]`, ownership: `devops→hermes`
- When: sub-regla (c) evalúa
- Then: violación; mensaje identifica que `unknown` no existe en la tabla de ownership.

### REQ-INVARIANT-4 (sub-regla d): `branchFigura == labelAgent`
**Given** la figura extraída de la rama (`agent/<figura>/TAL-N`) y el label `agent:<figura-label>`,
**When** se evalúa el invariante,
**Then** MUST verificar que `branchFigura == labelAgent` (comparación en lowercase, post-normalización).
Esta sub-regla realiza §5 de la CONSTITUTION: la rama corrobora la identidad del label.

**Scenario — OK:**
- Given: rama `agent/hermes/TAL-7`, label `agent:hermes`
- When: sub-regla (d) evalúa
- Then: sin violación.

**Scenario — figura de rama ≠ figura del label:**
- Given: rama `agent/hermes/TAL-7`, label `agent:atlas`
- When: sub-regla (d) evalúa
- Then: violación; mensaje identifica figura de rama (`hermes`) y figura del label (`atlas`).

### REQ-INVARIANT-5: Múltiples violaciones simultáneas
**Given** un conjunto de labels que viola más de una sub-regla al mismo tiempo,
**When** se evalúa el invariante completo,
**Then** MUST reportar TODAS las violaciones ocurridas, no solo la primera.
El resultado MUST incluir cada violación como un ítem separado.

**Scenario — todas las sub-reglas rotas:**
- Given: labels `[]`, rama `agent/hermes/TAL-7`
- When: se evalúa el invariante completo
- Then: al menos violación (a) "0 agent:*" y violación (b) "0 module:*" presentes en el resultado.

### REQ-INVARIANT-6: Invariante OK implica exit 0
**Given** labels que satisfacen las 4 sub-reglas (a)(b)(c)(d),
**When** `ch labels` corre sobre un issue cuya rama es `agent/<figura>/TAL-N`,
**Then** MUST imprimir un mensaje de éxito (o JSON con `verdict:"OK"`) y exit code MUST ser 0.

---

## REQ-LABELS — Subcomando `ch labels`

### REQ-LABELS-1: Comportamiento completo end-to-end
**Given** una invocación `ch labels --branch <rama>`,
**When** la rama sigue el formato canónico `agent/<figura>/TAL-N`,
**Then** `ch` MUST ejecutar los siguientes pasos en orden:
1. Parsear la rama extrayendo figura y JIRA-KEY.
2. Fetchear los labels del issue Jira identificado por la JIRA-KEY.
3. Leer la tabla de ownership desde el archivo de ownership (default:
   `team-context/ownership.md` relativo al root del repo).
4. Evaluar el invariante completo (sub-reglas a, b, c, d).
5. Emitir el resultado (texto legible o JSON si `--json`) y salir con exit 0 si OK, exit 1 si hay
   alguna violación.

### REQ-LABELS-2: ErrNoJiraKey corta antes de llamar a Jira
**Given** una rama que produce `ErrNoJiraKey`,
**When** `ch labels` parsea la rama,
**Then** MUST salir con exit 1 y el mensaje de error sin haber contactado Jira ni leído ownership.

**Scenario:**
- Given: `ch labels --branch develop`
- When: corre
- Then: exit 1, mensaje `ErrNoJiraKey`, sin red, sin lectura de archivo.

### REQ-LABELS-3: Flag `--ownership-file` overridea el path por defecto
**Given** `ch labels --branch <rama> --ownership-file <path>`,
**When** `ch` lee la tabla de ownership,
**Then** MUST leer el archivo ubicado en `<path>` en vez del default.
Si el archivo no existe en `<path>`, MUST retornar error y exit 1.

### REQ-LABELS-4: Flag `--site-url` overridea la URL de Jira
**Given** `ch labels --branch <rama> --site-url <url>`,
**When** `ch` fetchea labels de Jira,
**Then** MUST usar `<url>` como base de la URL en vez de la variable de entorno `JIRA_SITE_URL`.
Si `--site-url` no está presente y `JIRA_SITE_URL` no está seteada, MUST retornar error y exit 1.

---

## REQ-OWNERSHIP — Subcomando `ch ownership`

### REQ-OWNERSHIP-1: Validación offline de ownership.md
**Given** una invocación `ch ownership`,
**When** corre,
**Then** MUST leer y parsear la tabla de ownership (default `team-context/ownership.md`) sin
contactar Jira. El resultado MUST ser:
- exit 0 si el archivo parsea correctamente (tabla bien formada, sin duplicados, todos los
  owners son figuras conocidas).
- exit 1 con `ErrMalformedOwnership` en cualquier caso de malformación.

### REQ-OWNERSHIP-2: No hay duplicados de módulo
**Given** una tabla de ownership con dos filas con el mismo módulo,
**When** `ch ownership` parsea el archivo,
**Then** MUST retornar `ErrMalformedOwnership` con un mensaje que identifica el módulo duplicado.
Exit code MUST ser 1.

**Scenario:**
- Given: ownership.md con dos filas `devops→hermes`
- When: `ch ownership` corre
- Then: exit 1, `ErrMalformedOwnership`, mensaje nombra `devops` como duplicado.

### REQ-OWNERSHIP-3: Output legible por defecto, JSON con `--json`
**Given** `ch ownership` corriendo sin `--json`,
**When** la tabla es válida,
**Then** MUST imprimir la tabla como texto legible (módulo → agente, uno por línea o equivalente)
y exit 0.

**Given** `ch ownership --json`,
**When** la tabla es válida,
**Then** MUST imprimir un objeto JSON con al menos los pares `{módulo: agente, ...}` y exit 0.

---

## REQ-PARSE — Parseo de la tabla de ownership

### REQ-PARSE-1: Parseo de la tabla "Mapa módulo → agente"
**Given** el contenido de `team-context/ownership.md`,
**When** `ch` parsea la tabla de ownership,
**Then** MUST procesar SOLO la primera tabla cuyas filas contengan el prefijo `module:` en la
primera columna. La segunda tabla (mapa archivo→módulo) MUST ser ignorada (sus filas no tienen
el prefijo `module:` en la primera columna).

### REQ-PARSE-2: Normalización a lowercase
**Given** la tabla tiene agentes en UPPERCASE (e.g. `HERMES`, `ATLAS`),
**When** `ch` parsea cada fila,
**Then** MUST normalizar TANTO la clave (módulo, sin prefijo `module:`) COMO el valor (agente)
a lowercase. El resultado MUST ser un mapa cuyas claves y valores son todos lowercase.

**Scenario:**
- Given: fila `| \`module:devops\` | HERMES | ... |`
- When: parseada
- Then: entrada en el mapa `devops → hermes`.

**Scenario — casing mezclado:**
- Given: fila `| \`module:Qa\` | Themis | ... |`
- When: parseada
- Then: entrada en el mapa `qa → themis`.

### REQ-PARSE-3: Filas duplicadas → ErrMalformedOwnership
**Given** dos filas con el mismo módulo (post-normalización),
**When** `ch` parsea la tabla,
**Then** MUST retornar `ErrMalformedOwnership` identificando el módulo duplicado.
MUST NOT retornar un mapa parcial ni silenciar el duplicado.

### REQ-PARSE-4: Filas de encabezado y separadores son ignoradas
**Given** la tabla tiene una fila de encabezado (`| \`module:*\` | Dueño... |`) y filas separadoras
(`|---|---|...|`),
**When** `ch` parsea la tabla,
**Then** esas filas MUST ser ignoradas (no producen entradas en el mapa ni errores).
La fila de encabezado con valor literal `module:*` MUST ser identificada como no-válida y
descartada silenciosamente.

---

## REQ-JIRA — Adapter de Jira (comportamiento observable)

### REQ-JIRA-1: Fetch de labels por key
**Given** una JIRA-KEY válida (e.g. `TAL-7`),
**When** `ch` consulta Jira,
**Then** MUST realizar exactamente UNA petición HTTP GET al endpoint de issue por key con el
campo `labels` solicitado. La respuesta MUST ser decodificada para extraer el array de labels.
Credentials MUST provenir de las variables de entorno `JIRA_EMAIL`, `JIRA_API_TOKEN`, y
`JIRA_SITE_URL` (o el override `--site-url`).

### REQ-JIRA-2: Issue no encontrado (404) → ErrIssueNotFound
**Given** Jira retorna HTTP 404 para la JIRA-KEY solicitada,
**When** `ch` recibe la respuesta,
**Then** MUST retornar `ErrIssueNotFound` con la key afectada en el mensaje.
MUST NOT retornar un conjunto de labels vacío ni continuar con el check.
Exit code MUST ser 1.

**Scenario:**
- Given: `ch labels --branch agent/hermes/TAL-999` y TAL-999 no existe en Jira
- When: corre
- Then: exit 1, mensaje `ErrIssueNotFound` nombra `TAL-999`.

### REQ-JIRA-3: Error de autenticación (401) → HTTPError → exit 1
**Given** Jira retorna HTTP 401,
**When** `ch` recibe la respuesta,
**Then** MUST retornar un `HTTPError` con status code 401 y el body de la respuesta como contexto.
Exit code MUST ser 1.

### REQ-JIRA-4: Error de servidor (5xx) → HTTPError → exit 1
**Given** Jira retorna cualquier status 5xx,
**When** `ch` recibe la respuesta,
**Then** MUST retornar un `HTTPError` con el status code y body de respuesta. Fail-closed:
exit code MUST ser 1. MUST NOT reintentar automáticamente.

---

## REQ-JSON — Output `--json`

### REQ-JSON-1: Shape del output JSON de `ch labels --json`
**Given** `ch labels --branch <rama> --json` corre con cualquier resultado (OK o violación),
**When** `ch` produce output,
**Then** MUST emitir un objeto JSON válido con exactamente los siguientes campos:

```
{
  "branch":     string,   // rama original tal como se pasó
  "jira_key":   string,   // JIRA-KEY extraída de la rama
  "figura":     string,   // figura extraída (lowercase)
  "verdict":    string,   // "OK" | "VIOLATION"
  "labels":     []string, // labels del issue Jira (vacío si no se llegó a fetchear)
  "agent":      string,   // valor del label agent:* (vacío si no hay exactamente uno)
  "module":     string,   // valor del label module:* (vacío si no hay exactamente uno)
  "violations": []string  // lista de mensajes de violación; vacío si verdict=="OK"
}
```

El output MUST ser válido JSON parseable por `jq` u otras herramientas estándar.
MUST NOT incluir campos adicionales no documentados en esta spec (para estabilidad del contrato).

### REQ-JSON-2: `--json` en error de parseo de rama
**Given** `ch labels --branch develop --json`,
**When** corre,
**Then** MUST emitir un objeto JSON con `"verdict":"VIOLATION"` y la violación `ErrNoJiraKey`
en `"violations"`. MUST NOT emitir texto plano mixto con JSON.

---

## REQ-EXIT — Contrato de exit codes

### REQ-EXIT-1: Tabla de exit codes
**Given** cualquier invocación de `ch`,
**When** termina,
**Then** MUST usar exclusivamente los siguientes exit codes:

| Exit code | Condición |
|-----------|-----------|
| `0` | Invariante satisfecho / ownership bien formado / parseo OK |
| `1` | Violación del invariante; `ErrNoJiraKey`; `ErrIssueNotFound`; `HTTPError` (401/5xx); `ErrMalformedOwnership`; cualquier error no anticipado |

MUST NOT existir ningún código de "skip" o "omitir". El gate es binario: OK o falla.

### REQ-EXIT-2: Errores inesperados también son exit 1
**Given** cualquier condición de error no contemplada en el catálogo (e.g. IO inesperado,
argumento faltante),
**When** `ch` la encuentra,
**Then** MUST imprimir un mensaje de error legible a stderr y salir con exit 1.
MUST NOT hacer panic ni salir con exit code distinto de 0 o 1.

---

## REQ-GATES — Gates de CI (comportamiento observable)

Los gates son jobs en `.github/workflows/pr-checks.yml` que se disparan en eventos
`pull_request` hacia la rama `develop`. El comportamiento especificado aquí es observable desde
el estado de PR en GitHub (check verde / rojo).

### REQ-GATES-1: Gate `branch-name` — validación de formato de rama
**Given** un PR abierto hacia `develop` cuyo branch name NO sigue el formato
`^agent/[a-z]+/TAL-[0-9]+$` (CONSTITUTION §3),
**When** el gate `branch-name` corre,
**Then** el check MUST marcarse como fallido (GitHub Actions step con exit ≠ 0) y el PR MUST
quedar bloqueado hasta que el branch sea renombrado.

**Given** un PR con branch `agent/hermes/TAL-5`,
**When** el gate `branch-name` corre,
**Then** el check MUST pasar (exit 0).

### REQ-GATES-2: Gate `labels` — invariante §4
**Given** un PR cuyo branch sigue el formato canónico,
**When** el gate `labels` corre (después de que `branch-name` pasó, dado el `needs`),
**Then** MUST invocar `ch labels --branch <GITHUB_HEAD_REF> --json` con las credenciales Jira
inyectadas vía secrets. Si `ch` retorna exit 1, el check MUST marcarse como fallido y el PR
MUST quedar bloqueado.

**Scenario — issue sin labels:**
- Given: TAL-N no tiene labels
- When: gate `labels` corre
- Then: `ch labels` retorna exit 1, check rojo, PR bloqueado.

**Scenario — labels correctos:**
- Given: TAL-N tiene `agent:hermes`, `module:devops` y ownership confirma el par
- When: gate `labels` corre
- Then: `ch labels` retorna exit 0, check verde.

### REQ-GATES-3: Gate `dod` — tests y verify-report
**Given** un PR cuyo branch sigue el formato canónico,
**When** el gate `dod` corre (con `fetch-depth:0`, después de que `branch-name` pasó),
**Then** MUST realizar los siguientes pasos observables:
1. Detectar los módulos Go cambiados bajo `platform/` comparando contra el `base_ref` con
   diff de tres puntos (`origin/<base_ref>...HEAD`).
2. Para cada módulo Go con `go.mod` existente, ejecutar `go test -short ./...`. Si algún test
   falla (exit ≠ 0), el gate MUST marcarse como fallido.
3. Verificar que existe al menos un archivo `verify-report.md` bajo `openspec/changes/`. Si
   no existe ninguno, el gate MUST marcarse como fallido.
4. El paso de vitest DEBE estar presente como placeholder documentado (no bloquea en ausencia
   de frontend).

**Scenario — tests verdes + verify-report presente:**
- Given: `platform/ci-checks/` cambiado, tests pasan, `openspec/changes/.../verify-report.md` existe
- When: gate `dod` corre
- Then: check verde.

**Scenario — test roto:**
- Given: algún test falla bajo el módulo cambiado
- When: gate `dod` corre
- Then: check rojo, PR bloqueado.

**Scenario — verify-report ausente:**
- Given: no existe ningún `verify-report.md` bajo `openspec/changes/`
- When: gate `dod` corre
- Then: check rojo, PR bloqueado.

### REQ-GATES-4: Dependencia entre gates (`needs`)
**Given** el gate `labels` y el gate `dod` están configurados con `needs: branch-name`,
**When** el gate `branch-name` falla,
**Then** los gates `labels` y `dod` MUST NO correr (quedan en estado "skipped" o no iniciados).
Esto garantiza que `ch labels` recibe siempre una rama bien formada.

### REQ-GATES-5: Los gates corren en PRs a `develop` solamente
**Given** el workflow está configurado en `on: pull_request: branches:[develop]`,
**When** un push ocurre a cualquier rama distinta de `develop` (e.g. entre feature branches),
**Then** el workflow MUST NOT dispararse.

---

## REQ-ERR — Catálogo de errores tipados

### REQ-ERR-1: Catálogo normativo
**Given** cualquier operación que falla por una razón conocida,
**When** `ch` retorna el error,
**Then** MUST usar un error del siguiente catálogo (los identificadores son contratos de
comportamiento observable, no nombres de implementación):

| Identificador | Condición que lo dispara |
|---|---|
| `ErrNoJiraKey` | La rama no sigue el formato `agent/<figura>/TAL-N` — no hay JIRA-KEY extraíble |
| `ErrLabelInvariant` | Al menos una sub-regla (a)(b)(c)(d) del invariante §4 fue violada |
| `ErrMalformedOwnership` | El archivo de ownership no parsea correctamente (duplicado, formato inválido) |
| `ErrIssueNotFound` | Jira retornó HTTP 404 para la JIRA-KEY solicitada |
| `HTTPError` | Jira retornó cualquier status no-2xx distinto de 404 (incluye 401, 5xx) |

### REQ-ERR-2: No panic
**Given** cualquier condición de error, incluyendo inputs inesperados o respuestas Jira
malformadas,
**When** `ch` la encuentra,
**Then** MUST NOT hacer panic. MUST imprimir el error a stderr y salir con exit 1.

### REQ-ERR-3: Mensajes de error no-vacíos
**Given** cualquier error del catálogo,
**When** `ch` lo reporta,
**Then** el mensaje MUST nombrar el contexto relevante: para `ErrIssueNotFound` MUST nombrar
la key; para `ErrLabelInvariant` MUST listar cada violación; para `ErrMalformedOwnership` MUST
nombrar el campo o fila problemática.

---

## REQ-TEST — Testeabilidad (Strict TDD)

### REQ-TEST-1: Parseo de rama — tabla de casos
**Given** REQ-BRANCH-1 y REQ-BRANCH-2,
**When** se escriben tests,
**Then** MUST existir tests table-driven cubriendo: rama canónica lowercase, rama canónica
uppercase normalizada, rama `develop`, rama `agent/hermes/feature` (sin key), rama vacía.
Estos tests MUST tener cero dependencia de red, filesystem, ni binario Jira.

### REQ-TEST-2: Sub-reglas del invariante — tabla de casos
**Given** REQ-INVARIANT-1 a REQ-INVARIANT-5,
**When** se escriben tests,
**Then** MUST existir tests table-driven para cada sub-regla (a)(b)(c)(d) cubriendo al
menos: caso OK, 0 labels de ese tipo, 2 labels de ese tipo (donde aplique),
mismatch, módulo desconocido, figura rama ≠ figura label, y múltiples violaciones simultáneas.
Cada caso MUST ser un test nombrado, no una aserción ad-hoc.

### REQ-TEST-3: Parseo de tabla de ownership — casos de fixture
**Given** REQ-PARSE-1 a REQ-PARSE-4,
**When** se escriben tests,
**Then** MUST existir tests con un archivo fixture que contenga al menos: filas UPPERCASE,
filas de encabezado, separadores, segunda tabla, y una fila duplicada. Los tests MUST verificar:
normalización a lowercase, skip de encabezado y separadores, stop ante segunda tabla,
y `ErrMalformedOwnership` en duplicado.

### REQ-TEST-4: Service layer via port mocks
**Given** el servicio que orquesta parseo de rama + fetch Jira + lectura de ownership + evaluación
del invariante,
**When** se escriben tests del service layer,
**Then** MUST depender solo de interfaces (ports) para las operaciones de red y filesystem.
Los mocks MUST soportar recording de llamadas y MUST verificar:
- `ErrNoJiraKey` detiene la ejecución sin llamar al port de Jira (AssertNotCalled).
- Cada violación del invariante produce `ErrLabelInvariant`.
- `ErrIssueNotFound` y `HTTPError` del port de Jira burbujean correctamente.
- El orden de llamadas: parseo de rama → fetch labels → lectura de ownership → invariante.

### REQ-TEST-5: Adapter de Jira — integración Short-gated
**Given** el adapter de Jira,
**When** se escriben tests que requieren red real o servidor Jira,
**Then** MUST ser skipeados cuando `go test -short` es pasado.
Los tests con `httptest` (servidor HTTP en proceso) MUST correr siempre (no requieren red real).
Los tests `httptest` MUST cubrir: decode correcto con 200, `ErrIssueNotFound` con 404,
`HTTPError` con 401, `HTTPError` con 500, y validación de path y query params.

### REQ-TEST-6: Adapter de ownership — fixture de archivo
**Given** el adapter que lee `team-context/ownership.md`,
**When** se escriben tests,
**Then** MUST existir un archivo `testdata/ownership.md` en el directorio del adapter que
sirva como fixture. Los tests MUST usar `--ownership-file` o su equivalente para apuntar
al fixture; MUST NOT depender de la existencia del archivo real en el repo durante tests.

### REQ-TEST-7: Shape JSON — contrato de output
**Given** REQ-JSON-1,
**When** se escriben tests del output JSON de `ch labels`,
**Then** MUST existir al menos un test que decodifique el output JSON del comando y verifique
que todos los campos documentados en REQ-JSON-1 están presentes con los tipos correctos.
El test MUST cubrir tanto el caso `verdict:"OK"` como `verdict:"VIOLATION"`.

### REQ-TEST-8: Cobertura del catálogo de errores
**Given** REQ-ERR-1,
**When** se escriben tests,
**Then** MUST existir al menos un test por cada identificador del catálogo verificando que
la condición trigger retorna exactamente ese error.

---

## REQ-OUT-OF-SCOPE — Deferrales explícitos

Los siguientes ítems NO son requisitos de este change y MUST NOT ser implementados:

| Ítem | Diferido a |
|---|---|
| Vitest real (tests de frontend) | futuro change con frontend activo |
| Generación automática de `verify-report.md` | el gate solo verifica presencia |
| `--require-ci-green` en `mo` | este change lo provee; `mo` lo consume en futuro change |
| Retry automático de requests Jira | future change |
| Grace period para issue en estado mid-creación | diseño de `ch` confirma: ausencia de labels = violación real |
| `actionlint` como gate duro | advisory; no bloquea CI en MVP |
| Absorber lógica de `dod` dentro de `ch` | `ch` es solo motor de labels; tests corren en YAML/runner |
| Rate-limit wrapper | future change |

---

## REQ-CLEANUP — Archivos compartidos

### REQ-CLEANUP-1: Entrada en `.gitignore` para el binario `ch`
**Given** el change aplicado,
**When** se lee `.gitignore`,
**Then** MUST contener la entrada `/platform/ci-checks/ch` para que el binario compilado
no sea commiteado al repositorio.

### REQ-CLEANUP-2: Fila de ownership para `platform/ci-checks/`
**Given** el change aplicado,
**When** se lee `team-context/ownership.md`,
**Then** la segunda tabla (mapa archivo→módulo) MUST contener una fila que mapee
`platform/ci-checks/**` a `HERMES` identificando al CLI `ch` y el invariante §4.

### REQ-CLEANUP-3: Migración de `ci/` a `.github/workflows/`
**Given** el change aplicado,
**When** se lee el repositorio,
**Then** MUST existir el archivo `.github/workflows/pr-checks.yml` con los 3 jobs especificados.
El directorio `ci/` y su contenido MUST haber sido eliminados.

---

## Trazabilidad de Requisitos

| REQ-ID | Fuente | Ref. Constitución |
|--------|--------|-------------------|
| REQ-BRANCH-1–2 | Proposal §CLI surface, §Decisiones D1 | §3 (branch naming) |
| REQ-INVARIANT-1–6 | Proposal §invariante §4 (a)(b)(c)(d); explore §Q1 | §4 (label invariant) |
| REQ-LABELS-1–4 | Proposal §CLI surface; plan §ch labels | §4, §5 |
| REQ-OWNERSHIP-1–3 | Proposal §CLI surface; plan §ch ownership | §2, §4 |
| REQ-PARSE-1–4 | Explore §Q2 (ParseOwnershipTable contract) | §4 |
| REQ-JIRA-1–4 | Explore §Q1 (GET-by-key confirmed); proposal D3 | — |
| REQ-JSON-1–2 | Proposal §CLI surface (`--json` shape); plan §Strict TDD | — |
| REQ-EXIT-1–2 | Proposal §Exit codes; plan | — |
| REQ-GATES-1–5 | Proposal §Migración YAML; plan §3 jobs | §3, §4, §6 |
| REQ-ERR-1–3 | Proposal §Decisiones; explore §gotchas | — |
| REQ-TEST-1–8 | CONSTITUTION; project TDD convention; plan §Strict TDD | Strict TDD |
| REQ-CLEANUP-1–3 | Proposal §Cleanup in-scope | §2 |
| REQ-OUT-OF-SCOPE | Proposal §Out of scope; plan §Deferrals | — |

---

## Decisiones abiertas para la fase de diseño

| ID | Pregunta | Restricción de spec |
|----|----------|---------------------|
| DC-1 | ¿Cómo se detecta el `repoRoot` para el path por defecto de ownership? | MUST funcionar tanto en local como en CI (runner de GitHub Actions con checkout). |
| DC-2 | ¿Cómo se pasan las credenciales Jira al adapter — via env o argumento explícito? | REQ-JIRA-1 dice que creds MUST provenir de env vars; el diseño decide en qué capa se leen. |
| DC-3 | ¿El mapa de ownership que devuelve `ch ownership --json` incluye el prefijo `module:` en las claves? | REQ-OWNERSHIP-3 no lo prescribe; diseño decide por consistencia con los labels Jira. |
| DC-4 | ¿Qué glob exacto usa el gate `dod` para encontrar `verify-report.md`? | REQ-GATES-3 dice `openspec/changes/**/verify-report.md`; diseño confirma o ajusta. |
