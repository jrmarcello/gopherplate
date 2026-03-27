# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Conventional Commits](https://www.conventionalcommits.org/).

---

## [Unreleased]

## [0.5.0] - 2026-03-27

### Added

- **Observability stack**: ELK 8.13 + OTel Collector 0.114 with metrics pipeline, ECS mapping, and auto-setup
- **Kibana dashboard**: 20-panel dashboard (SLO, HTTP, latency, cache, DB pool, logs) via `generate_dashboard.py`
- **Alerting rules**: 6 SLO-based rules (burn rate, p95, p99, 5xx, DB saturation) via `generate_rules.py`
- **Uber Fx guide**: Tutorial for migrating from manual DI to Uber Fx (`docs/guides/fx-dependency-injection.md`)
- **Multi-database guide**: Strategy guide for services with multiple databases (`docs/guides/multi-database.md`)
- **Lefthook 3-layer quality gates**: Pre-commit (fmt + vet), commit-msg (conventional commits), pre-push (lint + test)

### Changed

- Observability network integrated with app network (`go-boilerplate-network`, external)
- Same Docker Desktop project group for all containers (`name: go-boilerplate`)

## [0.4.0] - 2026-03-27

### Added

- **Claude Code DX setup**: `.claude/` with settings, 6 hooks, 3 agents, 3 rules, 11 skills
- **DevContainer**: Dockerfile (Go 1.25 + Claude Code), firewall (default-deny), zsh + Powerline10k
- **Sandbox targets**: `make sandbox`, `make sandbox-claude`, `make sandbox-shell`, firewall/ssh checks
- **Pipeline restructure**: 4 parallel PR checks (lint, vulncheck, unit, e2e), Slack notifications, coverage threshold
- **go-sqlmock tests**: 18 test cases covering all repository CRUD methods
- **Singleflight**: Cache stampede protection in GetUseCase via `golang.org/x/sync/singleflight`
- **Idempotency Redis**: `pkg/idempotency/` with Store interface + RedisStore (SHA-256 fingerprint, lock/unlock)
- **Redis pool config**: PoolSize, MinIdleConns, DialTimeout, ReadTimeout, WriteTimeout
- **NetworkPolicy**: K8s manifest restricting ingress/egress to required services only
- **Container securityContext**: readOnlyRootFilesystem, drop ALL capabilities
- **Prerequisite checks**: `make kind-*`, `make load-*`, `make docker-*` show install instructions if tools missing
- **`make run`**: Full Docker stack (infra + migrations + API) via compose profile
- **`make observability-setup`**: Auto-import dashboard + data views + alerting rules
- **`make vulncheck`**: govulncheck dependency scanning

### Changed

- **ULID → UUID v7**: Migrated ID strategy to RFC 9562, native PostgreSQL UUID type
- **DB config**: Individual vars (DB_HOST, DB_PORT, etc.) replacing single DB_DSN (SRE-approved pattern)
- **DB cluster**: Repository now uses Writer/Reader split correctly
- **Update use case**: Wrapped in transaction (fixes TOCTOU race)
- **Error handling**: Sentinel `vo.ErrInvalidID`, timing-safe service key comparison, sanitized bind errors
- **Middleware**: Rate limiter with context cancellation + smart eviction, X-Request-ID validation
- **All error messages**: Standardized to English, generic auth errors (prevent enumeration)
- **Swagger**: Default disabled, apiKey security definitions for X-Service-Name/Key
- **OTel**: TLS option via `OTEL_INSECURE`, injected business metrics (removed global singleton)
- **Server**: Error channel pattern replacing `os.Exit` in goroutine
- **Cache interface**: ISP fix (Get/Set/Delete only, no Close)
- **ConfigMaps**: Credentials removed from non-dev overlays, `sslmode=require` for prod
- **Migration**: NOT NULL on active, partial index, VARCHAR(26)→UUID, IF EXISTS in down
- **Makefile**: Categorized help, DRY with COMPOSE variable, `-include .env` auto-loading
- **Go**: 1.24 → 1.25, removed `oklog/ulid` dependency
- **Load tests**: Realistic VU counts, correct entity payload, fixed response parsing
- **DTO validation**: `binding:"required,max=255"` on name/email fields

### Removed

- `.agent/` directory (consolidated into `.claude/`)
- `internal/infrastructure/cache/` (legacy duplicate of `pkg/cache`)
- `internal/domain/shared/interfaces/` (unused)
- CORS middleware (internal service, not needed)
- `ParseEmail()` (unused, bypassed validation)
- `ErrInvalidInput` (declared but never used)
- `GetRedisTTL()` (unused in production code)
- `oklog/ulid/v2` dependency

## [0.3.0] - 2026-03-12

### Added

- **Reusable packages**: `pkg/apperror`, `pkg/httputil`, `pkg/ctxkeys`, `pkg/logutil`, `pkg/telemetry`, `pkg/cache`, `pkg/database`
- ADR-007 (reusable packages) and ADR-008 (API response format)
- `.agent/` AI toolkit with 14 agents, 23 skills, 8 workflows

### Changed

- Standardized API responses via `httputil.SendSuccess`/`httputil.SendError`
- Extracted shared code from internal to pkg/ for cross-service reuse

### Fixed

- OpenTelemetry SDK upgraded to v1.40.0 (fixes GO-2026-4394 vulnerability)

## [0.2.0] - 2026-02-15

### Added

- **Service key authentication**: `X-Service-Name` + `X-Service-Key` middleware (ADR-005)
- **ArgoCD PreSync migrations**: `cmd/migrate/` binary for K8s Job (ADR-006)
- **Cache strategy**: Redis cache-aside pattern with builder `.WithCache()` (guide)
- Comprehensive unit tests with hand-written mocks
- E2E tests with error and auth scenarios
- Rate limiting and idempotency middleware

### Changed

- Config migrated from Viper to godotenv + os (simpler, fewer deps)
- Deploy overlay renamed from `dev` to `develop`
- Docker and CI/CD standardized with people-service-registry patterns

## [0.1.0] - 2026-01-08

### Added

- **Initial project structure**: Clean Architecture (domain → usecases → infrastructure)
- **Entity CRUD**: Create, Get, List, Update, Delete with ULID IDs
- **Domain layer**: Entity aggregate with ID and Email value objects
- **Use cases**: One file per operation with interface-based DI
- **Infrastructure**: Gin HTTP handlers, sqlx PostgreSQL repository, Redis cache, OpenTelemetry
- **Manual DI**: `cmd/api/server.go:buildDependencies()` wiring
- **Database**: PostgreSQL with Goose migrations
- **Docker**: docker-compose (Postgres + Redis), multi-stage production Dockerfile (distroless)
- **Kubernetes**: Kustomize overlays (develop, homologacao, producao), Kind support
- **CI/CD**: Bitbucket Pipelines with lint, test, Docker build, ECR push, Kustomize tag update
- **Observability**: OpenTelemetry traces + structured JSON logging
- **Testing**: Unit tests + E2E with TestContainers
- **Documentation**: README, ADRs (001-004), Swagger API docs, architecture guide
- **Dev tools**: Air (hot reload), Lefthook (git hooks), golangci-lint, k6 load tests

---

> This changelog can be auto-generated from conventional commits using tools like
> [git-cliff](https://github.com/orhun/git-cliff) or
> [conventional-changelog](https://github.com/conventional-changelog/conventional-changelog).
