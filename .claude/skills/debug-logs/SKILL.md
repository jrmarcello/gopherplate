---
name: debug-logs
description: Analyze logs from Kind cluster or docker-compose to diagnose issues
user-invocable: true
---

# /debug-logs [keyword]

Analyzes application logs from Kind cluster or Docker environment.

## Execution

### 1. Detect Environment
- Kind cluster running? → `kubectl logs -n gopherplate-dev -l app=gopherplate --tail=200`
- Docker Compose? → `docker compose -f docker/docker-compose.yml logs --tail=200`
- Local dev (air)? → Check `tmp/` directory for air logs

### 2. Fetch Logs
- Get last 200 lines from the appropriate source
- If keyword provided, filter with `grep -i <keyword>`

### 3. Analyze
- Filter for ERROR, WARN, PANIC, FATAL levels
- Extract trace IDs for correlation
- Check for repeated patterns (connection timeouts, query errors)
- Look for stack traces

### 4. Correlate
- Match trace IDs across services if available
- Check related services (PostgreSQL, Redis) for errors
- Cross-reference with recent code changes

### 5. Report

```
## Log Analysis

### Environment: Kind / Docker / Local
### Time range: ...
### Keyword filter: ...

### Errors Found
1. [ERROR] timestamp — message
   Context: ...
   Likely cause: ...

### Warnings
1. [WARN] timestamp — message

### Root Cause Analysis
...

### Suggested Actions
1. ...
```
