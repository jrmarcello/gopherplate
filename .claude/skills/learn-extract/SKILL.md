---
name: learn-extract
description: Stage 3 of the Learning Loop — triage candidates.jsonl into new skills, new memory entries, updates, or discards. Records every decision; never deletes.
argument-hint: "[--candidates <path>]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-5"
  - "spec:learning-loop-harness:REQ-6"
created_at: 2026-05-14T20:00:00Z
last_reviewed_at: 2026-05-14T20:00:00Z
---

# /learn-extract [--candidates <path>]

Stage 3 of the Learning Loop (REQ-5). Reads the candidate patterns produced by
`learn extract`, applies the skill-quality rubric (REQ-6, cited literally
below), and for each candidate decides among four actions:
**new-skill / new-memory / update / discard**. The skill DECIDES; the
deterministic binary `learn record-decision` APPLIES the file mutation and
records an audit row. Nothing is auto-applied without user approval on
judgment calls.

## What this covers

- Loading `candidates.jsonl` (output of `learn extract`).
- Scoring each candidate 1–5 on every rubric criterion with the anchor examples.
- Deciding among new-skill / new-memory / update existing / discard.
- Calling `learn record-decision` so every triage step persists with rationale.
- Presenting the proposed batch of actions to the user before mass-recording.

## What this does NOT cover

- Pattern extraction itself — invoke `learn extract` first; this skill only
  consumes the output.
- Refinement / merge of skills already in the index — that's `/learn-refine`.
- File deletion of any kind — anti-deletion principle (REQ-10).
- Auto-applying judgment-call decisions without user approval. Trivial cases
  (clearly-discard, clearly-update) may be recorded inline; new-skill /
  new-memory always present to the user first.

## The rubric (literal citation — REQ-6)

> The following block is cited verbatim from `.claude/rules/skill-quality.md`.
> Update the rule file, not this citation; periodic refinement of `/learn-extract`
> re-syncs the block.
>
> ### 1. Foco em repetição
> A good skill automates work that lives inside the 80% repetitive part of the
> daily routine. **Score 5** — invoked routinely (multiple times per week, or
> on every feature / every PR). **Score 3** — invoked occasionally but
> recurring. **Score 1** — invoked rarely or speculatively.
>
> Anchor positives: `/spec`, `/ralph-loop`, `/validate`. Anchor negative: a
> "Q4 OKR review" skill used once per quarter.
>
> ### 2. Anti-generalização
> The agent works best with **narrowly delimited** instructions. **Score 5**
> — single explicit purpose, typed argument shape, obvious stopping
> condition. **Score 3** — clear purpose with some edge ambiguity. **Score 1**
> — open-ended "plan/design/decide" with no bounded deliverable.
>
> Anchor positives: `/migrate`, `/new-endpoint`. Anchor negative: "do the
> architecture for the team".
>
> ### 3. Modularidade
> Skills are stored as individual Markdown files indexed in **Skill Memory**
> and retrieved via **FTS5** (Hermes 10:24). **Score 5** — single SKILL.md,
> explicit frontmatter, named sections that FTS5 ranks confidently.
> **Score 3** — mostly self-contained but borrows implicit conventions from
> siblings. **Score 1** — spread across multiple files / requires reading
> other skills to make sense / frontmatter so generic FTS5 cannot rank it.
>
> Anchor positives: `/spec`, `/ralph-loop`, `/spec-review`. Anchor negative:
> a skill scattered across 3 files with implicit dependencies.
>
> ### 4. Refinabilidade
> A good skill participates in a refinement cycle — editable, mergeable,
> splittable without prose rewrite. **Score 5** — explicit boundary
> ("What this covers" / "What this does NOT cover") + numbered/named
> workflow steps. **Score 3** — boundaries implicit but structure clean.
> **Score 1** — free-form prose with no scope statement.
>
> Anchor positives: `/fix-issue` (numbered phases), `/spec-review` (explicit
> "distinct from inline self-review" boundary). Anchor negative: a single
> free-form prose block with no addressable sub-sections.

## Triage rules (from `.claude/rules/skill-quality.md` § Triagem)

- Score ≥ 4 on **ALL FOUR** → `new-skill`.
- Score ≥ 4 on criteria 1, 2, 3 but < 4 on 4 (Refinabilidade) → `new-memory`
  (memory tolerates lower refinability; skills don't).
- Score < 4 on criterion 1 or 2 → `discard` with rationale recorded.
- Score < 4 on criterion 3 (Modularidade) → propose split (record discard
  with rationale; user may invoke `/learn-extract` again after splitting).

## Workflow

### Phase 1 — Load candidates

```bash
# Run extract first if candidates.jsonl is stale
learn extract --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"

# Inspect
wc -l candidates.jsonl
jq -s 'length' candidates.jsonl  # candidate count
```

If `candidates.jsonl` is missing or empty: abort with the message
*"Run `learn extract` first; no candidates to process."*

### Phase 2 — Score and decide per candidate

For each line in `candidates.jsonl`, parse the JSON (`kind`, `signature`,
`frequency`, `contexts`, `score`, `first_seen_at`, `last_seen_at`). Apply
the rubric:

- `kind=tool-sequence` + `frequency>=5` across distinct sessions → strong
  signal for criterion 1.
- `signature` length and specificity → criterion 2 signal (single tokens =
  vague; multi-token sequences with file-path tokens = sharp).
- `contexts` clustering → if all from one session, weaker; if multi-session,
  stronger.

Record scores per criterion (1–5), then apply the triage rule.

### Phase 3 — Draft body for new-skill / new-memory

For decisions of `new-skill` or `new-memory`:

1. Derive a kebab-case slug from the dominant tokens in `signature` (e.g.
   `read-then-edit-handler` for a `Read → Edit` sequence over `internal/`
   files).
2. Draft the body following the template at
   `.claude/skills/_template/SKILL.md` — sections: summary, "What this
   covers", "What this does NOT cover", numbered workflow phases, rules,
   "When NOT to use", "Related skills".
3. Save the proposed body to `/tmp/learn-extract-<slug>.md`.

### Phase 4 — Present and record

Present each non-trivial decision to the user as a single-screen summary:

```text
Candidate <signature> (kind=<kind>, freq=<N>, contexts=<M sessions>):
  Scores: foco=5 anti-gen=4 mod=5 refin=4   → decision: new-skill
  Target: .claude/skills/<slug>/SKILL.md
  Body preview: <first 5 lines of /tmp/learn-extract-<slug>.md>
  Rationale: <one paragraph linking scores to the triage rule>
```

After confirmation (or for trivial discards inline), call:

```bash
learn record-decision \
  --candidate-signature "<signature>" \
  --action new-skill \
  --target-path ".claude/skills/<slug>/SKILL.md" \
  --description "<one-line>" \
  --rationale "<short rationale citing scores>" \
  --body-file "/tmp/learn-extract-<slug>.md" \
  --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"
```

For discards:

```bash
learn record-decision \
  --candidate-signature "<sig>" \
  --action discard \
  --rationale "criterion <N> failed: <details>" \
  --db-path "..."
```

### Phase 5 — Summary table

Print to the user:

```text
Triaged N candidates:
  new-skill:  K   (paths: ...)
  new-memory: L   (paths: ...)
  update:     M   (paths + diffs: ...)
  discard:    D   (rationale per discard: ...)

Inspect rows: learn stats   (counts.decisions)
Validate any new skill: learn validate-skill <path>
```

## Rules

- ALWAYS go through `learn record-decision`. No direct file mutation from this
  skill; the binary owns the audit row + file effect (REQ-5).
- ALWAYS apply the rubric criteria in the order 1 → 2 → 3 → 4. Order is fixed
  by `.claude/rules/skill-quality.md` and downstream tooling assumes it.
- NEVER delete files. Discards record a `decisions` row only; deprecation of
  existing skills is `/learn-refine`'s job (REQ-10).
- NEVER auto-create a skill without user approval for non-trivial decisions.
  Trivial discards (clear criterion 1 or 2 failure) may be inline-recorded.
- ALWAYS sanitize body content via the `learn` binary — `record-decision` runs
  the sanitizer over `--body-file` before writing.

## When NOT to use

- Before running `learn extract` recently — candidates are stale.
- To merge or deprecate existing skills — use `/learn-refine`.
- To produce an audit report — use `/learn-audit-skills`.
- To author a skill from scratch by hand — just edit a file and run
  `learn validate-skill <path>` to verify frontmatter.

## Related skills

- `/learn-refine` — Stage 4. Consumes skills produced by this stage to detect
  merge candidates.
- `/learn-nudge` — Stage 5. May surface candidates for re-extraction via TTL.
- `/learn-audit-skills` — One-shot non-prescriptive review of all skills.
- `/learn-recall` — Query the KB for context before triaging.
