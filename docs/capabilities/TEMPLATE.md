# Capability: <name>

## Slug: <UPPER>

<!-- Short uppercase identifier, ^[A-Z][A-Z0-9]*$ — prefixes the guaranteed-REQ IDs below.
     Distinct from any spec slug; keeps cross-references unambiguous. -->

## Status: Active

<!-- Active | Superseded | Archived. If Superseded, add a "## Superseded-by" link section. -->

## Source

<!-- Where these guarantees came from. -->

- Specs: <slug(s), e.g. SDDX — or "n/a — pre-SDD">
- ADRs: <links to docs/adr/*.md, or "n/a">

## Guarantees (current truth)

<!-- The invariants this subsystem upholds TODAY. Present tense. Each one verifiable against
     the code. This is the durable contract — not a how-to, not a decision record. -->

- <invariant 1>
- <invariant 2>

## Guaranteed Requirements

<!-- Slug-prefixed, stable IDs for the guarantees above, so other docs/specs can reference them. -->

- <UPPER>-REQ-1: <one-line guarantee>
- <UPPER>-REQ-2: <one-line guarantee>

## Changelog

<!-- Amend-in-place: append here when the capability evolves. Never spawn a sibling doc.
     Use ADDED / MODIFIED / REMOVED headers, newest first, tagged with the date and the
     spec slug (or commit) that drove the change. -->

### ADDED <YYYY-MM-DD / spec-slug>

- <what was added>

### MODIFIED <YYYY-MM-DD / spec-slug>

- <what changed>

### REMOVED <YYYY-MM-DD / spec-slug>

- <what was removed>

## Related

<!-- Link sibling capabilities ([[slug]]), guides, and ADRs. -->

- [[<other-capability>]]
- Guide: <docs/guides/*.md>
- ADR: <docs/adr/*.md>
