---
name: example-skill-name
description: One-line description (used as fallback summary in FTS5 results)
argument-hint: "<optional argument shape>"
user-invocable: true
learning_provenance:
  - "placeholder:human-authored"
created_at: 2026-05-14T12:00:00Z
last_reviewed_at: 2026-05-14T12:00:00Z
---

<!--
Canonical template for new skills (human-authored or `/learn-extract` generated).
Authoring bar: a skill written from this template MUST score >= 4 on all four
criteria of the rubric at `.claude/rules/skill-quality.md` (REQ-6).

Frontmatter contract (REQ-7, enforced by `learn validate-skill`):
  - name                 — slug matching the directory name and the H1 below.
  - description          — single line; FTS5 fallback summary.
  - learning_provenance  — list of candidate signatures that originated this
                            skill. For human-authored skills, use a single
                            placeholder entry such as `"human:<author-or-date>"`
                            (the validator rejects empty lists).
  - created_at           — RFC3339 timestamp of first authoring.
  - last_reviewed_at     — RFC3339 timestamp bumped by `/learn-refine` or
                            `/learn-audit-skills`.

Optional but recommended:
  - argument-hint        — shape of the invocation argument(s).
  - user-invocable       — `true` for slash-command-exposed skills; `false`
                            for internal-only skills consumed by other skills.
-->

# /example-skill-name <argument-hint>

<One-paragraph summary of what this skill does and when to invoke. Lead with the
*deliverable*, not the *activity* — e.g. "Produces a Goose migration pair (Up +
Down) for a schema change" beats "Helps you with migrations".>

## What this covers

- <Concrete task 1 this skill handles.>
- <Concrete task 2 this skill handles.>
- <Concrete task 3 this skill handles.>

## What this does NOT cover

- <Explicit out-of-scope item 1 — points to the sibling skill that handles it.>
- <Explicit out-of-scope item 2 — points to the sibling skill that handles it.>

> This pair of sections ("What this covers" / "What this does NOT cover") is
> load-bearing for **criterion 4 (Refinability)** of the rubric — siblings use
> it to detect overlap and decide between merge and split.

## Example

```text
/example-skill-name <example args>
```

<Brief walkthrough of the expected interaction: what the user sees, what files
or artifacts are produced, what the stopping condition looks like.>

## Workflow

### 1. <Phase name>

<Concrete steps the skill performs in this phase. Cite tools/files by absolute
path or canonical name when possible — FTS5 indexes the body, so terminology
matters.>

### 2. <Phase name>

<Concrete steps. Each phase should have an observable output (a file written, a
command run, a message presented to the user).>

### 3. <Phase name>

<Aim for 2–5 phases. More than 5 is a smell — consider splitting the skill.>

## Rules

- <Hard constraint 1 (e.g. "Never modify Requirements during IN_PROGRESS").>
- <Hard constraint 2 (e.g. "Always run validation before presenting results").>
- <Constraint about when NOT to take an action (e.g. "Do not auto-commit; wait
  for explicit user approval").>

## When NOT to use

<One paragraph explaining when this skill is the wrong tool, naming the sibling
skill(s) that fit the alternative cases. This is the prose complement to the
"What this does NOT cover" bullets — together they make the boundary explicit
enough to survive merge/split refinement.>

## Related skills

- `/<sibling-skill>` — <one-line relationship (e.g. "runs before this skill to
  produce the input spec").>
- `/<sibling-skill>` — <one-line relationship.>

## How this template was derived

This template encodes the four-criterion bar from
[`.claude/rules/skill-quality.md`](../../rules/skill-quality.md) (REQ-6 of
`.specs/learning-loop-harness.md`):

1. **Foco em repetição** — the "When NOT to use" section forces authors to
   declare the invocation cadence, and to demote one-off rituals to memory
   entries.
2. **Anti-generalização** — `argument-hint` + the "What this covers" bullets
   keep scope sharp; numbered phases give an explicit stopping condition.
3. **Modularidade** — single SKILL.md with named sections (`## Workflow`,
   `## Rules`, `## When NOT to use`) carries the terminology FTS5 ranks on.
4. **Refinabilidade** — the "What this covers" / "What this does NOT cover"
   pair plus "Related skills" makes boundaries explicit and addressable, so
   `/learn-refine` and `/learn-audit-skills` can act without rewriting prose.

Frontmatter required fields are enforced by `learn validate-skill` (REQ-7);
running it against this template returns exit code 0.
