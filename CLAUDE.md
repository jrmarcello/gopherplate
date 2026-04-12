# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gopherplate/template for microservices, part of the Appmax ecosystem. Uses Clean Architecture with PostgreSQL, Redis cache, and OpenTelemetry observability. Hosted on GitHub, deployed to AWS EKS via ArgoCD with Kustomize overlays.

This project serves as a **starter template** with two example domains: `user` (full CRUD with cache, singleflight, idempotency) and `role` (simpler multi-domain DI example). Clone it, use them as reference, and rename to your domain.

## Common Commands

```bash
make setup          # Full setup: install tools + start Docker + run migrations
make dev            # Start server with hot reload (air)
make lint           # Run golangci-lint + gofmt
make vulncheck      # Run govulncheck for dependency vulnerabilities
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
go test ./internal/usecases/user/ -run TestCreateUseCase -v
```

Generate Swagger docs (required before CI lint passes):

```bash
swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
```

## Architecture

**Clean Architecture** with strict dependency rule: `domain` <- `usecases` <- `infrastructure`

### Layer Structure

- **`internal/domain/user/`** - User entity, value objects (ID, Email), domain errors. Zero external dependencies.
- **`internal/domain/role/`** - Role entity (simpler second domain). Zero external dependencies.
- **`internal/usecases/user/`** - One file per use case (create.go, get.go, update.go, delete.go, list.go). Each use case defines its own interfaces in `interfaces/` subdirectory. DTOs live in `dto/` subdirectory.
- **`internal/usecases/role/`** - Role use cases (create.go, list.go, delete.go). Simpler multi-domain DI example.
- **`internal/infrastructure/`** - All external concerns:
  - `web/handler/` - Gin HTTP handlers, translate domain errors to HTTP responses via `httpgin.SendSuccess`/`httpgin.SendError`
  - `web/router/` - Route registration, middleware wiring
  - `web/middleware/` - Logger, metrics, idempotency, service key auth
  - `db/postgres/repository/` - sqlx repository implementations
  - `telemetry/` - Business-specific metrics (user counters)
- **`pkg/`** - Reusable packages shared across services:
  - `apperror/` - Structured application errors (AppError with code, message, HTTP status)
  - `httputil/` - Standardized API response helpers: core `WriteSuccess`/`WriteError` (stdlib `http.ResponseWriter`) + Gin wrappers in `httputil/httpgin/` (`SendSuccess`, `SendError`)
  - `logutil/` - Structured logging with context propagation, fanout handler, PII masking
  - `telemetry/` - OpenTelemetry setup (traces + HTTP metrics + DB pool metrics)
  - `cache/` - Cache interface and Redis implementation
  - `database/` - Driver-agnostic (`database/sql`) connection with Writer/Reader cluster — supports postgres, mysql, sqlite3, etc.
  - `idempotency/` - Idempotency Store interface and Redis implementation
- **`config/`** - Configuration loading (godotenv + env vars)
- **`cmd/api/`** - Application entrypoint and manual DI wiring in `server.go`
- **`cmd/cli/`** - Template CLI (`gopherplate`) for scaffolding new services and domains. Contains Cobra commands, scaffold engine, and embedded templates. See `docs/guides/template-cli.md`.

### Key Patterns

- **Manual DI**: All wiring happens in `cmd/api/server.go:buildDependencies()`. No DI framework. Wires both `user` domain (with cache/singleflight) and `role` domain (simpler, no cache). Use cases accept interfaces via constructor, optional dependencies (cache) via `.WithCache()` builder method.
- **ID Strategy**: UUID v7 for all entity IDs. See `docs/adr/002-ids.md`.
- **DB Cluster**: Writer/Reader split via `pkg/database.DBCluster`. Reader is optional, falls back to writer.
- **API Response Format**: Gin handlers use `httpgin.SendSuccess(c, status, data)` and `httpgin.SendError(c, status, message)`. Core helpers (`httputil.WriteSuccess`/`httputil.WriteError`) work with stdlib `http.ResponseWriter`. Responses wrap in `{"data": ...}` or `{"errors": {"message": ...}}`.
- **Error Handling**: Domain defines pure errors (`user.ErrNotFound`, etc.). Use cases return `*apperror.AppError` via local `toAppError()`. Handler resolves generically via `errors.As()` + `codeToStatus` map — zero domain imports. Ref: ADR-009, `docs/guides/error-handling.md` (created by spec `error-handling-refactor`).
- **Span Error Classification**: Use case classifies errors via `shared.ClassifyError()`. Expected errors (validation, not found, conflict) -> `telemetry.WarnSpan` (span stays Ok). Unexpected errors (DB timeout, infra) -> `telemetry.FailSpan` (span marked Error). Handler never touches spans. Ref: ADR-009, `docs/guides/error-handling.md` (created by spec `error-handling-refactor`).
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
- **Error handling guide**: See `docs/guides/error-handling.md` for the practical guide on adding new errors, mapping patterns, and span classification (created by spec `error-handling-refactor`).

## CI Pipeline (GitHub Actions)

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

**Resolve on-demand:** sqlx (wrapper over `database/sql`), Goose, OpenTelemetry Go, go-sqlmock, go-redis, Swag, Lefthook, Air, TestContainers Go. Note: primary DB abstraction is `database/sql` (sqlx is the repository-layer wrapper).

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
| `/spec` | Create SDD specification (requirements, design, tasks) | Before implementing a new feature or complex change |
| `/ralph-loop` | Autonomous task-by-task execution from a spec | After `/spec` approval, for autonomous implementation |
| `/spec-review` | Review implementation against specification | After `/ralph-loop` completes or manual implementation |

### Agent Teams and Subagents

Agent Teams enabled (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`). Team skills spawn 3 parallel teammates each. Use for tasks where parallel exploration adds value: reviews, audits, debugging.

- `security-reviewer`, `code-reviewer`, `db-analyst` — all with persistent memory (`memory: project`). Delegate with "use a subagent to..."

### Rules

Auto-applied by file pattern: Go conventions (`**/*.go`), security (`**/*`), migrations (`**/migration/**`), SDD specs (`.specs/**`).

### Hooks

Three-layer quality enforcement:

- **PreToolUse[Bash]** — `guard-bash.sh`: blocks .env staging, `git add -A`, DROP statements, `--no-verify`
- **PostToolUse[Edit|Write]** — `lint-go-file.sh`: goimports/gopls diagnostics on every Go file edit
- **PostToolUse[Edit|Write]** — `validate-migration.sh`: ensures Up + Down sections in migrations
- **Stop** — `ralph-loop.sh`: checks spec task progress, returns exit 2 to continue autonomous execution (transparent when no loop active)
- **Stop** — `stop-validate.sh`: build + fmt + vet + swagger + lint + tests gate (auto-retry with tiered validation; skipped during active ralph-loop)
- **WorktreeCreate/Remove** — automated git worktree setup and cleanup

### Execution Directives

1. **Prefer subagents and parallelization** — use subagents or Agent Teams for independent discovery/analysis. Merge findings before coding.
2. **Mandatory cycle** for non-trivial tasks: **Plan** → **Implement** → **Review** → **Test** → **Validate**. Do not finish without concrete validation evidence.
3. **The Review step is MANDATORY and AUTOMATIC** — after implementing, re-read the plan/spec and diff what was implemented vs what was specified (files, patterns, mappings, wrapping). Verify: all files listed in `files:` metadata were created/modified, all patterns from the Design section are followed, all error mappings and classifications are complete, no implementation gap vs the spec. Only then proceed to tests. This is NEVER skipped.
4. **Post-implementation validation** — enforced automatically by the **Stop hook** (build + fmt + vet + swagger + lint + tests). The hook blocks completion until validation passes. For the full pipeline including E2E, Kind deploy, and smoke tests, run `/validate` explicitly.
5. **SDD workflow** for complex features: `/spec` → approve → `/ralph-loop` → `/spec-review`. Specs live in `.specs/`. The ralph-loop uses the Stop hook (exit code 2) to iterate task-by-task within the same session. See `docs/guides/sdd-ralph-loop.md`.
6. **Parallelism** — Three types: (a) **Intra-spec**: `/spec` auto-generates Parallel Batches from task `files:` and `depends:` metadata; ralph-loop launches parallel agents with `isolation: "worktree"` for multi-task batches. (b) **Inter-spec**: independent specs run in separate worktrees. (c) Shared files classified as exclusive, shared-additive (accumulator pattern), or shared-mutative (must serialize).
