---
name: learn-refine
description: Stage 4 of the Learning Loop — propose merges or deprecations for similar skills/memory. Records pending-approval decisions; applies only after explicit user approval. Anti-deletion.
argument-hint: "[--skill <path>] [--dry-run]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-8"
  - "spec:learning-loop-harness:REQ-9"
  - "spec:learning-loop-harness:REQ-10"
created_at: 2026-05-14T20:00:00Z
last_reviewed_at: 2026-05-14T20:00:00Z
---

# /learn-refine [--skill <path>] [--dry-run]

Stage 4 of the Learning Loop (REQ-9). Finds skills/memory entries similar to a
target via `learn similar`, applies the skill-quality rubric (REQ-6, cited
literally below) to decide **MERGE / KEEP_SEPARATE / DEPRECATE**, drafts the
unified content, and records each proposal as a **pending-approval** decision.
Applies only after explicit user approval — presente-before-commit pattern
identical to `/ralph-loop`. Never deletes (REQ-10: moves to `_deprecated/`
with the literal header).

## What this covers

- Finding near-duplicate skills/memory via `learn similar` (FTS5 BM25 +
  Levenshtein).
- Scoring each (target, candidate) pair against the rubric to decide if a
  merge is warranted, the two should stay separate, or the candidate should
  be deprecated entirely.
- Drafting the unified merged content (no knowledge loss).
- Recording every proposal as `decisions.action=pending-approval` with the
  proposed diff captured.
- Applying via `learn apply-decision --id <N>` only after explicit user
  approval (the skill never auto-applies).

## What this does NOT cover

- Creating brand-new skills from raw candidates — that's `/learn-extract`.
- One-shot audit of all skills — that's `/learn-audit-skills`.
- Cross-domain memory consolidation — focus on near-duplicates only.
- File deletion — anti-deletion principle (REQ-10). Deprecation = move to
  `_deprecated/` with header, not unlink.

## The rubric (literal citation — REQ-6)

> Cited verbatim from `.claude/rules/skill-quality.md`. Re-sync on rule edits.
>
> ### 1. Foco em repetição
> 80% repetitive workload. Score 5 = invoked routinely; 3 = occasionally;
> 1 = rarely/speculatively. Positives: `/spec`, `/ralph-loop`, `/validate`.
>
> ### 2. Anti-generalização
> Narrow scope, typed args, clear stopping condition. Score 5 = single
> explicit purpose; 3 = some edge ambiguity; 1 = open-ended "plan/design".
> Positives: `/migrate`, `/new-endpoint`.
>
> ### 3. Modularidade
> Single SKILL.md, named sections, FTS5-friendly frontmatter. Score 5 =
> self-contained + ranked confidently; 3 = implicit borrowing from siblings;
> 1 = scattered across files. Positives: `/spec`, `/ralph-loop`,
> `/spec-review`.
>
> ### 4. Refinabilidade
> Explicit "What this covers" / "What this does NOT cover" + numbered phases.
> Score 5 = boundary stated, phases addressable; 3 = implicit but clean
> structure; 1 = free-form prose with no scope statement. Positives:
> `/fix-issue` (5 numbered phases), `/spec-review` (explicit boundary
> against `/ralph-loop` inline review).

## Merge decision logic

For each (target, candidate) pair from `learn similar`:

- **MERGE** — both skills cover the same problem space; refinabilidade ≥ 4 on
  both; merging produces a skill that scores ≥ 4 on all four criteria.
- **KEEP_SEPARATE** — high textual similarity but the rubric reveals they
  address different aspects (e.g. `/learn-extract` and `/learn-refine` share
  vocabulary but are different stages).
- **DEPRECATE** — candidate is strictly inferior or its content is already
  present in target; refinabilidade-3 on candidate, refinabilidade-5 on
  target. Candidate moves to `_deprecated/` (REQ-10), target unchanged.

## Workflow

### Phase 1 — Find candidates

```bash
learn similar \
  --skill "<path>" \
  --top-k 5 \
  --threshold 0.6 \
  --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite" \
  > /tmp/learn-refine-candidates.json
```

If empty: print *"No merge candidates above similarity threshold 0.6"* and
stop.

### Phase 2 — Apply rubric per pair

For each candidate, score the (target + candidate) merge against the rubric.
Two key questions:

- Does the merged content score ≥ 4 on criteria 3 and 4? (If not, the merge
  creates a worse skill than either input.)
- Does the candidate score ≥ 4 on criterion 1 in isolation? (If not,
  candidate is a deprecation candidate, not a merge candidate — even if the
  texts overlap.)

### Phase 3 — Draft merged content (for MERGE)

For MERGE proposals only, write the unified content preserving:

- All unique sections from BOTH files.
- The strongest "What this covers" / "What this does NOT cover" pair (favor
  whichever is more explicit).
- All workflow steps from both, renumbered cleanly.
- Both `learning_provenance` entries.
- `last_reviewed_at` bumped to now (UTC RFC3339).

Save the merged body to `/tmp/learn-refine-<target-slug>.md`. Save the
unified diff (target → merged) to `/tmp/learn-refine-<target-slug>.diff`.

### Phase 4 — Present and record pending-approval

For each MERGE / DEPRECATE proposal, print to the user:

```text
Proposed action: MERGE
  Target (keep, update):   <target-path>
  Candidate (deprecate):   <candidate-path>
  Similarity:              BM25=<score>, edit_distance=<d>
  Rubric verdict (merged): foco=5 anti-gen=5 mod=5 refin=5

  Unified diff:
  <contents of /tmp/learn-refine-<target-slug>.diff>

  Rationale: <paragraph linking scores to MERGE decision>
```

Record EACH proposal as pending-approval (REQ-9):

```bash
learn record-decision \
  --candidate-signature "merge:<target>+<candidate>" \
  --action pending-approval \
  --target-path "<target-path>" \
  --diff-file "/tmp/learn-refine-<target-slug>.diff" \
  --rationale "merge with <candidate-path>; foco=5 anti-gen=5 mod=5 refin=5" \
  --db-path "..."
```

The binary returns the decision `id`. Echo all decision IDs to the user:

```text
Proposals recorded as pending-approval. Decision IDs: 42, 43, 44.
Preview with: learn apply-decision --id <N> --dry-run
Apply with:   learn apply-decision --id <N>
Reject by leaving the row at pending-approval (or remove manually).
```

### Phase 5 — Apply (only after explicit user approval)

When the user explicitly says "apply 42" or runs `/learn-refine` with an
already-approved decision ID:

```bash
# Preview first (REQ-9 dry-run support)
learn apply-decision --id 42 --dry-run --db-path "..."

# Then commit
learn apply-decision --id 42 --db-path "..."
```

The binary:

- Moves the candidate file to
  `.claude/skills/_deprecated/<slug>-<YYYYMMDDTHHMMSSZ>.md` (REQ-10 UTC
  timestamp).
- Inserts header on line 1 (REQ-10 verbatim):
  `> Deprecated <YYYY-MM-DD> by /learn-refine: merged into <target-path>`
- Writes the merged content to the target file.
- Updates the decision row to `action=applied`.

## Rules

- NEVER apply without explicit user approval. Pending-approval rows sit until
  the user explicitly invokes `learn apply-decision` (REQ-9).
- NEVER delete files. Deprecation = move to `_deprecated/` with the REQ-10
  header. The binary enforces this; the skill must not bypass it.
- NEVER lose content. Every unique section from both files must appear in
  the merge — explicitly note redundant content in the rationale if dropped.
- ALWAYS record decisions before presenting. The audit trail is sacred.
- `--dry-run` is the SAFE default for first-time apply; explicit
  no-`--dry-run` commits.

## When NOT to use

- For Stage 3 creation — that's `/learn-extract`.
- For TTL-based periodic deprecation — that's `/learn-nudge` (which may
  delegate clusters to this skill).
- For inspecting similar skills without intent to act — call
  `learn similar` directly, no skill wrapper needed.

## Related skills

- `/learn-extract` — Stage 3, produces inputs to refinement.
- `/learn-nudge` — Stage 5, surfaces TTL-expired clusters; delegates merge
  proposals here.
- `/learn-audit-skills` — observation-only audit; may flag refine candidates.
