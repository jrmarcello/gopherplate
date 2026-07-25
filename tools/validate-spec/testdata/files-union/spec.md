## Slug: FUNI

## Status: DRAFT

## Context

See docs/capabilities/x.md

## Requirements

- [ ] FUNI-REQ-1: first req

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| FUNI-TC-UC-01 | FUNI-REQ-1 | happy | does thing | ok |

## Design

d

## Tasks

- [ ] TASK-1: impl a
  - files: internal/domain/z/entity.go, internal/domain/a/entity.go
  - tests: FUNI-TC-UC-01

- [ ] TASK-2: impl b
  - files: internal/domain/m/entity.go, internal/domain/a/entity.go
  - tests: FUNI-TC-UC-01
  - depends: TASK-1

## Parallel Batches

Batch 1: [TASK-1]

Batch 2: [TASK-2]
- shared-additive: internal/domain/a/entity.go

## Validation Criteria

all pass

## Execution Log

