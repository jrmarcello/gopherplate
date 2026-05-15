# Skill Quality Rubric

> Source: Lucas Montano's analysis of the Hermes Agent
> (https://youtu.be/7R-LAADt6rY, 8:13–8:40, with FTS5 mention at 10:24).
> Applied as: REQ-6 of `.specs/learning-loop-harness.md`.
> Consumers: `/learn-extract`, `/learn-refine`, `/learn-audit-skills`.

This file is the **canonical rubric** for deciding whether a candidate pattern
deserves to become a Claude Code skill in this project, and for auditing skills
that already exist. The four criteria below come from Lucas Montano's breakdown
of the Hermes Agent's skill-memory design — they have been tuned to the
specifics of this repository (Clean Architecture Go microservice template, SDD
workflow, Skill Memory indexed via SQLite FTS5).

## When to apply

- Deciding whether a candidate pattern surfaced during `/learn-extract` should
  graduate to a new skill (etapa 3 of the learning loop).
- Comparing skills for potential merge or split during `/learn-refine`
  (etapa 4).
- Auditing existing skills via `/learn-audit-skills` (one-shot REQ-25).
- Authoring or modifying a skill manually — the same bar applies whether the
  proposal comes from the loop or from a human.

## The four criteria

Each criterion is scored independently 1–5 against the **anchor examples**
below. A skill that scores ≥ 4 on ALL FOUR is "ready". Anything 1–2 in any
criterion is a candidate for rework, demotion to a memory entry, or rejection.

The four criteria are derived literally from the Hermes Agent description and
must remain in this order — downstream skills (`/learn-extract`,
`/learn-refine`, `/learn-audit-skills`) cite this section as a block and assume
the numbering is stable.

### 1. Foco em repetição (Focus on repetition)

A good skill automates work that lives inside the 80% repetitive part of the
daily routine. It does **not** try to optimize for one-off rituals or
exception-handling. If the skill would be invoked once a quarter (or once), it
probably belongs in a runbook, a doc, or a memory entry — not in
`.claude/skills/`.

- **Score 5** — invoked routinely (multiple times per week, or on every feature
  / every PR). Central to how work actually flows through the repository. The
  skill replaces a sequence the user would otherwise type by hand every time.
- **Score 3** — invoked occasionally (weeks apart) but still recurring; the
  workflow is real but the cadence is lower. The skill is useful, just not
  daily.
- **Score 1** — invoked rarely or speculatively. The skill exists "in case" the
  scenario shows up. Maintenance cost will dominate value.

#### Anchor examples

- **5 (positive)**: `/spec` (`.claude/skills/spec/SKILL.md`) and `/ralph-loop`
  (`.claude/skills/ralph-loop/SKILL.md`) — the SDD pair runs on every non-trivial
  feature; `/validate` (`.claude/skills/validate/SKILL.md`) runs before every
  commit and inside the Stop hook.
- **1 (negative)**: A hypothetical "publish a Q4 OKR review report" skill —
  used once per quarter, not part of the 80% repetitive workflow. Belongs in a
  runbook, not in `.claude/skills/`.

### 2. Anti-generalização (Anti-generalization)

The agent works best with **narrowly delimited** instructions. A good skill has
a sharp scope, a small number of well-named inputs, and a clear stopping
condition. Vague catch-all skills like "plan everything based on the last PRs"
should be rejected — they ask the model to infer scope at runtime, which is
exactly when the model fails most.

- **Score 5** — single, explicit purpose. Argument shape is well-typed
  (subcommand, file path, short string). Acceptance criteria for "done" are
  obvious from the SKILL.md.
- **Score 3** — clear purpose but with some ambiguity at the edges (e.g. two
  closely related sub-flows blended into one skill). Could be split without
  losing meaning.
- **Score 1** — open-ended, asks the agent to "plan", "design", or "decide"
  without bounding the deliverable. Title is a verb in the imperative with no
  noun ("just figure it out").

#### Anchor examples

- **5 (positive)**: `/migrate` (`.claude/skills/migrate/SKILL.md`) — strictly
  scoped to Goose migrations with explicit `create | up | down | status`
  subcommands; `/new-endpoint` (`.claude/skills/new-endpoint/SKILL.md`) — fixed
  inputs `<method> <path> <description>` and a fixed Clean Architecture target
  layout.
- **1 (negative)**: A skill titled "do the architecture" or "plan everything
  for the team" — no bounded deliverable, no argument shape, no stopping
  condition. The agent would have to invent the scope each call.

### 3. Modularidade (Modularity)

Skills are stored as individual Markdown files indexed in **Skill Memory**, and
the agent retrieves them via **FTS5** full-text search (Hermes Agent, 10:24). A
modular skill is one that the FTS5 ranker can confidently identify and that the
agent can invoke in isolation without dragging in side-cars. Each skill should
be self-contained: a single SKILL.md whose frontmatter and named sections give
the search index strong signal.

- **Score 5** — single SKILL.md with explicit frontmatter (`name`,
  `description`, optional `argument-hint`, `user-invocable`). Named sections
  (`## Phases`, `## When to use`, `## Workflow`) carry the terminology a user
  would actually search for. No hidden coupling to other skills.
- **Score 3** — mostly self-contained but borrows from another skill's
  conventions implicitly (e.g. assumes the reader already knows what
  `/validate` does). Still indexable, but FTS5 ranking will be weaker because
  the terminology is diluted.
- **Score 1** — spread across multiple files, or relies on reading two
  unrelated skills to make sense, or has a frontmatter so generic that FTS5
  cannot distinguish it from siblings.

#### Anchor examples

- **5 (positive)**: `/spec` and `/ralph-loop` — each is a single SKILL.md with
  filled frontmatter (name, description, argument-hint, user-invocable) and
  named sections (`## Phases`, `## Discipline Checkpoints`) that FTS5 ranks
  confidently for queries like "spec review" or "ralph parallel batches".
  `/spec-review` (`.claude/skills/spec-review/SKILL.md`) explicitly carves out
  its boundary against `/ralph-loop`'s inline self-review — making the two
  separable for the index.
- **1 (negative)**: A skill scattered across three Markdown files with implicit
  dependencies between them, or a skill whose body says "see the other skill
  for details" without naming it. FTS5 cannot rank what isn't in the file.

### 4. Refinabilidade (Refinability)

A good skill participates in a refinement cycle: it can be edited, merged with
a sibling, or split into two without rewriting its prose from scratch.
Refinability comes from explicit boundaries — a skill that states *what it
covers* and *what it does NOT cover* lets the loop detect overlap with
neighbours and act on it. Free-form anecdote, however accurate, is fragile
under merge.

- **Score 5** — explicit boundary section ("What this covers" / "What this
  does NOT cover", or an equivalent scoping paragraph). Phases or workflow
  steps are numbered and addressable. Easy to diff against a sibling skill.
- **Score 3** — boundaries are implicit but the structure is clean enough to
  reason about. Merging would require some rewording but no structural rework.
- **Score 1** — content is mostly free-form prose with no explicit scope
  statement. Every refinement attempt risks breaking the narrative. Merging
  with a sibling would force a full rewrite.

#### Anchor examples

- **5 (positive)**: `/fix-issue` (`.claude/skills/fix-issue/SKILL.md`) — numbered
  workflow steps (1. Understand, 2. Plan, 3. Implement, 4. Test, 5. Validate)
  make every phase individually addressable. `/spec-review` opens with an
  explicit "distinct from the inline self-review that `/ralph-loop` does" —
  the kind of boundary statement that survives merges and splits.
- **1 (negative)**: A skill whose body is a single block of free-form prose
  describing "everything you might want to do with X", with no scope statement
  and no addressable sub-sections. Any refinement risks contradicting an
  unwritten assumption elsewhere in the prose.

## Scoring methodology (for the audit skill)

When `/learn-audit-skills` scores existing skills, it:

1. Reads frontmatter + body via `learn audit-skills-prep`.
2. For each skill, assigns a 1–5 score per criterion based on the anchor
   examples above. The anchors are the calibration — do not invent new
   reference points at audit time.
3. Records **observations**, not prescriptions. The report is a starting
   point for the user, never an auto-apply list of edits.
4. Flags any skill that scores < 3 on any criterion as a "review candidate".

## Triagem rules (for /learn-extract and /learn-refine)

These rules turn the four scores into a binary outcome the loop can act on:

- A candidate pattern that scores ≥ 4 on **all four** criteria → emit a new
  skill under `.claude/skills/<name>/SKILL.md`.
- A candidate that scores ≥ 4 on criteria 1, 2, 3 but < 4 on criterion 4
  (refinability) → emit as a **memory entry** instead. Memory tolerates lower
  refinability; skills do not.
- A candidate that scores < 4 on criterion 1 (repetição) or criterion 2
  (anti-generalização) → **discard**, with the rationale recorded in the
  `decisions` table so the loop does not re-propose the same pattern.
- A candidate that scores < 4 only on criterion 3 (modularidade) → propose a
  **split** (or a clearer boundary) before emitting. Do not emit a low-modularity
  skill — FTS5 cannot rank what it cannot disambiguate.

## Notes for human authors

When you write a skill by hand, run the four-criterion check on yourself before
committing the file. The loop will eventually audit it anyway, and rejecting
your own draft is cheaper than rewriting it after `/learn-audit-skills` flags
it. The anchor examples above (`/spec`, `/ralph-loop`, `/validate`, `/migrate`,
`/new-endpoint`, `/fix-issue`, `/spec-review`) are the calibration set —
compare your draft against them, not against an abstract ideal.
