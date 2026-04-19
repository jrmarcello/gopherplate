---
name: review
description: Single-agent code review for Clean Architecture, security, and conventions
user-invocable: true
---

# /review [file|branch]

Code review focused on Clean Architecture, security, and project conventions.

## Scope

- No arguments: review all uncommitted changes (`git diff` + `git diff --cached`)
- File path: review specific file
- Branch name: review all changes on branch vs main

## Checklist

### Architecture

- [ ] Domain layer has zero external dependencies
- [ ] Use cases define interfaces in `interfaces/`
- [ ] No infrastructure imports from domain/usecases
- [ ] Handlers use `httputil.SendSuccess`/`httputil.SendError`
- [ ] Error handling follows domain → AppError → HTTP translation

### Security

- [ ] No credentials in code
- [ ] Parameterized SQL queries only
- [ ] Input validation at handler layer
- [ ] No PII in logs or error responses

### Go Conventions

- [ ] Unique error variable names (no shadowing)
- [ ] Error wrapping with context
- [ ] Context propagation through layers
- [ ] Table-driven tests with descriptive names

### Testing

- [ ] New code has corresponding tests
- [ ] Hand-written mocks (no mocking frameworks)
- [ ] Tests cover error paths (error-path TCs should outnumber happy-path TCs per sdd.md rigor check)
- [ ] Table-driven structure with named subtests (`t.Run(tc.name, ...)`)
- [ ] No test smells: `time.Sleep` for sync, asserted `time.Now()`, unseeded random, empty bodies, naked `panic`
- [ ] Mocks assert calls when they matter (not record-and-forget)
- [ ] Boundary TCs on validated fields (valid min/max + invalid min-1/max+1)
- [ ] For SDD tasks: Execution Log shows `TDD: RED(N) -> GREEN(N) -> REFACTOR` evidence

For a deep audit of test quality across the branch, delegate to the `test-reviewer`
subagent ("use the test-reviewer subagent to audit these tests") or run `/full-review-team`.

### Observability

- [ ] OpenTelemetry spans for new operations
- [ ] Structured logging with `logutil`

### Template Quality

- [ ] Code is exemplary for teams cloning this template
- [ ] Patterns are easy to follow

## Output Format

For each finding:

```text
[SEVERITY] file:line — Description
  Suggested fix: ...
```

Severities: MUST FIX, SHOULD FIX, NICE TO HAVE
