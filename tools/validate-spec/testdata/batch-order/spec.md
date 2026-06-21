## Status: DRAFT

## Context

TASK-2 depends on TASK-1 but TASK-2 is in batch 1, TASK-1 is in batch 2. Violates batchOrder.

## Requirements

- [ ] REQ-1: Something

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | does thing | ok |

## Design

Design.

## Tasks

- [ ] TASK-1: Base task
  - files: internal/domain/x/a.go

- [ ] TASK-2: Dependent task
  - files: internal/domain/x/b.go
  - depends: TASK-1

## Parallel Batches

Batch 1: [TASK-2]

Batch 2: [TASK-1]

## Validation Criteria

All pass.

## Execution Log

