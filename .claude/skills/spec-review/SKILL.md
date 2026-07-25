---
name: spec-review
description: Independent post-merge review of an implemented spec — verifies REQs, runs validation, and appends a Review Results section to the spec
argument-hint: "<spec-file-path>"
user-invocable: true
---

# /spec-review <spec-file>

Independent audit of an **already-merged** spec implementation. Runs after
`/ralph-loop` finished and the user approved the commit. This is the formal
post-merge review — **distinct from the inline self-review** that `/ralph-loop`
does as part of its Phase 3.

> 🎯 **Princípio diretor:** qualidade > velocidade > custo. The whole point of
> `/spec-review` is to apply rigorous post-merge scrutiny independent of the
> inline review. NICE TO HAVE findings here are often rigor in disguise — apply
> the lens, escalate when in doubt. Pinned in [CLAUDE.md](../../../CLAUDE.md) and
> [.claude/rules/sdd.md](../../rules/sdd.md) §Princípio diretor.

## When to run

- Days/weeks after the spec was merged, to verify nothing rotted (deps changed,
  Go toolchain upgraded, `go.mod` evolved, schema drifted, etc.).
- When you want a fresh independent audit instead of trusting the inline review.
- Before tagging a release that includes the spec.
- After a refactor in adjacent code that might have invalidated the spec's
  assumptions.

## Example

```text
/spec-review .specs/user-audit-log.md
```

## Workflow

### 1. Load Spec

- Read the spec file (status should be `DONE`, or `SUPERSEDED`/`ARCHIVED` for a retired spec).
- Extract all `<SLUG>-REQ-N` entries verbatim — slug-prefixed; legacy specs without a `## Slug:`
  use bare `REQ-N` — so the report can quote them.
- Extract all Validation Criteria checkboxes.
- Note the Design section for architectural intent — the implementation must
  match it, not just "do something close".
- Note the Test Plan for the TC-IDs that should exist as concrete tests.

### 2. Verify Requirements (REQ-by-REQ trace)

For each REQ:

- Trace through the code (`internal/domain/`, `internal/usecases/`,
  `internal/infrastructure/`, `cmd/api/`) to find the code/config/test that
  satisfies it.
- Confirm the implementation matches the Design section — not silently diverged.
- Verify project conventions are followed:
  - Domain layer has zero external imports (only stdlib).
  - Use cases return `*apperror.AppError` via local `toAppError()`; no raw
    domain errors leak to handlers.
  - Use case classifies expected vs unexpected errors via
    `shared.ClassifyError(span, err, expectedErrors, "context")`.
  - Handler resolves errors generically via `errors.As(err, &appErr)` +
    `codeToStatus` map — no domain imports.
  - Handlers use `httpgin.SendSuccess` / `httpgin.SendError` from
    `pkg/httputil/httpgin`.
  - gRPC handlers (when present) translate via `toGRPCStatus()` for parity.
  - Manual DI in `cmd/api/server.go:buildDependencies()` (no DI framework).
  - Migrations have both `-- +goose Up` and `-- +goose Down` sections.
- **Flag any REQ that is partially / incorrectly / not implemented** — do not
  give benefit of the doubt; cite the file:line where evidence is missing.

### 3. Run Validation

Execute every checkbox in the spec's **Validation Criteria** section. At minimum:

- `gofmt -l .` — must be empty
- `go vet ./...`
- `golangci-lint run`
- `go build ./...`
- `go test ./internal/...` (or `make test`)
- `make test-e2e` if the spec added E2E TCs (`TC-E2E-*`)
- `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal` —
  must produce no diff vs committed `docs/` (Swagger drift = a regression)
- **Optional but recommended:** `make ci-local` — full fresh-clone CI parity in
  an isolated worktree (catches gitignored-artifact drift the local pipeline
  misses)
- Manual smoke (when applicable): start the service via `make dev` or
  `make run`, exercise the changed endpoint with `curl`, verify the response
  shape and side effects (DB rows, log lines, span attributes if reachable).
- Mark each criterion `[x] passed`, `[ ] deferred (reason)`, or
  `[ ] failed (details)`. **A failed criterion stops the review** — surface to
  the user before continuing.

### 4. Test Quality Audit (mandatory when the spec has tasks with `tests:`)

Delegate a deep test-quality audit to the `test-reviewer` subagent. Request:

> "Use the test-reviewer subagent to audit the tests added by spec
> `.specs/<name>.md` against its Test Plan. Verify every TC-ID maps to a real
> table-driven entry, test names are natural English (not TC-IDs), error-path
> TCs outnumber happy-path TCs, hand-written mocks in `mocks_test.go` per
> package, no `time.Sleep`, no `t.Skip` without reason, no shared state
> breaking `t.Parallel`. Flag MUST FIX / SHOULD FIX / NICE TO HAVE."

Include the test-reviewer findings as a dedicated **Test Quality** section in
the report. Apply the quality-first lens to triagem: NICE TO HAVE findings that
are actually rigor (missed boundary, weak assertion that admits zero values,
mock that doesn't assert call args) get escalated.

### 5. Generate Report

**Append** (do not overwrite) a `## Review Results` section to the spec file:

```markdown
## Review Results — YYYY-MM-DD

### Requirements verification

| Requirement | Status | Evidence |
| --- | --- | --- |
| REQ-1: <verbatim REQ text, truncated if long> | PASS / FAIL / PARTIAL | `path/file.go:NN` or test name |
| REQ-2: ... | ... | ... |

### Validation checks

| Check | Result |
| --- | --- |
| `gofmt -l .` | PASS / FAIL |
| `go vet ./...` | PASS / FAIL |
| `golangci-lint run` | PASS / FAIL |
| `go build ./...` | PASS / FAIL |
| `go test ./internal/...` | PASS / FAIL |
| `make test-e2e` (if E2E TCs) | PASS / FAIL / SKIP |
| `swag init` (no drift) | PASS / FAIL / SKIP |
| `make ci-local` | PASS / FAIL / SKIP |
| Manual smoke (if applicable) | PASS / FAIL / SKIP |

### Test Quality (test-reviewer findings)

- [SEVERITY] file:line — finding (suggested fix)

### Findings (other)

- [SEVERITY] file:line — finding (suggested fix)

### Notes

<observations, drift caught vs original Design, suggestions for follow-up specs>
```

### 6. Output to user

Summarize the report inline (don't dump the full markdown), highlight failures
and any MUST FIX / SHOULD FIX findings, suggest fixes. **If everything passes,
say so explicitly** — a clean review is a valid output, and saying "looks good"
is more honest than padding the report with cosmetic suggestions.

If there are findings, recommend the next step:

- **MUST FIX or PARTIAL REQ:** open a follow-up spec via `/spec
  fix-<original-spec-slug>` — the work is non-trivial and deserves the full SDD
  flow. Do NOT auto-fix.
- **SHOULD FIX:** offer to apply inline if the user wants, otherwise document in
  the Notes section so the next person knows.
- **NICE TO HAVE:** mention but don't block.

## Integration

- Standalone, manual.
- Recommended after `/ralph-loop` says DONE and the merge is settled — the
  inline self-review and the post-merge review serve different purposes (one
  catches the "this just shipped" issues, the other catches drift over time).
- For deeper review, also run `/full-review-team` or `/security-review-team`.
- For acting on findings: open a follow-up spec via `/spec` if the work is
  non-trivial; or fix inline if it's a typo / one-liner.

## What this skill does NOT do

- Does not auto-fix anything. Findings go to the report; the user decides.
- Does not modify the spec's Requirements or Test Plan — only appends a
  `## Review Results` section.
- Does not commit. The report is appended to the spec file in the working tree;
  the user commits when ready (or asks the assistant to).
