# Capability: User

## Slug: USER

## Status: Active

## Source

- Specs: n/a — pre-SDD (template example domain)
- ADRs:
  - [docs/adr/002-ids.md](../adr/002-ids.md) — UUID v7 identifier strategy
  - [docs/adr/008-api-response-format.md](../adr/008-api-response-format.md) — standardised HTTP response envelope
  - [docs/adr/009-error-handling.md](../adr/009-error-handling.md) — error classification in the use case layer

## Guarantees (current truth)

- Every user ID is a UUID v7 (`vo.NewID()` via `github.com/google/uuid`), time-ordered and globally unique. IDs are validated on parse; any non-UUID string returns `vo.ErrInvalidID`.
- The `Email` value object validates input with `net/mail.ParseAddress` at creation time; an invalid email returns `vo.ErrInvalidEmail` before the entity is constructed.
- The user entity (`internal/domain/user`) has zero external dependencies — no OTel, no `apperror`, no HTTP concepts.
- The subsystem exposes five use cases: Create, Get, Update, Delete, and List. All five are registered as HTTP routes (`POST /users`, `GET /users/:id`, `GET /users`, `PUT /users/:id`, `DELETE /users/:id`).
- Delete is a **soft delete** (`UPDATE users SET active = false`) — the row is not removed from the database. The repository span is named `db.update.users`, not `db.delete.users`, to reflect the actual DB operation.
- Get uses a **read-through cache** (optional, wired via `.WithCache()`). On a miss the result is fetched from the repository and stored. Update and Delete both **invalidate** the cache entry (`user:<id>`) after the mutation succeeds.
- Get protects against cache stampede via **singleflight** (optional, wired via `.WithFlight()`). Concurrent requests for the same ID share a single in-flight repository call; the shared outcome is recorded as a `singleflight.shared` span event.
- List executes COUNT and SELECT in a **read-only transaction** on the reader replica, ensuring the total count and the page of data are consistent with each other.
- List pagination is clamped by `ListFilter.Normalize()`: `Page` defaults to 1; `Limit` is clamped to `[1, 100]` with a default of 20.
- All use cases return `*apperror.AppError` (never raw domain errors) via the local `userToAppError()` function. The HTTP handler resolves errors generically via `errors.As` + `codeToStatus` map — it imports no domain packages.
- Expected errors (`vo.ErrInvalidEmail`, `vo.ErrInvalidID`, `user.ErrUserNotFound`, `user.ErrDuplicateEmail`) are classified as `telemetry.WarnSpan` (span stays `Ok`, a semantic attribute is recorded). All other errors are classified as `telemetry.FailSpan` (span marked `Error`, `error.type` attribute + stack trace recorded). The handler never calls `span.SetStatus()` or `span.RecordError()`.
- Idempotent writes are supported via the `Idempotency-Key` header. The middleware (wired globally in `router.Setup`) uses a SHA-256 fingerprint of the request body, a Redis-backed lock/unlock/complete cycle, and fail-open behaviour when Redis is unavailable. 5xx responses are not cached, allowing retries.
- The repository uses parameterised queries (sqlx `NamedExecContext` / `GetContext`) exclusively — no SQL string concatenation.

## Guaranteed Requirements

- USER-REQ-1: Every user ID is a time-ordered UUID v7; non-UUID strings are rejected before any repository call.
- USER-REQ-2: The `Email` value object validates format via `net/mail.ParseAddress`; invalid emails are rejected before the entity is created.
- USER-REQ-3: The domain layer (`internal/domain/user`, `internal/domain/user/vo`) has zero external dependencies.
- USER-REQ-4: Full CRUD is exposed via five HTTP endpoints (Create, Get, List, Update, Delete).
- USER-REQ-5: Delete is soft (sets `active = false`); the row is never removed from the database.
- USER-REQ-6: Get employs a read-through cache (optional); cache entries for a user are invalidated on Update and Delete.
- USER-REQ-7: Get is protected against cache stampede via singleflight; concurrent requests for the same ID share one in-flight repository call.
- USER-REQ-8: List runs COUNT and data SELECT inside a single read-only transaction to guarantee pagination consistency.
- USER-REQ-9: Use cases return `*apperror.AppError` via `userToAppError()`; the handler resolves errors generically with no domain imports.
- USER-REQ-10: Expected domain errors are classified as `WarnSpan` (span stays Ok); unexpected infra errors are classified as `FailSpan` (span marked Error with stack trace). Classification is the use case's sole responsibility.
- USER-REQ-11: POST writes support idempotency via the `Idempotency-Key` header (SHA-256 fingerprint, Redis lock/unlock, fail-open on store unavailability, 5xx not cached).
- USER-REQ-12: All repository queries use parameterised statements; SQL string concatenation is not used.

## Changelog

### ADDED 2026-06-21 / SDDX

- Capability doc created (current-truth snapshot).

## Related

- [[role]]
- [[idempotency]]
- [[caching]]
- Guide: [docs/guides/error-handling.md](../guides/error-handling.md)
- Guide: [docs/guides/observability.md](../guides/observability.md)
- ADR: [docs/adr/002-ids.md](../adr/002-ids.md)
- ADR: [docs/adr/009-error-handling.md](../adr/009-error-handling.md)
