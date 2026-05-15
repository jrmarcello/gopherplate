---
name: learn-refine
description: Stage 4 of the Learning Loop — propose merges or deprecations for similar skills/memory. Auto-applies each decision (REQ-9) + appends audit.jsonl (REQ-9a); reverse with /learn-rollback. Anti-deletion preserved.
argument-hint: "[--skill <path>] [--dry-run]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-8"
  - "spec:learning-loop-harness:REQ-9"
  - "spec:learning-loop-harness:REQ-10"
  - "spec:learning-loop-harness:REQ-9a"
created_at: 2026-05-14T20:00:00Z
last_reviewed_at: 2026-05-14T22:30:00Z
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

### Phase 4 — Present + auto-apply (REQ-9: auto-apply + audit policy)

For each MERGE / DEPRECATE proposal, print to the user **before** applying:

```text
Auto-applying MERGE:
  Target (keep, update):   <target-path>
  Candidate (deprecate):   <candidate-path>
  Similarity:              BM25=<score>, edit_distance=<d>
  Rubric verdict (merged): foco=5 anti-gen=5 mod=5 refin=5

  Unified diff:
  <contents of /tmp/learn-refine-<target-slug>.diff>

  Rationale: <paragraph linking scores to MERGE decision>
```

Then immediately call `learn refine-apply` (REQ-9, auto-apply policy). It
inserts the decision row, moves the candidate to `_deprecated/`, stamps the
REQ-10 header, and appends one line to `.claude/learning/audit.jsonl`
(REQ-9a):

```bash
learn refine-apply \
  --candidate-signature "merge:<target>+<candidate>" \
  --target-path "<candidate-path>" \
  --merged-into "<target-path>" \
  --diff-file "/tmp/learn-refine-<target-slug>.diff" \
  --rationale "merge with <target-path>; foco=5 anti-gen=5 mod=5 refin=5" \
  --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"
```

For MERGE you must ALSO write the merged content over the target file
(refine-apply only handles the candidate's move). Use `Write` for that step
explicitly.

After all proposals process, print the final summary:

```text
Applied N decisions. Review trail with:
  jq . .claude/learning/audit.jsonl

To roll back any decision:
  /learn-rollback <decision-id>
```

### Phase 5 — (Removed) — auto-apply makes the old "wait for approval" step obsolete

The previous design (REQ-9 pre-change) required the user to type "apply 42"
between Phase 4 and the actual move. Under the auto-apply policy this gate
is replaced by the audit + rollback path:

- Every move is logged in `audit.jsonl` (REQ-9a).
- Every move is reversible in one shot with `/learn-rollback <id>`.
- Anti-deletion (REQ-10) guarantees the deprecated file still exists.

The user reviews **after** the fact, not before. This is the explicit
quality-vs-velocity trade-off documented in the spec's REQ-9 narrative.

## Rules

- ALWAYS print the proposal block (with diff + rationale) BEFORE calling
  refine-apply. The user must SEE what was applied; auto-apply does not
  mean silent-apply.
- ALWAYS call `learn refine-apply` (not the legacy `record-decision +
  apply-decision` two-step) so the audit entry is guaranteed.
- NEVER delete files. Deprecation = move to `_deprecated/` with the REQ-10
  header. The binary enforces this; the skill must not bypass it.
- NEVER lose content. Every unique section from both files must appear in
  the merge — explicitly note redundant content in the rationale if dropped.
- If the user wants a preview before the irreversible step, use
  `--dry-run` on refine-apply explicitly — this prints the diff without
  mutating, and the user can then re-invoke without `--dry-run`.
- For high-risk merges (large files, much-used skills) consider proposing
  `--dry-run` first as a courtesy — the policy permits auto-apply but
  doesn't mandate it for every case.

## When NOT to use

- For Stage 3 creation — that's `/learn-extract`.
- For TTL-based periodic deprecation — that's `/learn-nudge` (which may
  delegate clusters to this skill).
- For inspecting similar skills without intent to act — call
  `learn similar` directly, no skill wrapper needed.

## Related skills

- `/learn-rollback` — One-shot reversal of any decision applied by this skill.
  Use this if a merge turns out wrong (auto-apply policy assumes you'll
  catch bad merges post-hoc).
- `/learn-extract` — Stage 3, produces inputs to refinement.
- `/learn-nudge` — Stage 5, surfaces TTL-expired clusters; delegates merge
  proposals here.
- `/learn-audit-skills` — observation-only audit; may flag refine candidates.
