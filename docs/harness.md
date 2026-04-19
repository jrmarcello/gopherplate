# Harness map

This document is the **inventory of the gopherplate harness**: every guide, sensor, and
control point that shapes how code is produced, reviewed, and validated in this project.

It follows the taxonomy from Martin Fowler's article
["Harness Engineering for Coding Agents"](https://martinfowler.com/articles/harness-engineering.html).
Use it as the single place to answer "where does this rule live?" or "which part of the harness
catches X?".

> For the **process** of evolving this harness (when to add a new guide or sensor, how to write a
> gap note), see [harness-self-steering.md](guides/harness-self-steering.md).

## What is a harness

Fowler defines a harness as **everything in an AI agent except the model itself** — the outer
system engineers build around a coding agent (or a human) to increase confidence in the output.
It operates as a cybernetic governor combining two kinds of controls:

- **Guides (feedforward)** — anticipate behavior before action. Examples: architecture docs,
  conventions, scaffolders, linter configs.
- **Sensors (feedback)** — observe after action and drive self-correction. Examples: tests,
  linters, type checkers, review skills.

Each control has an **execution type**:

- **Computational** — deterministic, fast (ms–s), cheap. Examples: tests, linters, type
  checkers, structural analysis.
- **Inferential** — semantic analysis via AI (LLM). Slower, non-deterministic, richer.
  Examples: review subagents, debug teams, spec reviewers.

Fowler's three regulation categories:

- **Maintainability harness** — internal code quality (duplication, complexity, coverage, style).
- **Architecture fitness harness** — architectural characteristics (performance, dependency
  boundaries, observability standards).
- **Behavior harness** — functional correctness (contracts, response shapes, business rules).

We add a fourth pragmatic category, **meta**, for controls that govern the harness itself
(worktree hooks, spec state files, ralph-loop execution).

## Inventory

Each row is an artifact (or coherent group) that currently exists in the repo. Classification:

- **Type**: `guide` (feedforward) or `sensor` (feedback).
- **Execution**: `computational` (C) or `inferential` (I).
- **Category**: `maint` / `arch-fitness` / `behavior` / `meta`.
- **Stage**: `pre-commit` / `on-edit` / `stop-hook` / `CI` / `post-integration` / `continuous` /
  `scaffold-time` / `review-time`.
- **Implementation**: file, skill, hook, or config path.

### Documental guides

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| Project instructions | guide | I | meta | on-read | [CLAUDE.md](../CLAUDE.md) |
| Go conventions | guide | I | maint | on-read | [.claude/rules/go-conventions.md](../.claude/rules/go-conventions.md) |
| Migration rules | guide | I | maint | on-read | [.claude/rules/migrations.md](../.claude/rules/migrations.md) |
| SDD rules | guide | I | meta | on-read | [.claude/rules/sdd.md](../.claude/rules/sdd.md) |
| Security rules | guide | I | behavior | on-read | [.claude/rules/security.md](../.claude/rules/security.md) |
| Architecture guide | guide | I | arch-fitness | on-read | [docs/guides/architecture.md](guides/architecture.md) |
| Error handling guide | guide | I | behavior | on-read | [docs/guides/error-handling.md](guides/error-handling.md) |
| gRPC guide | guide | I | behavior | on-read | [docs/guides/grpc.md](guides/grpc.md) |
| Cache guide | guide | I | arch-fitness | on-read | [docs/guides/cache.md](guides/cache.md) |
| Kubernetes guide | guide | I | arch-fitness | on-read | [docs/guides/kubernetes.md](guides/kubernetes.md) |
| Multi-database guide | guide | I | arch-fitness | on-read | [docs/guides/multi-database.md](guides/multi-database.md) |
| Recommended libraries | guide | I | maint | on-read | [docs/guides/recommended-libraries.md](guides/recommended-libraries.md) |
| SDD ralph-loop guide | guide | I | meta | on-read | [docs/guides/sdd-ralph-loop.md](guides/sdd-ralph-loop.md) |
| Template CLI guide | guide | I | meta | on-read | [docs/guides/template-cli.md](guides/template-cli.md) |
| ADRs (001–009) | guide | I | arch-fitness | on-read | [docs/adr/](adr/) |

### Skills (slash commands)

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| `/validate` | sensor | C | maint | stop-hook / on-demand | [.claude/skills/validate/](../.claude/skills/validate/) |
| `/new-endpoint` | guide | I | arch-fitness | scaffold-time | [.claude/skills/new-endpoint/](../.claude/skills/new-endpoint/) |
| `/fix-issue` | guide+sensor | I | behavior | review-time | [.claude/skills/fix-issue/](../.claude/skills/fix-issue/) |
| `/migrate` | guide | C | maint | scaffold-time | [.claude/skills/migrate/](../.claude/skills/migrate/) |
| `/review` | sensor | I | maint | review-time | [.claude/skills/review/](../.claude/skills/review/) |
| `/full-review-team` | sensor | I | maint+arch+behavior | review-time | [.claude/skills/full-review-team/](../.claude/skills/full-review-team/) |
| `/security-review-team` | sensor | I | behavior | review-time | [.claude/skills/security-review-team/](../.claude/skills/security-review-team/) |
| `/debug-logs` | sensor | I | behavior | review-time | [.claude/skills/debug-logs/](../.claude/skills/debug-logs/) |
| `/debug-team` | sensor | I | behavior | review-time | [.claude/skills/debug-team/](../.claude/skills/debug-team/) |
| `/load-test` | sensor | C | arch-fitness | on-demand | [.claude/skills/load-test/](../.claude/skills/load-test/) |
| `/spec` | guide | I | meta | scaffold-time | [.claude/skills/spec/](../.claude/skills/spec/) |
| `/ralph-loop` | guide | I | meta | scaffold-time | [.claude/skills/ralph-loop/](../.claude/skills/ralph-loop/) |
| `/spec-review` | sensor | I | meta | review-time | [.claude/skills/spec-review/](../.claude/skills/spec-review/) |
| `/atlassian` | guide | I | meta | on-demand | [.claude/skills/atlassian/](../.claude/skills/atlassian/) |

### Subagents (inferential sensors with persistent memory)

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| `code-reviewer` | sensor | I | maint | review-time | [.claude/agents/code-reviewer.md](../.claude/agents/code-reviewer.md) |
| `security-reviewer` | sensor | I | behavior | review-time | [.claude/agents/security-reviewer.md](../.claude/agents/security-reviewer.md) |
| `db-analyst` | sensor | I | arch-fitness | review-time | [.claude/agents/db-analyst.md](../.claude/agents/db-analyst.md) |

### Hooks

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| Bash guard (PreToolUse) | sensor | C | meta | on-edit | [.claude/hooks/guard-bash.sh](../.claude/hooks/guard-bash.sh) |
| gopls+goimports (PostToolUse) | sensor | C | maint | on-edit | [.claude/hooks/lint-go-file.sh](../.claude/hooks/lint-go-file.sh) |
| gopls hints postprocessor | guide | C | maint | on-edit | [.claude/hooks/gopls-hints.awk](../.claude/hooks/gopls-hints.awk) (enriches diagnostics with actionable "fix by:" hints) |
| Migration validator (PostToolUse) | sensor | C | maint | on-edit | [.claude/hooks/validate-migration.sh](../.claude/hooks/validate-migration.sh) |
| Ralph-loop continuation (Stop) | guide | C | meta | stop-hook | [.claude/hooks/ralph-loop.sh](../.claude/hooks/ralph-loop.sh) |
| Post-impl validation gate (Stop) | sensor | C | maint+arch | stop-hook | [.claude/hooks/stop-validate.sh](../.claude/hooks/stop-validate.sh) |
| Worktree setup (WorktreeCreate) | guide | C | meta | scaffold-time | [.claude/hooks/worktree-create.sh](../.claude/hooks/worktree-create.sh) |
| Worktree teardown (WorktreeRemove) | guide | C | meta | scaffold-time | [.claude/hooks/worktree-remove.sh](../.claude/hooks/worktree-remove.sh) |

### lefthook.yml

| Step | Type | Exec | Category | Stage | Commands |
| --- | --- | --- | --- | --- | --- |
| pre-commit: fmt | sensor | C | maint | pre-commit | `goimports -w {staged_files}` |
| pre-commit: lint | sensor | C | maint | pre-commit | `golangci-lint run --new-from-rev=HEAD` |
| pre-push: build | sensor | C | maint | pre-push | `go build ./...` |
| pre-push: test | sensor | C | behavior | pre-push | `go test ./internal/... -count=1` |
| pre-push: vulncheck | sensor | C | behavior | pre-push | `govulncheck -show verbose ./...` |
| commit-msg: conventional | sensor | C | meta | pre-commit | regex `^(feat\|fix\|...)(\(.+\))?: .+` |

Config: [lefthook.yml](../lefthook.yml)

### golangci-lint — each enabled linter is a sensor

Every linter below is `computational`, runs at `pre-commit` (via lefthook) and `CI` (via
`lint` job). Categorized per function.

| Linter | Category | Purpose |
| --- | --- | --- |
| `errcheck` | behavior | unchecked errors |
| `govet` (all-enabled sans fieldalignment) | maint+behavior | suspicious constructs |
| `ineffassign` | maint | unused variable assignments |
| `staticcheck` (includes gosimple) | maint+behavior | static analysis |
| `unused` | maint | unused consts, vars, funcs, types |
| `gosec` | behavior | security problems |
| `bodyclose` | behavior | HTTP response body closed |
| `durationcheck` | behavior | durations multiplied together |
| `copyloopvar` (Go 1.22+) | behavior | loop variable capture |
| `nilerr` | behavior | code returning nil error |
| `sqlclosecheck` | behavior | sql.Rows/Stmt closed |
| `gocritic` (diagnostic+style+performance) | maint | highly extensible checks |
| `misspell` | maint | misspelled English words |
| `unconvert` | maint | unnecessary type conversions |
| `unparam` | maint | unused function parameters |
| `prealloc` | maint | slice pre-allocation opportunities |
| `errorlint` (errorf+asserts+comparison) | behavior | error wrapping issues |
| `gofmt`, `goimports` (formatters) | maint | standard formatting |

Config: [.golangci.yml](../.golangci.yml)

### CI workflows (.github/workflows/)

| Workflow / Job | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| `ci.yml::lint` | sensor | C | maint | CI | golangci-lint v2.11.4, timeout 5m |
| `ci.yml::deadcode` | sensor | C | maint | CI | `deadcode -test -filter '(cmd\|internal)/'` — fails on unreachable funcs |
| `ci.yml::vulncheck` | sensor | C | behavior | CI | `govulncheck -show verbose ./...` |
| `ci.yml::unit-tests` | sensor | C | behavior | CI | `go test -race -coverprofile=...`, **60% coverage threshold** |
| `ci.yml::coverage-delta` | sensor | C | maint | CI | `diff-cover` on PR — fails if < 70% coverage on changed lines |
| `ci.yml::e2e-tests` | sensor | C | behavior | CI | `go test ./tests/e2e/... -count=1` via TestContainers |
| `perf-regression.yml::regression` | sensor | C | arch-fitness | CI | k6 `load` + `perfcompare` vs. `tests/load/baselines/<scenario>.json` (p95 35%, p99 70%) |
| `mutation-nightly.yml::mutation` | sensor | C | maint | post-integration | gremlins over `./internal/usecases/...`, daily 03:00 UTC, informational |
| `release.yml` | guide | C | meta | CI | release pipeline |

Files: [.github/workflows/ci.yml](../.github/workflows/ci.yml),
[.github/workflows/release.yml](../.github/workflows/release.yml)

### MCP servers

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| Context7 (external library docs) | guide | I | maint | on-demand | global MCP plugin, see [CLAUDE.md § MCP — Context7](../CLAUDE.md) |

### gopherplate CLI (scaffolders)

| Command | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| `gopherplate new` | guide | C | meta | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate add domain` | guide | C | arch-fitness | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate remove domain` | guide | C | arch-fitness | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate add endpoint` | guide | C | arch-fitness | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate remove endpoint` | guide | C | arch-fitness | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate doctor` | sensor | C | meta | on-demand | [cmd/cli/](../cmd/cli/) |
| `gopherplate wiring` | guide | C | arch-fitness | scaffold-time | [cmd/cli/](../cmd/cli/) |
| `gopherplate version` | guide | C | meta | on-demand | [cmd/cli/](../cmd/cli/) |

Reference: [docs/guides/template-cli.md](guides/template-cli.md)

### OpenTelemetry / business metrics (continuous sensors)

| Artifact | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| OTel traces + HTTP metrics + DB pool metrics | sensor | C | arch-fitness | continuous | [pkg/telemetry/](../pkg/telemetry/) |
| Business-specific metrics (user counters, etc.) | sensor | C | behavior | continuous | [internal/infrastructure/telemetry/](../internal/infrastructure/telemetry/) |

### Commands summary

Common entry points are defined in the [Makefile](../Makefile). Listed here for quick cross-
reference with sensors above (each target invokes one or more):

- `make lint` — golangci-lint + gofmt
- `make vulncheck` — govulncheck
- `make test` / `make test-unit` / `make test-e2e` / `make test-coverage`
- `make docker-up` / `make docker-down` — infra for tests
- `make migrate-up` / `make migrate-create NAME=...`
- `make kind-setup` — Kind cluster bring-up
- `make proto` / `make proto-lint` — buf generate / lint

## Known gaps

Gaps not yet covered by the current harness. Each row links to a dedicated spec that implements
the sensor or guide. Links may be broken until the corresponding spec ships.

| Gap | Category | Spec |
| --- | --- | --- |
| ~~No performance regression gate: `/load-test` runs but never fails on degradation.~~ **Resolved by spec k6-regression-gate** (DONE) — see `perf-regression.yml` job above and [guides/perf-regression.md](guides/perf-regression.md). | arch-fitness | [.specs/k6-regression-gate.md](../.specs/k6-regression-gate.md) |
| ~~Coverage measures execution, not verification (no mutation testing).~~ **Resolved by spec maintainability-harness** (DONE) — see `mutation-nightly.yml`, [guides/mutation-testing.md](guides/mutation-testing.md). | maint | [.specs/maintainability-harness.md](../.specs/maintainability-harness.md) |
| ~~`unused` catches unreferenced; no detection of unreachable-but-referenced code.~~ **Resolved by spec maintainability-harness** (DONE) — see `ci.yml::deadcode`. | maint | [.specs/maintainability-harness.md](../.specs/maintainability-harness.md) |
| ~~Coverage threshold is global 60%, not a delta on changed lines.~~ **Resolved by spec maintainability-harness** (DONE) — see `ci.yml::coverage-delta` (70% threshold on changed lines). | maint | [.specs/maintainability-harness.md](../.specs/maintainability-harness.md) |
| ~~`gopls` diagnostics in `lint-go-file.sh` are not optimized for LLM consumption.~~ **Resolved by spec maintainability-harness** (DONE) — see `.claude/hooks/gopls-hints.awk` postprocessor. | maint | [.specs/maintainability-harness.md](../.specs/maintainability-harness.md) |
| No golden / approved-fixtures pattern for HTTP and gRPC response shapes. | behavior | [.specs/behavior-harness.md](../.specs/behavior-harness.md) |
| No `buf breaking` check — proto can regress contracts silently. | behavior | [.specs/behavior-harness.md](../.specs/behavior-harness.md) |
| Organizational patterns (handler must use `httpgin.SendSuccess`, use case must `ClassifyError`, etc.) are convention-only — no Semgrep rules to catch drift. | behavior | [.specs/behavior-harness.md](../.specs/behavior-harness.md) |
| `gopherplate new` produces a single generalist template — no flavor per service topology (CRUD / event-processor / data-pipeline). | meta | [.specs/cli-harness-flavors.md](../.specs/cli-harness-flavors.md) |

For the process of identifying new gaps and evolving the harness, see
[harness-self-steering.md](guides/harness-self-steering.md).

## References

- Martin Fowler, ["Harness Engineering for Coding Agents"](https://martinfowler.com/articles/harness-engineering.html)
- [ADR-009: Error handling](adr/009-error-handling.md) — example of how guide + sensor reinforce
  each other (ADR + linter + rule + review skill).
