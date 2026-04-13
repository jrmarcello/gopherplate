---
applies-to: "**/*"
---
# Security Rules

## Credentials

- Never commit real credentials or secrets
- .env files must be in .gitignore
- Use environment variables for all secrets (DB passwords, API keys, Redis credentials)
- Default dev credentials are acceptable only in docker-compose.yml

## Data Protection

- Never log PII: full names, email addresses, phone numbers, tokens
- Sanitize error messages before returning to API clients
- Use the standard `{"errors": {"message": ...}}` response format for errors

## Code Safety

- Always use parameterized queries with sqlx (never string concatenation for SQL)
- Validate all user input at handler layer before passing to use cases
- Use Value Objects (ID, Email) for domain validation
- Context must be propagated through all layers for cancellation support

## Infrastructure

- Docker images must run as non-root user
- Never expose internal ports (PostgreSQL 5432, Redis 6379) in production
- Service key authentication required on all API endpoints
