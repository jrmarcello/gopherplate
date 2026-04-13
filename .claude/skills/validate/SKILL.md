---
name: validate
description: Post-implementation validation pipeline (static + tests + Kind + smoke)
user-invocable: true
---

# /validate [quick]

Post-implementation validation pipeline. Ensures code changes are production-ready.

## Phases

### Phase 1 — Static Validation

1. `go build ./...` — compilation check
2. `goimports -l .` or `gofmt -l .` — formatting
3. `go vet ./...` — static analysis
4. `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` — Swagger freshness

### Phase 2 — Automated Tests

1. `golangci-lint run ./...` — full linter suite
2. `go test ./internal/... -v -count=1` — unit tests
3. `go test ./pkg/... -v -count=1` — pkg tests

### Phase 3 — Kind Deploy & Smoke (skip with `quick`)

1. `make kind-deploy` — build + deploy to local Kind cluster
2. `curl http://localhost:8080/health` — health check
3. Basic CRUD smoke tests via `api.http` patterns

### Phase 4 — Functional Validation (skip with `quick`)

1. Verify the specific behavior that was implemented/fixed
2. Test edge cases and error paths
3. Confirm API response format matches `{"data": ...}` / `{"errors": {"message": ...}}`

## Usage

- `/validate` — Full pipeline (all 4 phases)
- `/validate quick` — Phases 1-2 only (static + tests)

## Output

| Phase | Check | Result |
|-------|-------|--------|
| 1 | Build | PASS/FAIL |
| 1 | Format | PASS/FAIL |
| 1 | Vet | PASS/FAIL |
| 1 | Swagger | PASS/FAIL |
| 2 | Lint | PASS/FAIL |
| 2 | Unit tests | PASS/FAIL |
| 2 | Pkg tests | PASS/FAIL |
| 3 | Kind deploy | PASS/FAIL/SKIP |
| 3 | Health check | PASS/FAIL/SKIP |
| 4 | Functional | PASS/FAIL/SKIP |
