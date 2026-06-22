# Capability: Idempotency

## Slug: IDEM

## Status: Active

## Source

- Specs: n/a — pre-SDD
- ADRs: [docs/adr/008-api-response-format.md](../adr/008-api-response-format.md)

## Code

- pkg/idempotency/ — Store interface + Redis implementation (SHA-256 fingerprint, lock/unlock)
- internal/infrastructure/web/middleware/idempotency.go — middleware (POST dedup, 409 on contention, 5xx-not-cached, fail-open)

Last-verified: 2026-06-21 (472fbb9)

## Guarantees (current truth)

- The `Idempotency-Key` header is the opt-in signal; if the header is absent, the request is processed normally with no deduplication.
- The middleware applies only to `POST` requests; other HTTP methods are passed through unchanged.
- On the first request with a given key, the request body is hashed with SHA-256 and stored as a fingerprint alongside a `PROCESSING` lock in Redis (via `SET NX`) before the handler executes.
- The Redis lock uses a short TTL (`lockTTL`, e.g. 30 s) to prevent orphaned locks; completed entries use a longer TTL (e.g. 24 h) configured at construction time.
- If the store is unavailable (Redis error), the middleware fails open: the request is forwarded to the handler as though no key was provided, and a warning span event plus a structured log entry are emitted.
- Concurrent requests arriving with the same key while the first is still `PROCESSING` receive `409 Conflict` immediately; the first request is not interrupted.
- A completed replay returns the stored HTTP status code and body verbatim, with `Content-Type: application/json`.
- If a replay request body produces a different SHA-256 fingerprint from the stored one, the middleware returns `422 Unprocessable Entity` — the key cannot be reused with a different body.
- Responses with status codes in the range 200–499 (inclusive) are stored for replay. Responses with status codes ≥ 500 are NOT stored; the lock is instead deleted (`Unlock`) so that the caller may retry the same key after a transient failure.
- The Redis key is namespaced: `idempotency:{X-Service-Name}:{key}` when `X-Service-Name` is present; `idempotency:{key}` otherwise.
- The idempotency `Store` interface is defined in `pkg/idempotency`; the Redis implementation lives in `pkg/idempotency/redisstore`. The middleware depends only on the interface, not on the concrete Redis type.

## Guaranteed Requirements

- IDEM-REQ-1: The `Idempotency-Key` header is optional; absent header results in normal (non-deduplicated) request processing.
- IDEM-REQ-2: Only `POST` requests are subject to idempotency deduplication.
- IDEM-REQ-3: The first request for a key acquires a Redis `SET NX` lock and stores a SHA-256 fingerprint of the request body before the handler executes.
- IDEM-REQ-4: Concurrent requests for a key whose lock is in `PROCESSING` state receive `409 Conflict` without interrupting the in-flight request.
- IDEM-REQ-5: A completed replay replays the original HTTP status code and body verbatim.
- IDEM-REQ-6: Replaying a key with a body whose SHA-256 fingerprint differs from the stored fingerprint returns `422 Unprocessable Entity`.
- IDEM-REQ-7: Responses in the 200–499 status range are stored for replay; responses with status ≥ 500 are NOT stored — the lock is deleted so the caller may retry the same key.
- IDEM-REQ-8: Redis unavailability causes fail-open behaviour: the request proceeds normally and a warning span event is recorded.
- IDEM-REQ-9: The Redis key is namespaced by `X-Service-Name` when that header is present.

## Changelog

### ADDED 2026-06-21 / SDDX

- Capability doc created; guarantees extracted from `pkg/idempotency/store.go`, `pkg/idempotency/redisstore/store.go`, and `internal/infrastructure/web/middleware/idempotency.go`.

## Related

- [[caching]]
- [[user]]
- Guide: [docs/guides/error-handling.md](../guides/error-handling.md)
- ADR: [docs/adr/008-api-response-format.md](../adr/008-api-response-format.md)
- ADR: [docs/adr/005-service-key-auth.md](../adr/005-service-key-auth.md)
