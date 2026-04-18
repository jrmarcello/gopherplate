# Spec: cli-harness-flavors

## Status: DRAFT

## Context

Fowler cita explicitamente **harness templates per service topology** como prática recomendada:
diferentes topologias (CRUD síncrono, event processor, data pipeline) têm perfis de harness
diferentes, e bundlar um template por topologia acelera onboarding e reduz decisões de
engenharia em cada novo serviço.

Hoje, `gopherplate new` gera um template generalista — HTTP+gRPC CRUD com Postgres. Queremos
introduzir **flavors**: variantes nomeadas do scaffold, cada uma pré-configurando o harness
apropriado para a topologia.

Esta é a **Spec 5 de 5** e **consome as outras 4** — assume que `.specs/k6-regression-gate.md`,
`.specs/maintainability-harness.md` e `.specs/behavior-harness.md` já foram executadas, pois os
flavors reutilizam os artefatos criados lá (k6 baselines, semgrep rules, gremlins config, golden
helpers).

### Flavors propostos

| Flavor | Topologia | Diferenciais |
| ------ | --------- | ------------ |
| `crud` (default) | HTTP+gRPC CRUD com DB relacional | Comportamento atual, refatorado para ser um flavor explícito |
| `event-processor` | Consumer Kafka/Redis pubsub | Skeleton `cmd/consumer/`, métricas de lag, baseline k6 focada em throughput, semgrep rule específica (todo handler deve ter retry+DLQ path) |
| `data-pipeline` | Batch ingest/export | Skeleton `cmd/worker/`, métricas de rows/sec, baseline k6 N/A (substituído por benchmark interno), semgrep rule (toda batch op deve ter idempotency key no input) |

Esta spec trabalha com escopo realista: **infraestrutura de flavors + 3 flavors com overlays
mínimos porém funcionais**. Flavors são extensíveis — adicionar um 4º depois é incremental.

## Requirements

### Infraestrutura de Flavors

- [ ] **REQ-1**: GIVEN o CLI `gopherplate`, WHEN o usuário roda `gopherplate new myservice
  --flavor crud`, THEN scaffolda o serviço com o comportamento atual (HTTP+gRPC CRUD).

- [ ] **REQ-2**: GIVEN `gopherplate new myservice` (sem flag), WHEN executado, THEN defaulta
  para `--flavor crud` e scaffolda igual ao comportamento atual (zero regressão).

- [ ] **REQ-3**: GIVEN `gopherplate new myservice --flavor event-processor`, WHEN executado,
  THEN gera scaffold com `cmd/consumer/main.go` em lugar de (ou adicional a) `cmd/api/main.go`,
  com consumer Kafka/Redis configurável.

- [ ] **REQ-4**: GIVEN `gopherplate new myservice --flavor data-pipeline`, WHEN executado, THEN
  gera scaffold com `cmd/worker/main.go`, job runner, e métricas de throughput.

- [ ] **REQ-5**: GIVEN um flavor inválido (`--flavor foo`), WHEN executado, THEN falha com
  mensagem lista os flavors disponíveis.

- [ ] **REQ-6**: GIVEN `gopherplate new --help`, WHEN executado, THEN documenta a flag
  `--flavor` com lista dos valores aceitos.

### Harness Overlay por Flavor

- [ ] **REQ-7**: GIVEN flavor `crud`, WHEN o scaffold é gerado, THEN inclui:
  - Baseline k6 em `tests/load/baselines/smoke.json` focada em CRUD (GET/POST de entidade).
  - Regras semgrep padrão (handlers.yml, usecases.yml).
  - Workflow `perf-regression.yml` ativo.

- [ ] **REQ-8**: GIVEN flavor `event-processor`, WHEN o scaffold é gerado, THEN inclui **além**
  do baseline/semgrep padrão:
  - Consumer de **Redis Streams** em `cmd/consumer/main.go` usando `XREADGROUP` + consumer
    group.
  - `.semgrep/event-processor.yml` com regra: "todo consumer handler deve ter bloco de retry
    OU chamada a DLQ publisher".
  - Métrica de consumer lag (exemplar) em `internal/infrastructure/telemetry/consumer.go`.
  - Baseline k6 adaptada: smoke publica mensagens em Redis Stream + mede tempo até
    processamento (p95 lag).

- [ ] **REQ-9**: GIVEN flavor `data-pipeline`, WHEN o scaffold é gerado, THEN inclui:
  - `.semgrep/data-pipeline.yml` com regra: "toda batch op deve aceitar `idempotency_key` no
    input e persistir run-state".
  - Benchmark interno em `internal/usecases/<domain>/benchmark_test.go` em lugar de k6.
  - Workflow ajustado: `perf-regression.yml` substituído por `benchmark.yml` que compara
    `go test -bench` contra baseline.

### Engenharia de Flavors

- [ ] **REQ-10**: GIVEN a implementação dos flavors, WHEN um dev quer adicionar um 4º flavor,
  THEN o processo é documentado (`docs/guides/cli-flavors.md`) e requer no máximo: (a) diretório
  `cmd/cli/internal/flavors/<name>/`; (b) registro em `cmd/cli/internal/flavors/registry.go`;
  (c) overlays de template em `cmd/cli/internal/flavors/<name>/templates/`.

- [ ] **REQ-11**: GIVEN a natureza additiva dos overlays, WHEN dois flavors têm arquivos comuns
  (ex: ambos crud e event-processor têm `Makefile`), THEN o base template provê o default e o
  flavor faz **append** declarativamente, não substituição cega.

- [ ] **REQ-12**: GIVEN cada flavor gerado, WHEN o serviço resultante compila, THEN
  `go build ./...` passa e `make lint` passa no projeto gerado. (Validação pós-scaffold é
  automática via smoke test.)

## Test Plan

### Use Case Tests (código do CLI)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-1 | happy | flavor registry lookup `crud` | retorna flavor válido |
| TC-UC-02 | REQ-5 | validation | flavor registry lookup `invalid` | erro com lista de flavors |
| TC-UC-03 | REQ-2 | happy | CLI sem flag defaulta para crud | flavor=crud |
| TC-UC-04 | REQ-11 | happy | overlay merge: flavor adiciona target no Makefile base | resultado contém ambos |
| TC-UC-05 | REQ-11 | edge | overlay merge: flavor sobrescreve explicitamente | valor do flavor prevalece, com warning |
| TC-UC-06 | REQ-11 | business | overlay tenta sobrescrever sem marker explícito | erro "conflito não resolvido" |
| TC-UC-07 | REQ-7 | happy | flavor crud inclui k6 baseline | arquivo existe no scaffold |
| TC-UC-08 | REQ-8 | happy | flavor event-processor inclui semgrep event-processor.yml | arquivo existe |
| TC-UC-09 | REQ-9 | happy | flavor data-pipeline inclui benchmark | arquivo existe |

### E2E Tests (scaffold de serviço de verdade)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-1, REQ-12 | happy | `gopherplate new test-crud --flavor crud` + `go build ./...` | build passa |
| TC-E2E-02 | REQ-3, REQ-12 | happy | `gopherplate new test-ep --flavor event-processor` + `go build ./...` | build passa |
| TC-E2E-03 | REQ-4, REQ-12 | happy | `gopherplate new test-dp --flavor data-pipeline` + `go build ./...` | build passa |
| TC-E2E-04 | REQ-1, REQ-12 | happy | após scaffold crud, `make lint` | passa |
| TC-E2E-05 | REQ-3, REQ-12 | happy | após scaffold event-processor, `make lint` | passa |
| TC-E2E-06 | REQ-4, REQ-12 | happy | após scaffold data-pipeline, `make lint` | passa |
| TC-E2E-07 | REQ-5 | validation | `gopherplate new x --flavor foo` | exit != 0, msg lista flavors |
| TC-E2E-08 | REQ-6 | happy | `gopherplate new --help` | stdout inclui `--flavor` e os 3 valores |
| TC-E2E-09 | REQ-8 | business | scaffold event-processor sem retry/DLQ no handler | semgrep do scaffold falha |
| TC-E2E-10 | REQ-9 | business | scaffold data-pipeline sem idempotency_key | semgrep do scaffold falha |

### Smoke Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-1 | happy | após scaffold crud, `make docker-up && make dev` sobe | app responde em :8080 |

Test Plan rigor check: 12 REQs → 20 TCs. Error/edge TCs (11) maior que happy (9). Cada flavor
tem TCs de build + lint. Overlay merge tem happy + conflict + explicit override.

## Design

### Architecture Decisions

- **Flavor = struct com (a) metadata + (b) conjunto de overlays de template + (c) validações
  pós-scaffold.** Vive em `cmd/cli/internal/flavors/`. Registry central em `registry.go`.

- **Overlays declarativos, não imperativos.** Cada overlay descreve:
  - `path`: caminho relativo no scaffold.
  - `action`: `create` (novo arquivo), `append` (adicionar ao final), `insert-marker` (inserir
    em marcador específico do base template, ex: `<!-- @flavor-makefile-targets -->`),
    `overwrite` (substituir, requer justificativa em comentário).
  - `template`: corpo do template Go text/template.

- **Base template é o que `gopherplate new` produz hoje.** Refator: extrair em
  `cmd/cli/internal/flavors/base/` como flavor implícito que todos herdam.

- **Flavor `crud` é um no-op explícito**. Herda tudo do base + adiciona apenas os overlays que
  vêm das outras 4 specs (k6 baseline, regras semgrep padrão, workflow perf). Serve como
  prova de conceito da mecânica sem introduzir complexidade de topologia.

- **`event-processor` vs. `crud`**: principais deltas:
  - `cmd/consumer/main.go` novo (em lugar de `cmd/api/main.go`, ou além dele).
  - Adiciona `internal/infrastructure/messaging/` (Kafka ou Redis Streams — NEEDS
    CLARIFICATION abaixo).
  - `.semgrep/event-processor.yml`.
  - Adapta `tests/load/baselines/smoke.json` para cenário de publish+consume.

- **`data-pipeline` vs. `crud`**: principais deltas:
  - `cmd/worker/main.go` novo.
  - Domínio scaffoldado é `internal/domain/job/` em vez de `user/`.
  - `benchmark_test.go` no use case principal.
  - Substitui workflow de perf-regression por workflow de benchmark.

- **Validação pós-scaffold automática**: após gerar o scaffold, o CLI roda `go build ./...` no
  diretório gerado como smoke test. Se falhar, scaffold é abortado com mensagem clara
  (ferramenta de dev, não produção — tempo extra aceitável).

- **Template rendering via Go `text/template`** — já usado pelo CLI hoje, zero dependência nova.

### Files to Create

- `cmd/cli/internal/flavors/registry.go` — registry + lookup.
- `cmd/cli/internal/flavors/flavor.go` — tipos `Flavor`, `Overlay`, `Action`.
- `cmd/cli/internal/flavors/overlay.go` — merge engine (create, append, insert-marker,
  overwrite).
- `cmd/cli/internal/flavors/overlay_test.go` — unit tests.
- `cmd/cli/internal/flavors/base/` — definição do base flavor (extraído do scaffold atual).
- `cmd/cli/internal/flavors/crud/flavor.go` — registro do flavor crud + overlays.
- `cmd/cli/internal/flavors/crud/templates/` — overlays específicos.
- `cmd/cli/internal/flavors/eventprocessor/flavor.go` — registro + overlays.
- `cmd/cli/internal/flavors/eventprocessor/templates/cmd/consumer/main.go.tmpl`
- `cmd/cli/internal/flavors/eventprocessor/templates/internal/infrastructure/messaging/` (mínimo
  viável)
- `cmd/cli/internal/flavors/eventprocessor/templates/.semgrep/event-processor.yml`
- `cmd/cli/internal/flavors/datapipeline/flavor.go`
- `cmd/cli/internal/flavors/datapipeline/templates/cmd/worker/main.go.tmpl`
- `cmd/cli/internal/flavors/datapipeline/templates/.semgrep/data-pipeline.yml`
- `cmd/cli/internal/flavors/datapipeline/templates/internal/usecases/job/benchmark_test.go.tmpl`
- `docs/guides/cli-flavors.md` — como funcionam, como adicionar novo.

### Files to Modify

- `cmd/cli/cmd/new.go` — adicionar flag `--flavor` + dispatch para registry.
- `cmd/cli/cmd/new_test.go` — cobrir TC-UC-01..09.
- `cmd/cli/internal/scaffold/` — refatorar para receber Flavor e aplicar overlays sobre o base.
- `docs/guides/template-cli.md` — documentar flag `--flavor`.
- `README.md` — mencionar os 3 flavors.
- `docs/harness.md` — adicionar seção "Harness per flavor" (condicional a harness-map existir).

### Dependencies

- Nenhuma externa nova.
- Para `event-processor`: **Redis Streams** (decidido no kick-off). Justificativa: já presente no
  `docker-compose.yml` do projeto, zero setup extra, e `github.com/redis/go-redis/v9` já está no
  `go.mod`. Kafka fica como overlay opcional em iteração futura.

## Tasks

- [ ] **TASK-1**: Definir tipos + registry.
  - `Flavor`, `Overlay`, `Action`, `Registry.Get(name string)`, `Registry.List()`.
  - Testes unitários cobrindo registry lookup.
  - files: `cmd/cli/internal/flavors/flavor.go`, `cmd/cli/internal/flavors/registry.go`,
    `cmd/cli/internal/flavors/registry_test.go`
  - tests: TC-UC-01, TC-UC-02

- [ ] **TASK-2**: Motor de overlay (merge engine).
  - Implementa as 4 ações (create, append, insert-marker, overwrite).
  - `overlay_test.go` com fixtures em memória.
  - files: `cmd/cli/internal/flavors/overlay.go`,
    `cmd/cli/internal/flavors/overlay_test.go`
  - tests: TC-UC-04, TC-UC-05, TC-UC-06

- [ ] **TASK-3**: Extrair base flavor do scaffold atual.
  - Refactor: mover templates existentes para `cmd/cli/internal/flavors/base/templates/`.
  - Nenhuma mudança funcional.
  - files: `cmd/cli/internal/flavors/base/flavor.go`,
    `cmd/cli/internal/flavors/base/templates/` (movidos do lugar antigo)
  - depends: TASK-1
  - tests: (indireto via TC-E2E-01)

- [ ] **TASK-4**: Implementar flavor `crud`.
  - Herda base + adiciona overlays das outras 4 specs:
    - `tests/load/baselines/smoke.json`
    - `.semgrep/handlers.yml`, `.semgrep/usecases.yml`
    - `.github/workflows/perf-regression.yml`
  - files: `cmd/cli/internal/flavors/crud/flavor.go`,
    `cmd/cli/internal/flavors/crud/templates/`
  - depends: TASK-3
  - tests: TC-UC-07

- [ ] **TASK-5**: Implementar flavor `event-processor`.
  - Skeleton consumer Redis Streams, métrica de lag, semgrep rule, baseline k6 adaptado.
  - files: `cmd/cli/internal/flavors/eventprocessor/flavor.go` + toda a árvore de
    `templates/`
  - depends: TASK-3
  - tests: TC-UC-08

- [ ] **TASK-6**: Implementar flavor `data-pipeline`.
  - Skeleton worker, benchmark, semgrep rule, workflow benchmark.yml.
  - files: `cmd/cli/internal/flavors/datapipeline/flavor.go` + árvore `templates/`
  - depends: TASK-3
  - tests: TC-UC-09

- [ ] **TASK-7**: Integrar flag `--flavor` no comando `new`.
  - Editar `cmd/cli/cmd/new.go`: adicionar flag, validar valor, chamar registry.
  - Atualizar `--help`.
  - files: `cmd/cli/cmd/new.go`, `cmd/cli/cmd/new_test.go`
  - depends: TASK-4, TASK-5, TASK-6
  - tests: TC-UC-03, TC-E2E-07, TC-E2E-08

- [ ] **TASK-8**: Validação pós-scaffold automática.
  - Após scaffold, CLI roda `go build ./...` no diretório gerado como smoke test.
  - Se falhar, imprime erro + não remove (usuário investiga).
  - files: `cmd/cli/cmd/new.go`
  - depends: TASK-7
  - tests: TC-E2E-01, TC-E2E-02, TC-E2E-03

- [ ] **TASK-9**: Smoke E2E dos três flavors.
  - Script (ou teste Go que usa `t.TempDir()`) que: scaffolda, roda `go build`, roda `make
    lint`, valida estrutura mínima esperada.
  - files: `cmd/cli/cmd/flavors_e2e_test.go`
  - depends: TASK-8
  - tests: TC-E2E-04, TC-E2E-05, TC-E2E-06

- [ ] **TASK-10**: Semgrep rule validation no scaffold.
  - Testar que scaffolds `event-processor` e `data-pipeline` têm regras semgrep funcionais
    (TC-E2E-09, TC-E2E-10) — injetar fixture violadora e rodar semgrep.
  - files: `cmd/cli/cmd/flavors_semgrep_test.go`
  - depends: TASK-9
  - tests: TC-E2E-09, TC-E2E-10

- [ ] **TASK-11**: Documentar em `docs/guides/cli-flavors.md` e atualizar outros docs.
  - Cobrir REQ-10: como adicionar novo flavor.
  - Atualizar `docs/guides/template-cli.md` com a flag.
  - Atualizar `README.md`.
  - Atualizar `docs/harness.md` (condicional).
  - files: `docs/guides/cli-flavors.md`, `docs/guides/template-cli.md`, `README.md`,
    `docs/harness.md`
  - depends: TASK-10
  - tests: (docs)

- [ ] **TASK-SMOKE**: Sanidade final.
  - `gopherplate new demo-crud --flavor crud`, `cd demo-crud`, `make setup`, `make dev`, curl em
    endpoint.
  - Repetir para event-processor e data-pipeline (ou documentar como DEFERRED se
    infraestrutura local não permitir).
  - files: (none — execução)
  - depends: TASK-11
  - tests: TC-S-01

## Parallel Batches

```text
Batch 1: [TASK-1, TASK-2]                       — foundation (sem overlap)
Batch 2: [TASK-3]                               — refactor base (depende de TASK-1)
Batch 3: [TASK-4, TASK-5, TASK-6]               — 3 flavors em paralelo (todos dep TASK-3, sem overlap entre si)
Batch 4: [TASK-7]                               — integra CLI (dep de todos os flavors)
Batch 5: [TASK-8]                               — validação pós-scaffold
Batch 6: [TASK-9, TASK-10]                      — E2E tests (paralelo)
Batch 7: [TASK-11]                              — docs
Batch 8: [TASK-SMOKE]                           — sanidade
```

**Overlap de arquivos:**

- `cmd/cli/cmd/new.go`: TASK-7 e TASK-8 — **shared-mutative** (ambos editam mesma função).
  Serializar (Batch 4 e Batch 5).
- `README.md`, `docs/harness.md`: **shared-additive** — TASK-11 já consolida.
- Flavors (`crud/`, `eventprocessor/`, `datapipeline/`): **diretórios exclusivos**, seguro em
  paralelo (Batch 3).

## Validation Criteria

- [ ] `gopherplate new demo-crud --flavor crud` gera scaffold que passa `go build ./...` + `make
  lint`.
- [ ] `gopherplate new demo-ep --flavor event-processor` idem.
- [ ] `gopherplate new demo-dp --flavor data-pipeline` idem.
- [ ] `gopherplate new x --flavor invalid` falha com mensagem clara.
- [ ] `gopherplate new --help` documenta os 3 flavors.
- [ ] `docs/guides/cli-flavors.md` descreve como adicionar novo flavor (REQ-10).
- [ ] Unit tests cobrem registry + overlay merge engine.
- [ ] E2E tests validam build + lint + estrutura para os 3 flavors.
- [ ] Semgrep rules específicas de event-processor e data-pipeline são funcionais no scaffold
  gerado.
- [ ] `make lint` e `make test` passam no projeto `gopherplate` (meta).

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
