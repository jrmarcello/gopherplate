# Spec: DX — SDD + TDD + Paralelismo com Multi-Agents

## Status: DRAFT

## Context

O fluxo de DX (Developer Experience) do boilerplate esta defasado em relacao ao banking-service-yield, onde fizemos uma revisao completa dos recursos Claude Code (.claude/rules, skills, agents). As principais lacunas:

1. **SDD sem Test Plan obrigatorio** — a regra `sdd.md` nao exige Test Plan, nao define formato de TC-IDs, nao tem coverage rules
2. **SDD sem TDD** — nao ha ciclo RED -> GREEN -> REFACTOR definido, tasks nao tem metadata `tests:`
3. **SDD sem Smoke Tests** — nao ha integracao com k6 no fluxo de spec
4. **ralph-loop sequential only** — nao suporta execucao paralela de tasks em batches via worktrees + multi-agents
5. **spec skill sem Test Plan generation** — nao gera Test Plan exaustivo, nao analisa paralelismo
6. **TEMPLATE.md desatualizado** — falta secoes de Test Plan e Smoke Tests
7. **go-conventions.md sem Span Classification** — nao documenta padrao FailSpan/WarnSpan
8. **code-reviewer.md generico** — nao inclui checklist de Span Error Classification

**Solucao:** Portar todas as melhorias de DX do yield para o boilerplate, adaptando referencias de dominio (savings -> user/role) e removendo referencias financeiras especificas.

**Referencia:** banking-service-yield `.claude/rules/sdd.md`, `.claude/skills/ralph-loop/SKILL.md`, `.claude/skills/spec/SKILL.md`, `.claude/agents/code-reviewer.md`, `.claude/rules/go-conventions.md`

## Requirements

- [ ] REQ-1: **Test Plan obrigatorio em toda spec**
  - GIVEN uma spec esta sendo criada
  - WHEN o status e DRAFT
  - THEN deve conter secao Test Plan com tabelas por camada (Domain, Use Case, E2E, Smoke)
  - AND cada TC tem: TC-ID, REQ reference, Category, Description, Expected
  - AND TC-ID format: `TC-D-NN` (domain), `TC-UC-NN` (use case), `TC-E2E-NN` (e2e), `TC-S-NN` (smoke/k6)

- [ ] REQ-2: **Coverage Rules definidas**
  - GIVEN uma spec tem Test Plan
  - WHEN revisada
  - THEN todo REQ tem >= 1 TC, todo erro de dominio tem >= 1 TC, todo campo validado tem TCs de boundary, toda dependencia externa tem >= 1 TC de falha infra, todo branch condicional tem TCs para ambos paths

- [ ] REQ-3: **TDD Execution no ralph-loop**
  - GIVEN uma task tem metadata `tests:`
  - WHEN executada pelo ralph-loop
  - THEN segue ciclo RED (escreve test primeiro) -> GREEN (implementa ate passar) -> REFACTOR
  - AND o Execution Log registra `TDD: RED(N failing) -> GREEN(N passing)`

- [ ] REQ-4: **Smoke Tests via k6 integrados ao SDD**
  - GIVEN uma spec tem TCs de smoke (TC-S-*)
  - WHEN a task TASK-SMOKE e executada
  - THEN escreve k6 checks, roda `k6 run --env SCENARIO=smoke tests/load/main.js`
  - AND nao segue ciclo RED/GREEN (execucao direta)
  - AND se app nao estiver rodando, loga `SMOKE: DEFERRED`

- [ ] REQ-5: **Execucao paralela de batches no ralph-loop**
  - GIVEN um batch tem 2+ tasks independentes (sem shared files, deps satisfeitas)
  - WHEN o ralph-loop identifica o batch
  - THEN lanca N agents em paralelo com `isolation: "worktree"`
  - AND todos os Agent calls estao num unico message (paralelismo real)
  - AND apos conclusao, merge os worktrees e verifica build/tests

- [ ] REQ-6: **spec skill gera Test Plan exaustivo**
  - GIVEN o usuario executa `/spec "feature description"`
  - WHEN a spec e gerada
  - THEN inclui Test Plan completo derivado dos Requirements e Design
  - AND inclui analise de Parallel Batches baseada em `files:` e `depends:` metadata

- [ ] REQ-7: **TEMPLATE.md inclui Test Plan e Smoke**
  - GIVEN `.specs/TEMPLATE.md` e usado como base para novas specs
  - WHEN inspecionado
  - THEN contem secoes: Test Plan (com Domain, Use Case, E2E, Smoke tables), task metadata com `tests:`

- [ ] REQ-8: **go-conventions.md documenta Span Error Classification**
  - GIVEN um desenvolvedor consulta as convencoes Go
  - WHEN le `.claude/rules/go-conventions.md`
  - THEN encontra secao sobre FailSpan/WarnSpan, quando usar cada, que camada decide span status

- [ ] REQ-9: **code-reviewer.md inclui Span Classification checklist**
  - GIVEN um code review e executado
  - WHEN o agent `code-reviewer` analisa o codigo
  - THEN verifica: use case decide span status, handler nao chama span.SetStatus, expected vs unexpected errors classificados corretamente

- [ ] REQ-10: **CLAUDE.md atualizado com novos padroes**
  - GIVEN `CLAUDE.md` documenta o projeto para o Claude Code
  - WHEN lido
  - THEN inclui: referencia a Span Error Classification como padrao obrigatorio, referencia a error handling guide e ADR

## Test Plan

<!-- Non-code spec: all changes are to configuration/documentation files (.claude/, .specs/, CLAUDE.md).
     No Go tests apply. Validation is via manual review of file contents. -->

N/A — Esta spec altera apenas arquivos de configuracao e documentacao (.claude/rules, .claude/skills, .claude/agents, .specs/TEMPLATE.md, CLAUDE.md). Nao ha codigo Go a testar. A validacao e via review dos arquivos gerados.

## Design

### Architecture Decisions

**Adaptacao yield -> boilerplate:**
- Referencias a `savings`/`deposit` sao substituidas por `user`/`role`
- Coverage rules removem "financial service" wording, mantendo rigor generico
- Smoke test files referenciam `tests/load/users.js` (nao `savings.js`)
- Span classification e documentada como padrao, referenciando arquivos que serao criados pela spec `error-handling-refactor`

**Dependencia entre specs:**
- Esta spec referencia arquivos criados pela spec `error-handling-refactor` (pkg/telemetry/span.go, internal/usecases/shared/classify.go)
- Pode ser implementada ANTES ou DEPOIS da error-handling-refactor — as referencias apontam para arquivos futuros

### Files to Modify

- `.specs/TEMPLATE.md` — adicionar secoes Test Plan (Domain, UC, E2E, Smoke) + task metadata `tests:`
- `.claude/rules/sdd.md` — adicionar secoes: Task Metadata `tests:`, Test Plan, Coverage Rules, Mutability, Smoke Tests, TDD Execution
- `.claude/rules/go-conventions.md` — adicionar secoes: Error Handling (AppError wrapping, toAppError), Span Error Classification (FailSpan/WarnSpan, expectedErrors, ClassifyError)
- `.claude/skills/ralph-loop/SKILL.md` — adicionar: Parallel Execution (decision flow, agent prompt template, merge strategy, quando nao paralelizar), TDD Execution (RED/GREEN/REFACTOR), Smoke Test Execution
- `.claude/skills/spec/SKILL.md` — adicionar: step 4 (Generate Test Plan), step 5 (Analyze Parallelism), atualizar step 3 (task metadata `tests:`)
- `.claude/agents/code-reviewer.md` — adicionar: secao Observability & Span Error Classification (FailSpan/WarnSpan, expectedErrors, ClassifyError, toAppError)
- `CLAUDE.md` — adicionar: referencia a Span Error Classification nos Key Patterns, referencia a error handling ADR/guide

### Files to Create

- `.claude/settings.local.json` — template com permissoes Docker extras (copiar padrao do yield)

### Dependencies

- Nenhuma dependencia externa
- Dependencia conceitual: `error-handling-refactor` spec (para que as referencias a span.go e classify.go existam no codigo)

## Tasks

- [ ] TASK-1: Atualizar .specs/TEMPLATE.md com Test Plan + task metadata
  - Adicionar secao `## Test Plan` entre Requirements e Design
  - Incluir subsecoes: Domain Tests, Use Case Tests, E2E Tests, Smoke Tests (k6) — cada com tabela template
  - Incluir comentarios explicativos sobre Coverage Rules
  - Adicionar `tests:` metadata no exemplo de task
  - Adicionar exemplo de TASK-SMOKE no template
  - Referencia: banking-service-yield `.specs/TEMPLATE.md`
  - files: `.specs/TEMPLATE.md`

- [ ] TASK-2: Atualizar .claude/rules/sdd.md com TDD + Test Plan + Smoke
  - Adicionar a `## Task Metadata`: item sobre `tests:` sub-item obrigatorio para tasks com codigo testavel
  - Adicionar secao `## Test Plan` completa: formato, TC-ID convention, Coverage Rules com header `### Coverage Rules` (sem "non-negotiable for financial service" — adaptar para linguagem generica)
  - Adicionar subsecao `### Mutability` — TCs podem ser adicionados durante IN_PROGRESS, nunca removidos
  - Adicionar subsecao `### Smoke Tests (k6)` — TC-S-*, TASK-SMOKE, execucao sem RED/GREEN, files convention (`tests/load/users.js`, `tests/load/main.js`, `tests/load/helpers.js` — NAO `savings.js`)
  - Adicionar secao `## TDD Execution` completa: RED/GREEN/REFACTOR, compilation failure = valid RED, test file antes de production file, mock_test.go, Execution Log format, exception para smoke tests
  - Adicionar item a Task Execution: "Mandatory review before testing"
  - Referencia: banking-service-yield `.claude/rules/sdd.md`
  - files: `.claude/rules/sdd.md`

- [ ] TASK-3: Atualizar .claude/rules/go-conventions.md com Error Handling + Span Classification
  - Expandir secao `## Error Handling`:
    - Adicionar: use cases retornam `*apperror.AppError` via local `toAppError()`
    - Adicionar: `apperror.Wrap(err, code, message)` preserva chain (errors.Is via Unwrap)
    - Adicionar: handler resolve via `errors.As()` + `codeToStatus` map — zero domain imports
    - Adicionar: referencia a `docs/guides/error-handling.md`, ADR-009
  - Adicionar secao `## Span Error Classification (OTel)`:
    - Use case decide span status — nao handler, nao infrastructure
    - `telemetry.FailSpan(span, err, msg)` para erros inesperados (DB timeout, connection reset, 5xx)
    - `telemetry.WarnSpan(span, key, value)` para erros esperados (validation, not found, conflict)
    - Handler nunca chama span.SetStatus/RecordError
    - Domain layer tem zero dependencia OTel
    - Cada use case define `expectedErrors` + chama `shared.ClassifyError()`
    - Referencia: `pkg/telemetry/span.go`, `internal/usecases/shared/classify.go`
  - Nota: os arquivos referenciados (`pkg/telemetry/span.go`, `internal/usecases/shared/classify.go`, `docs/guides/error-handling.md`, ADR-009) serao criados pela spec `error-handling-refactor`. Incluir nota inline: "Criados pela spec error-handling-refactor — referencias serao validas apos sua execucao"
  - Manter frontmatter `applies-to:` (convencao do boilerplate), nao copiar `paths:` do yield
  - Referencia: banking-service-yield `.claude/rules/go-conventions.md`
  - files: `.claude/rules/go-conventions.md`

- [ ] TASK-4: Atualizar .claude/skills/ralph-loop/SKILL.md com Parallel Execution + TDD
  - Adicionar secao `## Parallel Execution (multi-task batches)` apos Startup:
    - Decision Flow: 1 task -> sequential, 2+ tasks -> parallel agents in worktrees
    - How to Parallelize: identify tasks, launch Agent calls with `isolation: "worktree"` em single message, wait, collect, merge, verify, mark complete, log
    - Agent Prompt Template: task description, files, test plan TCs, TDD cycle, conventions
    - When NOT to Parallelize: shared mutative files, worktree unavailable, trivial tasks
    - Merge Strategy: all succeeded -> merge sequential, some failed -> merge successful, conflict resolution
  - Reescrever secao `## Per-Iteration Execution` para incluir TDD:
    - Check `tests:` metadata on task
    - If has `tests:`: TDD Cycle (RED -> GREEN -> REFACTOR) com detalhes
    - If has `tests: TC-S-*`: Smoke Test Execution (k6, nao RED/GREEN)
    - If no `tests:`: Normal Execution
    - Mandatory review before testing (re-read task, verify files/patterns)
  - Adicionar secao TDD Edge Cases
  - Atualizar On Final Task: mencionar spec-review
  - Atualizar Rules: "Parallel batches launch multiple agents"
  - Referencia: banking-service-yield `.claude/skills/ralph-loop/SKILL.md`
  - files: `.claude/skills/ralph-loop/SKILL.md`

- [ ] TASK-5: Atualizar .claude/skills/spec/SKILL.md com Test Plan + Parallelism
  - Adicionar step `### 4. Generate Test Plan`:
    - Derivar TCs dos Requirements e Design
    - Para cada REQ: happy-path + error/edge TCs
    - Para cada domain error: >= 1 TC
    - Para cada campo validado: boundary TCs
    - Para cada dependencia externa: infra-failure TC
    - Agrupar por camada: Domain (TC-D), Use Case (TC-UC), E2E (TC-E2E), Smoke (TC-S)
    - Para cada novo endpoint: smoke TCs cobrindo happy path, error statuses, auth, response format, boundaries, idempotency
    - Assign TCs to tasks via `tests:` metadata
    - Categories: happy, validation, business, edge, infra, concurrency, idempotency, security
  - Adicionar step `### 5. Analyze Parallelism`:
    - Build dependency graph from `depends:` e `files:`
    - Topological sort em batches
    - Classify shared files: exclusive, shared-additive, shared-mutative
    - Present batches com classificacao
  - Atualizar step 3 (Generate Spec): tasks incluem `tests:` metadata
  - Atualizar step 6 (Present for Approval): mencionar Test Plan e Parallel Batches
  - Atualizar referencia de dominios em step 2 ("Gather Context"): `user` e `role` (nao `savings` e `deposit`)
  - Referencia: banking-service-yield `.claude/skills/spec/SKILL.md`
  - files: `.claude/skills/spec/SKILL.md`

- [ ] TASK-6: Atualizar .claude/agents/code-reviewer.md com Span Classification
  - **Identidade do agent**: ja esta correta ("reviewing code for a Clean Architecture microservice boilerplate") — manter como esta
  - Verificar dominio de referencia: deve usar `user` (nao `savings`)
  - Adicionar secao `### Observability & Span Error Classification` ao Review Focus:
    - Use case decide span status — handler NUNCA chama span.SetStatus/RecordError
    - Expected errors (domain, validation, 4xx) -> telemetry.WarnSpan (span stays Ok)
    - Unexpected errors (infra, timeout, 5xx) -> telemetry.FailSpan (span marked Error)
    - Cada use case define `expectedErrors` slice + chama `shared.ClassifyError()`
    - Cada use case define local `toAppError()` mapping domain errors -> `*apperror.AppError`
    - Referencia: `internal/usecases/shared/classify.go`, pattern em `internal/usecases/user/create.go`, `docs/guides/error-handling.md`
  - Atualizar handler reference: `httpgin.SendSuccess`/`httpgin.SendError` (from `pkg/httputil/httpgin`)
  - Manter secao "Template Quality" existente (exclusiva do boilerplate — nao existe no yield)
  - files: `.claude/agents/code-reviewer.md`

- [ ] TASK-7: Atualizar CLAUDE.md + criar settings.local.json
  - CLAUDE.md — secao `### Key Patterns` (apos "Singleflight"):
    - Adicionar bullet: **Span Error Classification**: Use case classifica erros via `shared.ClassifyError()`. Expected errors -> `telemetry.WarnSpan` (span Ok), Unexpected -> `telemetry.FailSpan` (span Error). Handler nao toca spans. Ref: ADR-009, `docs/guides/error-handling.md`
  - CLAUDE.md — secao `### Key Patterns`, bullet **Error Handling** existente:
    - Expandir: adicionar "Use cases retornam `*apperror.AppError` via `toAppError()`. Handler generico via `errors.As()` + `codeToStatus` map — zero domain imports. Ref: ADR-009, `docs/guides/error-handling.md`"
  - CLAUDE.md — secao `### Conventions`:
    - Adicionar bullet: referencia a `docs/guides/error-handling.md` como guia pratico
  - Criar `.claude/settings.local.json` template:
    ```json
    {
      "permissions": {
        "allow": [
          "Bash(docker pull:*)",
          "Bash(docker manifest:*)",
          "Bash(file .claude/hooks/*.sh)",
          "Bash(docker stop:*)"
        ]
      }
    }
    ```
  - files: `CLAUDE.md`, `.claude/settings.local.json`

## Parallel Batches

```
Batch 1: [TASK-1, TASK-2, TASK-3, TASK-4, TASK-5, TASK-6, TASK-7]  — todos independentes (arquivos distintos)
```

File overlap analysis:
- `.specs/TEMPLATE.md`: TASK-1 only -> exclusive
- `.claude/rules/sdd.md`: TASK-2 only -> exclusive
- `.claude/rules/go-conventions.md`: TASK-3 only -> exclusive
- `.claude/skills/ralph-loop/SKILL.md`: TASK-4 only -> exclusive
- `.claude/skills/spec/SKILL.md`: TASK-5 only -> exclusive
- `.claude/agents/code-reviewer.md`: TASK-6 only -> exclusive
- `CLAUDE.md` + `.claude/settings.local.json`: TASK-7 only -> exclusive
- **Full parallelism possible** — all 7 tasks in a single batch

## Validation Criteria

- [ ] `.specs/TEMPLATE.md` contem secoes Test Plan com tabelas (Domain, UC, E2E, Smoke)
- [ ] `.claude/rules/sdd.md` contem secoes: Test Plan, Coverage Rules, Mutability, Smoke Tests, TDD Execution
- [ ] `.claude/rules/go-conventions.md` contem secao Span Error Classification
- [ ] `.claude/skills/ralph-loop/SKILL.md` contem secao Parallel Execution + TDD Execution
- [ ] `.claude/skills/spec/SKILL.md` contem steps 4 (Test Plan) e 5 (Parallelism)
- [ ] `.claude/agents/code-reviewer.md` contem checklist de Span Error Classification
- [ ] `CLAUDE.md` referencia Span Error Classification e Error Handling ADR/guide
- [ ] `.claude/settings.local.json` existe com permissoes Docker
- [ ] Todas as referencias de dominio usam `user`/`role` (nao `savings`/`deposit`)
- [ ] Nenhuma referencia a logica financeira especifica (grep por: `savings`, `deposit`, `CDI`, `FIFO`, `tax`, `financial service`, `cross-account`, `shopspring/decimal`)
- [ ] Smoke file references usam `users.js`/`roles.js` (nao `savings.js`)

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
