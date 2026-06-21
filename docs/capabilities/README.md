# Capability Docs

This directory holds the **living, current-truth guarantees** of each subsystem in this
service — *what it guarantees right now*, expressed as verifiable invariants. It is the
authoritative answer to "what does this service promise today, and which specs/ADRs prove it?"

It is the gopherplate adaptation of OpenSpec's "spec is the living source of truth" concept,
imported as a thin local convention (no OpenSpec tooling). See
[.specs/sdd-capabilities-and-validation.md](../../.specs/sdd-capabilities-and-validation.md)
(slug `SDDX`) for the rationale.

## What a capability doc IS (and is NOT)

| Artifact | Question it answers | Lifecycle |
| -------- | ------------------- | --------- |
| **Capability doc** (`docs/capabilities/*.md`) | *What does this subsystem guarantee NOW?* — invariants/contracts, present tense | Living; amended in place |
| Guide (`docs/guides/*.md`) | *How do I use/build this?* — tutorial/explanation | Living |
| ADR (`docs/adr/*.md`) | *Why was this decision made?* — point-in-time rationale | Immutable (can be `Superseded`) |
| Spec (`.specs/*.md`) | *What is the plan for THIS change?* — one feature/change | Ephemeral; ages into history once `DONE` |

A capability doc is **not** a how-to, **not** a decision record, and **not** a per-change
ticket. It is the durable contract that a human (or a future agent) can audit against the
deployed code — the one place where the invariants that specs weave in at every gate are
written down as current truth, instead of being reconstructed from code plus a pile of `DONE`
specs read as history.

### Why this matters here

In this repo `.specs/*.md` are version-controlled, but they are still per-change tickets that
reach `DONE` and stop being maintained. Without capability docs, the only way to learn what
`user`, `idempotency`, or `caching` guarantee today is to reverse-engineer the code plus the
specs that aged around it. Capability docs close that drift gap at the documentation layer.

## Lifecycle

```text
Active  →  Superseded  →  Archived
```

- **Active** — the subsystem exists and the doc reflects its current guarantees.
- **Superseded** — a newer capability replaces this one; the doc gains a forward link
  (`Superseded-by:`), but is kept for history.
- **Archived** — the subsystem was removed; the doc is retained as a record.

The lifecycle mirrors the SDD spec state machine's new terminal states
(`SUPERSEDED`/`ARCHIVED`) — see [.claude/rules/sdd.md](../../.claude/rules/sdd.md).

## Amend-in-place (do NOT spawn `fase-N+1` siblings)

A capability **evolves in place**: when the subsystem's behavior changes, you edit its
canonical doc and append a `Changelog` entry using `ADDED` / `MODIFIED` / `REMOVED` prose
headers. You do **not** create `caching-v2.md` or `caching-fase2.md`. This is the documentation
counterpart of the SDD amend-in-place rule, and it directly counters the phase-proliferation
anti-pattern (sibling specs accreting around one capability).

### Decision rule — amend vs. write a new spec

- **Amend the capability doc** (+ append its Changelog) when an *existing* subsystem's behavior
  changes: no new module, no new endpoint, no new approval gate needed.
- **Write a new spec** when a *new* subsystem is introduced, or the change is large enough to
  warrant its own approval gate and TDD cycle. The spec, when `DONE`, updates the relevant
  capability doc as a first-class audited artifact (the `/ralph-loop` wrap-up does this).

## Format

Every capability doc follows [TEMPLATE.md](TEMPLATE.md): a `Slug`, a `Status`, a `Source`
(specs/ADRs that built it), a `Guarantees (current truth)` section of present-tense invariants,
a slug-prefixed `Guaranteed Requirements` list, a `Changelog`, and `Related` links.

## Manifest

[MANIFEST.md](MANIFEST.md) is **generated** (not hand-maintained) from these docs by
`make capabilities-manifest` (`tools/validate-spec manifest --write`). It is the committed index
of capability → slug → status → guaranteed REQs → linked specs/ADRs; a diff on it reveals drift.
Do not edit it by hand.

## The four seeded examples

This repo ships four worked capability docs as the pattern reference — the two example domains
plus the two most illustrative cross-cutting subsystems:

- [user.md](user.md) — full CRUD domain with cache, singleflight, idempotency.
- [role.md](role.md) — the minimal multi-domain DI example.
- [idempotency.md](idempotency.md) — SHA-256 fingerprint, lock/unlock, replay, 5xx-not-cached.
- [caching.md](caching.md) — cache interface + Redis + singleflight anti-stampede.

Remaining subsystems are documented over time via amend-in-place as specs touch them.
