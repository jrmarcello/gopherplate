# Spec: Bootstrap DI Container

## Status: DRAFT

## Context

O DI (Dependency Injection) atual do boilerplate vive inteiro na funcao `buildDependencies()` dentro de `cmd/api/server.go` (~80 linhas). Todos os repos, use cases e handlers sao construidos inline numa funcao flat sem agrupamento por camada. Isso causa:

1. **Mistura de responsabilidades** — `server.go` combina lifecycle do servidor (logger, telemetry, graceful shutdown) com logica de wiring de DI
2. **Sem estrutura tipada** — dependencias sao variaveis locais sem agrupamento; nao ha como inspecionar o grafo de dependencias em compile-time
3. **Dificuldade de teste** — E2E tests (`setup_test.go`) precisam duplicar o wiring manualmente com `setupTestRouter()` e `setupTestRouterWithAuth()` locais, divergindo do codigo de producao
4. **Escalabilidade ruim** — adicionar um novo dominio requer editar uma funcao gigante e arriscar merge conflicts

**Solucao:** Extrair o wiring para um package `internal/bootstrap/` com um `Container` tipado que agrupa dependencias por camada (Repos, UseCases, Handlers). Test helpers reutilizam o mesmo container.

**Referencia:** banking-service-yield `internal/bootstrap/container.go`, `internal/bootstrap/test_helpers.go`

## Requirements

- [ ] REQ-1: **Container tipado com structs por camada**
  - GIVEN a aplicacao precisa de DI
  - WHEN `bootstrap.New(writer, reader, cache, metrics)` e chamado
  - THEN retorna um `*Container` com campos tipados: Repos, UserUseCases, RoleUseCases, Handlers
  - AND cada campo e uma struct com dependencias daquela camada

- [ ] REQ-2: **Construcao em fases (repos -> usecases -> handlers)**
  - GIVEN o Container esta sendo construido
  - WHEN `New()` executa
  - THEN chama `buildRepos()` primeiro, depois `buildUseCases()` (que consome repos), depois `buildHandlers()` (que consome use cases)
  - AND a ordem impede ciclos de dependencia

- [ ] REQ-3: **server.go simplificado**
  - GIVEN `cmd/api/server.go` tem `buildDependencies()`
  - WHEN refatorado para usar bootstrap
  - THEN `buildDependencies()` cria o container via `bootstrap.New()` e extrai handlers para `router.Dependencies`
  - AND o codigo de wiring inline (repos, use cases, handlers) e removido de server.go

- [ ] REQ-4: **Test helpers para E2E**
  - GIVEN testes E2E precisam de um router configurado
  - WHEN chamam `bootstrap.NewForTest(t, db, cache)`
  - THEN recebem um Container com o mesmo wiring de producao (mesmo DB como writer e reader, nil metrics)
  - WHEN chamam `bootstrap.SetupTestRouter(t, db, cache)`
  - THEN recebem um `*gin.Engine` pronto para testes HTTP (sem auth middleware)
  - WHEN chamam `bootstrap.SetupTestRouterWithAuth(t, db, cache, serviceKeys)`
  - THEN recebem um `*gin.Engine` com auth middleware habilitado (para testes de autenticacao)

- [ ] REQ-5: **Zero regressao**
  - GIVEN a refatoracao nao altera logica de negocio
  - WHEN todos os testes existentes sao executados
  - THEN passam sem modificacao (exceto setup de teste)

## Test Plan

### Unit Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-U-01 | REQ-1 | happy | bootstrap.New returns Container with all fields non-nil | Repos, UserUseCases, RoleUseCases, Handlers all populated |
| TC-U-02 | REQ-2 | happy | Container.Repos has all repositories populated | User, Role repos non-nil |
| TC-U-03 | REQ-2 | happy | Container.UserUseCases has all use cases populated | Create, Get, List, Update, Delete non-nil |
| TC-U-04 | REQ-2 | happy | Container.RoleUseCases has all use cases populated | Create, List, Delete non-nil |
| TC-U-05 | REQ-2 | happy | Container.Handlers has all handlers populated | User, Role non-nil |
| TC-U-06 | REQ-4 | happy | NewForTest uses same DB for writer and reader | Container returned, no error |
| TC-U-07 | REQ-4 | happy | SetupTestRouter returns working gin.Engine | Engine non-nil, routes registered |
| TC-U-08 | REQ-4 | happy | SetupTestRouterWithAuth returns router with auth middleware | Engine non-nil, auth enforced |

### E2E Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-5 | happy | All existing user E2E tests pass after refactor | zero failures |
| TC-E2E-02 | REQ-5 | happy | All existing role E2E tests pass after refactor | zero failures |
| TC-E2E-03 | REQ-4 | happy | SetupTestRouter produces router that handles requests | POST /users returns 201 |

## Design

### Architecture Decisions

**Estrutura do Container:**
```go
type Container struct {
    Repos          Repos
    UserUseCases   UserUseCases
    RoleUseCases   RoleUseCases
    Handlers       Handlers
}
```

Cada struct aninhada agrupa dependencias de uma camada. A construcao segue ordem estrita:
1. `buildRepos(writer, reader, cache)` — instancia todos os repositories
2. `buildUseCases()` — consome Repos para construir use cases (inclui `cache.NewFlightGroup()` para GetUseCase)
3. `buildHandlers(metrics)` — consome UseCases para construir handlers (metrics pode ser nil para testes)

**Decisao: businessMetrics como 4o param de New():**
- `New(writer, reader *sqlx.DB, cacheClient cache.Cache, metrics *infratelemetry.Metrics) *Container`
- `metrics` aceita nil (para testes e contextos sem OTel) — handler trata nil gracefully
- Isso evita que server.go construa o UserHandler fora do container, mantendo REQ-1

**Decisao: bootstrap importa infrastructure:**
- `internal/bootstrap` importa `internal/infrastructure/web/handler` e `internal/infrastructure/db/postgres/repository`
- Isso e aceitavel: bootstrap e o composition root, nao faz parte de nenhuma camada da Clean Architecture. E o unico pacote com permissao de importar todas as camadas.

**Decisao: test_helpers.go em non-test package:**
- `test_helpers.go` vive em `internal/bootstrap/` (nao `_test.go`) para que `tests/e2e/` possa importa-lo
- Funcoes aceitam `testing.TB` (interface generica) e chamam `t.Helper()`
- Este e um padrao aceito em Go para test-support packages (similar a `httptest` na stdlib)

**Relacao com server.go:**
- `buildDependencies()` em server.go continua existindo, mas fica minima: cria Redis, HealthChecker, IdempotencyStore, e chama `bootstrap.New()` para o resto
- Handlers do container sao extraidos para `router.Dependencies`
- Health checks e idempotency permanecem em server.go (sao cross-cutting, nao pertencem ao bootstrap de dominio)

**Test helpers:**
- `NewForTest(t, db, cache)` — cria Container usando mesmo DB como writer/reader, nil metrics, `cache.NewFlightGroup()` interno
- `SetupTestRouter(t, db, cache)` — cria gin.Engine com rotas registradas, sem auth middleware
- `SetupTestRouterWithAuth(t, db, cache, serviceKeys)` — cria gin.Engine com auth middleware habilitado

### Files to Create

- `internal/bootstrap/container.go` — Container struct + New() + build methods
- `internal/bootstrap/container_test.go` — testes do container
- `internal/bootstrap/test_helpers.go` — NewForTest + SetupTestRouter

### Files to Modify

- `cmd/api/server.go` — simplificar buildDependencies() para usar bootstrap.New()
- `tests/e2e/setup_test.go` — usar bootstrap.SetupTestRouter() em vez de helpers locais
- `tests/e2e/user_test.go` — adaptar setup para usar novo helper
- `tests/e2e/role_test.go` — adaptar setup para usar novo helper

### Dependencies

- Nenhuma dependencia externa nova

## Tasks

- [ ] TASK-1: Criar internal/bootstrap/container.go com Container + tipos + New()
  - Definir structs: Container, Repos, UserUseCases, RoleUseCases, Handlers
  - Implementar `New(writer, reader *sqlx.DB, cacheClient cache.Cache, metrics *infratelemetry.Metrics) *Container`
  - Implementar `buildRepos()`, `buildUseCases()`, `buildHandlers()` privados
  - `buildUseCases()`: cria `cache.NewFlightGroup()` interno, wires Get.WithCache().WithFlight(), Update/Delete.WithCache()
  - UserUseCases: Create, Get (com WithCache + WithFlight), List, Update (com WithCache), Delete (com WithCache)
  - RoleUseCases: Create, List, Delete
  - Handlers: User (recebe metrics — pode ser nil), Role
  - Testes usam go-sqlmock para criar *sqlx.DB fake, nil cache e nil metrics — verificam que structs sao populadas sem panic
  - files: `internal/bootstrap/container.go`, `internal/bootstrap/container_test.go`
  - tests: TC-U-01, TC-U-02, TC-U-03, TC-U-04, TC-U-05

- [ ] TASK-2: Criar internal/bootstrap/test_helpers.go
  - `NewForTest(t testing.TB, db *sqlx.DB, cache cache.Cache) *Container` — usa mesmo DB como writer e reader, nil metrics, FlightGroup interno
  - `SetupTestRouter(t testing.TB, db *sqlx.DB, cache cache.Cache) *gin.Engine` — gin.TestMode, rotas de user **E role** registradas (consolida `setupTestRouter()` + `setupRoleTestRouter()` em uma unica funcao), sem auth
  - `SetupTestRouterWithAuth(t testing.TB, db *sqlx.DB, cache cache.Cache, serviceKeys string) *gin.Engine` — idem com auth middleware habilitado
  - Nota: arquivo NAO e `_test.go` para permitir import de `tests/e2e/`. Usa `testing.TB` (interface).
  - files: `internal/bootstrap/test_helpers.go`
  - tests: TC-U-06, TC-U-07, TC-U-08
  - depends: TASK-1

- [ ] TASK-3: Refatorar cmd/api/server.go para usar bootstrap.New()
  - Remover construcao inline de repos, use cases e handlers de `buildDependencies()`
  - Chamar `c := bootstrap.New(sqlxWriter, sqlxReader, redisClient, businessMetrics)` 
  - Extrair handlers para `router.Dependencies`: `c.Handlers.User`, `c.Handlers.Role`
  - Manter: Redis setup, HealthChecker, IdempotencyStore em server.go (FlightGroup migra para dentro do container)
  - files: `cmd/api/server.go`
  - depends: TASK-1

- [ ] TASK-4: Migrar E2E tests para bootstrap.SetupTestRouter()
  - Remover `setupTestRouter()` e `setupTestRouterWithAuth()` de `user_test.go` (e onde estao definidas — NAO em setup_test.go)
  - Remover `setupRoleTestRouter()` de `role_test.go` (funcao separada que wires apenas role routes)
  - Substituir todas as chamadas por `bootstrap.SetupTestRouter()` (que registra rotas de user E role)
  - Substituir chamadas a `setupTestRouterWithAuth()` por `bootstrap.SetupTestRouterWithAuth()`
  - Manter `CleanupUsers()` e `CleanupRoles()` em setup_test.go (usam `testDB` diretamente — nao pertencem ao bootstrap)
  - Garantir que todos os testes existentes continuam passando
  - files: `tests/e2e/setup_test.go`, `tests/e2e/user_test.go`, `tests/e2e/role_test.go`
  - tests: TC-E2E-01, TC-E2E-02, TC-E2E-03
  - depends: TASK-2

## Parallel Batches

```
Batch 1: [TASK-1]           — fundacao (Container + tipos)
Batch 2: [TASK-2, TASK-3]   — parallel (test_helpers vs server.go, arquivos distintos)
Batch 3: [TASK-4]           — integracao E2E (depends de TASK-2)
```

File overlap analysis:
- `internal/bootstrap/container.go`: TASK-1 only -> exclusive
- `internal/bootstrap/test_helpers.go`: TASK-2 only -> exclusive
- `cmd/api/server.go`: TASK-3 only -> exclusive
- `tests/e2e/*.go`: TASK-4 only -> exclusive
- All files exclusive — full parallelism within batches

## Validation Criteria

- [ ] `go build ./...` passa
- [ ] `make lint` passa
- [ ] `make test` passa (todos os testes existentes + novos testes do bootstrap)
- [ ] `cmd/api/server.go:buildDependencies()` nao instancia repos/use cases/handlers diretamente
- [ ] E2E tests usam `bootstrap.SetupTestRouter()` (nao helpers locais duplicados)
- [ ] Container.New() retorna todas as dependencias populadas (compile-time safe)

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
