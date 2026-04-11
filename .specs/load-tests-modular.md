# Spec: Load Tests — Arquitetura Modular

## Status: DRAFT

## Context

Os load tests atuais vivem num unico arquivo monolitico `tests/load/scenarios.js` (309 linhas) que combina configuracao, helpers, operacoes HTTP e cenarios num so lugar. Isso causa:

1. **Sem reutilizacao** — helpers (headers, assertions, UUID) sao inline e nao podem ser importados por novos dominios
2. **Assertions frageis** — cada `check()` e manual com `try/catch`, sem tracking automatico de falhas
3. **Smoke test generico** — smoke roda 5 VUs por 30s fazendo CRUD aleatorio em vez de validacao funcional 1-a-1 (1 VU, 1 iteracao)
4. **Sem modularidade** — adicionar um novo dominio requer editar o arquivo inteiro
5. **Sem separacao de concerns** — HTTP client, assertions, config e cenarios misturados

**Solucao:** Modularizar em 3 arquivos: `main.js` (orquestrador de cenarios), `helpers.js` (HTTP client, assertions, UUID, headers), `users.js` (operacoes e smoke tests do dominio user). Novos dominios adicionam seu proprio `<domain>.js` e registram no orchestrador.

**Referencia:** banking-service-yield `tests/load/main.js`, `tests/load/helpers.js`, `tests/load/savings.js`

## Requirements

- [ ] REQ-1: **Helpers reutilizaveis em arquivo separado**
  - GIVEN um novo dominio precisa de operacoes HTTP
  - WHEN importa de `./helpers.js`
  - THEN tem acesso a: `post()`, `get()`, `put()`, `del()`, `baseHeaders()`, `headersWithIdempotency(key)`, `uuid()`, `parseData()`, `parseErrorMessage()`, `assertStatus()`, `assertField()`, `assertErrorContains()`, `assertFieldExists()`, `errorRate`

- [ ] REQ-2: **Assertions com tracking automatico de erro**
  - GIVEN um smoke test usa `assertStatus(res, 201, "label")`
  - WHEN a assertion falha
  - THEN o custom metric `errors` (Rate) e incrementado automaticamente
  - AND o output do k6 mostra o label descritivo

- [ ] REQ-3: **Smoke test funcional (1 VU, 1 iteracao)**
  - GIVEN o cenario `smoke` e selecionado
  - WHEN executado via `k6 run --env SCENARIO=smoke tests/load/main.js`
  - THEN roda 1 VU com 1 iteracao, executando TODAS as validacoes de negocio sequencialmente
  - AND cada validacao e um `group()` nomeado

- [ ] REQ-4: **Dominio user em arquivo separado**
  - GIVEN operacoes de user (CRUD, validacao, errors)
  - WHEN importadas de `./users.js`
  - THEN main.js tem acesso a todas as funcoes de smoke e load do dominio user

- [ ] REQ-5: **Orchestrador main.js gerencia cenarios**
  - GIVEN os cenarios smoke, load, stress, spike existem
  - WHEN `main.js` e executado
  - THEN seleciona o cenario via `__ENV.SCENARIO` e delega para o executor correto
  - AND exporta funcoes nomeadas para cada cenario: `smokeTest`, `loadTest`, `stressTest`, `spikeTest`
  - AND smoke chama funcoes de validacao de cada dominio sequencialmente

- [ ] REQ-6: **Smoke suites cobrindo paths de erro**
  - GIVEN o boilerplate tem endpoints de user e role
  - WHEN o smoke test executa
  - THEN valida: health check, happy path (create, get, list, update, delete), validation errors (email invalido, UUID invalido), not found, duplicate, response format consistency, auth errors

- [ ] REQ-7: **Makefile targets apontam para main.js**
  - GIVEN os targets load-smoke, load-test, load-stress, load-spike existem
  - WHEN executados
  - THEN referenciam `tests/load/main.js` (nao `scenarios.js`)

## Test Plan

### Smoke Tests (k6)

Estes TCs sao validados ao executar `k6 run --env SCENARIO=smoke tests/load/main.js`:

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-6 | happy | Health check /health endpoint | 200 + status "ok" |
| TC-S-02 | REQ-6 | happy | Readiness /ready endpoint | 200 + status "ready" + checks |
| TC-S-03 | REQ-6 | security | Request without service key (if auth enabled) | 401 |
| TC-S-04 | REQ-6 | happy | Create user with valid data | 201 + has id, name, email |
| TC-S-05 | REQ-6 | happy | Get user by ID | 200 + correct data |
| TC-S-06 | REQ-6 | happy | List users with pagination | 200 + has data array |
| TC-S-07 | REQ-6 | happy | Update user | 200 + updated fields |
| TC-S-08 | REQ-6 | happy | Delete user | 200 |
| TC-S-09 | REQ-6 | validation | Create user with invalid email | 400 + JSON error format |
| TC-S-10 | REQ-6 | validation | Get user with invalid UUID | 400 + JSON error format |
| TC-S-11 | REQ-6 | business | Get user not found | 404 + JSON error format |
| TC-S-12 | REQ-6 | business | Create user duplicate email | 409 + JSON error format |
| TC-S-13 | REQ-6 | edge | Error responses use `{"errors":{"message":...}}` format | consistent envelope |
| TC-S-14 | REQ-6 | happy | Create role with valid data | 201 + has id, name |
| TC-S-15 | REQ-6 | happy | List roles | 200 + has data array |
| TC-S-16 | REQ-6 | happy | Delete role | 200 |
| TC-S-17 | REQ-6 | business | Create role duplicate name | 409 + JSON error format |

## Design

### Architecture Decisions

**Estrutura modular:**
```
tests/load/
  main.js       — orquestrador (scenarios, setup/teardown, imports)
  helpers.js    — HTTP client, assertions, headers, UUID, config
  users.js      — operacoes do dominio user (smoke groups + load operations)
  roles.js      — operacoes do dominio role (smoke groups)
```

**Padrao de smoke test:**
- Cada validacao e um `group("NN - Description", () => { ... })` nomeado
- Smoke usa `per-vu-iterations` executor (1 VU, 1 iteracao) para validacao funcional deterministica
- Nota: thresholds de percentil (p95<500ms) nao se aplicam ao smoke (1 iteracao) — o threshold relevante para smoke e `errors` (Rate) que deve ser 0
- `sleep(0.1)` entre grupos para evitar rate limiting

**Padrao de assertions:**
- `assertStatus(res, expected, label)` — verifica HTTP status + incrementa errorRate se falhar
- `assertField(res, field, expected, label)` — verifica campo do response body
- `assertErrorContains(res, substring, label)` — verifica mensagem de erro
- `assertFieldExists(res, field, label)` — verifica que campo existe e nao e vazio

**Load test distribution:**
- 40% reads (list + get)
- 30% creates
- 20% updates (create + update)
- 10% deletes (create + delete)

### Files to Create

- `tests/load/main.js` — orquestrador de cenarios
- `tests/load/helpers.js` — utilitarios compartilhados
- `tests/load/users.js` — operacoes do dominio user
- `tests/load/roles.js` — operacoes do dominio role

### Files to Modify

- `Makefile` — atualizar targets para apontar para `tests/load/main.js`

### Files to Delete

- `tests/load/scenarios.js` — substituido pela estrutura modular

### Dependencies

- Nenhuma (k6 built-in modules)

## Tasks

- [ ] TASK-1: Criar tests/load/helpers.js
  - Exportar: `BASE_URL`, `SERVICE_KEY`, `SERVICE_NAME`, `errorRate` (Rate metric)
  - Funcoes de headers: `baseHeaders()`, `headersWithIdempotency(idempotencyKey)`
  - HTTP helpers: `post(url, body, headers)`, `get(url, headers)`, `put(url, body, headers)`, `del(url, headers)`
  - Response parsing: `parseData(res)`, `parseErrorMessage(res)`
  - UUID: `uuid()` — gera UUID v4 via pure-JS bit-manipulation (k6 nao tem `crypto.randomUUID()`): `'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, ...)`
  - Assertions: `assertStatus(res, expected, label)`, `assertField(res, field, expected, label)`, `assertErrorContains(res, substring, label)`, `assertFieldExists(res, field, label)`
  - files: `tests/load/helpers.js`

- [ ] TASK-2: Criar tests/load/users.js
  - Custom metrics: `createUserDuration`, `getUserDuration`, `listUsersDuration` (Trend)
  - Funcoes helper locais: `randomEmail()`, `randomName()`
  - Operacoes CRUD: `createUser()`, `getUser(id)`, `listUsers(page, limit)`, `updateUser(id)`, `deleteUser(id)`, `healthCheck()`
  - Smoke groups exportados:
    - `smokeHealthCheck()` — verifica /health e /ready
    - `smokeAuthErrors()` — request sem service key (se auth habilitado), graceful skip se desabilitado
    - `smokeUserCRUD()` — create, get, update, delete happy path
    - `smokeUserList()` — list com paginacao
    - `smokeValidationErrors()` — email invalido, UUID invalido
    - `smokeBusinessErrors()` — not found, duplicate email
    - `smokeResponseFormat()` — verifica envelope JSON consistente
  - Load operations exportadas: `loadUserOperations()` — distribuicao 40/30/20/10
  - files: `tests/load/users.js`
  - tests: TC-S-01, TC-S-02, TC-S-03, TC-S-04 a TC-S-13
  - depends: TASK-1

- [ ] TASK-3: Criar tests/load/roles.js
  - Smoke groups exportados:
    - `smokeRoleCRUD()` — create, list, delete role happy path
    - `smokeRoleErrors()` — duplicate name
  - Funcoes helper locais: `createRole()`, `listRoles()`, `deleteRole()`
  - files: `tests/load/roles.js`
  - tests: TC-S-14, TC-S-15, TC-S-16, TC-S-17
  - depends: TASK-1

- [ ] TASK-4: Criar tests/load/main.js
  - Importar smoke groups e load operations de `./users.js` e `./roles.js`
  - Importar `BASE_URL`, `get` de `./helpers.js`
  - Definir scenarios: smoke (per-vu-iterations, 1 VU, 1 iter), load, stress, spike (ramping-vus)
  - Exportar funcoes nomeadas: `smokeTest`, `loadTest`, `stressTest`, `spikeTest` (k6 exec: references)
  - Definir thresholds: `http_req_failed` (rate<0.50 para smoke, rate<0.01 para load/stress), `http_req_duration`, custom metrics, `errors`
  - Nota: smoke threshold principal e `errors: ['rate==0']` (assertion-based, nao percentil)
  - `smokeTest()` — chama smoke groups de users e roles sequencialmente
  - `loadTest()` — chama `loadUserOperations()`
  - `stressTest()` — heavy write load (mesma logica de loadTest com distribuicao diferente)
  - `spikeTest()` — burst traffic (create + get + list)
  - `setup()` — verifica health, loga config
  - `teardown()` — loga duracao
  - files: `tests/load/main.js`
  - depends: TASK-2, TASK-3

- [ ] TASK-5: Atualizar Makefile + remover scenarios.js
  - Atualizar targets: `load-smoke`, `load-test`, `load-stress`, `load-spike` para apontar `tests/load/main.js`
  - Atualizar target `load-kind` (usa `load-smoke` transitivamente — validar que funciona)
  - Remover `tests/load/scenarios.js`
  - files: `Makefile`, `tests/load/scenarios.js`
  - depends: TASK-4

- [ ] TASK-SMOKE: Executar smoke test para validar
  - Rodar `k6 run --env SCENARIO=smoke tests/load/main.js` (requer app rodando)
  - Verificar 100% checks passando
  - files: (nenhum — execucao apenas)
  - tests: TC-S-01 a TC-S-17
  - depends: TASK-5
  - note: `tests/load/results/` e criado por `make load-setup` e esta no .gitignore — nao precisa de task

## Parallel Batches

```
Batch 1: [TASK-1]           — helpers (sem dependencias)
Batch 2: [TASK-2, TASK-3]   — parallel (users.js vs roles.js, arquivos distintos)
Batch 3: [TASK-4]           — orchestrador (depends: TASK-2, TASK-3)
Batch 4: [TASK-5]           — makefile + cleanup (depends: TASK-4)
Batch 5: [TASK-SMOKE]       — validacao (depends: TASK-5)
```

File overlap analysis:
- `tests/load/helpers.js`: TASK-1 only -> exclusive
- `tests/load/users.js`: TASK-2 only -> exclusive
- `tests/load/roles.js`: TASK-3 only -> exclusive
- `tests/load/main.js`: TASK-4 only -> exclusive
- `Makefile`: TASK-5 only -> exclusive
- Batch 2 has full parallelism (users vs roles)

## Validation Criteria

- [ ] `tests/load/scenarios.js` removido
- [ ] `k6 run --env SCENARIO=smoke tests/load/main.js` executa sem erros de import
- [ ] Smoke test valida health, CRUD, errors, formato de resposta
- [ ] `make load-smoke` funciona corretamente (aponta para main.js)
- [ ] Helpers sao importaveis por novos dominios (basta criar `<domain>.js` e importar de `./helpers.js`)

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
