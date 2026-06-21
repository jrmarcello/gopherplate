# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 🎯 Princípio diretor: qualidade > velocidade > custo

**Esta máxima precede e prevalece sobre qualquer outra heurística desta documentação.**

Ao planejar, projetar, implementar, revisar ou tomar qualquer decisão neste repositório:

1. **Pergunte primeiro "qual a melhor opção?"**, não "qual a opção mais simples?" nem "qual a mais rápida?".
2. **Não economize trabalho ou retrabalho** quando isso comprometer rigor. Iterações múltiplas de self-review, refatorações largas, e specs densas são desejadas — não evitadas.
3. **"NICE TO HAVE" sob a lente de qualidade pode virar MUST.** Sempre revisitar findings de reviewers com essa lupa antes de descartar.
4. **Edge cases dismissados como "raros" exigem handling correto** — não pular cobertura porque "improvável".
5. **Triggers que sinalizam decisão pendendo pra velocidade** (e devem ser questionados): "pra simplicidade", "por enquanto", "defer pra Phase X", "user pode re-rodar", "YAGNI" usado fracamente, "implícito é suficiente", `interface{}`/`any` em vez de tipo concreto, optional+default em vez de required, error com pouca contextual info. Quando aparecerem na sua decisão, parar e perguntar: o que seria a versão **certa**? Implementar essa.
6. **Token cost só importa quando o usuário explicitamente pede pra economizar.** Default: gastar tokens à vontade pra entregar a melhor versão.
7. **Apresentar opções:** sempre liderar pela melhor (não a "pragmática"). Se houver trade-off de qualidade entre alternativas, **explicar honestamente** — nunca esconder a opção mais cara/rigorosa atrás de "essa é melhor mas mais trabalhosa".

Memory persistente em [memory/feedback_quality_first.md](../../.claude/projects/-Users-marcelojr-Development-Workspace-gopherplate/memory/feedback_quality_first.md). Implicações concretas no fluxo SDD em [.claude/rules/sdd.md](.claude/rules/sdd.md) §Princípio diretor.

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
make proto          # Generate Go stubs from proto files (buf generate)
make proto-lint     # Lint proto files (buf lint)
make help           # See all available make targets

# Harness sensors (see docs/harness.md)
make ci-local       # Simulate fresh clone + full CI pipeline in isolated worktree (pre-push gate)
make deadcode       # Detect unreachable funcs in cmd/(api|migrate) + internal/
make mutation       # Mutation testing via gremlins (internal/usecases/)
make coverage-delta # Coverage delta on changed lines vs main (diff-cover, 70% threshold)
make semgrep        # Run custom organizational rules (.semgrep/)
make semgrep-test   # Validate semgrep rules against fixtures
make buf-breaking   # Detect breaking changes in proto/ vs main
make golden-update  # Regenerate golden fixtures in tests/e2e/testdata/
make load-baseline SCENARIO=load    # Regenerate k6 perf baseline
make load-regression SCENARIO=load  # Fail if p95 degrades > 35% vs baseline
make validate-spec                  # Run deterministic SDD linter on all slug-bearing specs
make capabilities-manifest          # Regenerate docs/capabilities/MANIFEST.md
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
  - `grpc/handler/` - gRPC handlers (UserService, RoleService), translate domain errors to gRPC status codes via `toGRPCStatus()`
  - `grpc/interceptor/` - Recovery, structured logging, service key auth (constant-time compare)
  - `grpc/server.go` - gRPC server factory with OTel StatsHandler, interceptor chain, health check, reflection
  - `db/postgres/repository/` - sqlx repository implementations
  - `telemetry/` - Business-specific metrics (user counters)
- **`pkg/`** - Reusable packages shared across services:
  - `apperror/` - Structured application errors (AppError with code, message, HTTP status)
  - `httputil/` - Standardized API response helpers: core `WriteSuccess`/`WriteError` (stdlib `http.ResponseWriter`) + Gin wrappers in `httputil/httpgin/` (`SendSuccess`, `SendError`)
  - `logutil/` - Structured logging with context propagation, fanout handler, PII masking
  - `telemetry/` - OpenTelemetry setup (traces + HTTP metrics + DB pool metrics). Includes `naming.go` (`HTTPSpanName`, `DBSpanName`, `StartDBSpan` — `http.<verb>.<resource>` + `db.<op>.<table>` convention), `events.go` (`RecordEvent` helper for span events), and `span.go` with enriched `FailSpan` (adds `error.type` attribute + stack trace via `trace.WithStackTrace(true)`). See [docs/guides/observability.md](docs/guides/observability.md).
  - `cache/` - Cache interface and Redis implementation
  - `database/` - Driver-agnostic (`database/sql`) connection with Writer/Reader cluster — supports postgres, mysql, sqlite3, etc.
  - `idempotency/` - Idempotency Store interface and Redis implementation
- **`proto/`** - Protobuf definitions (contract-first API for gRPC). `buf generate` produces Go stubs in `gen/proto/` (gitignored).
- **`config/`** - Configuration loading (godotenv + env vars)
- **`cmd/api/`** - Application entrypoint and manual DI wiring in `server.go`
- **`cmd/cli/`** - Template CLI (`gopherplate`) with 8 commands: `new` (scaffold service), `add domain` / `remove domain` (manage domains), `add endpoint` / `remove endpoint` (manage endpoints with CRUD protection), `doctor` (diagnose tools/Docker/go.mod), `wiring` (auto-regenerate server.go/router.go/container.go from detected domains), `version`. Contains Cobra commands, scaffold engine, and embedded templates. See `docs/guides/template-cli.md`.

### Key Patterns

- **Manual DI**: All wiring happens in `cmd/api/server.go:buildDependencies()`. No DI framework. Wires both `user` domain (with cache/singleflight) and `role` domain (simpler, no cache). Use cases accept interfaces via constructor, optional dependencies (cache) via `.WithCache()` builder method.
- **Dual Server (HTTP + gRPC)**: Optional gRPC server alongside HTTP, controlled by `GRPC_ENABLED` (default false). Both managed by `errgroup` with coordinated graceful shutdown. gRPC handlers reuse the same use cases as HTTP handlers — zero business logic duplication. Proto definitions in `proto/`, stubs generated to `gen/proto/` via `buf generate` (`make proto`).
- **ID Strategy**: UUID v7 for all entity IDs. See `docs/adr/002-ids.md`.
- **DB Cluster**: Writer/Reader split via `pkg/database.DBCluster`. Reader is optional, falls back to writer.
- **API Response Format**: Gin handlers use `httpgin.SendSuccess(c, status, data)` and `httpgin.SendError(c, status, message)`. Core helpers (`httputil.WriteSuccess`/`httputil.WriteError`) work with stdlib `http.ResponseWriter`. Responses wrap in `{"data": ...}` or `{"errors": {"message": ...}}`.
- **Error Handling**: Domain defines pure errors (`user.ErrNotFound`, etc.). Use cases return `*apperror.AppError` via local `toAppError()`. Handler resolves generically via `errors.As()` + `codeToStatus` map — zero domain imports. Ref: ADR-009, `docs/guides/error-handling.md`.
- **Span Error Classification**: Use case classifies errors via `shared.ClassifyError()` with `[]ucshared.ExpectedError{Err, AttrKey, AttrValue}`. Expected errors (validation, not found, conflict) -> `telemetry.WarnSpan` with a semantic key from `shared.AttrKey*` constants (`app.result`, `app.validation_error`) — span stays Ok. Unexpected errors (DB timeout, infra) -> `telemetry.FailSpan` (span marked Error, `error.type` attribute + stack trace recorded). Handler never touches spans. Ref: ADR-009, `docs/guides/error-handling.md`, `docs/guides/observability.md`.
- **Service Key Auth**: Optional service-to-service authentication via `X-Service-Name` + `X-Service-Key` headers. Shared between HTTP (middleware) and gRPC (interceptor via metadata). See `docs/adr/005-service-key-auth.md`.
- **Singleflight**: GetUseCase uses `golang.org/x/sync/singleflight` to prevent cache stampede on concurrent reads for the same entity.
- **Idempotency**: Redis-backed idempotency via `pkg/idempotency.Store`, wired as optional middleware. Uses SHA-256 fingerprint + lock/unlock pattern.

### Conventions

- **Commit messages**: `type(scope): description` (feat, fix, refactor, docs, test, chore)
- **Error variable naming**: Use unique names to avoid shadowing (`parseErr`, `saveErr`, `bindErr` instead of reusing `err`)
- **Pre-commit hooks**: Lefthook runs `gofmt`, `go vet`, `golangci-lint` on staged `.go` files
- **Migrations**: Goose SQL files in `internal/infrastructure/db/postgres/migration/`
- **Tests**: Unit tests use hand-written mocks (`mocks_test.go` per package). E2E tests use TestContainers (Postgres + Redis).
- **Import rule**: Never import `infrastructure` from `domain` or `usecases`. Never import `usecases` from `domain`.
- **Error handling guide**: See `docs/guides/error-handling.md` for the practical guide on adding new errors, mapping patterns, and span classification.

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

The resources below (skills, subagents, rules, hooks) collectively form this project's **harness**
— the outer system that guides and validates agent-generated code (Fowler,
["Harness Engineering for Coding Agents"](https://martinfowler.com/articles/harness-engineering.html)).
[docs/harness.md](docs/harness.md) is the **canonical inventory** of every guide and sensor
(this file and README.md summarize; harness.md is authoritative when they diverge). For the
process of evolving the harness, see
[docs/guides/harness-self-steering.md](docs/guides/harness-self-steering.md).

Per-sensor deep-dive guides live in `docs/guides/`:

- [perf-regression.md](docs/guides/perf-regression.md) — k6 baseline + p95/p99 gate
- [mutation-testing.md](docs/guides/mutation-testing.md) — gremlins nightly report
- [golden-fixtures.md](docs/guides/golden-fixtures.md) — approved-fixtures for responses
- [semgrep-rules.md](docs/guides/semgrep-rules.md) — organizational-pattern rules
- [error-handling.md](docs/guides/error-handling.md) — ADR-009 practical guide
- [observability.md](docs/guides/observability.md) — span naming, event catalog, logs-vs-traces posture
- [spec-linter.md](docs/guides/spec-linter.md) — `tools/validate-spec` deterministic SDD linter (validator list, exit codes, grandfathering, manifest subcommand)

**Capability docs** (`docs/capabilities/`) são a **camada de verdade corrente** do projeto: cada arquivo documenta o que um subsistema garante AGORA como invariantes verificáveis. Distinct from guides (how-to) e ADRs (why, imutáveis). `MANIFEST.md` é gerado por `make capabilities-manifest`. Ver [docs/capabilities/README.md](docs/capabilities/README.md).

**`tools/validate-spec`** é o linter SDD determinístico: valida estrutura das specs antes dos review agents. Rode via `make validate-spec` (gate em `/spec` antes da self-review e em `/ralph-loop` na startup) ou `go run ./tools/validate-spec`. Exit codes: 0 = ok/warn-only, 1 = ERROR lint, 2 = tool error. Vive no módulo principal (não módulo separado). Ver [docs/guides/spec-linter.md](docs/guides/spec-linter.md).

### Skills (slash commands)

| Skill | Purpose | When to use |
| ----- | ------- | ----------- |
| `/validate` | Full validation pipeline (build, lint, tests, Kind, smoke) | Before committing any code change |
| `/validate quick` | Static validation + unit tests only | Quick feedback during development |
| `/new-endpoint` | Scaffold full Clean Architecture endpoint | Adding a new API route |
| `/fix-issue` | E2E bug fix workflow (understand → plan → implement → test) | Fixing reported bugs |
| `/migrate` | Create/run/rollback Goose migrations | Database schema changes |
| `/review` | Single-agent code review | Quick review of small changes |
| `/full-review-team` | Parallel review: architecture + security + DB + tests (Agent Team) | PRs, major changes, cross-layer work |
| `/security-review-team` | Parallel security audit with 3 specialists (Agent Team) | Releases, sensitive changes, compliance |
| `/debug-logs` | Analyze logs from Kind/Docker | Quick log-based debugging |
| `/debug-team` | Parallel bug investigation with competing hypotheses (Agent Team) | Complex bugs that resist sequential debugging |
| `/load-test` | Run k6 load tests + analyze results | Performance validation and regression |
| `/spec` | Author + self-review SDD specification, present for approval | Before implementing a new feature or complex change |
| `/ralph-loop` | Autonomous single-pass execution of an approved spec (parallel via worktrees, self-reviewed, present-before-commit) | After `/spec` approval, for autonomous implementation |
| `/spec-review` | Independent audit of implementation against specification | After `/ralph-loop` completes (or manual implementation) |
| `/learn-extract` | Triage `candidates.jsonl` into new skills, memory entries, updates, or discards | After extract runs (manually or via `/learn-nudge`) |
| `/learn-refine` | Propose merges/deprecations for similar skills/memory (presents diff, waits for approval) | When `/learn-extract` flags overlap, or after `/learn-audit-skills` |
| `/learn-nudge` | Periodic autoavaliação — surfaces TTL-based deprecation candidates, proposes consolidations, resets counter | Triggered after N spec DONEs, or manually |
| `/learn-recall <query>` | Manual FTS5 retrieval over skills/memory/patterns with filters | When you want to consult the KB without the auto-injected hook |
| `/learn-audit-skills` | One-shot non-prescriptive audit of every skill against `.claude/rules/skill-quality.md` | After a batch of skill changes or before tagging a release |

### Agent Teams and Subagents

Agent Teams enabled (`CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`). Team skills spawn 3 parallel teammates each. Use for tasks where parallel exploration adds value: reviews, audits, debugging.

- `spec-reviewer`, `security-reviewer`, `code-reviewer`, `db-analyst`, `test-reviewer` — all with persistent memory (`memory: project`). Delegate with "use a subagent to..."
- `spec-reviewer` audits SDD specs **before** implementation (gaps, ambiguity, missing TCs, layer violations, accumulator pattern). Used by `/spec` Phase 2 alongside `test-reviewer` and `code-reviewer`.
- `test-reviewer` specializes in test quality (mutation-survivor hints, error-path density, mocking discipline, test smells, TDD compliance). Use it whenever a change adds or modifies tests, or whenever you suspect under-tested behavior. Pairs with the mutation-nightly report artifact.

### Rules

Auto-applied by file pattern: Go conventions (`**/*.go`), security (`**/*`), migrations (`**/migration/**`), SDD specs (`.specs/**`).

### Hooks

Three-layer quality enforcement:

- **PreToolUse[Bash]** — `guard-bash.sh`: blocks .env staging, `git add -A`, DROP statements, `--no-verify`
- **PostToolUse[Edit|Write]** — `lint-go-file.sh`: goimports/gopls diagnostics on every Go file edit (enriched with actionable "fix by:" hints via [gopls-hints.awk](.claude/hooks/gopls-hints.awk))
- **PostToolUse[Edit|Write]** — `validate-migration.sh`: ensures Up + Down sections in migrations
- **Stop** — `stop-validate.sh`: build + fmt + vet + swagger + lint + tests gate (auto-retry with tiered validation)
- **WorktreeCreate/Remove** — automated git worktree setup and cleanup
- **Stop** — `stop-learn.sh`: records spec-DONE events into the learning-loop store; surfaces non-blocking `Learning nudge due` advisory when counter crosses `NUDGE_THRESHOLD`. Always exits 0 (best-effort).
- **UserPromptSubmit** — `user-prompt-submit-recall.sh`: queries the learning-loop FTS5 index for the incoming prompt; if matches exist, injects a `<system-reminder>` listing relevant skill/memory paths. Wraps `learn recall` in `timeout 2`.
- **PostToolUse[Edit|Write]** — `reindex-learning.sh`: incrementally re-indexes `.claude/skills/<name>/SKILL.md` or `memory/*.md` after edits. Best-effort.
- **Sourced helper** — `learn-hook-helpers.sh`: binary lookup (asdf shim), structured JSON logging to `.claude/learning/learn.log`, safe jq, db-path resolution.

### Learning loop

Closed-loop knowledge system inspired by the Hermes Agent. Five stages:

1. **Task completion** — `stop-learn.sh` records `events` when a spec hits DONE.
2. **Pattern extraction** — `learn extract` mines transcripts / specs / git / memory into `candidates.jsonl`, deterministic.
3. **Skill creation** — `/learn-extract` triage applies the rubric in `.claude/rules/skill-quality.md` to each candidate, decides new-skill / new-memory / update / discard; records every decision.
4. **Refinement** — `/learn-refine` uses FTS5 + edit distance to find merge candidates; presents diff, waits for approval; movements go to `_deprecated/` (anti-deletion).
5. **Periodic nudge** — `/learn-nudge` fires when spec-completion counter crosses threshold; proposes TTL-based deprecations and consolidations. Counter reset is deterministic (binary), not LLM.

**Closure**: `user-prompt-submit-recall.sh` queries FTS5 and injects top matches as a `<system-reminder>` on every prompt. Manual counterpart: `/learn-recall <query>`. Usage tracked via `learn track-use` from the hook, feeding TTL decisions in stage 5.

**Storage**: SQLite + FTS5 at `.claude/learning/db.sqlite` (gitignored). Skills/memory/decisions versioned; index local-only. Sanitization (AWS, OpenAI/Anthropic/GitHub/Slack tokens, SSH paths, `.env`) happens in-memory before any write.

**Binary**: `tools/learn/` (separate go.mod, pure-Go via `modernc.org/sqlite`). Operational targets: `make learn-build / learn-setup / learn-reindex / learn-stats / learn-smoke / learn-lint / learn-test`. See [docs/guides/learning-loop.md](docs/guides/learning-loop.md) and [docs/harness.md § Learning loop tooling](docs/harness.md).

### Execution Directives

1. **Prefer subagents and parallelization** — use subagents or Agent Teams for independent discovery/analysis. Merge findings before coding.
2. **Mandatory cycle** for non-trivial tasks: **Plan** → **Implement** → **Review** → **Test** → **Validate**. Do not finish without concrete validation evidence.
3. **The Review step is MANDATORY and AUTOMATIC** — after implementing, re-read the plan/spec and diff what was implemented vs what was specified (files, patterns, mappings, wrapping). Verify: all files listed in `files:` metadata were created/modified, all patterns from the Design section are followed, all error mappings and classifications are complete, no implementation gap vs the spec. Only then proceed to tests. This is NEVER skipped.
4. **Post-implementation validation** — enforced automatically by the **Stop hook** (build + fmt + vet + swagger + lint + tests). The hook blocks completion until validation passes. For the full pipeline including E2E, Kind deploy, and smoke tests, run `/validate` explicitly.
5. **SDD workflow** for complex features: `/spec` → approve → `/ralph-loop` → approve commit → `/spec-review` (optional). Specs live in `.specs/`. See `docs/guides/sdd-ralph-loop.md`.
    - **Slug prefix convention**: every new spec declares `## Slug: <UPPERCASE>` and uses `<SLUG>-REQ-N` / `<SLUG>-TC-<TYPE>-NN` IDs. Specs without a slug are grandfathered (linter skips prefix/capability checks). See `.claude/rules/sdd.md` §Naming → Slug & ID naming.
    - `/spec` runs four phases: **Author (declare slug, link impacted capability doc) → Lint (`tools/validate-spec` deterministic gate, blocks on ERROR) → Self-review (3 agents in parallel: spec-reviewer + test-reviewer + code-reviewer) → Present**. Trivial fixes applied inline; judgment calls surface as "Pontos de atenção". The skill never auto-runs `/ralph-loop`.
    - `/ralph-loop` runs five phases: **Validate (status + `tools/validate-spec` gate) → Execute (parallel via worktrees, with auto-rollback on partial failure) → Self-review (3 agents in parallel: code-reviewer + test-reviewer + security-reviewer) → Wrap-up (update impacted `docs/capabilities/*.md` + `make capabilities-manifest`) → Present → Commit (only after explicit user approval)**. Never auto-commits; never silently merges a partially-failed batch.
    - **Re-review on user feedback is MANDATORY**: if the user requests changes after either skill's Present phase, the self-review re-runs **from scratch** before re-presenting. Skipping this silently erodes the safety net.
    - **The user should never have to ask "did you validate?" or "did you review the spec?"** — those questions mean a checkpoint was skipped. See `.claude/rules/sdd.md` §Discipline Checkpoints.
6. **Parallelism** — Three types: (a) **Intra-spec**: `/spec` auto-generates Parallel Batches from task `files:` and `depends:` metadata; ralph-loop launches parallel agents with `isolation: "worktree"` for multi-task batches. (b) **Inter-spec**: independent specs run in separate worktrees. (c) Shared files classified as exclusive, shared-additive (accumulator pattern), or shared-mutative (must serialize).
7. **Agent worktree cleanup is MANUAL** — when launching `Agent` with `isolation: "worktree"`, the runtime does NOT auto-cleanup worktrees if the agent made changes. After merging files from a worktree, ALWAYS run `git worktree remove <path> --force && git worktree prune`. Orphan worktrees accumulate fast and break IDE Go tooling (each worktree has its own go.mod). The `WorktreeRemove` hook only fires on explicit removal.
