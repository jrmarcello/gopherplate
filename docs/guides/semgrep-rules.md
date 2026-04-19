# Semgrep rules

Semgrep enforces **organizational patterns** that `golangci-lint` cannot express —
project-specific conventions around response envelopes, error mapping, and architectural
boundaries.

Rules live in `.semgrep/` with their test fixtures as sibling `.go` files. Each rule is
validated against positive and negative examples before it ever scans production code.

## Running locally

```bash
make semgrep         # scan ./internal/ against .semgrep/ rules
make semgrep-test    # validate rules against fixtures in .semgrep/*.go
```

If `semgrep` is not installed, the targets print an install hint
(`pip install semgrep` or `brew install semgrep`). CI runs semgrep via the official Docker
image (`returntocorp/semgrep:latest`) so no local install is required for reviewers.

## Current rules

### `gopherplate-no-direct-gin-json` — handlers.yml

**Scope:** `internal/infrastructure/web/handler/**` (excluding tests).

**Triggers on:** `c.JSON`, `c.String`, `c.IndentedJSON`, `c.AbortWithStatusJSON`.

**Why:** The project standardizes on `httpgin.SendSuccess` / `SendError` /
`SendSuccessWithMeta` so the response envelope (`{"data": ...}` / `{"errors": {...}}`) is
defined in one place (see ADR-008). Handlers that reach for Gin's raw helpers break that
contract and make it impossible to evolve the envelope without finding every caller.

**Exception:** middleware (`internal/infrastructure/web/middleware/**`) is NOT in scope —
middleware legitimately uses `AbortWithStatusJSON` to short-circuit requests before the
handler runs.

### `gopherplate-handler-no-domain-errors-import` — handlers.yml

**Scope:** `internal/infrastructure/web/handler/**` (excluding tests).

**Triggers on:** any import of `github.com/jrmarcello/gopherplate/internal/domain/*/errors`.

**Why:** Handlers resolve errors generically via `errors.As(&appErr)` against
`*apperror.AppError`. Importing domain errors directly couples the handler to domain
internals and defeats the generic mapping (see ADR-009). The use-case layer owns the
translation; handlers just render.

### `gopherplate-usecase-no-direct-domain-error-bare-return` — usecases.yml

**Scope:** `internal/usecases/**` (excluding tests, `shared/`, `interfaces/`, `dto/`).

**Triggers on:** `return nil, <pkg>.Err<Something>` or `return x, <pkg>.Err<Something>`
where `<pkg>` is a short alias and `<Something>` is the sentinel error's exported name.

**Why:** Use cases must return `*apperror.AppError` via a local `toAppError()` mapping so
the handler layer has a single target for `errors.As`. Returning the bare domain error
breaks that contract (see ADR-009 and `docs/guides/error-handling.md`).

**Exception:** repositories (`internal/infrastructure/db/postgres/repository/**`) legitimately
return bare domain errors — the use case wraps them. The rule's `paths.include` ensures we
only enforce inside `internal/usecases/`.

## Adding a new rule

1. Draft the rule in `.semgrep/<topic>.yml`. Give it a unique ID prefixed with
   `gopherplate-` so it's easy to grep for and clearly ours.
2. Write a fixture `.semgrep/<topic>.go` with `ruleid:` and `ok:` markers above lines
   that should / shouldn't match. Use the `//go:build semgrep_fixture` tag so the Go
   compiler ignores it.
3. Run `make semgrep-test` — the test MUST pass before you scan production code.
4. Run `make semgrep` to check against `./internal/` and confirm no false positives.
5. Add the new rule to the "Current rules" section of this guide with scope, trigger,
   rationale, and any explicit exceptions.

## Debugging a false positive

`make semgrep` flags something that's actually correct? Two options:

1. **Tighten the rule.** Add `paths.exclude` for the legitimate case or narrow the
   `pattern` so it only matches the real drift. Commit the fix with a test fixture
   demonstrating the exception.
2. **Annotate the source.** Semgrep respects `// nosemgrep: <rule-id>` comments. Use
   sparingly and only with a comment explaining why. If you're about to add more than one
   or two, the rule is wrong — tighten it instead.

## Related

- [docs/harness.md](../harness.md) — semgrep rules are listed as behavior sensors.
- [.specs/behavior-harness.md](../../.specs/behavior-harness.md) — full spec.
- [docs/adr/008-api-response-format.md](../adr/008-api-response-format.md) — why the
  envelope helpers exist.
- [docs/adr/009-error-handling.md](../adr/009-error-handling.md) — why handlers never
  touch domain errors directly.
