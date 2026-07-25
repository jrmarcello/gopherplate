## Slug: GOLD

## Status: APPROVED

## Context

A golden spec used for testing purposes. See docs/capabilities/x.md for reference.

## Requirements

- [ ] GOLD-REQ-1: First requirement (happy path covered)
- [ ] GOLD-REQ-2: Second requirement (doc-only, no test needed) (no-test: doc-only)

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| GOLD-TC-UC-01 | GOLD-REQ-1 | happy | Creates a user successfully | 201 response |
| GOLD-TC-UC-02 | GOLD-REQ-1 | edge | Backtick example in Description column: `WRONG-TC-UC-99` should be ignored | error 400 |

## Design

Some design notes here.

## Tasks

- [ ] TASK-1: Implement domain
  - files: internal/domain/gold/entity.go
  - tests: GOLD-TC-UC-01

- [ ] TASK-2: Implement repository
  - files: internal/infrastructure/db/postgres/repository/gold/repository.go
  - tests: GOLD-TC-UC-02
  - depends: TASK-1

- [ ] TASK-3: Implement use case
  - files: internal/usecases/gold/create.go
  - files: internal/usecases/gold/shared.go
  - tests: GOLD-TC-UC-01
  - depends: TASK-1

## Parallel Batches

Batch 1: [TASK-1]

Batch 2: [TASK-2, TASK-3]
- shared-additive: internal/usecases/gold/shared.go

## Validation Criteria

- All tests pass
- Build succeeds

## Execution Log

