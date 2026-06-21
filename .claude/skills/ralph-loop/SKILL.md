---
name: ralph-loop
description: Autonomous single-run execution of an approved SDD spec — parallel via worktrees, self-reviewed, presented for approval before commit
argument-hint: "<spec-file-path>"
user-invocable: true
---

# /ralph-loop <spec-file>

Executes an approved spec **end-to-end in a single run** — no Stop-hook iteration, no
per-task pauses. Parallelizes whatever the Parallel Batches section allows (one
worktree per parallel task), then self-reviews the diff before handing back to the
user. Commits only after explicit user approval.

> 🎯 **Princípio diretor:** qualidade > velocidade > custo. Antes de cada decisão
> de execução (qual TC adicionar quando descobrir um cenário novo, qual finding
> triagem como trivial vs judgment, se um SHOULD FIX deveria ser MUST), aplicar a
> máxima do projeto pinada em [CLAUDE.md](../../../CLAUDE.md) e
> [.claude/rules/sdd.md](../../rules/sdd.md) §Princípio diretor. Concretamente:
> NICE TO HAVE pode ser MUST sob a lupa de qualidade. Round 3+ de self-review é
> feature, não bug. Quando em dúvida entre rollback parcial e merge silencioso de
> sucessos, **rollback** — preserva a capacidade do usuário de raciocinar sobre o
> working tree.

## Example

```text
/ralph-loop .specs/user-audit-log.md
```

## Phases

The skill runs five phases back-to-back:
**Validate → Execute → Self-review → Present → Commit (only after approval)**.
The only pause point is between Present and Commit (waiting for the user to approve).

### Phase 1 — Validate inputs

1. Read the spec file. Refuse if status ≠ `APPROVED` or `IN_PROGRESS` (a terminal `DONE` /
   `FAILED` / `SUPERSEDED` / `ARCHIVED` spec is not executable — see the state machine in
   `.claude/rules/sdd.md`).
2. Verify the **Parallel Batches** section exists. If missing, regenerate from
   `files:`/`depends:` and warn.
3. Verify the **Test Plan** section is non-empty.
4. **Deterministic gate:** run `go run ./tools/validate-spec <spec>` (or
   `make validate-spec FILE=<spec>`). If it exits non-zero (a lint ERROR), STOP and refuse to
   execute — report the findings and tell the user to fix the spec or re-run `/spec`. The linter
   encodes the structural rules of `.claude/rules/sdd.md`.
5. Verify no other `.specs/*.active.md` exists (legacy state file from old Stop-hook
   ralph-loop). If found, delete it — single-run mode doesn't use state files.
6. Set spec status to `IN_PROGRESS` (if not already).

If anything fails: stop, report what's missing, and tell the user to re-run `/spec`
or fix the spec manually.

### Phase 2 — Execute (autonomous, parallel where possible)

For each batch in **Parallel Batches**, sequentially:

#### Case A — Batch with 1 task (TASK-MERGE-* or anything else)

Execute inline in the main working tree (no worktree overhead):

1. Read spec for the task: `files:`, `tests:`, Design section, relevant rules.
2. **If the task name starts with `TASK-MERGE-`** (accumulator pattern, see
   `.claude/rules/sdd.md` §Merge Strategy):
   - Read every fragment under `.specs/wiring/<spec-slug>/`.
   - Group fragments by `Target`. Verify all targets in fragments match files in this
     task's `files:`.
   - For each target file, in fragment-name order (alphabetical sort of `<task-id>`):
     - Apply imports (deduplicated, merged into the existing import block).
     - For each `### Section: <anchor>` block, locate the named anchor in the target
       file and insert the code block at the correct position
       (`buildDependencies` → before `return Dependencies{...}`,
       `route registration` → inside the route group setup, etc. — see
       `.claude/rules/sdd.md` for the full anchor catalogue).
   - If two fragments contradict each other at the same anchor with different
     content: STOP, report the conflict, leave the task `[ ]`.
   - On merge success, run `gofmt -w` on the target file, then `go build ./...`.
3. **Else if `tests:` present (TDD cycle):**
   - **RED:** Write the test file(s) first with all listed TCs as table-driven
     entries (test names: natural English, not TC-IDs). Run
     `go test ./<relevant-pkg>/...` to confirm RED state (compile fail OR test fail).
   - **GREEN:** Implement production code until all tests pass.
   - **REFACTOR:** Clean up duplication, extract helpers, improve naming. Re-run
     tests + `go build ./...` — must stay green.
   - Re-read spec, verify all `files:` were touched and all patterns followed.
4. **Else** (migrations, config, schema-only): execute as described.
5. Run `go build ./...` to verify compilation. If the change touches HTTP handlers,
   run `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`.
6. Mark `- [ ] TASK-N:` → `- [x] TASK-N:` in the spec.
7. Append a one-line entry to the Execution Log:

   ```markdown
   ### TASK-N (YYYY-MM-DD HH:MM)
   TDD: RED(X) → GREEN(X) → REFACTOR(clean) — <1-line summary>
   ```

   For `TASK-MERGE-*`, the entry is `MERGE: <N> fragments → <target-file>`.

#### Case B — Batch with 2+ tasks (PARALLEL via worktrees)

Launch **all tasks in the batch as parallel `Agent` calls in a SINGLE message** with
`isolation: "worktree"`:

```text
[in one message:]
Agent(general-purpose, isolation: worktree): execute TASK-3 from <spec> ...
Agent(general-purpose, isolation: worktree): execute TASK-4 from <spec> ...
Agent(general-purpose, isolation: worktree): execute TASK-5 from <spec> ...
```

Each agent prompt is self-contained:

```text
Execute TASK-N from .specs/<name>.md.

## Task
<full task description>

## Files
<files: from task metadata>

## Test Plan (relevant rows)
<TC-IDs from this task's tests:, with descriptions and expected outcomes>

## Wiring fragments (if any)
If your `files:` includes a fragment path like
`.specs/wiring/<spec-slug>/<task-id>.<target-slug>.fragment.md`, you must
write that fragment instead of editing the shared target file directly.
Format spec: `.claude/rules/sdd.md` §Merge Strategy (accumulator pattern).

## TDD Cycle
1. Write tests FIRST (*_test.go) for all TCs listed
2. `go test ./<relevant-pkg>/...` to confirm RED
3. Implement production code
4. REVIEW: re-read Task and Files. Verify all files created/modified, all patterns
   followed, all error mappings/wrapping/classifications complete.
5. `go test ./<relevant-pkg>/...` to confirm GREEN
6. REFACTOR: clean duplication, improve naming. Tests must stay green.
7. `go build ./...` to confirm compile

## Conventions
- See .claude/rules/go-conventions.md, go-idioms.md, security.md
- Unique error variable names (parseErr, saveErr — never reuse `err`)
- Hand-written mocks in mocks_test.go per package — no frameworks
- Domain errors are pure sentinels (var Err... = errors.New(...))
- Use cases return *apperror.AppError via local toAppError()
- Use cases classify spans via shared.ClassifyError(span, err, expectedErrors, "ctx")
- Handlers use httpgin.SendSuccess / httpgin.SendError (pkg/httputil/httpgin)
- gRPC handlers translate via toGRPCStatus()

Report: list files created/modified, RED count → GREEN count, any issues.
```

After **all parallel agents return**:

##### Auto-rollback on partial failure

Before merging anything, count how many agents succeeded:

- **All agents succeeded:** continue to the merge step below.
- **Any agent failed:** STOP. **Do not merge any worktree.** Surface to the user:

  ```text
  ⚠️ Batch [TASK-X, TASK-Y, TASK-Z] — partial failure.

  ✅ TASK-X: <summary>
  ✅ TASK-Y: <summary>
  ❌ TASK-Z: <one-line failure cause>

  Nothing has been merged into main. Choose:
    (a) merge X and Y, leave Z for me to fix manually
    (b) discard everything, rerun the batch with adjustments
    (c) stop here so I can investigate
  ```

  Wait for explicit user direction. Default (no answer): treat as (c). **Never merge
  a partially-failed batch silently.** This is the auto-rollback contract.

##### Merge step (only when all succeeded, or after user picks option (a))

1. For each merged worktree, in order:
   - Copy files from the worktree path back into the main working tree (the spec's
     `files:` is the authoritative list — do not pull files outside it).
   - **Cleanup the worktree manually** (CRITICAL — runtime does NOT auto-cleanup
     when changes were made):

     ```bash
     git worktree remove <worktreePath> --force
     git worktree prune
     ```

     Orphan worktrees pile up fast (one per Agent call) and break the VS Code Go
     extension (each worktree carries its own `go.mod`). Run cleanup immediately
     after copying files out of each worktree.
   - If a shared-additive file conflicts (which shouldn't happen if accumulator
     pattern was applied properly): STOP, report, ask user. The fix is usually a
     missing `TASK-MERGE` in the next batch.
2. **Verify merged state:** `gofmt -l .`, `go vet ./...`, `go build ./...`,
   `go test ./internal/...`.
3. Mark all successfully-executed tasks `[x]` in the spec. Tasks from option (a)'s
   skipped set remain `[ ]`.
4. Append a single batch entry to the Execution Log:

   ```markdown
   ### Batch [TASK-3, TASK-4, TASK-5] (YYYY-MM-DD HH:MM)
   Parallel via worktrees.
   - TASK-3: <summary> — TDD: RED(X) → GREEN(X)
   - TASK-4: <summary> — TDD: RED(X) → GREEN(X)
   - TASK-5: <summary> — TDD: RED(X) → GREEN(X)
   ```

   For partial merge (option a), include `- TASK-Z: SKIPPED — <reason>`.

#### After all batches

- All successfully-executed tasks marked `[x]`. Set spec status to `DONE`
  (still subject to phase-4 approval — this is just bookkeeping).
- Run final validation in the working tree:
  - `gofmt -l .` (must be empty)
  - `go vet ./...`
  - `golangci-lint run` (matches CI lint configuration)
  - `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`
    (when HTTP handlers changed)
  - `go build ./...`
  - `go test ./internal/...`
  - **Optional but recommended:** `make ci-local` — simulates a fresh-clone CI run
    in an isolated worktree and catches drift the local pipeline misses (proto-gen,
    swag drift, lint variation). Worth the 30–60s if the change is non-trivial.
- **Wrap-up — capability docs (audited artifact, not an afterthought):** for each subsystem whose
  behavior changed, update its `docs/capabilities/*.md` — the `## Guarantees (current truth)`
  section plus a `## Changelog` `ADDED`/`MODIFIED`/`REMOVED` entry tagged with the spec slug — then
  regenerate the index: `make capabilities-manifest`. The Phase-3 reviewers check this was done.
- Capture the diff vs `main` (`git diff main...HEAD --stat`) for the self-review phase.

### Phase 3 — Self-review (BLOCKING — runs every time)

Spawn **three review agents in parallel** in a single message:

```text
Agent(code-reviewer): Review the implementation of .specs/<name>.md against:
  - the spec's Design and Tasks sections
  - .claude/rules/go-conventions.md, go-idioms.md, migrations.md
  - Clean Architecture layer rules: domain ← usecases ← infrastructure
  - apperror mapping (toAppError), span classification (WarnSpan / FailSpan)
  - DI pattern (manual wiring in cmd/api/server.go), httpgin response helpers
  - the impacted docs/capabilities/*.md was updated (guarantees + Changelog) when behavior
    changed, and the manifest regenerated (make capabilities-manifest)
  Flag MUST FIX / SHOULD FIX / NICE TO HAVE.

Agent(test-reviewer): Audit the tests added by .specs/<name>.md:
  - every TC in the spec's Test Plan has a matching test (table-driven)
  - test names are natural English, not TC-IDs
  - hand-written mocks in mocks_test.go (no testify/mock or gomock)
  - error-path TCs outnumber happy-path TCs
  - boundary tests on validated fields
  - go-sqlmock for repository tests; TestContainers only for E2E
  - no t.Skip without reason; no time.Sleep; no shared state breaking t.Parallel
  Flag MUST FIX / SHOULD FIX / NICE TO HAVE.

Agent(security-reviewer): Audit the diff for security/privacy violations:
  - no PII in logs, fixtures, or response bodies
  - parameterized queries (no string-concat SQL)
  - service-key auth on new endpoints (X-Service-Name + X-Service-Key)
  - no secrets in committed config or tests
  - no removal of input validation
  - migration Down sections present and reversible
  Flag CRITICAL / HIGH / MEDIUM / LOW.
```

Wait for all three. Aggregate findings:

1. **Apply trivially-correct fixes inline.** Then re-run `go build ./... && go test
   ./internal/...` to confirm nothing broke.
   - Test name converted from TC-ID to natural English.
   - Forgotten error-message context wrap (`fmt.Errorf("...: %w", err)`).
   - Missing `WarnSpan` / `FailSpan` call in a use case path.
   - `fmt.Println` / `log.Println` left in production code.
   - Missing index on a foreign-key column in a migration.
   - `.With...()` builder argument forgotten in DI wiring.
2. **Do NOT silently change** anything that requires judgment:
   - Architectural pushback ("this should be its own package").
   - Adding a TC the reviewer thinks should exist (mention it, let user decide).
   - Refactoring suggestions.
   - Security CRITICAL / HIGH findings — never auto-fix; always escalate.

3. **Triagem of NICE TO HAVE under the quality-first lens.** Before discarding a
   NICE TO HAVE finding, ask: "is this rigor or polish?"
   - **Rigor** (e.g., a missed boundary test, a forgotten `errors.Is` assertion,
     an unwrapped error context, a missing span attribute that drops observability,
     a hand-written mock that doesn't assert call args): **upgrade to MUST, apply
     inline if trivial, otherwise surface as "Pontos de atenção".**
   - **Polish** (e.g., comment phrasing, reorder of struct fields, naming
     preference): defer with an explicit one-line note in "Pontos de atenção".
   - When unsure: treat as rigor.

### Phase 4 — Present for approval

Output to the user, in this order:

1. **Spec status** — DONE (pending commit).
2. **Resumo da execução** — N tasks done, M batches, X paralelos via worktree, total
   tempo, total LOC adicionado.
3. **Diff stat** — `git diff main --stat` summary.
4. **Auto-revisão — fixes aplicados** — bullet list of trivial fixes from phase 3.
5. **⚠️ Pontos de atenção** — every MUST FIX / SHOULD FIX / CRITICAL / HIGH from the
   reviewers, with file:line and suggested fix.
6. **🟢 Validação** — `gofmt`, `go vet`, `golangci-lint`, `go build`, `go test`,
   `make ci-local` (if run) — pass/fail counts.
7. **🟢 Posso commitar?** — explicit ask for the user to either approve, push back,
   or request more changes.

**Stop here.** Do not commit yet.

#### What to do with user feedback

After presenting, three things can happen:

- **Approval ("ok", "commit", "pode commitar"):** advance to Phase 5.
- **More changes requested:** apply the requested changes, **then re-run Phase 3
  self-review from scratch** (3 reviewers in parallel, fix triviais inline),
  **then re-present Phase 4**. The cycle continues until the user approves.
  Re-running the review every loop is intentional — a correction is itself code
  that can introduce regressions. The cost is a few seconds; the alternative
  (skipping) silently erodes the safety net. **Round 3, Round 4, Round N are
  normal under the quality-first lens** — keep going until the user approves.
- **Rejection / abort:** mark spec status as `FAILED` with a one-line reason in the
  Execution Log. Stop.

### Phase 5 — Commit (only after explicit user approval)

1. Stage only the files in the spec's Tasks `files:` lists (plus the spec file
   itself, plus any merged-in fragment files under `.specs/wiring/<spec-slug>/`).
2. Commit with the message:

   ```text
   feat(<scope>): <one-line summary based on spec REQs>

   - <bullet per major REQ implemented>
   - Spec: .specs/<name>.md
   ```

   **Do NOT add `Co-Authored-By` trailers.** (User preference: no AI-attribution trailers.)
3. Show `git log -1` and current status.
4. Suggest next steps: `/spec-review .specs/<name>.md` for an independent post-merge
   audit, or `/spec <next-feature>` to start the next item.
5. **Do not push.** The user runs `git push` (or asks).

## Failure handling

- **Agent in a worktree fails:** auto-rollback semantics (Phase 2 Case B). **Do not
  merge any worktree** of the batch silently. Stop, report which task failed, and
  ask the user to choose between (a) merge successful + skip failed, (b) discard
  everything and rerun, (c) stop for manual investigation.
- **Merge conflict on a shared file (after agents succeeded):** stop the batch,
  leave all batch tasks unchecked, surface in Phase 4. The fix is usually a missing
  `TASK-MERGE` in the next batch — re-`/spec` may be needed.
- **`TASK-MERGE` conflict (two fragments contradict each other at the same anchor):**
  stop, leave the merge task unchecked, surface to user. The fix is to clarify
  intent in the spec.
- **Validation fails after a batch:** stop. Do not start the next batch. Surface
  what broke.
- **Test fails after RED→GREEN:** the implementing agent must fix it before
  reporting success. If it gives up, the task is unchecked, fail the batch.

## What the skill does NOT do

- Does not iterate task-by-task with the Stop hook (the old Ralph Loop). Single
  pass, parallel where possible.
- Does not use `.specs/*.active.md` state files — those were the trigger for the old
  Stop-hook loop; this skill is single-run.
- Does not auto-commit. Always waits for user approval.
- Does not modify the spec's Requirements or Test Plan during execution. Tasks may
  be marked `[x]`, the Execution Log appended to. Nothing else.
- Does not skip phase 3 (self-review). Even for trivial specs, the review pass runs.
- Does not push to remote. The user runs `git push` (or asks).

## When to use vs. /spec

- `/spec` writes the spec; you review and approve.
- `/ralph-loop` executes the approved spec end-to-end and presents results.

The two are explicitly separate — you always have a checkpoint between them.

## Resume after interruption

If the skill was interrupted (Ctrl+C, crash) mid-execution:

1. Re-running `/ralph-loop .specs/<name>.md` picks up from the first uncompleted
   `- [ ] TASK-N:`. The spec file is the source of truth.
2. Any orphan worktrees from the interrupted run should be cleaned manually:
   `git worktree list` to find them, then
   `git worktree remove <path> --force && git worktree prune`.
