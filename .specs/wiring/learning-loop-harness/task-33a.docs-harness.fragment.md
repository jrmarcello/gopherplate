# Fragment: TASK-33B → docs/harness.md

## Intent

Register every artifact introduced by the learning-loop-harness spec into the
canonical harness inventory (Fowler taxonomy). Adds rows to the existing
Skills, Hooks, and Documental-guides tables; introduces a new section for the
`tools/learn` tooling binary; closes the "no auto-evolving harness" gap.

## Target

docs/harness.md

## Additions

### Section: Skills

Append to the existing `### Skills (slash commands)` table, preserving
alphabetical ordering by Artifact name:

```markdown
| `/learn-audit-skills` | sensor | I | meta | review-time | [.claude/skills/learn-audit-skills/](../.claude/skills/learn-audit-skills/) |
| `/learn-extract` | sensor | I | meta+maint | review-time | [.claude/skills/learn-extract/](../.claude/skills/learn-extract/) |
| `/learn-nudge` | sensor | I | meta | review-time | [.claude/skills/learn-nudge/](../.claude/skills/learn-nudge/) |
| `/learn-recall` | guide | I | meta | on-demand | [.claude/skills/learn-recall/](../.claude/skills/learn-recall/) |
| `/learn-refine` | sensor | I | meta+maint | review-time | [.claude/skills/learn-refine/](../.claude/skills/learn-refine/) |
```

### Section: Hooks

Append to the existing `### Hooks` table:

```markdown
| Stop learn (Stop) | guide | C | meta | stop-hook | [.claude/hooks/stop-learn.sh](../.claude/hooks/stop-learn.sh) |
| UserPromptSubmit recall (UserPromptSubmit) | guide | I | meta | on-prompt | [.claude/hooks/user-prompt-submit-recall.sh](../.claude/hooks/user-prompt-submit-recall.sh) |
| Reindex learning (PostToolUse) | sensor | C | meta | on-edit | [.claude/hooks/reindex-learning.sh](../.claude/hooks/reindex-learning.sh) |
| Hook helpers (sourced) | guide | C | meta | on-source | [.claude/hooks/learn-hook-helpers.sh](../.claude/hooks/learn-hook-helpers.sh) |
```

### Section: Documental guides

Append to the existing `### Documental guides` table (preserve order by name):

```markdown
| Skill quality rubric | guide | I | meta | on-read | [.claude/rules/skill-quality.md](../.claude/rules/skill-quality.md) |
| Learning loop guide | guide | I | meta | on-read | [docs/guides/learning-loop.md](guides/learning-loop.md) |
```

### Section: Learning loop tooling

Insert a NEW section right after `### gopherplate CLI (scaffolders)`:

```markdown
### Learning loop tooling

| Command | Type | Exec | Category | Stage | Implementation |
| --- | --- | --- | --- | --- | --- |
| `learn init` | guide | C | meta | scaffold-time | [tools/learn/](../tools/learn/) |
| `learn complete-task` | guide | C | meta | stop-hook | [tools/learn/](../tools/learn/) |
| `learn extract` | sensor | C | meta | continuous | [tools/learn/](../tools/learn/) |
| `learn reindex` | sensor | C | meta | on-edit | [tools/learn/](../tools/learn/) |
| `learn similar` | sensor | C | meta | on-demand | [tools/learn/](../tools/learn/) |
| `learn recall` | guide | C | meta | on-prompt | [tools/learn/](../tools/learn/) |
| `learn track-use` | sensor | C | meta | on-prompt | [tools/learn/](../tools/learn/) |
| `learn nudge-tick` | sensor | C | meta | continuous | [tools/learn/](../tools/learn/) |
| `learn stats` | sensor | C | meta | on-demand | [tools/learn/](../tools/learn/) |
| `learn validate-skill` | sensor | C | meta | on-edit | [tools/learn/](../tools/learn/) |
| `learn record-decision` / `learn apply-decision` | guide | C | meta | review-time | [tools/learn/](../tools/learn/) |
| `learn audit-skills-prep` | guide | C | meta | review-time | [tools/learn/](../tools/learn/) |

Reference: [docs/guides/learning-loop.md](guides/learning-loop.md)
```

### Section: Known gaps

Append to the existing `## Known gaps` table:

```markdown
| ~~No closed-loop learning system — harness improvements are 100% manual; patterns surfaced by transcripts/specs/git are never extracted, indexed, or surfaced back to the agent.~~ **Resolved by spec learning-loop-harness** (DONE) — see [tools/learn/](../tools/learn/), the 5 `/learn-*` skills, 3 hooks, and [docs/guides/learning-loop.md](guides/learning-loop.md). | meta | [.specs/learning-loop-harness.md](../.specs/learning-loop-harness.md) |
```

## Notes

- The merge task (TASK-36) is a text patch — anchor matches on existing
  section heading strings. Anchors used:
  - `### Skills (slash commands)` → append rows to this table
  - `### Hooks` → append rows to this table
  - `### Documental guides` → append rows
  - `### gopherplate CLI (scaffolders)` → insert new section AFTER
  - `## Known gaps` → append a row inside the existing table
- Fowler taxonomy classification follows the spec's Context section:
  - Trigger `stop-learn.sh` → `complete-task`: **guide** computacional (feedforward — records data)
  - `extract`, `nudge-tick`: **sensor** computacional (analyzes stored output)
  - Skills agentic (extract/refine/nudge/audit): **sensor** inferencial
  - Retrieval auto-inject (UserPromptSubmit + `/learn-recall`): **guide** inferencial
- `docs/guides/learning-loop.md` is introduced by TASK-40, which runs after this
  merge. The inventory link will dangle until TASK-40 lands — intentional per
  spec's batch ordering.
- Preserve alphabetical sort wherever the target table is already sorted by
  Artifact name.
