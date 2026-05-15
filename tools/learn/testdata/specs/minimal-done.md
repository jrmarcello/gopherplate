# Spec: Minimal Done

## Status: DONE

## Context

A tiny fixture used by the spec parser unit tests.

## Requirements

- REQ-1 The parser MUST emit one record per status transition.

## Tasks

- [x] TASK-1 Implement the foo widget
  - files: `internal/foo/foo.go`, `internal/foo/foo_test.go`
  - tests: TC-UC-01
- [ ] TASK-2 Wire foo into the API server
  - files: `cmd/api/server.go`

## Execution Log

### TASK-1 (2026-05-14 18:42)

TDD: RED(3 compile-fail) → GREEN(3 passing) → REFACTOR(clean) — implementação
do foo widget concluída. Tests green.

### Batch 2 [TASK-2, TASK-3] (2026-05-14 18:50)

Parallel via worktrees. Merge applied cleanly.
