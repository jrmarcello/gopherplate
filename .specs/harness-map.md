# Spec: harness-map

## Status: DONE

## Context

Baseado no artigo de Martin Fowler ["Harness Engineering for Coding Agents"](https://martinfowler.com/articles/harness-engineering.html),
queremos **documentar e formalizar** o harness atual do gopherplate e **institucionalizar um ciclo
self-steering** de evolução contínua.

Hoje, o projeto possui um harness maduro (CLAUDE.md, `.claude/rules/`, skills, subagents, hooks,
lefthook, golangci-lint, CI, MCP Context7, gopherplate CLI, OTel), mas esse inventário está
**espalhado** por vários arquivos e não existe um mapa único que classifique cada peça segundo os
eixos do Fowler (guide vs. sensor, computational vs. inferential, categoria, estágio). Sem esse
mapa, à medida que o harness cresce:

- Guides e sensors podem entrar em conflito silenciosamente (**harness coherence**, limitação
  citada no artigo).
- Novos contribuidores não sabem onde adicionar uma regra nova (rule? linter? skill? hook? CI?).
- Lacunas ficam invisíveis — não sabemos o que não estamos cobrindo.

Esta spec entrega **apenas documentação e processo** — zero alteração em código Go, hooks,
linters, CI ou CLI. Ela é a **spec 4 de um conjunto de 5** e estabelece o vocabulário usado pelas
outras quatro (que implementam os gaps identificados aqui).

Ordem do conjunto: esta spec (**harness-map**) roda primeiro porque fornece o inventário e a
taxonomia que as outras referenciam. As outras são:

| Spec | Escopo |
| ---- | ------ |
| `.specs/k6-regression-gate.md` | Baseline k6 + gate de p95 no CI (architecture fitness) |
| `.specs/maintainability-harness.md` | Mutation testing + `deadcode` + coverage delta + mensagens gopls LLM-friendly |
| `.specs/behavior-harness.md` | Golden fixtures + `buf breaking` no CI + regras Semgrep custom |
| `.specs/cli-harness-flavors.md` | `gopherplate new --flavor {crud\|event-processor\|data-pipeline}` |

## Requirements

- [ ] **REQ-1**: GIVEN um leitor novo no projeto, WHEN abre `docs/harness.md`, THEN encontra uma
  introdução curta com as definições do Fowler (guides/sensors, computational/inferential, três
  categorias: maintainability / architecture fitness / behavior) antes do inventário.

- [ ] **REQ-2**: GIVEN o estado atual do projeto, WHEN consulto `docs/harness.md`, THEN o
  inventário classifica **todos** os artefatos existentes de harness nas colunas: Tipo (guide |
  sensor), Execução (computational | inferential), Categoria (maintainability |
  architecture-fitness | behavior | meta), Estágio (pre-commit | on-edit | stop-hook | CI |
  post-integration | continuous), Implementação (caminho do arquivo/skill/hook).

- [ ] **REQ-3**: GIVEN o inventário em `docs/harness.md`, WHEN o leitor cruza com o código do
  repositório, THEN o inventário cobre **no mínimo** os seguintes artefatos (um por linha da
  tabela, ou agrupados quando pertinente):
  - Guides documentais: `CLAUDE.md`, `.claude/rules/go-conventions.md`,
    `.claude/rules/migrations.md`, `.claude/rules/sdd.md`, `.claude/rules/security.md`, todos os
    `docs/guides/*.md`, todos os `docs/adr/*.md`.
  - Skills: `/validate`, `/new-endpoint`, `/fix-issue`, `/migrate`, `/review`,
    `/full-review-team`, `/security-review-team`, `/debug-logs`, `/debug-team`, `/load-test`,
    `/spec`, `/ralph-loop`, `/spec-review`, `/atlassian`.
  - Subagents com memória: `code-reviewer`, `security-reviewer`, `db-analyst`.
  - Hooks: `.claude/hooks/guard-bash.sh`, `.claude/hooks/lint-go-file.sh`,
    `.claude/hooks/validate-migration.sh`, `.claude/hooks/ralph-loop.sh`,
    `.claude/hooks/stop-validate.sh`, `.claude/hooks/worktree-create.sh`,
    `.claude/hooks/worktree-remove.sh`.
  - `lefthook.yml` (pre-commit fmt+lint, pre-push build+test+vulncheck, commit-msg conventional).
  - `.golangci.yml` (cada linter habilitado conta como um sensor).
  - `.github/workflows/ci.yml` (jobs: lint, vulncheck, unit-tests com threshold 60%, e2e-tests) e
    `.github/workflows/release.yml`.
  - MCP servers: Context7.
  - CLI `gopherplate` (cmd/cli): comandos `new`, `add domain`, `add endpoint`, `remove domain`,
    `remove endpoint`, `doctor`, `wiring`, `version`.
  - OTel / telemetry: `pkg/telemetry` (traces + métricas HTTP + pool DB),
    `internal/infrastructure/telemetry` (métricas de negócio).

- [ ] **REQ-4**: GIVEN `docs/harness.md`, WHEN o leitor chega à seção "Gaps conhecidos", THEN
  encontra uma tabela listando explicitamente os quatro conjuntos de gaps cobertos pelas outras
  specs, cada um com: descrição curta, categoria do harness, link relativo para a spec que
  resolve o gap (mesmo que a spec ainda não exista no momento do commit — links quebrados são
  esperados e resolvem-se ao longo da execução das outras specs).

- [ ] **REQ-5**: GIVEN um contribuidor que percebeu uma lacuna no harness, WHEN abre
  `docs/guides/harness-self-steering.md`, THEN encontra: (a) critérios objetivos para abrir uma
  "harness gap note"; (b) template markdown do registro; (c) checklist de revisão periódica do
  inventário; (d) referência ao self-steering loop do Fowler.

- [ ] **REQ-6**: GIVEN `README.md`, WHEN o leitor procura informação sobre engenharia de harness,
  THEN encontra uma seção curta "Harness engineering" apontando para `docs/harness.md` e
  descrevendo em 1–2 frases o que é o harness no contexto do projeto.

- [ ] **REQ-7**: GIVEN `CLAUDE.md`, WHEN o leitor abre a seção "Claude Code Resources", THEN
  encontra referências explícitas a `docs/harness.md` e `docs/guides/harness-self-steering.md`.

- [ ] **REQ-8**: GIVEN a regra "Não inventar guides/sensors que não existem", WHEN o revisor
  cruza o inventário com o repositório, THEN toda linha do inventário corresponde a um artefato
  existente no commit desta spec (nenhum item hipotético ou aspiracional — aspiracional vai na
  seção "Gaps conhecidos").

## Test Plan

**N/A** — esta spec é puramente de documentação e processo. Não há código Go, hook, linter,
script de CI, ou template do CLI alterado. A validação é feita por:

1. Revisão humana de que o inventário em `docs/harness.md` corresponde ao que existe no repo (a
   checklist de REQ-3 serve como critério objetivo).
2. Revisão humana de que `docs/guides/harness-self-steering.md` contém os 4 itens de REQ-5.
3. Inspeção visual de `README.md` e `CLAUDE.md` para confirmar REQ-6 e REQ-7.
4. `make lint` e `make test` continuam passando (nenhum arquivo Go foi tocado, então é smoke
   validation de que a spec não regrediu nada).

Justificativa da exceção à regra de coverage (toda REQ >= 1 TC): a regra se aplica a specs com
código testável. Specs 100% docs-only usam revisão humana como critério de aceite, conforme
`.claude/rules/sdd.md` ("Para specs não-código (config/docs apenas), o Test Plan pode ser `N/A`
com justificativa").

## Design

### Architecture Decisions

- **Zero alteração em código ou config operacional.** Esta spec não modifica `.go`, `.yml` de
  lint/CI, hooks, scripts, ou templates do CLI. A entrega é 2 arquivos novos em `docs/` + 2
  edições pontuais (`README.md` e `CLAUDE.md`).
- **Inventário é um snapshot no tempo do commit.** Não tentamos manter `docs/harness.md`
  auto-atualizado — o `docs/guides/harness-self-steering.md` define o processo de revisão
  periódica.
- **Taxonomia segue 1:1 o vocabulário do Fowler.** Não inventamos categorias próprias — usar o
  mesmo vocabulário do artigo facilita comunicação com a comunidade externa e com novos
  contribuidores que leram o artigo.
- **Gaps conhecidos apontam para specs por nome, não por conteúdo.** Assim, quando cada spec das
  outras 4 for aprovada e começar a executar, o link passa a resolver automaticamente — sem
  precisar editar `docs/harness.md` de novo.
- **Self-steering é leve, não burocrático.** Template de gap note em markdown simples, sem
  integração com Jira/Linear (o projeto não tem isso wired). Revisão periódica é checklist, não
  processo formal.

### Files to Create

- `docs/harness.md` — mapa/inventário completo do harness, cobrindo REQ-1 a REQ-4.
- `docs/guides/harness-self-steering.md` — processo de evolução do harness, cobrindo REQ-5.

### Files to Modify

- `README.md` — nova seção curta "Harness engineering" (REQ-6).
- `CLAUDE.md` — adicionar referências na seção "Claude Code Resources" (REQ-7).

### Dependencies

Nenhuma dependência externa. Nenhum tooling novo.

## Tasks

- [x] **TASK-1**: Criar `docs/harness.md` com introdução + inventário completo.
  - Conteúdo:
    - Seção "O que é harness" citando Fowler e resumindo os 3 eixos (guides/sensors,
      computational/inferential, 3 categorias + meta).
    - Seção "Inventário" com **uma tabela markdown** com as colunas: Artefato, Tipo, Execução,
      Categoria, Estágio, Implementação, Observação. Cada linha corresponde a um artefato (ou
      grupo coeso) da REQ-3.
    - Agrupamento sugerido dentro da tabela (ordem): Guides documentais → Skills → Subagents →
      Hooks → lefthook → golangci-lint → CI → MCP → CLI → OTel.
    - Seção "Gaps conhecidos" com tabela: Gap, Categoria, Spec responsável (link relativo).
  - Restrição: toda linha corresponde a arquivo/config existente no commit (exceto linhas da
    tabela de gaps).
  - files: `docs/harness.md`
  - tests: (none — docs-only)

- [x] **TASK-2**: Criar `docs/guides/harness-self-steering.md` com processo e template.
  - Conteúdo:
    - Seção "Quando abrir uma harness gap note": 3–5 critérios objetivos (bug escapou para prod,
      Stop hook falhando 3x na mesma classe de erro em uma semana, review humano pegou algo que
      o harness deveria pegar, métrica de negócio degradou sem alerta, etc).
    - Seção "Template de harness gap note" — bloco markdown com campos: `Sintoma`, `Categoria
      (maint | arch-fitness | behavior | meta)`, `Guide ou sensor proposto`, `Onde vive (rule |
      skill | hook | linter | CI | CLI | docs)`, `Custo estimado`, `Referências`.
    - Seção "Revisão periódica" — checklist mensal de 5–7 itens (confirmar inventário ainda
      reflete o repo, checar se há gaps novos, revisar histórico de gap notes abertas, etc).
    - Seção "Referência" — link para o artigo do Fowler e para a subseção "Self-Steering Loop"
      citada lá.
  - files: `docs/guides/harness-self-steering.md`
  - tests: (none — docs-only)

- [x] **TASK-3**: Atualizar `README.md` com nova seção curta "Harness engineering".
  - Posição: após a seção que descreve Clean Architecture / template (onde fizer mais sentido no
    fluxo do README atual).
  - Conteúdo: 2–3 frases explicando que o projeto adota o modelo de harness engineering do
    Fowler, com link para `docs/harness.md` e `docs/guides/harness-self-steering.md`.
  - files: `README.md`
  - depends: TASK-1, TASK-2
  - tests: (none — docs-only)

- [x] **TASK-4**: Atualizar `CLAUDE.md` — referências em "Claude Code Resources".
  - Adicionar 1–2 linhas apontando para `docs/harness.md` (mapa do harness) e
    `docs/guides/harness-self-steering.md` (processo de evolução) na seção "Claude Code
    Resources" (próximo à tabela de skills ou na introdução dessa seção).
  - files: `CLAUDE.md`
  - depends: TASK-1, TASK-2
  - tests: (none — docs-only)

## Parallel Batches

```text
Batch 1: [TASK-1, TASK-2]      — paralelo (arquivos distintos, sem dependências)
Batch 2: [TASK-3, TASK-4]      — paralelo (arquivos distintos, ambos dependem de TASK-1 e TASK-2)
```

**Análise de overlap:**

- `docs/harness.md`: exclusivo de TASK-1.
- `docs/guides/harness-self-steering.md`: exclusivo de TASK-2.
- `README.md`: exclusivo de TASK-3.
- `CLAUDE.md`: exclusivo de TASK-4.

Nenhum arquivo é compartilhado entre tasks. Classificação: todos **exclusive**. Não há
shared-additive nem shared-mutative — segurança total para paralelismo dentro de cada batch.

## Validation Criteria

- [ ] `docs/harness.md` existe e contém as 4 seções (definições, inventário, gaps, referências).
- [ ] Inventário em `docs/harness.md` cobre todos os artefatos listados em REQ-3.
- [ ] Cada linha do inventário aponta para um arquivo/skill/hook que **existe** no commit
  (exceção: tabela "Gaps conhecidos", que aponta para specs futuras).
- [ ] `docs/guides/harness-self-steering.md` existe e contém as 4 subseções de REQ-5.
- [ ] `README.md` contém seção "Harness engineering" com link para `docs/harness.md`.
- [ ] `CLAUDE.md` seção "Claude Code Resources" referencia os dois novos docs.
- [ ] `make lint` passa (verifica que nada Go foi tocado acidentalmente).
- [ ] `make test` passa (idem).
- [ ] Revisão humana confirma que o inventário está completo e a taxonomia bate com o artigo.

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Iteration 1 — TASK-1 (2026-04-18)

Criado `docs/harness.md` com introdução baseada em Fowler, inventário completo agrupado
(documental guides, skills, subagents, hooks, lefthook, golangci-lint linters, CI jobs, MCP,
CLI, OTel) e seção "Known gaps" apontando para as 4 specs derivadas. Zero código Go alterado.

### Iteration 2 — TASK-2 (2026-04-18)

Criado `docs/guides/harness-self-steering.md` com: 6 critérios objetivos para abrir gap note,
template markdown com frontmatter, tabela "onde novos controles vivem", checklist mensal de
coerência (7 itens), diagrama do self-steering loop, seção de não-objetivos, e referências ao
Fowler e ao inventário. Zero código Go alterado.

### Iteration 3 — TASK-3 (2026-04-18)

Atualizado `README.md`: (a) adicionadas duas linhas na tabela de guias (harness.md e
harness-self-steering.md); (b) nova seção "Harness engineering" entre "Documentação" e
"Roadmap" com 3 frases explicando o modelo Fowler e linkando para os dois docs. Zero código
Go alterado.

### Iteration 4 — TASK-4 (2026-04-18)

Atualizado `CLAUDE.md`: adicionado parágrafo-chapéu no início da seção "Claude Code Resources"
explicando que skills/subagents/rules/hooks formam o harness do projeto, com links para
`docs/harness.md` e `docs/guides/harness-self-steering.md`. Zero código Go alterado.

### Status final

Todas as 4 tasks da Spec 4 (harness-map) concluídas. Spec completa — aguardando Stop hook
rodar validation (build + lint + test) e transicionar para status DONE.
