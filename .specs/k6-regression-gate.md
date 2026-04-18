# Spec: k6-regression-gate

## Status: DRAFT

## Context

Hoje o projeto possui um conjunto de smoke tests k6 (`tests/load/`) e o skill `/load-test` que roda
cenários de carga contra a aplicação local. Porém **não existe um gate de regressão**: se alguém
introduz um PR que degrada p95 em 40%, nada falha — o load test roda, imprime números, e os
números são ignorados.

Na taxonomia do Fowler, isto é uma lacuna de **architecture fitness harness**: temos um sensor
(k6 rodando), mas falta o feedback loop que compara output contra baseline e bloqueia regressão.
Fowler cita explicitamente "performance tests feeding back degradation signals" como prática
recomendada.

Esta spec entrega:

1. Baseline de performance committado em `tests/load/baselines/*.json` (um por cenário).
2. Script de comparação (Go) que lê o summary do k6 e falha se p95 degradar além de um threshold.
3. Job de CI que roda cenário smoke, compara, e falha o build em regressão.
4. Makefile target explícito para **atualizar** baseline (`make load-baseline`) — atualização
   nunca é automática, sempre intencional.

Esta é a **Spec 1 de 5** derivadas da spec mãe `.specs/harness-map.md` (seção "Gaps conhecidos").

## Requirements

- [ ] **REQ-1**: GIVEN um cenário k6 (hoje: smoke), WHEN executado com export de summary, THEN
  produz JSON com p50, p95, p99 e RPS por endpoint medidos.

- [ ] **REQ-2**: GIVEN `tests/load/baselines/smoke.json` committado, WHEN `make load-regression`
  roda, THEN executa o cenário smoke + compara summary atual contra baseline e imprime diff
  legível.

- [ ] **REQ-3**: GIVEN `make load-regression`, WHEN p95 do cenário ultrapassa `baseline.p95 *
  (1 + THRESHOLD)`, THEN o comando sai com código != 0 e imprime quais métricas degradaram e em
  quantos %. `THRESHOLD` é configurável via env `PERF_REGRESSION_THRESHOLD` (default: `0.15` =
  15%).

- [ ] **REQ-4**: GIVEN `make load-baseline`, WHEN executado, THEN roda cenário smoke e
  **sobrescreve** `tests/load/baselines/smoke.json` com o summary atual. Nunca é chamado pelo CI
  — apenas por humano deliberadamente.

- [ ] **REQ-5**: GIVEN um PR aberto, WHEN o workflow de performance roda, THEN sobe
  `docker-compose` com app + Postgres + Redis, roda cenário smoke, compara contra baseline e
  falha o job em regressão.

- [ ] **REQ-6**: GIVEN que baselines degradam naturalmente com novas features, WHEN um PR
  legítimo sobe p95 acima do threshold, THEN o autor atualiza o baseline localmente com `make
  load-baseline` e committa o novo JSON — o workflow aceita o novo baseline automaticamente (não
  há "aprovação dupla"; o diff do baseline em review é o controle).

- [ ] **REQ-7**: GIVEN múltiplos cenários no futuro (load, stress, soak), WHEN quisermos
  adicionar baseline para cenário `X`, THEN basta commitar `tests/load/baselines/X.json` e
  `make load-regression SCENARIO=X` já funciona — nenhuma mudança no script.

## Test Plan

### Use Case Tests (script de comparação Go)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-3 | happy | summary dentro do threshold | exit 0, mensagem "OK" |
| TC-UC-02 | REQ-3 | happy | summary melhor que baseline | exit 0, mensagem "improved by X%" |
| TC-UC-03 | REQ-3 | business | p95 degrada exatamente no limite (15%) | exit 0 (limite inclusivo: `<=`) |
| TC-UC-04 | REQ-3 | business | p95 degrada 15.01% | exit != 0, mensagem lista métrica degradada |
| TC-UC-05 | REQ-3 | business | p99 degrada mas p95 não | exit != 0 (comparação abrange p95 e p99) |
| TC-UC-06 | REQ-3 | edge | baseline sem endpoint que agora aparece no summary | warning (endpoint novo), exit 0 |
| TC-UC-07 | REQ-3 | edge | summary sem endpoint que está no baseline | exit != 0 ("endpoint X ausente") |
| TC-UC-08 | REQ-3 | validation | arquivo de baseline malformado (JSON inválido) | exit != 0 com mensagem clara |
| TC-UC-09 | REQ-3 | validation | arquivo de summary ausente | exit != 0 |
| TC-UC-10 | REQ-3 | edge | threshold customizado via env (0.05) | 6% de regressão falha |
| TC-UC-11 | REQ-7 | happy | `SCENARIO=load` com baseline próprio | usa `tests/load/baselines/load.json` |

### E2E Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-2 | happy | fluxo completo: sobe app, roda smoke, compara baseline | comando termina com exit 0 |
| TC-E2E-02 | REQ-5 | happy | workflow CI roda e passa em branch limpa | job verde |

### Smoke Tests (k6)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-1 | happy | smoke scenario exporta JSON válido com p50/p95/p99 | JSON existe, tem as chaves esperadas |

Test Plan rigor check: 7 REQs → 14 TCs (11 UC + 2 E2E + 1 smoke), erro/edge TCs (7) maior que
happy (4). Cobertura de threshold (limite, acima, abaixo), ausência/presença de endpoint, JSON
malformado, env var — todos os branches principais do compare tool cobertos.

## Design

### Architecture Decisions

- **Compare tool em Go**, não bash+jq. Justificativa: testável com `go test`, tipo-seguro,
  integra com o resto do projeto (sem dependência de `jq` no runner). Vive em
  `tests/load/cmd/perfcompare/main.go`.
- **Summary k6 é a fonte de verdade**. `k6 run --summary-export=summary.json` já é suporte
  nativo. Não reinventamos coleta de métricas.
- **Baseline é por cenário**, não por endpoint. Arquivo `tests/load/baselines/<scenario>.json`.
  Dentro, o compare tool navega por endpoint e métrica.
- **Threshold único aplicado a p95**. p99 é comparado mas com threshold 2x (mais ruidoso por
  natureza). p50 informativo, não gate.
- **Workflow CI opcional no PR, obrigatório em main**. Performance não deve bloquear hotfix —
  roda em `push: main` e em `pull_request` com label `perf`. Adicionar como job separado, não
  dentro de `ci.yml`.
- **Docker-compose no CI**: reaproveita `docker-compose.yml` do projeto. App sobe via `make
  docker-up && go run ./cmd/api &` com health-check antes do k6 rodar.
- **Não integrar com métricas externas** (Prometheus, Datadog). O baseline local committado é
  suficiente para o template — integrações específicas ficam para derivados.

### Files to Create

- `tests/load/baselines/.gitkeep` — diretório existe vazio até `make load-baseline` rodar uma vez.
- `tests/load/baselines/smoke.json` — committado após primeira execução de `make load-baseline`.
- `tests/load/cmd/perfcompare/main.go` — CLI Go que compara summary vs. baseline.
- `tests/load/cmd/perfcompare/compare.go` — lógica pura de comparação (testável).
- `tests/load/cmd/perfcompare/compare_test.go` — table-driven tests cobrindo TC-UC-01..11.
- `tests/load/cmd/perfcompare/testdata/` — fixtures JSON (baseline + summary) para testes.
- `.github/workflows/perf-regression.yml` — novo workflow, roda em `push: main` e PRs com label
  `perf`.

### Files to Modify

- `Makefile` — adicionar targets `load-baseline`, `load-regression` (com flag `SCENARIO`).
- `tests/load/main.js` — garantir que o scenario exporta summary (flag `--summary-export` no
  invocador, não no script — mas documentar em comentário).
- `docs/harness.md` — adicionar linha no inventário (após spec harness-map ter sido executada).
  `NOTA:` esta modificação só ocorre se harness-map já foi mergeada; caso contrário é pulada.

### Dependencies

- k6 (já instalado via `make setup`).
- Go 1.23+ (já requisito do projeto).
- Nenhuma dependência externa nova.

## Tasks

- [ ] **TASK-1**: Implementar lógica pura de comparação em
  `tests/load/cmd/perfcompare/compare.go` + testes.
  - Função `Compare(baseline, summary K6Summary, threshold float64) Report`.
  - Struct `Report` com campos `Passed bool`, `Regressions []Regression`, `Improvements []...`,
    `NewEndpoints []...`, `MissingEndpoints []...`.
  - Table-driven tests cobrem TC-UC-01..11.
  - files: `tests/load/cmd/perfcompare/compare.go`, `tests/load/cmd/perfcompare/compare_test.go`,
    `tests/load/cmd/perfcompare/testdata/baseline_ok.json`,
    `tests/load/cmd/perfcompare/testdata/summary_regression.json`, (mais fixtures conforme TCs)
  - tests: TC-UC-01, TC-UC-02, TC-UC-03, TC-UC-04, TC-UC-05, TC-UC-06, TC-UC-07, TC-UC-08,
    TC-UC-09, TC-UC-10, TC-UC-11

- [ ] **TASK-2**: Implementar CLI wrapper `main.go` que lê baseline + summary do disco, chama
  `Compare`, imprime report e sai com código apropriado.
  - Flags: `--baseline`, `--summary`, `--threshold` (fallback para env
    `PERF_REGRESSION_THRESHOLD`, default 0.15).
  - files: `tests/load/cmd/perfcompare/main.go`
  - depends: TASK-1
  - tests: (coberto indiretamente por E2E)

- [ ] **TASK-3**: Adicionar Makefile targets.
  - `load-baseline`: roda k6 smoke com `--summary-export` e sobrescreve
    `tests/load/baselines/smoke.json`. Parametrizável por `SCENARIO=X`.
  - `load-regression`: roda k6 + perfcompare. Parametrizável por `SCENARIO=X`.
  - files: `Makefile`
  - depends: TASK-2
  - tests: (coberto por TC-E2E-01)

- [ ] **TASK-4**: Gerar baseline inicial committado.
  - Executar `make load-baseline` localmente após TASK-3 (ou em um runner limpo).
  - Committar `tests/load/baselines/smoke.json`.
  - files: `tests/load/baselines/smoke.json`
  - depends: TASK-3
  - tests: (smoke validation manual)

- [ ] **TASK-5**: Adicionar workflow de CI `.github/workflows/perf-regression.yml`.
  - Triggers: `push` em `main`, `pull_request` com label `perf`.
  - Steps: checkout → setup Go → `make docker-up` → rodar app em background → health-check →
    `make load-regression`.
  - files: `.github/workflows/perf-regression.yml`
  - depends: TASK-4
  - tests: TC-E2E-02

- [ ] **TASK-6**: Documentar uso no README e no docs/harness.md.
  - Nova subseção em `README.md` ou `docs/guides/`: "Performance regression gate" com instruções
    de como atualizar baseline, rodar local, e semântica do threshold.
  - Se `docs/harness.md` existir (spec harness-map executada), adicionar linha no inventário.
  - files: `docs/guides/perf-regression.md`, `README.md`, `docs/harness.md` (condicional)
  - depends: TASK-5
  - tests: (docs)

- [ ] **TASK-SMOKE**: Validar ponta-a-ponta.
  - Rodar `make load-regression` localmente — deve passar contra o baseline recém-committado.
  - Simular regressão: injetar `time.Sleep(50*time.Millisecond)` temporariamente em um handler,
    rodar `make load-regression` — deve falhar.
  - Reverter mudança.
  - files: (none — execução only)
  - depends: TASK-5
  - tests: TC-S-01

## Parallel Batches

```text
Batch 1: [TASK-1]                  — foundation (lib + testes)
Batch 2: [TASK-2]                  — CLI wrapper (depends: TASK-1)
Batch 3: [TASK-3]                  — Makefile (depends: TASK-2)
Batch 4: [TASK-4]                  — gerar baseline (depends: TASK-3)
Batch 5: [TASK-5]                  — CI workflow (depends: TASK-4)
Batch 6: [TASK-6, TASK-SMOKE]      — paralelo: docs + smoke validation (depends: TASK-5)
```

Pipeline linear, sem paralelismo interno significativo. Trade-off: manter correto > ganho de
paralelismo (baseline JSON precisa existir antes do CI; CI precisa existir antes da doc final).

Overlap de arquivos: nenhum compartilhado entre tasks na mesma batch.

## Validation Criteria

- [ ] `make load-regression` retorna exit 0 contra baseline committado (happy path).
- [ ] `make load-regression` retorna exit != 0 em regressão induzida manualmente.
- [ ] Unit tests do `perfcompare` cobrem todos os TC-UC-NN definidos.
- [ ] Workflow `perf-regression.yml` passa em branch limpa.
- [ ] `make load-baseline` sobrescreve o JSON corretamente.
- [ ] Doc `docs/guides/perf-regression.md` descreve fluxo de atualização e semântica do
  threshold.
- [ ] `make lint` e `make test` passam.

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->
