---
name: full-review-team
description: Parallel 3-agent review (architecture + security + database)
user-invocable: true
---

# /full-review-team

Launches a parallel code review with 3 specialized agents auditing the codebase independently.

## Team

### 1. Architecture Reviewer (code-reviewer agent)

- Clean Architecture compliance
- Layer boundary violations
- Go idioms and conventions
- DI patterns and interface design
- Template quality (as a boilerplate)

### 2. Security Reviewer (security-reviewer agent)

- OWASP Top 10 vulnerabilities
- SQL injection, command injection
- Auth middleware gaps
- Data exposure (PII, credentials)
- Infrastructure security

### 3. Database Reviewer (db-analyst agent)

- Schema design and normalization
- Query performance (N+1, missing indexes)
- Migration quality (Up/Down, reversibility)
- DBCluster usage patterns
- Connection pool configuration

## Execution

Launch all 3 agents in parallel using Agent tool:

```text
Agent(code-reviewer): Review codebase for architecture compliance and Go conventions
Agent(security-reviewer): Audit codebase for security vulnerabilities
Agent(db-analyst): Analyze database schema, queries, and migrations
```

## Output

Synthesize findings into a unified report and save as `docs/review-report-YYYY-MM-DD.md` (using the current date).

The report must include:

1. Executive summary table (severity x category counts)
2. All findings grouped by priority (CRITICAL/MUST FIX first)
3. Deduplicated findings (when multiple reviewers flag the same issue)
4. Positive findings section (patterns to preserve)
5. Recommended action plan in phases

| Category | Severity | Count |
|----------|----------|-------|
| Architecture | MUST FIX / SHOULD FIX / NICE TO HAVE | N |
| Security | CRITICAL / HIGH / MEDIUM / LOW | N |
| Database | MUST FIX / SHOULD FIX / NICE TO HAVE | N |
