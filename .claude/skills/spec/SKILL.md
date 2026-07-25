---
name: spec
description: Create + self-review an SDD specification (requirements, test plan, tasks, parallel batches) and present for user approval
argument-hint: "<feature-description>"
user-invocable: true
---

# /spec <feature-description>

End-to-end spec authoring with built-in self-review. **No iteration loops** — runs once,
presents the result, waits for your approval. After approval, run
`/ralph-loop .specs/<name>.md` to execute it.

> 🎯 **Princípio diretor:** qualidade > velocidade > custo. Antes de cada decisão
> nesta skill (qual REQ escrever, qual TC adicionar, qual finding triagem como
> trivial vs judgment), aplicar a máxima do projeto pinada em
> [CLAUDE.md](../../../CLAUDE.md) e [.claude/rules/sdd.md](../../rules/sdd.md)
> §Princípio diretor. Concretamente: pergunte "qual a versão **certa** desta REQ?"
> não "esta REQ é boa o suficiente?". NICE TO HAVE pode ser MUST sob a lupa de
> qualidade. Round 3+ de self-review é feature, não bug. Token cost é secundário.

## Example

```text
/spec "Add audit logging to all user write operations"
```

## Phases

The skill runs three phases back-to-back in a single response:
**Author → Self-review → Present**.

### Phase 1 — Author

1. **Understand the request.** Parse the feature description; identify which domain(s)
   are affected (`internal/domain/...`, `internal/usecases/...`,
   `internal/infrastructure/...`, `cmd/api/`, `pkg/`); classify the change (new
   domain, new use case, new endpoint, schema change, refactor, bug fix).
2. **Gather context.** Read:
   - `.specs/TEMPLATE.md`
   - `.claude/rules/sdd.md`, `.claude/rules/go-conventions.md`, `.claude/rules/go-idioms.md`,
     `.claude/rules/security.md`, `.claude/rules/migrations.md`
   - Relevant ADRs in `docs/adr/`
   - Closest existing pattern as reference (the `user` domain is the canonical
     full-featured example with cache, singleflight, idempotency; the `role` domain
     is the minimal multi-domain DI example)
   - `docs/guides/error-handling.md` and `docs/guides/observability.md` if the change
     adds error paths or instrumentation
   - `docs/capabilities/README.md` + the capability doc(s) of the subsystem(s) this
     change touches — identify the **impacted capability** to link in Design (or note
     "new subsystem → a new capability doc will be created")
3. **Pick a name.** Lowercase, hyphen-separated:
   `.specs/user-audit-log.md`, `.specs/role-permissions.md`, `.specs/fix-cache-stampede.md`.
4. **Write `.specs/<name>.md`** from `.specs/TEMPLATE.md`. Fill in:
   - **Slug** — a `## Slug:` line (UPPERCASE, `^[A-Z][A-Z0-9]*$`). All REQ/TC IDs are
     slug-prefixed: `<SLUG>-REQ-N` and `<SLUG>-TC-<TYPE>-NN` (registered TYPEs:
     `D`/`UC`/`E2E`/`S`, plus `SH`/`CT` for harness specs). Documentation-only REQs carry a
     `(no-test: <reason>)` annotation on their declaration line.
   - **Context** — why the feature exists, link to relevant ADRs / guides.
   - **Requirements** in GIVEN/WHEN/THEN form. No vague "should kinda".
   - **Test Plan** (see *Test Plan rigor* below — this is the load-bearing section).
   - **Design** — approach paragraph + affected files + dependencies, plus an
     **Impacted Capability** link to the relevant `docs/capabilities/*.md` (or
     "new subsystem → a new capability doc will be created"). Mark unknown items
     `[NEEDS CLARIFICATION]`.
   - **Tasks** — concrete, ordered, each with `files:`, `tests:` (TC-IDs),
     `depends:`. The accumulator pattern (see below) must be applied here, before
     batches are computed.
   - **Parallel Batches** — auto-generated from `files:` and `depends:` (see
     *Parallelism analysis* below).
   - **Validation Criteria** — `make lint`, `make test`, `make test-e2e` (when E2E
     TCs exist), `swag init` (when HTTP handlers change), plus smoke step if a
     `TASK-SMOKE` exists.
   - **Status: DRAFT**.

### Phase 2 — Self-review (BLOCKING — runs every time)

**Phase 2a — Deterministic linter gate (runs FIRST, before the agents).** Run
`go run ./tools/validate-spec .specs/<name>.md` (or `make validate-spec FILE=.specs/<name>.md`).
If it exits non-zero (a lint ERROR — missing required section, broken REQ↔TC coverage, cyclic
`depends:`, batch file-overlap, bad slug/ID format, …), fix the reported issues (mechanically when
trivial; surface as a judgment call otherwise) and re-run until it exits 0. The linter is the
deterministic pre-filter — it encodes the structural rules of `.claude/rules/sdd.md`; only once it
passes do you spawn the inferential agents, which focus on the semantic gaps it cannot see.

**Phase 2b — Spawn four review agents in parallel** in a single message with four Agent calls:

```text
Agent(spec-reviewer): Review .specs/<name>.md for gaps, ambiguity, missing tests, rule violations, and architectural mismatches.

Agent(test-reviewer): Audit the Test Plan section of .specs/<name>.md for coverage gaps — every REQ has TC, every sentinel error has TC, every validated field has boundary TCs, every external dependency call has an infra-failure TC, every conditional branch has TCs for both paths. Apply the sdd.md rigor check (error TCs outnumber happy-path TCs).

Agent(code-reviewer): Audit the Design section of .specs/<name>.md for project-rule adherence — Clean Architecture layer rules (domain ← usecases ← infrastructure), apperror mapping, span classification (WarnSpan vs FailSpan), DI pattern, response helpers (httpgin), idempotency on write paths, gRPC parity if applicable.

Agent(security-reviewer): Audit .specs/<name>.md in spec-review mode (pre-code — no diff exists yet). Check:
  1. Tenant/scope isolation — does the spec plan appropriate resource scoping so one tenant cannot read another's data?
  2. PII in planned logs/fixtures — any REQ or Design element that would log, fixture, or expose email addresses, full names, phone numbers, or tokens? Flag per .claude/rules/security.md "never log PII". These are MUST FIX.
  3. Service-key auth for every new endpoint — for each planned HTTP route and gRPC method, verify the spec names the service-key middleware (HTTP) or interceptor (gRPC). Reject "internal-only ⇒ no auth" reasoning outright (ADR-005; .claude/rules/security.md "service key authentication required on all API endpoints"). Missing auth on any new endpoint is MUST FIX.
  4. Sentinel→status resource leak — do the planned error→HTTP-status mappings risk leaking a cross-context resource's existence? (e.g. returning 404 vs 403 in a way that reveals another tenant's ID)
  5. Secrets in planned fixtures — any test fixture, golden file, or seed data that would contain real credentials, API keys, or tokens?
  6. Dependencies section — for every new dependency listed in the spec's ## Dependencies, flag any that add network calls (HTTP clients, SDKs, message-bus libs) or replace stdlib (crypto, encoding, TLS). These require explicit justification in the spec. Flag as MUST FIX if justification is absent.
  Rate findings: CRITICAL / HIGH / MEDIUM / LOW. CRITICAL and HIGH are never auto-fixed; always surface to user.
```

Wait for all four. Aggregate findings:

1. **Apply trivially-correct fixes inline** to the spec file:
   - Missing TC-IDs for declared sentinel errors → add the entry to the Test Plan.
   - Missing `tests:` mapping on a task that produces testable code → add it.
   - Wrong file path in `files:` → fix it.
   - Missing dependency in `depends:` → add it.
   - Boundary TCs missing for a validated field → add them.
   - Privacy/auth constraint missing in Validation Criteria → add it.
   - `make lint` / `make test` missing in Validation Criteria → add.
   - `swag init` missing when HTTP handlers change → add.
   - Trivial wording fixes.

2. **Do NOT silently change** anything that requires a judgment call:
   - Architectural choices ("use a different pattern", "split this domain").
   - Adding/removing a REQ.
   - Splitting a task into smaller tasks.
   - Changing the parallel-batch grouping.
   - Anything tagged MUST FIX that isn't a trivial typo or missing-mapping fix.

   These go to the user as "Pontos de atenção".

3. **Triagem of NICE TO HAVE under the quality-first lens.** Before discarding a
   NICE TO HAVE finding, ask: "is this rigor or polish?"
   - **Rigor** (e.g., a missed boundary TC, a missing infra-failure TC, an
     ambiguous REQ phrasing that admits two readings, an unwrapped error context,
     a missing `errors.Is`/`errors.As` assertion in a test): **upgrade to MUST,
     apply inline if trivial, otherwise surface as "Pontos de atenção".**
   - **Polish** (e.g., comment phrasing, alphabetical ordering of imports, naming
     style preference): defer with an explicit one-line note in "Pontos de
     atenção" so the user knows it was considered.
   - When unsure: treat as rigor.

4. Re-read the modified spec end-to-end to confirm consistency after inline fixes.

### Phase 3 — Present for approval

Output to the user, in this order:

1. **Path of the spec file** (clickable).
2. **Resumo da spec** — 3–5 bullets covering scope, approach, and parallelism summary.
3. **Test Plan stats** — count by layer (Domain / Use Case / E2E / Smoke) and by
   importance (REQ-bound / boundary / infra-failure / security).
4. **Parallel Batches** — short summary
   (e.g. "Batch 1: 1 task foundation, Batch 2: 4 tasks paralelos via worktree, Batch 3: TASK-MERGE em server.go").
5. **Auto-revisão — fixes aplicados** — bullet list of trivial fixes made during phase 2.
6. **⚠️ Pontos de atenção que NÃO foram aplicados** — every MUST FIX / SHOULD FIX
   from the reviewers that requires user judgment, grouped by severity, with
   file:line and suggested fix.
7. **🟢 Aprovado o suficiente para implementar?** — explicit ask for the user to
   either approve (status → APPROVED), edit, or push back on the points of attention.

**Stop here.** Do not start implementation. Do not run `/ralph-loop`. The user
explicitly drives the next step.

#### What to do with user feedback

After presenting, three things can happen:

- **Approval ("ok", "aprovado", "pode rodar /ralph-loop"):** flip the spec status from
  `DRAFT` to `APPROVED`. Stop. The user runs `/ralph-loop` themselves.
- **Changes requested:** apply the requested changes to the spec file, **then re-run
  Phase 2 self-review from scratch** (4 reviewers in parallel, fixes triviais
  inline), **then re-present Phase 3**. The cycle continues until the user approves
  or rejects. Re-running the review on every iteration is intentional — a correction
  is itself spec text that can introduce regressions, and the runtime cost (seconds
  per pass) is insignificant compared to merging a flawed spec. **Round 3, Round 4,
  Round N are normal under the quality-first lens** — keep going until the user
  approves or rejects.
- **Rejection ("descarta", "não faz"):** delete the spec file (or move to
  `.specs/archive/<name>.md` if the user wants to keep history). Stop.

## Test Plan rigor

This is the load-bearing section. The expanded checklist:

1. **Per REQ:** ≥ 1 happy-path TC + all error/edge TCs that the requirement implies.
2. **Per sentinel error** in the design's domain `errors.go`: ≥ 1 TC that triggers it.
3. **Per validated field:** boundary TCs (valid min, valid max, invalid min-1,
   invalid max+1).
4. **Per external dependency** (sqlx repo, Redis cache, idempotency store, HTTP
   client, gRPC client): ≥ 1 failure-mode TC (DB timeout, cache down, network
   timeout, 5xx upstream).
5. **Per conditional branch** in the use case flow: TCs for both paths.
6. **Concurrency** specifics: operations with advisory lock, optimistic locking, or
   `singleflight` need explicit concurrency TCs (leader vs waiter, contention).
7. **Idempotency** specifics: write endpoints using the idempotency middleware need
   TCs for the lock/unlock pattern (replay returns cached response, lock contention
   returns 409).
8. **HTTP endpoint** specifics: every new endpoint has Smoke TCs covering happy path
   (status + every response field), every distinct error status (400/409/422), auth
   (missing/invalid service key), response format
   (`{"data": ...}` / `{"errors": {"message": ...}}`), field boundaries.
9. **gRPC handler** specifics: every new method has TCs for the `toGRPCStatus()`
   mapping (every domain error → gRPC status code).
10. **Group by layer:**
    - **Domain** (`TC-D-NN`): pure logic, value objects, invariants. NO mocks, NO containers.
    - **Use case** (`TC-UC-NN`): hand-written mocks for collaborators, fast.
    - **E2E** (`TC-E2E-NN`): TestContainers (Postgres + Redis), real HTTP via `httptest`.
    - **Smoke** (`TC-S-NN`): k6, validates deployed behavior. NOT subject to TDD RED/GREEN.
11. **Assign TCs to tasks** via `tests:` — each TC belongs to exactly one task.
    Smoke TCs go in a dedicated `TASK-SMOKE` at the end of the spec.
12. **Rigor check (do this last):** count happy-path vs error-path TCs. Errors should
    outnumber happy paths. If they don't, you missed something — surface specific
    gaps before presenting.

> **Quality-first lens applied to the Test Plan:** when in doubt between covering
> a scenario and skipping it, **cover it**. Boundary TCs, infra-failure TCs,
> branch-both-paths TCs, concurrency TCs (singleflight leader/waiter, advisory
> lock contention), idempotency TCs (replay, lock contention), version evolution
> paths — none of these are "nice to have"; they are rigor. The cost of writing a
> table-driven entry is seconds; the cost of a missing TC is a regression in
> production.

Categories: `happy`, `validation`, `business`, `edge`, `infra`, `concurrency`,
`idempotency`, `security`.

## Parallelism analysis

After tasks are written:

1. Build a dependency graph from `depends:` and `files:` overlap.
2. Two tasks **cannot** be in the same batch if either: one depends on the other, or
   they share a file in `files:`.
3. Topological sort into batches:
   - Batch 1: zero-dep tasks.
   - Batch N: tasks whose deps are satisfied by Batches 1..N-1.
4. Classify any shared file:
   - **Exclusive** — only one task touches it (safe for parallel).
   - **Shared-additive** — multiple tasks add to it without removing existing content.
     Common in this codebase: `cmd/api/server.go` (DI wiring in `buildDependencies`),
     `cmd/api/router.go` (route registration), `cmd/api/grpc.go` (gRPC service
     registration when applicable). **Apply the accumulator pattern** (see below) —
     never let parallel tasks edit the shared file directly.
   - **Shared-mutative** — multiple tasks modify existing code: must serialize. Put
     them in different batches.
5. Present the batches in the spec and in the Phase 3 summary, with classification
   per shared file.

### Accumulator pattern (for shared-additive files)

When you detect a shared-additive file, **rewrite the parallel tasks** so they don't
touch it directly:

1. Each parallel task **drops the shared file from its own `files:`**.
2. Each parallel task **gains a fragment file** in
   `.specs/wiring/<spec-slug>/<task-id>.<target-slug>.fragment.md` that describes its
   addition. The fragment lives in `files:` so the task owns it.
3. Add a new `TASK-MERGE-<TARGET>` task in the **next batch after the parallel one**.
   It:
   - Has `files:` listing the shared file (e.g. `cmd/api/server.go`) plus all the
     relevant fragment paths.
   - Has `depends:` listing every parallel task that produces a fragment for that
     target.
   - Has no `tests:` of its own unless wiring correctness requires a quick smoke check.
   - Will be executed sequentially by `/ralph-loop` in the main working tree (it
     reads the fragments and applies them).

Fragment format is defined in `.claude/rules/sdd.md` §Merge Strategy. Each fragment
is a markdown file with sections:

- **Intent** — one sentence
- **Target** — full path of the shared file
- **Imports** — Go imports to add (optional)
- **Additions** — code blocks grouped under `### Section: <named anchor>` (the
  registered anchors are listed in `.claude/rules/sdd.md`; the canonical ones for
  this project are `buildDependencies` in `cmd/api/server.go` and
  `route registration` in `cmd/api/router.go`)
- **Notes** — optional ordering hints / known conflicts

When you write parallel-batch tasks for the spec, **always look first** for
shared-additive files and apply this pattern *before* presenting the batches. If you
don't, the merge inevitably surfaces as a conflict in `/ralph-loop`'s phase 2.

## Rules

- Spec files in `.specs/` directory.
- File naming: lowercase, hyphen-separated, optionally numeric prefix.
- Never include tasks that require user decisions — ask upfront, before writing the spec.
- Reference existing code: if a task is similar to an existing use case / handler /
  repository, name those files in the Design section.
- Match spec depth to task complexity: a simple bug fix doesn't need 30 TCs.
- This skill **never** runs `/ralph-loop` automatically. The user does that
  explicitly after reviewing the spec.

## Integration

After the user approves:

```text
/ralph-loop .specs/<name>.md
```

That's the next step — `/ralph-loop` executes the whole spec autonomously
(parallelizing per batch via worktrees), self-reviews the implementation, and
presents results back to you.
