# Fragment: TASK-33C → CLAUDE.md

## Intent

Surface the 5 learning-loop skills + 3 hooks to Claude Code's project
instructions so the agent sees them when reasoning about the harness. Also
introduce a short subsection explaining the learning loop's role.

## Target

CLAUDE.md

## Additions

### Section: Skills table

Append rows to the existing `### Skills (slash commands)` table (after the
`/spec-review` row):

```markdown
| `/learn-extract` | Triage `candidates.jsonl` into new skills, memory entries, updates, or discards | After `/learn-nudge` signals threshold reached, or manually post-extract |
| `/learn-refine` | Propose merges/deprecations for similar skills/memory (presents diff, waits for approval) | After `/learn-extract` flags overlap, or when /learn-audit-skills surfaces candidates |
| `/learn-nudge` | Periodic autoavaliação — surfaces deprecation candidates by TTL, proposes consolidations, resets the counter | Triggered automatically after N spec DONEs, or manually |
| `/learn-recall <query>` | Manual FTS5 retrieval over skills/memory/patterns with explicit filters | When you want to consult the KB without the auto-injected hook |
| `/learn-audit-skills` | One-shot non-prescriptive audit of every skill against the rubric in `.claude/rules/skill-quality.md` | After a batch of skill changes or before tagging a release |
```

### Section: Hooks bullets

Append to the existing `### Hooks` bullet list (after `Stop` hook entry):

```markdown
- **Stop** — `stop-learn.sh`: records spec-DONE events into the learning-loop store; surfaces a non-blocking `Learning nudge due` advisory when the counter crosses `NUDGE_THRESHOLD`. Best-effort: always exits 0.
- **UserPromptSubmit** — `user-prompt-submit-recall.sh`: queries the learning-loop FTS5 index for the incoming prompt; if matches above `RECALL_MIN_SCORE` exist, injects a `<system-reminder>` listing the relevant skill/memory paths. Wraps `learn recall` in `timeout 2` to never block the prompt.
- **PostToolUse[Edit|Write]** — `reindex-learning.sh`: incrementally re-indexes `.claude/skills/<name>/SKILL.md` or `memory/*.md` after edits, so retrieval stays fresh. Best-effort.
- **Sourced helper** — `learn-hook-helpers.sh`: binary lookup (with asdf shim support), structured JSON logging to `.claude/learning/learn.log`, safe jq, db-path resolution. All hooks above source it.
```

### Section: Learning loop (new subsection in Claude Code Resources)

Insert a new subsection AFTER the existing `### Hooks` section, before
`### Execution Directives`:

```markdown
### Learning loop

Closed-loop knowledge system inspired by the Hermes Agent (see
[docs/guides/learning-loop.md](docs/guides/learning-loop.md)). Five stages:

1. **Task completion** — `stop-learn.sh` records `events` when a spec hits
   DONE.
2. **Pattern extraction** — `learn extract` mines transcripts / specs / git
   / memory into `candidates.jsonl`, deterministic.
3. **Skill creation** — `/learn-extract` triage applies the rubric in
   `.claude/rules/skill-quality.md` to each candidate, decides
   new-skill / new-memory / update / discard, records every decision.
4. **Refinement** — `/learn-refine` uses FTS5 + edit distance to find
   merge candidates; presents diff, waits for approval; movements go to
   `_deprecated/` (REQ-10 anti-deletion).
5. **Periodic nudge** — `/learn-nudge` fires when the spec-completion
   counter crosses threshold; proposes TTL-based deprecations and
   consolidations. Counter reset is deterministic (binary), not LLM.

Closure: `user-prompt-submit-recall.sh` queries the FTS5 index and injects
the top matches as a `<system-reminder>` on every user prompt. Manual
counterpart: `/learn-recall <query>`. Usage tracked by `learn track-use`
called from the hook, feeding TTL decisions in stage 5.

Storage: SQLite + FTS5 at `.claude/learning/db.sqlite` (gitignored).
Skills/memory/decisions are versioned; the index is local-only.
Privacy: every parser sanitizes secrets in-memory before any write
(AWS, OpenAI/Anthropic/GitHub/Slack tokens, SSH paths, `.env` lines).

Binary: `tools/learn/` (separate go.mod, pure-Go via `modernc.org/sqlite`).
Operational targets: `make learn-build / learn-setup / learn-reindex /
learn-stats / learn-smoke / learn-lint / learn-test`. See
[docs/harness.md § Learning loop tooling](docs/harness.md).
```

## Notes

- The merge task (TASK-37) is a text patch — anchors are the existing
  H3-level section headings.
- Anchor strings (must match verbatim in CLAUDE.md):
  - `### Skills (slash commands)` — append rows to its table
  - `### Hooks` — append bullets to its list
  - `### Execution Directives` — insert the new `### Learning loop` subsection
    BEFORE this heading (so logical order is: Skills → Agent Teams → Hooks →
    Learning loop → Execution Directives).
- Keep the `When to use` column phrasing consistent with neighbouring rows
  (concise, "when X happens" or "after Y" style).
- Memory of the audit non-destructive constraint (REQ-25) is captured in the
  `/learn-audit-skills` table row's "what" column.
