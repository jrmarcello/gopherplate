---
name: refactor-flat-layout-baseline-discrepancies
description: Round-1 per-package count errors in refactor spec now fixed; residual gaps tracked after round-2 review
metadata:
  type: project
---

Round-1 errors were corrected in the spec (round-2 verified 2026-05-15):
- Per-package table totals now sum to 210 correctly.
- TC-CT-06 now asserts name-set equality, not just length.
- TC-ET-02 now says "15 tests" (7+8), not "30+".
- REQ-1, REQ-2, REQ-7, REQ-12 now each have explicit TC coverage.
- TC-CT-10 (invalid flag) and TC-CT-11 (missing arg) added for exit-code triggers.
- TC-CT-12 (persistent flag inheritance) added.
- TC-HI-05 formalised as a TC covering skill body binary invocations.
- TC-BM-08 (make learn-setup idempotency) present.

Residual gaps found in round-2 (may be carried into round-3 if not resolved):
1. TC-BM-05 (`make learn-reindex`) and TC-BM-07 (`make learn-stats` JSON) appear in the TC table but are NOT cited in any task's `tests:` metadata. TASK-SMOKE says `TC-BM-01..08` — that range notation is implicit, not explicit; the executor may skip TC-BM-05 and TC-BM-07 without a concrete verification step in TASK-SMOKE's prose. MUST FIX: add explicit prose steps for each.
2. TASK-BASELINE `tests:` list includes TC-CT-01 and TC-CT-02, but those TCs verify the *post-refactor* binary CLI surface. At baseline time only the pre-refactor binary exists. The tests: assignment is misleading — those TCs belong to TASK-7 or TASK-8, not TASK-BASELINE.
3. REQ-12 lists `test TempDir setup` as a silent-change risk but no TC exercises it explicitly. TC-BM-03 (`make learn-test` 210 pass) is the implicit cover, but it is not cited in REQ-12's "REQ verified by" clause.
4. TC-CT-06 implementation hint names `root.Commands()` (good) but does not state that `Commands()` returns only directly registered subcommands (not help/completion auto-commands). If cobra adds implicit subcommands, set equality will fail. Should document expected set size = 16 only registered commands, excluding cobra's auto-injected ones.

**Why:** Spec was authored in two rounds; cross-checking task tests: lists against TC table revealed TC-BM-05/07 are orphaned. TASK-BASELINE TC assignment mismatches the lifecycle of TC-CT-01/02.

**How to apply:** When reviewing any spec with a TC table and per-task `tests:` lists, enumerate orphaned TCs (in table but in no task's `tests:`) and misassigned TCs (in early task but verifiable only after later task's deliverable exists).
