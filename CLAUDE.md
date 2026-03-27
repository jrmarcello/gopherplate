# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go boilerplate/template for microservices, part of the Appmax ecosystem. Uses Clean Architecture with PostgreSQL, Redis cache, and OpenTelemetry observability. Hosted on Bitbucket, deployed to AWS EKS via ArgoCD with Kustomize overlays.

This project serves as a **starter template** — clone it and rename `entity_example` to your domain entity.

## Common Commands

```bash
make setup          # Full setup: install tools + start Docker + run migrations
make dev            # Start server with hot reload (air)
make lint           # Run go vet + gofmt
make lint-full      # Run golangci-lint (same as CI)
make test           # Run all tests: go test ./... -v
make test-unit      # Unit tests only: go test ./internal/... -v
make test-e2e       # E2E tests (requires Docker): go test ./tests/e2e/... -v -count=1
make test-coverage  # Generate HTML coverage report
make docker-up      # Start infrastructure containers (Postgres, Redis)
make docker-down    # Stop infrastructure containers
make migrate-up     # Run database migrations
make migrate-create NAME=add_something  # Create new migration
make kind-setup     # Full Kind cluster setup (cluster + db + migrate + deploy)
make help           # See all available make targets
```

Run a single test file or function:

```bash
go test ./internal/usecases/entity_example/ -run TestCreateUseCase -v
```

Generate Swagger docs (required before CI lint passes):

```bash
swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
```

## Architecture

**Clean Architecture** with strict dependency rule: `domain` <- `usecases` <- `infrastructure`

### Layer Structure

- **`internal/domain/entity_example/`** - Entities, value objects (ID, Email), domain errors. Zero external dependencies.
- **`internal/usecases/entity_example/`** - One file per use case (create.go, get.go, update.go, delete.go, list.go). Each use case defines its own interfaces in `interfaces/` subdirectory. DTOs live in `dto/` subdirectory.
- **`internal/infrastructure/`** - All external concerns:
  - `web/handler/` - Gin HTTP handlers, translate domain errors to HTTP responses via `httputil.SendSuccess`/`httputil.SendError`
  - `web/router/` - Route registration, middleware wiring
  - `web/middleware/` - Logger, metrics, rate limiting, idempotency, service key auth
  - `db/postgres/repository/` - sqlx repository implementations
  - `telemetry/` - Business-specific metrics (entity counters)
- **`pkg/`** - Reusable packages shared across services:
  - `apperror/` - Structured application errors (AppError with code, message, HTTP status)
  - `httputil/` - Standardized API response helpers (SendSuccess, SendError)
  - `logutil/` - Structured logging with context propagation, fanout handler, PII masking
  - `telemetry/` - OpenTelemetry setup (traces + HTTP metrics + DB pool metrics)
  - `cache/` - Cache interface and Redis implementation
  - `database/` - PostgreSQL connection with Writer/Reader cluster
  - `idempotency/` - Idempotency Store interface and Redis implementation
- **`config/`** - Configuration loading (godotenv + env vars)
- **`cmd/api/`** - Application entrypoint and manual DI wiring in `server.go`

### Key Patterns

- **Manual DI**: All wiring happens in `cmd/api/server.go:buildDependencies()`. No DI framework. Use cases accept interfaces via constructor, optional dependencies (cache) via `.WithCache()` builder method.
- **ID Strategy**: UUID v7 for all entity IDs. See `docs/adr/002-ids.md`.
- **DB Cluster**: Writer/Reader split via `pkg/database.DBCluster`. Reader is optional, falls back to writer.
- **API Response Format**: Always use `httputil.SendSuccess(c, status, data)` and `httputil.SendError(c, status, message)`. Responses wrap in `{"data": ...}` or `{"errors": {"message": ...}}`.
- **Error Handling**: Domain defines pure errors (`entity.ErrNotFound`, etc.). `pkg/apperror.AppError` provides structured errors with HTTP status. Handlers translate errors via `handler.HandleError()`. Never return HTTP concepts from domain layer.
- **Service Key Auth**: Optional service-to-service authentication via `X-Service-Name` + `X-Service-Key` headers. See `docs/adr/005-service-key-auth.md`.
- **Singleflight**: GetUseCase uses `golang.org/x/sync/singleflight` to prevent cache stampede on concurrent reads for the same entity.
- **Idempotency**: Redis-backed idempotency via `pkg/idempotency.Store`, wired as optional middleware. Uses SHA-256 fingerprint + lock/unlock pattern.

### Conventions

- **Commit messages**: `type(scope): description` (feat, fix, refactor, docs, test, chore)
- **Error variable naming**: Use unique names to avoid shadowing (`parseErr`, `saveErr`, `bindErr` instead of reusing `err`)
- **Pre-commit hooks**: Lefthook runs `gofmt`, `go vet`, `golangci-lint` on staged `.go` files
- **Migrations**: Goose SQL files in `internal/infrastructure/db/postgres/migration/`
- **Tests**: Unit tests use hand-written mocks (`mocks_test.go` per package). E2E tests use TestContainers (Postgres + Redis).
- **Import rule**: Never import `infrastructure` from `domain` or `usecases`. Never import `usecases` from `domain`.

## CI Pipeline (Bitbucket)

PRs run: `swag init` -> `golangci-lint run` -> `go test ./internal/...` with coverage. Branch pushes to `develop`/`main` build Docker image, push to ECR, and update Kustomize image tags.

## MCP — Context7

Context7 is installed as a global MCP plugin. It fetches up-to-date documentation directly from library sources.

**Usage directives:**

- Always consult Context7 before writing code that depends on external library APIs (Gin, sqlx, Goose, golangci-lint, OpenTelemetry, etc.)
- Use `resolve-library-id` to find the library ID, then `query-docs` to fetch the docs
- Maximum 3 calls per question (Context7 rate limit)
- Do NOT include sensitive data (API keys, passwords) in the `query` parameter
- Prioritize results with source reputation "High" and high benchmark score

**Pre-resolved library IDs:**

| Library | Context7 ID |
| ------- | ----------- |
| golangci-lint | `/golangci/golangci-lint` |
| Gin | `/gin-gonic/gin` |
| Testify | `/stretchr/testify` |

**Resolve on-demand:** sqlx, Goose, OpenTelemetry Go, go-sqlmock, go-redis, Swag, Lefthook, Air, TestContainers Go

**When NOT to use Context7:** Go stdlib — use built-in knowledge instead.

## Claude Code Resources

### Skills (slash commands)

| Skill | Purpose | When to use |
| ----- | ------- | ----------- |
| `/validate` | Full validation pipeline (build, lint, tests, Kind, smoke) | Before committing any code change |
| `/validate quick` | Static validation + unit tests only | Quick feedback during development |
| `/new-endpoint` | Scaffold full Clean Architecture endpoint | Adding a new API route |
| `/fix-issue` | E2E bug fix workflow (understand → plan → implement → test) | Fixing reported bugs |
| `/migrate` | Create/run/rollback Goose migrations | Database schema changes |
| `/review` | Single-agent code review | Quick review of small changes |
| `/full-review-team` | Parallel review: architecture + security + DB (Agent Team) | PRs, major changes, cross-layer work |
| `/security-review-team` | Parallel security audit with 3 specialists (Agent Team) | Releases, sensitive changes, compliance |
| `/debug-logs` | Analyze logs from Kind/Docker | Quick log-based debugging |
| `/debug-team` | Parallel bug investigation with competing hypotheses (Agent Team) | Complex bugs that resist sequential debugging |
| `/load-test` | Run k6 load tests + analyze results | Performance validation and regression |

### Agent Teams and Subagents

Agent Teams enabled (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`). Team skills spawn 3 parallel teammates each. Use for tasks where parallel exploration adds value: reviews, audits, debugging.

- `security-reviewer`, `code-reviewer`, `db-analyst` — all with persistent memory (`memory: project`). Delegate with "use a subagent to..."

### Rules

Auto-applied by file pattern: Go conventions (`**/*.go`), security (`**/*`), migrations (`**/migration/**`).

### Hooks

Three-layer quality enforcement:

- **PreToolUse[Bash]** — `guard-bash.sh`: blocks .env staging, `git add -A`, DROP statements, `--no-verify`
- **PostToolUse[Edit|Write]** — `lint-go-file.sh`: goimports/gopls diagnostics on every Go file edit
- **PostToolUse[Edit|Write]** — `validate-migration.sh`: ensures Up + Down sections in migrations
- **Stop** — `stop-validate.sh`: build + fmt + vet + swagger + lint + tests gate (auto-retry with tiered validation)
- **WorktreeCreate/Remove** — automated git worktree setup and cleanup

### Execution Directives

1. **Prefer subagents and parallelization** — use subagents or Agent Teams for independent discovery/analysis. Merge findings before coding.
2. **Mandatory cycle** for non-trivial tasks: **Plan** → **Implement** → **Test** → **Validate**. Do not finish without concrete validation evidence.
3. **Post-implementation validation** — enforced automatically by the **Stop hook** (build + fmt + vet + swagger + lint + tests). The hook blocks completion until validation passes. For the full pipeline including E2E, Kind deploy, and smoke tests, run `/validate` explicitly.
