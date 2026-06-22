---
applies-to: ".specs/**"
---

# SDD Spec Rules

## 🎯 Princípio diretor: qualidade > velocidade > custo

**Antes de qualquer decisão tomada nesse fluxo (autoria de spec, escolha de design,
triagem de findings de reviewers, escolha de TC coverage, etc.):** aplicar a máxima
do projeto pinada em [CLAUDE.md](../../CLAUDE.md) e
[memory](../../../.claude/projects/-Users-marcelojr-Development-Workspace-gopherplate/memory/feedback_quality_first.md).

Implicações concretas pra SDD:

- **Authoring de spec:** quando confrontado com "esta REQ é boa o suficiente?",
  perguntar "esta é a versão **certa** desta REQ?". DI required > optional com
  default mágico. Error classes com contexto rico > strings genéricas. Retry/backoff
  > "user re-roda". Span classification explícita > silence em paths de erro.
  Idempotência em writes > "user fica de olho em duplicatas".
- **Triagem de self-review findings:** **NICE TO HAVE não é descartável**.
  Revisitar cada NICE TO HAVE pela lente "isso é rigor, ou é polish?". Rigor →
  upgrade pra MUST. Polish → defer com justificativa explícita.
- **Multiple review rounds são feature, não bug.** Se ainda houver findings após
  Round 2, rodar Round 3 (e Round N) sem hesitar. Cada round refina. Custo de
  tokens é insignificante perto de spec defeituosa virando código defeituoso.
- **TC coverage:** quando em dúvida entre "cobrir" e "skipar", **cobrir**.
  Boundary TCs, infra-failure TCs, branch-both-paths TCs, concurrency TCs, version
  evolution paths — tudo isso é rigor obrigatório, não opcional.
- **Trade-off transparency:** se decidir pragmático em vez de "melhor",
  **documentar explicitamente na spec** com justificativa (e.g., "Idempotência via
  Redis lock seria mais robusta que UNIQUE constraint, mas a tabela tem volume
  baixo o suficiente que constraint é defensável — ADR-XX"). Não esconder.

## Flow

```text
/spec <description>
   ├─ Author: write .specs/<name>.md from TEMPLATE (declare ## Slug:, slug-prefixed IDs, link impacted capability)
   ├─ Lint: tools/validate-spec (deterministic gate) — block on ERROR, BEFORE the agents
   ├─ Self-review: 4 agents in parallel (spec-reviewer, test-reviewer, code-reviewer, security-reviewer)
   ├─ Apply trivial fixes inline
   └─ Present + wait for user approval (status DRAFT)

   (user approves)

/ralph-loop .specs/<name>.md
   ├─ Validate: status APPROVED/IN_PROGRESS, batches present, tools/validate-spec passes (gate)
   ├─ Execute: per-batch parallel via worktrees (one Agent call per task in a single message)
   ├─ Self-review: 3 agents in parallel (code-reviewer, test-reviewer, security-reviewer)
   ├─ Wrap-up: update impacted docs/capabilities/*.md + make capabilities-manifest
   ├─ Apply trivial fixes inline + re-validate
   └─ Present + wait for user approval (status DONE pending commit)

   (user approves)

/ralph-loop commits with feat(scope) message linking to the spec.
```

There is **no Stop-hook iteration**, no `.active.md` state files, no per-task
pauses. The autonomy boundary is per-spec: one approval to author, one approval to
commit.

## Spec File Integrity

- Never modify the Requirements section during execution (only during DRAFT status)
- Never remove tasks — mark them as `[x]` (done) or `BLOCKED`
- Always append to Execution Log, never overwrite previous entries
- Status transitions: `DRAFT -> APPROVED -> IN_PROGRESS -> DONE | FAILED`; a `DONE` spec may
  further move to `SUPERSEDED` (replaced by another spec — add a `Superseded-by:` note) or
  `ARCHIVED` (the work was retired). `SUPERSEDED`/`ARCHIVED` are terminal and immutable. The ad-hoc
  `.specs/archive/<name>.md` move on rejection (see `spec/SKILL.md`) is the informal precursor of
  `ARCHIVED` — prefer setting `## Status: ARCHIVED` and keeping the file in place.
- **Status header format**: exactly one `## Status: <STATE>` line, a single canonical token (no
  inline qualifier — move "(MVP; N blocked)"-style notes to the Execution Log). `tools/validate-spec`
  enforces this.
- **Amend-in-place over sibling specs**: to evolve an existing subsystem's guarantees, edit its
  canonical capability doc (+ append its Changelog) and, when re-opening a spec, append to the
  Execution Log — do NOT spawn `fix-<slug>` / `fase-N+1` sibling specs. Reserve a new spec for a
  genuinely new subsystem, or a change large enough to warrant its own approval + TDD cycle (see
  § Capability Docs).

## Capability Docs

`docs/capabilities/*.md` hold the **living, current-truth guarantees** of each subsystem (what it
guarantees NOW, as verifiable invariants) — distinct from guides (how-to), ADRs (why, immutable),
and specs (the plan for one change, which ages into history). See
[docs/capabilities/README.md](../../docs/capabilities/README.md) and
`docs/capabilities/TEMPLATE.md`.

- A capability doc has `## Slug:`, `## Status: Active | Superseded | Archived`, `## Source`
  (specs/ADRs), a `## Code` section (source paths + a `Last-verified: <date> (<commit>)` marker),
  `## Guarantees (current truth)`, a slug-prefixed `## Guaranteed Requirements` list, and a
  `## Changelog` (`ADDED`/`MODIFIED`/`REMOVED`).
- **Drift check**: `tools/validate-spec capabilities docs/capabilities` (`make capabilities-check`)
  `os.Stat`s every `## Code` path (a dead path is an ERROR) and WARNs when a path's latest git
  commit is newer than `Last-verified` (an accidental-drift signal, not an adversarial guard).
  `tools/validate-spec bootstrap-capability <pkg>` emits a skeleton (source paths + sentinel errors
  auto-filled, the WHY left as TODO) to seed a new doc.
- **Amend-in-place**: evolve a capability by editing its doc + appending its Changelog — never
  `caching-v2.md` / `caching-fase2.md`.
- **Decision rule (amend vs. new spec)**: amend the capability doc when an *existing* subsystem's
  behavior changes (no new module/endpoint); write a new spec when a *new* subsystem is introduced
  or the change warrants its own approval + TDD cycle.
- **Flow wiring**: `/spec` links the impacted capability doc in the spec's Design (§ Impacted
  Capability); `/ralph-loop`'s wrap-up updates that doc (current-truth + Changelog) as a
  first-class audited artifact and regenerates the manifest (`make capabilities-manifest`); the
  `spec-reviewer`/`code-reviewer` check that the capability doc was updated when behavior changed.
- `docs/capabilities/MANIFEST.md` is **generated** (`tools/validate-spec manifest --write`), never
  hand-edited.

## Deterministic Spec Linter (gate)

`tools/validate-spec` (a main-module Go tool, run via `go run ./tools/validate-spec` or
`make validate-spec`) is the **deterministic** encoding of the structural rules in this file. It
runs as a gate BEFORE the inferential review agents: `/spec` runs it before the 4-lens
self-review (block on ERROR); `/ralph-loop` runs it at startup (refuse to execute a failing spec).
It is NOT a CI content gate (CI only builds + unit-tests the tool).

It checks, deterministically: required sections present; `## Status:` is a single canonical token
in the allowed set; `## Slug:` (when present) matches `^[A-Z][A-Z0-9]*$`; every REQ is covered by
≥1 TC (unless `(no-test: <reason>)`-annotated); every task has `files:`; `depends:` is an acyclic
DAG of existing tasks; each task is in exactly one batch; no batch shares a non-shared-additive
file; batch order respects `depends:`; TC types are in the registered set; REQ/TC IDs carry the
slug prefix (when a slug is declared) and are unique; every TC references a real REQ; **every
`tests:` entry names a declared TC and every TC is referenced by ≥1 task (the TC↔task round-trip,
slug-gated; multi-reference allowed)**. A task whose `files:` is the `(none — execution only)`
sentinel is exempt from the files check and excluded from the declared-files union and batch-overlap.
Production-`.go` tasks without `tests:`, and slug-bearing specs with no capability link, emit
WARNINGS.

The linter handles the **mechanically-decidable** subset; the semantic rigor (boundary TCs per
field, infra-failure per dependency, both-branch coverage, errors-outnumber-happy) stays with the
review agents. Specs without `## Slug:` are **grandfathered** — slug/prefix/capability checks are
skipped, so pre-SDDX specs remain valid.

Beyond the default `lint`, the tool offers: `manifest` (capability index), `files <spec>` (the
deduplicated declared-files union, consumed by `/ralph-loop`'s files-vs-diff audit — any changed
file with no owning task is a MUST FIX), `capabilities [dir]` (capability↔code drift, § Capability
Docs), and `bootstrap-capability <pkg>` (skeleton generator). The task-declaration matcher
recognizes named tasks (`TASK-SMOKE`, `TASK-MERGE-*`, `TASK-FINAL`), not only `TASK-N`.

## Discipline Checkpoints (non-negotiable)

Two checkpoints exist alongside the normal flow. Skipping either is a process violation.

### After creating a spec — mandatory self-review

The `/spec` skill MUST run Phase 2 before presenting: 4 review agents in parallel
(`spec-reviewer`, `test-reviewer`, `code-reviewer`, `security-reviewer`), trivial fixes applied inline,
judgment-calls surfaced as "Pontos de atenção". See
`.claude/skills/spec/SKILL.md` § Phase 2.

If the user requests changes after the present, the self-review **re-runs from
scratch** before re-presenting. This is intentional: it protects against regressions
in the corrections themselves and keeps the audit honest.

### After executing a spec — mandatory self-review + present-before-commit

The `/ralph-loop` skill MUST run Phase 3 before presenting: 3 review agents in
parallel (`code-reviewer`, `test-reviewer`, `security-reviewer`), trivial fixes
applied inline, judgment-calls surfaced as "Pontos de atenção". See
`.claude/skills/ralph-loop/SKILL.md` § Phase 3.

The skill **never auto-commits**. It presents results in Phase 4 and waits for
explicit user approval. If the user requests more changes, the self-review
**re-runs from scratch** before re-presenting.

**The user should never have to ask "did you validate?" — that question means a
checkpoint was skipped.**

## Task Execution

- Each task must be independently verifiable (`go build ./...` should pass after
  each task — RED phase is the explicit exception, where the test file references
  symbols not yet implemented but the production tree still compiles)
- Tasks are architecture-agnostic — no mandatory layer ordering
- Order tasks logically for the feature, respecting the project's chosen structure
- If a task is unclear, mark it `BLOCKED` with a reason and stop execution
- **Mandatory review before testing**: after implementing a task, re-read the task
  description and verify ALL specified files, patterns, and behaviors were
  implemented. Check: all files listed in `files:` metadata were created/modified,
  all patterns from the Design section are followed, all error mappings and
  wrapping are complete, no implementation gap vs the spec. Only then proceed to
  tests. This is NEVER skipped.

## Task Metadata

- Every task MUST have a `files:` sub-item listing files it creates or modifies
- Tasks with dependencies MUST have a `depends:` sub-item listing prerequisite TASK-N IDs
- `depends:` must form a DAG (no circular dependencies)
- Tasks that share files in their `files:` lists cannot be in the same parallel batch
- Tasks with testable code MUST have a `tests:` sub-item listing TC-IDs from the
  Test Plan (triggers TDD cycle in `/ralph-loop`)
- Every `tests:` TC-ID must be a TC declared in the Test Plan, and every Test-Plan TC must be
  referenced by ≥1 task's `tests:` (the TC↔task round-trip; multi-reference allowed) — the linter
  enforces this on slug-bearing specs
- An execution-only task (a dogfood/smoke task with no source files) declares
  `- files: (none — execution only)`; the linter treats it as zero files (exempt from the files
  check, excluded from the declared-files union and batch-overlap)

## Test Plan

Every spec MUST include a `## Test Plan` section between Requirements and Design.
The Test Plan contains tables grouped by layer:

TC IDs are slug-prefixed and globally unique: `<SLUG>-TC-<TYPE>-NN` (two-digit `NN`). Canonical
TYPE prefixes:

- **Domain Tests** (`<SLUG>-TC-D-NN`): pure domain logic, value objects, entity invariants
- **Use Case Tests** (`<SLUG>-TC-UC-NN`): application logic, dependency interactions, error mapping
- **E2E Tests** (`<SLUG>-TC-E2E-NN`): full HTTP round-trip via TestContainers
- **Smoke Tests** (`<SLUG>-TC-S-NN`): k6-based validation of deployed behavior

Registered harness/tooling TYPE prefixes (only for harness/tooling specs, e.g. `learning-loop`,
`tools/learn`): `<SLUG>-TC-SH-NN` (shell/hook tests), `<SLUG>-TC-CT-NN` (contract tests). Any other
TYPE is a lint error — no ad-hoc prefixes. REQ IDs are `<SLUG>-REQ-N`.

Each TC row has: `| TC | REQ | Category | Description | Expected |`

Categories: `happy`, `validation`, `business`, `edge`, `infra`, `concurrency`,
`idempotency`, `security`

For non-code specs (config/docs only), the Test Plan may be `N/A` with a
justification.

### Coverage Rules

Every spec MUST satisfy all of the following:

- Every REQ has >= 1 TC (at minimum the happy path), OR a `(no-test: <reason>)` annotation on its
  declaration line (non-empty reason) for documentation-only REQs — `tools/validate-spec` enforces
  this exemption
- Every sentinel error in domain `errors.go` has >= 1 TC that triggers it
- Every validated field has boundary TCs: valid min, valid max, invalid min-1,
  invalid max+1
- Every external dependency call (repo, cache, publisher) has >= 1 infra-failure TC
- Every conditional branch in use case flow has TCs for both paths
- Concurrency scenarios required for operations with advisory lock or optimistic locking
- Every new HTTP endpoint has smoke TCs: happy path (201/200 + all response fields),
  each distinct error status (400/409/422), response format, auth, field
  boundaries, idempotency
- **Rigor check**: error/edge TCs should outnumber happy-path TCs — review the
  complete Test Plan and verify no business rule untested, no error path missing,
  no boundary unchecked

### Mutability

- TCs may be **added** during IN_PROGRESS (new edge cases discovered during
  implementation — quality-first lens means this happens often, not rarely)
- TCs may NEVER be **removed** during IN_PROGRESS — if a TC is no longer
  applicable, mark it as `SKIPPED` with a reason. Removal only allowed during
  DRAFT, and re-running the self-review afterwards is mandatory.
- REQ references in TCs must remain valid
- **Never modify Requirements and Test Plan in the same change.** REQ changes
  invalidate TC mappings; doing both at once erases the audit trail. Update REQ
  first, re-run self-review, then update TCs in a separate pass.
- **Freeze trade-off + controlled amendment (Böckeler).** Freezing Requirements + Test Plan during
  execution protects the audit trail, but heavy up-front design can lock the *wrong* REQ before a
  genuinely-unclear feature is understood. So a REQ MAY be amended while status is `IN_PROGRESS`
  **only if** (a) the full `/spec` self-review (all four lenses: spec / test / code / security)
  re-runs from scratch, and (b) an Execution Log entry records the amendment + its rationale. This
  makes amendment *auditable*, not forbidden — it is not a license to loosen rigor.

### Smoke Tests (k6)

- TC-S-* are validated by running `k6 run --env SCENARIO=smoke tests/load/main.js`
- Smoke tests are executed by `TASK-SMOKE` — a dedicated task at the end of the spec
- Smoke tests do NOT follow the TDD RED/GREEN cycle (they are executed directly)
- If the app is not running, log `SMOKE: DEFERRED` in the Execution Log
- Smoke file convention: `tests/load/users.js`, `tests/load/roles.js`,
  `tests/load/main.js`, `tests/load/helpers.js`

## TDD Execution

When a task has `tests:` metadata, `/ralph-loop` (or the parallel agent assigned
the task) follows the TDD cycle:

### RED Phase

1. Write the test file FIRST (before the production code)
2. Tests reference the function/type that will be implemented
3. Run `go test` — tests MUST fail (compilation failure counts as valid RED)
4. If tests pass before implementation: the test is not testing the right thing — fix it

### GREEN Phase

1. Write the MINIMUM production code to make tests pass
2. Follow existing patterns: hand-written mocks in `mocks_test.go`, table-driven tests
3. Run `go test` — all tests listed in `tests:` MUST pass
4. If other tests break: fix immediately before proceeding

### REFACTOR Phase

1. Clean up production code: remove duplication, improve naming, extract helpers
2. Run `go test` again — all tests MUST still pass
3. Run `go build ./...` — must compile cleanly

### Execution Log Format

When a task follows TDD, the Execution Log entry includes:

```text
TDD: RED(N failing) -> GREEN(N passing) -> REFACTOR(clean)
```

### Exceptions

- **Smoke tests** (TC-S-*): executed directly via k6, not via TDD cycle
- **Non-code tasks** (docs, config): no TDD — normal execution
- **Tasks without `tests:` metadata**: normal execution (no TDD cycle required)

## Parallel Batches

- The Parallel Batches section is auto-generated by `/spec` based on dependency and
  file analysis
- Batches are sequential: Batch N+1 starts only after all tasks in Batch N complete
- Tasks within a batch are independent: no shared files, no inter-dependencies
- Shared files are classified as:
  - **exclusive** — only one task touches it (safe for parallel)
  - **shared-additive** — multiple tasks add to it, e.g. DI wiring, route
    registration (accumulator pattern candidate)
  - **shared-mutative** — multiple tasks modify existing code (must serialize)

### Auto-rollback semantics (parallel batches)

When a batch with 2+ tasks runs in parallel via worktrees and **any agent fails**,
`/ralph-loop` MUST NOT silently merge the successful worktrees. The contract:

1. Stop after the failing batch.
2. Surface to the user: which tasks succeeded, which failed, the failure cause
   (one line per failure).
3. Offer three options: (a) merge successful + skip failed, (b) discard everything
   and rerun, (c) stop for manual investigation.
4. Default (no answer) is (c). Never merge a partially-failed batch silently.
5. **The user's choice is recorded in the Execution Log** so the spec history
   shows what happened — never silently revise it after.

This contract is non-negotiable — it preserves the user's ability to reason about
the working tree state. **Even if the failure is in an "independent" task,
dependencies between tasks may not be fully visible from `depends:` alone**
(shared imports, shared test fixtures, shared package-level state). The
quality-first lens says: when in doubt, stop and let the user verify.

## Merge Strategy (accumulator pattern)

When parallel tasks share **additive** files (e.g. `cmd/api/server.go` for DI
wiring, `cmd/api/router.go` for route registration), use the accumulator pattern:

- Each parallel task generates a wiring fragment in
  `.specs/wiring/<spec-slug>/<task-id>.<target-slug>.fragment.md` instead of
  editing the shared file directly
- A dedicated merge task (`TASK-MERGE-<TARGET>`) reads all fragments and applies
  them sequentially in the next batch
- Fragments describe **intent** (what to add), not patches
- Shared-mutative files always serialize (different batches) — never run in parallel

### Fragment format

A fragment is a markdown file with these sections (all required unless noted):

```markdown
# Fragment: TASK-<N> → <target-file>

## Intent

<one-sentence description of what this fragment adds>

## Target

<full path of the shared file, e.g. cmd/api/server.go>

## Imports

<optional — Go import lines to add to the target file's import block,
deduplicated when merged>

```go
"github.com/marcelojr/gopherplate/internal/usecases/audit"
"github.com/marcelojr/gopherplate/internal/infrastructure/db/postgres/repository/audit"
```

## Additions

### Section: <named anchor>

<code block to insert at this anchor>

```go
auditRepo := audit.NewRepository(db.Writer())
auditUC := audit.NewLogUseCase(auditRepo)
```

### Section: <another named anchor>

```go
deps.AuditUseCase = auditUC
```

## Notes

<optional — ordering hints, known interactions, things the merge task
should be aware of>
```

### Registered anchors (canonical for this project)

The merge task locates the named anchor in the target file and inserts the
fragment's code block. Anchors are project-specific because they map to known
locations in known files:

| Target file | Anchor | Insert position |
| ----------- | ------ | --------------- |
| `cmd/api/server.go` | `buildDependencies` | inside `buildDependencies(...)`, before `return Dependencies{...}` |
| `cmd/api/server.go` | `Dependencies struct` | inside the `Dependencies` struct definition, alphabetical by field name |
| `cmd/api/router.go` | `route registration` | inside the route group setup, after existing route declarations |
| `cmd/api/grpc.go` | `service registration` | inside the gRPC server registration block (when applicable) |

When a new shared-additive target is needed, add an anchor row to this table in
the same PR — never let fragments use unregistered anchors silently.

### Merge conflict semantics

If two fragments target the same anchor with **incompatible content** (different
code performing the same wiring slot, e.g. two competing definitions of the same
variable), the merge task STOPS, leaves the merge unchecked, and surfaces the
conflict to the user. The fix is to clarify intent in the spec — usually this
means one of the parallel tasks needed an explicit `depends:` on the other.

## Re-review on user feedback

Both `/spec` (Phase 2) and `/ralph-loop` (Phase 3) re-run their full self-review
when the user requests more changes after the present. This is intentional and
**non-negotiable**:

- A correction is itself code (or spec text) that can introduce regressions.
- Skipping the audit on round 2+ silently erodes the safety net.
- The runtime cost is small (seconds per pass) compared to the cost of merging a
  flawed correction or approving a spec with a regressed REQ.
- Quality-first lens: **multiple review rounds are a feature, not a bug.** Round
  3, Round 4 are fine. Stop only when the user approves or rejects.

## Naming

- Spec files: lowercase, hyphen-separated: `user-audit-log.md`, `role-permissions.md`
- Wiring fragments:
  `.specs/wiring/<spec-slug>/<task-id>.<target-slug>.fragment.md`
  (e.g. `.specs/wiring/user-audit-log/task-3.cmd-api-server-go.fragment.md`)

### Slug & ID naming (concept #3, introduced by spec `SDDX`)

- Every new spec declares exactly one `## Slug:` — a short UPPERCASE id matching
  `^[A-Z][A-Z0-9]*$` (e.g. `AUDIT`, `RBAC`, `SDDX`), decoupled from the (possibly long) filename
  so IDs stay terse and globally unique.
- REQ IDs are `<SLUG>-REQ-N`; TC IDs are `<SLUG>-TC-<TYPE>-NN` with a two-digit `NN`.
- Registered TC `<TYPE>` set (anything else is a lint error): canonical `D` / `UC` / `E2E` / `S`;
  harness/tooling `SH` (shell/hook) / `CT` (contract). To add a new TYPE, register it here AND in
  `tools/validate-spec` in the same PR — never invent one ad hoc.
- Documentation-only REQs carry a `(no-test: <reason>)` annotation on the declaration line
  (non-empty reason) so `reqCovered` skips them.
- Specs without a `## Slug:` are **grandfathered** (the linter skips slug/prefix/capability
  checks). Do NOT retrofit the pre-`SDDX` specs.
