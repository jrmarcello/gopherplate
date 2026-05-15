# Learning Loop

Closed-loop knowledge system inspired by the Hermes Agent
([video reference](https://youtu.be/7R-LAADt6rY), 5:40–10:40), implemented as
an extension of the gopherplate harness. The agent's collaboration with this
repo gets demonstrably better over time because patterns surfaced during
day-to-day work get extracted, indexed, and surfaced back to the agent on the
next prompt.

> **Origin**: spec [`.specs/learning-loop-harness.md`](../../.specs/learning-loop-harness.md).
> See also the [harness map](../harness.md) for the full Fowler-taxonomy
> inventory.

## The 5 stages

```text
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ 1. Task         │───>│ 2. Pattern      │───>│ 3. Skill        │
│    Completion   │    │    Extraction   │    │    Creation     │
│  stop-learn.sh  │    │  learn extract  │    │ /learn-extract  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                       │
        ┌──────────────────────────────────────────────┘
        v
┌─────────────────┐    ┌─────────────────┐
│ 4. Refinement   │───>│ 5. Periodic     │
│ /learn-refine   │    │    Nudge        │
│ → _deprecated/  │    │ /learn-nudge    │
└─────────────────┘    └─────────────────┘
        ^                       │
        │                       │
        │   ┌─────────────────┐ │
        └───│  Closure        │<┘
            │  retrieval hook │
            │  /learn-recall  │
            └─────────────────┘
```

1. **Task Completion** — when `/ralph-loop` flips a spec to `DONE`, the Stop
   hook (`stop-learn.sh`) calls `learn complete-task`, which inserts an `events`
   row and atomically increments the `nudge_state.counter`.
2. **Pattern Extraction** — `learn extract` mines the four sources
   (`.specs/*.md` Execution Logs, git log + diffs, Claude Code transcript
   JSONL, memory `*.md`) into a sorted, deterministic `candidates.jsonl`.
   Output is byte-identical across runs with the same inputs.
3. **Skill Creation** — `/learn-extract` (agentic) consumes `candidates.jsonl`,
   applies the rubric in [`.claude/rules/skill-quality.md`](../../.claude/rules/skill-quality.md),
   and routes each candidate to one of four actions: `new-skill`,
   `new-memory`, `update`, `discard`. Each decision lands in the `decisions`
   table for audit.
4. **Refinement** — `/learn-refine` uses `learn similar` (FTS5 BM25 + edit
   distance) to find merge candidates among existing skills/memory. The skill
   presents the diff and waits for **explicit user approval** before applying.
   Approved moves go to `_deprecated/<name>-YYYYMMDDTHHMMSSZ.md` with a
   literal header — anti-deletion principle (REQ-10).
5. **Periodic Nudge** — `learn nudge-tick` calculates TTL-based deprecation
   candidates (`created_at < now - 2×TTL_DAYS` AND `last_used_at < now -
   TTL_DAYS`, inclusive). When the counter from stage 1 crosses
   `NUDGE_THRESHOLD`, the `stop-learn.sh` hook surfaces a
   `Learning nudge due` advisory — the user invokes `/learn-nudge` to walk
   through the candidates. The counter reset is **deterministic** (the binary
   does it via `--reset`), never the LLM.

## Closure — without retrieval, the loop doesn't close

`user-prompt-submit-recall.sh` fires on every user prompt:

1. Reads `prompt` from stdin JSON.
2. Skips if `len(prompt) < MIN_PROMPT_LEN_FOR_RECALL` (default 20).
3. Calls `learn recall --format=system-reminder --prompt "<text>"` wrapped in
   `timeout 2` — never blocks the user.
4. If matches exist with `score ≥ RECALL_MIN_SCORE` (default 0.4), prints the
   REQ-14a system-reminder template to stdout (Claude Code injects it as
   context).
5. Calls `learn track-use --paths <comma-list>` to bump `usage_count` and
   `last_used_at` for every path it surfaced — feeds stage 5's TTL math.

Manual counterpart: `/learn-recall <query>` with filters
(`--kind=skill|memory|pattern`, `--since=<duration>`, `--max=N`,
`--format=json`).

## Storage

SQLite + FTS5 at `.claude/learning/db.sqlite` (gitignored — local per dev).
Connection always opens with `PRAGMA journal_mode=WAL` and
`PRAGMA busy_timeout=5000` so concurrent hook invocations don't fail with
`SQLITE_BUSY`.

```text
.claude/learning/
├── db.sqlite            # gitignored (privado por dev)
├── config.yml           # gitignored
└── learn.log            # gitignored

.claude/skills/          # versionado — beneficia o time
├── <name>/SKILL.md
└── _deprecated/<name>-<timestamp>.md

memory/                  # versionado — feedback curado
├── <name>.md
├── _deprecated/<name>-<timestamp>.md
└── MEMORY.md            # index (curado pelo /learn-extract)

.specs/reports/          # versionado — relatórios de /learn-audit-skills
└── skill-audit-<YYYY-MM-DD>.md
```

### Schema (high-level)

- **events** — one row per spec DONE: `spec_path`, `session_id`,
  `completed_at`, `commit_sha`, `tasks_count`, `outcome`.
- **patterns** — extracted n-grams (`kind`, `signature`, `frequency`, `score`,
  `first_seen_at`, `last_seen_at`). Unique by `signature`.
- **candidates** — extracted but not yet decided (`pattern_id`,
  `contexts_json`, `score`, `created_at`).
- **decisions** — audit trail for stages 3 & 4 (`candidate_signature`,
  `action`, `target_path`, `rationale`, `diff`, `decided_at`).
- **skill_index** / **memory_index** — full-text indexed bodies (path is PK).
- **skill_fts** / **memory_fts** / **pattern_fts** — FTS5 virtual tables with
  triggers that sync on INSERT/UPDATE/DELETE.
- **skill_usage** / **memory_usage** — `usage_count`, `last_used_at` keyed by
  path. Bumped by `learn track-use`.
- **nudge_state** — singleton row with `counter` and `last_nudge_at`.

## Privacy

Every parser runs the input through `internal/sanitize` **in memory before
any disk write**. Default patterns (configurable via
`config.secret_patterns`):

- AWS access keys: `AKIA[0-9A-Z]{16}`
- OpenAI / OpenAI project / Anthropic tokens: `sk-(ant|proj)?-?[A-Za-z0-9_-]{20,}`
- GitHub PAT (classic): `ghp_[A-Za-z0-9]{36}`
- GitHub fine-grained: `github_pat_[A-Za-z0-9_]{82}`
- Slack tokens: `xox[abprs]-[A-Za-z0-9-]+`
- SSH paths: `/\.ssh/[A-Za-z0-9_.-]+`
- `.env` lines: `^[A-Z][A-Z0-9_]*=.+$` (line-anchored)

Matches are replaced with `<REDACTED:<kind>>`. The policy is **conservative**
— we accept false positives in exchange for zero false negatives of known
classes.

## Module structure

The `tools/learn` module uses a flat single-package layout (`package learn` at
the root). It was refactored from a 14-package Clean-Architecture style on
2026-05-15 (see [`.specs/refactor-tools-learn-flat-layout.md`](../../.specs/refactor-tools-learn-flat-layout.md)
for the full rationale and migration plan). The rationale in short: a CLI binary
has no layer boundaries to defend; the previous layout was a stylistic mirror of
the main server's conventions without delivering the boundary protection that
Clean Architecture earns in domain-rich code. All files now live directly under
`tools/learn/` (e.g. `tools/learn/store.go`, `tools/learn/sanitize.go`), and the
binary entrypoint stays at `tools/learn/cmd/learn/main.go`.

## Operational commands

```bash
make learn-build      # Compile bin/learn (root-relative; gitignored)
make learn-setup      # Initialize .claude/learning/ (db + config)
make learn-reindex    # Full re-index of skills + memory FTS5
make learn-stats      # JSON snapshot of the KB
make learn-smoke      # End-to-end fixture smoke (TC-E2E-06)
make learn-lint       # golangci-lint over tools/learn module
make learn-test       # go test over tools/learn module
```

Direct CLI usage:

```bash
bin/learn init --dir .claude/learning
bin/learn complete-task --spec .specs/foo.md --session "<id>"
bin/learn extract --specs-dir .specs --transcripts-dir ~/.claude/projects/<dir>
bin/learn similar --skill .claude/skills/<name>/SKILL.md
bin/learn recall --prompt "search query"
bin/learn nudge-tick               # list candidates
bin/learn nudge-tick --reset       # reset counter (called by /learn-nudge)
bin/learn validate-skill .claude/skills/<name>/SKILL.md
bin/learn audit-skills-prep --skills-dir .claude/skills
bin/learn stats
```

## Skills (slash commands)

| Skill | Stage | Role |
| --- | --- | --- |
| `/learn-extract` | 3 | Triage `candidates.jsonl` |
| `/learn-refine` | 4 | Propose merges / deprecations |
| `/learn-nudge` | 5 | Periodic autoavaliação |
| `/learn-recall` | closure | Manual retrieval |
| `/learn-audit-skills` | one-shot | Audit all skills against the rubric |

All skills cite the rubric in
[`.claude/rules/skill-quality.md`](../../.claude/rules/skill-quality.md) as a
literal block so the LLM has the criteria in-context, not just referenced.

## Debugging

```bash
make learn-stats                           # Counts + last-extract / last-nudge timestamps
cat .claude/learning/learn.log | tail -20  # Structured JSON logs from hooks
bin/learn reindex --verbose                # (planned) verbose reindex
sqlite3 .claude/learning/db.sqlite ".tables" # Inspect schema
sqlite3 .claude/learning/db.sqlite "SELECT * FROM decisions ORDER BY decided_at DESC LIMIT 10"
```

If a hook fails silently: every hook logs warns to `.claude/learning/learn.log`
in JSON format. Grep for `event=binary_not_found`, `event=binary_nonzero_exit`,
or `event=broken_link` to triage.

## Rollback

Anti-deletion means recovery is trivial:

1. Identify the deprecated artifact:

   ```bash
   ls .claude/skills/_deprecated/
   ls memory/_deprecated/
   ```

2. Inspect the header to see the merge target:

   ```bash
   head -3 .claude/skills/_deprecated/<name>-<timestamp>.md
   # > Deprecated YYYY-MM-DD by /learn-refine: merged into <target-path>
   ```

3. Move it back and remove the header line manually:

   ```bash
   mv .claude/skills/_deprecated/<name>-<timestamp>.md \
      .claude/skills/<name>/SKILL.md
   $EDITOR .claude/skills/<name>/SKILL.md   # remove the deprecation header
   make learn-reindex
   ```

4. Optionally, also remove the `apply-decision` row from the database — or
   simply leave it as audit history.

## Trade-offs vs Hermes Agent

This is a pragmatic adaptation, not a 1:1 port:

| Aspect | Hermes | This implementation |
| --- | --- | --- |
| Storage | SQLite + FTS5 (matches us) | SQLite + FTS5 (`modernc.org/sqlite`, pure Go) |
| Stage automation | More end-to-end automated | Explicit approval gates at stages 3, 4, 5 (anti-deletion + user-in-the-loop by default) |
| Scope | General agent learning | Project-scoped — feeds back into one repo's harness |
| Secret handling | Implementation-specific | Sanitize in-memory before any write; conservative policy |
| Counter / TTL | Implementation-specific | Counter increment atomic SQL; reset deterministic (binary, not LLM) |

## Trouble-shooting recipes

**"Nudge advisory never fires"** — confirm `nudge_state.counter` is being
bumped:

```bash
sqlite3 .claude/learning/db.sqlite "SELECT counter FROM nudge_state"
```

If 0 after multiple spec DONEs, check `stop-learn.sh` log entries.

**"Retrieval injects nothing"** — confirm the index has entries:

```bash
sqlite3 .claude/learning/db.sqlite "SELECT count(*) FROM skill_index"
sqlite3 .claude/learning/db.sqlite "SELECT count(*) FROM memory_index"
```

If both zero, run `make learn-reindex`.

**"Hook silently fails"** — every hook logs structured JSON warns:

```bash
tail -20 .claude/learning/learn.log | jq .
```

## Manual smoke walkthrough

```bash
make learn-setup
# create a fake spec with Status: DONE in .specs/test.md
bin/learn complete-task --spec .specs/test.md --session "test-1" \
  --db-path .claude/learning/db.sqlite
bin/learn extract --specs-dir .specs \
  --db-path .claude/learning/db.sqlite \
  --out /tmp/candidates.jsonl
bin/learn reindex --db-path .claude/learning/db.sqlite
bin/learn recall --prompt "test refactoring user use case" \
  --db-path .claude/learning/db.sqlite \
  --format json
bin/learn stats --db-path .claude/learning/db.sqlite
```

Each command should exit 0 with the expected output shape.
