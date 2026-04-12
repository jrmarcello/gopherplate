---
name: spec
description: Create a structured SDD specification (requirements, design, tasks) for a new feature or change
argument-hint: "<feature-description>"
user-invocable: true
---

# /spec <feature-description>

Creates a structured specification document following Specification-Driven Development (SDD) principles.

## Example

```text
/spec "Add audit logging to all user write operations"
```

## Workflow

### 1. Understand the Request

- Parse the feature description
- Identify affected domain(s) and code areas
- Determine the type of change: new feature, refactor, bug fix, new domain, etc.

### 2. Gather Context

- Read existing code for affected areas
- Check ADRs in `docs/adr/` for relevant architectural decisions
- Identify existing patterns to follow (use `user` and `role` domains as reference)
- Respect the project's chosen architecture (separate layers, collapsed, or hybrid)

### 3. Generate Spec

- Create `.specs/<feature-name>.md` from the template at `.specs/TEMPLATE.md`
- Fill in all sections: Context, Requirements, Test Plan, Design, Tasks, Parallel Batches, Validation Criteria
- Requirements should use **GIVEN/WHEN/THEN** format for unambiguous acceptance criteria
- Mark uncertain items with `[NEEDS CLARIFICATION]` instead of assuming
- Tasks must be:
  - Concrete and independently verifiable (`go build ./...` should pass after each)
  - Ordered logically for the feature (not necessarily by architecture layer)
  - Small enough to complete in a single focused iteration
  - Self-contained — each task description should be understandable without reading previous tasks
- Each task MUST include:
  - `files:` — concrete file paths this task creates or modifies
  - `depends:` — other TASK-N IDs that must complete first (omit if no dependencies)
  - `tests:` — TC-IDs from the Test Plan that this task must satisfy (triggers TDD cycle in ralph-loop; omit for non-code tasks)

### 4. Generate Test Plan

After generating Requirements and Design, derive an **exhaustive** Test Plan. If a scenario can happen in production, it must have a test case.

1. **For each REQ**: derive at least one happy-path TC plus all error/edge TCs
2. **For each sentinel error** in domain `errors.go`: ensure >= 1 TC triggers it
3. **For each validated field**: boundary TCs — valid min, valid max, invalid min-1, invalid max+1
4. **For each external dependency** (repo, cache, publisher): >= 1 infra-failure TC
5. **For each conditional branch** in use case flow: TCs for both paths
6. **Concurrency scenarios**: required for operations with advisory lock, optimistic locking, or REPEATABLE READ
7. **For each new HTTP endpoint**, generate smoke TCs covering:
   - Happy path (201/200 + all response fields validated)
   - Each distinct error status (400, 409, 422)
   - Auth errors (missing/invalid service key)
   - Response format consistency (`{"data": ...}` or `{"errors": {"message": ...}}`)
   - Field boundaries and edge cases
   - Idempotency (if applicable)
8. **Assign TCs to tasks** via `tests:` metadata — smoke TCs go in dedicated `TASK-SMOKE`
9. **Rigor check**: review the complete Test Plan and verify — no business rule untested, no error path missing, no boundary unchecked. Error/edge TCs should outnumber happy-path TCs.

Group TCs by layer:

- **Domain Tests** (TC-D-NN): pure domain logic, value objects, entity invariants
- **Use Case Tests** (TC-UC-NN): application logic, dependency interactions, error mapping
- **E2E Tests** (TC-E2E-NN): full HTTP round-trip via TestContainers
- **Smoke Tests** (TC-S-NN): k6 checks against running app — HTTP status, response format, auth, field boundaries, idempotency, business rules visible via API

Categories: `happy`, `validation`, `business`, `edge`, `infra`, `concurrency`, `idempotency`, `security`

For non-code specs (config/docs only), the Test Plan may be `N/A` with justification.

### 5. Analyze Parallelism

After generating tasks, build the **Parallel Batches** section:

1. Build a dependency graph from `depends:` and `files:` metadata
2. Two tasks **cannot** be parallel if:
   - One appears in the other's `depends:` list
   - They share any file in their `files:` lists
3. Group tasks into sequential batches using topological sort:
   - Batch 1: all tasks with no dependencies
   - Batch 2: all tasks whose dependencies are fully satisfied by Batch 1
   - Batch N: all tasks whose dependencies are fully satisfied by Batches 1..N-1
4. Classify shared files:
   - **Exclusive**: only one task touches it — safe for parallel
   - **Shared-additive**: multiple tasks touch it, but all are additive (e.g., `server.go` wiring, `router.go` routes) — candidate for accumulator pattern
   - **Shared-mutative**: multiple tasks modify existing code in the same file — must serialize
5. Present the batches to the user with the classification

Example output:

```text
## Parallel Batches

Batch 1: [TASK-1]                    — foundation
Batch 2: [TASK-2, TASK-3, TASK-4]    — parallel (no shared files)
Batch 3: [TASK-5]                    — sequential (shared: cmd/api/server.go [additive])
Batch 4: [TASK-6]                    — sequential (depends: TASK-2, TASK-3)

File overlap analysis:
- cmd/api/server.go: TASK-2, TASK-3, TASK-5 -> classified as shared-additive (DI wiring)
- All other files: exclusive to one task
```

### 6. Present for Approval

- Display the spec to the user, highlighting the **Test Plan** and **Parallel Batches** sections
- Set status to `DRAFT`
- Ask: "Review this spec. Edit anything you want, then approve to begin implementation."
- If parallel batches exist, note: "Batches with multiple tasks can run in parallel via worktree agents or sequentially via `/ralph-loop`."
- On approval, set status to `APPROVED`

## Rules

- Spec files go in `.specs/` directory
- File naming: lowercase, hyphen-separated: `.specs/user-audit-log.md`
- Never include tasks that require user decisions — ask upfront during spec creation
- Reference existing patterns: if a task is similar to existing code, note which files to use as reference
- Match spec depth to task complexity — a simple bug fix needs fewer sections than a new domain
- Architecture is flexible: the template recommends Clean Architecture but does not impose it. Adapt the spec to the project's chosen structure

## Integration

- After approval, run `/ralph-loop .specs/<name>.md` for autonomous execution
- Or execute tasks manually one at a time
- Use `/spec-review .specs/<name>.md` after implementation to verify against requirements
