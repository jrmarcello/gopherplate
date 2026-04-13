---
name: new-endpoint
description: Scaffold a new Clean Architecture endpoint (domain → usecases → infra)
user-invocable: true
---

# /new-endpoint <method> <path> <description>

Scaffolds a new API endpoint following Clean Architecture patterns.

> **Domínio inteiro?** Para criar um domínio completo (entity + usecases + infra + migration), use:
>
> ```bash
> gopherplate add domain <name>
> ```
>
> **Endpoint padrão (CRUD-style)?** Use o CLI direto:
>
> ```bash
> gopherplate add endpoint <domain> <name>
> ```
>
> Este skill é uma alternativa para adicionar endpoints com customização guiada por Claude — útil quando o padrão CRUD não cobre o caso ou voce quer ajustes além do template.

## Example

```text
/new-endpoint POST /api/v1/users "Create a new user"
```

## Shared Patterns

Both this skill and the CLI scaffold engine (`cmd/cli/scaffold/`) follow the same patterns defined in the existing `user` and `role` domains. Use them as reference for naming, structure, and conventions.

## Implementation Order (Clean Architecture inside-out)

### 1. Domain Layer (`internal/domain/user/`)

- Add/update domain fields if needed
- Add domain errors if needed
- Add Value Objects if needed

### 2. Use Case Layer (`internal/usecases/user/`)

- Create use case file (e.g., `create.go`)
- Define interfaces in `interfaces/` subdirectory
- Create DTOs in `dto/` subdirectory
- Write unit tests with hand-written mocks

### 3. Infrastructure Layer

- **Repository**: `internal/infrastructure/db/postgres/repository/` — implement interface with sqlx
- **Handler**: `internal/infrastructure/web/handler/` — HTTP handler using `httputil.SendSuccess`/`httputil.SendError`
- **Router**: `internal/infrastructure/web/router/` — register route

### 4. DI Wiring

- Wire in `cmd/api/server.go:buildDependencies()`
- Constructor injection for required deps, `.WithCache()` for optional

### 5. Documentation

- Add Swagger annotations to handler
- Run `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`

### 6. Validation

- `make lint` — passes
- `make test` — passes
- Test endpoint manually via `api.http`

## Rules

- Never put business logic in handlers
- Handlers translate domain errors to HTTP via `handler.HandleError()`
- Use unique error variable names (`bindErr`, `createErr`, `fetchErr`)
- All responses use `httputil.SendSuccess` / `httputil.SendError`
