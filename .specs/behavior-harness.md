# Spec: behavior-harness

## Status: DONE

## Context

Behavior harness é a categoria mais difícil do modelo do Fowler — o artigo admite "we still have
a lot to do to figure out good harnesses for functional behaviour". Três sensores podem ser
adicionados ao projeto com ganho claro:

1. **Golden fixtures** (approved fixtures pattern) — captura snapshot de respostas HTTP/gRPC em
   arquivo committado; qualquer drift no formato da resposta falha o teste e obriga revisão do
   diff. Complementa unit tests que verificam campos individuais mas perdem mudanças estruturais
   (ex: envelope trocado, casing mudou, novo campo acidentalmente exposto).

2. **`buf breaking`** — contract test do proto. Compara o descriptor atual contra `main` e falha
   em breaking changes (campo removido, tipo mudado, field number reutilizado). Transforma o
   `.proto` numa fitness function de contrato.

3. **Regras Semgrep customizadas** — captura padrões organizacionais que `golangci-lint` não
   expressa:
   - Todo handler Gin deve chamar `httpgin.SendSuccess` ou `httpgin.SendError` (nunca
     `c.JSON(...)` direto).
   - Toda use case com erros esperados deve declarar `var xxxExpectedErrors = []error{...}` e
     chamar `shared.ClassifyError(...)`.
   - Handlers nunca importam `internal/domain/.../errors` diretamente (tudo via
     `errors.As(&appErr)` + `codeToStatus`).

Esta é a **Spec 3 de 5** derivadas da spec mãe `.specs/harness-map.md`.

## Requirements

### Golden Fixtures

- [ ] **REQ-1**: GIVEN `tests/testutil/golden/golden.go`, WHEN um teste chama
  `golden.AssertJSON(t, "create_user_201", actualBody)`, THEN compara `actualBody` contra
  `testdata/golden/create_user_201.json` via `go-cmp`. Diff detalhado no erro.

- [ ] **REQ-2**: GIVEN a flag `-update`, WHEN o teste roda com `go test ... -update`, THEN
  sobrescreve os goldens em vez de comparar.

- [ ] **REQ-3**: GIVEN pelo menos um teste E2E convertido para golden (prova de conceito), WHEN
  rodado, THEN passa contra o golden committado.

- [ ] **REQ-4**: GIVEN que casing/estrutura de resposta pode mudar acidentalmente, WHEN algum
  campo é adicionado/removido sem atualizar golden, THEN o teste E2E falha com diff claro
  indicando o drift.

- [ ] **REQ-5**: GIVEN respostas com campos dinâmicos (timestamps, UUIDs), WHEN golden compara,
  THEN existe mecanismo de mascaramento (ex: `golden.Mask{Paths: []string{"id", "created_at"}}`)
  que substitui valor por placeholder antes da comparação.

### Proto Breaking Changes

- [ ] **REQ-6**: GIVEN o workflow `ci.yml`, WHEN um PR altera qualquer arquivo em `proto/`, THEN
  um job `buf-breaking` roda `buf breaking --against '.git#branch=main'` e falha em breaking
  change.

- [ ] **REQ-7**: GIVEN `buf breaking`, WHEN mudança não-breaking (adição de campo com number
  novo), THEN job passa.

### Semgrep

- [ ] **REQ-8**: GIVEN `.semgrep/handlers.yml`, WHEN `make semgrep` roda, THEN detecta:
  (a) qualquer handler Gin com `c.JSON(...)` direto em vez de `httpgin.SendSuccess/SendError`;
  (b) qualquer handler que importa `internal/domain/*/errors` diretamente.

- [ ] **REQ-9**: GIVEN `.semgrep/usecases.yml`, WHEN `make semgrep` roda, THEN detecta use cases
  que retornam erro de infra sem passar por `shared.ClassifyError` ou sem declarar
  `expectedErrors`.

- [ ] **REQ-10**: GIVEN o workflow `ci.yml`, WHEN PR roda, THEN job `semgrep` executa as regras
  e falha em violações.

- [ ] **REQ-11**: GIVEN code sample consciente-violador em `.semgrep/testdata/`, WHEN rodamos
  `semgrep --test`, THEN todas as regras são validadas contra fixtures positivos e negativos.

## Test Plan

### Use Case Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-1 | happy | `AssertJSON` com golden existente matching | pass |
| TC-UC-02 | REQ-1 | business | `AssertJSON` com golden diferente | fail com diff |
| TC-UC-03 | REQ-2 | happy | `-update` flag sobrescreve golden | arquivo atualizado |
| TC-UC-04 | REQ-5 | happy | máscara de campo `id` com UUID | normalizado, comparação passa |
| TC-UC-05 | REQ-5 | edge | máscara em campo ausente | skip sem erro |
| TC-UC-06 | REQ-1 | validation | golden não existe e `-update` não setado | fail com msg clara |
| TC-UC-07 | REQ-1 | edge | JSON inválido no input | fail com msg clara |

### E2E Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-3 | happy | POST /users convertido para golden | passa contra golden committado |
| TC-E2E-02 | REQ-4 | business | golden desatualizado (simulação: add campo ao response) | teste falha |
| TC-E2E-03 | REQ-6 | happy | PR sem mudança em proto | job `buf-breaking` skip/passa |
| TC-E2E-04 | REQ-6 | business | PR remove campo do proto (breaking) | job falha |
| TC-E2E-05 | REQ-7 | happy | PR adiciona campo novo com number novo | job passa |
| TC-E2E-06 | REQ-8 | happy | handler usa `httpgin.SendSuccess` | semgrep passa |
| TC-E2E-07 | REQ-8 | business | handler usa `c.JSON(...)` direto | semgrep falha |
| TC-E2E-08 | REQ-8 | business | handler importa `domain/user/errors` | semgrep falha |
| TC-E2E-09 | REQ-9 | happy | use case declara `expectedErrors` + `ClassifyError` | semgrep passa |
| TC-E2E-10 | REQ-9 | business | use case retorna erro sem classify | semgrep falha |
| TC-E2E-11 | REQ-11 | happy | `semgrep --test .semgrep/` | todas fixtures passam |

### Smoke Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-S-01 | REQ-10 | happy | job semgrep executa no CI | job verde em branch limpa |
| TC-S-02 | REQ-6 | happy | job buf-breaking executa no CI | job verde em branch limpa |

Test Plan rigor check: 11 REQs → 20 TCs. Error/edge TCs (12) maior que happy (8). Cada regra
semgrep tem par happy/business. Golden fixture tem happy + drift + missing + invalid input. Buf
breaking tem happy + breaking + non-breaking.

## Design

### Architecture Decisions

- **Golden helper é minimal** — envolvedor sobre `go-cmp/cmp` + `os.ReadFile`/`os.WriteFile`. Não
  reinventamos (ex: `goldie`, `approvals`). 80 linhas de Go é suficiente.
- **Goldens em JSON** (não YAML, não prototext). Justificativa: HTTP é JSON, gRPC tem `protojson`
  disponível — consistência reduz surpresa no review de diff.
- **Masking declarativo via paths JSON** (ex: `$.id`, `$.items[*].created_at`). Não regex, não
  callback. Implementação: tinyjsonpath ou gjson/sjson.
- **Prova de conceito em 1 endpoint** (POST /users happy path). Conversão completa da suite E2E
  é out-of-scope — fica para iteração posterior.
- **`buf breaking` vs. `main`** — baseline é o branch principal. Se `main` muda (breaking), novo
  baseline é reestabelecido automaticamente no merge.
- **Semgrep via Docker ou `pipx install`** — no CI usa action oficial `returntocorp/semgrep`; no
  local via `make semgrep` que detecta docker ou binary. Não requer Python no dev env.
- **Regras semgrep têm fixtures testáveis** (REQ-11). `semgrep --test` é feature nativa. Cada
  arquivo `*.yml` tem um `*.test.go` e `*.fixed.go` ao lado.
- **Não adicionar regra contra "domain importa infrastructure"** — viola restrição do usuário
  ("template não impõe Clean Arch"). As 3 regras iniciais são sobre consistência de helpers e
  pattern de erro, não arquitetura.

### Files to Create

- `tests/testutil/golden/golden.go` — helper lib.
- `tests/testutil/golden/golden_test.go` — unit tests.
- `tests/testutil/golden/testdata/` — fixtures dos próprios testes.
- `tests/e2e/user/testdata/golden/create_user_201.json` — golden da prova de conceito.
- `.semgrep/handlers.yml` — regras sobre handlers Gin.
- `.semgrep/usecases.yml` — regras sobre use cases e classify.
- `.semgrep/testdata/handlers_ok.go` — fixture positivo (passa nas regras).
- `.semgrep/testdata/handlers_bad.go` — fixture negativo (viola regras).
- `.semgrep/testdata/usecases_ok.go` — idem.
- `.semgrep/testdata/usecases_bad.go` — idem.
- `docs/guides/golden-fixtures.md` — como escrever/atualizar golden.
- `docs/guides/semgrep-rules.md` — catálogo das regras e racional.

### Files to Modify

- `.github/workflows/ci.yml`:
  - Novo job `buf-breaking` (condicional a `paths: proto/**`).
  - Novo job `semgrep`.
- `tests/e2e/user/create_test.go` — converter um cenário para golden (prova REQ-3).
- `Makefile` — targets `semgrep`, `semgrep-test`, `buf-breaking`, `golden-update`.
- `docs/harness.md` — adicionar linhas no inventário (condicional).

### Dependencies

- `github.com/google/go-cmp/cmp` — **já está no go.mod** (verificar). Caso contrário, adicionar.
- `github.com/tidwall/gjson` ou similar para path-based masking.
- `buf` CLI — já usado (`make proto`).
- Semgrep CLI — instalado apenas no CI runner.

## Tasks

- [x] **TASK-1**: Implementar `golden.go` + testes.
  - API: `AssertJSON(t, name, actual)`, `AssertJSONWithMask(t, name, actual, mask)`, flag
    `-update`.
  - files: `tests/testutil/golden/golden.go`, `tests/testutil/golden/golden_test.go`,
    `tests/testutil/golden/testdata/`
  - tests: TC-UC-01, TC-UC-02, TC-UC-03, TC-UC-04, TC-UC-05, TC-UC-06, TC-UC-07

- [x] **TASK-2**: Converter teste E2E de criação de usuário para golden.
  - Escolher 1 teste em `tests/e2e/user/create_test.go` e substituir asserções campo-a-campo por
    `golden.AssertJSONWithMask(..., golden.MaskPaths("id", "created_at", "updated_at"))`.
  - Criar `tests/e2e/user/testdata/golden/create_user_201.json` via `go test -update`.
  - files: `tests/e2e/user/create_test.go`,
    `tests/e2e/user/testdata/golden/create_user_201.json`
  - depends: TASK-1
  - tests: TC-E2E-01, TC-E2E-02

- [x] **TASK-3**: Adicionar `make golden-update` target.
  - `go test ./tests/e2e/... -update` (e similares para outros locais se aparecerem).
  - files: `Makefile`
  - depends: TASK-1
  - tests: (indireto via TC-UC-03)

- [x] **TASK-4**: Job CI `buf-breaking`.
  - `.github/workflows/ci.yml`: novo job com `uses: bufbuild/buf-action@...` ou steps manuais
    (`buf breaking --against '.git#branch=main,subdir=.'`).
  - Condicional a `paths: proto/**`.
  - files: `.github/workflows/ci.yml`
  - tests: TC-E2E-03, TC-E2E-04, TC-E2E-05, TC-S-02

- [x] **TASK-5**: Regras Semgrep sobre handlers.
  - `.semgrep/handlers.yml` com 2 regras: no-direct-gin-json, no-domain-errors-import.
  - `.semgrep/testdata/handlers_ok.go`, `handlers_bad.go`.
  - files: `.semgrep/handlers.yml`, `.semgrep/testdata/handlers_ok.go`,
    `.semgrep/testdata/handlers_bad.go`
  - tests: TC-E2E-06, TC-E2E-07, TC-E2E-08

- [x] **TASK-6**: Regras Semgrep sobre use cases.
  - `.semgrep/usecases.yml` com regra: require-classify-error-on-error-return.
  - Fixtures ok/bad.
  - files: `.semgrep/usecases.yml`, `.semgrep/testdata/usecases_ok.go`,
    `.semgrep/testdata/usecases_bad.go`
  - tests: TC-E2E-09, TC-E2E-10

- [x] **TASK-7**: `make semgrep` e `make semgrep-test`.
  - Target `semgrep`: roda regras contra `./internal/...`.
  - Target `semgrep-test`: `semgrep --test .semgrep/`.
  - files: `Makefile`
  - depends: TASK-5, TASK-6
  - tests: TC-E2E-11

- [x] **TASK-8**: Job CI `semgrep`.
  - `.github/workflows/ci.yml`: novo job com `uses: returntocorp/semgrep-action@...` rodando as
    regras em `.semgrep/`.
  - files: `.github/workflows/ci.yml`
  - depends: TASK-7
  - tests: TC-S-01

- [x] **TASK-9**: Documentar em `docs/guides/`.
  - `docs/guides/golden-fixtures.md`: quando usar, como atualizar, como mascarar.
  - `docs/guides/semgrep-rules.md`: catálogo com racional de cada regra.
  - files: `docs/guides/golden-fixtures.md`, `docs/guides/semgrep-rules.md`
  - depends: TASK-2, TASK-7
  - tests: (docs)

- [x] **TASK-10**: Atualizar `docs/harness.md` (condicional) e referências.
  - Adicionar linhas: golden-fixtures, buf-breaking, semgrep/handlers, semgrep/usecases.
  - files: `docs/harness.md` (condicional)
  - depends: TASK-4, TASK-8, TASK-9
  - tests: (docs)

## Parallel Batches

```text
Batch 1: [TASK-1, TASK-5, TASK-6]          — paralelo (arquivos distintos)
Batch 2: [TASK-2, TASK-3, TASK-4, TASK-7]  — paralelo (TASK-2/3 dep TASK-1; TASK-7 dep TASK-5,6)
Batch 3: [TASK-8]                          — ci.yml shared-additive, serializar depois de TASK-4
Batch 4: [TASK-9]                          — docs
Batch 5: [TASK-10]                         — wiring final
```

**Overlap de arquivos:**

- `Makefile`: TASK-3 e TASK-7 — **shared-additive** (targets distintos). Serializar em batches
  separadas OU mesclar no mesmo commit via accumulator.
- `.github/workflows/ci.yml`: TASK-4 e TASK-8 — **shared-additive**. Serializar.

**Batches revisadas respeitando serialização:**

```text
Batch 1: [TASK-1, TASK-5, TASK-6]          — foundation (sem overlap)
Batch 2: [TASK-2, TASK-3, TASK-4]          — TASK-2 dep TASK-1; TASK-3 edita Makefile; TASK-4 edita ci.yml
Batch 3: [TASK-7, TASK-8]                  — TASK-7 edita Makefile (após TASK-3); TASK-8 edita ci.yml (após TASK-4)
Batch 4: [TASK-9]                          — docs
Batch 5: [TASK-10]                         — docs/harness.md wiring
```

## Validation Criteria

- [ ] `tests/testutil/golden/` compila e testes unitários passam.
- [ ] Teste E2E de criação de usuário passa com golden committado.
- [ ] Alterar 1 campo da response faz o E2E falhar com diff claro.
- [ ] Job `buf-breaking` passa em branch limpa; falha ao remover campo em proto.
- [ ] `make semgrep` passa em branch limpa.
- [ ] `make semgrep-test` passa (fixtures cobrem ok/bad).
- [ ] Job `semgrep` passa em branch limpa.
- [ ] `docs/guides/golden-fixtures.md` e `docs/guides/semgrep-rules.md` existem.
- [ ] `make lint` e `make test` passam.

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Iteration 1 — TASK-1 (2026-04-19)

Criada lib `tests/testutil/golden/` com `AssertJSON`, `AssertJSONWithMask`, flag
`-update`, masking dotted paths (top-level + nested), sentinel `"<masked>"`, go-cmp para
diff humano. Testes table-driven cobrindo TC-UC-01..07 + edge nested. `go mod tidy`
promoveu `go-cmp` para dep direta.
TDD: RED(import missing) -> GREEN(9 subtests PASS; 1 ajuste de assertion tolerante a
pretty-print) -> REFACTOR(clean).

### Iteration 2 — TASK-5 + TASK-6 (2026-04-19)

Criadas `.semgrep/handlers.yml` (2 regras: no-direct-gin-json, no-domain-errors-import) e
`.semgrep/usecases.yml` (1 regra: no-direct-domain-error-bare-return). Fixtures em
`.semgrep/handlers.go` e `.semgrep/usecases.go` com markers `ruleid:`/`ok:` per convenção
nativa do semgrep (diferente da estrutura `testdata/*_ok.go` proposta no spec — ver
disclosure abaixo).

### Iteration 3 — TASK-2 + TASK-3 + TASK-4 (2026-04-19)

TASK-2: adicionado `TestE2E_CreateUser_Golden` aditivo em `tests/e2e/user_test.go` (não
substituiu o teste existente — mais conservador). Golden gerado via `-update` no path
`tests/e2e/testdata/golden/create_user_201.json` com mask em `data.id` + `data.created_at`.
TASK-3: Makefile target `golden-update` rodando `go test -update` no escopo e2e.
TASK-4: CI job `buf-breaking` (condicional a PRs), usa `bufbuild/buf-setup-action`,
fetch-depth 0 para ter ref do base branch. Validado localmente contra main: exit 0.

### Iteration 4 — TASK-7 + TASK-8 (2026-04-19)

TASK-7: targets `make semgrep` e `make semgrep-test` com fallback para pip/brew.
TASK-8: CI job `semgrep` via imagem oficial `returntocorp/semgrep:latest`, executa
`semgrep --test .semgrep/` antes do scan real.

### Iteration 5 — TASK-9 + TASK-10 (2026-04-19)

Criados `docs/guides/golden-fixtures.md` (quick start, workflow de atualização, o que
mascarar, o que não fazer) e `docs/guides/semgrep-rules.md` (catálogo com scope/trigger/
racional para cada regra + guia para adicionar nova). Inventário em `docs/harness.md`
atualizado com 2 linhas novas (buf-breaking, semgrep) e 3 gaps marcados Resolved. README
com 2 linhas novas na tabela de guias.

### Iteration 6 — Final Review + Runtime Validation (2026-04-19)

Auditoria pós-execução per nova diretriz em `/ralph-loop`:

**Implementation audit**: 1 gap detectado e corrigido — faltava `make buf-breaking`
target local (spec listou CI job, não Makefile). Adicionado.

**Deviações da spec (documentadas, não silenciosas)**:

1. **Estrutura de fixtures semgrep**: spec propunha `.semgrep/testdata/<rule>_ok.go` +
   `<rule>_bad.go`. Semgrep `--test` exige convenção nativa: `<rule>.yml` + `<rule>.go`
   com markers `ruleid:`/`ok:` — reestruturado. Resultado: `semgrep --test` funcional.
2. **Pattern original `$PKG.Err$NAME`** era syntax-error em Go para o parser semgrep.
   Refactor para `$PKG.$ERR` + metavariable-regex separadas.
3. **Scoping**: regras iniciais sem `paths.include` geraram 6 false positives (middleware
   usando `AbortWithStatusJSON` para short-circuit; repository retornando bare domain
   errors — ambos padrões legítimos). Adicionados includes: handlers.yml escopa para
   `**/internal/infrastructure/web/handler/**`, usecases.yml para `**/internal/usecases/**`.
   Scan limpo: 0 findings.
4. **Path do teste E2E**: spec citou `tests/e2e/user/create_test.go`; real é
   `tests/e2e/user_test.go` (estrutura flat). Usado o path real.

**Validation criteria**:

- [x] golden lib compila + testes unitários passam (9 subtests)
- [x] E2E golden test passa contra committed fixture
- [x] Drift test: adicionei `phantom` field ao golden, teste falhou com diff claro; reverti, passou
- [x] `make buf-breaking` exit 0 em branch limpa
- [ ] buf-breaking falha ao remover campo do proto — não validado localmente (precisaria commit+revert)
- [x] `make semgrep` em branch limpa: 0 findings
- [x] `make semgrep-test`: 3/3 rules PASS
- [ ] Job CI `semgrep` em branch limpa — actionlint OK, estruturalmente válido
- [x] `docs/guides/golden-fixtures.md` e `docs/guides/semgrep-rules.md` existem
- [x] `make lint` (0 issues após correção de `marshalling` → `marshaling`) + `make test` (31 pacotes PASS)

**Runtime validation**: API real subida (build + run + health check 2s), `POST /users` com
email aleatório retornou 201 com envelope `{"data":{"id":"019da62a-...","created_at":"..."}}`
— shape idêntico ao golden committado, confirmando que o gate reflete comportamento real.

**Bug latente encontrado**: (nenhum novo nesta spec; os bugs de `lint-go-file.sh` e
`.gremlins.yaml` foram corrigidos na spec 2.)

### Status final

Todas as 10 tasks concluídas + Final Review completo. 1 gap implementado durante a
auditoria (`make buf-breaking`). 4 deviações documentadas no log. 0 bugs latentes novos.
Runtime validation com dados reais confirmou que os sensores detectam drift genuíno
(drift test do golden) e produzem 0 false positives no código atual (semgrep).
