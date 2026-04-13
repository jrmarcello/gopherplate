# Changelog

Todas as mudanças notáveis deste projeto estão documentadas aqui.

Formato baseado em [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Commits seguem [Conventional Commits](https://www.conventionalcommits.org/).

---

## [0.10.0] - 2026-04-12

### Documentação

- Update CONTRIBUTING.md with SDD workflow, error handling, and load tests

### Funcionalidades

- **load-tests**: Modularize k6 load tests into helpers/users/roles/main

### Manutenção

- **release**: V0.10.0 [skip ci]

### Refatoração

- Remove error-handling specification and modularize load tests

## [0.9.0] - 2026-04-12

### Funcionalidades

- **dx**: Enhance review process, coverage rules, and parallelism from yield

### Refatoração

- **di**: Extract DI wiring into typed bootstrap container

## [0.8.0] - 2026-04-12

### Refatoração

- **error-handling**: Structured errors in use cases with span classification

## [0.7.0] - 2026-04-12

### Correções

- **deps**: Upgrade docker/docker v28.5.1 → v28.5.2 (2 HIGH vulnerabilities)
- Consolidate migrations, fix /entities → /users routes
- **sandbox**: Guard git config against empty GIT_AUTHOR_EMAIL
- **sandbox**: Ignore non-zero exit code from interactive shell and claude
- **hooks**: Inherit git identity in worktree-create (host → global → env → warn)
- **sandbox**: Pass host git identity to container for commits
- **deploy**: Enable service key auth in Kind cluster (develop overlay)
- **hooks**: Use -show verbose in pre-push vulncheck
- **vulncheck**: Use -show verbose to catch package-level vulnerabilities
- **deps**: Upgrade grpc v1.78.0 → v1.79.3 to fix GO-2026-4762 (auth bypass)
- **settings**: Comment out sensitive file read permissions in settings.json
- **config**: Enable pool settings by default in .env.example
- **observability**: Use same compose project name for Docker Desktop grouping
- **observability**: Disable ES disk watermarks for local dev
- **load**: Rewrite k6 scenarios for boilerplate entity model
- **dev**: Fix make run and air build to compile all cmd/api files
- Resolve Priority 3+4 findings (MEDIUM/LOW) from full review
- Resolve Priority 2 findings (HIGH/SHOULD FIX) from full review
- Resolve Priority 1 findings from full review
- **deps**: Upgrade otel/sdk to v1.40.0 to fix GO-2026-4394 vulnerability

### Documentação

- Migrate all references from Bitbucket to GitHub
- **cli**: Fix template-cli guide to match actual implementation
- **readme**: Update Quick Start and features to highlight Template CLI
- Add gRPC guide and update documentation index
- Add parallelism and merge strategy references to CLAUDE.md and README
- **readme**: Add AI DX tools section with SDD + Ralph Loop references
- Add complementary modules section and recommended libraries guide
- **readme**: Reorganize sections by developer journey
- Replace all entity/entity_example references with user/role across project
- Add make changelog to help, README commands and CONTRIBUTING PR checklist
- **adr-003**: Add SERVICE_KEYS_ENABLED, OTEL_INSECURE, SWAGGER_HOST and fix ConfigMap example
- **readme**: Add Sandbox (DevContainer) section with usage, tools and firewall docs
- **readme**: Replace PII acronym with clear Portuguese description and LGPD context
- **readme**: Remove stale presentation.md reference from guides table
- **readme**: Consolidate README with presentation, add FAQ, roadmap and project structure
- **readme**: Add DX and AI integration highlights to intro
- **readme**: Add project intro with quick start and presentation link
- Move presentation.md to docs/guides/
- **presentation**: Fix Portuguese accents and punctuation throughout
- Add presentation guide for gopherplate showcase
- Add missing guide references, fix env example, document coverage threshold
- Fix stale lint-full references and cache import paths
- **claude**: Remove rate limiting reference from middleware list
- **contributing**: Rewrite with issue tracker, dev workflow, and architecture refs
- Add CHANGELOG.md with full project history
- **guides**: Add multi-database strategy guide
- **guides**: Add Uber Fx dependency injection guide
- Comprehensive documentation update to match current codebase
- **agents**: Add .agent/ toolkit references to CLAUDE.md and AGENTS.md
- **readme**: Update with pkg/, new ADRs, commands and features
- Fix markdown lint errors
- Fix broken links to renamed ADRs
- Rename ADRs to use numeric prefixes (001-006)
- Standardize ADRs format (001-006)
- Update README and AGENTS guidelines
- Add cache strategy guide with cache-aside pattern explanation
- Reorganize documentation structure
- Update roadmap with future improvements
- Rewrite README as reusable boilerplate template
- Comprehensive rewrite of AGENTS.md with AI guidelines
- Add documentation references to AGENTS.md
- Reorganize - move guides to dedicated folder, remove empty app/
- Standardize all ADRs with consistent format
- Improve clean architecture ADR with pillars and implementation
- Refine clean arch adr with implementation details
- Refactor arch docs to ADRs
- Rename decisions to ADRs and add config strategy
- Fix markdown table format in README
- Update architecture diagram and description for entity domain
- Remove MIT license references and regenerate swagger
- Add uuid v7 comparison to ulid adr
- Complete roadmap items
- Update README and regenerate Swagger for entity domain
- Add project documentation and tooling
- Add Swagger API documentation

### Funcionalidades

- **dx**: Add TDD, Test Plan, parallel execution, and span classification to SDD workflow
- **cli**: Add boilerplate CLI for scaffolding services and domains
- **dx**: Add parallelism detection and hybrid B+C merge strategy to SDD
- **dx**: Add SDD + Ralph Loop workflow for AI-assisted development
- **wiring**: Integrate role domain into server and router
- **domain**: Add role entity as second domain (multi-domain DI example)
- **sandbox**: Add Docker socket mount for make commands inside container
- **server**: Add Swagger descriptions, MaxHeaderBytes and telemetry graceful degradation
- **pkg**: Wire PII masking into slog pipeline via MaskingHandler
- **config**: Add IdempotencyConfig, GinMode, MaxBodySize and Validate()
- **deploy**: Add PodDisruptionBudget and startup probe
- **auth**: Add fail-closed service key auth (SERVICE_KEYS_ENABLED)
- **health**: Merge health checker package branch
- **health**: Add reusable health checker package with dependency registration
- **observability**: Upgrade ELK stack, add metrics pipeline, dashboard and alerting
- **docker**: Add make run for full Docker stack (infra + migrate + API)
- **dx**: Add Claude Code DX setup with devcontainer, hooks, agents and skills
- **migrations**: Implement ArgoCD PreSync Job strategy
- **auth**: Add service key authentication middleware
- **config**: Implement viper for flexible configuration
- **app**: Add application bootstrap and server
- **api**: Add HTTP handlers and router for Entity CRUD
- **infra**: Add Redis cache and OpenTelemetry integration
- **infra**: Add PostgreSQL repository and migration for Entity
- **usecases**: Add CRUD use cases for Entity
- **domain**: Add Entity aggregate with ID and Email value objects
- **config**: Add application configuration with env support

### Manutenção

- Remove outdated template CLI specification document
- **release**: Prepare for version 0.6.0 with updated changelog and Makefile enhancements
- **deps**: Remove unused dependencies and update indirect dependencies
- **deploy**: Toggle Swagger per environment and add @Security annotations
- **claude**: Remove .env read deny rules from settings
- **config**: Add GinMode, MaxBodySize and Idempotency vars to .env.example
- **sandbox**: Add sandbox-clean to remove container, image and volumes
- **build**: Unify air output to bin/ (remove tmp/ directory)
- **build**: Include migrate binary in make build output
- **tests**: Consolidate all test artifacts under tests/ directory
- **changelog**: Add git-cliff config and make changelog command
- **deps**: Go mod tidy (move x/time to indirect)
- **ci**: Exclude bootstrap/wiring packages from coverage, raise threshold to 60%
- **docs**: Add markdownlint config and fix all markdown issues
- **middleware**: Remove unused rate limiter
- **api**: Remover arquivo de configuração de API obsoleto
- **lefthook**: Rewrite git hooks with 3-layer quality gates
- **makefile**: Add docker prerequisite check
- **makefile**: Add prerequisite checks for all external tool targets
- **makefile**: Add kind/kubectl prerequisite check for kind-* targets
- **deps**: Upgrade Go directive to 1.25 (align with CI and people)
- **dx**: Add review report to gitignore, fix sandbox port conflict, improve skill output
- **infra**: Standardize Docker and CI/CD with people-service-registry
- Stop tracking roadmap.md
- Refactor bitbucket pipelines with YAML anchors and variables
- Refactor Makefile with configurable project variables
- Unify config with root .env and viper
- Setup config examples and gitignore
- **deploy**: Add Docker and Kubernetes deployment configs
- Add development config files (gitignore, air, golangci, lefthook)
- Initialize Go module as ms-boilerplate-go

### Refatoração

- Migrate module path to GitHub and upgrade golangci-lint to v2
- Rename entity_example to user across entire project
- **pkg**: Decouple database from sqlx and PostgreSQL driver
- **pkg**: Decouple telemetry from hardcoded gRPC exporters
- **pkg**: Decouple logutil PII masking from hardcoded BR fields
- **pkg**: Separate Gin implementation from httputil response helpers
- **pkg**: Separate Redis implementations into subpackages
- **cache**: Move singleflight from usecases to pkg/cache
- **middleware**: Delete pkg/ctxkeys, propagate CallerService via LogContext
- **logutil**: Merge fanout handler, masking, Logger interface
- **logutil**: Add fanout handler, PII masking, Logger interface, context propagation
- **apperror**: Merge remove HTTPStatus branch
- **apperror**: Remove HTTPStatus, move HTTP mapping to handler layer
- **cleanup**: Remove dead code, unused constants, rename database file
- **observability**: Integrate with app network (external)
- **dx**: Remove .agent/ directory, consolidate into .claude/
- **cache**: Add pool config, singleflight protection, remove legacy impl
- **ids**: Migrate from ULID to UUID v7 (RFC 9562)
- **ci**: Restructure pipeline with parallel steps, vulncheck, and Slack notifications
- **makefile**: Improve DX with categorized help, sandbox targets and vulncheck
- **pkg**: Extract reusable packages and standardize API responses
- Fix imports and tests after entity renaming
- **cache**: Move interface to shared and add Ping method
- Rename deploy/overlays/dev to deploy/overlays/develop
- **config**: Migrate from Viper to godotenv + os
- **deploy**: Rename dev-local overlay to dev
- Remove all people/person references from project
- Keep legacy person domain as reference (deprecated)

### Testes

- Comprehensive test coverage for multi-domain architecture
- **cache**: Add singleflight unit tests (deduplication, error, independent keys)
- **cache**: Add comprehensive unit tests with miniredis
- Add comprehensive unit tests for middleware and pkg packages
- **repository**: Add go-sqlmock unit tests for all CRUD methods
- **e2e**: Improve test coverage with error and auth scenarios
- Add comprehensive unit tests with mocks
- Add e2e tests for Entity CRUD

---

> Gerado automaticamente com [git-cliff](https://github.com/orhun/git-cliff).
> Para changelog curado manualmente, edite `CHANGELOG.md`.
