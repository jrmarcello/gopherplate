# Capability: Role

## Slug: ROLE

## Status: Active

## Source

- Specs: n/a — pre-SDD (template example domain, multi-domain DI reference)
- ADRs:
  - [docs/adr/002-ids.md](../adr/002-ids.md) — UUID v7 identifier strategy
  - [docs/adr/008-api-response-format.md](../adr/008-api-response-format.md) — standardised HTTP response envelope
  - [docs/adr/009-error-handling.md](../adr/009-error-handling.md) — error classification in the use case layer

## Code

- internal/domain/role/ — entity, sentinel errors
- internal/usecases/role/ — Create/List/Delete use cases, error mapping
- internal/infrastructure/db/postgres/repository/role.go — sqlx repository (hard delete, read-only list tx)
- internal/infrastructure/web/handler/role.go — HTTP handler
- internal/infrastructure/web/router/role.go — route registration

Last-verified: 2026-06-21 (472fbb9)

## Guarantees (current truth)

- Every role ID is a UUID v7 (shared `vo.NewID()` from `internal/domain/user/vo`), time-ordered and globally unique. The ID value object is reused from the `user` domain — role has no separate `vo` package.
- The role entity (`internal/domain/role`) has zero external dependencies — no OTel, no `apperror`, no HTTP concepts.
- The subsystem exposes three use cases: Create, List, and Delete. All three are registered as HTTP routes (`POST /roles`, `GET /roles`, `DELETE /roles/:id`). There is no Get-by-ID or Update endpoint.
- Delete is a **hard delete** (`DELETE FROM roles WHERE id = $1`) — the row is physically removed. This is the deliberate contrast with the user domain's soft delete.
- Create enforces name uniqueness by calling `FindByName` before inserting. A duplicate name returns `role.ErrDuplicateRoleName` regardless of whether the uniqueness constraint is triggered at the DB level.
- The subsystem intentionally has **no cache and no singleflight**. It is the simpler multi-domain DI reference, showing that optional capabilities (`WithCache`, `WithFlight`) are additive and not required by the pattern.
- List executes COUNT and SELECT in a **read-only transaction** on the reader replica, ensuring the total count and the page of data are consistent with each other.
- List pagination is clamped by `ListFilter.Normalize()`: `Page` defaults to 1; `Limit` is clamped to `[1, 100]` with a default of 20. The role `ListFilter` supports only `Name` as a filter field (no `ActiveOnly`).
- All use cases return `*apperror.AppError` (never raw domain errors) via the local `roleToAppError()` function. The HTTP handler resolves errors generically via `errors.As` + `codeToStatus` map — it imports no domain packages.
- Expected errors (`vo.ErrInvalidID`, `role.ErrRoleNotFound`, `role.ErrDuplicateRoleName`) are classified as `telemetry.WarnSpan` (span stays `Ok`, a semantic attribute is recorded). All other errors are classified as `telemetry.FailSpan` (span marked `Error`, `error.type` attribute + stack trace recorded). The handler never calls `span.SetStatus()` or `span.RecordError()`.
- The repository uses parameterised queries (sqlx `NamedExecContext` / `GetContext` / `ExecContext`) exclusively — no SQL string concatenation.

## Guaranteed Requirements

- ROLE-REQ-1: Every role ID is a time-ordered UUID v7 (shared `vo.ID` from `internal/domain/user/vo`); non-UUID strings are rejected before any repository call.
- ROLE-REQ-2: The domain layer (`internal/domain/role`) has zero external dependencies.
- ROLE-REQ-3: Three HTTP operations are exposed: Create (`POST /roles`), List (`GET /roles`), and Delete (`DELETE /roles/:id`). No Get-by-ID or Update endpoint exists.
- ROLE-REQ-4: Delete is a hard delete; the row is physically removed from the database.
- ROLE-REQ-5: Create checks name uniqueness via `FindByName` before insert; a duplicate name returns `role.ErrDuplicateRoleName`.
- ROLE-REQ-6: The subsystem has no cache and no singleflight; it is the intentionally minimal multi-domain DI example.
- ROLE-REQ-7: List runs COUNT and data SELECT inside a single read-only transaction to guarantee pagination consistency.
- ROLE-REQ-8: Use cases return `*apperror.AppError` via `roleToAppError()`; the handler resolves errors generically with no domain imports.
- ROLE-REQ-9: Expected domain errors are classified as `WarnSpan` (span stays Ok); unexpected infra errors are classified as `FailSpan` (span marked Error with stack trace). Classification is the use case's sole responsibility.
- ROLE-REQ-10: All repository queries use parameterised statements; SQL string concatenation is not used.

## Changelog

### ADDED 2026-06-21 / SDDX

- Capability doc created (current-truth snapshot).

## Related

- [[user]]
- Guide: [docs/guides/error-handling.md](../guides/error-handling.md)
- Guide: [docs/guides/observability.md](../guides/observability.md)
- ADR: [docs/adr/002-ids.md](../adr/002-ids.md)
- ADR: [docs/adr/009-error-handling.md](../adr/009-error-handling.md)
