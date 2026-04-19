---
name: fix-issue
description: End-to-end bug fix workflow (understand → plan → implement → test → validate)
user-invocable: true
---

# /fix-issue <issue-description>

End-to-end issue analysis and fixing workflow.

## Workflow

### 1. Understand

- Read the issue description and any related files
- Reproduce the issue if possible
- Identify root cause through code analysis

### 2. Plan

- Define scope of changes
- List affected files
- Identify risks
- Choose validation strategy

### 3. Implement

- Apply changes following Clean Architecture (domain → usecases → infrastructure)
- Use unique error variable names
- Follow project conventions from AGENTS.md

### 4. Test

- Create/update unit tests for the fix
- Ensure existing tests still pass
- Run `make test`
- **Write a regression test first** (RED) — the test must fail on the broken code and pass
  after the fix. If the test also passes on the pre-fix code, the test doesn't cover the bug.
- For non-trivial fixes, delegate a test-quality review to the `test-reviewer` subagent
  before finalizing: "use the test-reviewer subagent to audit the regression test added for
  this fix."

### 5. Validate

- Run `make lint`
- Verify the fix addresses the original issue
- Check for regressions
- Update Swagger if handler/router changed: `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`

### 6. Commit

- Stage specific files (never `git add -A`)
- Commit with `fix(scope): description` format

## Rules

- Never skip the validation step
- If the fix requires architecture changes, ask the user first
- If multiple approaches exist, present options before implementing
