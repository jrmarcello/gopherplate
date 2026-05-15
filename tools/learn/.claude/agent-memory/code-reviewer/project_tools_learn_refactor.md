---
name: tools-learn-refactor
description: Flat-layout refactor of tools/learn (14 internal packages → single package learn). Key audit findings, identifier traps, and testdata gotchas.
metadata:
  type: project
---

Spec: `.specs/refactor-tools-learn-flat-layout.md`. Status: DRAFT under review.

Key audit findings (from Design section review, 2026-05-15):

- `FormatTimestamp` in `internal/audit/audit.go` is exported (PascalCase) but is NOT used by main.go. The spec lists it as going to `formatAuditTimestamp` (unexported). This is correct, but the source shows a comment "Exported for tests" — that comment is wrong because the test IS in the same package (white-box), so it can access unexported names. No action needed beyond removing the misleading comment.
- `audit.FileName` constant is exported. Used in test via `FileName` constant (white-box test). After merge it becomes package-private — the test still works. Spec calls it `auditFileName`. But `FileName` is a *const*, not a type — the rename table in the spec lists it but doesn't distinguish const vs func vs type. Not a bug, just note.
- `similar.Query` (free function) is referenced in `recall.go` as the production `queryFn`. After merge it becomes unexported `runSimilar`... wait: spec renames `similar.Query` → `runSimilar` is for the struct method `Query` in similar package. Actually `bm25.go` has `func Query(...)` which is a free function. In cmd/similar.go there is ALSO a `type queryFn` and a `runSimilar` function. Name collision: `similar.Query` → `runSimilar` would collide with cmd/similar.go's `runSimilar` local helper if both end up in `package learn`. Spec needs to acknowledge this.
- `transcript` testdata is missing `with-secrets.jsonl` from the glob list — it exists (referenced in parser_test.go) but is not in `internal/ingest/transcript/testdata/transcripts/`. The glob showed only `malformed.jsonl` and `valid.jsonl`. This is an existing pre-refactor gap that the spec's testdata consolidation does not address.
- `ingest/git` tests use NO testdata files — all fixtures are generated inline. Safe to move.
- `ingest/memory` tests use NO testdata files — all fixtures written to t.TempDir(). Safe to move.
- `ingest/transcript` tests reference `"testdata/transcripts/..."` as relative paths. After move to package `learn` at the root, those paths resolve to `tools/learn/testdata/transcripts/` — correct.
- `ingest/spec` tests reference `filepath.Join("testdata", "specs", name)` as relative paths (via `fixture()` helper). After move to root, resolves to `tools/learn/testdata/specs/` — correct.
- Batch 8 parallelism: TASK-8 writes `cli_contract_test.go` which uses `NewRootCmd()` and `RunCmd()` — these are in `cli.go` (flat). No shared write conflict with TASK-9 (pure execution) or TASK-10 (docs). Parallel is safe.

**Why:** Needed to track spec gaps before ralph-loop execution to prevent silent regressions.
**How to apply:** When ralph-loop is run against this spec, reference these findings to guide TASK-2/TASK-3/TASK-4 implementation.
