---
name: learn-nudge
description: Stage 5 of the Learning Loop — periodic autoavaliação. Surfaces skills/memory candidates for deprecation by TTL, proposes consolidation clusters, and resets the counter via the deterministic binary.
argument-hint: "[--ttl-days N]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-11"
  - "spec:learning-loop-harness:REQ-12"
  - "spec:learning-loop-harness:REQ-13"
created_at: 2026-05-14T20:00:00Z
last_reviewed_at: 2026-05-14T20:00:00Z
---

# /learn-nudge [--ttl-days N]

Stage 5 of the Learning Loop (REQ-12). Triggered manually OR after the Stop
hook prints the advisory `Learning nudge due — run /learn-nudge when ready
(<N> specs since last nudge)` (REQ-13 — the hook NEVER auto-invokes this
skill). Queries deprecation candidates via TTL, categorizes them, surfaces
consolidation suggestions, and resets the counter via the deterministic
binary after presenting (REQ-11 — only the binary mutates `nudge_state`).

## What this covers

- Querying `learn nudge-tick` for deprecation candidates (skills + memory with
  `indexed_at < now() - 2×TTL` AND `last_used_at ≤ now() - TTL`, inclusive).
- Categorizing each candidate: DEPRECATE / PROMOTE_TO_SKILL / DELEGATE / KEEP.
- Surfacing cluster patterns that benefit from `/learn-refine`.
- Recording any DEPRECATE recommendation as `decisions.action=pending-approval`.
- Calling `learn nudge-tick --reset` in Phase 4 — always, regardless of the
  user's response to the suggestions (REQ-11).

## What this does NOT cover

- Auto-applying deprecations — pending-approval is the maximum the skill does.
- Pattern extraction (Stage 2) or skill creation (Stage 3).
- Merging skill clusters in detail — delegate to `/learn-refine` for actual
  diff drafting and approval workflow.
- File deletion — anti-deletion (REQ-10). The binary handles `_deprecated/`
  moves.
- Resetting the counter outside this skill or `learn nudge-tick --reset`.

## Trigger semantics (REQ-11, REQ-13)

There are exactly two valid entry points:

1. **After the Stop hook advisory**. When `learn complete-task` emits
   `LEARN_NUDGE_DUE=true counter=<N>` on stderr, the `stop-learn.sh` hook
   prints to stderr (verbatim, REQ-13):
   ```text
   Learning nudge due — run /learn-nudge when ready (<N> specs since last nudge)
   ```
   The hook MUST NOT invoke any skill. The user reads the advisory and runs
   `/learn-nudge` when ready.
2. **Manual**, any time, regardless of counter. The skill still queries
   candidates and resets the counter via the binary in Phase 4.

## Workflow

### Phase 1 — Query candidates

```bash
learn nudge-tick \
  --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite" \
  > /tmp/learn-nudge-candidates.json
```

The output JSON shape:

```json
{
  "checked_at": "RFC3339",
  "ttl_days": 90,
  "candidates": [
    {"kind": "skill",  "path": "...", "indexed_at": "...", "last_used_at": "...", "usage_count": 0},
    {"kind": "memory", "path": "...", "indexed_at": "...", "last_used_at": null, "usage_count": 3}
  ]
}
```

Empty candidates → still proceed to Phase 4 (reset). Print:
*"No deprecation candidates at TTL=<N>d. Counter reset."*

### Phase 2 — Categorize per candidate

For each candidate, decide one of:

- **DEPRECATE** — `usage_count==0` AND `indexed_at` ≥ 2×TTL ago. Strong
  signal the artifact is dead. Record pending-approval.
- **PROMOTE_TO_SKILL** — memory entry with `usage_count > 0` recently but
  never promoted to a skill. Surface to user for manual consideration.
- **DELEGATE_TO_REFINE** — candidate is part of a textually-similar cluster
  (check via `learn similar`). Suggest user invoke `/learn-refine` instead.
- **KEEP** — low confidence; surface as observation only, no action.

### Phase 3 — Present to user

Print a structured summary:

```text
Nudge results (TTL=<N>d, evaluated at <timestamp>):

DEPRECATE candidates (M):
  - .claude/skills/<slug>/SKILL.md
      indexed_at: <date>  (age: <Nd>)
      last_used:  <date>  (idle: <Md>)
      Recommendation: pending-approval (decision id will be returned)

PROMOTE candidates (M):
  - memory/<slug>.md
      usage_count: <N>
      Recommendation: consider promoting to skill (manual)

DELEGATE TO /learn-refine (M clusters):
  Cluster 1: .claude/skills/{a, b, c}/SKILL.md
      Run: /learn-refine --skill .claude/skills/a/SKILL.md

KEEP / observation only (M):
  - <path>: <one-line reason>
```

For each DEPRECATE, record pending-approval (REQ-9):

```bash
learn record-decision \
  --candidate-signature "ttl-expired:<path>" \
  --action pending-approval \
  --target-path "<path>" \
  --rationale "Indexed <Nd> ago; last used <Md> ago (TTL=<TTL>d, 2×TTL=<2TTL>d)" \
  --db-path "..."
```

### Phase 4 — Reset counter (mandatory, deterministic)

ALWAYS reset, regardless of user response above:

```bash
learn nudge-tick --reset --db-path "..."
```

This is the deterministic operation: `nudge_state.counter → 0`,
`last_nudge_at → now()`. REQ-11 mandates this is binary-only — never
LLM-driven.

### Phase 5 — Final report

```text
Counter reset. K pending-approval decisions recorded.
Decision IDs: 42, 43.
Inspect: learn stats   (counts.decisions, last_nudge_at)
Apply a deprecation: learn apply-decision --id <N>
```

## Rules

- ALWAYS call `learn nudge-tick --reset` in Phase 4 — even if the user
  rejects every suggestion. Reset is the binary's job (REQ-11).
- NEVER auto-apply deprecations. Pending-approval is the cap.
- NEVER delete files. Use `_deprecated/` with the REQ-10 header (the binary
  enforces this during `learn apply-decision`).
- ALWAYS record decisions for each DEPRECATE — audit trail required.
- The advisory output (REQ-13 literal string) is the hook's job, not this
  skill's. This skill runs AFTER the advisory, on user request.
- This skill MUST NOT modify the counter directly via SQL or by other paths
  — only `learn nudge-tick --reset` is sanctioned (REQ-11).

## When NOT to use

- For new-skill creation — that's `/learn-extract`.
- For merge diff drafting and apply — that's `/learn-refine`.
- For a one-shot rubric review of all skills — that's `/learn-audit-skills`.
- For tracking skill usage per query — that's the
  `user-prompt-submit-recall.sh` hook + `learn track-use`.

## Related skills

- `/learn-extract` — Stage 3, produces new skills that eventually age out via
  TTL and surface here.
- `/learn-refine` — Stage 4, applies merges flagged by this skill via the
  DELEGATE_TO_REFINE category.
- `/learn-audit-skills` — different scope; rubric audit independent of TTL.
- `/learn-recall` — query the KB to understand usage patterns informing
  PROMOTE decisions.
