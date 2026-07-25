# Capability: Caching

## Slug: CACHE

## Status: Active

## Source

- Specs: n/a — pre-SDD
- ADRs: [docs/adr/007-pkg-reusable-packages.md](../adr/007-pkg-reusable-packages.md)

## Code

- pkg/cache/ — Cache interface
- pkg/cache/redisclient/ — nil-safe Redis implementation
- pkg/cache/singleflight.go — singleflight anti-stampede wrapper
- internal/usecases/user/get.go — read-through cache + singleflight wiring (.WithCache / .WithFlight)

Last-verified: 2026-06-21 (472fbb9)

## Guarantees (current truth)

- The `Cache` interface (`pkg/cache/cache.go`) defines driver-agnostic `Get`, `Set`, `Delete`, `Ping`, and `Close` operations; use cases depend only on this interface, never on a concrete driver.
- The Redis implementation (`pkg/cache/redisclient`) is nil-safe: all methods check `r == nil` first and return a no-op result (cache miss on `Get`, nil error on `Set`/`Delete`/`Close`), so callers never need a nil guard.
- When `REDIS_ENABLED=false` (or the `Enabled` config flag is false), `NewRedisClient` returns `nil, nil` — the nil client satisfies the nil-safe contract above.
- Cache is an OPTIONAL dependency in the user `GetUseCase`: it is wired via the `.WithCache()` builder method. Absence of the cache degrades gracefully to direct repository reads with no error.
- `FlightGroup` (`pkg/cache/singleflight.go`) wraps `golang.org/x/sync/singleflight` to deduplicate concurrent backend calls for the same key: only the first goroutine to call `Do(key, fn)` executes `fn`; all others block and receive the same result when it completes.
- `FlightGroup` is an OPTIONAL dependency in the user `GetUseCase`, wired via `.WithFlight()`. Absence means no stampede protection; each concurrent cache-miss issues its own repository call.
- The user `GetUseCase` applies cache and singleflight in order: (1) cache lookup, (2) on miss, singleflight-protected repository fetch, (3) cache set of the result. A span event (`cache.hit`, `cache.miss`, `cache.set`, `singleflight.shared`) is recorded for each branch.
- A `cache.set` failure after a repository fetch is non-fatal: the use case logs a span event (`cache.set_failed`) and returns the successfully fetched entity to the caller.
- Cache TTL is configured at construction time (`RedisConfig.TTL`); the default when the TTL string is unparseable is 5 minutes.
- The idempotency subsystem uses a separate `*redis.Client` obtained via `RedisClient.UnderlyingClient()` and manages its own TTL and key namespace; it does not share the `Cache` interface.

## Guaranteed Requirements

- CACHE-REQ-1: The `Cache` interface is the sole dependency point for use-case caching; no use case imports a concrete cache driver.
- CACHE-REQ-2: The Redis implementation is nil-safe; a nil `*RedisClient` is a valid no-op cache (miss on reads, success on writes).
- CACHE-REQ-3: Cache is wired as an optional dependency via `.WithCache()`; its absence causes graceful degradation to direct repository reads.
- CACHE-REQ-4: `FlightGroup.Do` ensures that concurrent goroutines presenting the same key execute the wrapped function exactly once; all share the result of the single execution.
- CACHE-REQ-5: Singleflight is wired as an optional dependency via `.WithFlight()`; its absence causes no error — each concurrent cache-miss issues its own repository call.
- CACHE-REQ-6: The user `GetUseCase` read path is: cache lookup → (miss) singleflight-protected repository fetch → cache set. Each branch emits an OTel span event.
- CACHE-REQ-7: A failure to write to cache after a successful repository fetch is non-fatal; the fetched entity is returned to the caller.
- CACHE-REQ-8: Cache TTL defaults to 5 minutes when the configured TTL string cannot be parsed.

## Changelog

### ADDED 2026-06-21 / SDDX

- Capability doc created; guarantees extracted from `pkg/cache/cache.go`, `pkg/cache/redisclient/client.go`, `pkg/cache/singleflight.go`, and `internal/usecases/user/get.go`.

## Related

- [[idempotency]]
- [[user]]
- Guide: [docs/guides/observability.md](../guides/observability.md)
- ADR: [docs/adr/007-pkg-reusable-packages.md](../adr/007-pkg-reusable-packages.md)
