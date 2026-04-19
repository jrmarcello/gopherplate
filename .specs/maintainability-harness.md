# Spec: maintainability-harness

## Status: DONE

## Context

Quatro melhorias agrupadas de **maintainability harness** (na taxonomia do Fowler), todas
sensores computacionais de baixa frequência — rodam após integração ou em escala nightly, sem
onerar o loop de desenvolvimento rápido (pre-commit, on-edit, stop hook).

| Gap | Resolução | Estágio |
| --- | --------- | ------- |
| Cobertura mede execução, não verificação | Mutation testing (gremlins) | nightly CI |
| `unused` pega não-referenciado, mas não inalcançável | `golang.org/x/tools/cmd/deadcode` | CI (cada PR) |
| Threshold de cobertura global de 60% é grosseiro | Coverage delta por PR em linhas alteradas | CI (cada PR) |
| Mensagens do `gopls` não orientam correção por LLM | Postprocessor no `lint-go-file.sh` que reescreve diagnósticos em formato "fix by:" | on-edit |

Fowler destaca explicitamente que sensores devem ser otimizados para consumo por LLM — este é o
racional do 4º item. Os três primeiros aplicam o princípio "Keep Quality Left": checks caros vão
para post-integration; checks rápidos e direcionados ficam próximos da edição.

Esta é a **Spec 2 de 5** derivadas da spec mãe `.specs/harness-map.md`.

## Requirements

- [ ] **REQ-1**: GIVEN o projeto, WHEN `make mutation` é executado localmente, THEN roda gremlins
  sobre `internal/...` e imprime report com mutation score por pacote.

- [ ] **REQ-2**: GIVEN o workflow `mutation-nightly.yml`, WHEN é disparado em cron noturno (ex:
  03:00 UTC), THEN roda mutation testing sobre `internal/usecases/...` e publica artefato com
  report. Não falha o build em score baixo (informativo nesta iteração).

- [ ] **REQ-3**: GIVEN `.gremlins.yaml` committado, WHEN a ferramenta é executada, THEN respeita
  config (mutators habilitados, testes ignorados, timeout).

- [ ] **REQ-4**: GIVEN `make deadcode`, WHEN executado, THEN roda
  `golang.org/x/tools/cmd/deadcode` sobre `./...` e lista funções inalcançáveis. Exclui
  `cmd/cli/` (scaffolding engine usa callbacks dinâmicos) e `cmd/api/docs/` (gerado).

- [ ] **REQ-5**: GIVEN um PR com código morto introduzido, WHEN o CI `ci.yml` roda o job
  `deadcode`, THEN o job falha listando as funções não-alcançáveis.

- [ ] **REQ-6**: GIVEN um PR com novo código sem testes, WHEN o job de coverage delta roda, THEN
  compara `coverage.out` do PR contra `main` e comenta no PR as linhas alteradas sem cobertura.
  Falha o job se cobertura nas linhas alteradas for menor que `NEW_CODE_COVERAGE_THRESHOLD`
  (default: `0.70` = 70%).

- [ ] **REQ-7**: GIVEN `lint-go-file.sh`, WHEN o hook pega um diagnóstico conhecido do `gopls`
  (ex: "unused variable", "shadows declaration", "assignment to non-pointer"), THEN reescreve a
  mensagem adicionando linha `>> fix by: <sugestão acionável>` abaixo do diagnóstico original.

- [ ] **REQ-8**: GIVEN o postprocessor de diagnósticos, WHEN encontra um diagnóstico não mapeado,
  THEN imprime mensagem original inalterada (fallback seguro).

- [ ] **REQ-9**: GIVEN a lookup table de diagnósticos → sugestões, WHEN quisermos adicionar novo
  mapeamento, THEN basta editar um único arquivo (`.claude/hooks/gopls-hints.awk` ou similar) sem
  mudar o shell script.

## Test Plan

### Use Case Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-7 | happy | diagnóstico "unused variable" mapeado | output contém `>> fix by:` |
| TC-UC-02 | REQ-7 | happy | diagnóstico "shadows declaration" mapeado | output contém sugestão específica |
| TC-UC-03 | REQ-8 | edge | diagnóstico desconhecido | output idêntico ao input do gopls |
| TC-UC-04 | REQ-7 | edge | múltiplos diagnósticos no mesmo arquivo | todos processados |
| TC-UC-05 | REQ-9 | edge | adição de nova entrada na lookup table | novo diagnóstico reconhecido |

### E2E Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-1 | happy | `make mutation` roda sem crash | exit 0, stdout inclui "mutation score" |
| TC-E2E-02 | REQ-4 | happy | `make deadcode` em branch limpa | exit 0 |
| TC-E2E-03 | REQ-4 | business | `make deadcode` com função morta introduzida | exit != 0, stdout lista a função |
| TC-E2E-04 | REQ-5 | happy | job `deadcode` do CI roda em branch limpa | verde |
| TC-E2E-05 | REQ-5 | business | job `deadcode` falha em PR com código morto | vermelho |
| TC-E2E-06 | REQ-6 | happy | PR sem mudança de código | job coverage delta skip/passa |
| TC-E2E-07 | REQ-6 | business | PR com linhas sem cobertura | comentário no PR + job falha se < threshold |
| TC-E2E-08 | REQ-6 | edge | threshold customizado via env | valor passa a 50%, 60% falha, 70% passa |

### Smoke Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-2 | happy | workflow mutation-nightly dispara no cron | artefato publicado |
| TC-S-02 | REQ-7 | happy | edição Go com gopls warning dispara postprocessor | PostToolUse hook output contém hint |

Test Plan rigor check: 9 REQs → 15 TCs. Erro/edge TCs (8) maior que happy (7). Coverage delta,
mutation, deadcode, e postprocessor — todos têm TCs de happy + business + edge. REQ-3 validado
implicitamente via execução dos outros TCs que dependem do config.

## Design

### Architecture Decisions

- **Mutation testing é informativo, não gate**. Gremlins roda pesado (pode multiplicar tempo de
  teste por 50x). Nightly, publica artefato, não bloqueia PR. Futura evolução pode adicionar
  threshold mínimo.
- **Escopo do mutation restrito a `internal/usecases/...`**. Domain é simples demais (muita
  mutação trivial, baixo sinal). Handlers são melhor cobertos por E2E. Use cases têm lógica
  condicional onde mutation testing brilha.
- **Coverage delta usa `diff-cover`** (Python). Alternativa: `gocovsh` ou script Go custom. Razão
  da escolha: `diff-cover` é maduro, tem comentário no PR pronto via action, e o CI job instala
  em segundos. Aceitamos a dependência Python apenas no runner CI.
- **Deadcode via `golang.org/x/tools/cmd/deadcode`** (oficial). Exclusões explícitas necessárias:
  - `cmd/cli/internal/scaffold/` usa reflection/callbacks dinâmicos que o deadcode marca como
    morto falsamente.
  - `docs/docs.go` é gerado pelo swag.
  - `gen/proto/...` é gerado pelo buf.
- **Postprocessor em awk**, não Go. Razão: é 1 lookup table + regex, awk resolve em 40 linhas
  sem dependência nova. Go seria overkill para text munging desse tamanho.
- **Lookup table é um arquivo separado** (`.claude/hooks/gopls-hints.awk`). Isso atende REQ-9:
  editar mapeamento = editar 1 arquivo sem tocar o shell script.

### Files to Create

- `.gremlins.yaml` — config do gremlins (mutators, timeout, paths).
- `Makefile` additions: targets `mutation`, `deadcode`, `coverage-delta`.
- `.github/workflows/mutation-nightly.yml` — workflow schedule cron.
- `.claude/hooks/gopls-hints.awk` — lookup table diagnóstico → sugestão.
- `.claude/hooks/gopls-hints_test.sh` — smoke test do awk (dado input X, output contém Y).
- `docs/guides/mutation-testing.md` — guia curto de como ler o report.

### Files to Modify

- `.github/workflows/ci.yml`:
  - Novo job `deadcode`.
  - Novo job `coverage-delta` (ou passo adicional dentro de `unit-tests`).
- `.claude/hooks/lint-go-file.sh` — pipe da saída gopls pelo awk de hints.
- `docs/harness.md` — adicionar linhas no inventário (condicional à spec harness-map ter sido
  executada).

### Dependencies

- `github.com/go-gremlins/gremlins` — mutation testing (instalado via `go install`).
- `golang.org/x/tools/cmd/deadcode` — dead code analysis (instalado via `go install`).
- `diff-cover` — Python tool (instalado apenas no CI runner, via `pip install diff-cover`).

## Tasks

- [x] **TASK-1**: Config + Makefile target de mutation testing.
  - Criar `.gremlins.yaml` com mutators default, timeout 10m, target `internal/usecases/...`.
  - Adicionar `make mutation` target.
  - Rodar local para verificar.
  - files: `.gremlins.yaml`, `Makefile`
  - tests: TC-E2E-01

- [x] **TASK-2**: Workflow nightly de mutation testing.
  - `.github/workflows/mutation-nightly.yml` com `schedule: '0 3 * * *'`.
  - Steps: checkout, setup Go, `go install gremlins`, `make mutation`, upload report como
    artefato.
  - files: `.github/workflows/mutation-nightly.yml`
  - depends: TASK-1
  - tests: TC-S-01

- [x] **TASK-3**: Makefile target e job CI de deadcode.
  - `make deadcode`: `deadcode -test ./...` com lista de exclusões.
  - Job `deadcode` no `ci.yml`: instala tool + `make deadcode`.
  - Validar em branch limpa.
  - files: `Makefile`, `.github/workflows/ci.yml`
  - tests: TC-E2E-02, TC-E2E-04

- [x] **TASK-4**: Smoke test do deadcode com função morta induzida.
  - Criar branch temporária, adicionar função `unreachableFoo()` em algum pacote sem call-site,
    rodar `make deadcode` localmente.
  - Confirmar exit != 0 + listagem.
  - Reverter. Não commitar a função.
  - files: (none — execução)
  - depends: TASK-3
  - tests: TC-E2E-03, TC-E2E-05

- [x] **TASK-5**: Job CI de coverage delta.
  - Adicionar step após `unit-tests` que instala `diff-cover` e roda contra
    `coverage.out` do PR vs. `main`.
  - Comentário automático via `mshick/add-pr-comment` action (ou output direto do diff-cover).
  - Threshold env `NEW_CODE_COVERAGE_THRESHOLD=0.70`.
  - files: `.github/workflows/ci.yml`
  - tests: TC-E2E-06, TC-E2E-07, TC-E2E-08

- [x] **TASK-6**: Lookup table de hints gopls + postprocessor awk.
  - `.claude/hooks/gopls-hints.awk` — script awk com mapeamento de pelo menos 10 diagnósticos
    comuns (unused variable, shadows, assigned but not used, missing return, nil deref possible,
    unreachable code, loop variable captured, etc).
  - Fallback: diagnóstico não mapeado imprime-se inalterado.
  - files: `.claude/hooks/gopls-hints.awk`
  - tests: TC-UC-01, TC-UC-02, TC-UC-03, TC-UC-04, TC-UC-05

- [x] **TASK-7**: Integrar awk no `lint-go-file.sh`.
  - Pipe da saída `gopls check` pelo awk de hints antes de imprimir.
  - Manter compatibilidade: se awk não existir ou falhar, output original é impresso.
  - files: `.claude/hooks/lint-go-file.sh`
  - depends: TASK-6
  - tests: TC-S-02

- [x] **TASK-8**: Smoke test do awk postprocessor.
  - Script bash em `.claude/hooks/gopls-hints_test.sh` que simula input de diagnóstico gopls e
    verifica que o awk produz output esperado.
  - Cobre os 5 TC-UC-NN do postprocessor.
  - files: `.claude/hooks/gopls-hints_test.sh`
  - depends: TASK-6
  - tests: (valida TC-UC-01..05)

- [x] **TASK-9**: Documentar em `docs/guides/mutation-testing.md`.
  - Como ler o report, o que é mutation score, como interpretar mutações sobreviventes, quando
    escrever testes novos vs. quando ignorar.
  - files: `docs/guides/mutation-testing.md`
  - tests: (docs)

- [x] **TASK-10**: Atualizar `docs/harness.md` (se harness-map executada) e referências.
  - Adicionar linhas no inventário: mutation-nightly, deadcode, coverage-delta,
    gopls-hints.awk.
  - files: `docs/harness.md` (condicional)
  - depends: TASK-2, TASK-3, TASK-5, TASK-7, TASK-9
  - tests: (docs)

## Parallel Batches

```text
Batch 1: [TASK-1, TASK-3, TASK-5, TASK-6]   — paralelo (arquivos distintos, sem dep)
Batch 2: [TASK-2, TASK-4, TASK-7, TASK-8]   — paralelo (cada um depende de uma task do Batch 1)
Batch 3: [TASK-9]                           — sem dep nas outras (poderia ir no Batch 1, mas
                                              melhor esperar para doc refletir o impl final)
Batch 4: [TASK-10]                          — wiring final em docs/harness.md
```

**Overlap de arquivos:**

- `Makefile`: TASK-1, TASK-3 — ambos adicionam targets distintos. Classificação:
  **shared-additive**. Mitigação: editar em tasks separadas mas na mesma batch só se o diff for
  ortogonal (targets diferentes, linhas diferentes). Como Batch 1 tem TASK-1 E TASK-3, reavaliar:
  mover TASK-3 para Batch 2 serializa o Makefile. Recomendo **serializar**: TASK-3 fica em Batch
  2 (junto com TASK-4, TASK-7, TASK-8), Makefile é modificado por TASK-1 em Batch 1 e por TASK-3
  em Batch 2 — sequencial.
- `.github/workflows/ci.yml`: TASK-3 e TASK-5 — **shared-additive** (jobs novos). Mesma regra:
  serializar em batches diferentes.

**Batches revisadas para respeitar serialização de arquivos shared-additive:**

```text
Batch 1: [TASK-1, TASK-6]                   — Makefile (target mutation) + awk hints
Batch 2: [TASK-2, TASK-3, TASK-8]           — workflow nightly + deadcode (Makefile+ci.yml) + test awk
Batch 3: [TASK-4, TASK-5, TASK-7]           — smoke deadcode + coverage-delta (ci.yml) + integração awk no lint-hook
Batch 4: [TASK-9]                           — docs
Batch 5: [TASK-10]                          — wiring final
```

## Validation Criteria

- [ ] `make mutation` executa e produz report.
- [ ] `make deadcode` passa em branch limpa; falha com função morta induzida.
- [ ] Workflow `mutation-nightly` executa (validar em branch temporária via `workflow_dispatch`).
- [ ] Job `deadcode` do CI passa em branch limpa.
- [ ] Job `coverage-delta` comenta no PR e falha abaixo do threshold.
- [ ] `lint-go-file.sh` imprime `>> fix by:` para diagnósticos mapeados.
- [ ] `lint-go-file.sh` imprime diagnóstico original inalterado para mensagens não mapeadas.
- [ ] `docs/guides/mutation-testing.md` existe e documenta fluxo.
- [ ] `make lint` e `make test` passam.

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Iteration 1 — TASK-1 (2026-04-18)

Criado `.gremlins.yaml` com mutators conservadores, sem thresholds (informativo nesta
iteração), e excludes para `gen/`, `docs/docs.go`, migrations. Adicionado target `make
mutation` que auto-instala via `go install` se binário não existir, rodando sobre
`./internal/usecases/...`. Smoke contra `internal/usecases/role`: 11 mutantes detectados,
5 killed / 4 lived / 2 timeout, efficacy 55.56% — sinal real para melhoria de testes.

### Iteration 2 — TASK-6 (2026-04-18)

Criado `.claude/hooks/gopls-hints.awk` com 22 padrões de diagnóstico gopls mapeados em 6
grupos: unused/unreferenced, shadowing, control flow, nil/unsafe, loop/closure, printf,
style/correctness, type assertion. Cada match imprime a linha original seguida de
`>> fix by: <sugestão acionável>`. Linhas sem match passam inalteradas (fallback-safe). Smoke test
local: 2 diagnósticos conhecidos receberam hints, 1 desconhecido passou unchanged.

### Iteration 3 — TASK-2 + TASK-3 + TASK-8 (2026-04-19)

Batch 2 concluída: workflow `mutation-nightly.yml` (cron 03:00 UTC + workflow_dispatch,
informational), target `make deadcode` + job `ci.yml::deadcode` com filter `(cmd|internal)/`
para excluir falsos positivos em `pkg/` (library API) e `tests/` (test helpers); smoke test
`gopls-hints_test.sh` com 7 asserts cobrindo TC-UC-01..05. actionlint passou em todos os
workflows.

### Iteration 4 — TASK-4 + TASK-5 + TASK-7 (2026-04-19)

Batch 3 concluída: smoke deadcode induzido e revertido via git (exit 1 detectado, revert
limpo); job CI `coverage-delta` com conversão Cobertura + `diff-cover` contra `main`, comenta
no PR via sticky-pull-request-comment, threshold 70% nas linhas alteradas; integração do awk
no `lint-go-file.sh` — descoberto bug latente (passava diretório em vez de arquivo para
`gopls check`), corrigido junto.

### Iteration 5 — TASK-9 + TASK-10 (2026-04-19)

Criado `docs/guides/mutation-testing.md` cobrindo semântica (killed/lived/timeout),
interpretação do score, quando agir/não agir, mutators habilitados, extensão de escopo.
`docs/harness.md` atualizado com 5 novas linhas no inventário (mutation-nightly, deadcode,
coverage-delta, gopls hints postprocessor) e 4 gaps marcados como Resolved. Linha adicionada
ao README.md.

### Iteration 6 — Validação com dados reais (2026-04-19)

Validação ponta-a-ponta requisitada pelo usuário. Findings:

- `make lint`: 0 issues. `make test`: PASS. `make deadcode`: OK.
- `actionlint` em todos os workflows: OK.
- Smoke awk: 7/7 PASS.
- Pipeline `lint-go-file.sh` simulado com input real (Go file com `y := 99` unused): gopls
  detectou, awk enriqueceu com `>> fix by:`, hook saiu com código 2. **Bug latente encontrado
  e corrigido**: hook passava `./DIR` para `gopls check` que exige file path; diagnostics
  silenciosos por anos. Correção simples: passar `$FILE_PATH` direto.
- **Bug de configuração gremlins**: default `timeout-coefficient: 3` era baixo demais nesta
  máquina — produzia 11/11 TIMED_OUT (sem sinal). Elevado para 30 em `.gremlins.yaml`;
  resultado final contra `internal/usecases/role`: 6 killed / 5 lived / 0 timeout, efficacy
  54.55% (consistente com a iteração 1 após o ajuste).
- API real: `/health` verde, `POST /users` cria com UUID v7 legítimo.

### Status final

Todas as 10 tasks concluídas. Dois bugs latentes corrigidos durante a validação com dados
reais (lint-go-file.sh gopls check com dir, gremlins timeout-coefficient default). Todos os
validation criteria validáveis localmente passaram; CI jobs (mutation-nightly,
coverage-delta, ci.yml::deadcode) estruturalmente válidos via actionlint mas só exercitáveis
via push + PR — documentado na próxima iteração de validação pós-merge.
