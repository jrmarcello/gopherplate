---
name: learn-recall
description: Manual retrieval from the learning loop knowledge base — search skills, memory, and patterns via FTS5 with explicit filters. Counterpart to the automatic UserPromptSubmit hook. Tracks usage.
argument-hint: "<query> [--kind=skill|memory|pattern] [--since=7d] [--max=10]"
user-invocable: true
learning_provenance:
  - "spec:learning-loop-harness:REQ-14"
  - "spec:learning-loop-harness:REQ-16"
  - "spec:learning-loop-harness:REQ-17"
created_at: 2026-05-14T20:00:00Z
last_reviewed_at: 2026-05-14T20:00:00Z
---

# /learn-recall <query> [--kind=skill|memory|pattern] [--since=7d] [--max=10]

Explicit manual retrieval from the Learning Loop knowledge base (REQ-16). Use
when the automatic `user-prompt-submit-recall.sh` hook didn't surface what
you wanted, or when you need to scope a search to a specific kind, recent
time window, or higher result count than the hook's default (`--top-k=3`).
Increments usage tracking via `learn track-use` for surfaced paths so TTL
deprecation in `/learn-nudge` reflects real access (REQ-17).

## What this covers

- Manual FTS5 BM25 retrieval over `skill_index`, `memory_index`, `pattern_fts`.
- Explicit filters: `--kind`, `--since`, `--max`, `--min-score`.
- Pretty-printing matches in the chat (one line per match: path + score +
  summary excerpt + `Read` instruction).
- Tracking usage on every surfaced path (REQ-17 closure: tracking is by
  *presentation*, not by Claude's downstream consumption).

## What this does NOT cover

- Automatic retrieval at prompt-submit time — that's the
  `user-prompt-submit-recall.sh` hook. No agent involvement, no skill.
- Modifying skills/memory — out of scope (use `/learn-extract`,
  `/learn-refine`).
- Reading the FULL body of any surfaced file — this skill surfaces paths +
  summaries only. The user (or Claude on the next turn) uses the `Read` tool
  on the path of interest.
- Pattern extraction — Stage 2 (`learn extract`).
- Producing audit reports — `/learn-audit-skills`.

## Workflow

### Phase 1 — Parse intent

Read the user's `<query>` and any filters. Defaults:

- `--max=10` (wider than the automatic hook's `--top-k=3` because manual
  invocation benefits from a broader view).
- `--kind` unset → all three kinds (skill / memory / pattern).
- `--since` unset → no time filter.
- `--min-score` from `config.yml` (default 0.4).

Validation errors (e.g. `--kind=invalid`, `--since=abc`, `--max=-1`) surface
as `learn`'s `*UsageError` — fix the filter and re-invoke.

### Phase 2 — Run the deterministic query

```bash
learn recall \
  --format json \
  --prompt "<query>" \
  --top-k <max> \
  --max-tokens 2000 \
  --kind <kind-if-set> \
  --since <since-if-set> \
  --db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"
```

The binary returns a JSON array of `{kind, path, score, summary}`. Parse it.

### Phase 3 — Present results

For each match, print:

```text
Result <N>: <path> (<kind>, score=<X.XX>)
  <summary excerpt, sanitized, ≤120 chars>

  Read full content with: Read <path>
```

If no results: print *"No matches above min-score (<X>); try a broader query
or relax filters."*

### Phase 4 — Track usage (REQ-17 closure)

For every path surfaced:

```bash
learn track-use \
  --paths "<comma-separated-paths>" \
  --db-path "..."
```

Best-effort: `track-use` failures are logged but never block the user output.
Even for paths that aren't in the index (unlikely, since they came from
recall), `track-use` exits 0 silently and logs a structured warn (REQ-17:
*tracking best-effort — não bloqueia retrieval se a heurística falhar*).

## Rules

- ALWAYS call `learn track-use` in Phase 4 (REQ-17). The skill being invoked
  counts as "presented to the agent" for TTL purposes — the same accounting
  the automatic hook does after injection.
- NEVER read full file bodies inside this skill. Surface paths + summaries
  only; the user / next-turn Claude does the `Read` if relevant.
- NEVER modify the KB — this skill is strictly read-only.
- Respect `--since` and `--kind` strictly. The binary validates them; on
  `*UsageError`, surface the validation error to the user verbatim.
- The system-reminder template is for the automatic hook only. This skill
  uses `--format json` and reformats for readable chat output.

## When NOT to use

- For broad context every prompt — the automatic hook covers that with
  `--top-k=3`. Use this skill only when you need scoping (specific kind,
  time window, or larger result set).
- For pattern counts and KB health — `learn stats` (or no skill at all) is
  the right tool.
- For deciding on a merge — that's `/learn-refine`, which has its own
  similarity workflow via `learn similar`.

## Related skills

- `/learn-extract` — Stage 3. New skills produced by it are immediately
  recall-able through this skill.
- `/learn-refine` — Stage 4. Uses a different similarity API
  (`learn similar`) but the same FTS5 backbone.
- `/learn-nudge` — Stage 5. Reads usage tracking populated by this skill +
  the hook to compute TTL candidates.
- `/learn-audit-skills` — one-shot rubric review, complementary information.

## Automatic counterpart

For every-prompt context retrieval, the hook
[`.claude/hooks/user-prompt-submit-recall.sh`](../../hooks/user-prompt-submit-recall.sh)
runs `learn recall --format=system-reminder` with `--top-k=3
--max-tokens=500`. It injects the REQ-14a template into the prompt and calls
`learn track-use` afterwards — exactly the same closure pattern as this
manual skill, just at every prompt boundary instead of on-demand.
