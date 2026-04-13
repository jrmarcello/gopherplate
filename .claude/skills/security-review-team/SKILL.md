---
name: security-review-team
description: Parallel 3-agent security audit (code + infra + data)
user-invocable: true
---

# /security-review-team

Launches a parallel security audit with 3 specialized reviewers.

## Team

### 1. Code Security Reviewer

Focus: Application-level vulnerabilities

- SQL injection (check all sqlx queries)
- Command injection (check Bash/exec usage)
- Input validation gaps
- Authentication/authorization bypass
- Error information disclosure
- Unsafe type assertions

### 2. Infrastructure Security Reviewer

Focus: Deployment and configuration security

- Docker image security (non-root, minimal base)
- Kubernetes manifests (RBAC, resource limits, network policies)
- Environment variable handling
- Secret management
- Exposed ports and services
- CI pipeline security

### 3. Data Security Reviewer

Focus: Data protection and privacy

- PII handling (logging, error responses, storage)
- Database access patterns (least privilege)
- Redis cache security (sensitive data TTL)
- API response filtering (no internal IDs leak)
- Credential storage patterns

## Execution

Launch all 3 agents in parallel. Each agent works independently with competing analysis.

## Output

Synthesize findings by severity:

| Severity | Finding | File:Line | Recommendation |
|----------|---------|-----------|----------------|
| CRITICAL | ... | ... | ... |
| HIGH | ... | ... | ... |
| MEDIUM | ... | ... | ... |
| LOW | ... | ... | ... |
