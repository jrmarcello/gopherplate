# Spec: SDD Capability Docs, Deterministic Spec Validation & Slug-Prefixed IDs

## Status: DONE

## Slug: SDDX

## Context

This spec evolves the gopherplate DX/harness by adopting **three concepts from
[OpenSpec](https://openspec.dev/)** as thin local fixes — **without** adopting the OpenSpec
tool itself. The same analysis ran on the sister projects `banking-api-ledger` and
`go-boilerplate`; the conclusion (here and there) is the same: our homegrown SDD is stricter than
OpenSpec on every axis that matters (blocking approval gate, mandatory Test Plan, TDD RED/GREEN,
adversarial multi-agent review, real parallel execution via worktrees). But three of OpenSpec's
concepts are genuinely missing and worth borrowing as local conventions + tooling.

This is a **harness-evolution** change and follows the project's own process
([docs/guides/harness-self-steering.md](../docs/guides/harness-self-steering.md)): a multi-file
infra change goes through `/spec` → `/ralph-loop`.

### Reference implementation

The sister project `go-boilerplate` is implementing the same three concepts
(`go-boilerplate/.specs/sdd-capabilities-and-validation.md`). This spec reuses its structure
(a linter Test Plan with a golden vacuous-pass guard, a fence-aware parser, the `manifest`
subcommand, the `(no-test: <reason>)` REQ-exemption convention) but adapts every detail to
gopherplate's **different reality** (see *Divergences* below). It is NOT a copy.

### Facts about THIS repository that shape the adaptation

1. **`.specs/*.md` ARE version-controlled here** (unlike go-boilerplate, where only `TEMPLATE.md`
   is tracked). There are 11 committed specs + `TEMPLATE.md`. Consequence: a CI gate against spec
   *content* would be *possible* here — but by explicit decision (see SDDX-REQ-8) we keep CI to
   building+testing the tool only; spec gates live in `/spec`, `/ralph-loop`, and `make`.
2. **`tools/learn/` already exists as a separate Go module** (`go.mod` of its own) — it is separate
   because it carries `modernc.org/sqlite` (FTS5), a heavy dependency the service binary must not
   inherit. `tools/validate-spec` carries **no** such dependency (stdlib + testify only), so the
   isolation rationale does NOT apply: it lives in the **main module** (see SDDX-REQ-8/Design),
   where it is covered for free by the existing `golangci-lint run ./...` and `go test`/`go build`
   gates.
3. **Phase proliferation is a live pain here, not hypothetical.** Two clusters exist: the
   `harness-map` family that grew from "5 specs" to 6+ by accretion
   (`harness-map.md` → `k6-regression-gate`, `maintainability-harness`, `behavior-harness`,
   `cli-harness-flavors`, `otel-strategy-alignment`, `learning-loop-harness`), and the `tools/learn`
   3-spec chain (`refactor-tools-learn-flat-layout` → `fix-refactor-tools-learn-contract-test-rigor`
   → `fix-cobra-error-exit-codes`). Plus a **dangling** `.specs/flavors-event-data.md` cited 12×
   that was never created. So the amend-in-place convention (concept #1) solves a real, observed
   problem.
4. **Status headers are already ~uniform**: every spec uses `## Status: <STATE>` on line 3. The only
   off-format is `cli-harness-flavors.md:3` (an inline parenthetical). The hypothesized
   `**Status (histórico):**` variant does NOT exist. So "standardize the header" (concept #6) is a
   documented grammar + linter check, not a migration.
5. **Module path is `github.com/jrmarcello/gopherplate`** (root `go.mod`). The `marcelojr` path in
   some `CLAUDE.md` fragment examples is stale; all new code uses `jrmarcello`. There is no
   `go.work` file (so `go list ./tools/...` reaches only the main-module `tools/validate-spec`; the
   nested `tools/learn` module is excluded automatically).

### The three concepts (as adapted here)

- **#1 — Living capability docs (HIGH).** A versioned `docs/capabilities/` directory holding the
  *current-truth guarantees* of each subsystem, plus `SUPERSEDED`/`ARCHIVED` spec states and an
  amend-in-place convention (evolve a capability by editing its canonical doc + appending its
  Changelog, not by spawning `fase-N+1` sibling specs).
- **#2 — Deterministic spec validation (MEDIUM).** A Go linter `tools/validate-spec` that checks
  the structural rules already written in prose in [.claude/rules/sdd.md](../.claude/rules/sdd.md)
  (required sections, REQ↔TC coverage, acyclic `depends:` DAG, batch file-overlap, standardized
  Status header, ID format/uniqueness). Wired as a deterministic gate that runs *before* the
  inferential `spec-reviewer` agent.
- **#3 — Global requirement identity (LOW).** Slug-prefixed REQ/TC IDs for cross-spec identity,
  linter-enforced, plus a generated capability manifest.

### Dogfood

This spec **dogfoods concept #3**: it declares `## Slug: SDDX` and uses slug-prefixed REQ/TC IDs
(`SDDX-REQ-1`, `SDDX-TC-UC-01`). These conventions do not exist in `.specs/TEMPLATE.md` /
`.claude/rules/sdd.md` yet — they are introduced *by* this spec, so the spec is its own first
conforming example, and `tools/validate-spec` must exit 0 against it once built (Validation
Criteria). For that to hold, the documentation-only REQs carry a `(no-test: <reason>)` annotation
(see SDDX-REQ-5) so the linter's `reqCovered` validator does not flag them as uncovered.

### Out of scope

- Adopting the OpenSpec tool, `openspec init`, or the delta/archive change format.
- **Retrofitting the 11 existing committed specs** to slug-prefixed IDs. They are grandfathered
  (see SDDX-REQ-9): the linter only enforces slug/prefix/capability rules on specs that declare
  `## Slug:`. Legacy specs without it are skipped by `make validate-spec`'s glob, and even when one
  is passed explicitly the slug-specific validators are no-ops. This avoids churning the audit trail
  of shipped DONE specs and avoids fighting their `N/A` Test Plans and drifted TC prefixes
  (`TC-H/HI/BM/ET`).
- **Normalizing `cli-harness-flavors.md:3`'s inline-parenthetical status.** It is a grandfathered
  DONE spec (no `## Slug:`), so `make validate-spec` skips it and the off-format line is cosmetic
  history — left untouched to avoid editing shipped work. (Note: an *explicit* `FILE=` invocation
  against it WOULD report the off-format status, since structural validators always run — documented
  in `docs/guides/spec-linter.md`.)
- **A CI gate against spec content** (decision in SDDX-REQ-8). The tool itself is built and
  unit-tested in CI via the existing Go gates.
- A capability doc for **every** subsystem. This spec ships four worked examples
  (`user`, `role`, `idempotency`, `caching`); the rest are documented over time via amend-in-place
  as specs touch them.
- Wiring the linter into `stop-validate.sh` / `ci-local.sh` hooks — a possible follow-up, kept out
  to match the reference scope.
- Embedding the validator inside the `gopherplate` CLI (`cmd/cli`). It is a standalone `package
  main` dev tool; a future `gopherplate doctor` integration would require exporting its types — a
  known, accepted limitation, not addressed here.

## Requirements

<!-- GIVEN/WHEN/THEN acceptance criteria. Implementation lives in Design.
     Documentation-only REQs carry a `(no-test: <reason>)` annotation on their declaration line so
     the linter's reqCovered validator skips them (see SDDX-REQ-5). -->

- [ ] SDDX-REQ-1: **Capability docs convention exists.** (no-test: documentation/convention — verified by review + grep of docs/capabilities/README.md and TEMPLATE.md)
  GIVEN a contributor opening `docs/capabilities/`,
  WHEN they read `README.md` and `TEMPLATE.md`,
  THEN they find: what a capability doc is (current-truth guarantees of a subsystem — *not* a
  how-to guide, *not* an ADR decision, *not* a per-change spec), its lifecycle
  (`Active` → `Superseded` → `Archived`), its relationship to the tracked-but-ephemeral specs and
  to ADRs, when to amend-in-place vs. write a new doc, and a fill-in `TEMPLATE.md` carrying a
  `Slug`, `Status`, `Source` (specs/ADRs), a Guarantees/Invariants section, a slug-prefixed
  guaranteed-REQ list, and a `Changelog` using `ADDED`/`MODIFIED`/`REMOVED` prose headers.

- [ ] SDDX-REQ-2: **Four worked capability docs reflect real code.** (no-test: documentation — verified by review that each doc matches the cited source packages + grep for Status/Slug)
  GIVEN the example domains and cross-cutting subsystems already in the repo,
  WHEN a reader opens `docs/capabilities/{user,role,idempotency,caching}.md`,
  THEN each doc accurately describes the *current* guarantees of that subsystem as implemented
  (`internal/usecases/user/`, `internal/domain/user/`, `internal/usecases/role/`,
  `pkg/idempotency/`, `internal/infrastructure/web/middleware/idempotency.go`, `pkg/cache/` incl.
  singleflight), follows `TEMPLATE.md`, has `Status: Active`, and uses slug-prefixed
  guaranteed-REQ IDs.

- [ ] SDDX-REQ-3: **SDD state machine extended + amend-in-place documented.** (no-test: convention — verified by grep of .claude/rules/sdd.md for the new states/section + review)
  GIVEN `.claude/rules/sdd.md`,
  WHEN a reader consults the status transitions and the new capability-docs section,
  THEN the lifecycle includes `SUPERSEDED` and `ARCHIVED` (in addition to
  `DRAFT → APPROVED → IN_PROGRESS → DONE | FAILED`); an explicit amend-in-place convention states
  that evolving a capability means editing its canonical doc + appending to its Changelog rather
  than spawning `fase-N+1` sibling specs, with an explicit decision rule (**amend** the capability
  doc when an existing subsystem's behavior changes — no new module, no new endpoint; **write a new
  spec** when a new subsystem is introduced or the change is large enough to deserve its own
  approval gate + TDD cycle); and the ad-hoc `.specs/archive/<name>.md` rejection move in
  `spec/SKILL.md` is reconciled with the formal `ARCHIVED` status.

- [ ] SDDX-REQ-4: **The SDD flow wires capability docs in.**
  GIVEN a spec that changes the behavior of a subsystem,
  WHEN `/spec` authors it and `/ralph-loop` executes it,
  THEN `/spec` identifies and links the impacted capability doc, `/ralph-loop`'s wrap-up updates
  the relevant capability doc as a first-class audited artifact (current-truth + Changelog) and
  runs `make capabilities-manifest`, and the review agents (`spec-reviewer`, `code-reviewer`) check
  that the capability doc was updated when behavior changed. (The linter half — a WARN when a spec
  references no capability doc — is covered by SDDX-TC-UC-14; the flow/agent half is doc-level,
  verified by grep/review of the skill and agent files.)

- [ ] SDDX-REQ-5: **Deterministic spec linter validates structure.**
  GIVEN a spec file path,
  WHEN `tools/validate-spec <file>` runs,
  THEN it deterministically reports, in `file:line: severity: message` form (findings sorted by
  `(line, message)` for stable output), any violation of: required sections present; every REQ
  referenced by ≥1 TC **unless** the REQ declaration line carries a `(no-test: <non-empty-reason>)`
  annotation (an empty reason `(no-test:)` is itself an ERROR); every task has `files:`; `depends:`
  forms an acyclic DAG (self-loops included) referencing existing tasks; every task appears in
  exactly one Parallel Batch; no two tasks in a batch share a file unless that file is declared
  shared-additive; batch order respects `depends:`; every TC references a valid REQ-ID; TC/REQ IDs
  are unique; TC type prefixes belong to the registered set. It exits 1 when any `ERROR` is present,
  2 on a tool/usage/IO error, and 0 when only `WARN`s or clean. Production-`.go` tasks without
  `tests:` emit a `WARN` (not `ERROR`); a missing impacted-capability link emits a `WARN`.

- [ ] SDDX-REQ-6: **Status header is standardized and parseable.**
  GIVEN any spec passed to the linter,
  WHEN the linter parses it,
  THEN `statusHeader` (a structural validator that always runs, independent of slug) requires
  exactly one `## Status: <STATE>` heading line where `<STATE>` is the first whitespace-delimited
  token and must be in `{DRAFT, APPROVED, IN_PROGRESS, DONE, FAILED, SUPERSEDED, ARCHIVED}`; a
  trailing inline qualifier after the token (e.g. `## Status: DONE (MVP; ...)`), an unknown state,
  or more than one status line is an `ERROR`. `TEMPLATE.md` models the canonical form.

- [ ] SDDX-REQ-7: **Capability manifest is generated, not hand-maintained.**
  GIVEN the capability docs,
  WHEN `tools/validate-spec manifest --write` runs,
  THEN it scans `docs/capabilities/*.md` and writes `docs/capabilities/MANIFEST.md` listing, per
  capability: name, slug, status, guaranteed REQ-IDs, and linked specs/ADRs; the rendering is pure
  (sorted by slug) so running it twice with no doc changes is idempotent (stable output, no diff);
  a malformed capability doc (missing `## Slug:` or `## Status:`) does not crash the run — its
  entry renders with a `<missing>` placeholder and a warning to stderr.

- [ ] SDDX-REQ-8: **Linter lives in the main module, wired into make + the SDD flow as a gate (not CI spec-content checking).** (no-test: make/CI/skill wiring is not hermetically unit-testable — exercised by the TASK-12 dogfood + Validation Criteria grep/exit-code checks)
  GIVEN the linter exists under `tools/validate-spec/` in the **root module**,
  WHEN a contributor runs `make validate-spec` (every `.specs/*.md` that declares `## Slug:`, except
  `TEMPLATE.md` by path; or `FILE=<path>`) or `make capabilities-manifest`, or runs `/spec` /
  `/ralph-loop`,
  THEN `make` targets invoke the tool via `go run ./tools/validate-spec`; `/spec` runs it as a
  deterministic gate *before* the 3-agent self-review; `/ralph-loop` runs it at startup as a gate
  (refusing to execute a spec that fails); the tool is covered by `golangci-lint run ./...`, by the
  CI Unit-Tests step (which adds `./tools/...` to its package list so the tool's tests run and its
  coverage feeds `coverage-delta`), by `make test-unit`/`make test-coverage` (both gain
  `./tools/...`), and a `gosec` exemption for `tools/validate-spec/` (authored alongside the code in
  the same task to avoid an intermediate lint-failing state) mirrors the existing `cmd/cli/` one;
  and **no** GitHub Actions step lints `.specs/*.md` content.

- [ ] SDDX-REQ-9: **Slug-prefixed ID convention is adopted and enforced for new specs only.**
  GIVEN `TEMPLATE.md`, `.claude/rules/sdd.md`, and `.claude/skills/spec/SKILL.md`,
  WHEN a new spec is authored,
  THEN it declares a short uppercase `## Slug:` (matching `^[A-Z][A-Z0-9]*$`, exactly one) and uses
  `<SLUG>-REQ-N` and `<SLUG>-TC-{D,UC,E2E,S,SH,CT}-NN` IDs; capability docs use slug-prefixed
  guaranteed-REQ IDs; the linter errors on IDs missing the declared prefix, with a mismatched
  prefix, or with an unregistered TC type; the canonical-vs-registered TC-prefix set is documented
  (the canonical `D/UC/E2E/S` plus the registered harness prefixes `SH` shell/hook and `CT`
  contract); the `| TC |` vs `| TC-ID |` table-header drift between `TEMPLATE.md` and `sdd.md:132`
  is reconciled to `| TC |` in both files; and specs that do NOT declare `## Slug:` are
  grandfathered (the linter's slug/prefix/capability validators are no-ops on them and `make
  validate-spec` skips them).

- [ ] SDDX-REQ-10: **Harness inventory and contributor-facing docs stay in sync.** (no-test: documentation sync — verified by grep + cross-reference resolution review)
  GIVEN the new artifacts,
  WHEN a reader consults `docs/harness.md`, `CLAUDE.md`, `README.md`, `CONTRIBUTING.md`, and
  `docs/guides/sdd-ralph-loop.md`,
  THEN capability docs (guide) and `validate-spec` (computational sensor + its own
  `docs/guides/spec-linter.md` deep-dive) appear in the harness inventory, the relevant
  "Known gaps" rows are resolved, and the SDD-flow descriptions mention the deterministic linter
  gate, capability-doc updates, and the slug-prefix convention. All cross-references resolve.

## Test Plan

<!--
  This change adds a Go dev-tool (tools/validate-spec) plus documentation/convention files.
  The ONLY testable production code is the linter. There are intentionally NO Domain, E2E, or
  Smoke tests: no domain entity, no HTTP endpoint, and no running-system surface is introduced.

  Documentation/convention REQs (SDDX-REQ-1, -2, -3, -8, -10) carry a `(no-test: <reason>)`
  annotation on their declaration line — the linter's reqCovered validator skips them, and they are
  verified by Validation Criteria + grep/review (the project already allows this under
  .claude/rules/sdd.md § "non-code specs ... Test Plan may be N/A with justification"; the
  annotation makes that machine-checkable). SDDX-REQ-4 is split: its linter half (capability WARN)
  is covered by SDDX-TC-UC-14; its flow/agent half is doc-level (grep/review of skill+agent files).

  Passing-path model (vacuous-pass guard): testdata/valid/golden.md (SDDX-TC-UC-18) is the SINGLE
  canonical all-pass fixture exercising the success path of EVERY validator (including an empty
  ## Execution Log section, a shared-additive-annotated batch file, and a capability link). Each
  failing fixture asserts only its own error/warn path. The two WARN validators (goProdTaskHasTests,
  capabilityLinked) additionally have a named passing path (golden). This guarantees every validator
  is exercised in both directions.
-->

### Use Case Tests (linter — `tools/validate-spec`)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| SDDX-TC-UC-01 | SDDX-REQ-5 | validation | Spec missing a required section (e.g. no `## Test Plan`) | 1 ERROR naming the missing section |
| SDDX-TC-UC-02 | SDDX-REQ-6 | validation | Status off-format, two distinct fixtures: inline parenthetical (`status-offformat.md`: `## Status: DONE (MVP; ...)`) and unknown state (`status-unknown.md`: `## Status: WIP`) | exactly 1 ERROR per fixture (2 total) |
| SDDX-TC-UC-03 | SDDX-REQ-5 | validation | A REQ that no TC references and no `(no-test:)` annotation | ERROR naming the uncovered REQ |
| SDDX-TC-UC-04 | SDDX-REQ-5 | validation | A task with no `files:` sub-item | ERROR naming the task |
| SDDX-TC-UC-05 | SDDX-REQ-5 | edge | A task touching a production `.go` file (not `*_test.go`/server/container/router/migration) with no `tests:`, incl. a MIXED case (one excluded + one non-excluded file) | WARN fires (not ERROR) even in the mixed case; a task with `tests:` or only excluded files → no warn |
| SDDX-TC-UC-06 | SDDX-REQ-5 | validation | `depends:` forms a cycle (T1→T2→T1) and, separately, a self-loop (T1→T1) | two fixtures asserted INDEPENDENTLY: `depends-cycle.md` → 1 ERROR; `depends-selfloop.md` → 1 ERROR |
| SDDX-TC-UC-07 | SDDX-REQ-5 | validation | `depends:` references a non-existent TASK-ID | ERROR naming the dangling dependency |
| SDDX-TC-UC-08 | SDDX-REQ-5 | validation | Two tasks in the same batch share a file, NOT declared shared-additive | ERROR naming the file + batch |
| SDDX-TC-UC-09 | SDDX-REQ-5 | validation | A task absent from all batches, and a task listed in two batches | exactly 2 ERRORs — each ERROR's message must NAME its specific condition (absent-from-all-batches vs. in-two-batches), not merely count==2 |
| SDDX-TC-UC-10 | SDDX-REQ-5 | validation | Batch order violates `depends:` (dependent in an earlier/same batch as its prerequisite) | ERROR |
| SDDX-TC-UC-11 | SDDX-REQ-9 | validation | (slug declared) A TC-ID missing the slug prefix, one with a wrong prefix (`WRONG-TC-UC-01`), and one with a bad type suffix (`SDDX-TC-X-01`) | ERROR for each (3 total); each message NAMES its condition (missing prefix / wrong prefix / unrecognized TC type) |
| SDDX-TC-UC-12 | SDDX-REQ-9 | validation | (slug declared) A REQ-ID missing the slug prefix and one with a wrong prefix | ERROR for each |
| SDDX-TC-UC-13 | SDDX-REQ-5 | validation | A TC referencing a REQ-ID that does not exist | ERROR naming the dangling REQ ref |
| SDDX-TC-UC-14 | SDDX-REQ-4 | edge | A slug-declared spec with no impacted-capability reference (`no-capability-link.md`) | WARN; the golden fixture (references `docs/capabilities/*.md`) → no warn |
| SDDX-TC-UC-15 | SDDX-REQ-9 | validation | A duplicate TC-ID AND a duplicate REQ-ID in the same spec | ERROR for each (idUnique covers both ID kinds) |
| SDDX-TC-UC-16 | SDDX-REQ-5 | business | Exit codes, named fixtures: any ERROR fixture (`missing-section.md`) → exit 1; warn-only fixture (`goprod-no-tests.md`) → exit 0; golden fixture → exit 0 | exit codes as described |
| SDDX-TC-UC-17 | SDDX-REQ-7 | idempotency | `manifest` over capability-doc fixtures via the pure `renderManifest()` | output lists each capability (slug/status/guaranteed-REQs/links); empty fixture dir → valid empty manifest; `renderManifest()` called twice on identical input → identical string (idempotent, no file I/O in the test) |
| SDDX-TC-UC-18 | SDDX-REQ-5 | happy | Golden-valid SYNTHETIC fixture (`testdata/valid/golden.md`, not a copy of this spec) exercising the all-pass path of every validator; full sections (incl. empty Execution Log), `## Slug:`, slug-prefixed IDs, **≥2 REQs (one covered by a TC, one carrying `(no-test:)`** — so both `reqCovered` branches pass), a shared-additive batch file, a capability reference, AND a backtick-quoted bad-ID example in a Description cell that must be ignored (declaration-site parsing) | 0 ERRORs and 0 WARNs — the canonical passing path for ALL validators (vacuous-pass guard) |
| SDDX-TC-UC-19 | SDDX-REQ-9 | validation | `## Slug:` present with an invalid value (`## Slug: my-slug`) and present twice; valid `## Slug: SDDX` → no slug error | ERROR for the invalid value + ERROR for the duplicate; valid → no slug error |
| SDDX-TC-UC-20 | SDDX-REQ-5 | edge | A real `## Status: DRAFT` plus a `## Status: Active`, an `SDDX-REQ-9`, and a `## Test Plan` shown INSIDE fenced code blocks — a plain ```` ``` ````, a language-tagged ```` ```go ````, a ```` ```markdown ````, and a `~~~` fence, some indented up to 3 spaces | only the real lines are parsed; ALL fenced examples ignored (no duplicate-status ERROR, no phantom REQ/section) |
| SDDX-TC-UC-21 | SDDX-REQ-9 | edge | GRANDFATHERING: a legacy spec WITHOUT `## Slug:` using bare `REQ-1`/`TC-UC-01` → slug/prefix/capability validators are no-ops (no ERROR/WARN on those; structural validators still run); the SAME content WITH `## Slug: SDDX` but bare IDs → ERROR (prefix enforced) | grandfathered path: 0 slug/prefix findings, exit 0; slugged path: ERROR per bare ID |
| SDDX-TC-UC-22 | SDDX-REQ-9 | validation | TC-prefix registry: registered prefixes (`TC-D/UC/E2E/S/SH/CT`) accepted in a slug-declared spec; an unregistered/invented prefix (`SDDX-TC-ZZ-01`) → ERROR | registry enforced |
| SDDX-TC-UC-23 | SDDX-REQ-6 | validation | `statusHeader` count path: a spec with TWO `## Status:` lines (`status-duplicate.md`) | ERROR (more than one status line) — guards the `count == 1`, not `>= 1`, rule |
| SDDX-TC-UC-24 | SDDX-REQ-5 | happy | `batchFileOverlap` PASSING path: two tasks in one batch DO share a file, but it is declared shared-additive in the batch analysis (`batch-additive-ok.md`) | no ERROR — exercises the accept branch (mutation guard for the shared-additive exception) |
| SDDX-TC-UC-25 | SDDX-REQ-5 | validation | `reqCovered` annotation: a REQ with a valid `(no-test: some reason)` → no ERROR (skipped); a REQ with empty `(no-test:)` → ERROR | as described (both branches of the exemption) |
| SDDX-TC-UC-26 | SDDX-REQ-7 | edge | `manifest` over a malformed capability doc (missing `## Slug:`/`## Status:`) | entry renders with `<missing>` placeholder + warning to stderr; no crash, run continues |
| SDDX-TC-UC-27 | SDDX-REQ-5 | infra | `main` exit-code mapping: a non-existent / unreadable file path → exit 2 (tool/IO error); an unknown subcommand or bad flag → exit 2 (usage); a clean spec → exit 0; an ERROR-bearing spec → exit 1 | exit codes as described |
| SDDX-TC-UC-28 | SDDX-REQ-5 | edge | All required sections present but `## Execution Log` is empty (heading only) — fixture `empty-execlog.md` | 0 ERRORs from `requiredSections` — heading presence alone satisfies the check (mutation guard vs. a "non-empty content" tightening) |
| SDDX-TC-UC-29 | SDDX-REQ-6 | validation | `## Status: SUPERSEDED` and, separately, `## Status: ARCHIVED` (fixtures `status-superseded.md`, `status-archived.md`) | 0 status ERRORs for each — both are in the allowed-state whitelist (mutation guard for the extended state set) |
| SDDX-TC-UC-30 | SDDX-REQ-5 | edge | Task `files: [internal/domain/user/server.go]`, no `tests:` (basename `server.go` but path ≠ `**/cmd/api/server.go`) | 1 WARN — the path-based exclusion does NOT match; the old basename rule would have wrongly suppressed it |
| SDDX-TC-UC-31 | SDDX-REQ-5 | edge | Task `files: [cmd/api/server.go, cmd/api/container.go, cmd/api/router.go]`, no `tests:` | 0 WARNs — all three match the path-based exclusions |
| SDDX-TC-UC-32 | SDDX-REQ-5 | edge | Task `files:` includes a `.go` file under `**/migration/**`, no `tests:` | 0 WARNs — migration-path exclusion applies |
| SDDX-TC-UC-33 | SDDX-REQ-5 | edge | A REQ with BOTH a valid `(no-test: doc-only)` annotation AND a TC referencing it | 0 ERRORs — the annotation unconditionally suppresses `reqCovered`; the TC reference is not penalized |
| SDDX-TC-UC-34 | SDDX-REQ-9 | validation | (slug declared) A TC-ID with a single-digit numeric suffix (`SDDX-TC-UC-1`) | ERROR — violates the `\d{2,}` (2-digit `NN`) requirement (off-by-one boundary guard) |
| SDDX-TC-UC-35 | SDDX-REQ-9 | edge | A legacy spec (no `## Slug:`) with NO `docs/capabilities/*.md` reference anywhere | 0 WARNs from `capabilityLinked` — the validator is a no-op when slug is absent (distinct from TC-21's idFormat focus) |
| SDDX-TC-UC-36 | SDDX-REQ-7 | business | `manifest --write` invoked with a temp output root | file written at the expected path; content equals `renderManifest()` output; re-run produces no diff (covers the G306 write branch) |
| SDDX-TC-UC-37 | SDDX-REQ-6 | edge | A spec with NO `## Status:` line at all (all other required sections present) — fixture `status-none.md` | ≥1 ERROR from `statusHeader` (exactly-one ⇒ zero fails); confirms `statusHeader` fires independently of `requiredSections` |
| SDDX-TC-UC-38 | SDDX-REQ-5 | edge | A `~~~go` language-tagged tilde fence containing `## Status: WIP` | 0 ERRORs from `statusHeader` — language-tagged tilde fence suppressed (prefix-match guard) |
| SDDX-TC-UC-39 | SDDX-REQ-5 | edge | HTML comment (multi-line block) containing `## ` headings, a fake REQ, a fake TC row, and a `Batch` line; plus an inline trailing comment (`## Status: DONE <!-- note -->`) | commented content is NOT parsed (no phantom section/REQ/TC/batch; status line count unaffected); the inline trailing comment keeps the real status token — comment-awareness, parallel to the fence guard |

## Design

### Architecture Decisions

**`tools/validate-spec` lives in the MAIN module (concept #2 + enforcement for #3 + #6).** A single
cohesive `package main` Go program under `tools/validate-spec/` inside the root module
(`github.com/jrmarcello/gopherplate/tools/validate-spec`) — **no** nested `go.mod`. This differs
from `tools/learn/` (which is a separate module *only* to isolate `modernc.org/sqlite`); the linter
needs no heavy dependency, so the main-module placement gives it free coverage from the existing
`golangci-lint run ./...` (lint+vet) gate and a place in `go test ./...`. **Known limitation
(accepted):** `package main` means the parser/validator types are not importable, so a future
`gopherplate doctor` embedding would need a refactor to a library package — out of scope here.
Implemented as one task (not split across parallel agents) because it is one tightly-coupled
package — one author with full TDD yields higher quality than agents stomping shared package files.
Structure (mirrors go-boilerplate's proven factoring):

- `main.go` — thin CLI: subcommand dispatch (`validate-spec <files...>` default; `validate-spec
  manifest [--write]`), flag parsing, file reads, exit-code mapping. Keep logic-free so coverage
  stays high. File-read/glob errors wrapped with context (`fmt.Errorf("reading spec %q: %w", path,
  err)`) per `go-conventions.md § Error Handling`. **Exit-code mapping (linter convention, a
  deliberate divergence from `learnerr.go`'s usage=1/runtime=2 because findings are the primary
  output):** `0` = clean or WARN-only; `1` = at least one lint ERROR present; `2` = tool error (bad
  usage / unknown subcommand / unreadable file / glob error). Callers distinguish exit 1 (spec
  invalid) from exit 2 (tool failed) by the exit code, and see the `file:line: error:` lines on
  stdout. Documented for the `/spec` and `/ralph-loop` integrations in `docs/guides/spec-linter.md`.
- `parser.go` — pure functions that parse a spec's markdown into a typed model: `status`, `slug`,
  section presence, `[]requirement{id, line, noTestReason, noTestPresent}`,
  `[]testCase{id, reqRef, line}`, `[]task{id, files, tests, depends, line}`,
  `[]batch{index, taskIDs, sharedAdditive, line}`, capability links. Takes file contents as a
  string (no I/O) so tests are hermetic. **Declaration-site parsing (critical):** REQ IDs are
  extracted ONLY from Requirements list-item declaration lines (`- [ ] <ID>: …`); TC IDs ONLY from
  the FIRST column of Test Plan table rows; a TC's REQ reference ONLY from the SECOND column;
  `files:`/`tests:`/`depends:` ONLY from task sub-item lines. IDs that appear anywhere else — in
  prose, in backtick-quoted *examples* inside a Description/Expected cell (this spec deliberately
  cites `WRONG-TC-UC-01`, `SDDX-TC-X-01`, `SDDX-TC-ZZ-01` as negative examples), or in fenced
  blocks — are NOT treated as declarations, so they never trip `idFormat`/`idUnique`. This is what
  lets the spec dogfood itself. **Fence-aware:** the line scanner tracks an `inFence` flag
  toggled by a fence delimiter, detected as
  `strings.HasPrefix(strings.TrimLeft(line, " \t"), "```")` OR `… "~~~"` (prefix match on the
  left-trimmed line, per CommonMark §4.5 — this correctly handles language-tagged fences
  (```` ```go ````, ```` ```markdown ````, ```` ```text ````) and up to 3 spaces of indentation;
  an exact-equality check would miss them and falsely parse the structural keywords this spec shows
  inside such fences). While `inFence`, structural matching is suppressed — with one deliberate
exception: `Batch N:` lines are parsed regardless of fence state, because the Parallel Batches
section is itself a fenced ```` ```text ```` block. The parser
  returns `(model, error)` ONLY for unrecoverable failures (file not text-decodable); every
  structural issue is reported downstream as a `finding`, never as a Go `error`.
- `validate.go` — one function per validator (table below), each returning `[]finding`. A top-level
  `validate(model) []finding` composes them in table order and then **sorts the aggregate by
  `(line, message)` before returning** so output is deterministic (no map-iteration flakiness; tests
  that assert "exactly N ERRORs" compare an unordered set of messages, but the printed output is
  stably ordered). `type severity int` with `iota` constants `severityWarn`, `severityError` and a
  `String()` method returning the lowercase labels `"warning"` / `"error"` (matching the
  `go vet` / golangci-lint output convention at the bottom of this section). `finding` is unexported.
  **Grandfathering (canonical contract): the skip lives inside `validate()`** — when
  `model.slug == ""`, the slug-specific validators (`slugDeclared` format, `idFormat` prefix,
  `capabilityLinked`) are no-ops; structural validators (`requiredSections`, `statusHeader`,
  `reqCovered`, `taskHasFiles`, `dependsDAG`, `dependsExist`, `batchMembership`, `batchFileOverlap`,
  `batchOrder`, `idUnique`, `tcRefValid`, `goProdTaskHasTests`, and the `idFormat` *registered-type*
  check) still run. `make validate-spec` additionally pre-filters its glob to slug-bearing specs for
  discoverability; the in-`validate()` skip is the authoritative behavior for explicit invocations.
- `manifest.go` — scans `docs/capabilities/*.md`, parses each capability's header (`Slug`, `Status`,
  `Source`, guaranteed-REQ IDs), and exposes a **pure** `renderManifest(caps) string`
  (capabilities sorted by slug → deterministic, idempotent). A doc missing `Slug`/`Status` yields an
  entry with a `<missing>` placeholder + a stderr warning (no crash). The `--write` path writes the
  rendered string to `docs/capabilities/MANIFEST.md`; default prints to stdout. Keeping render pure
  lets the idempotency test (SDDX-TC-UC-17) compare two `renderManifest()` calls without file I/O.
- `*_test.go` — table-driven tests over `testdata/` fixtures, hermetic. stdlib `testing` + testify
  only. **No `mocks_test.go`** — all logic is pure functions over strings/structs, nothing to mock.
- `testdata/` — `valid/golden.md` is the SINGLE canonical all-pass fixture (SDDX-TC-UC-18): a
  minimal but COMPLETE synthetic spec (not a copy of this file) satisfying every validator,
  including an empty `## Execution Log`, a `(no-test:)`-annotated REQ, a shared-additive batch file,
  and a `docs/capabilities/*.md` reference (covering the `capabilityLinked` passing path). Plus one
  fixture per failing case: `missing-section.md`, `status-offformat.md`, `status-unknown.md`,
  `status-duplicate.md`, `status-in-fence.md`, `slug-bad.md`, `req-uncovered.md`,
  `reqcovered-notest.md`, `task-no-files.md`, `goprod-no-tests.md` (structurally valid except the
  WARN — doubles as the warn-only exit-code fixture for SDDX-TC-UC-16), `depends-cycle.md`,
  `depends-selfloop.md`, `depends-dangling.md`, `batch-file-overlap.md`, `batch-additive-ok.md`,
  `batch-membership.md`, `batch-order.md`, `tcid-bad-prefix.md`, `reqid-bad.md`,
  `tc-prefix-unregistered.md`, `tc-dangling-req.md`, `id-duplicate.md`, `no-capability-link.md`,
  `grandfathered-no-slug.md`, `fence-lang.md`, `empty-execlog.md`, `status-superseded.md`,
  `status-archived.md`, `status-none.md`, `goprod-domain-server.md`, `goprod-cmdapi-excluded.md`,
  `goprod-migration.md`, `reqcovered-notest-and-tc.md`, `tcid-single-digit.md`,
  `grandfathered-no-capability.md`, `fence-tilde-lang.md`; plus `capabilities/` fixtures (valid + a
  malformed one) for the manifest tests (`manifest --write` uses a `t.TempDir()` output root).

**Validator semantics (the deterministic encoding of `.claude/rules/sdd.md`):**

| Validator | Severity | Rule |
|-----------|----------|------|
| `requiredSections` | ERROR | All required (any order): `Status`, `Context`, `Requirements`, `Test Plan`, `Design`, `Tasks`, `Parallel Batches`, `Validation Criteria`, `Execution Log` (the gopherplate `TEMPLATE.md` set; `Slug` is NOT required → grandfathering). Presence = the `## <name>` heading exists; an empty/comment-only section satisfies the check |
| `statusHeader` | ERROR | Structural, ALWAYS runs (slug-independent). Exactly one `## Status: <STATE>` line; `<STATE>` = first token, in the allowed set (incl. `SUPERSEDED`/`ARCHIVED`); trailing inline qualifier, an unknown state, zero status lines, or more than one → ERROR (fires independently of `requiredSections`) |
| `slugDeclared` | ERROR (cond.) | If `## Slug:` present: exactly one, matches `^[A-Z][A-Z0-9]*$`. Absence is allowed (grandfathered) |
| `reqCovered` | ERROR | Every REQ-ID appears in ≥1 TC's REQ column, UNLESS the REQ line carries `(no-test: <non-empty-reason>)`; an empty reason `(no-test:)` → ERROR |
| `taskHasFiles` | ERROR | Every task has a `files:` sub-item |
| `goProdTaskHasTests` | WARN | A task whose `files:` include a production `.go` file should have `tests:`. Exclusions by basename `*_test.go`, and by PATH `**/cmd/api/server.go`, `**/cmd/api/container.go`, `**/cmd/api/router.go`, `**/migration/**` (path-based, not bare basename, to avoid excluding a same-named domain file). WARN fires if ANY non-excluded production `.go` lacks `tests:` (mixed excluded+non-excluded still warns) |
| `dependsDAG` | ERROR | `depends:` graph is acyclic (self-loop = cycle) |
| `dependsExist` | ERROR | Every `depends:` entry names an existing task |
| `batchMembership` | ERROR | Each task appears in exactly one batch |
| `batchFileOverlap` | ERROR | No two tasks in one batch share a `files:` entry UNLESS that file is annotated shared-additive in the Parallel Batches analysis |
| `batchOrder` | ERROR | For every `depends:` edge, the prerequisite is in a strictly earlier batch |
| `idFormat` | ERROR (cond.) | TC type ∈ registry `{D,UC,E2E,S,SH,CT}` (always checked). If `## Slug:` present, REQ/TC IDs must carry the `<SLUG>-` prefix (`<SLUG>-REQ-\d+`, `<SLUG>-TC-(D\|UC\|E2E\|S\|SH\|CT)-\d{2,}`); if absent, bare legacy IDs accepted (grandfathered) |
| `idUnique` | ERROR | No duplicate REQ-IDs or TC-IDs |
| `tcRefValid` | ERROR | Every TC's REQ column references an existing REQ-ID |
| `capabilityLinked` | WARN (cond.) | (slug present) Spec body (non-fenced) contains a substring matching `docs/capabilities/[^/\s]+\.md` |

Section *ordering* is intentionally NOT enforced (presence only). Output line format:
`<file>:<line>: error|warning: <message>` (lowercase severity, aligning with `go vet` /
golangci-lint so editors can parse it), aggregate sorted by `(line, message)`. A trailing summary
(`N error(s), M warning(s)`) goes to stderr. Exit `1` if any ERROR, `2` on tool error, else `0`.

**Standardized Status header (concept #6).** Canonical form is the `## Status: <STATE>` heading.
`TEMPLATE.md` and this spec use it. The amend-in-place Changelog (capability docs) and the spec
Execution Log carry *history*; the Status line carries only the *current* state.

**`docs/capabilities/` (concept #1).** A capability doc states *what a subsystem guarantees right
now*. It is distinct from: a **guide** (`docs/guides/*`, how-to), an **ADR** (`docs/adr/*`, why a
past decision was made — immutable), a **spec** (`.specs/*`, the plan for a single change — ages
into history). `docs/capabilities/TEMPLATE.md` shape:

```markdown
# Capability: <name>

## Slug: <UPPER>

## Status: Active            <!-- Active | Superseded | Archived -->

## Source

- Specs: <slug(s) or "n/a — pre-SDD">
- ADRs: <ADR links>

## Guarantees (current truth)

<Invariants the subsystem upholds today. Present tense. Verifiable.>

## Guaranteed Requirements

- <SLUG>-REQ-1: ...

## Changelog

### ADDED <date/slug>
### MODIFIED <date/slug>
### REMOVED <date/slug>

## Related

- [[other-capability]], guides, ADRs
```

Lifecycle: `Active` → `Superseded` (a newer capability replaces it; link forward) → `Archived`
(subsystem removed). **Amend-in-place:** to evolve a capability, edit its canonical doc and append a
`Changelog` entry — never create `caching-v2.md` / `caching-fase2.md`. Decision rule (also in
`sdd.md`): amend when an existing subsystem's behavior changes; write a new spec when a new
subsystem is introduced or the change warrants its own approval+TDD cycle.

**Slug-prefixed IDs (concept #3).** Each new spec/capability declares a short uppercase `## Slug:`;
all REQ/TC IDs carry it as a prefix, giving `SDDX-REQ-1` a different identity from `OTHER-REQ-1`.
The decoupling of a short `Slug` from the (possibly long) filename keeps IDs terse. **TC-prefix
registry:** the canonical layer prefixes `D/UC/E2E/S` are extended with the harness prefixes already
used in practice — `SH` (shell/hook tests, e.g. `learning-loop-harness`) and `CT` (contract tests,
e.g. the `tools/learn` chain) — so future drift (`TC-H/HI/BM/ET` etc.) becomes a lint error, not
silent invention. The `| TC |` header (TEMPLATE wins) replaces `| TC-ID |` at `sdd.md:132`.

**Manifest (concept #3 artifact).** `docs/capabilities/MANIFEST.md` is generated from the
capability docs — the committed index of capability → slug → status → guaranteed REQs → linked
specs/ADRs. `make capabilities-manifest` regenerates it; the committed file makes drift visible in
diffs.

**Flow wiring (concepts #1, #2, #4, #8).**

- `.claude/skills/spec/SKILL.md`: add a step after spec generation and **before** the 3-agent
  self-review — run `tools/validate-spec` (via `go run ./tools/validate-spec <new-spec>`); block on
  exit 1 (fix mechanically or surface). Add a "Gather Context" step to identify/link the impacted
  capability doc in Design. Generate `## Slug:` + slug-prefixed IDs.
- `.claude/skills/ralph-loop/SKILL.md`: in "Validate/Startup", add `tools/validate-spec` as a gate
  (refuse to execute a failing spec). In the wrap-up/post-execution audit, update the impacted
  `docs/capabilities/*.md` (current-truth + Changelog) as an audited artifact and run `make
  capabilities-manifest`. Make the status checks aware of `SUPERSEDED`/`ARCHIVED`, referencing the
  canonical state set in `.claude/rules/sdd.md § Flow` (do not hardcode).
- `.claude/agents/spec-reviewer.md` + `.claude/skills/spec-review/SKILL.md`: add checks — `## Slug:`
  present and IDs slug-prefixed; impacted capability doc linked; note that `validate-spec` is the
  deterministic pre-filter (the agent focuses on semantic gaps the linter cannot see); accept/set
  `SUPERSEDED`/`ARCHIVED`; make the REQ-extraction regex slug-aware.

**`Makefile`.** Add `validate-spec` (default: lint every `.specs/*.md` that declares `## Slug:`
except `TEMPLATE.md` by path; `FILE=<path>` lints one) and `capabilities-manifest` (`go run
./tools/validate-spec manifest --write`). Add `./tools/...` to BOTH `test-unit` (line 270) and
`test-coverage` (line 276) `go list`s so local unit-run and coverage match CI. Register both new
targets in `.PHONY` + `help`.

**`.golangci.yml` (authored in TASK-1 alongside the code).** Add a per-path `gosec` exclusion for
`tools/validate-spec/` (G304 file-path-from-arg on user-supplied spec paths + G306 file-write on the
manifest are by-design), mirroring the existing `cmd/cli/` exemption. It MUST land in the same task
as the file-I/O code so no intermediate state (and no Stop-hook lint run during execution) fails on
G304/G306.

**CI (`.github/workflows/ci.yml`).** The **Unit Tests** job's package list (`go list ./internal/...
./pkg/... ./config/...`) gains `./tools/...` so the new package's tests run AND its coverage lands
in `coverage.out` (feeding the `coverage-delta` diff-cover ≥70% gate). A comment notes that
`./tools/...` intentionally reaches only `tools/validate-spec` because `tools/learn` is a nested
module (and there is no `go.work`); if a `go.work` is ever added, this assumption must be revisited.
`go list ./tools/...` reaches only main-module packages. `deadcode` already filters to
`cmd/(api|migrate)|internal`, so `tools/` is out of its scope — no change there. `make build` builds
only shipped binaries (`cmd/...`); the linter is a dev tool run via `go run`, so it is not added to
`make build`. The CI **Lint** job's `golangci-lint run ./...` already reaches the main-module tool.

### Files to Create

- `tools/validate-spec/main.go`
- `tools/validate-spec/main_test.go`
- `tools/validate-spec/parser.go`
- `tools/validate-spec/parser_test.go`
- `tools/validate-spec/validate.go`
- `tools/validate-spec/validate_test.go`
- `tools/validate-spec/manifest.go`
- `tools/validate-spec/manifest_test.go`
- `tools/validate-spec/testdata/valid/golden.md` + one fixture per failing validator + `testdata/capabilities/*.md`
- `docs/capabilities/README.md`
- `docs/capabilities/TEMPLATE.md`
- `docs/capabilities/user.md`
- `docs/capabilities/role.md`
- `docs/capabilities/idempotency.md`
- `docs/capabilities/caching.md`
- `docs/capabilities/MANIFEST.md` (generated by TASK-12)
- `docs/guides/spec-linter.md`

### Files to Modify

- `.golangci.yml` — `gosec` exclusion for `tools/validate-spec/` (mirroring `cmd/cli/`). **Owned by
  TASK-1** (authored with the code).
- `.specs/TEMPLATE.md` — `## Slug:` line (a literal placeholder `<SLUG>`, NOT a real uppercase value,
  so the path-based `TEMPLATE.md` exclusion is the only guard needed), canonical `## Status:` form
  (extended state set noted), capability-reference field in Design, slug-prefixed example IDs,
  TC-ID convention block + registry, the `(no-test:)` annotation note, `| TC |` header.
- `.claude/rules/sdd.md` — `SUPERSEDED`/`ARCHIVED` states + amend-in-place (+ decision rule), new
  "Capability Docs" section, new "Deterministic Spec Linter (gate)" section, slug-prefix +
  TC-prefix-registry + `(no-test:)` annotation rules in Naming/Test Plan, `| TC-ID |` → `| TC |` at
  line 132, reconcile the `.specs/archive/` rejection move with `ARCHIVED`.
- `.claude/skills/spec/SKILL.md` — linter gate before self-review; identify/link capability;
  slug-prefixed IDs + `## Slug:`.
- `.claude/skills/ralph-loop/SKILL.md` — startup linter gate; capability-doc update + `make
  capabilities-manifest` in wrap-up; reviewers check capability docs; status checks aware of new
  states (reference sdd.md canonical set).
- `.claude/skills/spec-review/SKILL.md` — accept/set `SUPERSEDED`/`ARCHIVED`; slug-aware REQ
  extraction.
- `.claude/agents/spec-reviewer.md` — capability-link + slug-prefix checks; linter-as-pre-filter
  note.
- `Makefile` — `validate-spec` + `capabilities-manifest` targets; `./tools/...` in `test-unit` +
  `test-coverage`; `.PHONY` + `help` rows.
- `.github/workflows/ci.yml` — add `./tools/...` to the Unit-Tests `go list` package set (+ the
  nested-module comment).
- `docs/harness.md` — inventory rows (capability docs = guide; `validate-spec` = computational
  sensor; manifest); resolve relevant "Known gaps".
- `docs/guides/sdd-ralph-loop.md` — flow with linter gate + capability-doc update + slug prefix.
- `CLAUDE.md` — capability docs, `validate-spec`, slug-prefix in the right sections.
- `README.md` — SDD-flow description + docs map sync.
- `CONTRIBUTING.md` — SDD-workflow description sync (the file exists).

### Dependencies

None external beyond what is already vendored. Go stdlib (`os`, `bufio`/`strings`, `regexp`,
`path/filepath`, `sort`) + testify (already a dependency) for tests. Module path:
`github.com/jrmarcello/gopherplate` (the linter is `…/tools/validate-spec`).

## Tasks

<!-- Every file is exclusive to exactly one task — no shared-additive files, so no wiring fragments. -->

- [x] TASK-1: Implement `tools/validate-spec` (parser + all validators + `manifest` subcommand +
  thin `main`) in the MAIN module using full TDD, AND author the `.golangci.yml` gosec exclusion for
  `tools/validate-spec/` in the SAME task (so no intermediate lint-failing state). Write the
  table-driven tests and `testdata/` fixtures FIRST (RED), then implement until green. One cohesive
  `package main` — do not split. Fence-aware parser (prefix-match on left-trimmed line); finding
  output sorted by `(line, message)`; `type severity int` with `iota` + `String()` returning
  `error`/`warning`; grandfathering gated on `## Slug:` presence inside `validate()`; exit codes
  0/1/2; `reqCovered` honoring `(no-test: <reason>)`; manifest tolerant of malformed docs.
  - files: tools/validate-spec/main.go, tools/validate-spec/main_test.go, tools/validate-spec/parser.go, tools/validate-spec/parser_test.go, tools/validate-spec/validate.go, tools/validate-spec/validate_test.go, tools/validate-spec/manifest.go, tools/validate-spec/manifest_test.go, tools/validate-spec/testdata/, .golangci.yml
  - tests: SDDX-TC-UC-01, SDDX-TC-UC-02, SDDX-TC-UC-03, SDDX-TC-UC-04, SDDX-TC-UC-05, SDDX-TC-UC-06, SDDX-TC-UC-07, SDDX-TC-UC-08, SDDX-TC-UC-09, SDDX-TC-UC-10, SDDX-TC-UC-11, SDDX-TC-UC-12, SDDX-TC-UC-13, SDDX-TC-UC-14, SDDX-TC-UC-15, SDDX-TC-UC-16, SDDX-TC-UC-17, SDDX-TC-UC-18, SDDX-TC-UC-19, SDDX-TC-UC-20, SDDX-TC-UC-21, SDDX-TC-UC-22, SDDX-TC-UC-23, SDDX-TC-UC-24, SDDX-TC-UC-25, SDDX-TC-UC-26, SDDX-TC-UC-27, SDDX-TC-UC-28, SDDX-TC-UC-29, SDDX-TC-UC-30, SDDX-TC-UC-31, SDDX-TC-UC-32, SDDX-TC-UC-33, SDDX-TC-UC-34, SDDX-TC-UC-35, SDDX-TC-UC-36, SDDX-TC-UC-37, SDDX-TC-UC-38, SDDX-TC-UC-39

- [x] TASK-2: Create the capability-docs convention: `docs/capabilities/README.md` (what/why/
  lifecycle/amend-in-place + decision rule, and why it matters here given specs age into history)
  and `docs/capabilities/TEMPLATE.md` (the shape from Design).
  - files: docs/capabilities/README.md, docs/capabilities/TEMPLATE.md

- [x] TASK-3: Write domain capability docs `docs/capabilities/user.md` (CRUD, cache, singleflight,
  idempotency) and `docs/capabilities/role.md` (simpler multi-domain DI), reflecting real code
  (`internal/usecases/user/`, `internal/domain/user/`, `internal/usecases/role/`). Follow
  `TEMPLATE.md`; `Status: Active`; slug-prefixed guaranteed-REQ IDs.
  - files: docs/capabilities/user.md, docs/capabilities/role.md
  - depends: TASK-2

- [x] TASK-4: Write cross-cutting capability docs `docs/capabilities/idempotency.md` (SHA-256
  fingerprint, lock/unlock, replay, 5xx-not-cached — `pkg/idempotency/`,
  `internal/infrastructure/web/middleware/idempotency.go`) and `docs/capabilities/caching.md`
  (cache interface + Redis + singleflight anti-stampede — `pkg/cache/`), reflecting real code.
  Follow `TEMPLATE.md`.
  - files: docs/capabilities/idempotency.md, docs/capabilities/caching.md
  - depends: TASK-2

- [x] TASK-5: Update `.specs/TEMPLATE.md` — add `## Slug: <SLUG>` (literal placeholder, NOT a real
  value), canonical `## Status: <STATE>` form (extended state set noted), a capability-reference
  field in Design, slug-prefixed example IDs, the `(no-test: <reason>)` annotation note, the `| TC |`
  header, and document the TC-prefix registry in the convention comment.
  - files: .specs/TEMPLATE.md

- [x] TASK-6: Update `.claude/rules/sdd.md` — add `SUPERSEDED`/`ARCHIVED` to the state machine
  (line 67) + the amend-in-place convention + decision rule (Spec File Integrity, 62-67), a new
  "Capability Docs" section, a new "Deterministic Spec Linter (gate)" section, slug-prefix +
  TC-prefix-registry + `(no-test:)` annotation rules in Naming (348-353) + Test Plan/Coverage
  (127-156), fix `| TC-ID |` → `| TC |` at line 132, and reconcile the `.specs/archive/` rejection
  move with the formal `ARCHIVED` status. Canonical convention source.
  - files: .claude/rules/sdd.md

- [x] TASK-7: Update `.claude/skills/spec/SKILL.md` — run `tools/validate-spec` as a deterministic
  gate before the 3-agent self-review (block on exit 1); add a "Gather Context" step to
  identify/link the impacted capability doc; generate `## Slug:` + slug-prefixed IDs.
  - files: .claude/skills/spec/SKILL.md
  - depends: TASK-5, TASK-6

- [x] TASK-8: Wire the linter into make + CI as a main-module dev tool. Add `Makefile` targets
  `validate-spec` (globs `.specs/*.md` declaring `## Slug:`, except `TEMPLATE.md` by path, or
  `FILE=<path>`, via `go run ./tools/validate-spec`) and `capabilities-manifest` (`go run
  ./tools/validate-spec manifest --write`); add `./tools/...` to BOTH the `test-unit` and
  `test-coverage` `go list`s; register both new targets in `.PHONY` + `help`. Add `./tools/...` to
  the Unit-Tests `go list` in `.github/workflows/ci.yml` (with the nested-module comment) so the
  package's tests run and its coverage feeds `coverage-delta`. (The `.golangci.yml` gosec exclusion
  is authored in TASK-1, NOT here.)
  - files: Makefile, .github/workflows/ci.yml
  - depends: TASK-1

- [x] TASK-9: Update `.claude/skills/ralph-loop/SKILL.md` — add `tools/validate-spec` as a startup
  gate (refuse on exit 1); update the impacted `docs/capabilities/*.md` + run `make
  capabilities-manifest` in wrap-up/post-execution audit; reviewers check capability docs; make
  status checks aware of `SUPERSEDED`/`ARCHIVED`, referencing the canonical state set in
  `.claude/rules/sdd.md § Flow` (do not hardcode).
  - files: .claude/skills/ralph-loop/SKILL.md
  - depends: TASK-5, TASK-6

- [x] TASK-10: Update `.claude/agents/spec-reviewer.md` and `.claude/skills/spec-review/SKILL.md` —
  add checks for `## Slug:` + slug-prefixed IDs and impacted-capability linkage; note that
  `validate-spec` is the deterministic pre-filter; accept/set `SUPERSEDED`/`ARCHIVED`; make the
  REQ-extraction slug-aware.
  - files: .claude/agents/spec-reviewer.md, .claude/skills/spec-review/SKILL.md
  - depends: TASK-6

- [x] TASK-11: Sync contributor-facing docs — `docs/harness.md` (new inventory rows + resolve
  "Known gaps" for living-truth and deterministic spec validation), `CLAUDE.md`, `README.md`,
  `CONTRIBUTING.md`, `docs/guides/sdd-ralph-loop.md` (linter gate + capability-doc step + slug
  prefix), and a new `docs/guides/spec-linter.md` deep-dive (matching the per-sensor guide
  convention; documents the exit-code contract and the direct-invocation-vs-glob grandfathering
  behavior). **Reference the four capability docs by filename/path only — do not quote or copy their
  content; use `docs/capabilities/TEMPLATE.md` (TASK-2, Batch 1) for any example.** Ensure all
  cross-references resolve.
  - files: docs/harness.md, CLAUDE.md, README.md, CONTRIBUTING.md, docs/guides/sdd-ralph-loop.md, docs/guides/spec-linter.md
  - depends: TASK-1, TASK-2, TASK-5, TASK-6

- [x] TASK-12: Generate `docs/capabilities/MANIFEST.md` via `make capabilities-manifest`; verify it
  is non-empty, lists all four capabilities, and is idempotent (re-run yields no diff). Run `make
  validate-spec FILE=.specs/sdd-capabilities-and-validation.md` and confirm it exits 0, and `make
  validate-spec` (glob mode) exits 0 (dogfood).
  - files: docs/capabilities/MANIFEST.md
  - depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-8

## Parallel Batches

<!-- All files exclusive to one task → no shared-additive, no fragments. Batches by depends only. -->

```text
Batch 1: [TASK-1, TASK-2, TASK-5, TASK-6]                              — foundations (no deps)
Batch 2: [TASK-3, TASK-4, TASK-7, TASK-8, TASK-9, TASK-10, TASK-11]    — parallel (deps in Batch 1)
Batch 3: [TASK-12]                                                     — integration (manifest gen + dogfood)
```

File overlap analysis:

- Every file belongs to exactly one task — **no shared files**, hence no shared-additive/
  shared-mutative classes and no `.specs/wiring/` fragments for this spec.
- `tools/validate-spec/*` **and `.golangci.yml`** → TASK-1 (the gosec exemption is authored with the
  code to avoid an intermediate lint-failing state); capability docs split across TASK-2/3/4 by
  distinct filenames; each rule/skill/agent file is owned by a single task; `Makefile` +
  `.github/workflows/ci.yml` → TASK-8; the doc-sync umbrella (TASK-11) owns the cross-cutting docs
  exclusively (and references TASK-3/4's capability docs by filename only, so it needs no dependency
  on them); `MANIFEST.md` is generated only by TASK-12.

## Validation Criteria

- [ ] `go build ./...` passes (includes `tools/validate-spec`)
- [ ] `go vet ./...` clean; `goimports -l .` empty
- [x] `go test ./tools/...` passes — all SDDX-TC-UC-01..39 green
- [ ] `go test -cover ./tools/...` reports high statement coverage for `tools/validate-spec`
      (well-tested by construction); `.github/workflows/ci.yml` Unit-Tests `go list` includes
      `./tools/...` so the `coverage-delta` gate (≥70% on changed lines) sees it
- [ ] `diff-cover` reports coverage for changed `tools/validate-spec/*.go` lines (verify
      `gocover-cobertura` resolves the `…/tools/validate-spec/…` paths — grep `tools/validate-spec`
      in the diff-cover output)
- [ ] `golangci-lint run ./...` clean — no new findings; `gosec` exemption for `tools/validate-spec`
      applied in `.golangci.yml`
- [ ] `make test-unit` and `make test-coverage` both include `./tools/...`
- [ ] `make validate-spec FILE=.specs/sdd-capabilities-and-validation.md` exits 0 (this spec is its
      own conforming example — the dogfood gate)
- [ ] `make validate-spec` (glob mode, no `FILE=`) exits 0 and processes at least this spec;
      `TEMPLATE.md` is NOT processed (path exclusion); grandfathered no-slug specs are skipped
- [ ] `make capabilities-manifest` generates `docs/capabilities/MANIFEST.md` listing all four
      capabilities; re-running produces no diff (idempotent)
- [ ] `docs/capabilities/{user,role,idempotency,caching}.md` exist, follow `TEMPLATE.md`, are
      `Status: Active`, and accurately describe current code (review check)
- [ ] `.claude/rules/sdd.md` contains `SUPERSEDED`/`ARCHIVED`, amend-in-place + decision rule, the
      Capability Docs section, the Deterministic Spec Linter section, and slug-prefix +
      TC-prefix-registry + `(no-test:)` rules (grep-verify)
- [ ] `.specs/TEMPLATE.md`, `spec/SKILL.md`, `ralph-loop/SKILL.md`, `spec-reviewer.md`,
      `spec-review/SKILL.md` reference the linter gate and capability docs (grep-verify)
- [ ] `docs/harness.md` has rows for capability docs + `validate-spec`; the relevant "Known gaps"
      rows are resolved; `CLAUDE.md`/`README.md`/`CONTRIBUTING.md` mention the new artifacts; all
      markdown cross-references resolve
- [ ] `make lint` passes; `make test` passes

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Batch 1 (2026-06-21)

Foundations. TASK-1 via worktree agent; TASK-2/5/6 inline.

- TASK-1: `tools/validate-spec` implemented with full TDD — fence- + comment-aware,
  declaration-site parser; 15 validators with deterministic sorted output; `manifest`
  subcommand; thin `main` (exit 0/1/2); `.golangci.yml` gosec exemption. 172 test runs, 0 failures.
- TASK-2: `docs/capabilities/{README,TEMPLATE}.md` (convention, lifecycle, amend-in-place).
- TASK-5: `.specs/TEMPLATE.md` — `## Slug:`, canonical `## Status:`, Impacted Capability field,
  slug-prefixed example IDs, TC-prefix registry, `(no-test:)` note.
- TASK-6: `.claude/rules/sdd.md` — `SUPERSEDED`/`ARCHIVED` + amend-in-place, Capability Docs
  section, Deterministic Spec Linter section, slug/registry/`(no-test:)` rules, `| TC-ID |`→`| TC |`.

### Batch 2 (2026-06-21)

TASK-3/4 via background agents (capability docs from real code); TASK-7/8/9/10/11 inline.

- TASK-3: `user.md` (USER, 12 REQs), `role.md` (ROLE, 10 REQs) — verified against real code.
- TASK-4: `idempotency.md` (IDEM, 9 REQs — 5xx-not-cached + 409-on-contention verified),
  `caching.md` (CACHE, 8 REQs — singleflight anti-stampede verified).
- TASK-7: `spec/SKILL.md` — linter gate (Phase 2a, before the 3 agents), capability link in
  Gather Context, slug-prefixed IDs.
- TASK-8: `Makefile` (`validate-spec` + `capabilities-manifest`; `./tools/...` in
  `test-unit`/`test-coverage`), `.github/workflows/ci.yml` (`./tools/...` in Unit-Tests).
- TASK-9: `ralph-loop/SKILL.md` — startup linter gate, capability-doc + manifest wrap-up,
  reviewer check, `SUPERSEDED`/`ARCHIVED` awareness.
- TASK-10: `spec-reviewer.md` + `spec-review/SKILL.md` — slug/capability checks,
  linter-as-pre-filter, `SUPERSEDED`/`ARCHIVED`, slug-aware REQ extraction.
- TASK-11: `docs/harness.md`, `CLAUDE.md`, `README.md`, `CONTRIBUTING.md`,
  `docs/guides/sdd-ralph-loop.md` synced + new `docs/guides/spec-linter.md`.

### Batch 3 (2026-06-21)

- TASK-12: `docs/capabilities/MANIFEST.md` generated (4 capabilities, idempotent). Dogfood:
  `make validate-spec FILE=.specs/sdd-capabilities-and-validation.md` → 0 errors / 0 warnings,
  exit 0; glob mode exit 0.

### Fixes discovered during execution (via the dogfood gate)

- **manifest CLI**: aligned to REQ-7 — `--write` defaults to `docs/capabilities/MANIFEST.md` and
  caps-dir defaults to `docs/capabilities`; `runManifest` skips `README.md`/`TEMPLATE.md` (not just
  `MANIFEST.md`) so only real capability docs are listed. +test.
- **parser comment-awareness**: added `stripComments` — HTML comments routinely contain `## `
  headings and REQ/TC examples (the gopherplate TEMPLATE does), which were being mis-parsed as
  structure and silently dropped all TCs. +test (`TestParseSpec_IgnoresHTMLComments`); +TC-UC-39.
