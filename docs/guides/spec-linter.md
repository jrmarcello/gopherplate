# Deterministic Spec Linter (`tools/validate-spec`)

`tools/validate-spec` is the **deterministic encoding** of the structural rules in
[`.claude/rules/sdd.md`](../../.claude/rules/sdd.md). It runs as a gate *before* the
inferential review agents so that purely mechanical errors (missing sections, broken `depends:`
cycles, REQ↔TC gaps, off-format IDs) are caught cheaply and instantly — not after burning LLM
cycles on a structurally invalid spec.

This guide covers the day-to-day flow: what it checks, how callers interpret exit codes,
grandfathering, the `manifest` subcommand, and how `/spec` and `/ralph-loop` wire it as a gate.
For the full spec see [.specs/sdd-capabilities-and-validation.md](../../.specs/sdd-capabilities-and-validation.md)
(slug `SDDX`).

## Why a deterministic gate

The 3-agent self-review in `/spec` is inferential: it reads the spec semantically, spots boundary
TC gaps, checks Clean Architecture alignment, flags missing error paths. What it should *not* be
doing is counting whether every REQ has a TC, or verifying that `depends: [TASK-3]` points to a
task that actually exists. Those checks are decidable — they have a yes/no answer that doesn't
require a model. Handling them in a dedicated linter gives:

- **Determinism** — same spec, same result, every time. No "the reviewer missed it this run."
- **Speed** — milliseconds vs. seconds per agent call.
- **Division of labor** — the agents focus on semantic rigor (boundary TCs per field, both-branch
  coverage, infra-failure per dependency); the linter handles structural hygiene.

## Running the linter

```bash
# Lint all slug-bearing specs (skips grandfathered specs without ## Slug:)
make validate-spec

# Lint a specific file (runs structural validators on any spec, with or without a slug)
go run ./tools/validate-spec FILE=.specs/user-audit-log.md

# Or equivalently:
go run ./tools/validate-spec .specs/user-audit-log.md

# Print the union of all files: declared across tasks (used by /ralph-loop wrap-up)
make spec-files-audit FILE=.specs/user-audit-log.md
# Equivalent: go run ./tools/validate-spec files .specs/user-audit-log.md

# Detect capability↔code drift (## Code os.Stat + git-log staleness)
make capabilities-check
# Equivalent: go run ./tools/validate-spec capabilities docs/capabilities/

# Generate a skeleton capability doc for a package not yet documented
go run ./tools/validate-spec bootstrap-capability <pkg>
```

When invoked without a `FILE=` argument (or when run via `make validate-spec`), the tool globs
`.specs/**/*.md` and skips files without a `## Slug:` header — so pre-SDDX grandfathered specs
are not reported.

When invoked with an explicit file path, it still runs the **structural validators** (required
sections, status token format, task metadata shape) on any spec regardless of whether a slug is
declared. Slug-gated validators (prefix checks, capability link, coverage cross-reference) are
only activated when `## Slug:` is present. This means an explicit invocation *will* report a
malformed status token on a grandfathered spec.

## Validator list

Each validator is tagged with a severity: **ERROR** (exit code 1) or **WARNING** (exit code 0).

### Structural validators (run on every spec, slug or not)

| Validator | Severity | What it checks |
| --- | --- | --- |
| `requiredSections` | ERROR | `## Context`, `## Requirements`, `## Test Plan`, `## Design`, `## Tasks`, `## Parallel Batches`, `## Validation Criteria` all present |
| `statusToken` | ERROR | Exactly one `## Status: <TOKEN>` line; `<TOKEN>` in `{DRAFT, APPROVED, IN_PROGRESS, DONE, FAILED, SUPERSEDED, ARCHIVED}` |
| `taskFilesPresent` | ERROR | Every task block has at least one `- files:` sub-item (or the sentinel `(none — execution only)` for tasks that produce no file artifacts, e.g. pure smoke-test execution tasks) |
| `dependsDAG` | ERROR | `depends:` references form an acyclic directed graph of task IDs that exist in the same spec |
| `batchCoverage` | ERROR | Every task appears in exactly one batch |
| `batchFileOverlap` | ERROR | No two tasks in the same batch share a non-shared-additive file (would require serialization) |
| `batchOrder` | ERROR | Batch `N` contains no task that `depends:` on a task in batch `N` or later |
| `taskMissingTests` | WARNING | Production-`.go` tasks without a `tests:` sub-item (may be intentional, e.g. migration-only tasks) |

### Slug-gated validators (run only when `## Slug:` is declared)

| Validator | Severity | What it checks |
| --- | --- | --- |
| `slugFormat` | ERROR | `## Slug:` value matches `^[A-Z][A-Z0-9]*$` |
| `reqIDFormat` | ERROR | All REQ IDs in `## Requirements` follow `<SLUG>-REQ-N` |
| `tcIDFormat` | ERROR | All TC IDs in `## Test Plan` follow `<SLUG>-TC-<TYPE>-NN` (two-digit `NN`) |
| `tcTypeRegistry` | ERROR | TC `<TYPE>` is in the registered set: `D`, `UC`, `E2E`, `S`, `SH`, `CT` — no ad-hoc types |
| `idUniqueness` | ERROR | All REQ IDs and TC IDs are globally unique within the spec |
| `reqCoverage` | ERROR | Every REQ has at least one TC referencing it, OR carries a `(no-test: <reason>)` annotation (non-empty reason) |
| `tcRefValid` | ERROR | Every TC's `REQ` column references a REQ ID that exists in `## Requirements` |
| `capabilityLink` | WARNING | Slug-bearing spec has no `§ Impacted Capability` link in `## Design` (the `/ralph-loop` wrap-up needs it) |
| `testsRefValid` | ERROR | Every TC ID listed in a task's `tests:` sub-item exists in `## Test Plan` |
| `tcReferenced` | ERROR | Every TC declared in `## Test Plan` is referenced by at least one task's `tests:` sub-item |

### Named-task matcher

The linter recognises a set of special task names that are **execution-only** and are exempt from
certain structural requirements (e.g. the `batchCoverage` check treats them as first-class tasks):

- `TASK-SMOKE` — dedicated smoke-test execution task (runs k6; produces no source file)
- `TASK-MERGE-*` — accumulator merge task (reads wiring fragments; the shared file it writes
  is listed in its own `files:`, not in the parallel producers)
- `TASK-FINAL` — wrap-up task (documentation updates, manifest regeneration, etc.)

These names are matched literally; casing must be exact.

### The `(none — execution only)` files sentinel

A task that produces no file artifacts (e.g. `TASK-SMOKE` or a pure validation step) may
declare:

```text
- files: (none — execution only)
```

`taskFilesPresent` accepts this sentinel and does not report a missing `files:` sub-item.
The sentinel must be written verbatim — any other variation is treated as a file path.

### The `(no-test: <reason>)` exemption

Documentation-only REQs that cannot be covered by an automated test may carry an inline
annotation on the same line as their declaration:

```text
- AUDIT-REQ-5: The audit log format MUST be documented. (no-test: documentation-only REQ)
```

`reqCoverage` skips these REQs. The reason must be non-empty — `(no-test:)` alone is an ERROR.

## Exit codes

| Code | Meaning | When emitted |
| --- | --- | --- |
| `0` | Clean or warn-only | All ERROR validators passed; WARNINGs are printed but do not fail |
| `1` | Lint ERROR | At least one ERROR validator fired |
| `2` | Tool error | I/O failure (file not found, unreadable), usage error (unknown flag), or internal parse failure unrelated to spec content |

Callers must distinguish code `1` (spec needs fixing) from code `2` (tool or environment
problem). In `/spec` and `/ralph-loop` a code `2` surfaces as "linter tool error — environment
issue, not a spec problem" and asks the user to investigate before retrying.

## Grandfathering

Specs written before slug-prefixed IDs were introduced (spec `SDDX`) are **grandfathered**: they
have no `## Slug:` header and do not need to be retrofitted. The linter's slug-gated validators
are skipped for them.

When invoked via `make validate-spec` (glob mode), grandfathered specs are skipped entirely —
they do not appear in output at all. When invoked with an explicit path
(`go run ./tools/validate-spec .specs/old-spec.md`), the structural validators still run
(so a malformed `## Status:` token on a grandfathered spec *will* be reported), but the slug,
prefix, coverage, and capability validators are skipped.

**Do NOT retrofit pre-SDDX specs** to add slugs. Leave them as-is; they are historical records.

## TC-prefix registry

Canonical TYPE values for `<SLUG>-TC-<TYPE>-NN`:

| TYPE | Scope | Example ID |
| --- | --- | --- |
| `D` | Domain: pure entity/VO/error logic | `AUDIT-TC-D-01` |
| `UC` | Use case: application logic, mock interactions, error mapping | `AUDIT-TC-UC-03` |
| `E2E` | End-to-end: full HTTP round-trip via TestContainers | `AUDIT-TC-E2E-01` |
| `S` | Smoke: k6-based functional validation of deployed endpoints | `AUDIT-TC-S-01` |
| `SH` | Shell/hook: harness and tooling shell tests (harness/tooling specs only) | `SDDX-TC-SH-01` |
| `CT` | Contract: cross-boundary contract tests (harness/tooling specs only) | `SDDX-TC-CT-02` |

Any other TYPE is a lint ERROR. To add a new TYPE, register it here AND in `tools/validate-spec`
(update the allowed-types constant) in the same PR — never invent one ad hoc in a spec.

## The `manifest` subcommand

```bash
# Regenerate docs/capabilities/MANIFEST.md (committed index of all capability docs)
make capabilities-manifest

# Equivalent direct invocation:
go run ./tools/validate-spec manifest --write
```

The `manifest` subcommand reads every `docs/capabilities/*.md` file (excluding `README.md` and
`MANIFEST.md` itself), extracts the slug, status, guaranteed REQ IDs, and linked specs/ADRs, and
writes `docs/capabilities/MANIFEST.md` as a Markdown table. This file is **committed** (not
gitignored), so a `git diff docs/capabilities/MANIFEST.md` on a PR reveals exactly which
capability guarantees changed.

`MANIFEST.md` is **never hand-edited** — it is the output of `make capabilities-manifest`.
The `/ralph-loop` wrap-up runs this command automatically after updating the impacted capability
doc.

## The `files` subcommand

```bash
# Print the declared-files union for a spec (used by /ralph-loop wrap-up files-vs-diff audit)
make spec-files-audit FILE=.specs/user-audit-log.md

# Equivalent direct invocation:
go run ./tools/validate-spec files .specs/user-audit-log.md
```

The `files` subcommand reads every task's `files:` sub-items and prints the deduplicated union.
The `/ralph-loop` wrap-up runs this command after executing all tasks and diffs the output against
the actual working-tree diff. Any file present in the diff but absent from the declared union is
reported as **MUST FIX** — the task's `files:` metadata must be corrected before commit.

## The `capabilities` subcommand

```bash
# Detect capability↔code drift across all capability docs
make capabilities-check

# Equivalent direct invocation:
go run ./tools/validate-spec capabilities docs/capabilities/
```

The `capabilities` subcommand inspects every `docs/capabilities/*.md` file and:

1. **`os.Stat` check** — for each path listed under `## Code`, verifies the path exists in the
   working tree. A missing path is an ERROR (the capability doc references code that no longer exists).
2. **Staleness check** — runs `git log --since` against the `Last-verified` marker in each doc.
   If the referenced code has commits newer than `Last-verified`, emits a WARNING that the doc
   may be stale and should be re-reviewed.

### The `## Code` section and `Last-verified` marker

Every capability doc must carry a `## Code` section listing the primary source paths that
implement the described guarantees:

```markdown
## Code

- `internal/usecases/user/` — use cases
- `pkg/cache/` — cache interface and Redis implementation

Last-verified: 2026-06-01
```

The `Last-verified` date is updated by the author whenever the doc is reviewed against the
actual code. `make capabilities-check` uses this date for the staleness heuristic.

## The `bootstrap-capability` subcommand

```bash
# Generate a skeleton capability doc for a package not yet in docs/capabilities/
go run ./tools/validate-spec bootstrap-capability <pkg>
# Example: go run ./tools/validate-spec bootstrap-capability pkg/idempotency
```

Generates a pre-filled `docs/capabilities/<pkg-name>.md` skeleton using the project's
capability doc template (`docs/capabilities/TEMPLATE.md`). The skeleton includes the correct
frontmatter, all required sections (`## Overview`, `## Guarantees`, `## Code`,
`## Non-guarantees`, `## Changelog`), and a `Last-verified` marker set to today. Edit the
skeleton to fill in the actual guarantees before committing.

## How `/spec` and `/ralph-loop` wire the gate

### In `/spec`

After the **Author** phase (spec written, slug declared, capability link added in Design):

1. Run `make validate-spec FILE=.specs/<name>.md` (or `go run ./tools/validate-spec <path>`).
2. If exit code 1 (lint ERROR): print the errors, fix them inline, re-run the linter. Repeat
   until exit code 0. **Do not call the review agents with a failing spec.**
3. If exit code 2 (tool error): surface as "linter environment issue" and stop — do not proceed
   to review.
4. If exit code 0: proceed to the **4-agent self-review** (`spec-reviewer`, `test-reviewer`,
   `code-reviewer`, `security-reviewer`). The fourth lens (`security-reviewer`) was added in
   spec SDDF to catch threat-model gaps and insecure defaults at authoring time, before any code
   is written.

WARNINGs (exit 0) are printed and surfaced in the "Pontos de atenção" section of the Present
output, alongside findings from the review agents.

### In `/ralph-loop`

At the start of the **Validate** phase (before executing any task):

1. Run `make validate-spec FILE=.specs/<name>.md`.
2. If exit code 1 or 2: **refuse to execute**. Surface the errors and ask the user to fix the
   spec first. This prevents partial execution on a structurally invalid spec.
3. If exit code 0: proceed to Execute.

The linter is not re-run between batches (the spec's Requirements and Test Plan sections are
immutable during `IN_PROGRESS`).

During the **Wrap-up** phase (after all tasks complete):

1. Run `make spec-files-audit FILE=.specs/<name>.md` to get the declared-files union.
2. Diff against the actual working-tree diff. Any file in the diff but absent from the union is
   reported as **MUST FIX** — the corresponding task's `files:` must be updated before committing.
   This is the files-vs-diff audit added in spec SDDF.
3. Run `make capabilities-manifest` (regenerates `docs/capabilities/MANIFEST.md`).
4. If the impacted capability doc's `## Code` paths have changed, update the doc's `Last-verified`
   date so `make capabilities-check` does not immediately flag the updated doc as stale.

## Module placement

`tools/validate-spec` lives in the **main module** (same `go.mod` as the service), unlike
`tools/learn` which has its own module. Consequences:

- It is covered by `golangci-lint run ./...` — no separate lint step needed.
- It is included in the unit-test and coverage scope (`go test ./...` covers it).
- CI builds and tests it as part of the normal `ci.yml::unit-tests` job. CI does **not** run
  the linter against actual specs (that would make spec authoring a CI gate, which is not the
  intent — it is a local dev-time gate wired into `/spec` and `/ralph-loop`).
- `go run ./tools/validate-spec` works from the repo root without any module setup.

## References

- [docs/harness.md](../harness.md) — `tools/validate-spec` is listed as a `sensor / C / meta`
  in the "Spec tooling" section of the harness inventory.
- [docs/capabilities/README.md](../capabilities/README.md) — capability docs: lifecycle,
  amend-in-place, manifest.
- [docs/guides/sdd-ralph-loop.md](sdd-ralph-loop.md) — full SDD flow narrative (how the linter
  gates fit into `/spec` and `/ralph-loop`).
- [.claude/rules/sdd.md](../../.claude/rules/sdd.md) — canonical rules (§ Deterministic Spec
  Linter, § Slug & ID naming, § Capability Docs).
- [.specs/sdd-capabilities-and-validation.md](../../.specs/sdd-capabilities-and-validation.md)
  — spec `SDDX`: full test plan and rationale for the linter and capability docs.
