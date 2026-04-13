# Spec: CLI Template Quality — Production-Grade Domain Scaffolding

## Status: DONE

## Context

O `gopherplate add domain` gera 18 arquivos mas com qualidade muito abaixo dos dominios de referencia (`user`/`role`). Problemas criticos:

1. **Zero testes gerados** — nenhum `_test.go` para entity, use cases, repository ou handler. O dominio `user` tem 9 arquivos de teste; o scaffold gera 0.
2. **Error handling quebrado** — use cases nao usam `ClassifyError`, `toAppError`, `expectedErrors`, nem extraem span do contexto. Violam o padrao definido no ADR-009.
3. **Handler com assinatura errada** — template chama `HandleError(c, span, err)` com 3 params, mas apos o refactor de error handling a assinatura e `HandleError(c, err)` com 2 params.
4. **Sem `usecases/errors.go`** — nao gera o arquivo de mapeamento `toAppError` + `expectedErrors` que e obrigatorio.
5. **Sem `mocks_test.go`** — nao gera os hand-written mocks que o padrao exige.
6. **Sem metricas no handler** — referencia tem `h.Metrics.RecordCreate(ctx)`, template nao.

**Principio:** O codigo gerado pelo scaffold DEVE ter a mesma qualidade e seguir os mesmos padroes dos dominios de referencia. Se um dev nao consegue distinguir codigo gerado de codigo escrito manualmente, o scaffold esta correto.

**Referencia:** `internal/domain/user/`, `internal/usecases/user/`, `internal/usecases/role/`, `internal/infrastructure/db/postgres/repository/user.go`, `internal/infrastructure/web/handler/user.go`

## Requirements

- [ ] REQ-1: **Use case templates usam ClassifyError + toAppError**
  - GIVEN o scaffold gera um use case (create, get, update, delete, list)
  - WHEN o codigo e gerado
  - THEN extrai span via `trace.SpanFromContext(ctx)`
  - AND chama `ucshared.ClassifyError(span, err, expectedErrors, contextMsg)` em todo error path
  - AND retorna `toAppError(err)` em vez de erro cru
  - AND wraps erros de infra com `fmt.Errorf("creating {{domain}}: %w", err)`

- [ ] REQ-2: **Scaffold gera `usecases/{{domain}}/errors.go`**
  - GIVEN o scaffold gera use cases
  - WHEN o dominio e criado
  - THEN existe `internal/usecases/{{domain}}/errors.go` com:
  - `var createExpectedErrors`, `getExpectedErrors`, `updateExpectedErrors`, `deleteExpectedErrors` (slices de erros de dominio)
  - `func {{domain}}ToAppError(err error) *apperror.AppError` mapeando: NotFound → CodeNotFound, DuplicateKey → CodeConflict, default → CodeInternalError

- [ ] REQ-3: **Handler usa `HandleError(c, err)` com 2 params**
  - GIVEN o scaffold gera um handler
  - WHEN ocorre erro
  - THEN chama `HandleError(c, execErr)` (2 params, sem span)
  - AND handler NAO chama `span.SetStatus` nem `span.RecordError`

- [ ] REQ-4: **Scaffold gera `mocks_test.go`**
  - GIVEN o scaffold gera use cases
  - WHEN o dominio e criado
  - THEN existe `internal/usecases/{{domain}}/mocks_test.go` com MockRepository implementando a interface Repository
  - AND cada metodo retorna valores configurados via campos do mock

- [ ] REQ-5: **Scaffold gera testes para todos os use cases**
  - GIVEN o scaffold gera use cases
  - WHEN o dominio e criado
  - THEN existem: `create_test.go`, `get_test.go`, `update_test.go`, `delete_test.go`, `list_test.go`
  - AND cada teste tem: happy path + error path (repo error)
  - AND assertions usam `errors.As(err, &appErr)` para verificar `appErr.Code`
  - AND testes sao table-driven com nomes descritivos

- [ ] REQ-6: **Scaffold gera `entity_test.go`**
  - GIVEN o scaffold gera o dominio
  - WHEN o dominio e criado
  - THEN existe `internal/domain/{{domain}}/entity_test.go` com teste da factory (NewEntity) e metodos de negocio

- [ ] REQ-7: **Scaffold gera `filter_test.go`**
  - GIVEN o scaffold gera o dominio
  - WHEN o dominio e criado
  - THEN existe `internal/domain/{{domain}}/filter_test.go` com testes de Normalize (defaults) e Offset (calculo)

- [ ] REQ-8: **Scaffold gera `repository_test.go` com go-sqlmock**
  - GIVEN o scaffold gera o repositorio
  - WHEN o dominio e criado
  - THEN existe `internal/infrastructure/db/postgres/repository/{{domain}}_test.go`
  - AND usa `go-sqlmock` para testar Create, FindByID, List, Update, Delete
  - AND testa cenario de sql.ErrNoRows → ErrNotFound

- [ ] REQ-9: **Domain errors.go tem ErrDuplicate**
  - GIVEN o scaffold gera erros de dominio
  - WHEN o dominio e criado
  - THEN `internal/domain/{{domain}}/errors.go` contem tanto `Err{{Name}}NotFound` quanto `Err{{Name}}DuplicateKey`

- [ ] REQ-10: **Repository detecta PG unique constraint violation**
  - GIVEN o scaffold gera o repositorio
  - WHEN ocorre violacao de unique constraint (PG code 23505)
  - THEN o repositorio retorna `Err{{Name}}DuplicateKey` em vez do erro bruto do PG

- [ ] REQ-11: **Migration tem campo unique**
  - GIVEN o scaffold gera a migration
  - WHEN a tabela e criada
  - THEN tem `name VARCHAR(255) NOT NULL` com `UNIQUE` constraint
  - AND tem ambas secoes `-- +goose Up` e `-- +goose Down`

- [ ] REQ-12: **Codigo gerado compila e testes passam**
  - GIVEN o scaffold gera o dominio completo
  - WHEN `go build ./...` e executado no projeto com o dominio adicionado
  - THEN compila sem erros
  - WHEN `go test ./internal/domain/{{domain}}/... ./internal/usecases/{{domain}}/...` e executado
  - THEN todos os testes passam

- [ ] REQ-13: **Teste de integracao valida scaffold completo**
  - GIVEN existe um teste de integracao para `add domain`
  - WHEN executa `add domain product` em um projeto scaffoldado
  - THEN verifica: todos os arquivos existem (30+), conteudo contem patterns corretos (ClassifyError, HandleError 2-params, expectedErrors, mocks), codigo compila

## Test Plan

### Integration Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-I-01 | REQ-12 | happy | add domain product generates compilable code | `go build ./...` passes |
| TC-I-02 | REQ-12 | happy | generated domain tests pass | `go test ./internal/.../product/...` passes |
| TC-I-03 | REQ-1 | happy | create_usecase.go contains ClassifyError | string match in generated file |
| TC-I-04 | REQ-1 | happy | create_usecase.go contains toAppError | string match |
| TC-I-05 | REQ-2 | happy | errors.go exists with expectedErrors | file exists + content check |
| TC-I-06 | REQ-3 | happy | handler.go calls HandleError with 2 params | `HandleError(c, execErr)` not `HandleError(c, span, execErr)` |
| TC-I-07 | REQ-4 | happy | mocks_test.go exists with MockRepository | file exists + content check |
| TC-I-08 | REQ-5 | happy | all 5 use case test files exist | create_test.go through list_test.go |
| TC-I-09 | REQ-6 | happy | entity_test.go exists | file exists + has TestNew function |
| TC-I-10 | REQ-7 | happy | filter_test.go exists | file exists + has TestNormalize |
| TC-I-11 | REQ-8 | happy | repository_test.go exists with sqlmock | file exists + has go-sqlmock import |
| TC-I-12 | REQ-9 | happy | domain errors.go has DuplicateKey | ErrDuplicateKey present |
| TC-I-13 | REQ-10 | happy | repository detects PG 23505 | pq.Error code check in file |
| TC-I-14 | REQ-11 | happy | migration has UNIQUE constraint | content check |
| TC-I-15 | REQ-13 | happy | 30+ files generated | file count check |
| TC-I-16 | REQ-1 | edge | all 5 use cases have span extraction | `trace.SpanFromContext` in each |

## Design

### Architecture Decisions

**Templates seguem exatamente os padroes de referencia:**
Cada template deve produzir codigo que e indistinguivel do escrito manualmente nos dominios `user`/`role`. A referencia e a fonte de verdade; o template e uma reproducao parametrizada.

**Novos templates a criar (13):**
1. `usecase_errors.go.tmpl` — expectedErrors + toAppError
2. `mocks_test.go.tmpl` — MockRepository hand-written
3. `create_usecase_test.go.tmpl` — success + error tests
4. `get_usecase_test.go.tmpl`
5. `update_usecase_test.go.tmpl`
6. `delete_usecase_test.go.tmpl`
7. `list_usecase_test.go.tmpl`
8. `entity_test.go.tmpl`
9. `filter_test.go.tmpl`
10. `repository_postgres_test.go.tmpl` — go-sqlmock pattern
11. (handler_test nao incluido — handlers sao testados via E2E)

**Templates existentes a corrigir (6):**
1. `create_usecase.go.tmpl` — add ClassifyError + toAppError + span + wrapping
2. `get_usecase.go.tmpl` — idem
3. `update_usecase.go.tmpl` — idem
4. `delete_usecase.go.tmpl` — idem
5. `list_usecase.go.tmpl` — idem
6. `handler.go.tmpl` — fix HandleError to 2 params, remove span.SetStatus
7. `errors.go.tmpl` — add ErrDuplicateKey
8. `repository_postgres.go.tmpl` — add PG 23505 detection
9. `migration.sql.tmpl` — add UNIQUE constraint on name

**Arquivo de mapeamento de templates (embed.go):**
Atualizar para incluir os novos .tmpl files.

**add_domain.go:**
Atualizar `buildTemplateMappings` para mapear novos templates a output paths.

### Files to Create

- `cmd/cli/templates/domain/usecase_errors.go.tmpl`
- `cmd/cli/templates/domain/mocks_test.go.tmpl`
- `cmd/cli/templates/domain/create_usecase_test.go.tmpl`
- `cmd/cli/templates/domain/get_usecase_test.go.tmpl`
- `cmd/cli/templates/domain/update_usecase_test.go.tmpl`
- `cmd/cli/templates/domain/delete_usecase_test.go.tmpl`
- `cmd/cli/templates/domain/list_usecase_test.go.tmpl`
- `cmd/cli/templates/domain/entity_test.go.tmpl`
- `cmd/cli/templates/domain/filter_test.go.tmpl`
- `cmd/cli/templates/domain/repository_postgres_test.go.tmpl`

### Files to Modify

- `cmd/cli/templates/domain/create_usecase.go.tmpl` — ClassifyError + toAppError + span
- `cmd/cli/templates/domain/get_usecase.go.tmpl` — idem
- `cmd/cli/templates/domain/update_usecase.go.tmpl` — idem
- `cmd/cli/templates/domain/delete_usecase.go.tmpl` — idem
- `cmd/cli/templates/domain/list_usecase.go.tmpl` — idem
- `cmd/cli/templates/domain/handler.go.tmpl` — HandleError 2 params, no span.SetStatus
- `cmd/cli/templates/domain/errors.go.tmpl` — add ErrDuplicateKey
- `cmd/cli/templates/domain/repository_postgres.go.tmpl` — PG 23505 detection
- `cmd/cli/templates/domain/migration.sql.tmpl` — UNIQUE constraint
- `cmd/cli/templates/domain/embed.go` — include new templates
- `cmd/cli/commands/add_domain.go` — update buildTemplateMappings for new files
- `cmd/cli/scaffold/integration_test.go` — comprehensive integration test

### Dependencies

- Nenhuma dependencia externa nova

## Tasks

- [x] TASK-1: Corrigir 5 use case templates com ClassifyError + toAppError + span
  - Para cada template (create, get, update, delete, list):
    - Adicionar `span := trace.SpanFromContext(ctx)` no inicio
    - Adicionar import de `ucshared` e `trace`
    - Em cada error path: `ucshared.ClassifyError(span, err, xxxExpectedErrors, "context")` + `return nil, {{domainCamel}}ToAppError(err)`
    - Adicionar error wrapping: `fmt.Errorf("creating {{domain}}: %w", err)`
  - Referencia: `internal/usecases/user/create.go` como padrao exato
  - files: `cmd/cli/templates/domain/create_usecase.go.tmpl`, `get_usecase.go.tmpl`, `update_usecase.go.tmpl`, `delete_usecase.go.tmpl`, `list_usecase.go.tmpl`

- [x] TASK-2: Criar template usecase_errors.go.tmpl
  - Gera `internal/usecases/{{domain}}/errors.go` com:
    - `var createExpectedErrors = []error{...}` para cada operacao
    - `func {{domainCamel}}ToAppError(err error) *apperror.AppError` com switch/case para cada erro de dominio
    - Imports: errors, domain package, apperror
  - Referencia: `internal/usecases/user/errors.go` e `internal/usecases/role/errors.go`
  - files: `cmd/cli/templates/domain/usecase_errors.go.tmpl`

- [x] TASK-3: Corrigir handler.go.tmpl — HandleError 2 params + remover span.SetStatus
  - Mudar `HandleError(c, span, execErr)` para `HandleError(c, execErr)`
  - Remover qualquer `span.SetStatus(codes.Error, ...)` de bind errors
  - Remover imports de `go.opentelemetry.io/otel/codes` se nao usado
  - Manter span para atributos (user.id, etc) — so nao classificar erros no handler
  - Referencia: `internal/infrastructure/web/handler/user.go` pos-refactor
  - files: `cmd/cli/templates/domain/handler.go.tmpl`

- [x] TASK-4: Corrigir errors.go.tmpl (domain) + repository + migration
  - `errors.go.tmpl`: adicionar `Err{{Name}}DuplicateKey = errors.New("{{domain}} already exists")`
  - `repository_postgres.go.tmpl`: adicionar deteccao PG 23505 no Create method → `Err{{Name}}DuplicateKey`
  - `migration.sql.tmpl`: adicionar `UNIQUE` constraint em `name`
  - Referencia: `internal/domain/user/errors.go`, `internal/infrastructure/db/postgres/repository/user.go`
  - files: `cmd/cli/templates/domain/errors.go.tmpl`, `repository_postgres.go.tmpl`, `migration.sql.tmpl`

- [x] TASK-5: Criar template mocks_test.go.tmpl
  - Gera `internal/usecases/{{domain}}/mocks_test.go` com:
    - `type mockRepository struct` com campos funcionais (nao framework)
    - Implementa todos os metodos da interface Repository
    - Cada metodo retorna valores dos campos configurados
  - Padrao hand-written (sem testify/mock, sem mockery) — seguir exatamente o padrao de `mocks_test.go` do user/role
  - Referencia: `internal/usecases/user/mocks_test.go` ou `internal/usecases/role/mocks_test.go`
  - files: `cmd/cli/templates/domain/mocks_test.go.tmpl`
  - depends: TASK-1

- [x] TASK-6: Criar templates de teste para use cases (5 arquivos)
  - Criar: `create_usecase_test.go.tmpl`, `get_usecase_test.go.tmpl`, `update_usecase_test.go.tmpl`, `delete_usecase_test.go.tmpl`, `list_usecase_test.go.tmpl`
  - Cada teste deve ter: happy path + repo error path
  - Assertions usam `errors.As(err, &appErr)` para verificar `appErr.Code`
  - Table-driven com nomes descritivos
  - Usam MockRepository do mocks_test.go
  - Referencia: `internal/usecases/role/create_test.go` (mais simples que user, melhor como template)
  - files: `cmd/cli/templates/domain/create_usecase_test.go.tmpl`, `get_usecase_test.go.tmpl`, `update_usecase_test.go.tmpl`, `delete_usecase_test.go.tmpl`, `list_usecase_test.go.tmpl`
  - depends: TASK-5

- [x] TASK-7: Criar templates de teste para domain (entity + filter)
  - `entity_test.go.tmpl`: testa factory New{{Name}} (campos preenchidos, ID gerado, timestamps)
  - `filter_test.go.tmpl`: testa Normalize (defaults page=1, limit=20) e Offset (calculo)
  - Referencia: `internal/domain/user/entity_test.go`, `internal/domain/user/filter_test.go`
  - files: `cmd/cli/templates/domain/entity_test.go.tmpl`, `filter_test.go.tmpl`

- [x] TASK-8: Criar template repository_postgres_test.go.tmpl
  - Testa com go-sqlmock: Create (success), FindByID (success + not found), List, Update, Delete
  - Padrao: `sqlmock.New()` → `sqlx.NewDb()` → execute → verify expectations
  - Referencia: `internal/infrastructure/db/postgres/repository/user_test.go`
  - files: `cmd/cli/templates/domain/repository_postgres_test.go.tmpl`

- [x] TASK-9: Atualizar embed.go + add_domain.go para incluir novos templates
  - `embed.go`: verificar que todos os novos .tmpl files sao incluidos no embed
  - `add_domain.go` (`buildTemplateMappings`): adicionar mapeamentos para: usecase_errors.go, mocks_test.go, create_test.go, get_test.go, update_test.go, delete_test.go, list_test.go, entity_test.go, filter_test.go, repository_test.go
  - Atualizar contagem de arquivos no output (de 18 para ~28)
  - files: `cmd/cli/templates/domain/embed.go`, `cmd/cli/commands/add_domain.go`
  - depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6, TASK-7, TASK-8

- [x] TASK-10: Teste de integracao — scaffold + build + test
  - Expandir `cmd/cli/scaffold/integration_test.go` para verificar:
    - Todos os 28+ arquivos gerados existem
    - Conteudo de create.go contem "ClassifyError", "toAppError", "SpanFromContext"
    - Conteudo de handler.go contem `HandleError(c, execErr)` (2 params)
    - Conteudo de errors.go (domain) contem "DuplicateKey"
    - Conteudo de errors.go (usecases) contem "expectedErrors", "ToAppError"
    - Conteudo de mocks_test.go contem "mockRepository"
    - Conteudo de create_test.go contem "errors.As"
    - Conteudo de repository.go contem "23505"
    - Conteudo de migration.sql contem "UNIQUE"
    - `go build ./...` passa no projeto gerado
  - files: `cmd/cli/scaffold/integration_test.go`
  - tests: TC-I-01 a TC-I-16
  - depends: TASK-9

## Parallel Batches

```
Batch 1: [TASK-1, TASK-2, TASK-3, TASK-4]   — template fixes (independent files)
Batch 2: [TASK-5, TASK-7, TASK-8]            — test templates (TASK-5 depends on TASK-1 for mock interface; TASK-7/8 independent)
Batch 3: [TASK-6]                            — use case tests (depends on TASK-5 mocks)
Batch 4: [TASK-9]                            — wiring (depends on all templates)
Batch 5: [TASK-10]                           — integration test (depends on everything)
```

File overlap analysis:
- Use case templates: TASK-1 only → exclusive
- usecase_errors template: TASK-2 only → exclusive
- handler template: TASK-3 only → exclusive
- errors/repo/migration templates: TASK-4 only → exclusive
- mocks template: TASK-5 only → exclusive
- UC test templates: TASK-6 only → exclusive
- entity/filter test templates: TASK-7 only → exclusive
- repo test template: TASK-8 only → exclusive
- embed.go + add_domain.go: TASK-9 only → exclusive
- integration_test.go: TASK-10 only → exclusive

## Validation Criteria

- [ ] `go build ./...` passa (template project)
- [ ] `make lint` passa
- [ ] `go test ./cmd/cli/...` passa (todos os testes de scaffold)
- [ ] Teste manual: `gopherplate add domain order` em projeto scaffoldado gera 28+ arquivos
- [ ] Teste manual: `go build ./...` passa no projeto com dominio adicionado
- [ ] Teste manual: `go test ./internal/domain/order/... ./internal/usecases/order/...` passa
- [ ] Grep: nenhum arquivo gerado contem `HandleError(c, span,` (3 params)
- [ ] Grep: todos os use cases gerados contem `ClassifyError`
- [ ] Grep: todos os use cases gerados contem `SpanFromContext`
- [ ] Grep: errors.go (usecases) contem `expectedErrors` e `ToAppError`
- [ ] Codigo gerado e indistinguivel em qualidade dos dominios de referencia

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Iteration 1 — Batch 1: TASK-1, TASK-2, TASK-3, TASK-4 (2026-04-12 19:00)

Executed 4 tasks in parallel via worktree agents. TASK-1: updated all 5 UC templates with span extraction, ClassifyError on every error path (9 total), toAppError returns, fmt.Errorf wrapping. TASK-2: created `usecase_errors.go.tmpl` with expectedErrors per UC + toAppError mapping (NotFound→CodeNotFound, DuplicateKey→CodeConflict). TASK-3: fixed handler template — HandleError to 2 params, removed 3 span.SetStatus calls and codes import. TASK-4: added ErrDuplicateKey to domain errors, PG 23505 detection in repo Create, UNIQUE constraint + gen_random_uuid() + DEFAULT NOW() in migration.

### Iteration 2 — Batch 2: TASK-5, TASK-7, TASK-8 (2026-04-12 20:40)

TASK-5: created `mocks_test.go.tmpl` with hand-written mockRepository (function fields, no framework) implementing all 5 Repository methods. TASK-7: created `entity_test.go.tmpl` (factory + UpdateName tests) and `filter_test.go.tmpl` (Normalize defaults + Offset calculation, table-driven). TASK-8: created `repository_postgres_test.go.tmpl` with go-sqlmock tests for Create (success/duplicate/error), FindByID (success/not found/error), List (success/empty/filtered/tx errors), Update (success/not found/errors), Delete (success/not found/error). Fixed vo.NewID → uuid.New in repo test template. Updated add_domain.go mappings (20→23 files) and integration_test.go expected templates.

### Iteration 3 — Batch 3+4: TASK-6, TASK-9 (2026-04-12 20:42)

TASK-6: created 5 UC test templates (create/get/update/delete/list_usecase_test.go.tmpl) each with success, error, and edge case tests using function-field mockRepository and AppError code assertions. TASK-9 (merged into this iteration): updated add_domain.go mappings (23→28 files), add_domain_test.go count, and integration_test.go expected templates + buildTestTemplateMappings for all 10 new templates.

### Iteration 4 — Batch 5: TASK-10 (2026-04-12 20:55)

Manual end-to-end test: rebuilt CLI, scaffolded `test-quality` project, added `order` domain (28 files). Fixed worktree merge failures — TASK-1/3/4 templates had NOT been applied. Manually rewrote all 5 UC templates (ClassifyError+toAppError+span), handler template (2-param HandleError, no span.SetStatus/codes), domain errors (DuplicateKey), repo (PG 23505), migration (UNIQUE+gen_random_uuid). Fixed update_test.go *string pointer issue. Added isParseError to usecase_errors.go.tmpl for UUID parse→INVALID_REQUEST mapping. Final result: 28 files generated, `go build` passes, all domain+UC tests pass.
