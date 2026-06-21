## Status: DRAFT

## Context

Task has a mix: one excluded file (cmd/api/server.go) and one production Go file.
Should still WARN because of the non-excluded file.

## Requirements

- [ ] REQ-1: Something

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | does thing | ok |

## Design

Design.

## Tasks

- [ ] TASK-1: Do something mixed
  - files: cmd/api/server.go
  - files: internal/domain/x/entity.go

## Parallel Batches

Batch 1: [TASK-1]

## Validation Criteria

All pass.

## Execution Log

