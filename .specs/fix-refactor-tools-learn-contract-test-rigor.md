# Spec: fix refactor-tools-learn contract-test rigor

## Status: DONE

## Context

`/spec-review` of `refactor-tools-learn-flat-layout.md` (commit `ac0b1ad`)
surfaced **2 MUST FIX + 3 SHOULD FIX + 1 NICE TO HAVE** in the contract test
file `tools/learn/cli_contract_test.go` and one spec-vs-test count
discrepancy. All findings are test-rigor improvements — the refactor's
functional correctness is sound (222 tests pass, build/lint green, all
external contracts preserved). This fix-spec bundles the findings into one
small targeted change.

The findings, ordered by severity:

1. **Spec REQ-1 says 31 production .go files; test asserts 32; reality is 32.**
   The refactor added `doc.go` (centralizing the 4 leaf packages' doc
   comments) but the spec was never updated to reflect it. Resolution:
   update the spec to say "exactly 32 (31 from the original ledger plus
   `doc.go`)" everywhere the count appears.
2. **TC-CT-04 misses 2 of 3 spec-mandated assertions.** Spec requires (a)
   exit 2, (b) stderr `level=ERROR` slog line in `runtimeError.Error()`
   format, (c) JSON-decodable with `LOG_FORMAT=json`. Test currently checks
   only (a) plus a loose keyword match.
3. **TC-CT-02 missing stderr-empty assertion.** Spec expects stderr empty
   for `--help`; a noisy startup warning would be invisible.
4. **TC-CT-10 silently accepts exit 1 OR 2 without deviation comment** —
   inconsistent with TC-CT-03 which explains the cobra short-circuit.
5. **`locatePkgDir` silently falls back to CWD on walk failure** — risks
   false-positive PASS in TC-CT-07/08 under unusual test runners.
6. **(NICE TO HAVE)** Inconsistent exit-code strictness across CT-03 /
   CT-10 / CT-11; a `TODO: wire root.SetFlagErrorFunc` cross-reference
   comment would make the asymmetry traceable.

This spec is intentionally narrow: ~30 LOC delta in one Go file plus a
docs-only patch to the parent spec for the 31→32 reconciliation.

References:
- Parent spec: [`.specs/refactor-tools-learn-flat-layout.md`](refactor-tools-learn-flat-layout.md)
- Parent-spec history: structural refactor committed in `ac020ee`;
  `/spec-review` Review Results section appended in `ac0b1ad` (this fix is the
  follow-up that ac0b1ad's report recommended).
- The 6 findings are quoted verbatim from the `/spec-review` test-quality
  audit; this spec turns each into a REQ.

## Requirements

- [ ] **REQ-1 — Reconcile spec-vs-test production-file count.**
      GIVEN the refactor added `doc.go` at the root of `tools/learn/`,
      AND the parent spec's REQ-1, Design § Architecture Decisions,
      TC-CT-07, TASK-7 VERIFY, and Validation Criteria all say "exactly
      31" (which excludes `doc.go`), WHEN this fix is applied, THEN
      every one of those locations consistently says "exactly 32 (31
      from the original ledger plus `doc.go`)". The contract test
      `cli_contract_test.go` already asserts 32; no change needed to
      the test on this REQ — only to the parent spec's prose and the
      TC-CT-07 row to match reality. REQ verified by TC-D-01.

- [ ] **REQ-2 — TC-CT-04 must assert all 3 spec-mandated postconditions
      on the runtime-error path.** GIVEN the parent spec's TC-CT-04
      requires (a) exit 2, (b) stderr contains a `level=ERROR` slog
      line whose message matches the `runtimeError.Error()` format
      `"runtime: <msg>"`, (c) the error is JSON-decodable when
      `LOG_FORMAT=json` is set, WHEN the test runs, THEN all three
      postconditions are asserted. A regression that drops slog
      wiring and falls back to `fmt.Fprintf` must fail the test.
      **TC-CT-04 must also DROP its `t.Parallel()` call** (the
      function currently calls `t.Parallel()` and adding `t.Setenv`
      without removing parallel would panic at runtime per the
      `testing` package contract). Add an inline comment matching
      TC-CT-09's pattern: `// NOT parallel: mutates process env via
      t.Setenv`. REQ verified by TC-D-02 (which also asserts no
      `t.Parallel()` call remains inside TC-CT-04's body).

- [ ] **REQ-3 — TC-CT-02 must assert stderr empty.** GIVEN the parent
      spec's TC-CT-02 expected says "exit 0; stderr empty; stdout
      non-empty", WHEN each subcommand's `--help` is invoked, THEN the
      test asserts `stderr.String() == ""` (or equivalent
      whitespace-trimmed equality). A subcommand that prints a startup
      warning to stderr while reporting --help must fail the test. REQ
      verified by TC-D-03.

- [ ] **REQ-4 — TC-CT-10 must document the cobra-fallback deviation
      with the same pattern as TC-CT-03.** GIVEN TC-CT-03 accepts exit
      1 or 2 with an inline block comment explaining the cobra
      short-circuit (cli_contract_test.go:96–115), AND TC-CT-10
      currently accepts any non-zero exit without comment, WHEN this
      fix is applied, THEN TC-CT-10 either (option A) carries the
      same deviation comment block referencing the same root cause,
      OR (option B) tightens the assertion to `code == 1`
      contingent on `root.SetFlagErrorFunc` being wired in this same
      spec — but wiring `SetFlagErrorFunc` is explicitly OUT OF SCOPE
      here, so **option A is mandated**. REQ verified by TC-D-04.

- [ ] **REQ-5 — `locatePkgDir` must fail loud on walk failure.**
      GIVEN the current helper at `cli_contract_test.go:388–406`
      silently falls back to CWD when the `go.mod` ascent fails,
      AND TC-CT-07 and TC-CT-08 depend on the returned path being
      `tools/learn/`, WHEN this fix is applied, THEN `locatePkgDir`
      accepts `*testing.T` (`locatePkgDir(t *testing.T) string`),
      calls `t.Helper()` as its first line (so failure attribution
      points at the caller, not the helper internals), and calls
      `t.Fatalf` on walk failure. Silent CWD fallback is removed.
      Both call sites (TC-CT-07, TC-CT-08) MUST be updated to pass
      `t`, and the pre-existing two-return error-handling pattern
      (`pkgDir, locErr := locatePkgDir()` + `if locErr != nil`) MUST
      be fully removed (no dangling `locErr` variable). A
      misconfigured test runner with unexpected CWD must now produce
      a clearly-attributed test failure pointing at the calling TC,
      not a false-positive PASS. REQ verified by TC-D-05.

- [ ] **REQ-6 — Document the asymmetric exit-code strictness across
      CT-03, CT-10, CT-11.** GIVEN CT-03 documents the deviation in a
      comment, CT-10 will (per REQ-4 above) document it identically,
      AND CT-11 asserts exact exit 1 (because `record-decision`'s
      validation flows through root's `RunE` and gets the
      `*usageError` wrap), WHEN this fix is applied, THEN a single
      `// TODO: wire root.SetFlagErrorFunc` comment appears once
      (in CT-03's existing comment block) with explicit cross-refs
      to CT-10 and CT-11 so a new contributor can find the
      explanation from any of the three. REQ verified by code
      inspection (TC-D-06).

- [ ] **REQ-7 — All 222 existing test functions preserved; total
      test-function count stays 222 after this fix; new
      `findJSONLogLine` helper is actually wired.** GIVEN the parent
      refactor's 222 tests (210 baseline + 12 contract), AND this
      fix only strengthens existing TCs without adding or removing
      **test functions** (the `findJSONLogLine` helper is a non-test
      function and does not count toward `^func Test`), WHEN
      `make learn-test` runs after the fix, THEN: (a) `grep -c
      '^func Test'` returns exactly 222; (b) `findJSONLogLine` is
      declared once AND called from BOTH TC-CT-04 and TC-CT-09
      (DRY motivation honored — see REQ-2 and TASK-2). REQ
      verified by TC-D-07 (count) and TC-D-08 (helper wiring).

## Test Plan

This fix is targeted enough that all verification happens via direct
inspection of the modified file (`cli_contract_test.go`) and the
modified parent spec, plus running the existing test suite to confirm
nothing regressed. We add 0 new test functions — only strengthen
existing TC bodies — so the total test count stays 222.

> **Quality-first lens applied:** the rigor here is in tightening
> existing assertions, not adding new tests. Each strengthened
> assertion is a regression guard for a specific spec promise that the
> current test fails to enforce. The TCs below are inspection-style
> (verify the test code reaches the right structure) rather than
> table-driven new tests.

### Direct verification TCs (TC-D-NN)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-D-01 | REQ-1 | edge | Parent spec no longer carries any "31"-as-count phrasing for the production .go file count; "32" appears in all 6 known locations with explanatory note. | (a) `grep -Pn 'is exactly \*\*31\*\*\|exactly 31 production\|file count == 31\|Production \.go count: exactly 31\|production .go file count at root == 31\|31 production \+' .specs/refactor-tools-learn-flat-layout.md` returns 0 hits. (b) Each of the 6 known anchors (REQ-1 prose, TC-CT-07 row, Design § Architecture Decisions, Files to Create count, TASK-7 VERIFY, Validation Criteria) carries `"32"` plus the explanatory phrase `"31 from the original ledger plus \`doc.go\`"` at least once. |
| TC-D-02 | REQ-2 | infra | TC-CT-04 body in `cli_contract_test.go` (a) does NOT call `t.Parallel()`; (b) calls `t.Setenv("LOG_FORMAT", "json")`; (c) scans stderr for ≥ 1 line where `json.Unmarshal` succeeds AND `obj["level"] == "ERROR"` AND `obj["msg"]` is a non-empty string starting with `"runtime: "`. The loose keyword union `{runtime, open, unable, no such}` is applied to the parsed `obj["msg"]` only — NOT as a fallback against raw stderr. | All 3 assertions present and active. Specifically: `grep -A 60 'func Test_TC_CT_04' cli_contract_test.go` does NOT contain `t.Parallel()`; same block contains `t.Setenv("LOG_FORMAT", "json")`, `json.Unmarshal`, and a literal check on `obj["level"]` or equivalent. |
| TC-D-03 | REQ-3 | validation | TC-CT-02 subtest body asserts `strings.TrimSpace(stderr.String()) == ""` (or equivalent) for each subcommand's `--help`. | Assertion present; `make learn-test` PASS |
| TC-D-04 | REQ-4 | edge | TC-CT-10 in `cli_contract_test.go` carries a deviation comment block matching TC-CT-03's pattern, referencing the same root cause by literal name (`cobra` short-circuit) AND the same follow-up by literal name (`SetFlagErrorFunc`). | `grep -B 10 'func Test_TC_CT_10' cli_contract_test.go` includes the literal strings `cobra` AND `SetFlagErrorFunc`; comment block is at least 5 lines. |
| TC-D-05 | REQ-5 | infra | `locatePkgDir` accepts `*testing.T`, calls `t.Helper()` as its first body line, calls `t.Fatalf` on walk failure, AND no silent CWD fallback remains. Both call sites (TC-CT-07, TC-CT-08) pass `t` and have no residual `locErr` variable. | (a) `grep -n 'func locatePkgDir(t \*testing.T)' cli_contract_test.go` returns 1 hit. (b) `grep -A 2 'func locatePkgDir' cli_contract_test.go \| grep -q 't.Helper()'`. (c) `grep -n 'Fallback: assume CWD\|return wd, nil' cli_contract_test.go` returns 0 hits. (d) `grep -n 'locErr' cli_contract_test.go` returns 0 hits. (e) `go build ./tools/learn/` compiles cleanly. |
| TC-D-06 | REQ-6 | edge | `cli_contract_test.go` has one `// TODO: wire root.SetFlagErrorFunc` comment that lives in TC-CT-03's comment block AND references both CT-10 and CT-11 explicitly. The comment placement is verified by anchoring to TC-CT-03's block, not by raw count. | `awk '/func Test_TC_CT_03/{f=1} f&&/TODO: wire root.SetFlagErrorFunc/{print}' cli_contract_test.go` returns ≥ 1 line; the matched comment text contains both `CT-10` and `CT-11` (or `TC-CT-10` and `TC-CT-11`) as literal substrings. |
| TC-D-07 | REQ-7 | edge | `make learn-test` after the fix shows exactly 222 test functions. | exit 0; `grep -h '^func Test' $(find tools/learn -name '*_test.go') \| wc -l` == 222 |
| TC-D-08 | REQ-7 | edge | The new `findJSONLogLine` helper introduced by TASK-2 is declared once and called from BOTH TC-CT-04 and TC-CT-09 (DRY motivation honored). | (a) `grep -cE '^func findJSONLogLine\b' cli_contract_test.go` returns 1. (b) `grep -c 'findJSONLogLine(' cli_contract_test.go` returns ≥ 3 (1 declaration + 1 call inside TC-CT-04 + 1 call inside TC-CT-09). |

### Use Case / E2E / Smoke Tests

Not applicable — this fix-spec touches only test-code rigor and a
docs-only patch to the parent spec. No use case logic, no HTTP
endpoint, no migration, no smoke surface.

### Mutability and rigor check

- 7 TCs (TC-D-01..07). All are inspection-style edge/infra/validation
  assertions on the modified files. 0 happy-path TCs.
- Error/edge ratio: 7/0. By definition outnumbers happy paths (the
  fix has no happy path of its own — it strengthens existing tests).
- Quality-first lens: each TC corresponds to a specific finding from
  `/spec-review`'s test-quality audit. None are "polish" — all are
  rigor (closing assertion gaps that allowed silent regressions).

## Design

### Approach

Single-file Go change in `tools/learn/cli_contract_test.go` plus a
search-and-replace pass on the parent spec for the 31→32
reconciliation. No new test functions; no production code touched;
no Makefile / hook / skill / doc edits beyond the parent spec.

### Files to Modify

- `tools/learn/cli_contract_test.go` — tighten TC-CT-02, TC-CT-04,
  TC-CT-10; harden `locatePkgDir`; add cross-ref `TODO` comment in
  TC-CT-03's block. **All edits within existing function bodies; no
  new test functions; no removed test functions.**
- `.specs/refactor-tools-learn-flat-layout.md` — search-and-replace
  31→32 in 6 known locations (REQ-1 prose, Design § Architecture
  Decisions, TC-CT-07 row, TASK-7 VERIFY, Validation Criteria, plus
  any incidental "31" reference detected during the edit). Add a
  one-line explanatory note: "32 production `.go` files (31 from
  the original ledger plus `doc.go`)".

### Files to Create

None.

### Files to Delete

None.

### Specific edit targets

**1. Parent spec count reconciliation (REQ-1).** All replacements are
**content-anchored** (use `grep` patterns, not line numbers — line
numbers drift as TASK-1 itself edits the file). The 6 anchor patterns
are:

- REQ-1 prose: `is exactly **31**` → `is exactly **32** (31 from the
  original ledger plus \`doc.go\`)`
- TC-CT-07 row (in the CT TC table): `is exactly 31 | rg exit 1
  (no matches); production \`.go\` file count == 31` → `is exactly
  32 | rg exit 1 (no matches); production \`.go\` file count == 32`
- Design § Architecture Decisions: `Production .go count: exactly 31`
  → `Production .go count: exactly 32`
- Files to Create: `31 production + ~16 test` → `32 production
  + ~16 test`
- TASK-7 VERIFY: `production .go file count at root == 31` → `== 32`
- Validation Criteria: `(excluding \`_test.go\`) is exactly 31` →
  `(excluding \`_test.go\`) is exactly 32`

> **Note:** at fix-spec authoring time these anchors mapped to lines
> 70, 309, 440, 754, 1028, 1161 respectively. Line numbers are
> approximate; ralph-loop MUST use the content patterns above (not
> the line numbers) since TASK-1's own edits will shift them.

Add one explanatory note next to REQ-1's prose: `(31 from the
original ledger plus \`doc.go\` — see fix-spec ac0b1ad → this
spec)`.

**Counter-anchor for false positives (TC-D-01):** the `/spec-review`
report section (under `## Review Results`) already contains the
phrase "exactly 32" in its findings. TC-D-01 must NOT count those
hits — that's why the grep in TC-D-01 anchors to the SPECIFIC old
phrases being replaced, not to "32" generically.

**2. TC-CT-02 stderr-empty assertion (REQ-3):**
- Inside the subtest body in TC-CT-02 (cli_contract_test.go:78–94),
  after the existing `if strings.TrimSpace(stdout.String()) == ""`
  check, add a symmetric `if s := strings.TrimSpace(stderr.String());
  s != "" { t.Errorf("%s --help: expected empty stderr, got %q",
  name, s) }` assertion.

**3. TC-CT-04 level=ERROR + JSON-decodable assertions (REQ-2):**
- **Remove `t.Parallel()` from TC-CT-04's body** (currently at
  cli_contract_test.go:122 area). Add comment `// NOT parallel:
  mutates process env via t.Setenv` matching TC-CT-09's pattern.
  This is non-optional — `t.Setenv` panics if `t.Parallel` was
  called.
- Refactor TC-CT-04 to call `t.Setenv("LOG_FORMAT", "json")` at the
  start.
- After invoking `RunCmd(...)`, scan stderr line-by-line. For each
  line that starts with `{`, attempt `json.Unmarshal` into
  `map[string]any`. If any line parses AND has `obj["level"] ==
  "ERROR"` AND `obj["msg"]` is a non-empty string starting with
  `"runtime: "`, mark the assertion satisfied.
- The existing loose keyword union (`runtime, open, unable, no such`)
  is a check on the parsed `obj["msg"]` value only — NOT a fallback
  against raw stderr. This forces the test to fail if slog wiring
  drops and falls back to `fmt.Fprintf` (which would emit a raw
  string that fails `json.Unmarshal`).
- **MANDATORY (not optional): extract the JSON-line-scanner into
  a shared helper** at the bottom of the file with signature
  `func findJSONLogLine(stderr []byte, level string) (map[string]any,
  bool)`. **TC-CT-09 MUST also be refactored** to use this helper
  (without that step the helper has one call site and triggers
  unused-via-DRY-motivation smell; the test-reviewer's audit
  explicitly flagged this as SHOULD FIX). When `findJSONLogLine`
  fails to find a matching line, the caller's `t.Errorf` MUST
  include the full stderr captured (`stderr.String()`) so the
  diagnostic shows what was actually emitted.

**4. TC-CT-10 deviation comment + REQ-6 cross-ref TODO (REQ-4, REQ-6):**
- Above TC-CT-10's function body, add a comment block of at least
  5 lines that MUST contain the literal strings `cobra` AND
  `SetFlagErrorFunc` (so TC-D-04's grep can verify content, not
  just length). Match the explanatory shape of TC-CT-03's existing
  block.
- In TC-CT-03's existing comment block (around lines 96–103), add a
  `// TODO: wire root.SetFlagErrorFunc` line that explicitly
  references both `TC-CT-10` and `TC-CT-11` by name, so a contributor
  searching from any of the three TCs lands on a single source of
  truth. Suggested wording: `// TODO: wire root.SetFlagErrorFunc to
  wrap cobra flag/command errors into *usageError → tightens TC-CT-03,
  TC-CT-10, and TC-CT-11 to exit 1 across the board (currently CT-03
  and CT-10 accept exit 1 OR 2; CT-11 already asserts exact 1 because
  its error path flows through root.RunE).`

**5. `locatePkgDir` fail-loud (REQ-5):**
- Change the helper signature to `locatePkgDir(t *testing.T) string`
  (T-aware variant chosen — all callers are tests and always have
  `t` in scope; error-returning variant would force every caller to
  add a redundant `if err != nil` branch).
- First line of the new body MUST be `t.Helper()` so failure
  attribution points at the calling TC, not the helper internals.
- On walk failure (after the 8-level loop exhausts), replace the
  current `return wd, nil` with `t.Fatalf("locatePkgDir: could not
  find go.mod ascending from %q after 8 levels", wd)`. Delete the
  `// Fallback: assume CWD.` comment line entirely.
- Update both call sites (TC-CT-07, TC-CT-08): change
  `pkgDir, locErr := locatePkgDir()` + `if locErr != nil { t.Fatalf(...) }`
  to a single `pkgDir := locatePkgDir(t)`. The `locErr` variable
  must be fully removed (no dangling assignments).

### Dependencies

No new Go modules. No changes to `go.mod`. The fix uses only
`encoding/json` (already imported by `cli_contract_test.go`),
`strings` (already imported), and `testing` (already imported).

## Tasks

- [x] **TASK-1: Reconcile parent-spec 31→32 count and add explanatory note (REQ-1).**
  - Search-and-replace the 6 known locations in
    `.specs/refactor-tools-learn-flat-layout.md`.
  - Add the explanatory note "(31 from the original ledger plus
    `doc.go`)" to the REQ-1 prose so the reason is documented.
  - Verify with `grep -n "exactly 31\|file count == 31" .specs/
    refactor-tools-learn-flat-layout.md` returning 0 hits.
  - VERIFY: docs-only change, no build impact.
  - files: `.specs/refactor-tools-learn-flat-layout.md`
  - tests: TC-D-01

- [x] **TASK-2: Strengthen TC-CT-04 with level=ERROR + JSON-decodable
      assertions (REQ-2, REQ-7).**
  - Remove `t.Parallel()` from TC-CT-04's body; add `// NOT
    parallel: mutates process env via t.Setenv` comment (matches
    TC-CT-09's pattern).
  - Refactor TC-CT-04 body in `tools/learn/cli_contract_test.go` per
    the Design § specific edit targets section above.
  - Extract `findJSONLogLine(stderr []byte, level string)
    (map[string]any, bool)` helper at the bottom of the file.
  - **MANDATORY: refactor TC-CT-09 to use the same helper.** Without
    this, the helper has one call site and triggers DRY-smell.
  - VERIFY:
    - `make learn-build && make learn-lint && make learn-test` green;
      total test-function count still 222.
    - TC-CT-04 alone passes via `go test -run Test_TC_CT_04 ./tools/learn/`.
    - TC-CT-09 alone passes via `go test -run Test_TC_CT_09 ./tools/learn/`.
    - `grep -cE '^func findJSONLogLine\b' tools/learn/cli_contract_test.go`
      returns 1.
    - `grep -c 'findJSONLogLine(' tools/learn/cli_contract_test.go`
      returns ≥ 3.
    - `grep -A 60 'func Test_TC_CT_04' tools/learn/cli_contract_test.go`
      does NOT contain `t.Parallel()`.
  - files: `tools/learn/cli_contract_test.go`
  - tests: TC-D-02, TC-D-07, TC-D-08

- [x] **TASK-3: Add stderr-empty assertion to TC-CT-02 (REQ-3).**
  - Inside the subtest body, add the symmetric stderr-trimmed-empty
    assertion as described in the Design section.
  - VERIFY: `make learn-test` green; TC-CT-02 still passes (16
    subcommand subtests).
  - files: `tools/learn/cli_contract_test.go`
  - tests: TC-D-03, TC-D-07

- [x] **TASK-4: Add TC-CT-10 deviation comment + TC-CT-03 cross-ref
      TODO (REQ-4, REQ-6).**
  - Above TC-CT-10, add the deviation comment block matching TC-CT-03's
    pattern, referencing CT-03 and `root.SetFlagErrorFunc`.
  - Expand TC-CT-03's existing comment to include the explicit
    cross-ref to CT-10 and CT-11 and the follow-up TODO.
  - VERIFY: grep for the comment patterns succeeds; `make learn-lint`
    green (linter accepts the new comments).
  - files: `tools/learn/cli_contract_test.go`
  - tests: TC-D-04, TC-D-06

- [x] **TASK-5: Harden `locatePkgDir` to fail loud (REQ-5).**
  - Refactor `locatePkgDir` to T-aware signature with `t.Helper()`
    first line + `t.Fatalf` on walk failure, per Design § item 5.
  - Update both callers (TC-CT-07, TC-CT-08) to pass `t`; remove
    `locErr` variable and the surrounding `if locErr != nil` branch
    in each.
  - Remove the `// Fallback: assume CWD.` line and the silent
    `return wd, nil`.
  - VERIFY:
    - `go build ./...` from `tools/learn/` compiles cleanly (no
      "unused variable locErr" or "wrong arity for locatePkgDir").
    - `make learn-test` green; TC-CT-07 and TC-CT-08 still pass.
    - `grep -n 'Fallback: assume CWD\|return wd, nil' tools/learn/cli_contract_test.go`
      returns 0 hits.
    - `grep -n 'locErr' tools/learn/cli_contract_test.go` returns 0 hits.
    - `grep -A 2 'func locatePkgDir' tools/learn/cli_contract_test.go`
      shows `t.Helper()` as the function body's first statement.
  - files: `tools/learn/cli_contract_test.go`
  - tests: TC-D-05, TC-D-07

- [x] **TASK-SMOKE: Full validation gate.**
  - Run `make learn-build && make learn-lint && make learn-test`
    from `/Users/marcelojr/Development/Workspace/gopherplate/`.
  - Confirm test count is exactly 222 (no test added, no test
    removed).
  - Run `go test -v -run "Test_TC_CT" ./tools/learn/` and confirm all
    12 contract tests pass.
  - Run `make learn-smoke` and confirm exit 0.
  - files: (none — execution only)
  - depends: TASK-1, TASK-2, TASK-3, TASK-4, TASK-5
  - tests: TC-D-07

## Parallel Batches

All 5 implementation tasks (TASK-1..5) touch the same file
(`cli_contract_test.go`) except TASK-1 which touches the parent
spec. **TASK-2, TASK-3, TASK-4, TASK-5 are shared-mutative on
`cli_contract_test.go` — they must serialize.** TASK-1 is
independent (different file) and could parallelize with any of the
others.

For execution simplicity and because the scope is tiny (~30 LOC total
delta in one .go file), serial execution is preferred:

```
Batch 1: [TASK-1]                       — parent-spec docs reconciliation (independent)
Batch 2: [TASK-2]                       — TC-CT-04 strengthening + shared helper extraction
Batch 3: [TASK-3]                       — TC-CT-02 stderr-empty
Batch 4: [TASK-4]                       — TC-CT-10 deviation comment + TC-CT-03 cross-ref
Batch 5: [TASK-5]                       — locatePkgDir hardening
Batch 6: [TASK-SMOKE]                   — final gate
```

A parallel variant (TASK-1 + TASK-5 in same batch since they touch
different files) is possible but not worth the worktree overhead for
a 30-LOC change.

## Validation Criteria

- [ ] `make learn-build` PASS
- [ ] `make learn-lint` 0 issues
- [ ] `make learn-test` PASS with exactly **222** test functions (no
      count change)
- [ ] `make learn-smoke` PASS
- [ ] Parent-spec count reconciliation (REQ-1, TC-D-01) — the 6 old
      phrasings are absent and the 6 anchors all show the new "32"
      form. Use the exact grep from TC-D-01.
- [ ] `grep -n "Fallback: assume CWD" tools/learn/cli_contract_test.go`
      returns 0 hits.
- [ ] `grep -n "locErr" tools/learn/cli_contract_test.go` returns 0
      hits.
- [ ] `grep -n "TODO: wire root.SetFlagErrorFunc"
      tools/learn/cli_contract_test.go` returns ≥ 1 hit inside
      TC-CT-03's comment block (per TC-D-06's anchored awk).
- [ ] TC-CT-04 calls `t.Setenv("LOG_FORMAT", "json")` AND does NOT
      call `t.Parallel()` (TC-D-02).
- [ ] TC-CT-02 asserts trimmed stderr equals empty (TC-D-03).
- [ ] TC-CT-10's deviation comment block contains literal `cobra`
      AND literal `SetFlagErrorFunc` (TC-D-04).
- [ ] `findJSONLogLine` declared once, called ≥ 3 times (1 decl + 1
      from TC-CT-04 + 1 from TC-CT-09 — TC-D-08).
- [ ] `locatePkgDir` body starts with `t.Helper()` (TC-D-05).

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
