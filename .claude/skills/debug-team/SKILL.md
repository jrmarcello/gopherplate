---
name: debug-team
description: Parallel 3-agent bug investigation with competing hypotheses
user-invocable: true
---

# /debug-team <bug description>

Creates a team of 3 specialized investigators who independently investigate the bug using competing hypotheses, then debate findings to converge on root cause.

## Team

### 1. Application Layer Investigator

Focus: Business logic and code flow

- Trace the request path through handler → use case → repository
- Check error handling chains
- Verify DI wiring in `cmd/api/server.go`
- Look for logic errors in domain/use case layer

### 2. Infrastructure & Data Investigator

Focus: Database, cache, external services

- Check SQL queries and migrations
- Verify database schema matches code expectations
- Check Redis cache behavior
- Review Docker/K8s configuration
- Verify environment variables

### 3. Test & Reproduction Investigator

Focus: Reproducing and isolating the bug

- Write a minimal reproduction case
- Check existing test coverage for the affected area
- Run relevant tests and analyze failures
- Check for race conditions or timing issues

## Execution

1. Launch all 3 agents in parallel with the bug description
2. Each agent independently investigates and forms a hypothesis
3. Collect findings from all agents
4. Synthesize: identify agreement points and contradictions
5. Present root cause analysis and recommended fix

## Output

```text
## Root Cause Analysis

### Hypothesis (agreed by N/3 investigators)
...

### Evidence
- [Investigator 1]: ...
- [Investigator 2]: ...
- [Investigator 3]: ...

### Recommended Fix
...

### Dissenting Views (if any)
...
```
