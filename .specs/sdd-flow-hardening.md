# Spec: SDD Flow Hardening (Fowler best-practices follow-up)

## Status: DONE

## Slug: SDDF

## Context

This spec is a **follow-up increment** to `.specs/sdd-capabilities-and-validation.md` (slug `SDDX`,
already merged: `tools/validate-spec`, `docs/capabilities/`, slug-prefixed IDs, `SUPERSEDED`/
`ARCHIVED`, amend-in-place). It absorbs the genuinely-new best-practice gaps distilled from Martin
Fowler's SDD article ([sdd-3-tools](https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html))
plus a multi-source audit (GitHub Spec Kit, AWS Kiro, EARS, agentic-loop patterns) run on the
sister project `banking-api-ledger`. The audit's verdict: our flow already follows — and on the
deterministic axis exceeds — market SDD practice; the remaining gaps are low-effort mesh-closures
that make the flow *coherent with its own rigor*, not conceptual holes.

Two gaps from the Ledger analysis do **not** apply here and are recorded for the audit trail:

- **Gap #6 (phantom refs) — N/A.** `/fix-issue` (`.claude/skills/fix-issue/SKILL.md`) and
  `.specs/TEMPLATE.md` both already exist in this repo. Nothing to materialize.
- **Gap #1 (validate-spec in server-side CI) — dropped.** `validate-spec` validates a *process*
  artifact (specs/capability docs), not a production contract. A malformed spec on `main` is an
  imperfect doc — it does not corrupt runtime, money, or deploy. Production gates (lint, unit, e2e,
  vulncheck) already run in CI; the spec gate belongs at authoring/execution (the `/spec` Phase 2a
  gate, the `/ralph-loop` startup gate, the pre-push hook). Forcing a deploy-pipeline gate on a
  process artifact is the "ceremony as false control" the article warns against. Kept pre-push only.

### The gaps absorbed here

- **#2 — TC↔task round-trip (HIGH).** The linter parses `tests:` but never proves the round-trip:
  an orphan TC (declared, referenced by no task) or a dangling `tests:` ref is invisible. (The
  Ledger's code-grep `--resolve-tests` does not fit gopherplate, whose tests use natural-English
  names, not embedded TC-IDs — so we enforce the **spec-internal** round-trip deterministically.)
- **#3 — Security as a first-class authoring lens (MEDIUM, new).** `/spec` Phase 2 has three lenses
  (spec/test/code) but no security — yet the thesis is "a wrong spec becomes wrong code faithfully".
- **#4 — Brownfield search-first + duplication lens (MEDIUM, new).** A mature repo under
  `/ralph-loop` risks re-implementing what exists. Add "search before creating" + a duplication lens.
- **#5 — Cross-artifact `files:` vs. diff (MEDIUM, new).** An over-eager agent can add a file no
  task declared; a modified file with no owning task should be a MUST FIX.
- **#7 — Capability↔code drift (LOW).** `## Source` lists only specs/ADRs; no machine-checkable code
  path, so a doc can silently outlive the code it describes.
- **#8 — Capability bootstrap (LOW).** A generator that emits the discovery boilerplate (paths +
  sentinel errors) accelerates coverage; the human fills the WHY.
- **Böckeler escape-hatch (doctrine).** `sdd.md` freezes Requirements + Test Plan during execution;
  document the trade-off and add a *controlled* amendment escape-hatch.

A latent linter bug surfaced while authoring this spec (gap #11 below): the task-declaration matcher
is `TASK-\d+`-only, so named tasks (`TASK-SMOKE`/`TASK-MERGE-*`/`TASK-FINAL`) are unrecognized and
their `files:`/`depends:` bleed into the preceding numbered task — corrupting the declared-files set
(#5) and the round-trip (#2). It is a correctness prerequisite, fixed here (REQ-11).

### Out of scope

- The Ledger code-grep `--resolve-tests` (TC-IDs in `*_test.go`) — our convention is
  natural-English test names; the spec-internal round-trip (#2) is the robust equivalent.
- `validate-spec` in server-side CI (gap #1, dropped above).
- Auto-authoring capability docs for undocumented subsystems — gap #8 ships the **generator only**.
- Domain-invariant test harness (the Ledger's `internal/testkit/invariants`) — Ledger-specific.

## Requirements

<!-- GIVEN/WHEN/THEN. Documentation-only REQs carry (no-test: <reason>) on the declaration line. -->

- [ ] SDDF-REQ-1: **TC↔task round-trip is enforced deterministically.**
  GIVEN a slug-bearing spec,
  WHEN `tools/validate-spec <file>` runs,
  THEN `testsRefValid` errors (naming the task and the TC-ID) if any TC-ID in a task's `tests:` is
  not declared in the Test Plan (dangling), and `tcReferenced` errors (naming the TC-ID) if any
  declared TC is referenced by **no** task's `tests:` (orphan). A TC referenced by **one or more**
  tasks is fine — multi-reference is allowed (the rule is ≥1, not exactly-one). Specs without
  `## Slug:` are grandfathered (skipped), consistent with the other slug-gated validators.

- [ ] SDDF-REQ-2: **The declared-files set is machine-readable, and execution-only tasks are
  first-class.**
  GIVEN a spec file,
  WHEN `tools/validate-spec files <file>` runs,
  THEN it prints, one per line, the deduplicated, sorted union of every task's `files:` entries
  (exit 0). A task whose sole `files:` value is the execution-only sentinel `(none — execution
  only)` (or `(none)`) contributes **no** paths to that union, is **exempt** from `taskHasFiles`,
  and never participates in `batchFileOverlap` — so `validate-spec files` and the other validators
  never see the sentinel parenthetical as a path.

- [ ] SDDF-REQ-3: **`/ralph-loop` audits the working tree against declared files.** (no-test: flow/skill wiring — verified by review + the TASK-9 dogfood, like other skill-wiring REQs)
  GIVEN a spec under execution,
  WHEN `/ralph-loop` reaches its wrap-up,
  THEN it compares the **working-tree** changed set — staged (`--cached`) + unstaged + untracked
  (`git ls-files --others --exclude-standard`); at wrap-up the spec is not yet committed (ralph-loop
  commits in Phase 5, after this gate), so the working tree captures exactly this spec's changes
  without prior-spec noise, and `git ls-files --others` is required because plain `git diff` omits
  newly-created files (the very thing the gate must catch) — against `validate-spec files <spec>`
  (a declared directory entry like `pkg/foo/` covers everything beneath it by prefix; exact paths
  match exactly) plus an allowlist (the spec file itself,
  `.specs/wiring/<spec-slug>/**`, `docs/capabilities/MANIFEST.md`, `gen/**`); any remaining changed
  file is an **undeclared file** surfaced as a MUST FIX (stop + ask, mirroring auto-rollback). The
  allowlist exempts a file only from this declaration gate, never from the content review lenses;
  `gen/**` is trusted on the assumption it stays gitignored. A `make spec-files-audit FILE=<spec>`
  target performs the same comparison on demand.

- [ ] SDDF-REQ-4: **Security is a first-class lens in `/spec` authoring.** (no-test: skill/agent doc wiring — verified by grep/review of spec/SKILL.md + security-reviewer.md)
  GIVEN the `/spec` self-review,
  WHEN Phase 2b spawns its review agents,
  THEN it spawns **four** lenses in parallel — `spec-reviewer`, `test-reviewer`, `code-reviewer`,
  `security-reviewer` — where the security lens audits the *spec* for: planned tenant scope; PII in
  planned logs/fixtures; **the service-key middleware/interceptor named for every new endpoint
  (reject "internal-only ⇒ no auth")** per ADR-005/`security.md`; sentinel→status mappings that
  could leak a cross-context resource; secrets in fixtures; and **the `## Dependencies` section for
  any new network-calling or stdlib-replacing dependency**. `security-reviewer.md` documents this
  pre-code spec-review mode distinct from its post-code diff mode.

- [ ] SDDF-REQ-5: **Brownfield reuse is enforced (search-first + duplication lens).** (no-test: skill/agent doc wiring — verified by grep/review of ralph-loop/SKILL.md + code-reviewer.md)
  GIVEN `/ralph-loop` executing a task in a mature repo,
  WHEN the task would add new code or a new file,
  THEN `ralph-loop/SKILL.md` requires searching first (grep `internal/`/`pkg/` for an existing
  implementation; reuse > re-implement), and `code-reviewer.md` carries an explicit duplication lens
  ("does this re-implement something already in `internal/`/`pkg/` that should be reused?").

- [ ] SDDF-REQ-6: **Capability↔code drift is detectable.**
  GIVEN capability docs that declare a `## Code` section of source paths and a `Last-verified` marker,
  WHEN `tools/validate-spec capabilities [dir]` runs (skipping `README.md`/`TEMPLATE.md`/`MANIFEST.md`),
  THEN every path under a doc's `## Code` is `os.Stat`-checked (a non-existent path is an ERROR
  naming the doc and the path), a doc with no `## Code` section emits a WARN, and a path whose latest
  git commit is newer than the doc's `Last-verified` marker emits a WARN; git stderr is discarded and
  any git/marker/parse failure degrades to "no WARN" (never blocks). The `Last-verified` marker is an
  honesty signal — it catches *accidental* drift, not adversarial backdating; the dead-path ERROR is
  the only hard guarantee. Exit 1 on any ERROR, else 0.

- [ ] SDDF-REQ-7: **Capability docs carry a machine-checkable `## Code` section.** (no-test: documentation/convention — verified by `make capabilities-check` (REQ-6) at the final criteria + grep; the check ships in TASK-7/Batch-2, after the docs in TASK-2/Batch-1)
  GIVEN `docs/capabilities/`,
  WHEN a reader opens `TEMPLATE.md` and the four worked docs (`user`, `role`, `idempotency`,
  `caching`),
  THEN each declares a `## Code` section listing the real source paths/packages it describes plus a
  `Last-verified: <date> (<commit>)` line, and `os.Stat` of every listed path succeeds. All fixtures
  and docs use **synthetic** data only — no real emails, tokens, hostnames, or PII.

- [ ] SDDF-REQ-8: **A capability-doc skeleton can be bootstrapped from code.**
  GIVEN a Go package path,
  WHEN `tools/validate-spec bootstrap-capability <pkg>` runs,
  THEN it prints (stdout only — never auto-writing a file; the human fills the WHY) a skeleton
  following `docs/capabilities/TEMPLATE.md`: `Status: Active`; a `## Code` section auto-populated with
  the package's top-level non-`_test.go` `.go` files (no recursion; symlinked entries skipped) + a
  `Last-verified` marker; an Error-Contract section auto-populated with the package's sentinel errors
  (`var Err… = errors.New(…)`); and `Guarantees`/`Invariants`/`Source` left as `TODO`. A package with
  no sentinel errors yields an empty Error-Contract (not a crash); a non-existent package exits 2.

- [ ] SDDF-REQ-9: **The freeze trade-off is documented with a controlled escape-hatch.** (no-test: convention — verified by grep/review of sdd.md + spec/ralph-loop SKILLs)
  GIVEN `.claude/rules/sdd.md`,
  WHEN a reader consults Spec File Integrity / Mutability,
  THEN it documents the up-front-design-vs-late-discovery trade-off (Böckeler) and defines a narrow
  escape-hatch: a REQ MAY be amended while status is `IN_PROGRESS` **only if** the full self-review
  (now four lenses) re-runs from scratch and an Execution Log entry records the amendment + its
  rationale; `spec/SKILL.md` and `ralph-loop/SKILL.md` reflect the four-lens re-review.

- [ ] SDDF-REQ-10: **Harness inventory and contributor docs stay in sync.** (no-test: documentation sync — verified by grep + cross-reference resolution review)
  GIVEN the new validators/modes/lenses,
  WHEN a reader consults `docs/harness.md`, `docs/guides/spec-linter.md`, `CLAUDE.md`, `README.md`,
  `CONTRIBUTING.md`, and `docs/guides/sdd-ralph-loop.md`,
  THEN the round-trip validators, the `files`/`capabilities`/`bootstrap-capability` modes, the
  fourth (security) authoring lens, the brownfield search-first rule, the files-vs-diff audit, and
  the `## Code` convention all appear; the relevant "Known gaps" are resolved; gap #6 is noted as
  already-satisfied. All cross-references resolve.

- [ ] SDDF-REQ-11: **The linter recognizes named tasks (`TASK-SMOKE`, `TASK-MERGE-*`, `TASK-FINAL`).**
  GIVEN a spec using the named-task convention from `.specs/TEMPLATE.md` / `sdd.md`,
  WHEN `tools/validate-spec <file>` parses it,
  THEN the task-declaration matcher recognizes `TASK-<digits>` AND `TASK-<UPPER-segments>` — exactly
  `^- \[[ x]\] (TASK-(?:\d+|[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*)): ` — so each named task
  (`TASK-SMOKE`, `TASK-MERGE-<TARGET>`, `TASK-FINAL`) is parsed as its own task and its
  `files:`/`tests:`/`depends:` are attributed correctly; a `TASK-…` token appearing in prose (not as
  a `- [x] TASK-…: ` declaration line) is NOT parsed as a task. This closes the latent
  metadata-bleed bug that would corrupt the declared-files set (REQ-2) and the round-trip (REQ-1).

## Test Plan

<!--
  Testable production code = the validate-spec additions. NO Domain/E2E/Smoke tests: no entity,
  endpoint, or running system. Documentation/flow REQs (-3, -4, -5, -7, -9, -10) carry (no-test:)
  and are verified by Validation Criteria + grep/review + the TASK-9 dogfood.

  The capabilities git-log staleness shells out to git; its PURE `isStale(lastVerified, lastCommit)`
  is unit-tested (incl. the equal-timestamp boundary), the os.Stat dead-path branch is tested with
  fixtures, and the git-absent suppression branch is tested by injecting the commit-date lookup (no
  real repo needed — no flake). The thin exec(git) wrapper is exercised by the dogfood. The golden
  (TC-03) is the all-pass vacuous-pass guard and is updated to satisfy the new round-trip rule.

  Assertions check message CONTENT (the named TC/task/path), not just severity counts, so a
  right-severity-wrong-content mutation cannot survive. Fixtures are synthetic (no PII/secrets).
-->

### Use Case Tests (linter — `tools/validate-spec`)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| SDDF-TC-UC-01 | SDDF-REQ-1 | validation | A task's `tests:` lists a TC-ID not declared in the Test Plan (dangling) | ERROR whose message names BOTH the dangling TC-ID and the task ID |
| SDDF-TC-UC-02 | SDDF-REQ-1 | validation | A TC declared in the Test Plan referenced by no task's `tests:` (orphan) | ERROR whose message names the specific orphan TC-ID |
| SDDF-TC-UC-03 | SDDF-REQ-1 | happy | Golden synthetic spec: every TC referenced by ≥1 task, every `tests:` entry a real TC | 0 round-trip findings (golden stays 0 ERROR / 0 WARN overall) |
| SDDF-TC-UC-04 | SDDF-REQ-1 | edge | A grandfathered spec (no `## Slug:`) with an orphan TC | round-trip SKIPPED — no error |
| SDDF-TC-UC-05 | SDDF-REQ-1 | edge | A smoke TC (`<SLUG>-TC-S-01`) referenced only via `TASK-SMOKE`'s `tests:` (TASK-SMOKE present in a batch) | not flagged orphan (TASK-SMOKE counts as a referencing task) |
| SDDF-TC-UC-06 | SDDF-REQ-2 | happy | `files <spec>` over a multi-task spec whose paths are deliberately out-of-lexical-order | prints the union, one per line, in lexicographic order (exact slice equality) |
| SDDF-TC-UC-07 | SDDF-REQ-2 | edge | Two tasks list the same file plus each a unique file | the shared file appears exactly once; both unique files present (dedup keeps all distinct) |
| SDDF-TC-UC-08 | SDDF-REQ-6 | validation | A capability doc whose `## Code` lists a non-existent path | ERROR whose message names BOTH the doc filename and the dead path |
| SDDF-TC-UC-09 | SDDF-REQ-6 | happy | A capability doc whose `## Code` paths all exist | no ERROR from the path check |
| SDDF-TC-UC-10 | SDDF-REQ-6 | edge | A capability doc with no `## Code` section | WARN (drift cannot be verified); not an ERROR |
| SDDF-TC-UC-11 | SDDF-REQ-6 | edge | `isStale(lastVerified, lastCommit)`: commit newer → true; commit older → false; **commit equal → false** (boundary) | pure function correct in all three cases; a stale doc emits a WARN (not ERROR) |
| SDDF-TC-UC-12 | SDDF-REQ-6 | business | Exit codes for `capabilities`: a dead-path fixture → exit 1; a stale-only fixture → exit 0; a no-`## Code` fixture → exit 0; an all-valid fixture → exit 0 (each its own fixture) | exit codes as described |
| SDDF-TC-UC-13 | SDDF-REQ-8 | happy | `bootstrap-capability` on a fixture package with sentinel errors AND an `entity_test.go` | skeleton has `Status: Active`, `## Code` listing the non-test `.go` files ONLY (`_test.go` excluded) + `Last-verified`, an Error-Contract listing the sentinel errors, and `TODO` Guarantees/Invariants/Source |
| SDDF-TC-UC-14 | SDDF-REQ-8 | edge | `bootstrap-capability` on a fixture package with NO sentinel errors | skeleton with an empty Error-Contract section + the `## Code` paths (no crash) |
| SDDF-TC-UC-15 | SDDF-REQ-8 | infra | `bootstrap-capability` on a non-existent package path | exit 2; stderr message contains the requested package path (context-wrapped) |
| SDDF-TC-UC-16 | SDDF-REQ-1 | business | A spec that passes EVERY existing validator and fails ONLY the round-trip (one orphan TC, nothing else wrong) → exit 1; the golden → exit 0 | exit codes as described (round-trip causally isolated) |
| SDDF-TC-UC-17 | SDDF-REQ-11 | validation | A spec whose last task is `TASK-SMOKE` (and one `TASK-MERGE-SERVER`) carrying its own `files:`/`depends:` | each named task is a DISTINCT task; its `files:`/`depends:` are NOT attributed to the preceding numbered task |
| SDDF-TC-UC-18 | SDDF-REQ-6 | infra | The commit-date lookup returns "absent" (injected) for a path with no git history | no staleness WARN emitted (suppression branch); no error |
| SDDF-TC-UC-19 | SDDF-REQ-11 | edge | A numbered task whose DESCRIPTION text mentions `TASK-SMOKE` (e.g. "run after TASK-SMOKE"), not as a declaration line | no spurious `TASK-SMOKE` task parsed; `model.tasks` holds only the real declarations |
| SDDF-TC-UC-20 | SDDF-REQ-1 | edge | A TC referenced by TWO tasks' `tests:` entries | not flagged orphan; 0 round-trip findings (multi-reference is legal) |
| SDDF-TC-UC-21 | SDDF-REQ-6 | edge | `capabilities` over a dir containing `MANIFEST.md` + `README.md` + a valid doc | findings only for the valid doc; no phantom findings from MANIFEST/README (skip honored) |
| SDDF-TC-UC-22 | SDDF-REQ-2 | edge | A task whose only `files:` value is `(none — execution only)` | contributes no path to `files` output; `taskHasFiles` does NOT error on it; not in any `batchFileOverlap` comparison |

## Design

### Architecture Decisions

**`tools/validate-spec` extensions.** Stays a single `package main` in the root module (SDDX
decision), golangci-clean under the existing `tools/validate-spec/` `gosec` exemption (which already
covers G304 and the new G204 from `os/exec` — **not widened**). `main.go` dispatch gains `files`,
`capabilities`, `bootstrap-capability`. Module path is `github.com/jrmarcello/gopherplate`.

- `parser.go` — widen the task matcher to exactly
  `reTaskDecl = ^- \[[ x]\] (TASK-(?:\d+|[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*)): ` (REQ-11). It still
  requires the `- [ ] ` prefix + trailing `: `, so a `TASK-…` token in prose never matches. Add the
  execution-only sentinel: when a task's `files:` value (trimmed) is `(none — execution only)` or
  `(none)`, the parser records **zero** files and sets a `task.execOnly` flag (REQ-2). No other
  parser change — `model.tcs` and `task.tests` are already collected (SDDX).
- `validate.go` — two new validators composed into `validate()`, **slug-gated** (no-op when
  `model.slug == ""`):
  - `testsRefValid` (ERROR): every `tests:` entry must be a declared TC; dangling →
    `task %s tests: references undeclared TC %q`.
  - `tcReferenced` (ERROR): every Test-Plan TC must appear in ≥1 task's `tests:` (any task, incl.
    `TASK-SMOKE`); orphan → `TC %s is declared but referenced by no task's tests:`. **Multi-reference
    is allowed** (≥1, not exactly-one) — the golden has a shared TC and must stay green.
    Requires REQ-11 (named tasks parsed) to count `TASK-SMOKE` references — both ship in TASK-1.
  - `taskHasFiles` is amended to **exempt** `task.execOnly` tasks (REQ-2).
- `files.go` — pure `collectFiles(model) []string`: union of `task.files` (excluding execOnly tasks,
  which carry none), dedupe via a set, `sort.Strings`. `main` prints one per line (exit 0); read
  error → exit 2.
- `capabilities.go` — `runCapabilities(dir)`: for each `*.md` under `dir` **skipping
  `README.md`/`TEMPLATE.md`/`MANIFEST.md`**, parse a `## Code` path list + `Last-verified: <date>
  (<commit>)`. Per path: `os.Stat` → dead = ERROR (`capability %s: code path %q does not exist`). No
  `## Code` → WARN. Staleness: a **pure** `isStale(lastVerified, lastCommit time.Time) bool`
  (strictly-newer ⇒ true; equal ⇒ false) drives a WARN; the date is supplied by a thin, injectable
  `commitDate(path) (time.Time, bool)` defaulting to
  `exec.Command("git", "log", "-1", "--format=%cI", "--", path)` (fixed args, **no shell**, `--`
  kept, `Cmd.Stderr` discarded, `path` resolved via `filepath.Abs`). Any failure (no git, untracked,
  unparseable date/marker) ⇒ `(zero, false)` ⇒ no WARN. The lookup is a function field on the
  validator so tests inject a fake (no real repo → no flake). Exit 1 on any ERROR, else 0.
- `bootstrap.go` — `runBootstrap(pkgDir)`: `os.ReadDir` (non-existent → exit 2, context-wrapped
  message incl. the path), collect **top-level** non-`_test.go` `.go` files (no recursion; skip
  entries whose `info.Type()&os.ModeSymlink != 0`), grep each for
  `reSentinel = ^\s*(Err[A-Za-z0-9]+)\s*=\s*errors\.New\(`, and render via a **pure**
  `renderBootstrap(name, files, errs, date, commit) string` to **stdout only** (never auto-write).
  Note: `pkg/apperror` defines constructors, not `var Err… = errors.New(…)` sentinels, so it yields
  an empty Error-Contract (TC-14's case); the sentinel source is the domain packages
  (`internal/domain/*/errors.go`), which the dogfood (TASK-9) exercises.
- `*_test.go` — table-driven, hermetic, stdlib `testing` + testify; pure functions tested directly,
  `os.Stat`/`ReadDir` via `testdata/`, the git lookup injected. **All new testdata is synthetic** —
  placeholder paths/commits, `ErrFixture…`-style sentinel names, no PII/secrets.
- `testdata/` — round-trip `tests-dangling.md`, `tc-orphan.md`, `grandfathered-orphan.md`,
  `smoke-referenced.md` (TASK-SMOKE in a batch), `tc-multiref.md`, `task-execonly.md`, `prose-task.md`,
  `roundtrip-only.md` (clean except one orphan, for TC-16); `files-union.md`; capability fixtures
  `caps-deadpath/`, `caps-nocode/`, `caps-stale/`, `caps-valid/`, `caps-skip/` (with MANIFEST/README);
  bootstrap `bootstrap/witherrors/` (incl. an `entity_test.go`), `bootstrap/noerrors/`. The existing
  `valid/golden.md` is updated so every TC is referenced by ≥1 task and every `tests:` entry is a
  real TC (vacuous-pass guard).

**`## Code` convention (concept #7).** Capability docs gain a `## Code` section (source paths/
packages) + a `Last-verified: <YYYY-MM-DD> (<short-commit>)` line. `os.Stat`-checkable. `TEMPLATE.md`
models it; the four worked docs get their real paths. `bootstrap-capability` emits this section.

**Security 4th lens / brownfield / files-vs-diff / Böckeler** — as in the REQs; skill/agent/rule
files updated (TASK-3/4/5/6).

### Impacted Capability

Extends the **SDD harness tooling** (`tools/validate-spec`) and conventions — harness-internal, no
service-capability doc to update. It introduces the `## Code` convention that
`docs/capabilities/TEMPLATE.md`, `docs/capabilities/user.md`, `docs/capabilities/role.md`,
`docs/capabilities/idempotency.md`, and `docs/capabilities/caching.md` adopt (TASK-2).

### Files to Create

- `tools/validate-spec/files.go` + `files_test.go`
- `tools/validate-spec/capabilities.go` + `capabilities_test.go`
- `tools/validate-spec/bootstrap.go` + `bootstrap_test.go`
- `tools/validate-spec/testdata/` fixtures (see Architecture)

### Files to Modify

- `tools/validate-spec/parser.go` (+ `parser_test.go`) — named-task matcher (REQ-11); execOnly sentinel (REQ-2)
- `tools/validate-spec/validate.go` (+ `validate_test.go`) — `testsRefValid`, `tcReferenced`; `taskHasFiles` execOnly exemption
- `tools/validate-spec/main.go` (+ `main_test.go`) — dispatch `files`/`capabilities`/`bootstrap-capability`
- `tools/validate-spec/testdata/valid/golden.md` — satisfy the round-trip rule
- `docs/capabilities/TEMPLATE.md` + `user.md` + `role.md` + `idempotency.md` + `caching.md` — `## Code` + `Last-verified`
- `.claude/skills/spec/SKILL.md` — 4th (security) lens + four-lens re-review
- `.claude/skills/ralph-loop/SKILL.md` — search-first; files-vs-diff wrap-up (committed+staged+unstaged); four-lens re-review
- `.claude/agents/code-reviewer.md` — duplication lens; `.claude/agents/security-reviewer.md` — spec-review mode
- `.claude/rules/sdd.md` — round-trip rule; `files`/`capabilities`/`bootstrap` + `## Code` convention; execOnly sentinel; Böckeler escape-hatch; gap-#6-satisfied note
- `Makefile` — `capabilities-check`, `spec-files-audit`
- `docs/harness.md` — sensor rows; `docs/guides/spec-linter.md` — new modes/validators; `CLAUDE.md`, `README.md`, `CONTRIBUTING.md`, `docs/guides/sdd-ralph-loop.md` — sync

### Dependencies

None external. Go stdlib (`os`, `os/exec`, `path/filepath`, `regexp`, `sort`, `strings`, `time`) +
testify (tests). Module path `github.com/jrmarcello/gopherplate`.

## Tasks

<!-- Files exclusive to one task — no shared-additive, no wiring fragments. -->

- [x] TASK-1: Implement all `tools/validate-spec` changes with full TDD — the named-task matcher
  (exact regex, REQ-11) + execution-only `(none …)` sentinel (REQ-2) in `parser.go`; the
  `testsRefValid` + `tcReferenced` validators (slug-gated, ≥1) + `taskHasFiles` execOnly exemption in
  `validate.go`; the `files`/`capabilities`/`bootstrap-capability` subcommands (pure cores + injectable
  git lookup + thin `os`/`exec` wrappers, stderr discarded, `filepath.Abs`) in new files; the `main`
  dispatch; the new synthetic `testdata/` fixtures; and the golden update to satisfy the round-trip
  rule. One cohesive `package main` — do not split. RED → GREEN → REFACTOR;
  `golangci-lint run ./tools/validate-spec/...` clean.
  - files: tools/validate-spec/parser.go, tools/validate-spec/parser_test.go, tools/validate-spec/validate.go, tools/validate-spec/validate_test.go, tools/validate-spec/main.go, tools/validate-spec/main_test.go, tools/validate-spec/files.go, tools/validate-spec/files_test.go, tools/validate-spec/capabilities.go, tools/validate-spec/capabilities_test.go, tools/validate-spec/bootstrap.go, tools/validate-spec/bootstrap_test.go, tools/validate-spec/phase3_review_test.go, tools/validate-spec/testdata/
  - tests: SDDF-TC-UC-01, SDDF-TC-UC-02, SDDF-TC-UC-03, SDDF-TC-UC-04, SDDF-TC-UC-05, SDDF-TC-UC-06, SDDF-TC-UC-07, SDDF-TC-UC-08, SDDF-TC-UC-09, SDDF-TC-UC-10, SDDF-TC-UC-11, SDDF-TC-UC-12, SDDF-TC-UC-13, SDDF-TC-UC-14, SDDF-TC-UC-15, SDDF-TC-UC-16, SDDF-TC-UC-17, SDDF-TC-UC-18, SDDF-TC-UC-19, SDDF-TC-UC-20, SDDF-TC-UC-21, SDDF-TC-UC-22

- [x] TASK-2: Add a `## Code` section + `Last-verified` line (real source paths) to
  `docs/capabilities/TEMPLATE.md` and the four worked docs; verify each path exists.
  - files: docs/capabilities/TEMPLATE.md, docs/capabilities/user.md, docs/capabilities/role.md, docs/capabilities/idempotency.md, docs/capabilities/caching.md

- [x] TASK-3: Update `.claude/skills/spec/SKILL.md` — Phase 2b spawns four lenses (add
  `security-reviewer`, spec-focused prompt incl. service-key/`## Dependencies`); four-lens re-review.
  - files: .claude/skills/spec/SKILL.md
  - depends: TASK-6

- [x] TASK-4: Update `.claude/skills/ralph-loop/SKILL.md` — search-first clause; files-vs-diff audit
  in wrap-up (committed + staged + unstaged vs `validate-spec files` + allowlist); four-lens re-review.
  - files: .claude/skills/ralph-loop/SKILL.md
  - depends: TASK-6

- [x] TASK-5: Update `.claude/agents/code-reviewer.md` (brownfield duplication lens) and
  `.claude/agents/security-reviewer.md` (pre-code spec-review mode + the service-key/`## Dependencies`
  checks, distinct from post-code diff).
  - files: .claude/agents/code-reviewer.md, .claude/agents/security-reviewer.md
  - depends: TASK-6

- [x] TASK-6: Update `.claude/rules/sdd.md` — the TC↔task round-trip rule; the `files` /
  `capabilities` / `bootstrap-capability` modes; the `## Code` convention; the execution-only
  `(none …)` sentinel; the Böckeler freeze trade-off + controlled IN_PROGRESS amendment escape-hatch
  (four-lens re-review + Execution Log); a note that gap #6 is already satisfied. Canonical source.
  - files: .claude/rules/sdd.md

- [x] TASK-7: Add `Makefile` targets `capabilities-check` (`go run ./tools/validate-spec capabilities
  docs/capabilities`) and `spec-files-audit` (changed-set [committed+staged+unstaged] vs
  `validate-spec files` + allowlist); register in `.PHONY` + `help`.
  - files: Makefile
  - depends: TASK-1

- [x] TASK-8: Sync contributor docs — `docs/harness.md` (sensor rows; resolve relevant Known gaps),
  `docs/guides/spec-linter.md` (new modes/validators + four-lens note), `CLAUDE.md`, `README.md`,
  `CONTRIBUTING.md`, `docs/guides/sdd-ralph-loop.md`. Reference capability docs by filename only.
  - files: docs/harness.md, docs/guides/spec-linter.md, CLAUDE.md, README.md, CONTRIBUTING.md, docs/guides/sdd-ralph-loop.md
  - depends: TASK-1, TASK-6

- [x] TASK-9: Dogfood — `make validate-spec FILE=.specs/sdd-flow-hardening.md` exits 0 (the new
  round-trip validators against this very spec), `make capabilities-check` exits 0 (the four docs'
  `## Code` paths exist), and `tools/validate-spec bootstrap-capability internal/domain/user`
  produces a non-empty skeleton whose Error-Contract lists `internal/domain/user`'s sentinel errors
  (smoke — exercises the sentinel path on a real domain package). Named `TASK-9`, not `TASK-FINAL`,
  because the named-task matcher fix it relies on ships in this same spec (REQ-11).
  - files: (none — execution only)
  - depends: TASK-1, TASK-2, TASK-7

## Parallel Batches

<!-- All files exclusive to one task → no shared-additive, no fragments. -->

```text
Batch 1: [TASK-1, TASK-2, TASK-6]                 — foundations (no deps)
Batch 2: [TASK-3, TASK-4, TASK-5, TASK-7, TASK-8] — parallel (deps satisfied by Batch 1)
Batch 3: [TASK-9]                                 — dogfood (validator + docs + make targets in place)
```

File-overlap analysis: every file belongs to exactly one task — no shared files, no
shared-additive/mutative classes, no `.specs/wiring/` fragments. `tools/validate-spec/*` → TASK-1;
the four capability docs + TEMPLATE → TASK-2; `sdd.md` → TASK-6; each skill/agent file owned by a
single task; `Makefile` → TASK-7; the cross-cutting docs → TASK-8 (forward-references capability docs
by filename only). TASK-9 is execution-only (`(none …)` sentinel) and owns no files.

## Validation Criteria

- [ ] `go build ./...` passes; `go vet ./...` clean; `goimports -l .` (excl. `gen/`) empty
- [ ] `go test ./tools/...` passes — all SDDF-TC-UC-01..22 green
- [ ] `golangci-lint run ./...` clean (the `tools/validate-spec/` gosec exemption already covers `os/exec` G204; `.golangci.yml` NOT modified)
- [ ] `make validate-spec FILE=.specs/sdd-flow-hardening.md` exits 0 — this spec passes the **new** round-trip validators (dogfood)
- [ ] `make validate-spec` (glob) exits 0 — no regression on slug-bearing specs
- [ ] `make capabilities-check` exits 0 — every `## Code` path in the four docs exists
- [ ] `tools/validate-spec files .specs/sdd-flow-hardening.md` prints the deduplicated, sorted declared-files union (no `(none …)` sentinel token)
- [ ] `tools/validate-spec bootstrap-capability internal/domain/user` prints a non-empty TEMPLATE-shaped skeleton whose Error-Contract lists the domain's sentinel errors
- [ ] `make spec-files-audit FILE=.specs/sdd-flow-hardening.md` reports 0 undeclared files on a clean tree
- [ ] `.claude/rules/sdd.md` contains the round-trip rule, the `## Code`/`capabilities`/`files`/`bootstrap` docs, the execOnly sentinel, and the Böckeler escape-hatch (grep)
- [ ] `.claude/skills/spec/SKILL.md` spawns four lenses; `ralph-loop/SKILL.md` has the search-first clause + files-vs-diff audit (grep)
- [ ] `docs/harness.md` + `docs/guides/spec-linter.md` document the new validators/modes; cross-references resolve
- [ ] `make lint` passes; `make test` passes

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Batch 1 (2026-06-21)
Parallel: TASK-1 via worktree (validate-spec); TASK-2 + TASK-6 inline.
- TASK-1: named-task matcher (REQ-11) + `(none …)` execOnly sentinel (REQ-2) in parser.go; `testsRefValid` + `tcReferenced` (slug-gated, ≥1) + `taskHasFiles` execOnly exemption in validate.go; `files`/`capabilities`/`bootstrap-capability` subcommands (pure cores + injectable git lookup, stderr discarded, filepath.Abs) in new files; 22 TCs + synthetic fixtures; golden updated for the round-trip. TDD: RED → GREEN(121 passing) → REFACTOR(clean). golangci 0.
- TASK-2: `## Code` + `Last-verified` added to user/role/idempotency/caching + TEMPLATE (real source paths, os.Stat-verified).
- TASK-6: sdd.md — round-trip rule, files/capabilities/bootstrap modes, `## Code` convention, execOnly sentinel, Böckeler freeze trade-off + IN_PROGRESS amendment escape-hatch, /spec 3→4 lenses.

### Batch 2 (2026-06-21)
Parallel via background agents (markdown/Makefile, no build).
- TASK-3: spec/SKILL.md Phase 2b — 4th security lens with spec-review prompt (tenant scope, PII in planned logs/fixtures, service-key per new endpoint, sentinel→status leak, fixture secrets, ## Dependencies supply-chain).
- TASK-4: ralph-loop/SKILL.md — search-first (Case A + Case B), files-vs-diff audit in wrap-up (committed + staged + unstaged), Böckeler escape-hatch note.
- TASK-5: code-reviewer.md brownfield duplication lens; security-reviewer.md two-modes (post-code diff + pre-code spec-review).
- TASK-7: Makefile — `capabilities-check` + `spec-files-audit` (FILE guard, three-way diff, allowlist via case, grep -qF).
- TASK-8: doc-sync — harness.md, spec-linter.md, CLAUDE.md, README.md, CONTRIBUTING.md, sdd-ralph-loop.md.

### Merge integrity (2026-06-21)
The TASK-1 worktree was branched pre-SDDX (0223a5c). Verified `git diff HEAD -- tools/validate-spec/manifest.go` is empty → SDDX core untouched; the diff vs HEAD shows only the expected SDDF deltas (parser +17, validate +49, main +18, tests). Worktree removed + pruned; no stray binary.

### Batch 3 — dogfood (TASK-9, 2026-06-21)
- `validate-spec .specs/sdd-flow-hardening.md` → 0 error / 0 warning (the spec self-passes the new round-trip validators).
- `validate-spec capabilities docs/capabilities` → exit 0 (all `## Code` paths exist, no staleness).
- `validate-spec files .specs/sdd-flow-hardening.md` → clean sorted union, `(none — execution only)` correctly excluded.
- `validate-spec bootstrap-capability internal/domain/user` → skeleton with `## Code` (entity.go, errors.go, filter.go — `_test.go` excluded) + Error Contract (ErrDuplicateEmail, ErrUserNotFound).

### Dogfood bug caught + fixed (2026-06-21)
The `capabilities` parser treated the whole `- <path> — <description>` bullet as the path (the agent's fixtures used bare paths, so unit tests missed it; the real docs exposed it). Fixed `parseCapabilityDoc` to take the first whitespace field (backticks stripped); added `TestParseCapabilityDoc_ExtractsPathFromDescribedBullet` as a unit-level regression guard + updated the caps-* fixtures to the realistic `- path — description` format. Re-ran: capabilities-check exit 0, all tests pass.

### Final validation (2026-06-21)
`gen/proto` regenerated via buf. gofmt clean · go vet ./... 0 · go build ./... ok · go test ./internal/... ./tools/... ok · golangci-lint run 0 issues.

### Self-review (Phase 3, 2026-06-21)
The inferential 5-lens review Workflow hit transient API 529 overload twice; the review was run inline (mechanical battery + critical read). Findings + fixes (all applied):
- **files-vs-diff audit (REQ-3 / TASK-4 SKILL + TASK-7 Makefile) had three defects, all fixed.** (1) It computed the changed set with `git diff` only, which OMITS untracked new files — exactly what the gate must catch; now unions `git ls-files --others --exclude-standard`. (2) No directory-prefix matching: a task that declares a directory (`tools/validate-spec/testdata/`) saw every file beneath it falsely flagged; a trailing-slash declared entry now covers its subtree (awk prefix match). (3) `main...HEAD` over-captured committed predecessor specs on a stacked branch; the audit now scopes to the working tree (correct at pre-commit wrap-up). REQ-3 wording refined to match — controlled amendment per §Mutability, the guarantee (undeclared file = MUST FIX) unchanged.
- **TASK-1 `files:` omitted `tools/validate-spec/phase3_review_test.go`**, which the TASK-1 agent modified (`warnOnlySpec` updated for the new `tcReferenced` validator). The files-vs-diff dogfood caught it; added to TASK-1 `files:`.
- Clean lenses: PII/secret grep of testdata + capability docs (synthetic only, REQ-7); `.golangci.yml` unchanged (gosec exemption not widened); doc-consistency (/spec = 4 lenses, /ralph-loop = 3, round-trip "multi-reference allowed").
- Re-verified after fixes: `make spec-files-audit FILE=.specs/sdd-flow-hardening.md` → OK (0 undeclared); validate-spec gate 0/0; go test ./tools/... + golangci-lint green.

## Review Results — 2026-06-21

Independent post-merge audit (`/spec-review`) of commit `fc1c615`.

### Requirements verification

| Requirement | Status | Evidence |
| --- | --- | --- |
| SDDF-REQ-1: TC↔task round-trip enforced | PASS | `tools/validate-spec/validate.go:228` (testsRefValid), `:250` (tcReferenced) — slug-gated, ≥1 |
| SDDF-REQ-2: declared-files set + execution-only first-class | PASS | `files.go:11` collectFiles, `:30` runFiles; `parser.go:310` execOnly sentinel |
| SDDF-REQ-3: /ralph-loop files-vs-diff audit | PASS | `ralph-loop/SKILL.md:248`; `Makefile` spec-files-audit — proven by negative+positive test (working-tree+untracked+prefix) |
| SDDF-REQ-4: security as 4th /spec lens | PASS | `spec/SKILL.md:86` (four agents), `:95` security-reviewer spec-review prompt |
| SDDF-REQ-5: brownfield search-first + dup-lens | PASS | `ralph-loop/SKILL.md` SEARCH-FIRST; `code-reviewer.md` duplication lens |
| SDDF-REQ-6: capability↔code drift detectable | PASS | `capabilities.go:100` runCapabilities (os.Stat dead=ERROR, no-Code WARN, isStale WARN, skip meta-docs) |
| SDDF-REQ-7: `## Code` convention in docs | PASS | 5 docs (user/role/idempotency/caching/TEMPLATE) — every path os.Stat-verified by `make capabilities-check` (exit 0) |
| SDDF-REQ-8: bootstrap-capability generator | PASS | `bootstrap.go:19` renderBootstrap, `:43` runBootstrap — dogfood emitted skeleton w/ ErrDuplicateEmail/ErrUserNotFound |
| SDDF-REQ-9: Böckeler freeze trade-off + escape-hatch | PASS | `.claude/rules/sdd.md:258` §Mutability |
| SDDF-REQ-10: harness/contributor docs synced | PASS | harness.md, spec-linter.md, CLAUDE.md, README.md, CONTRIBUTING.md, sdd-ralph-loop.md |
| SDDF-REQ-11: named-task matcher | PASS | `parser.go:62` reTaskDecl (TASK-N / TASK-SMOKE / TASK-MERGE-*) |

### Validation checks

| Check | Result |
| --- | --- |
| `gofmt -l .` (excl gen/) | PASS (empty) |
| `go vet ./...` | PASS |
| `golangci-lint run` | PASS (0 issues) |
| `go build ./...` | PASS |
| `go test ./internal/... ./tools/...` | PASS (142 TCs in validate-spec) |
| `validate-spec` gate (this spec) | PASS (0/0) |
| `make validate-spec` (glob) | PASS |
| `make capabilities-check` | PASS (exit 0) |
| `make spec-files-audit` | PASS (0 undeclared) |
| `files` mode (no sentinel token) | PASS |
| `bootstrap-capability` (sentinels listed) | PASS |
| `swag init` drift | N/A (no HTTP handlers touched) |
| `make test-e2e` | N/A (no E2E TCs) |

### Test Quality (inline audit — the independent test-reviewer agent was blocked by repeated API 529)

- **Strong:** `validate_test.go` asserts message CONTENT (TC-01/02: `hasError(fs, "TASK-1")` / `"undeclared TC"` / `"referenced by no task")`; `parser_test.go` + `bootstrap_test.go` are content-rich. Round-trip, named-task-distinct, prose-no-spurious, multi-ref, execOnly, grandfathered-skip all covered. Error-path TCs outnumber happy. Injected fakes (no mock frameworks). No `time.Sleep`/unexplained `t.Skip`.
- **[SHOULD FIX]** `capabilities_test.go` (TestRunCapabilities_DeadPath_Error and siblings) and `main_test.go` (TC-15 nonexistent-pkg) assert **only exit codes**, not the ERROR/stderr **message content** the Test Plan promises (TC-08 "names doc + path"; TC-15 "stderr contains pkg path"). Those strings are verified E2E by the dogfood but a message-format mutation would survive the unit suite. Suggested fix: capture stdout/stderr in those tests and assert the substrings (the parse-side content is already guarded by `TestParseCapabilityDoc_ExtractsPathFromDescribedBullet`).

### Findings (other)

- **Security (independent agent): CLEAN** — synthetic fixtures (REQ-7), `exec(git)` safe (fixed argv, `--`, no shell, stderr discarded), `.golangci.yml` gosec exemption not widened, `spec-files-audit`/`capabilities-check` no shell-injection, bootstrap read-only/stdout-only.
- **Code (inline): correct** — named-task regex, execOnly sentinel, both round-trip validators (slug-gated, ≥1), parseCapabilityDoc path extraction (fixed during Phase 3), isStale (strictly-newer), injectable git lookup (all-failures→no-warn), bootstrap (top-level non-test, symlink skip, exit 2).
- **[NICE TO HAVE]** `.claude/rules/sdd.md` §Merge Strategy fragment example references `github.com/marcelojr/gopherplate`; the real module is `github.com/jrmarcello/gopherplate`. Pre-SDDX doc prose, no functional impact.

### Notes

- The independent code + test review agents failed 4× with transient API 529 (overload); their lenses were covered by the inline Phase-3 + this audit's read of the validators/tests + 142 green TCs + the dogfood. Re-running `/full-review-team` when the API stabilizes would add extra independence.
- **CI (PR):** the three red jobs (Vulnerability Scan, CI-Parity, Coverage Delta) were pre-existing/environmental, NOT SDDF code — fixed in commit `5011542` (go.mod 1.26.2→1.26.4 for a patched stdlib CVE; ci.yml gocover-cobertura via `go run`).
