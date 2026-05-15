---
name: feedback-refactor-spec-patterns
description: Recurring failure modes in flat-layout / package-collapse refactor specs for Go CLI tools — test helper collisions, rename matrix gaps, init() global state, testdata inventory mismatches
metadata:
  type: feedback
---

When reviewing refactor specs that collapse many packages into one, watch for:

1. **Test helper name collisions**: functions like `newTestSanitizer`, `initStoreForTest`, `writeSkillFile` are currently isolated by package. After merge into one `package learn`, they collide if two packages defined the same helper name. The spec MUST produce a collision inventory before any rename task.

2. **Package-level mutable variable sharing**: `registrars []func(*cobra.Command)` (or any package-level slice/map) becomes shared across ALL tests in the merged package. Snapshot/restore patterns like `snapshotRegistrars` still work within white-box tests (`package learn`), but tests that reset state without restore risk polluting 200+ co-resident tests.

3. **Rename collision matrix**: free function names like `recall.Recall` → `runRecall` collide with local function names in the fat consumer (`internal/cmd/recall.go` already has `runRecall`). The rename table must be cross-referenced against all existing names in the consumer package.

4. **Testdata inventory**: use `grep -r 'testdata' tools/learn/internal/` to build the complete fixture inventory. Missing fixtures (e.g., `with-secrets.jsonl`) either mean a pre-existing test failure or an incomplete consolidation plan.

5. **Hardcoded counts in REQs**: "210 tests" and "15 test files" should be captured by TASK-BASELINE, not hardcoded in REQs. Hardcoded counts become false gates.

6. **`slog.SetDefault` in test helpers**: global slog handler mutation in test helpers (e.g., `captureSlog`) is safe only when `t.Parallel()` is absent from the test. After package collapse, adding `t.Parallel()` to any test that uses `captureSlog` without a test-scoped slog backend causes data races.

**Why:** Reviewed spec `refactor-tools-learn-flat-layout.md` (2026-05-15). All of the above caused MUST FIX findings.

**How to apply:** In any refactor spec that collapses N packages into 1, add a pre-task "collision inventory" step to TASK-BASELINE and require the merge tasks to list concrete file paths (not globs) in their `files:` metadata.
