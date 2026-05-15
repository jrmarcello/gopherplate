# Spec: fix cobra-surfaced error exit codes (1 instead of 2)

## Status: DONE

## Context

The parent spec `.specs/refactor-tools-learn-flat-layout.md` (commit
`ac020ee`) and its rigor follow-up `.specs/fix-refactor-tools-learn-
contract-test-rigor.md` (commit `d808a59`) both documented a known
production deviation: cobra-surfaced errors (unknown command, unknown
flag, missing required flag) currently exit **2** instead of the
spec-mandated **1**.

Per parent REQ-4 ("WHEN a usage error occurs (unknown subcommand,
invalid flag, missing required argument, or other validation failure),
THEN exit code 1"), all three should be **usage errors** (`*usageError`
→ exit 1). The actual behavior:

- `learn unknownsubcmd` → cobra returns its own internal error from
  `Command.Find()`, which short-circuits BEFORE reaching root's `RunE`
  (the only path currently wrapping into `*usageError`). Falls through
  `exitCode`'s fallback branch → exit **2**.
- `learn recall --unknown-flag x` → cobra's flag parser emits an
  unwrapped error, same fallthrough → exit **2**.
- `learn record-decision` (missing required positional) → the subcommand's
  own validation runs through `root.RunE` and DOES wrap into
  `*usageError` → exit **1**. (CT-11 already asserts this; only CT-03
  and CT-10 have the deviation.)

The contract tests `TC-CT-03` (unknown subcommand) and `TC-CT-10`
(unknown flag) currently accept exit 1 OR 2 with inline DEVIATION
comments referencing this exact follow-up. `TC-CT-11` (missing required)
already asserts exact exit 1.

This spec wires the two cobra hooks needed to deliver REQ-4's
exit-code contract uniformly, then tightens the three contract tests
to assert exact exit 1 (removing the OR-2 escape hatches and the
DEVIATION comments).

Scope:

1. **Production code:** add a hook in `NewRootCmd` that wraps cobra
   flag-parse errors into `*usageError` via `SetFlagErrorFunc`. Add a
   wrap step in `RunCmd` that catches the unknown-command path (which
   has no public cobra hook) and converts it to `*usageError`.
2. **Tests:** tighten TC-CT-03 and TC-CT-10 to `code == 1`; remove the
   DEVIATION comments and the cross-ref TODO in TC-CT-03's block
   (since the follow-up is now done). TC-CT-11 stays as-is (already
   asserts 1).

No new test functions. No changes outside `tools/learn/`. Test count
stays at 222.

References:
- Parent: `.specs/refactor-tools-learn-flat-layout.md` (REQ-4
  exit-code semantics; deviation documented in Review Results +
  Execution Log).
- Predecessor fix: `.specs/fix-refactor-tools-learn-contract-test-
  rigor.md` (added the cross-ref TODO in TC-CT-03's block).
- Cobra docs:
  https://pkg.go.dev/github.com/spf13/cobra#Command.SetFlagErrorFunc
- Predecessor commits: `ac020ee` (structural refactor), `ac0b1ad`
  (/spec-review report), `d808a59` (contract-test rigor).

## Requirements

- [ ] **REQ-1 — Unknown-flag errors exit 1.** GIVEN any invocation
      `learn <subcmd> --unknown-flag <val>`, WHEN cobra's flag
      parser rejects the flag, THEN the returned error is wrapped
      into `*usageError` and `exitCode` returns 1. The stderr
      message still carries the substring `"unknown flag"` or
      `"unknown shorthand"` so the existing assertion in TC-CT-10
      holds. Implementation: `root.SetFlagErrorFunc(func(cmd,
      err) error { return usagef("%s", err) })` in `NewRootCmd`.
      REQ verified by TC-CT-10 (tightened).

- [ ] **REQ-2 — Missing required flag errors exit 1.** GIVEN any
      invocation that omits a `cobra.MarkFlagRequired`-marked flag,
      WHEN cobra emits the `required flag(s) ... not set` error,
      THEN the error is detected inside `RunCmd` (after
      `root.Execute()` returns) by `wrapCobraError` and wrapped into
      `*usageError`. **Important correction (round-1 review):**
      `SetFlagErrorFunc` does NOT fire for required-flag validation —
      cobra's `Command.ValidateRequiredFlags` runs AFTER flag parse
      and returns the error directly from `Execute()`. The fix
      therefore extends `wrapCobraError` to ALSO match the substring
      `"required flag"` (in addition to `"unknown command"`). REQ
      verified by TC-D-12: a NEW synthetic test in `cli_test.go` that
      builds a `cobra.Command` with `MarkFlagRequired`, invokes it
      without the flag, and asserts `code == 1`. This is the one
      exception to the "no new test functions" goal (REQ-7 amended
      accordingly): one synthetic structural test for REQ-2 is
      required because no current subcommand uses `MarkFlagRequired`,
      and "static reasoning" alone leaves the hook mutation-blind per
      the test-reviewer audit.

- [ ] **REQ-3 — Unknown-command errors exit 1.** GIVEN any invocation
      `learn <unknownsubcmd>`, WHEN cobra's `Command.Find()` returns
      its internal unknown-command error AND the error short-circuits
      BEFORE reaching root's `RunE`, THEN the error is detected
      inside `RunCmd` (after `root.Execute()` returns) by
      `wrapCobraError` and wrapped into `*usageError`. Detection
      criterion: the error is neither `*usageError` nor
      `*runtimeError` (verified via `errors.As` to walk the chain),
      AND its message contains the substring `"unknown command"`.
      **Correction (round-1 review):** the previous draft listed
      three patterns including `"unknown subcommand"` and `"Error:
      unknown command"` — both are removed. Cobra v1 emits exactly
      `unknown command "X" for "learn"` (no `"unknown subcommand"`
      variant), and `SilenceErrors: true` is already set in
      `NewRootCmd` so cobra never prepends `"Error: "` to the error
      string returned from `Execute()`. The single
      `"unknown command"` substring is the only reachable pattern.
      REQ verified by TC-CT-03 (tightened to assert exact `code == 1`).

- [ ] **REQ-4 — `*runtimeError` exit code unchanged.** GIVEN any
      invocation that triggers a `*runtimeError` (DB IO failure,
      embed parse error, etc.), WHEN the error bubbles up through
      `exitCode`, THEN exit code stays **2**. The wrapping logic
      added in REQ-1..3 MUST NOT misclassify a real runtime error
      as a usage error. Detection in REQ-3 explicitly checks the
      error is not already typed before string-matching the
      cobra-internal shape. REQ verified by TC-CT-04 (which exercises
      the runtime-error path and asserts exit 2 — must continue
      passing unchanged).

- [ ] **REQ-5 — Other typed errors flow through unchanged.** GIVEN
      a subcommand that already returns `*usageError` from its own
      `RunE` (e.g. `record-decision` for missing required positional),
      WHEN the error bubbles up, THEN no double-wrap happens — the
      existing `*usageError` is preserved by-type. The
      `SetFlagErrorFunc` hook only fires on FLAG-parse errors (which
      are not yet typed); the unknown-command wrap in `RunCmd` only
      fires when the error is neither `*usageError` nor
      `*runtimeError`. REQ verified by TC-CT-11 (unchanged behavior,
      still exit 1 via the subcommand's own RunE wrap).

- [ ] **REQ-6 — Stderr messages preserved.** GIVEN the wrapping
      operations from REQ-1 / REQ-3, WHEN stderr is captured, THEN
      every assertion currently in TC-CT-03, TC-CT-10, TC-CT-11
      about stderr CONTENT (e.g. `"unknown command"`, `"unknown flag"`)
      continues to pass. The wrap must preserve the original error
      message verbatim inside the `*usageError`'s wrapped chain so
      `errors.Unwrap` + `Error()` continue to emit the cobra-internal
      message. Implementation: `usagef("%s", err)` rather than a
      template that obscures the cobra wording. REQ verified by the
      three CT tests' content assertions (unchanged).

- [ ] **REQ-7 — Tests tightened; deviation comments removed; test
      count grows by 1 for REQ-2 coverage.** GIVEN this fix wires
      the cobra hooks, WHEN TC-CT-03 and TC-CT-10 are updated, THEN:
      (a) both assert `code == 1` (not `1 OR 2`); (b) both lose the
      DEVIATION comment paragraphs that explained the deferral; (c)
      TC-CT-03's `// TODO: wire root.SetFlagErrorFunc` cross-ref TODO
      is REMOVED (the TODO is now done). TC-CT-11 stays unchanged.
      Test count grows from 222 to **223** because REQ-2's
      `MarkFlagRequired` path has no current subcommand exercising it
      — one new synthetic test `Test_CobraRequiredFlag_exits_one` is
      added in `cli_test.go`. The DEVIATION comment in TC-CT-09
      (about `--db` flag inheritance — UNRELATED to the cobra-exit
      issue) MUST be left untouched; only the DEVIATION blocks in
      TC-CT-03 and TC-CT-10 are removed. REQ verified by anchored
      grep (per TC-D-04 below) + TC-D-NN.

## Test Plan

This spec touches CLI plumbing that's already covered by existing
contract tests. Strategy: tighten the 3 affected TCs, validate that
all other 219 tests stay green, add inspection-style TC-D verifications
for the cobra-wire correctness and the deviation-comment removal.

### Direct verification TCs (TC-D-NN)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-D-01 | REQ-1, REQ-6, REQ-7 | edge | TC-CT-10 (`learn recall --unknown-flag x`) asserts exact `code == 1` (no `\|\| code == 2`); stderr still contains `"unknown flag"` or `"unknown shorthand"`. | Test passes; `grep -A 30 'func Test_TC_CT_10' tools/learn/cli_contract_test.go` shows `code != 1` assertion (not `code != 1 && code != 2`). |
| TC-D-02 | REQ-3, REQ-6, REQ-7 | edge | TC-CT-03 (`learn unknownsubcmd`) asserts exact `code == 1`; stderr still contains `"unknown command"`. | Test passes; `grep -A 25 'func Test_TC_CT_03' tools/learn/cli_contract_test.go` shows `code != 1` assertion. |
| TC-D-03 | REQ-5, REQ-7 | edge | TC-CT-11 unchanged — still asserts `code == 1`. | `grep -A 15 'func Test_TC_CT_11' tools/learn/cli_contract_test.go` shows `code != 1` unchanged. |
| TC-D-04 | REQ-7 | edge | DEVIATION comment blocks above TC-CT-03 and TC-CT-10 are removed. The `// TODO: wire root.SetFlagErrorFunc` block in TC-CT-03 is also gone (follow-up is now done). The UNRELATED DEVIATION comment in TC-CT-09 (about `--db` flag inheritance) is preserved untouched. | (a) `grep -B 5 'func Test_TC_CT_03' tools/learn/cli_contract_test.go \| grep -cE 'DEVIATION\|TODO: wire root\.SetFlagErrorFunc'` returns 0 (CT-03 cleaned). (b) `grep -B 5 'func Test_TC_CT_10' tools/learn/cli_contract_test.go \| grep -c 'DEVIATION'` returns 0 (CT-10 cleaned). (c) `grep -c 'DEVIATION' tools/learn/cli_contract_test.go` returns ≥ 1 (TC-CT-09's unrelated DEVIATION survives). (d) `grep -c 'TODO: wire root\.SetFlagErrorFunc' tools/learn/cli_contract_test.go` returns 0 (the cross-ref TODO is gone). |
| TC-D-05 | REQ-1, REQ-2 | edge | `NewRootCmd` in `cli.go` calls `root.SetFlagErrorFunc` with a function that returns a `*usageError`. | `grep -A 3 'SetFlagErrorFunc' tools/learn/cli.go` shows the function body returns `usagef(...)` or wraps via `&usageError{...}`. |
| TC-D-06 | REQ-3 | edge | `RunCmd` in `cli.go` inspects the error from `root.Execute()` for the unknown-command shape and wraps into `*usageError` BEFORE calling `exitCode`. The guard against double-wrap uses the correct Go idiom: `var ue *usageError; var re *runtimeError; if errors.As(err, &ue) \|\| errors.As(err, &re) { return err }` — `errors.As` (not direct type-assert) is deliberate because callers may wrap typed errors via `fmt.Errorf("...: %w", typedErr)`. The string-match for `"unknown command"` runs ONLY after the type check passes. | grep shows the new wrap logic; first lines of `wrapCobraError` are the `var ue *usageError; var re *runtimeError` declarations followed by the `errors.As` guards. |
| TC-D-07 | REQ-4 | infra | TC-CT-04 (runtime-error path) STILL asserts exit 2 — the new wrap logic must NOT misclassify a real `*runtimeError`. | `go test -run Test_TC_CT_04 ./tools/learn/` exits 0. |
| TC-D-08 | REQ-7 | edge | Total test count is exactly 223 (222 baseline + 1 new `Test_CobraRequiredFlag_exits_one` for REQ-2 coverage). No existing test removed. | `grep -h '^func Test' $(find tools/learn -name '*_test.go' -not -path '*/bin/*') \| wc -l` == 223. |
| TC-D-09 | REQ-1, REQ-3, REQ-5 | edge | All 12 contract TCs pass after the wiring change. | `go test -run "Test_TC_CT" ./tools/learn/` exits 0. |
| TC-D-10 | REQ-1, REQ-2, REQ-3, REQ-4, REQ-5 | edge | Full `make learn-test` suite passes with 222/222 tests. Build/lint green. | `make learn-build && make learn-lint && make learn-test` all exit 0. |
| TC-D-11 | REQ-2 | edge | NEW synthetic test `Test_CobraRequiredFlag_exits_one` in `cli_test.go`: builds a `cobra.Command` with `MarkFlagRequired`, calls it without the flag via `RunCmd`, asserts (a) exit code == 1; (b) stderr contains `"required flag"`. Test is parallel-safe (no env mutation). | Test PASSES; `grep -n 'Test_CobraRequiredFlag_exits_one' tools/learn/cli_test.go` returns 1 hit; total `^func Test` count is exactly 223. |
| TC-D-12 | REQ-6 | edge | Stderr content for TC-CT-03 / TC-CT-10 unchanged post-wire — substring assertions in the tightened tests (`"unknown command"`, `"unknown flag"`/`"unknown shorthand"`) still pass. This is implicitly verified by TC-D-01 and TC-D-02 (which both require the tightened tests to PASS, and PASS requires the substring assertions to still hold). No separate verification step. | Subsumed by TC-D-01 + TC-D-02 passing. |

### Use Case / E2E / Smoke Tests

Not applicable — this is CLI plumbing in `tools/learn/`, no HTTP
endpoint, no migration, no service-level integration.

### Mutability and rigor check

- 12 TCs, all `edge` or `infra`. Zero happy-path. Appropriate for a
  fix-spec — happy path is the unchanged 219 existing tests plus
  TC-CT-04 (runtime-error path which already exits 2).
- Convergent coverage: TC-D-01..03 verify the test tightening;
  TC-D-04 verifies anchored comment removal (preserves TC-CT-09's
  unrelated DEVIATION); TC-D-05..06 verify production wire; TC-D-07
  verifies non-regression of the runtime path; TC-D-08..10 verify
  overall suite integrity; TC-D-11 NEW synthetic exercises REQ-2's
  `MarkFlagRequired` path; TC-D-12 documents that REQ-6 (stderr
  content) is subsumed by TC-D-01 + TC-D-02.
- Rigor check: error TCs outnumber happy 12:0. Spec target met.

## Design

### Approach

Two small additions to `tools/learn/cli.go`:

1. **`SetFlagErrorFunc` hook in `NewRootCmd`**: wraps any
   cobra-emitted flag-parse error (unknown flag, missing required
   flag, bad flag value) into `*usageError`. This is the cleanest
   path because cobra exposes the hook explicitly.
2. **Unknown-command wrap in `RunCmd`**: after `root.Execute()`
   returns an error, check if it's neither `*usageError` nor
   `*runtimeError` AND its message matches cobra's unknown-command
   shape. If so, wrap into `*usageError`. The double-typed-error
   guard prevents misclassifying a real `*runtimeError` whose
   wrapped chain happens to mention "command" somewhere.

Both pathways converge on the existing `*usageError` type, so
`exitCode` returns 1 uniformly. The stderr emission in `RunCmd`
still uses `fmt.Fprintln(stderr, "Error:", execErr)` so message
content stays unchanged for tests and operators.

### Files to Modify

- `tools/learn/cli.go` — add `SetFlagErrorFunc` hook to `NewRootCmd`;
  add a small helper `wrapCobraError(err error) error` called in
  `RunCmd` before `exitCode`.
- `tools/learn/cli_contract_test.go` — tighten TC-CT-03 and TC-CT-10
  exit-code assertions; remove DEVIATION comment blocks; remove the
  `// TODO: wire root.SetFlagErrorFunc` block.

### Files to Create

None.

### Files to Delete

None.

### Specific edit targets

**1. `cli.go` — `NewRootCmd` (REQ-1, REQ-2):**

Inside the returned `&cobra.Command{...}` literal, AFTER the
`SilenceErrors: true,` line and the closing of the literal but BEFORE
the function returns the value, add:

```go
root := &cobra.Command{ /* existing fields */ }
root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
    // Wrap cobra's flag-parse errors (unknown flag, missing required
    // flag, bad flag value) into *usageError so exitCode returns 1
    // uniformly. Without this hook, the unwrapped error falls through
    // exitCode's fallback to exit 2, violating REQ-4.
    return usagef("%s", err)
})
return root
```

This requires refactoring `NewRootCmd` from a single `return
&cobra.Command{...}` into `root := &cobra.Command{...}; root.Set...;
return root`.

**2. `cli.go` — `RunCmd` (REQ-2, REQ-3, REQ-4, REQ-5):**

Add a small helper:

```go
// wrapCobraError converts cobra's untyped errors (unknown command +
// missing-required-flag) into *usageError so exitCode returns 1
// (matching REQ-4 of the parent refactor spec). Errors already typed
// as *usageError or *runtimeError pass through unchanged — this
// prevents misclassifying a runtime error whose wrapped chain happens
// to mention "command" or "flag".
//
// errors.As is deliberate (not direct type-assert) because callers
// may wrap typed errors via fmt.Errorf("...: %w", typedErr) — we want
// to detect the typed error anywhere in the wrap chain, not just at
// the top.
//
// String-match patterns are taken from cobra v1's internal messages
// and MUST be re-verified on any major cobra version upgrade
// (Command.Find emits "unknown command \"X\" for \"Y\""; required-flag
// validation emits "required flag(s) \"X\", ... not set").
func wrapCobraError(err error) error {
    if err == nil {
        return nil
    }
    var ue *usageError
    var re *runtimeError
    if errors.As(err, &ue) || errors.As(err, &re) {
        return err
    }
    msg := err.Error()
    if strings.Contains(msg, "unknown command") ||
        strings.Contains(msg, "required flag") {
        return usagef("%s", err)
    }
    return err
}
```

Modify `RunCmd` to call `wrapCobraError` before `exitCode`:

```go
if execErr := root.Execute(); execErr != nil {
    execErr = wrapCobraError(execErr)
    if stderr != nil {
        // After the wrap, execErr is the *usageError (or original
        // typed error if no wrap occurred). Either way, .Error()
        // returns the verbatim cobra message (see learnerr.go:
        // splitWrapped uses fmt.Errorf("%s", err) which is non-%w, so
        // usageError.Msg == err.Error() and usageError.Err == nil →
        // (*usageError).Error() returns Msg verbatim, no prefix).
        _, _ = fmt.Fprintln(stderr, "Error:", execErr)
    }
    return exitCode(execErr)
}
```

**Confirmed `(*usageError).Error()` semantics (was previously
`[NEEDS CLARIFICATION]`):** read `learnerr.go` — `usagef("%s", err)`
calls `splitWrapped("%s", err)` which calls `fmt.Errorf("%s", err)`
(NOT `%w`). Result: `splitWrapped` returns `Msg = err.Error()`, `Err =
nil`. Therefore `(*usageError).Error()` returns `Msg` verbatim with NO
prefix. The cobra message substrings (`"unknown command"`, `"unknown
flag"`, `"required flag"`) are preserved byte-for-byte through the
wrap and the `fmt.Fprintln(stderr, "Error:", execErr)` emission.
TASK-1 re-reads `learnerr.go` to record this chain in the Execution
Log as a sanity check before TASK-2 starts.

**3. `cli_contract_test.go` — TC-CT-10 (REQ-1, REQ-7):**

Remove the DEVIATION block in the comment above
`Test_TC_CT_10_unknown_flag`. Replace with a single-line description.
Tighten the assertion:

```go
// TC-CT-10: unknown flag yields a usage error (exit 1).
func Test_TC_CT_10_unknown_flag(t *testing.T) {
    t.Parallel()
    var stdout, stderr bytes.Buffer
    root := buildRoot(t)
    code := RunCmd(root, []string{"recall", "--unknown-flag", "x"}, &stdout, &stderr)
    if code != 1 {
        t.Fatalf("expected exit 1 for unknown flag, got %d (stderr=%q)", code, stderr.String())
    }
    combined := stderr.String()
    if !strings.Contains(combined, "unknown flag") && !strings.Contains(combined, "unknown shorthand") {
        t.Errorf("expected stderr to contain \"unknown flag\" or \"unknown shorthand\", got: %q", combined)
    }
}
```

**4. `cli_contract_test.go` — TC-CT-03 (REQ-3, REQ-7):**

Remove the long DEVIATION block AND the `// TODO: wire
root.SetFlagErrorFunc` block. Tighten the assertion:

```go
// TC-CT-03: unknown subcommand yields a usage error (exit 1).
func Test_TC_CT_03_unknown_subcommand_exits_one(t *testing.T) {
    t.Parallel()
    var stdout, stderr bytes.Buffer
    root := buildRoot(t)
    code := RunCmd(root, []string{"unknownsubcmd"}, &stdout, &stderr)
    if code != 1 {
        t.Fatalf("expected exit 1 for unknown subcommand, got %d (stderr=%q)", code, stderr.String())
    }
    if !strings.Contains(stderr.String(), "unknown command") {
        t.Errorf("expected stderr to contain \"unknown command\", got: %q", stderr.String())
    }
}
```

### Dependencies

No new Go modules. The wrap helper uses `errors` and `strings`. Current
`cli.go` imports are only `"fmt"`, `"io"`, and `"github.com/spf13/cobra"`
— **both `"errors"` and `"strings"` MUST be added** in TASK-2.

## Tasks

- [x] **TASK-1: Read `usageError.Error()` to verify message-preservation contract (REQ-6 prereq).**
  - Read `tools/learn/learnerr.go` to confirm
    `(*usageError).Error()` returns the wrapped error's message
    intact (no prefix that would break `strings.Contains(stderr,
    "unknown command")` or `strings.Contains(stderr, "unknown
    flag")`).
  - If `Error()` adds a prefix, fall back to the explicit
    `&usageError{Msg: msg, Wrapped: err}` construction so the wrapped
    chain preserves cobra's verbatim message; otherwise `usagef("%s",
    err)` is fine.
  - Document the finding in the Execution Log entry for this task.
  - files: (read-only; informs TASK-2)
  - tests: (none — prerequisite check)

- [x] **TASK-2: Wire `SetFlagErrorFunc` + `wrapCobraError` in `cli.go` (REQ-1, REQ-2, REQ-3, REQ-4, REQ-5, REQ-6).**
  - Refactor `NewRootCmd` to use a `root := &cobra.Command{...};
    root.SetFlagErrorFunc(...); return root` pattern.
  - Add `wrapCobraError(err error) error` helper.
  - Modify `RunCmd` to call `wrapCobraError(execErr)` before
    `exitCode`.
  - Add `errors` and (if not already) `strings` to imports.
  - VERIFY:
    - `make learn-build` exit 0.
    - `make learn-lint` 0 issues.
    - `grep -n 'SetFlagErrorFunc' tools/learn/cli.go` returns ≥ 1 hit.
    - `grep -n 'wrapCobraError' tools/learn/cli.go` returns ≥ 2 hits
      (1 declaration + 1 call).
    - Direct test: `(cd tools/learn && go test -run "Test_TC_CT_03|Test_TC_CT_10|Test_TC_CT_11|Test_TC_CT_04" -count=1 -v)` — CT-03 and CT-10 still PASS under the existing loose assertions (1 OR 2 → 1 is still accepted); CT-11 unchanged; CT-04 (runtime-error path) still asserts exit 2 and passes.
  - files: `tools/learn/cli.go`
  - tests: TC-D-05, TC-D-06, TC-D-07
  - depends: TASK-1

- [x] **TASK-3: Tighten TC-CT-03 + TC-CT-10 + remove DEVIATION/TODO blocks (REQ-7).**
  - In `tools/learn/cli_contract_test.go`:
    - TC-CT-03: replace `if code != 1 && code != 2 {` with `if code != 1 {`. Remove the long DEVIATION comment block (the paragraph explaining `cobra` short-circuit) AND the multi-line `// TODO: wire root.SetFlagErrorFunc ...` cross-ref block. Keep a single-line description: `// TC-CT-03: unknown subcommand yields a usage error (exit 1).`
    - TC-CT-10: replace any `code == 0` or `code != 1 && code != 2` check with `if code != 1 {`. Remove the deviation comment block referencing the cobra short-circuit. Keep a single-line description: `// TC-CT-10: unknown flag yields a usage error (exit 1).`
    - TC-CT-11: NO change (already asserts exit 1).
    - **TC-CT-09's UNRELATED DEVIATION comment (about `--db` flag inheritance) MUST be left untouched.** It explains a different, persistent deviation (TC-CT-09 uses `--db-path` instead of `--db` as the closest available surrogate). Do not delete or modify it.
  - Adjust failure message of `t.Fatalf` to mention "exit 1" not "1 or 2".
  - VERIFY (anchored to avoid TC-CT-09 false-positives):
    - `make learn-build` exit 0.
    - `make learn-lint` 0 issues.
    - `(cd tools/learn && go test -run "Test_TC_CT_03|Test_TC_CT_10|Test_TC_CT_11" -count=1 -v)` all PASS.
    - CT-03 cleaned: `grep -B 5 'func Test_TC_CT_03' tools/learn/cli_contract_test.go | grep -cE 'DEVIATION|TODO: wire root\.SetFlagErrorFunc'` returns 0.
    - CT-10 cleaned: `grep -B 5 'func Test_TC_CT_10' tools/learn/cli_contract_test.go | grep -c 'DEVIATION'` returns 0.
    - CT-09 preserved: `grep -B 5 'func Test_TC_CT_09' tools/learn/cli_contract_test.go | grep -c 'DEVIATION'` returns ≥ 1 OR `grep -c 'DEVIATION' tools/learn/cli_contract_test.go` returns ≥ 1 (CT-09's DEVIATION survives the cleanup).
    - Global TODO check: `grep -c 'TODO: wire root\.SetFlagErrorFunc' tools/learn/cli_contract_test.go` returns 0.
  - files: `tools/learn/cli_contract_test.go`
  - tests: TC-D-01, TC-D-02, TC-D-03, TC-D-04
  - depends: TASK-2

- [x] **TASK-4: Add TC-D-11 synthetic test for REQ-2 coverage.**
  - In `tools/learn/cli_test.go`, add a new test function:
    `Test_CobraRequiredFlag_exits_one(t *testing.T)`.
  - Body: build a `cobra.Command` locally (not from `NewRootCmd`) with
    a subcommand that has `cobra.MarkFlagRequired("required-flag")`.
    Attach to a root via the same `SetFlagErrorFunc` wiring (or call
    `RunCmd` against a root that has the hook installed — this is the
    cleaner path because it exercises the real production wire). Call
    without the required flag. Assert `code == 1` (not 1 OR 2) and
    `stderr` contains `"required flag"`.
  - Use `t.Parallel()` — no env mutation in this path.
  - VERIFY:
    - `make learn-build` exit 0.
    - `make learn-lint` 0 issues.
    - `(cd tools/learn && go test -run Test_CobraRequiredFlag_exits_one -count=1 -v)` PASSES.
    - Total test count is exactly **223** (`grep -h '^func Test' $(find tools/learn -name '*_test.go' -not -path '*/bin/*') | wc -l` == 223).
  - files: `tools/learn/cli_test.go`
  - tests: TC-D-11, TC-D-08
  - depends: TASK-2

- [x] **TASK-SMOKE: Full validation gate.**
  - `make learn-build && make learn-lint && make learn-test &&
    make learn-smoke` all green.
  - Test count: exactly **223** (222 baseline + 1 new
    `Test_CobraRequiredFlag_exits_one`).
  - Stderr content for CT-03 / CT-10 / CT-11 implicitly verified by
    the tightened CT-03/CT-10 asserts passing (their `strings.Contains`
    assertions still hold — see TC-D-12).
  - All 12 contract tests + new synthetic pass: `(cd tools/learn && go test -run "Test_TC_CT|Test_CobraRequiredFlag" -count=1)`.
  - files: (none — execution only)
  - depends: TASK-3, TASK-4
  - tests: TC-D-07, TC-D-08, TC-D-09, TC-D-10, TC-D-11, TC-D-12

## Parallel Batches

```
Batch 1: [TASK-1]              — read-only prereq (usageError.Error semantics)
Batch 2: [TASK-2]              — wire SetFlagErrorFunc + wrapCobraError in cli.go
Batch 3: [TASK-3, TASK-4]      — parallel: tighten TCs (cli_contract_test.go) + add TC-D-11 (cli_test.go)
Batch 4: [TASK-SMOKE]          — full validation gate
```

TASK-2 must precede TASK-3 because tightening the test assertions to
`code == 1` would fail without the production wiring landing first.
TASK-2 must also precede TASK-4 because the new synthetic test
exercises the wired hook. TASK-3 and TASK-4 touch DIFFERENT files
(`cli_contract_test.go` vs `cli_test.go`) so they can parallelize in
Batch 3 — but given the trivial size (~10 LOC each), serial execution
within Batch 3 is also acceptable. TASK-1 is a read-only prereq that
finishes in seconds.

## Validation Criteria

- [ ] `make learn-build` PASS
- [ ] `make learn-lint` 0 issues
- [ ] `make learn-test` PASS with exactly **223** test functions (222
      baseline + 1 new `Test_CobraRequiredFlag_exits_one`)
- [ ] `make learn-smoke` PASS
- [ ] `grep -n 'SetFlagErrorFunc' tools/learn/cli.go` returns ≥ 1 hit
- [ ] `grep -n 'wrapCobraError' tools/learn/cli.go` returns ≥ 2 hits
- [ ] CT-03 + CT-10 deviation cleanup (anchored, preserves CT-09):
      - `grep -B 5 'func Test_TC_CT_03' tools/learn/cli_contract_test.go | grep -cE 'DEVIATION|TODO: wire root\.SetFlagErrorFunc'` returns 0
      - `grep -B 5 'func Test_TC_CT_10' tools/learn/cli_contract_test.go | grep -c 'DEVIATION'` returns 0
      - `grep -c 'TODO: wire root\.SetFlagErrorFunc' tools/learn/cli_contract_test.go` returns 0 (global)
      - `grep -c 'DEVIATION' tools/learn/cli_contract_test.go` returns ≥ 1 (TC-CT-09's UNRELATED deviation survives)
- [ ] `grep -A 25 'func Test_TC_CT_03' tools/learn/cli_contract_test.go`
      shows assertion `code != 1` (single condition, not OR)
- [ ] `grep -A 30 'func Test_TC_CT_10' tools/learn/cli_contract_test.go`
      shows assertion `code != 1` (single condition, not OR)
- [ ] Stderr substring assertions in TC-CT-03 / TC-CT-10 / TC-CT-11
      unchanged (`"unknown command"`, `"unknown flag"` /
      `"unknown shorthand"`, non-empty stderr for missing required)
- [ ] `go test -run Test_TC_CT_04 ./tools/learn/` PASS (runtime-error
      exit 2 unchanged)
- [ ] `go test -run Test_CobraRequiredFlag_exits_one ./tools/learn/` PASS
- [ ] No production code changes outside `tools/learn/cli.go`
- [ ] No test changes outside `tools/learn/cli_contract_test.go` and
      `tools/learn/cli_test.go`

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
