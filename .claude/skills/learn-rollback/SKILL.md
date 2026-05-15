---
name: learn-rollback
description: Reverses a previously-applied learning-loop decision — restores the deprecated file back to its canonical path and appends a symmetric audit.jsonl entry.
argument-hint: "<decision-id> [--dry-run]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-9a"
created_at: 2026-05-14T22:30:00Z
last_reviewed_at: 2026-05-14T22:30:00Z
---

# /learn-rollback &lt;decision-id&gt; [--dry-run]

> The recovery counterpart to `/learn-refine` under the auto-apply policy
> (REQ-9). Given a `decision-id` (from `audit.jsonl` or the chat output of
> `/learn-refine`), restore the deprecated file back to its canonical path
> and record the reversal symmetrically in audit + decisions.

## What this covers

- Looking up a decision by id in `.claude/learning/audit.jsonl`.
- Stripping the `> Deprecated …` header from the `_deprecated/<name>-<ts>.md`
  file and moving the body back to its original canonical path.
- Inserting a `rolled-back` row in the `decisions` table that references the
  original decision via `candidate_signature=rollback:decision=<id>`.
- Appending a `rolled-back` line to `audit.jsonl` so the trail is symmetric
  (every `applied` line has a matching reversal, or none).

## What this does NOT cover

- Reversing decisions that produce NEW skills/memory (`new-skill` /
  `new-memory`) — these don't move anything to `_deprecated/`, so there's
  nothing to restore. Delete the file manually with `git rm` if you want
  to discard it.
- Restoring memory entries that were `updated` (action=update modifies in
  place; the original is gone unless captured in git history).
- Re-applying a previously rolled-back decision. If you change your mind
  again, run `/learn-refine` from scratch with the same target.
- Compensating for `_deprecated/` files that were manually deleted or moved.

## When to use

- A `/learn-refine` auto-apply turned out wrong (low-quality merge,
  duplicated content, lost nuance).
- A `/learn-nudge`-driven deprecation flagged a skill that turned out to
  still be in active use.
- Compliance / audit: rolling back a decision because the merged skill's
  rationale doesn't hold under closer review.

## Workflow

### Phase 1 — Identify the decision

The user typically passes the id directly (e.g. `/learn-rollback 42`). If
not, surface candidates by reading the audit log:

```bash
jq -r 'select(.action=="applied") | "\(.decision_id) \(.timestamp) \(.source_path) -> \(.deprecated_path)"' \
  .claude/learning/audit.jsonl | tail -20
```

### Phase 2 — Preview (recommended)

Run with `--dry-run` first to confirm what will happen:

```bash
learn rollback --id <id> --dry-run --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"
```

Output names both the deprecated path and the canonical restore target.
If either looks wrong (e.g. canonical path is now occupied), abort and
investigate before the live run.

### Phase 3 — Apply

```bash
learn rollback --id <id> --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"
```

The binary:

1. Reads the audit entry for the id.
2. Validates: entry exists, not already rolled back, deprecated file exists,
   canonical path is free.
3. Strips the `> Deprecated …` header and the trailing blank line.
4. Writes the restored body to the canonical path (0o600, dirs 0o750).
5. Removes the `_deprecated/<name>-<ts>.md` file.
6. Inserts a `rolled-back` decisions row.
7. Appends a `rolled-back` line to `audit.jsonl`.

### Phase 4 — Confirm

After the rollback succeeds, print the restored path and a one-line note:

```text
Restored .claude/skills/<slug>/SKILL.md from _deprecated/.
The original decision (id=42) remains in `decisions` for traceability; the
rollback is a NEW row (action=rolled-back) referencing it.

If you want this skill to NEVER be re-merged, edit it now to add an
explicit "What this does NOT cover" section that breaks the rubric's
modularity match against the candidate it was merged with.
```

## Rules

- ALWAYS run `--dry-run` first when the id was discovered by grep'ing
  audit.jsonl (not supplied directly by the user) — operator may have
  picked the wrong line.
- NEVER delete the `_deprecated/` file outside of a rollback. The audit
  trail expects it to exist; manual deletion breaks the rollback path.
- NEVER edit `audit.jsonl` by hand. Append-only via the binary preserves
  the invariant that every applied entry can be located by id.
- If the canonical path is already occupied (another file landed there
  after the apply), STOP and ask the user to resolve. Don't blindly
  overwrite — the new file may be legitimate work.
- If the rollback succeeds but the decisions-row insert fails, the
  binary surfaces a RuntimeError; the file is already restored, so the
  follow-up is to re-insert manually or accept the inconsistent state
  (canonical path correct, audit complete, decisions row only logs the
  original apply — survivable but worth noting in the chat).

## When NOT to use

- For decisions whose action is `new-skill` / `new-memory` / `update` /
  `discard` — these don't have a `_deprecated/` move to reverse. Use
  `git` to undo file-level changes (`git checkout HEAD -- <path>`).
- For undoing a `discard` row — discards don't touch the filesystem; the
  audit trail captures the rationale, but there's no file to "bring back".
  If the candidate it discarded was still valid, rerun `/learn-extract`
  with adjusted thresholds.

## Related skills

- `/learn-refine` — the primary producer of `applied` decisions this skill
  reverses. The pair forms the round-trip for refinement.
- `/learn-recall` — to confirm the restored skill is back in the FTS5
  index after rollback (a follow-up `learn reindex --path <restored>` is
  usually wise; the rollback itself doesn't trigger reindex).
- `/learn-nudge` — if rollback was triggered by a stale-deprecation in
  the nudge output, also adjust TTL or pin the skill explicitly.
