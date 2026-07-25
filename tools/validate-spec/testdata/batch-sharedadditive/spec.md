## Status: DRAFT

## Context

Two tasks in same batch share a file that IS in shared-additive. No batchFileOverlap error expected.

## Requirements

- [ ] REQ-1: Something

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | does thing | ok |

## Design

Design.

## Tasks

- [ ] TASK-1: First
  - files: internal/domain/x/shared.go
  - files: internal/domain/x/a.go

- [ ] TASK-2: Second
  - files: internal/domain/x/shared.go
  - files: internal/domain/x/b.go

## Parallel Batches

Batch 1: [TASK-1, TASK-2]
- shared-additive: internal/domain/x/shared.go

## Validation Criteria

All pass.

## Execution Log

