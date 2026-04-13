---
applies-to: ".specs/**"
---

# SDD Spec Rules

## Spec File Integrity

- Never modify the Requirements section during execution (only during DRAFT status)
- Never remove tasks — mark them as `[x]` (done) or `BLOCKED`
- Always append to Execution Log, never overwrite previous entries
- Status transitions: DRAFT -> APPROVED -> IN_PROGRESS -> DONE | FAILED

## Task Execution

- Each task must be independently verifiable (`go build ./...` should pass after each task)
- Tasks are architecture-agnostic — no mandatory layer ordering
- Order tasks logically for the feature, respecting the project's chosen structure
- If a task is unclear, mark it `BLOCKED` with a reason and stop execution
- **Mandatory review before testing**: after implementing a task, re-read the task description and verify ALL specified files, patterns, and behaviors were implemented. Check: all files listed in `files:` metadata were created/modified, all patterns from the Design section are followed, all error mappings and wrapping are complete, no implementation gap vs the spec. Only then proceed to tests. This is NEVER skipped.

## Task Metadata

- Every task MUST have a `files:` sub-item listing files it creates or modifies
- Tasks with dependencies MUST have a `depends:` sub-item listing prerequisite TASK-N IDs
- `depends:` must form a DAG (no circular dependencies)
- Tasks that share files in their `files:` lists cannot be in the same parallel batch
- Tasks with testable code MUST have a `tests:` sub-item listing TC-IDs from the Test Plan (triggers TDD cycle in ralph-loop)

## Test Plan

Every spec MUST include a `## Test Plan` section between Requirements and Design. The Test Plan contains tables grouped by layer:

- **Domain Tests** (TC-D-NN): pure domain logic, value objects, entity invariants
- **Use Case Tests** (TC-UC-NN): application logic, dependency interactions, error mapping
- **E2E Tests** (TC-E2E-NN): full HTTP round-trip via TestContainers
- **Smoke Tests** (TC-S-NN): k6-based validation of deployed behavior

Each TC row has: `| TC-ID | REQ | Category | Description | Expected |`

Categories: `happy`, `validation`, `business`, `edge`, `infra`, `concurrency`, `idempotency`, `security`

For non-code specs (config/docs only), the Test Plan may be `N/A` with a justification.

### Coverage Rules

Every spec MUST satisfy all of the following:

- Every REQ has >= 1 TC (at minimum the happy path)
- Every sentinel error in domain `errors.go` has >= 1 TC that triggers it
- Every validated field has boundary TCs: valid min, valid max, invalid min-1, invalid max+1
- Every external dependency call (repo, cache, publisher) has >= 1 infra-failure TC
- Every conditional branch in use case flow has TCs for both paths
- Concurrency scenarios required for operations with advisory lock or optimistic locking
- Every new HTTP endpoint has smoke TCs: happy path (201/200 + all response fields), each distinct error status (400/409/422), response format, auth, field boundaries, idempotency
- **Rigor check**: error/edge TCs should outnumber happy-path TCs — review the complete Test Plan and verify no business rule untested, no error path missing, no boundary unchecked

### Mutability

- TCs may be **added** during IN_PROGRESS (new edge cases discovered during implementation)
- TCs may NEVER be **removed** — if a TC is no longer applicable, mark it as `SKIPPED` with a reason
- REQ references in TCs must remain valid

### Smoke Tests (k6)

- TC-S-* are validated by running `k6 run --env SCENARIO=smoke tests/load/main.js`
- Smoke tests are executed by `TASK-SMOKE` — a dedicated task at the end of the spec
- Smoke tests do NOT follow the TDD RED/GREEN cycle (they are executed directly)
- If the app is not running, log `SMOKE: DEFERRED` in the Execution Log
- Smoke file convention: `tests/load/users.js`, `tests/load/roles.js`, `tests/load/main.js`, `tests/load/helpers.js`

## TDD Execution

When a task has `tests:` metadata, the ralph-loop follows the TDD cycle:

### RED Phase

1. Write the test file FIRST (before the production code)
2. Tests reference the function/type that will be implemented
3. Run `go test` — tests MUST fail (compilation failure counts as valid RED)
4. If tests pass before implementation: the test is not testing the right thing — fix it

### GREEN Phase

1. Write the MINIMUM production code to make tests pass
2. Follow existing patterns: hand-written mocks in `mocks_test.go`, table-driven tests
3. Run `go test` — all tests listed in `tests:` MUST pass
4. If other tests break: fix immediately before proceeding

### REFACTOR Phase

1. Clean up production code: remove duplication, improve naming, extract helpers
2. Run `go test` again — all tests MUST still pass
3. Run `go build ./...` — must compile cleanly

### Execution Log Format

When a task follows TDD, the Execution Log entry includes:

```text
TDD: RED(N failing) -> GREEN(N passing) -> REFACTOR(clean)
```

### Exceptions

- **Smoke tests** (TC-S-*): executed directly via k6, not via TDD cycle
- **Non-code tasks** (docs, config): no TDD — normal execution
- **Tasks without `tests:` metadata**: normal execution (no TDD cycle required)

## Parallel Batches

- The Parallel Batches section is auto-generated by `/spec` based on dependency and file analysis
- Batches are sequential: Batch N+1 starts only after all tasks in Batch N complete
- Tasks within a batch are independent: no shared files, no inter-dependencies
- Shared files are classified as:
  - **exclusive** — only one task touches it (safe for parallel)
  - **shared-additive** — multiple tasks add to it, e.g. DI wiring, route registration (accumulator pattern candidate)
  - **shared-mutative** — multiple tasks modify existing code (must serialize)

## Merge Strategy (Hybrid B+C)

When parallel tasks share additive files (e.g. `server.go`), use the accumulator pattern:

- Each parallel task generates a wiring fragment in `.specs/wiring/` instead of editing the shared file
- A dedicated merge task (`TASK-MERGE`) reads all fragments and applies them sequentially
- Fragments describe intent (what to add), not patches
- Shared-mutative files always serialize (Opção B) — never run in parallel

## Naming

- Spec files: lowercase, hyphen-separated: `user-audit-log.md`, `role-permissions.md`
- Active state files: `<name>.active.md` (auto-created by ralph-loop, do not edit manually)
