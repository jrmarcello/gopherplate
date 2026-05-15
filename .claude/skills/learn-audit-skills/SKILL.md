---
name: learn-audit-skills
description: One-shot audit of every .claude/skills/<name>/SKILL.md against the skill-quality rubric. Produces a non-prescriptive report in .specs/reports/.
argument-hint: "[--skills-dir <dir>]"
user-invocable: true
learning_provenance:
  - spec:learning-loop-harness#REQ-25
  - task:learning-loop-harness#TASK-32
created_at: 2026-05-14T00:00:00Z
last_reviewed_at: 2026-05-14T00:00:00Z
---

# /learn-audit-skills [--skills-dir <dir>]

> ⚠️ **This is an observation report. Do not modify any skill file. All
> suggestions are non-prescriptive and require user approval before any
> action.** REQ-25 of `.specs/learning-loop-harness.md`.

One-shot audit of the harness's skill collection. Reads every
`.claude/skills/<name>/SKILL.md`, scores it 1–5 against the four-criterion
rubric in `.claude/rules/skill-quality.md`, and writes a dated, append-only
report to `.specs/reports/skill-audit-<YYYY-MM-DD>.md`. The skill never edits
any other file in `.claude/skills/`.

This skill exists so the learning loop (REQ-25) can periodically take stock of
the skill collection without giving any agent the right to mutate it. Acting on
the findings is a separate, explicit user decision.

> 🎯 **Princípio diretor:** qualidade > velocidade > custo. When in doubt about
> a score, prefer the harsher score and write a richer observation — a richer
> report is cheaper than a missed weakness graduating into the index.

## When to run

- After significant changes to the skill collection (a new batch of skills
  landed, a `/learn-refine` merge cycle finished, a manual rewrite touched
  several SKILL.md files).
- Before tagging a release of the harness, so the release notes can cite the
  current shape of the collection.
- When the rubric in `.claude/rules/skill-quality.md` itself updates — every
  prior audit becomes stale and a fresh pass is the only way to know which
  skills slipped.
- Periodically (e.g., monthly) as a hygiene pass, even when no obvious trigger
  fired.

## When NOT to run

- During a normal feature workflow. The audit is one-shot, not per-task.
- Inside `/ralph-loop` or `/spec`. Those flows have their own inline review
  agents (see `.claude/rules/sdd.md` § Discipline Checkpoints).
- To score memory entries. Memory has a different rubric and a different
  tolerance for low refinability — see the triagem rules in
  `.claude/rules/skill-quality.md`.

## Workflow

### 1. Gather input

Run the deterministic helper to dump structured metadata for every skill. The
agent does **not** read SKILL.md files directly — it goes through the helper
so the audit is reproducible and the inputs are stable across runs.

```bash
learn audit-skills-prep --skills-dir .claude/skills > /tmp/skills-audit-input.json
```

The helper emits one JSON object per skill with at least:
`{path, name, description, argument_hint, user_invocable, body_excerpt,
body_size_bytes, tags}`. If the user passed `--skills-dir <dir>`, forward it
verbatim to the helper.

If the input array is empty, **still produce a report** stating "0 skills
audited" with the timestamp and the directory that was scanned. An empty
report is a valid audit result.

### 2. Apply the rubric

Read `.claude/rules/skill-quality.md`. The four criteria are cited verbatim
below so the agent invoking this skill has the rubric in-context (REQ-6
literal-citation rule, also imposed by TC-E2E-11b). When the rubric file
changes, this block must be re-synced — the citation is authoritative for the
audit, not a paraphrase.

> ### 1. Foco em repetição (Focus on repetition)
>
> A good skill automates work that lives inside the 80% repetitive part of the
> daily routine. It does **not** try to optimize for one-off rituals or
> exception-handling. If the skill would be invoked once a quarter (or once),
> it probably belongs in a runbook, a doc, or a memory entry — not in
> `.claude/skills/`.
>
> - **Score 5** — invoked routinely (multiple times per week, or on every
>   feature / every PR). Central to how work actually flows through the
>   repository. The skill replaces a sequence the user would otherwise type by
>   hand every time.
> - **Score 3** — invoked occasionally (weeks apart) but still recurring; the
>   workflow is real but the cadence is lower. The skill is useful, just not
>   daily.
> - **Score 1** — invoked rarely or speculatively. The skill exists "in case"
>   the scenario shows up. Maintenance cost will dominate value.
>
> Anchor examples: `/spec`, `/ralph-loop`, `/validate` score 5 (run on every
> feature / every commit). A hypothetical "publish a Q4 OKR review" skill
> scores 1 — once-per-quarter, belongs in a runbook.
>
> ### 2. Anti-generalização (Anti-generalization)
>
> The agent works best with **narrowly delimited** instructions. A good skill
> has a sharp scope, a small number of well-named inputs, and a clear stopping
> condition. Vague catch-all skills like "plan everything based on the last
> PRs" should be rejected — they ask the model to infer scope at runtime,
> which is exactly when the model fails most.
>
> - **Score 5** — single, explicit purpose. Argument shape is well-typed
>   (subcommand, file path, short string). Acceptance criteria for "done" are
>   obvious from the SKILL.md.
> - **Score 3** — clear purpose but with some ambiguity at the edges (e.g.
>   two closely related sub-flows blended into one skill). Could be split
>   without losing meaning.
> - **Score 1** — open-ended, asks the agent to "plan", "design", or "decide"
>   without bounding the deliverable. Title is a verb in the imperative with
>   no noun ("just figure it out").
>
> Anchor examples: `/migrate` (`create | up | down | status` subcommands) and
> `/new-endpoint` (`<method> <path> <description>`) score 5. A hypothetical
> "do the architecture" skill scores 1.
>
> ### 3. Modularidade (Modularity)
>
> Skills are stored as individual Markdown files indexed in **Skill Memory**,
> and the agent retrieves them via **FTS5** full-text search. A modular skill
> is one that the FTS5 ranker can confidently identify and that the agent can
> invoke in isolation without dragging in side-cars. Each skill should be
> self-contained: a single SKILL.md whose frontmatter and named sections give
> the search index strong signal.
>
> - **Score 5** — single SKILL.md with explicit frontmatter (`name`,
>   `description`, optional `argument-hint`, `user-invocable`). Named sections
>   (`## Phases`, `## When to use`, `## Workflow`) carry the terminology a
>   user would actually search for. No hidden coupling to other skills.
> - **Score 3** — mostly self-contained but borrows from another skill's
>   conventions implicitly (e.g. assumes the reader already knows what
>   `/validate` does). Still indexable, but FTS5 ranking will be weaker
>   because the terminology is diluted.
> - **Score 1** — spread across multiple files, or relies on reading two
>   unrelated skills to make sense, or has a frontmatter so generic that FTS5
>   cannot distinguish it from siblings.
>
> Anchor examples: `/spec`, `/ralph-loop`, `/spec-review` score 5
> (self-contained, explicit frontmatter, distinguishable terminology). A
> skill scattered across three files with implicit dependencies scores 1.
>
> ### 4. Refinabilidade (Refinability)
>
> A good skill participates in a refinement cycle: it can be edited, merged
> with a sibling, or split into two without rewriting its prose from scratch.
> Refinability comes from explicit boundaries — a skill that states *what it
> covers* and *what it does NOT cover* lets the loop detect overlap with
> neighbours and act on it. Free-form anecdote, however accurate, is fragile
> under merge.
>
> - **Score 5** — explicit boundary section ("What this covers" / "What this
>   does NOT cover", or an equivalent scoping paragraph). Phases or workflow
>   steps are numbered and addressable. Easy to diff against a sibling skill.
> - **Score 3** — boundaries are implicit but the structure is clean enough
>   to reason about. Merging would require some rewording but no structural
>   rework.
> - **Score 1** — content is mostly free-form prose with no explicit scope
>   statement. Every refinement attempt risks breaking the narrative. Merging
>   with a sibling would force a full rewrite.
>
> Anchor examples: `/fix-issue` (numbered steps 1–5) and `/spec-review`
> (explicit boundary against `/ralph-loop`) score 5. A free-form prose skill
> with no scope statement scores 1.

For EACH skill in the input JSON, apply the rubric and capture:

- 4 scores (one per criterion, integer 1–5).
- 1-line observation per criterion (a positive note OR a concern, never
  both — pick whichever is more useful to the human reader).
- Overall **review-candidate?** boolean — `true` if any criterion scored `< 3`
  (matches the rubric's own flag at `.claude/rules/skill-quality.md`
  § Scoring methodology, step 4).
- Suggested action if review-candidate: one of `NONE`, `SPLIT`,
  `MERGE_WITH_<other-skill>`, `REWRITE`, `DEPRECATE`. Annotate every action as
  a **suggestion**, never as an instruction. The user decides.

Use the anchor examples in the rubric as the calibration set. Do not invent
new reference points at audit time — that would make scores drift between
runs.

### 3. Generate report

Write the report to `.specs/reports/skill-audit-<YYYY-MM-DD>.md` (use today's
date in ISO 8601). If a file with the same name already exists from an
earlier run today, append a `-<NN>` suffix (`-01`, `-02`, …) rather than
overwriting — anti-deletion principle (REQ-10).

Report skeleton (fill in the bracketed parts):

```markdown
# Skill Audit — <YYYY-MM-DD>

> Generated by `/learn-audit-skills` (REQ-25 of
> `.specs/learning-loop-harness.md`). Observations only — no changes have
> been made to any skill file. All suggestions require explicit approval.

## Summary

- Total skills audited: <N>
- Review candidates (any score < 3): <M>
- Average score per criterion:
  - Foco em repetição: <avg>
  - Anti-generalização: <avg>
  - Modularidade: <avg>
  - Refinabilidade: <avg>

## Per-skill results

### `.claude/skills/<name>/SKILL.md`

| Criterion | Score | Observation |
|---|---|---|
| Foco em repetição | <s>/5 | <one-line observation> |
| Anti-generalização | <s>/5 | <one-line observation> |
| Modularidade | <s>/5 | <one-line observation> |
| Refinabilidade | <s>/5 | <one-line observation> |

**Review candidate**: <yes/no> (<reason if yes — which criterion(s) < 3>)
**Suggested action**: <NONE | SPLIT | MERGE_WITH_<other> | REWRITE | DEPRECATE>
— <one-line rationale>

<... one block per skill, in alphabetical order by skill name ...>

## Patterns observed

<Cross-skill themes the agent noticed while scoring. Examples:
"5 skills lack an explicit 'What this does NOT cover' section — consider a
one-time template upgrade pass." or "All review-team skills score 5/5 across
the board — the parallel-3-agent pattern is consistently applied.">
```

### 4. Present, do not act

After writing the report, print to the conversation:

- The report file path (absolute).
- The top 3 review candidates ranked by lowest single-criterion score
  (ties broken by lowest sum of scores).
- This reminder, verbatim: *"No skill file was modified. To act on these
  suggestions, run `/learn-refine` (for merges) or edit skills manually."*

Do not propose a follow-up action yourself, do not invoke `/learn-refine`, do
not write to any file other than the report.

## What this does NOT cover

- This skill does **NOT** modify any `.claude/skills/<name>/SKILL.md`. It is
  read-only on the skill collection.
- This skill does **NOT** call `/learn-refine` or `/learn-extract`. Those are
  separate user-driven entry points.
- This skill does **NOT** score memory entries. Memory uses a different rubric
  (see `.claude/rules/skill-quality.md` § Triagem rules — refinability is
  tolerated lower for memory) and a different audit path.
- This skill does **NOT** delete or move files. Anti-deletion principle
  (REQ-10 of `.specs/learning-loop-harness.md`).
- This skill does **NOT** rerun itself or schedule a follow-up. One invocation
  = one report.

## Rules

- ALWAYS run `learn audit-skills-prep` first. The agent does NOT read
  `SKILL.md` files directly; it consumes the helper's JSON so the audit is
  reproducible and stable across runs.
- ALWAYS produce a report file, even if zero skills were found. An empty input
  produces a valid report stating "0 skills audited".
- ALWAYS cite the rubric criteria in the report's preamble or treat the
  embedded quoted block in this SKILL.md as the source of truth — never
  paraphrase the criteria at scoring time.
- NEVER apply suggestions automatically. The user must explicitly invoke
  `/learn-refine` (for merges) or edit skills manually (for rewrites).
- NEVER cite the body of any skill verbatim in the report — only its scores
  and short observations — to avoid copying sanitized-but-sensitive content
  into a tracked report.
- NEVER overwrite a prior audit report. Append a `-<NN>` suffix when a same-day
  collision happens (anti-deletion, REQ-10).

## Related skills

- `/learn-refine` — to actually merge skill pairs flagged here as candidates.
- `/learn-recall` — to inspect skills retrieved by the harness day-to-day.
- `/learn-extract` — to feed new candidates into the loop (this skill audits
  what already exists; `/learn-extract` proposes new ones).
