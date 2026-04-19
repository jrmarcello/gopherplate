# Mutation testing

Mutation testing exercises your **test suite**, not your production code. It injects tiny
changes ("mutants") into the source — `>` becomes `>=`, `nil` becomes `errors.New("x")`, a
`return` is removed — and re-runs your tests. Each mutant resolves to one of three outcomes:

- **Killed** ✅ — a test failed, so the mutation was detected. This is what we want.
- **Lived** ❌ — every test still passed even with the bug in place. The mutant "survived",
  meaning the test suite doesn't actually exercise that branch.
- **Timed out / not viable** — the mutant caused an infinite loop or didn't compile; ignored.

The ratio `killed / (killed + lived)` is the **mutation score (efficacy)**. It answers a
question line-coverage cannot: not "did any test touch this line?" but "would any test notice if
this line were wrong?".

## Why we track it

80% line coverage with a 30% mutation score means tests **execute** the code but don't
**verify** behavior — e.g., they call a function but never assert on its output. Mutation
testing finds these blind spots.

Fowler lists mutation testing as a **post-integration maintainability sensor** (slow, expensive,
runs after merge). The template is configured this way: nightly CI job, 30-day artifact, no PR
gate. Efficacy is read as a trend over weeks, not as a commit-level check.

## Running it

### Locally

```bash
# Auto-installs gremlins on first run.
make mutation
```

Scope is `./internal/usecases/...` — domain is usually simple enough that mutations are
trivial, handlers are better covered by E2E, and use cases are where conditional logic lives.

Expect it to take several minutes: gremlins runs the test suite once per mutant. First a few
minutes for gathering coverage, then ~5-30s per mutant depending on test runtime.

### In CI

[.github/workflows/mutation-nightly.yml](../../.github/workflows/mutation-nightly.yml) runs daily
at 03:00 UTC and uploads the full report as a workflow artifact (`mutation-report-<run-id>`,
30-day retention). Manual trigger via `workflow_dispatch` is available.

## Reading a report

Example output excerpt:

```text
      KILLED CONDITIONALS_NEGATION at create.go:38:13
       LIVED CONDITIONALS_NEGATION at list.go:60:18
   TIMED OUT CONDITIONALS_NEGATION at create.go:52:52

Killed: 5, Lived: 4, Not covered: 0
Timed out: 2, Not viable: 0, Skipped: 0
Test efficacy: 55.56%
Mutator coverage: 100.00%
```

### What to act on

1. **Every `LIVED` mutant is a concrete gap**. Open the file at the given line, identify the
   expression that was mutated (see the mutator name), and ask: *what test would have caught
   this change?* Write it.
2. **`TIMED OUT` usually means a loop invariant was mutated** into an infinite loop. Not a test
   gap — ignore.
3. **Mutation score trending down over weeks** signals accumulating test debt. A drop of >5%
   without coverage dropping is an especially strong signal — new code is being added without
   behavioral tests.

### What not to over-interpret

- **A `LIVED` mutant in logging, error messages, or trivial getters is usually fine.** These
  have no behavioral consequence worth asserting on. Use judgment.
- **Don't chase 100% efficacy.** Above ~70% the ROI on new tests drops sharply, and you start
  writing tests for the sake of the score. Pick the easy wins.

## Mutators enabled

Configured in [.gremlins.yaml](../../.gremlins.yaml):

| Mutator | Example | Default? |
| --- | --- | --- |
| ARITHMETIC_BASE | `a + b` → `a - b` | yes |
| CONDITIONALS_BOUNDARY | `a > b` → `a >= b` | yes |
| CONDITIONALS_NEGATION | `if x` → `if !x` | yes |
| INCREMENT_DECREMENT | `i++` → `i--` | yes |
| INVERT_NEGATIVES | `-x` → `x` | yes |

More aggressive mutators (`invert-assignments`, `invert-logical`, `invert-loopctrl`, etc.) are
disabled — they produce a lot of mutants that no realistic test would catch, drowning the
signal in noise. Enable selectively if a package is mature and you want deeper analysis.

## Extending scope

When you add a domain that's behaviorally rich (complex use cases, business rules), keep it
under `internal/usecases/` so gremlins naturally covers it. If you need to mutate other
packages, add them to the `make mutation` target or run directly:

```bash
gremlins unleash ./internal/domain/...
```

## Related

- [docs/harness.md](../harness.md) — mutation testing is listed as a maintainability sensor.
- [docs/guides/harness-self-steering.md](harness-self-steering.md) — when to open a gap note
  for new mutators or scope changes.
- [.specs/maintainability-harness.md](../../.specs/maintainability-harness.md) — full spec with
  test plan and rationale.
