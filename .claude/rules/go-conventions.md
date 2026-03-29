---
applies-to: "**/*.go"
---
# Go Code Conventions

## Error Handling
- Use unique error variable names to avoid shadowing: `parseErr`, `saveErr`, `fetchErr` (never reuse `err`)
- Wrap errors with context: `fmt.Errorf("creating user: %w", err)`
- Domain errors are pure: `user.ErrNotFound`, `user.ErrDuplicateEmail`
- Never return HTTP status codes from domain or usecases

## Architecture
- Domain layer: zero external dependencies, only stdlib
- Use cases: define interfaces in `interfaces/` subdirectory, DTOs in `dto/`
- One use case per file: create.go, get.go, update.go, delete.go, list.go
- Handlers: always use `httputil.SendSuccess(c, status, data)` and `httputil.SendError(c, status, message)`

## DI Pattern
- Constructor injection for required deps (interfaces)
- Builder methods for optional deps: `.WithCache()`
- All wiring in `cmd/api/server.go:buildDependencies()`

## Reusable Packages (pkg/)
- Use `pkg/apperror` for structured errors with HTTP status
- Use `pkg/httputil` for standardized API responses
- Use `pkg/cache` for cache interface (not `internal/infrastructure/cache`)
- Use `pkg/database` for DB connections
- Use `pkg/logutil` for structured logging
- Use `pkg/telemetry` for OpenTelemetry setup

## Testing
- Table-driven tests with descriptive names
- Hand-written mocks in `mock_test.go` per package
- go-sqlmock for repository tests
- No test frameworks beyond stdlib `testing` package + testify assertions
