## Status: DRAFT

## Context

Status lines inside fenced blocks should be ignored.

```go
## Status: WIP
```

```markdown
## Status: Active
```

~~~
## Status: WIP
~~~

~~~go
## Status: WIP
~~~

   ```
   ## Status: WIP
   ```

## Requirements

- [ ] REQ-1: Something

## Test Plan

| TC-ID | REQ | Category | Description | Expected |
| --- | --- | --- | --- | --- |
| TC-UC-01 | REQ-1 | happy | does thing | ok |

## Design

Design.

## Tasks

- [ ] TASK-1: Do something
  - files: internal/domain/x/entity.go
  - tests: TC-UC-01

## Parallel Batches

Batch 1: [TASK-1]

## Validation Criteria

All pass.

## Execution Log

