# Spec: Error Handling Refactor

## Status: DRAFT

## Context

O error handling atual do boilerplate viola o Open/Closed Principle: o handler (`error.go`) tem um `translateError()` god-switch que importa pacotes de dominio (`userdomain`, `roledomain`, `vo`) e precisa ser editado a cada novo erro de dominio. Alem disso:

1. **Use cases retornam erros crus** — domain errors propagam direto sem contexto, dificultando debugging
2. **Handler acoplado ao dominio** — `translateError()` com 6 cases importando 3 pacotes de dominio
3. **Sem classificacao de spans** — observabilidade nao distingue erros esperados (validacao, not found) de inesperados (DB timeout), poluindo dashboards de error rate
4. **Sem recovery customizado** — `gin.Recovery()` retorna HTML em vez de JSON padronizado
5. **Sem 422 (Unprocessable Entity)** — nao ha distincao entre erro de formato (400) e violacao de regra de negocio (422)
6. **Sem error wrapping** — use cases retornam `return nil, err` sem contexto, impossibilitando rastrear a origem

**Racional:** Baseado no ADR-010 do banking-service-yield e no artigo "Error handling in Go HTTP applications" (refletido em `docs/guides/error-handling.md` do yield). A abordagem escolhida move a responsabilidade de classificacao para o use case (onde vive o conhecimento de dominio), mantendo o handler generico e o dominio puro.

**Referencia:** banking-service-yield `docs/adr/010-error-handling-strategy.md`, `docs/guides/error-handling.md`, `pkg/telemetry/span.go`, `internal/usecases/shared/classify.go`

## Requirements

- [ ] REQ-1: **Use cases retornam `*apperror.AppError`**
  - GIVEN um use case recebe uma chamada
  - WHEN ocorre um erro de dominio (validacao, not found, conflict)
  - THEN o use case retorna `*apperror.AppError` com code e message adequados, preservando o erro original via `Unwrap()`

- [ ] REQ-2: **Classificacao de span no use case**
  - GIVEN um use case tem um span de tracing ativo
  - WHEN ocorre um erro esperado (dominio/validacao)
  - THEN o span recebe `WarnSpan` (status Ok, atributo semantico)
  - WHEN ocorre um erro inesperado (infra/timeout)
  - THEN o span recebe `FailSpan` (status Error, erro gravado)

- [ ] REQ-3: **ClassifyError centralizado**
  - GIVEN cada use case define um slice `expectedErrors` com seus erros de dominio
  - WHEN `ClassifyError(span, err, expectedErrors, msg)` e chamado
  - THEN erros que dao match via `errors.Is()` sao roteados para WarnSpan, e os demais para FailSpan

- [ ] REQ-4: **FailSpan e WarnSpan como utilitarios de pkg/telemetry**
  - GIVEN um span OpenTelemetry
  - WHEN `FailSpan(span, err, msg)` e chamado
  - THEN o span e marcado com status Error e o erro e gravado como evento
  - WHEN `WarnSpan(span, key, value)` e chamado
  - THEN o span recebe um atributo semantico sem marcar erro

- [ ] REQ-5: **Handler generico sem imports de dominio**
  - GIVEN o handler recebe um erro do use case
  - WHEN chama `HandleError(c, err)`
  - THEN extrai `*apperror.AppError` via `errors.As()` e mapeia code->HTTP status via lookup map
  - AND se nao for AppError, retorna 500 Internal Server Error

- [ ] REQ-6: **CustomRecovery retorna JSON padronizado**
  - GIVEN um panic ocorre em qualquer handler
  - WHEN o recovery middleware captura o panic
  - THEN retorna `{"errors":{"message":"internal server error"}}` com status 500
  - AND loga o panic com stack trace via slog

- [ ] REQ-7: **CodeUnprocessableEntity (422) para violacoes de regra de negocio**
  - GIVEN um erro de dominio que representa violacao de regra de negocio (nao formato/sintaxe)
  - WHEN mapeado para AppError
  - THEN usa code `UNPROCESSABLE_ENTITY` que mapeia para HTTP 422
  - Nota: nenhum erro existente no boilerplate usa 422 atualmente — o code e adicionado como capacidade para novos dominios que precisem distinguir 400 (formato) de 422 (regra de negocio)

- [ ] REQ-8: **Error wrapping contextual em use cases**
  - GIVEN um use case chama uma dependencia de infraestrutura (repo, cache)
  - WHEN a chamada falha
  - THEN o erro e wrapped com contexto: `fmt.Errorf("creating user: %w", err)`

## Test Plan

### Domain Tests

N/A — Esta refatoracao nao altera a camada de dominio. Os sentinels existentes (`ErrUserNotFound`, `ErrRoleNotFound`, `ErrDuplicateRoleName`, `vo.ErrInvalidEmail`, `vo.ErrInvalidID`) permanecem inalterados.

### Unit Tests (pkg + shared)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-U-01 | REQ-4 | happy | FailSpan sets Error status and records error event | span.Status=Error, events contain error |
| TC-U-02 | REQ-4 | happy | WarnSpan adds attribute without setting Error status | span.Status=Unset, attribute present |
| TC-U-03 | REQ-4 | edge | FailSpan with nil span is no-op | no panic |
| TC-U-04 | REQ-3 | happy | ClassifyError routes expected error to WarnSpan | WarnSpan called |
| TC-U-05 | REQ-3 | happy | ClassifyError routes unexpected error to FailSpan | FailSpan called |
| TC-U-06 | REQ-3 | edge | ClassifyError with wrapped expected error still matches | WarnSpan called (errors.Is traverses chain) |
| TC-U-07 | REQ-3 | edge | ClassifyError with empty expectedErrors treats all as unexpected | FailSpan called |
| TC-U-08 | REQ-3 | edge | ClassifyError with nil error is no-op | no calls |
| TC-U-09 | REQ-6 | happy | CustomRecovery catches string panic and returns JSON 500 | `{"errors":{"message":"internal server error"}}` |
| TC-U-10 | REQ-6 | happy | CustomRecovery passes through when no panic | original handler runs, 200 OK |
| TC-U-11 | REQ-6 | edge | CustomRecovery catches error-type panic and returns JSON 500 | `{"errors":{"message":"internal server error"}}` |
| TC-U-12 | REQ-5 | happy | HandleError maps INVALID_REQUEST to 400 | HTTP 400 + JSON |
| TC-U-13 | REQ-5 | happy | HandleError maps NOT_FOUND to 404 | HTTP 404 + JSON |
| TC-U-14 | REQ-5 | happy | HandleError maps CONFLICT to 409 | HTTP 409 + JSON |
| TC-U-15 | REQ-7 | happy | HandleError maps UNPROCESSABLE_ENTITY to 422 | HTTP 422 + JSON |
| TC-U-16 | REQ-5 | edge | HandleError with non-AppError returns 500 | HTTP 500 + JSON |
| TC-U-17 | REQ-1 | happy | apperror.Wrap preserves original error chain | errors.Is(wrapped, original) == true |
| TC-U-18 | REQ-7 | happy | apperror constants include CodeUnprocessableEntity | constant exists with value "UNPROCESSABLE_ENTITY" |

### Use Case Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-1 | happy | CreateUser success returns nil | nil error, user returned |
| TC-UC-02 | REQ-1 | validation | CreateUser with invalid email returns AppError INVALID_REQUEST | code=INVALID_REQUEST, errors.Is(err, vo.ErrInvalidEmail) |
| TC-UC-03 | REQ-1 | business | CreateUser duplicate email returns AppError CONFLICT | code=CONFLICT |
| TC-UC-04 | REQ-2,8 | infra | CreateUser DB error returns AppError INTERNAL_ERROR | code=INTERNAL_ERROR, err contains "creating user:" context |
| TC-UC-05 | REQ-1 | happy | GetUser success returns user | nil error |
| TC-UC-06 | REQ-1 | business | GetUser not found returns AppError NOT_FOUND | code=NOT_FOUND |
| TC-UC-07 | REQ-2,8 | infra | GetUser DB error returns AppError INTERNAL_ERROR | code=INTERNAL_ERROR |
| TC-UC-08 | REQ-1 | happy | UpdateUser success returns nil | nil error |
| TC-UC-09 | REQ-1 | business | UpdateUser not found returns AppError NOT_FOUND | code=NOT_FOUND |
| TC-UC-10 | REQ-1 | validation | UpdateUser invalid email returns AppError INVALID_REQUEST | code=INVALID_REQUEST |
| TC-UC-11 | REQ-1 | happy | DeleteUser success returns nil | nil error |
| TC-UC-12 | REQ-1 | business | DeleteUser not found returns AppError NOT_FOUND | code=NOT_FOUND |
| TC-UC-13 | REQ-1 | validation | GetUser invalid ID returns AppError INVALID_REQUEST | code=INVALID_REQUEST, errors.Is(err, vo.ErrInvalidID) |
| TC-UC-14 | REQ-1 | happy | ListUsers success returns users | nil error |
| TC-UC-15 | REQ-2,8 | infra | ListUsers DB error returns AppError INTERNAL_ERROR | code=INTERNAL_ERROR |
| TC-UC-16 | REQ-1 | happy | RoleCreate success returns nil | nil error |
| TC-UC-17 | REQ-1 | business | RoleCreate duplicate name returns AppError CONFLICT | code=CONFLICT |
| TC-UC-18 | REQ-2,8 | infra | RoleCreate DB error returns AppError INTERNAL_ERROR | code=INTERNAL_ERROR |
| TC-UC-19 | REQ-1 | happy | RoleList success returns roles | nil error |
| TC-UC-20 | REQ-2,8 | infra | RoleList DB error returns AppError INTERNAL_ERROR | code=INTERNAL_ERROR |
| TC-UC-21 | REQ-1 | happy | RoleDelete success returns nil | nil error |
| TC-UC-22 | REQ-1 | business | RoleDelete not found returns AppError NOT_FOUND | code=NOT_FOUND |

### E2E Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-5 | validation | POST /users invalid email returns JSON 400 | `{"errors":{"message":"..."}}` |
| TC-E2E-02 | REQ-5 | business | POST /users duplicate email returns JSON 409 | `{"errors":{"message":"..."}}` |
| TC-E2E-03 | REQ-5 | business | GET /users/:id not found returns JSON 404 | `{"errors":{"message":"..."}}` |
| TC-E2E-04 | REQ-5 | validation | GET /users/:id invalid UUID returns JSON 400 | `{"errors":{"message":"..."}}` |
| TC-E2E-05 | REQ-6 | edge | Panic recovery returns JSON 500 (not HTML) | `{"errors":{"message":"internal server error"}}` |

### Smoke Tests (k6)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-5 | validation | POST /users invalid email | 400 + JSON error format |
| TC-S-02 | REQ-5 | business | POST /users duplicate email | 409 + JSON error format |
| TC-S-03 | REQ-5 | business | GET /users/:id not found | 404 + JSON error format |
| TC-S-04 | REQ-5 | validation | GET /users/:id invalid UUID | 400 + JSON error format |
| TC-S-05 | REQ-5,6 | edge | All error responses use `{"errors":{"message":...}}` format | consistent JSON envelope |

## Design

### Architecture Decisions

**Fluxo de erro (3 camadas):**
```
Domain Layer              Use Case Layer                    Handler Layer
errors.New("...")    ->   toAppError() + ClassifyError()   ->   errors.As() + codeToStatus map
(sentinels puros)         (wrapping + span classification)      (HTTP translation generico)
```

**Decisao de HTTP status:**
- **400 INVALID_REQUEST** = erro de formato/sintaxe (UUID invalido, JSON malformado, email invalido)
- **409 CONFLICT** = duplicata ou conflito de versao
- **422 UNPROCESSABLE_ENTITY** = formato valido mas regra de negocio violada
- **404 NOT_FOUND** = recurso nao encontrado
- **500 INTERNAL_ERROR** = falha de infra (DB, cache, timeout)

**Padrao por use case (3 elementos):**
1. `var createExpectedErrors = []error{...}` — slice de erros de dominio esperados
2. `func userToAppError(err error) *apperror.AppError` — mapeia dominio -> AppError
3. Chamada a `ucshared.ClassifyError(span, err, createExpectedErrors, "context msg")` antes do return

### Files to Create

- `pkg/telemetry/span.go` — FailSpan, WarnSpan helpers
- `pkg/telemetry/span_test.go`
- `internal/usecases/shared/classify.go` — ClassifyError
- `internal/usecases/shared/classify_test.go`
- `internal/infrastructure/web/middleware/recovery.go` — CustomRecovery
- `internal/infrastructure/web/middleware/recovery_test.go`
- `docs/adr/009-error-handling.md` — ADR documentando a decisao
- `docs/guides/error-handling.md` — guia pratico de implementacao

### Files to Modify

- `pkg/apperror/apperror.go` — adicionar CodeUnprocessableEntity + UnprocessableEntity() (Wrap ja existe)
- `internal/infrastructure/web/handler/error.go` — remover translateError, simplificar HandleError(c, err)
- `internal/infrastructure/web/router/router.go` — trocar gin.Recovery() por CustomRecovery()
- `internal/usecases/user/create.go` — expectedErrors + toAppError + ClassifyError + error wrapping
- `internal/usecases/user/get.go` — idem
- `internal/usecases/user/update.go` — idem
- `internal/usecases/user/delete.go` — idem
- `internal/usecases/user/list.go` — idem
- `internal/usecases/user/create_test.go` — atualizar para esperar *apperror.AppError
- `internal/usecases/user/get_test.go` — idem
- `internal/usecases/user/update_test.go` — idem
- `internal/usecases/user/delete_test.go` — idem
- `internal/usecases/user/list_test.go` — idem
- `internal/usecases/role/create.go` — expectedErrors + toAppError + ClassifyError
- `internal/usecases/role/list.go` — idem
- `internal/usecases/role/delete.go` — idem
- `internal/usecases/role/create_test.go` — atualizar testes
- `internal/usecases/role/delete_test.go` — atualizar testes
- `internal/infrastructure/web/handler/user.go` — simplificar chamadas HandleError (remover span param)
- `internal/infrastructure/web/handler/role.go` — idem
- `tests/e2e/user_test.go` — verificar respostas de erro JSON
- `tests/e2e/role_test.go` — idem

### Dependencies

- Nenhuma dependencia externa nova (OpenTelemetry ja esta no go.mod)

## Tasks

- [ ] TASK-1: Adicionar CodeUnprocessableEntity ao pkg/apperror
  - Adicionar constante `CodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"`
  - Adicionar constructor `UnprocessableEntity(message string) *AppError` (1 param com code hardcoded — diverge dos existentes que usam 2 params `(code, message)`, mas e melhor DX: `apperror.UnprocessableEntity("msg")` e mais claro que `apperror.UnprocessableEntity("UNPROCESSABLE_ENTITY", "msg")`)
  - Nota: `Wrap()` ja existe em `apperror.go` — apenas verificar que preserva chain via `Unwrap()` (TC-U-17 e teste de regressao)
  - files: `pkg/apperror/apperror.go`, `pkg/apperror/apperror_test.go`
  - tests: TC-U-17, TC-U-18

- [ ] TASK-2: Criar FailSpan/WarnSpan em pkg/telemetry/span.go
  - `FailSpan(span trace.Span, err error, msg string)` — seta status Error, grava erro como evento
  - `WarnSpan(span trace.Span, key, value string)` — adiciona atributo semantico, nao marca erro
  - Ambos sao no-op se span for nil
  - files: `pkg/telemetry/span.go`, `pkg/telemetry/span_test.go`
  - tests: TC-U-01, TC-U-02, TC-U-03

- [ ] TASK-3: Criar ClassifyError em internal/usecases/shared/
  - `ClassifyError(span trace.Span, err error, expectedErrors []error, contextMsg string)`
  - Itera `expectedErrors`, usa `errors.Is()` para match (suporta wrapping)
  - Match -> WarnSpan; no match -> FailSpan
  - nil error -> no-op
  - files: `internal/usecases/shared/classify.go`, `internal/usecases/shared/classify_test.go`
  - tests: TC-U-04, TC-U-05, TC-U-06, TC-U-07, TC-U-08
  - depends: TASK-2

- [ ] TASK-4: Criar CustomRecovery middleware
  - `CustomRecovery() gin.HandlerFunc` usando `gin.CustomRecovery`
  - Captura panic, loga via slog.Error com stack trace, retorna JSON 500 padronizado
  - files: `internal/infrastructure/web/middleware/recovery.go`, `internal/infrastructure/web/middleware/recovery_test.go`
  - tests: TC-U-09, TC-U-10, TC-U-11

- [ ] TASK-5: Simplificar handler/error.go — remover translateError
  - `HandleError(c *gin.Context, err error)` — apenas 2 params (sem span)
  - Nota: `codeToStatus` map ja existe (7 entries) — adicionar entry `CodeUnprocessableEntity: 422` (total: 8 entries)
  - Remover o fallback `translateError()` — apos refatoracao dos use cases (TASK-6/7), todos os erros serao `*apperror.AppError`, tornando o fallback desnecessario
  - Remover todos os imports de dominio (`userdomain`, `roledomain`, `vo`) e de OTel span (`go.opentelemetry.io/otel/codes`, `go.opentelemetry.io/otel/trace`)
  - Se nao for AppError, retorna 500 com mensagem generica
  - files: `internal/infrastructure/web/handler/error.go`, `internal/infrastructure/web/handler/error_test.go`
  - tests: TC-U-12, TC-U-13, TC-U-14, TC-U-15, TC-U-16
  - depends: TASK-1

- [ ] TASK-6: Refatorar user use cases — expectedErrors + toAppError + ClassifyError
  - Pre-requisito neste task: adicionar `ErrDuplicateEmail = errors.New("email already exists")` em `internal/domain/user/errors.go` (ao lado de `ErrUserNotFound`). O repositorio deve traduzir PG unique constraint violation → `user.ErrDuplicateEmail` (mantendo Clean Architecture: repo devolve domain error, nao PG error)
  - Para cada use case (create, get, update, delete, list):
    - Definir `var xxxExpectedErrors = []error{...}` com erros de dominio esperados
    - Criar `func userToAppError(err error) *apperror.AppError` (compartilhada ou por arquivo)
    - Chamar `ucshared.ClassifyError(span, err, xxxExpectedErrors, "context msg")` antes do return
    - Wrapping contextual: `fmt.Errorf("creating user: %w", err)` em chamadas de infra
    - Retornar `*apperror.AppError` em vez de erros crus
  - Atualizar interfaces se necessario (return type pode mudar para `error` generico, mas o valor concreto sera AppError)
  - Atualizar TODOS os testes unitarios de user use cases para verificar AppError codes
  - files: `internal/domain/user/errors.go`, `internal/infrastructure/db/postgres/repository/user_repository.go`, `internal/usecases/user/create.go`, `internal/usecases/user/get.go`, `internal/usecases/user/update.go`, `internal/usecases/user/delete.go`, `internal/usecases/user/list.go`, `internal/usecases/user/create_test.go`, `internal/usecases/user/get_test.go`, `internal/usecases/user/update_test.go`, `internal/usecases/user/delete_test.go`, `internal/usecases/user/list_test.go`
  - Nota: `vo.ErrInvalidID` deve ser incluido em expectedErrors dos use cases get/update/delete. `user.ErrDuplicateEmail` em create/update expectedErrors.
  - tests: TC-UC-01, TC-UC-02, TC-UC-03, TC-UC-04, TC-UC-05, TC-UC-06, TC-UC-07, TC-UC-08, TC-UC-09, TC-UC-10, TC-UC-11, TC-UC-12, TC-UC-13, TC-UC-14, TC-UC-15
  - depends: TASK-1, TASK-3

- [ ] TASK-7: Refatorar role use cases — expectedErrors + toAppError + ClassifyError
  - Mesmo padrao de TASK-6 aplicado a role (create, list, delete)
  - Atualizar testes unitarios de role use cases
  - files: `internal/usecases/role/create.go`, `internal/usecases/role/list.go`, `internal/usecases/role/delete.go`, `internal/usecases/role/create_test.go`, `internal/usecases/role/list_test.go`, `internal/usecases/role/delete_test.go`
  - tests: TC-UC-16, TC-UC-17, TC-UC-18, TC-UC-19, TC-UC-20, TC-UC-21, TC-UC-22
  - depends: TASK-1, TASK-3

- [ ] TASK-8: Simplificar handlers — remover span param do HandleError
  - Atualizar `user_handler.go`: todas as chamadas de HandleError passam apenas `(c, err)`, remover imports de OTel span (`go.opentelemetry.io/otel/codes`, `go.opentelemetry.io/otel/trace`) — manter import de `internal/infrastructure/telemetry` (business metrics)
  - Atualizar `role_handler.go`: idem
  - Atualizar router.go: trocar `gin.Recovery()` por `middleware.CustomRecovery()`
  - files: `internal/infrastructure/web/handler/user.go`, `internal/infrastructure/web/handler/role.go`, `internal/infrastructure/web/router/router.go`
  - depends: TASK-3, TASK-4, TASK-5, TASK-6, TASK-7

- [ ] TASK-9: Atualizar E2E tests para verificar error responses
  - Verificar que erros retornam JSON padronizado (nao HTML)
  - Verificar status codes corretos (400, 404, 409)
  - Adicionar test de panic recovery (force panic, verify JSON 500)
  - files: `tests/e2e/user_test.go`, `tests/e2e/role_test.go`
  - tests: TC-E2E-01, TC-E2E-02, TC-E2E-03, TC-E2E-04, TC-E2E-05
  - depends: TASK-8

- [ ] TASK-10: Criar ADR-009 + guia de error handling + superseder ADR-004
  - ADR-009 documenta: problema (4 requisitos conflitantes), alternativas avaliadas, decisao (structured errors no use case), consequencias
  - Referenciar ADR-004 como predecessor: "Supersedes ADR-004 (Error Handling Layered Translation) which established the layered approach. ADR-009 refines it by moving classification to the use case and eliminating the domain-importing translateError."
  - Atualizar `docs/adr/004-error-handling.md` status para "Superseded by ADR-009"
  - Guia pratico: principios, fluxo completo, anatomia do error handling no use case, checklist para adicionar novos erros
  - files: `docs/adr/009-error-handling.md`, `docs/adr/004-error-handling.md`, `docs/guides/error-handling.md`

- [ ] TASK-11: k6 smoke tests para error responses
  - Adicionar smoke groups em `tests/load/scenarios.js` (ou novo arquivo modular se load-tests-modular spec ja foi executada)
  - Testar: invalid email (400), duplicate (409), not found (404), invalid UUID (400), formato de resposta
  - files: `tests/load/scenarios.js`
  - tests: TC-S-01, TC-S-02, TC-S-03, TC-S-04, TC-S-05
  - depends: TASK-8

## Parallel Batches

```
Batch 1: [TASK-1, TASK-2, TASK-4, TASK-10]  — fundacoes independentes (apperror, telemetry, middleware, docs)
Batch 2: [TASK-3, TASK-5]                    — depends de TASK-1/TASK-2 (classify, handler)
Batch 3: [TASK-6, TASK-7]                    — parallel (user vs role use cases, arquivos distintos)
Batch 4: [TASK-8]                            — integracao (handlers + router, depends de todos anteriores)
Batch 5: [TASK-9, TASK-11]                   — parallel (E2E tests vs k6 smoke, arquivos distintos)
```

File overlap analysis:
- `pkg/apperror/apperror.go`: TASK-1 only -> exclusive
- `pkg/telemetry/span.go`: TASK-2 only -> exclusive
- `internal/usecases/shared/classify.go`: TASK-3 only -> exclusive
- `internal/infrastructure/web/middleware/recovery.go`: TASK-4 only -> exclusive
- `internal/infrastructure/web/handler/error.go`: TASK-5 only -> exclusive
- `internal/domain/user/errors.go`: TASK-6 only -> exclusive
- `internal/infrastructure/db/postgres/repository/user_repository.go`: TASK-6 only -> exclusive
- `internal/usecases/user/*.go`: TASK-6 only -> exclusive
- `internal/usecases/role/*.go`: TASK-7 only -> exclusive
- `internal/infrastructure/web/handler/user.go`: TASK-8 only -> exclusive
- `internal/infrastructure/web/router/router.go`: TASK-8 only -> exclusive
- `docs/adr/004-error-handling.md`: TASK-10 only -> exclusive
- All other files: exclusive to one task

## Validation Criteria

- [ ] `go build ./...` passa
- [ ] `make lint` passa
- [ ] `make test` passa (todos os testes unitarios + E2E)
- [ ] Handler `error.go` nao importa nenhum pacote de dominio (`domain/user`, `domain/role`)
- [ ] Todos os use cases retornam `*apperror.AppError` (nao erros crus)
- [ ] `errors.Is()` funciona atraves da chain de wrapping
- [ ] Panic em handler retorna JSON 500 (nao HTML)
- [ ] Nenhum span e classificado no handler (apenas no use case)
- [ ] GET /users/:id com UUID invalido retorna 400 (nao 500) — regressao de `vo.ErrInvalidID`

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
