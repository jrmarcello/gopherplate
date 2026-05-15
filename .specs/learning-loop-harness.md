# Spec: learning-loop-harness

## Status: DONE

## Context

Implementa um **closed-loop learning system** inspirado no Hermes Agent (vídeo do
Lucas Montano: https://youtu.be/7R-LAADt6rY, 5:40–10:40), aplicado ao **nosso
harness atual** — não é serviço Go pra outros agentes consumirem. Objetivo: melhorar
DX com IA neste projeto fazendo o harness se auto-evoluir.

Cinco etapas do loop:

1. **Task Completion** — gatilho disparado quando uma spec passa `IN_PROGRESS → DONE`
   via `/ralph-loop`. Evento registrado em SQLite local.
2. **Pattern Extraction** — sobre as fontes (`.specs/*.md` execution logs, git log,
   memory MD, transcripts JSONL do Claude Code em
   `~/.claude/projects/<dir>/*.jsonl`), o binário `tools/learn` gera **candidatos**
   determinísticos (n-gramas de tool calls, sequências de arquivos editados, padrões
   commit→test→fix) com frequência ≥ threshold. Output ordenado lexicograficamente
   por `(kind, signature)` pra reprodutibilidade byte-a-byte.
3. **Skill Creation** — skill agentic `/learn-extract` consome candidatos, Claude
   triagem (qualidade-first: foco em repetição, especificidade, anti-generalização,
   modularidade — rubrica do vídeo 8:13–8:40), gera arquivo MD em `.claude/skills/`
   (skill nova), `memory/` (insight curado) ou descarta.
4. **Skill Refinement** — skill agentic `/learn-refine` usa `tools/learn similar`
   (FTS5 BM25 + edit distance) pra propor merges; Claude decide consolidação,
   evita duplicidade. Decisão aplicada **apenas após aprovação explícita** do
   usuário (presente-before-commit, idêntico ao `/ralph-loop`).
5. **Periodic Nudge** — counter no SQLite incrementado pelo gatilho da etapa 1;
   ao atingir `NUDGE_THRESHOLD` (default 5 specs DONE), dispara skill agentic
   `/learn-nudge`. Skill autoavalia: o que persistir, TTL pra info obsoleta, sugere
   deprecations. Também invocável manual. **Reset do counter é deterministico**
   (binário, no fim de `nudge-tick`), não LLM.

Três camadas de memória (alinhadas com vídeo 9:10–9:50):

- **Sessão**: contexto da conversa atual (gerenciado pelo Claude Code, fora do escopo).
- **Persistente**: `memory/*.md` + `memory/MEMORY.md` (já existe, curado).
- **Skills**: `.claude/skills/<name>/SKILL.md` (já existe, 14 skills atuais).

**Closure do loop (consumo do KB)**: sem retrieval, etapas 1–5 produzem mas
ninguém consome. A spec inclui:

- Hook `UserPromptSubmit` chamando `tools/learn recall "<prompt>"` que injeta top-K
  matches (BM25 sobre FTS5) como `<system-reminder>` no contexto.
- O **mesmo hook** chama `learn track-use --paths <list>` **após** a injeção, com a
  lista dos paths que ele acabou de surfacear — registrando que esses paths foram
  apresentados ao Claude. (Não é "Claude consumiu", é "foi apresentado"; aproximação
  suficiente pra TTL).
- Skill manual `/learn-recall <query>` pra busca explícita com filtros. A skill
  também chama `learn track-use` antes de retornar.

**Storage**: SQLite com FTS5 em `.claude/learning/db.sqlite` (gitignored — privado
por dev). Conexão sempre aberta em **WAL mode** (`PRAGMA journal_mode=WAL`) +
`PRAGMA busy_timeout=5000` pra concorrência: writers múltiplos (hooks
simultâneos) não falham com `SQLITE_BUSY` imediato. Skills geradas
(`.claude/skills/`) e memory MD (`memory/`) seguem versionados normalmente,
então **o time inteiro se beneficia** das curações, enquanto o índice cru fica
local.

**Encaixe no harness atual** (categorização Fowler — ver
[docs/harness.md](../docs/harness.md)):

- Etapa 1 (`stop-learn.sh` + `complete-task`): **guide computacional** (feedforward:
  registra dado pra alimentar sensores inferenciais posteriores). Categoria meta.
- Etapas 2 (`extract`) e parte de 5 (`nudge-tick` cálculo de TTL): **sensor
  computacional** (analisa output armazenado → produz candidatos / sinais).
- Etapas 3, 4, parte de 5 (skills agentic): **sensor inferencial**. Categoria
  meta+maint.
- Retrieval (hook + `/learn-recall`): **guide inferencial** auto-injetado.

Esta spec é a **6ª** da linha de specs derivadas do `harness-map.md`
(seguindo `behavior-harness`, `maintainability-harness`, `k6-regression-gate`,
`cli-harness-flavors`, `otel-strategy-alignment`).

Princípio diretor (qualidade > velocidade > custo) aplicado:

- LLM-driven onde qualidade textual importa (Skill Creation, Refinement, Nudge);
  heurística determinística pra pre-filtragem barata.
- Retrieval automático **e** manual (não escolher um) — closure do loop é
  obrigatório, manual é escape pra casos de prompt curto/óbvio.
- Anti-generalização codificada na rubrica passada ao Claude — citação direta dos
  pontos 8:13–8:40 do vídeo.
- TTL marca deprecation, **não** apaga — usuário aprova consolidations.
- Versionado no repo, mas DB local — beneficia time sem expor histórico privado
  de cada dev.
- Erros tipados (UsageError vs RuntimeError) pra exit codes corretos; não confiar
  em `strings.Contains(err.Error(), ...)` no dispatcher.
- Determinismo onde possível: pattern extraction output ordenado lexicograficamente
  pra `candidates.jsonl` ser byte-idêntico em runs repetidos.

## Requirements

### Trigger e ingestão

- [ ] **REQ-1**: GIVEN o `/ralph-loop` completa o commit de uma spec e o status
  passa `IN_PROGRESS → DONE`, WHEN o hook `stop-learn.sh` é disparado, THEN ele
  chama `tools/learn complete-task --spec <path> --session <id>`, que insere um
  evento na tabela `events` (campos: `spec_path`, `session_id`, `completed_at`,
  `commit_sha`, `tasks_count`, `outcome=success`).

- [ ] **REQ-2**: GIVEN um evento de task completion registrado, WHEN
  `tools/learn extract` é executado, THEN o binário lê as 4 fontes
  (execution logs em `.specs/*.md`, git log + diffs do commit, transcripts JSONL
  da sessão, memory MD existente) e produz `candidates.jsonl` com sequências
  de freq ≥ `MIN_PATTERN_FREQ` (default 3). Output **ordenado
  lexicograficamente por `(kind, signature)` antes de serializar** —
  reprodutibilidade byte-a-byte em runs repetidos com mesmo input.

- [ ] **REQ-2a**: GIVEN o schema de `candidates.jsonl`, THEN cada linha é JSON com
  campos tipados: `kind: string ∈ {tool-sequence, file-sequence, commit-pattern,
  error-fix}`; `signature: string`; `frequency: int (≥ MIN_PATTERN_FREQ)`;
  `contexts: []string` (spec_paths/session_ids); `score: float64 ∈ [0.0, 1.0]`
  (frequência normalizada); `first_seen_at: RFC3339`; `last_seen_at: RFC3339`.
  Schema documentado em `tools/learn/internal/pattern/schema.go` (tipo Go) e em
  `docs/guides/learning-loop.md`.

- [ ] **REQ-3**: GIVEN o parser de transcripts JSONL, WHEN um arquivo de transcript
  inacessível ou corrompido é encontrado, THEN o parser pula o arquivo, loga aviso
  estruturado (`level=warn`, `source=transcript`, `reason=...`), e continua com as
  fontes restantes. **Não falha** o pipeline de extração.

- [ ] **REQ-4**: GIVEN privacidade, WHEN qualquer fonte é processada, THEN
  sanitização **em memória, antes de qualquer write em disco** (incluindo
  `candidates.jsonl`, `decisions`, FTS5). Padrões default cobrem:
  (a) AWS keys (`AKIA[0-9A-Z]{16}`);
  (b) tokens prefixados — OpenAI (`sk-[A-Za-z0-9]{20,}`), OpenAI project
      (`sk-proj-[A-Za-z0-9_-]{20,}`), Anthropic (`sk-ant-[A-Za-z0-9_-]{20,}`),
      GitHub PAT (`ghp_[A-Za-z0-9]{36}`), GitHub fine-grained (`github_pat_[A-Za-z0-9_]{82}`),
      Slack (`xox[abprs]-[A-Za-z0-9-]+`);
  (c) paths absolutos contendo `/.ssh/`;
  (d) linhas de `.env` no formato `KEY=value` (chave em UPPER_CASE, valor
      qualquer não-vazio).
  Valor sanitizado substituído por `<REDACTED:kind>` onde `kind` ∈
  `{aws_key, token, ssh_path, env_value}`. Policy explícita: aceitamos falso
  positivo (sanitizar uma string que parece-mas-não-é) em troca de zero falso
  negativo de classes conhecidas. Padrões adicionáveis via `config.secret_patterns`.

### Skill Creation (etapa 3)

- [ ] **REQ-5**: GIVEN `candidates.jsonl` produzido pela etapa 2, WHEN o usuário
  invoca `/learn-extract`, THEN a skill carrega os candidatos, aplica a **rubrica
  de skill quality** (REQ-6) e Claude decide pra cada candidato uma de 4 ações:
  (a) virar nova skill em `.claude/skills/<name>/SKILL.md`;
  (b) virar entrada de memory em `memory/<name>.md` (+ atualizar `MEMORY.md`);
  (c) atualizar skill/memory existente;
  (d) descartar com razão registrada.
  Para **cada candidato processado**, uma linha é inserida em `decisions`
  (`candidate_signature`, `action ∈ {new-skill, new-memory, update, discard}`,
  `target_path`, `rationale`, `decided_at`). Roteamento de write pra `.claude/`
  ou `memory/` faz `learn record-decision` (deterministic helper) — não o LLM
  diretamente.

- [ ] **REQ-6**: GIVEN a rubrica de skill quality (vídeo 8:13–8:40), THEN ela está
  documentada em `.claude/rules/skill-quality.md` (arquivo novo) e citada
  **literalmente como bloco** em `learn-extract`, `learn-refine` e
  `learn-audit-skills` (não apenas referenciada). Os 4 critérios com **anchor
  examples** (1 exemplo positivo + 1 negativo por critério, pra calibrar scoring 1–5):
  (1) Foco em repetição (cobre tarefa do 80% do trabalho repetitivo, não exceções);
  (2) Anti-generalização (skills delimitadas; rejeitar candidatos vagos);
  (3) Modularidade (1 problema, 1 arquivo MD, invocável via FTS5);
  (4) Refinabilidade (estrutura que permite merge — frontmatter + seções nomeadas).

- [ ] **REQ-7**: GIVEN uma skill nova proposta por `/learn-extract`, THEN o
  frontmatter inclui campos obrigatórios: `name`, `description`,
  `learning_provenance` (signatures dos candidatos que originaram), `created_at`
  (RFC3339), `last_reviewed_at` (RFC3339). Todos os 5 campos validados por
  `learn validate-skill <path>` antes do write. Conteúdo segue o template
  `.claude/skills/_template/SKILL.md`.

### Skill Refinement (etapa 4)

- [ ] **REQ-8**: GIVEN uma skill recém-criada ou já existente, WHEN
  `tools/learn similar --skill <path>` é executado, THEN retorna top-K skills
  similares (K=5 default) com score BM25 da query FTS5 sobre `skill_index`,
  acompanhado de edit distance entre os corpos. Skills com `score ≥ SIMILARITY_THRESHOLD`
  (default 0.6) são candidatas a merge. Output determinístico (ordenação estável
  por `(-score, path)`).

- [x] **REQ-9** (REVISED — auto-apply policy): GIVEN candidatas a merge
  identificadas, WHEN o usuário invoca `/learn-refine`, THEN Claude apresenta
  cada par com diff + rationale **antes** da apply, e em seguida chama `learn
  refine-apply` que (a) insere row em `decisions` com `action=pending-approval`,
  (b) move o candidato pra `_deprecated/` com header REQ-10, (c) bump da row
  pra `action=applied`, (d) append em `audit.jsonl` (REQ-9a) — tudo numa
  única invocação atômica. **Não há mais gate de aprovação manual no chat**;
  a rede de segurança vira o par (audit log + `/learn-rollback`). `--dry-run`
  no `refine-apply` imprime diff sem mutar nada. Trade-off explícito: trocamos
  approval-per-pair (rigor antes) por reversibilidade post-hoc (rigor depois);
  custo de descobrir merge ruim sobe ligeiramente mas o gargalo de aprovação
  per-pair desaparece, alinhando com o espírito autônomo do Hermes Agent.

- [x] **REQ-9a** (NOVO — audit trail da REQ-9 revisada): GIVEN cada
  `learn apply-decision` ou `learn refine-apply` que mover um arquivo, AND
  cada `learn rollback` bem-sucedido, THEN um JSON line é appendado a
  `<learningDir>/audit.jsonl` com schema fixo:
  `{timestamp (RFC3339Nano), decision_id (int64), action ('applied'|'rolled-back'),
  source_path, deprecated_path, merged_into, candidate_signature, rationale}`.
  Audit é best-effort (falha do append loga warning mas não desfaz a apply —
  audit é observability, não source of truth). Schema documentado em
  `tools/learn/internal/audit/audit.go` (`Entry` struct).

- [x] **REQ-10**: GIVEN o agente nunca silenciosamente deleta skills/memory,
  WHEN um merge acontece, THEN a skill/memory descartada é **movida** para
  `.claude/skills/_deprecated/<name>-<timestamp>.md` (ou `memory/_deprecated/`),
  onde `<timestamp>` = `YYYYMMDDTHHMMSSZ` (UTC, evita colisão entre múltiplas
  deprecations no mesmo segundo). Arquivo movido recebe header inserido na
  primeira linha: `> Deprecated <YYYY-MM-DD> by /learn-refine: merged into
  <target-path>`. Nunca deletado do filesystem. Reversão automatizada via
  `/learn-rollback <decision-id>` (consome `audit.jsonl`, restaura o arquivo
  no path canonical, append entry symmetric `action=rolled-back`).

### Periodic Nudge (etapa 5)

- [ ] **REQ-11**: GIVEN o counter `tasks_since_last_nudge` no SQLite, WHEN
  `tools/learn complete-task` é executado, THEN incrementa o counter via
  `UPDATE nudge_state SET counter = counter + 1` (atômico, não
  read-modify-write). Se counter ≥ `NUDGE_THRESHOLD` (default 5), imprime na
  stderr `LEARN_NUDGE_DUE=true`. **Reset do counter é responsabilidade do
  binário** (`learn nudge-tick --reset`), invocado pela skill `/learn-nudge`
  ao final da apresentação ao usuário, **antes** do prompt de aprovação —
  garante reset mesmo se usuário rejeitar as sugestões. Nunca o LLM altera o
  counter diretamente.

- [ ] **REQ-12**: GIVEN `/learn-nudge` invocado (manual ou após sinal), WHEN
  executado, THEN Claude (a) lê `skill_usage` e `memory_usage` (tabelas
  populadas pelo retrieval da etapa 6), (b) identifica skills/memory **com**
  `created_at < now() - 2 × TTL_DAYS` **AND** `last_used_at < now() - TTL_DAYS`
  (comparação inclusiva — `≤`), (c) propõe ao usuário: consolidations
  adicionais (delegando a `/learn-refine`), deprecations (movimento para
  `_deprecated/`), promoções de memory→skill ou vice-versa. Default
  `TTL_DAYS=90`. Nada aplicado sem aprovação.

- [ ] **REQ-13**: GIVEN o nudge automático nunca deve rodar fora do controle do
  usuário, WHEN o hook detecta `LEARN_NUDGE_DUE=true`, THEN imprime exatamente:
  `Learning nudge due — run /learn-nudge when ready (<N> specs since last
  nudge)` onde `<N>` = `nudge_state.counter`. **Não** invoca a skill
  automaticamente.

### Retrieval (closure)

- [ ] **REQ-14**: GIVEN o hook `UserPromptSubmit` configurado em `settings.json`,
  WHEN o usuário envia um prompt com ≥ `MIN_PROMPT_LEN_FOR_RECALL` chars
  (default 20), THEN o hook chama `tools/learn recall --prompt "<text>"
  --top-k <RECALL_TOP_K> --max-tokens <RECALL_MAX_TOKENS>` (defaults 3 / 500).
  Output JSON com matches (`kind`, `path`, `score`, `summary`) é injetado como
  `<system-reminder>` contendo apenas paths + summaries (não o corpo das
  skills — Claude lê sob demanda). **Wrapped em `timeout 2` (configurável via
  `LEARN_RECALL_TIMEOUT`, default 2s)** — se exceder, hook retorna no-op
  silencioso (não bloqueia o prompt).

- [ ] **REQ-14a**: GIVEN o formato do system-reminder injetado, THEN segue
  template fixo:
  ```text
  <system-reminder>
  Learning loop recall — N relevant artifacts:
  - <path1> (<kind>, score=<X>): <summary1>
  - <path2> (<kind>, score=<X>): <summary2>
  ...
  Read the file(s) above if relevant; ignore otherwise.
  </system-reminder>
  ```
  Formato é estável (cobertura por TC). Mudanças no template requerem PR
  separado.

- [ ] **REQ-15**: GIVEN o retrieval automático nunca pode poluir contexto, WHEN
  nenhum match com `score ≥ RECALL_MIN_SCORE` (default 0.4) é encontrado, THEN
  o hook não injeta nada (sai 0 sem stdout).

- [ ] **REQ-16**: GIVEN skill manual `/learn-recall <query>`, WHEN invocada com
  filtros (`--kind=skill|memory|pattern`, `--since=<duration>`, `--max=<int>`),
  THEN o binário retorna lista formatada (não system-reminder injetado, output
  direto no chat). Validação de input: `--kind` ∈ valores enumerados (caso
  contrário, exit 1 com lista de valores válidos); `--since` parseado por
  `time.ParseDuration` (`7d`, `24h`, `30m`); `--max` ≥ 1.

- [ ] **REQ-17**: GIVEN tracking de uso de skills/memory, THEN o **hook
  `user-prompt-submit-recall.sh`** chama `learn track-use --paths <p1,p2,...>`
  **após** injetar o system-reminder, listando os paths que apresentou. O
  binário incrementa `skill_usage.usage_count` (ou `memory_usage`) e atualiza
  `last_used_at` pra cada path. A skill manual `/learn-recall` invoca o mesmo
  `track-use` antes de retornar resultado pro chat. **Não há heurística passiva
  sobre o corpo da mensagem do Claude** — o tracking é explícito, baseado em
  "apresentado ao Claude", suficiente pra TTL. Tracking best-effort: falha do
  binário em `track-use` não bloqueia o retrieval.

### Storage e indexação

- [ ] **REQ-18**: GIVEN `tools/learn init` (executado por `make learn-setup`),
  WHEN executado pela primeira vez, THEN cria `.claude/learning/db.sqlite` com
  schema completo: tabelas `events`, `patterns`, `candidates`, `decisions`,
  `skill_index`, `memory_index`, `skill_usage`, `memory_usage`, `nudge_state`,
  `config`; virtual tables FTS5 `skill_fts`, `memory_fts`, `pattern_fts` com
  triggers de sincronização (AFTER INSERT/UPDATE/DELETE em cada tabela base);
  conexão sempre aberta com `PRAGMA journal_mode=WAL` e
  `PRAGMA busy_timeout=5000`; `.claude/learning/` adicionado ao `.gitignore`.

- [ ] **REQ-19**: GIVEN `tools/learn reindex` (full ou incremental via flag
  `--since-mtime` ou `--path <file>`), WHEN executado, THEN re-popula
  `skill_index` e `memory_index` + FTS5 a partir do filesystem
  (`.claude/skills/**/SKILL.md` e `memory/*.md`, **excluindo
  `**/_deprecated/**`** tanto na população quanto na remoção de órfãos), e
  remove entradas órfãs cujos arquivos não existem mais. Execução **idempotente**
  (re-rodar produz mesmo estado). Implementação usa upsert (`INSERT ... ON
  CONFLICT(path) DO UPDATE`), nunca duplica row por path.

- [ ] **REQ-19a**: GIVEN broken-link warning, WHEN `reindex` encontra uma
  referência `[[name]]` em memory MD que resolve a path em `_deprecated/`,
  THEN loga warn estruturado (`level=warn`, `event=broken_link`, `source=<path>`,
  `target_deprecated=<path>`). Não bloqueia reindex.

- [ ] **REQ-20**: GIVEN o hook PostToolUse já existente, WHEN um arquivo dentro
  de `.claude/skills/**/SKILL.md` ou `memory/*.md` é criado/editado/escrito, THEN
  o hook `reindex-learning.sh` chama `tools/learn reindex --path <file>` pra
  atualizar índice incremental. Falhas do hook **não bloqueiam** o save
  (best-effort, log only).

### Tooling Go

- [ ] **REQ-21**: GIVEN o binário `tools/learn`, WHEN compilado, THEN é Go puro
  (`modernc.org/sqlite` ao invés de `mattn/go-sqlite3` pra evitar cgo). Mora em
  **go.mod separado** em `tools/learn/go.mod` pra não acoplar deps do tooling
  ao servidor principal. Isolamento verificável: `go build ./...` do root
  **não** baixa nem compila `modernc.org/sqlite`; `cd tools/learn && go build
  ./...` compila standalone. `make learn-build` instala o binário em
  `bin/learn` (root `bin/` já gitignored). Documentação em
  `docs/guides/learning-loop.md` explica que o uso direto requer
  `export PATH="$PWD/bin:$PATH"` ou alias; hooks chamam por path absoluto
  via `learn-hook-helpers.sh`, então não precisam do PATH.

- [ ] **REQ-22**: GIVEN a CLI `learn`, WHEN executada com `--help`, THEN lista
  subcomandos: `init`, `complete-task`, `extract`, `similar`, `recall`,
  `nudge-tick`, `reindex`, `stats`, `validate-skill`, `record-decision`,
  `apply-decision`, `track-use`. Cada subcomando tem `--help` próprio (exit 0)
  e retorna exit codes documentados (0 sucesso, 1 erro de uso, 2 erro de
  runtime). Classificação de exit code via `errors.As` no root dispatcher pra
  tipos `*learn.UsageError` (exit 1) ou `*learn.RuntimeError` (exit 2); demais
  erros caem em exit 2.

- [ ] **REQ-23**: GIVEN observability, WHEN qualquer subcomando do `learn` é
  executado, THEN emite logs estruturados em JSON (`time`, `level`, `cmd`,
  `event`, key/value específicos) na stderr. `learn stats` retorna **JSON**
  na stdout (não texto), com schema fixo (`counts.events`,
  `counts.patterns`, `counts.candidates`, ..., `last_extract_at`,
  `last_nudge_at`, `top_skills_by_use`, `bottom_skills_by_use`,
  `db_size_bytes`).

### Configuração

- [ ] **REQ-24**: GIVEN configuração centralizada, WHEN o `learn` é executado,
  THEN lê `.claude/learning/config.yml` (criado por `init` com defaults). Campos:
  `min_pattern_freq` (int ≥ 1, default 3), `similarity_threshold` (float ∈
  [0.0, 1.0], default 0.6), `nudge_threshold` (int ≥ 1, default 5), `ttl_days`
  (int ≥ 1, default 90), `recall_min_score` (float ∈ [0.0, 1.0], default 0.4),
  `min_prompt_len_for_recall` (int ≥ 1, default 20), `recall_top_k` (int ≥ 1,
  default 3), `recall_max_tokens` (int ≥ 1, default 500),
  `recall_timeout_seconds` (int ≥ 1, default 2), `llm_model` (string,
  informativo), `secret_patterns` (lista de objetos
  `{kind, pattern}`). Variáveis `LEARN_<UPPER>` (mapping mecânico do nome
  field → upper-case com underscore) sobrepõem o YAML. Validation: tipos
  errados ou valores fora de range → exit 1 com mensagem citando o campo.

### Auditoria one-shot das skills atuais

- [ ] **REQ-25**: GIVEN as 14 skills atuais (`.claude/skills/<name>/SKILL.md`),
  WHEN `/learn-audit-skills` (skill agentic one-shot) é invocada, THEN aplica
  a rubrica do REQ-6 a cada skill, gera relatório em
  `.specs/reports/skill-audit-<YYYY-MM-DD>.md` com: para cada skill, scores em
  cada critério (1–5, calibrados pelos anchor examples do REQ-6), achados,
  sugestões de melhoria (merge candidates, splits, rewrites, deprecation
  candidates). **Constraint explícito no SKILL.md**: a skill começa com aviso
  "This is an observation report. Do not modify any skill file. All suggestions
  are non-prescriptive and require user approval before any action." e termina
  sem nunca chamar Edit/Write em `.claude/skills/`. O relatório **não cita
  corpo verbatim** de nenhuma skill (só scores + abstracts) — evita comitar
  conteúdo sanitizado-mas-sensível.

### Hooks (robustez)

- [ ] **REQ-Hook-1**: GIVEN qualquer hook desta spec (`stop-learn.sh`,
  `user-prompt-submit-recall.sh`, `reindex-learning.sh`), WHEN o script
  executa, THEN começa com `set -euo pipefail` ou `set -uo pipefail` com
  tratamento manual de erros (consistente com `stop-validate.sh`). Source de
  `learn-hook-helpers.sh` usa path absoluto via `"$CLAUDE_PROJECT_DIR"`. Falha
  do binário `learn` ou ausência dele → hook loga warn estruturado em
  `.claude/learning/learn.log` e sai 0 (nunca bloqueia o evento Claude
  upstream).

- [ ] **REQ-Hook-2**: GIVEN env propagation, WHEN hooks invocam `learn`, THEN
  passam explicitamente `--db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"`
  pra evitar dependência de auto-detection por cwd (hooks podem rodar em
  contextos onde cwd não é repo root).

### Docs e harness inventory

- [ ] **REQ-26**: GIVEN o harness inventory canônico em `docs/harness.md`, WHEN
  esta spec mergeia, THEN o inventory contém entradas para: skills
  `/learn-extract`, `/learn-refine`, `/learn-nudge`, `/learn-recall`,
  `/learn-audit-skills`; hooks `stop-learn.sh`, `user-prompt-submit-recall.sh`,
  `reindex-learning.sh`; binário `tools/learn` (nova seção `Learning loop
  tooling`); rules `.claude/rules/skill-quality.md`; guia
  `docs/guides/learning-loop.md`.

- [ ] **REQ-27**: GIVEN documentação operacional, WHEN um dev novo entra no
  projeto, THEN encontra `docs/guides/learning-loop.md` cobrindo as 7 seções:
  (1) visão dos 5 estágios, (2) schema do SQLite com diagrama,
  (3) fluxo de retrieval, (4) fluxo de nudge, (5) como invocar cada skill,
  (6) como debugar (`learn stats`, `learn reindex --verbose`),
  (7) como rollback (mover de `_deprecated/` de volta), com cabeçalhos `##`
  identificáveis por grep.

### CI / Lint coverage

- [ ] **REQ-CI-1**: GIVEN o tools/learn é módulo separado, WHEN `make lint` e
  `make ci-local` rodam, THEN executam **adicionalmente** `cd tools/learn &&
  golangci-lint run ./...` e `cd tools/learn && go test ./...`. Targets
  `make learn-lint` e `make learn-test` introduzidos pela TASK-32A. CI workflow
  (`.github/workflows/ci.yml`) recebe job ou step adicional cobrindo
  `tools/learn/`.

### Validação completa

- [ ] **REQ-28**: GIVEN `make learn-smoke`, WHEN executado, THEN roda end-to-end
  com fixtures sintéticas (não toca filesystem real do projeto):
  (a) `init` em diretório temporário; (b) ingere 3 specs fake; (c) gera
  candidates; (d) chama `recall` com prompt previsível; (e) verifica match
  esperado. Exit 0 ao final.

## Test Plan

Coverage rules aplicadas: cada REQ tem ≥1 TC, todas as fontes de erro têm TC,
branches conditional têm TC, deps externas (FS, SQLite, transcripts, git) têm
infra-failure TC, boundary TCs nos thresholds (`MIN_PATTERN_FREQ`,
`NUDGE_THRESHOLD`, `SIMILARITY_THRESHOLD`, `RECALL_MIN_SCORE`,
`MIN_PROMPT_LEN_FOR_RECALL`, `RECALL_TOP_K`, `RECALL_MAX_TOKENS`, `TTL_DAYS`).

### Domain Tests (entities, value objects, validation pura)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-D-01 | REQ-2 | boundary | n-gram extractor com freq=`MIN_PATTERN_FREQ`=3 (at threshold inclusive) | candidato emitido (>=) |
| TC-D-02 | REQ-2 | boundary | freq=2 (threshold-1, abaixo) | candidato NÃO emitido |
| TC-D-03 | REQ-2 | boundary | freq=4 (threshold+1, acima) | candidato emitido |
| TC-D-03a | REQ-2 | edge | input vazio | zero candidatos, exit 0 |
| TC-D-03b | REQ-2 | edge | extract executado 2× com mesma fonte | `candidates.jsonl` byte-idêntico nos 2 runs |
| TC-D-04 | REQ-4 | security | sanitização AWS key `AKIA0123456789012345` | `<REDACTED:aws_key>` |
| TC-D-05a | REQ-4 | security | OpenAI key `sk-...` | `<REDACTED:token>` |
| TC-D-05b | REQ-4 | security | OpenAI project key `sk-proj-...` | `<REDACTED:token>` |
| TC-D-05c | REQ-4 | security | Anthropic key `sk-ant-api03-...` | `<REDACTED:token>` |
| TC-D-05d | REQ-4 | security | GitHub PAT `ghp_<36 chars>` | `<REDACTED:token>` |
| TC-D-05e | REQ-4 | security | GitHub fine-grained `github_pat_...` | `<REDACTED:token>` |
| TC-D-05f | REQ-4 | security | Slack token `xoxb-...` | `<REDACTED:token>` |
| TC-D-05g | REQ-4 | security | SSH path `/Users/x/.ssh/id_rsa` | `<REDACTED:ssh_path>` |
| TC-D-06 | REQ-4 | security | linha de `.env` `API_KEY=value123` | linha sanitizada (`<REDACTED:env_value>`) |
| TC-D-07 | REQ-4 | edge | string `xoxb-not-real` em comentário (não é secret mas casa regex) | substituído (policy aceita falso positivo) |
| TC-D-07a | REQ-4 | security | sanitização executada **em memória antes** de qualquer write a disco | mock store recebe valor já sanitizado |
| TC-D-08a | REQ-7 | validation | frontmatter faltando `name` | erro com campo |
| TC-D-08b | REQ-7 | validation | frontmatter faltando `description` | erro com campo |
| TC-D-08c | REQ-7 | validation | frontmatter faltando `learning_provenance` | erro com campo |
| TC-D-08d | REQ-7 | validation | frontmatter faltando `created_at` | erro com campo |
| TC-D-08e | REQ-7 | validation | frontmatter faltando `last_reviewed_at` | erro com campo |
| TC-D-08f | REQ-7 | happy | frontmatter com todos 5 campos válidos | sem erro |
| TC-D-09 | REQ-8 | happy | Levenshtein "kitten" vs "sitting" | distance == 3 |
| TC-D-10 | REQ-24 | happy | parser de config.yml com defaults | struct populada conforme defaults |
| TC-D-11 | REQ-24 | happy | env `LEARN_NUDGE_THRESHOLD=3` sobrepõe YAML | NudgeThreshold == 3 |
| TC-D-11a | REQ-24 | happy | env `LEARN_TTL_DAYS=30` | TTLDays == 30 |
| TC-D-11b | REQ-24 | happy | env `LEARN_RECALL_MIN_SCORE=0.55` | RecallMinScore == 0.55 |
| TC-D-11c | REQ-24 | happy | env `LEARN_SECRET_PATTERNS` (YAML inline) | SecretPatterns sobrescreve default |
| TC-D-12 | REQ-24 | validation | YAML malformado | erro com line/column |
| TC-D-13 | REQ-24 | validation | `ttl_days = -1` | erro citando campo + range |
| TC-D-13a | REQ-24 | validation | env `LEARN_TTL_DAYS=abc` (não-int) | erro citando campo + valor inválido |
| TC-D-13b | REQ-24 | validation | `recall_min_score = 1.5` (fora de [0.0, 1.0]) | erro citando campo + range |
| TC-D-14 | REQ-2a | validation | `candidates.jsonl` linha contendo campo inesperado | unmarshal estrito falha |
| TC-D-15 | REQ-22 | validation | UsageError envelopado retornado pelo subcomando | dispatcher retorna exit 1 |
| TC-D-16 | REQ-22 | validation | RuntimeError envelopado retornado | dispatcher retorna exit 2 |
| TC-D-17 | REQ-22 | validation | erro plain (não envelopado) retornado | dispatcher retorna exit 2 (fallback) |

### Use Case Tests (subcomandos da CLI `learn`, com SQLite in-memory + FS fake)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-UC-01 | REQ-1, REQ-18 | happy | `learn init` em dir vazio | DB criado, schema completo, config.yml gerado, PRAGMA WAL + busy_timeout ativos |
| TC-UC-01a | REQ-18 | validation | após `init`, virtual tables `skill_fts`, `memory_fts`, `pattern_fts` existem | SELECT em `sqlite_master` retorna as 3 |
| TC-UC-02 | REQ-18 | edge | `learn init` quando DB já existe | exit 0, sem destruir DB (idempotente) |
| TC-UC-03 | REQ-1 | happy | `complete-task` insere evento | row em `events` com todos campos |
| TC-UC-04 | REQ-1, REQ-11 | happy | `complete-task` incrementa counter | `nudge_state.counter` += 1 |
| TC-UC-05 | REQ-11 | business | counter ≥ threshold após `complete-task` | stderr contém `LEARN_NUDGE_DUE=true` |
| TC-UC-06a | REQ-11 | boundary | counter=threshold-1 (=4) | NÃO emite sinal |
| TC-UC-06b | REQ-11 | boundary | counter=threshold (=5) | emite sinal |
| TC-UC-06c | REQ-11 | boundary | counter=threshold+1 (=6) | emite sinal |
| TC-UC-06d | REQ-11 | business | `nudge-tick --reset` zera counter | `nudge_state.counter == 0` |
| TC-UC-06e | REQ-11 | concurrency | 2 `complete-task` concorrentes (goroutines, mesma DB) | counter == 2 (sem lost update; WAL + atomic UPDATE) |
| TC-UC-07 | REQ-2 | happy | `extract` com 3 specs e padrão repetido 3× | candidato em jsonl, score normalizado |
| TC-UC-08 | REQ-2 | business | `extract --since=7d` | só specs dentro da janela |
| TC-UC-08a | REQ-2 | validation | `extract` schema do output (REQ-2a) | jsonl unmarshala em struct tipado |
| TC-UC-09 | REQ-3 | infra | transcript JSONL malformado | warn logado, pipeline continua |
| TC-UC-10 | REQ-3 | infra | transcript com permissão negada | warn logado, pula |
| TC-UC-11 | REQ-3 | infra | dir de transcripts inexistente | warn logado, pipeline continua sem fonte |
| TC-UC-11a | REQ-2 | infra | git log com diff binário (não-utf8) | pula commit, log warn, continua |
| TC-UC-12 | REQ-19 | happy | `reindex` full com 2 skills + 1 memory | 2 rows em `skill_index`, 1 em `memory_index` |
| TC-UC-12a | REQ-19 | edge | `reindex` 2× sem mudanças (idempotência) | row counts idênticos, sem duplicação |
| TC-UC-13 | REQ-19 | business | `reindex` após delete de skill | row órfã removida |
| TC-UC-14 | REQ-19 | edge | `reindex --path <single-file>` | só atualiza essa entrada |
| TC-UC-15 | REQ-19 | edge | full `reindex` com arquivo em `_deprecated/` (sem prior entry) | NÃO indexado como nova entry |
| TC-UC-15a | REQ-19 | edge | `reindex` preserva entries com path em `_deprecated/` previamente indexadas em path normal (caso de move pós-index) | row preservada (não deletada como órfã) |
| TC-UC-15b | REQ-19a | infra | memory MD com `[[deprecated-skill-name]]` resolvendo a `_deprecated/` | warn estruturado emitido |
| TC-UC-16 | REQ-8 | happy | `similar --skill <path>` com 3 skills no índice | top-K ordenado por (-score, path), determinístico |
| TC-UC-16a | REQ-8 | edge | `similar` re-rodado, mesma DB state, mesma query | resultados byte-idênticos (mesma ordem, mesmos scores) |
| TC-UC-17 | REQ-8 | edge | `similar` sem outras skills no índice | lista vazia, exit 0 |
| TC-UC-18a | REQ-8 | boundary | threshold=0.6: score=0.59 | NÃO retornado |
| TC-UC-18b | REQ-8 | boundary | threshold=0.6: score=0.60 | retornado (inclusivo) |
| TC-UC-18c | REQ-8 | boundary | threshold=0.6: score=0.61 | retornado |
| TC-UC-19 | REQ-14 | happy | `recall --prompt "..."` com matches acima do score | JSON com top-K |
| TC-UC-19a | REQ-14a | validation | output formato system-reminder match template REQ-14a | string match |
| TC-UC-20 | REQ-14 | edge | prompt curto (< MIN_PROMPT_LEN) | exit 0, stdout vazio |
| TC-UC-20a | REQ-14 | boundary | prompt com exatamente MIN_PROMPT_LEN chars | retrieval ATIVA (inclusivo) |
| TC-UC-21a | REQ-15 | boundary | score=0.39 (< default 0.4) | filtrado |
| TC-UC-21b | REQ-15 | boundary | score=0.40 (== default) | retornado (inclusivo) |
| TC-UC-21c | REQ-15 | boundary | score=0.41 | retornado |
| TC-UC-22 | REQ-14 | edge | `--max-tokens=500` com 10 matches longos | mantém top-N completos ≤ budget; nunca trunca mid-JSON; items beyond drop |
| TC-UC-22a | REQ-14 | boundary | `--top-k=1` com 5 matches | 1 result retornado |
| TC-UC-22b | REQ-14 | edge | `--top-k=5` com 2 matches | 2 results, sem erro |
| TC-UC-22c | REQ-14 | validation | `--top-k=0` | exit 1, validation error |
| TC-UC-22d | REQ-14 | validation | `--max-tokens=0` | exit 1, validation error |
| TC-UC-23 | REQ-16 | happy | `recall --kind=memory --since=7d --max=10` | filtros aplicados |
| TC-UC-23a | REQ-16 | validation | `--kind=invalid` | exit 1, stderr lista valores válidos |
| TC-UC-23b | REQ-16 | validation | `--since=not-a-duration` | exit 1, stderr explica formato |
| TC-UC-23c | REQ-16 | validation | `--max=-1` | exit 1 |
| TC-UC-24 | REQ-17 | happy | `track-use --paths .claude/skills/a/SKILL.md,memory/b.md` | `skill_usage.usage_count`+=1 pra A; `memory_usage`+=1 pra B; `last_used_at` atualizado |
| TC-UC-25 | REQ-17 | infra | `track-use` em path inexistente no índice | exit 0 silencioso (best-effort), warn logado |
| TC-UC-26 | REQ-12 | happy | `nudge-tick` (sem --reset) retorna candidatos por TTL | output JSON com items expirados |
| TC-UC-27a | REQ-12 | boundary | created há 2×TTL-1 dias, last_used há TTL+1 | NÃO candidato (created não atingiu) |
| TC-UC-27b | REQ-12 | boundary | created há exatamente 2×TTL_DAYS, last_used há exatamente TTL_DAYS | candidato (inclusivo, ≤) |
| TC-UC-27c | REQ-12 | boundary | created há 2×TTL+1, last_used há TTL-1 | NÃO candidato (last_used recente) |
| TC-UC-28 | REQ-23 | happy | `stats` em DB populado retorna JSON com schema documentado | parse + assert chaves esperadas |
| TC-UC-29 | REQ-23 | edge | `stats` em DB vazio | JSON com zeros |
| TC-UC-30 | REQ-22 | validation | subcomando inexistente | exit 1, stderr help |
| TC-UC-30a | REQ-22 | happy | `learn --help` (root) | exit 0, help impresso |
| TC-UC-30b | REQ-22 | happy | `learn recall --help` | exit 0 |
| TC-UC-31 | REQ-21 | infra | DB corrompido (truncado) | exit 2, structured error |
| TC-UC-31a | REQ-22 | infra | DB abre, mas write falha (permissão revogada mid-run) | exit 2, structured error |
| TC-UC-32 | REQ-18 | infra | dir `.claude/learning/` sem permissão de escrita | exit 2, erro claro |
| TC-UC-33 | REQ-21 | validation | `go build ./...` do root NÃO inclui `modernc.org/sqlite` no build graph | `go list -deps ./... \| grep modernc` retorna vazio |
| TC-UC-34 | REQ-21 | validation | `cd tools/learn && go build ./...` standalone | exit 0 |
| TC-UC-35 | REQ-5 | happy | `extract` lê candidates.jsonl, `record-decision --action=new-skill --target .claude/skills/X/SKILL.md` | file criado com frontmatter; row em `decisions` |
| TC-UC-35a | REQ-5 | happy | `record-decision --action=new-memory --target memory/y.md` | file criado, MEMORY.md atualizado, row em `decisions` |
| TC-UC-35b | REQ-5 | business | `record-decision --action=discard --rationale "vague"` | decisions.action=discard, decisions.rationale gravado, sem file change |
| TC-UC-35c | REQ-5 | infra | `extract` com candidates.jsonl ausente | exit 0, zero decisions, warn logado |
| TC-UC-36 | REQ-9 | happy | `record-decision --action=pending-approval --target A.md --rationale-diff ...` | row inserida; nenhum file ainda movido |
| TC-UC-36a | REQ-9 | happy | `apply-decision <id>` após approval | move pra `_deprecated/`, row update `applied` |
| TC-UC-36b | REQ-9 | edge | `apply-decision --dry-run` | mostra diff, não move file |
| TC-UC-37 | REQ-25 | business | `learn audit-skills-prep --skills-dir .claude/skills/` (helper agnóstico ao LLM) | lê 14 skills, retorna struct com path+frontmatter+body excerpt; NÃO modifica nenhum file |

### Shell hook tests (testes bash dos hooks `.claude/hooks/`)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-SH-01 | REQ-1 | happy | `stop-learn.sh` com mudança em `.specs/<x>.md` status `DONE` | chama `learn complete-task`, sai 0 |
| TC-SH-02 | REQ-1 | edge | `stop-learn.sh` sem mudança em `.specs/**` | no-op, sai 0 |
| TC-SH-03 | REQ-Hook-1 | edge | `stop-learn.sh` quando `learn` não está instalado | warn em `learn.log`, sai 0 |
| TC-SH-03a | REQ-Hook-1 | infra | `stop-learn.sh` quando `learn complete-task` exit 2 (crash runtime) | hook sai 0, error logado |
| TC-SH-03b | REQ-Hook-1 | infra | `stop-learn.sh` quando processo `learn` é morto (SIGKILL) | hook sai 0, error logado, próximo call recupera (insert idempotente) |
| TC-SH-04 | REQ-13 | happy | hook detecta `LEARN_NUDGE_DUE=true` | stderr contém `Learning nudge due — run /learn-nudge when ready (<N> specs since last nudge)` literal, com N=counter; NÃO invoca skill |
| TC-SH-05 | REQ-14, REQ-14a | happy | `user-prompt-submit-recall.sh` com prompt longo + matches | injeta system-reminder matching template REQ-14a |
| TC-SH-05a | REQ-17 | happy | mesmo hook após injetar, chama `learn track-use --paths` com os paths injetados | `skill_usage.usage_count` += 1 |
| TC-SH-06 | REQ-14 | boundary | prompt com `MIN_PROMPT_LEN - 1` chars | no-op |
| TC-SH-06a | REQ-14 | boundary | prompt com exatamente `MIN_PROMPT_LEN` chars | retrieval roda |
| TC-SH-07 | REQ-15 | edge | sem matches acima do score | no-op, stdout vazio |
| TC-SH-07a | REQ-14 | infra | `learn recall` excede timeout (mock que dorme 5s; timeout config=2s) | hook sai 0, no-op silencioso |
| TC-SH-07b | REQ-17 | edge | hook injeta matches, mas `learn track-use` falha (mock retorna exit 1) | retrieval mantido, warn em learn.log, hook sai 0 |
| TC-SH-08 | REQ-20 | happy | `reindex-learning.sh` em edit de `.claude/skills/foo/SKILL.md` | chama `learn reindex --path` |
| TC-SH-09 | REQ-20 | edge | edit de arquivo não-relacionado (ex: `cmd/api/main.go`) | no-op |
| TC-SH-10 | REQ-20 | infra | `learn reindex` falha (DB lock) | hook loga warn, sai 0 (não bloqueia o save) |
| TC-SH-11 | REQ-Hook-1 | validation | cada hook script começa com `set -euo pipefail` ou `set -uo pipefail` | grep no fonte |
| TC-SH-12 | REQ-Hook-2 | validation | cada hook chama `learn` com `--db-path "$CLAUDE_PROJECT_DIR/.claude/learning/db.sqlite"` | grep no fonte |

### E2E Tests (fixture-driven roundtrip do loop)

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-E2E-01 | REQ-1, REQ-2, REQ-18 | happy | fixtures determinísticas: 3 specs DONE + 2 transcripts + git fake → `init` + `complete-task`×3 + `extract` | events=3, `candidates.jsonl` contém signature exata `"Read→Edit→Bash"` com freq=3 |
| TC-E2E-02 | REQ-19, REQ-20 | business | criar skill MD; `reindex --path`; `recall` por keyword; editar skill MD com nova keyword; `reindex --path` novamente; `recall` com nova keyword | match retornado pra nova keyword (FTS5 triggers sincronizados) |
| TC-E2E-03 | REQ-11, REQ-13 | business | 5× `complete-task`, último imprime `LEARN_NUDGE_DUE=true` | sinal emitido no 5º, NÃO no 4º |
| TC-E2E-04 | REQ-10 | validation | simular merge: skill A movida; verificar arquivo em `_deprecated/<name>-<YYYYMMDDTHHMMSSZ>.md`, header literal `> Deprecated YYYY-MM-DD by /learn-refine: merged into <target>` na primeira linha, original removido | tudo conforme |
| TC-E2E-05 | REQ-12 | business | DB com skill created há 200d (TTL=90; 2*TTL=180), last_used 100d (> TTL); `nudge-tick` | retorna como candidato |
| TC-E2E-06 | REQ-28 | happy | `make learn-smoke` | exit 0, log final `SMOKE OK` |
| TC-E2E-07 | REQ-4 | security | fixture spec com AWS key fake no execution log, `extract` rodado | `candidates.jsonl` contém `<REDACTED:aws_key>` no campo afetado, nunca o valor original |
| TC-E2E-08 | REQ-23 | happy | `learn stats` após sequência completa | JSON com counts esperados |
| TC-E2E-09 | REQ-26 | validation | `grep -E "(learn-extract\|learn-refine\|learn-nudge\|learn-recall\|learn-audit-skills)" docs/harness.md \| wc -l` ≥ 5 | conforme |
| TC-E2E-09a | REQ-26 | validation | `grep -E "(stop-learn\|user-prompt-submit-recall\|reindex-learning)" docs/harness.md` retorna 3 | conforme |
| TC-E2E-10 | REQ-27 | validation | `docs/guides/learning-loop.md` contém 7 cabeçalhos `##` correspondendo às 7 seções de REQ-27 | grep `^## ` retorna entradas |
| TC-E2E-11 | REQ-6 | validation | `learn-extract/SKILL.md` contém citação literal de bloco da rubrica `.claude/rules/skill-quality.md` | grep retorna match |
| TC-E2E-11a | REQ-6 | validation | `learn-refine/SKILL.md` idem | idem |
| TC-E2E-11b | REQ-6 | validation | `learn-audit-skills/SKILL.md` idem | idem |
| TC-E2E-12 | REQ-25 | happy | `/learn-audit-skills` invocada em fixtures (2 skills mock) | report criado em `.specs/reports/skill-audit-<date>.md`, contém scores 1–5 pra 4 critérios pra cada skill |
| TC-E2E-12a | REQ-25 | business | mtime de todas `.claude/skills/**/SKILL.md` inalterado após audit | non-destructive verificado |
| TC-E2E-12b | REQ-25 | validation | report não contém corpo verbatim de skills (heurística: nenhuma linha do report match exatamente uma linha de algum SKILL.md) | conforme |
| TC-E2E-13 | REQ-CI-1 | validation | `make learn-lint` exit 0 em estado limpo; introduzir issue de lint em `tools/learn/internal/cmd/init.go`, rodar de novo | exit não-zero |

### Smoke Tests (k6) — N/A

Este sensor é local (não toca runtime HTTP/gRPC). Smoke do loop completo é
`TC-E2E-06` via `make learn-smoke`. Justificativa: k6 testa runtime deployado;
learning loop é pre-runtime tooling. Smoke de skills agentic não é
auto-testável (depende de Claude no loop) — coberto via manual walkthrough no
`docs/guides/learning-loop.md`.

### Rigor check

REQs (incluindo subcláusulas `-a`, `-b`, Hook-1, Hook-2, CI-1): 36. TCs totais:
33 (D) + 60 (UC) + 17 (SH) + 17 (E2E) = **127**. TCs happy: ~30. TCs
erro/edge/security/boundary/business/infra/concurrency/validation: ~97 (mais
de 3× happy).

Verificação por REQ:

- REQ-1: TC-UC-01, 03, 04; TC-SH-01, 02, 03; TC-E2E-01 ✓
- REQ-2: TC-D-01, 02, 03, 03a, 03b; TC-UC-07, 08, 08a, 11a; TC-E2E-01 ✓
- REQ-2a: TC-D-14; TC-UC-08a ✓
- REQ-3: TC-UC-09, 10, 11 ✓
- REQ-4: TC-D-04, 05a–g, 06, 07, 07a; TC-E2E-07 ✓
- REQ-5: TC-UC-35, 35a, 35b, 35c ✓
- REQ-6: TC-E2E-11, 11a, 11b ✓
- REQ-7: TC-D-08a–f ✓
- REQ-8: TC-D-09; TC-UC-16, 16a, 17, 18a, 18b, 18c ✓
- REQ-9: TC-UC-36, 36a, 36b ✓
- REQ-10: TC-E2E-04 ✓
- REQ-11: TC-UC-04, 05, 06a–e ✓
- REQ-12: TC-UC-26, 27a, 27b, 27c; TC-E2E-05 ✓
- REQ-13: TC-SH-04 ✓
- REQ-14: TC-UC-19, 20, 20a, 22, 22a–d; TC-SH-05, 06, 06a; TC-SH-07a ✓
- REQ-14a: TC-UC-19a; TC-SH-05 ✓
- REQ-15: TC-UC-21a, 21b, 21c; TC-SH-07 ✓
- REQ-16: TC-UC-23, 23a, 23b, 23c ✓
- REQ-17: TC-UC-24, 25; TC-SH-05a, 07b ✓
- REQ-18: TC-D-10; TC-UC-01, 01a, 02, 31, 32 ✓
- REQ-19: TC-UC-12, 12a, 13, 14, 15, 15a; TC-E2E-02 ✓
- REQ-19a: TC-UC-15b ✓
- REQ-20: TC-SH-08, 09, 10; TC-E2E-02 ✓
- REQ-21: TC-UC-31, 33, 34 ✓
- REQ-22: TC-D-15, 16, 17; TC-UC-30, 30a, 30b, 31a ✓
- REQ-23: TC-UC-28, 29; TC-E2E-08 ✓
- REQ-24: TC-D-10, 11, 11a–c, 12, 13, 13a, 13b ✓
- REQ-25: TC-UC-37; TC-E2E-12, 12a, 12b ✓
- REQ-26: TC-E2E-09, 09a ✓
- REQ-27: TC-E2E-10 ✓
- REQ-28: TC-E2E-06 ✓
- REQ-Hook-1: TC-SH-03, 03a, 03b, 11 ✓
- REQ-Hook-2: TC-SH-12 ✓
- REQ-CI-1: TC-E2E-13 ✓

Todos thresholds (`MIN_PATTERN_FREQ`, `NUDGE_THRESHOLD`, `SIMILARITY_THRESHOLD`,
`RECALL_MIN_SCORE`, `TTL_DAYS`, `MIN_PROMPT_LEN_FOR_RECALL`, `RECALL_TOP_K`,
`RECALL_MAX_TOKENS`) têm boundary TC com valor abaixo/no/acima. Toda dep externa
(FS, SQLite, transcripts, git, hook execution) tem infra-failure TC.
Sanitização tem 8 TCs (7 classes + 1 falso positivo + 1 in-memory order).
Closure (REQ-17 tracking explícito) coberta. Concorrência do counter coberta
(TC-UC-06e). Determinismo coberto (TC-D-03b, TC-UC-16a, TC-UC-12a).

## Design

### Architecture Decisions

- **Binário Go puro, go.mod separado**. `tools/learn/` é módulo independente
  (`tools/learn/go.mod`). Usa `modernc.org/sqlite` (puro Go, sem cgo).
  Trade-off explícito documentado em `docs/guides/learning-loop.md`.

- **SQLite + FTS5, sempre WAL + busy_timeout**. Conexão configurada com
  `PRAGMA journal_mode=WAL` + `PRAGMA busy_timeout=5000` na abertura
  (`store.Open`). Counter usa `UPDATE nudge_state SET counter = counter + 1`
  (atômico — SQLite serializa SQL statements via WAL writer lock). Sem
  read-modify-write em código Go.

- **Erros tipados pra exit codes**. Package `tools/learn/internal/learnerr/`
  define `*UsageError` (exit 1) e `*RuntimeError` (exit 2). Root Cobra
  `RunE` faz `errors.As` pra classificar; fallback é exit 2. Subcomandos
  envelopam internos via `fmt.Errorf("...: %w", err)` preservando o tipo
  via `errors.Is/As`.

- **Heurística + LLM, não LLM-puro**. Etapas 2 e parte de 4 (similarity scoring)
  determinísticas — testáveis via go test, custo zero. Etapas 3, 4 (decisão de
  merge), 5 LLM-driven via skills agentic. Skill calls helper deterministico
  pra effect (e.g., `learn record-decision`, `learn apply-decision`) — LLM
  decide, binário grava.

- **Closure via UserPromptSubmit hook**. Hook chama `learn recall` em até 2s;
  injeta system-reminder enxuto (paths + summaries). Após injeção, chama
  `learn track-use --paths <list>` (não passive heuristic). Tracking baseado
  em "apresentado ao Claude", suficiente pra TTL.

- **Anti-deleção. Tudo move pra `_deprecated/`**. Skills/memory descartadas pelo
  refinement ou nudge vão pra `_deprecated/<name>-<YYYYMMDDTHHMMSSZ>.md` com
  header explicativo. Reindex ignora `_deprecated/` na população de novas
  entradas, mas preserva entradas existentes que apontavam pra path normal
  agora deprecated.

- **JSON merge via jq pra settings.json**. `.claude/settings.json` é JSON.
  TASK-31 (MERGE-SETTINGS) usa `jq` (já permitido no allow-list) pra fazer
  structural merge — nunca text concat. Cada fragment é JSON puro válido
  (objeto com chave + valor a injetar). Merge task: lê settings.json, parsa,
  aplica cada fragment via `jq --argjson`, valida com `jq empty`, escreve.
  Tests do merge: TC-E2E não direto, mas validação pós-merge via
  `jq empty .claude/settings.json` em TASK-31 + na CI step.

- **Rubrica de skill quality como arquivo dedicado**. Em
  `.claude/rules/skill-quality.md`, com anchor examples 1–5 por critério.
  Skills agentic citam **literalmente** (TC-E2E-11) — não basta referenciar.

- **Sanitização agressiva, em memória, antes de write**. Falso positivo
  aceito; falso negativo de classes conhecidas (AWS, OpenAI, Anthropic, GitHub,
  Slack, SSH, .env) **não**. Padrões em `config.yml` editáveis. Aplicada na
  borda dos parsers (transcript, git diff, spec body) — output já sanitizado
  para o resto do pipeline.

- **DB local, skills/memory versionados**. `.claude/learning/db.sqlite`
  gitignored. Skills, memory, fragments (`.specs/wiring/`), rules
  (`.claude/rules/skill-quality.md`), relatório de auditoria
  (`.specs/reports/skill-audit-*.md`) — versionados.

- **`_template/SKILL.md`** em `.claude/skills/_template/SKILL.md`. Frontmatter
  obrigatório + seções esperadas.

- **Hooks falham silenciosamente, com `set -euo pipefail` + timeout**. Log em
  `.claude/learning/learn.log` (gitignored). `learn recall` no UserPromptSubmit
  é wrapped em `timeout 2`. ENV: `--db-path` passado explicitamente.

- **`bin/learn` (root) gitignored**. Diff vs `gopherplate` (instalado via `go
  install` em `$GOBIN`): tooling de harness fica local-only ao repo, não
  compete com binários globais do dev. Doc explica como adicionar ao PATH.

### Files to Create

**Tooling Go:**

- `tools/learn/go.mod` — módulo separado
- `tools/learn/cmd/learn/main.go` — entrypoint Cobra
- `tools/learn/internal/store/schema.sql` — embed
- `tools/learn/internal/store/store.go` — abertura DB com PRAGMA WAL + busy_timeout
- `tools/learn/internal/store/queries.go` — queries tipadas
- `tools/learn/internal/learnerr/learnerr.go` — UsageError, RuntimeError
- `tools/learn/internal/config/config.go` — YAML + env loader + validation
- `tools/learn/internal/ingest/spec/parser.go`
- `tools/learn/internal/ingest/git/log.go`
- `tools/learn/internal/ingest/transcript/parser.go`
- `tools/learn/internal/ingest/memory/parser.go`
- `tools/learn/internal/sanitize/sanitize.go` — regex redaction
- `tools/learn/internal/pattern/ngram.go`
- `tools/learn/internal/pattern/score.go`
- `tools/learn/internal/pattern/schema.go` — tipo Go pra `candidates.jsonl`
- `tools/learn/internal/similar/levenshtein.go`
- `tools/learn/internal/similar/bm25.go`
- `tools/learn/internal/recall/recall.go`
- `tools/learn/internal/cmd/*.go` — um arquivo por subcomando Cobra
- `tools/learn/internal/cmd/*_test.go` — testes UC
- `tools/learn/testdata/` — fixtures
- `tools/learn/Makefile` — `build`, `test`, `lint`, `install`
- `tools/learn/.golangci.yml` — opcional; ou compartilhar root via cd

**Hooks:**

- `.claude/hooks/stop-learn.sh`
- `.claude/hooks/user-prompt-submit-recall.sh`
- `.claude/hooks/reindex-learning.sh`
- `.claude/hooks/learn-hook-helpers.sh` — funções (binary lookup com asdf shim, log structured, db path resolver)

**Skills (agentic):**

- `.claude/skills/learn-extract/SKILL.md`
- `.claude/skills/learn-refine/SKILL.md`
- `.claude/skills/learn-nudge/SKILL.md`
- `.claude/skills/learn-recall/SKILL.md`
- `.claude/skills/learn-audit-skills/SKILL.md`
- `.claude/skills/_template/SKILL.md`

**Rules:**

- `.claude/rules/skill-quality.md`

**Docs:**

- `docs/guides/learning-loop.md`

**Reports:**

- `.specs/reports/.gitkeep`

**Wiring fragments:**

- `.specs/wiring/learning-loop-harness/task-20.settings-json.fragment.md` (JSON)
- `.specs/wiring/learning-loop-harness/task-21.settings-json.fragment.md` (JSON)
- `.specs/wiring/learning-loop-harness/task-22.settings-json.fragment.md` (JSON)
- `.specs/wiring/learning-loop-harness/task-1.makefile.fragment.md`
- `.specs/wiring/learning-loop-harness/task-32a.makefile.fragment.md`
- `.specs/wiring/learning-loop-harness/task-33a.docs-harness.fragment.md`
- `.specs/wiring/learning-loop-harness/task-34a.claude-md.fragment.md`

### Files to Modify

- `.gitignore` — adicionar `.claude/learning/db.sqlite`, `.claude/learning/learn.log`
- `.claude/settings.json` — via TASK-31 com `jq` (3 fragments)
- `Makefile` — via TASK-32 (2 fragments)
- `docs/harness.md` — via TASK-33
- `CLAUDE.md` — via TASK-34
- `.github/workflows/ci.yml` — adicionar step ou job pra `cd tools/learn && go test && golangci-lint run`
  (TASK-CI-1)
- `.claude/hooks/stop-validate.sh` — **NÃO MODIFICAR**. `stop-learn.sh` é hook Stop
  paralelo, registrado adicionalmente em settings.json.

### Dependencies

- `modernc.org/sqlite` v1.x — SQLite puro Go
- `github.com/spf13/cobra` v1.x
- `gopkg.in/yaml.v3`
- (dev) `github.com/stretchr/testify`

### Registered anchors (esta spec)

| Target | Anchor | Insert position | Merge tool |
|---|---|---|---|
| `.claude/settings.json` | `hooks.Stop` | append to `hooks.Stop[0].hooks` | `jq` |
| `.claude/settings.json` | `hooks.UserPromptSubmit` | create array if absent, append | `jq` |
| `.claude/settings.json` | `hooks.PostToolUse[Edit\|Write]` | append to existing | `jq` |
| `Makefile` | `learn targets` | new section `## Learning loop`, before `help:` | text patch |
| `docs/harness.md` | `Skills` | append rows, alfabético | text patch |
| `docs/harness.md` | `Hooks` | append rows | text patch |
| `docs/harness.md` | `Learning loop tooling` | new section após `gopherplate CLI` | text patch |
| `CLAUDE.md` | `Skills table` | append rows | text patch |
| `CLAUDE.md` | `Hooks bullets` | append bullets | text patch |

## Tasks

> Convenção: tasks com `tests:` seguem TDD (RED→GREEN→REFACTOR). Tasks de
> integração (`.gitignore`, scripts shell, merges) são verificadas via E2E ou
> inspeção, não TDD direto.

### Foundation

- [x] **TASK-1**: Bootstrap `tools/learn/` (go.mod separado + skeleton Cobra +
  `make learn-build` fragment).
  - files: `tools/learn/go.mod`, `tools/learn/go.sum`,
    `tools/learn/cmd/learn/main.go`, `tools/learn/Makefile`,
    `.specs/wiring/learning-loop-harness/task-1.makefile.fragment.md`
  - tests: TC-UC-30, TC-UC-30a, TC-UC-34

- [x] **TASK-2**: Config loader + schema + validation.
  - files: `tools/learn/internal/config/config.go`,
    `tools/learn/internal/config/config_test.go`,
    `tools/learn/internal/config/defaults.go`
  - depends: TASK-1
  - tests: TC-D-10, TC-D-11, TC-D-11a, TC-D-11b, TC-D-11c, TC-D-12, TC-D-13,
    TC-D-13a, TC-D-13b

- [x] **TASK-3**: learnerr package (UsageError, RuntimeError) + dispatcher
  classifier in root cmd.
  - files: `tools/learn/internal/learnerr/learnerr.go`,
    `tools/learn/internal/learnerr/learnerr_test.go`,
    `tools/learn/cmd/learn/main.go` (atualiza dispatcher)
  - depends: TASK-1
  - tests: TC-D-15, TC-D-16, TC-D-17

- [x] **TASK-4**: Store layer — schema SQL + abertura DB com PRAGMA WAL +
  busy_timeout + migrations + queries tipadas + FTS5 triggers.
  - files: `tools/learn/internal/store/schema.sql`,
    `tools/learn/internal/store/store.go`,
    `tools/learn/internal/store/store_test.go`,
    `tools/learn/internal/store/queries.go`
  - depends: TASK-1, TASK-3
  - tests: TC-UC-01a, TC-UC-31, parte de TC-UC-01

- [x] **TASK-5**: Subcomando `init`.
  - files: `tools/learn/internal/cmd/init.go`,
    `tools/learn/internal/cmd/init_test.go`
  - depends: TASK-2, TASK-4
  - tests: TC-UC-01, TC-UC-02, TC-UC-32

### Domain entities + utilitários

- [x] **TASK-6**: Sanitização.
  - files: `tools/learn/internal/sanitize/sanitize.go`,
    `tools/learn/internal/sanitize/sanitize_test.go`,
    `tools/learn/internal/sanitize/patterns.go`
  - depends: TASK-1
  - tests: TC-D-04, TC-D-05a, TC-D-05b, TC-D-05c, TC-D-05d, TC-D-05e, TC-D-05f,
    TC-D-05g, TC-D-06, TC-D-07, TC-D-07a

- [x] **TASK-7**: N-gram extractor + scoring + schema.
  - files: `tools/learn/internal/pattern/ngram.go`,
    `tools/learn/internal/pattern/ngram_test.go`,
    `tools/learn/internal/pattern/score.go`,
    `tools/learn/internal/pattern/schema.go`,
    `tools/learn/internal/pattern/schema_test.go`
  - depends: TASK-1
  - tests: TC-D-01, TC-D-02, TC-D-03, TC-D-03a, TC-D-03b, TC-D-14

- [x] **TASK-8**: Edit distance + BM25 wrapper.
  - files: `tools/learn/internal/similar/levenshtein.go`,
    `tools/learn/internal/similar/levenshtein_test.go`,
    `tools/learn/internal/similar/bm25.go`,
    `tools/learn/internal/similar/bm25_test.go`
  - depends: TASK-4
  - tests: TC-D-09

### Ingest sources

- [x] **TASK-9**: Parser de specs.
  - files: `tools/learn/internal/ingest/spec/parser.go`,
    `tools/learn/internal/ingest/spec/parser_test.go`,
    `tools/learn/testdata/specs/`
  - depends: TASK-6
  - tests: parte de TC-UC-07, TC-UC-08

- [x] **TASK-10**: Parser de git log + diffs.
  - files: `tools/learn/internal/ingest/git/log.go`,
    `tools/learn/internal/ingest/git/log_test.go`
  - depends: TASK-6
  - tests: TC-UC-11a

- [x] **TASK-11**: Parser de transcripts JSONL (com sanitização aplicada).
  - files: `tools/learn/internal/ingest/transcript/parser.go`,
    `tools/learn/internal/ingest/transcript/parser_test.go`,
    `tools/learn/testdata/transcripts/`
  - depends: TASK-6
  - tests: TC-UC-09, TC-UC-10, TC-UC-11

- [x] **TASK-12**: Parser de memory MD.
  - files: `tools/learn/internal/ingest/memory/parser.go`,
    `tools/learn/internal/ingest/memory/parser_test.go`
  - depends: TASK-6
  - tests: parte de TC-UC-12

### CLI commands base

- [x] **TASK-13**: `complete-task` (etapa 1 + counter).
  - files: `tools/learn/internal/cmd/complete_task.go`,
    `tools/learn/internal/cmd/complete_task_test.go`
  - depends: TASK-5
  - tests: TC-UC-03, TC-UC-04, TC-UC-05, TC-UC-06a, TC-UC-06b, TC-UC-06c,
    TC-UC-06e

- [x] **TASK-14**: `extract` (etapa 2).
  - files: `tools/learn/internal/cmd/extract.go`,
    `tools/learn/internal/cmd/extract_test.go`
  - depends: TASK-7, TASK-9, TASK-10, TASK-11, TASK-12
  - tests: TC-UC-07, TC-UC-08, TC-UC-08a, TC-UC-35c

- [x] **TASK-15**: `reindex` (incremental + full, `_deprecated/` policy +
  broken-link warn).
  - files: `tools/learn/internal/cmd/reindex.go`,
    `tools/learn/internal/cmd/reindex_test.go`
  - depends: TASK-4, TASK-12
  - tests: TC-UC-12, TC-UC-12a, TC-UC-13, TC-UC-14, TC-UC-15, TC-UC-15a,
    TC-UC-15b

- [x] **TASK-16**: `similar` (etapa 4 helper).
  - files: `tools/learn/internal/cmd/similar.go`,
    `tools/learn/internal/cmd/similar_test.go`
  - depends: TASK-8, TASK-15
  - tests: TC-UC-16, TC-UC-16a, TC-UC-17, TC-UC-18a, TC-UC-18b, TC-UC-18c

- [x] **TASK-17**: `recall` (closure retrieval) + `track-use`.
  - files: `tools/learn/internal/cmd/recall.go`,
    `tools/learn/internal/cmd/recall_test.go`,
    `tools/learn/internal/cmd/track_use.go`,
    `tools/learn/internal/cmd/track_use_test.go`,
    `tools/learn/internal/recall/recall.go`,
    `tools/learn/internal/recall/recall_test.go`
  - depends: TASK-8, TASK-15
  - tests: TC-UC-19, TC-UC-19a, TC-UC-20, TC-UC-20a, TC-UC-21a, TC-UC-21b,
    TC-UC-21c, TC-UC-22, TC-UC-22a, TC-UC-22b, TC-UC-22c, TC-UC-22d, TC-UC-23,
    TC-UC-23a, TC-UC-23b, TC-UC-23c, TC-UC-24, TC-UC-25

- [x] **TASK-18**: `nudge-tick` + reset.
  - files: `tools/learn/internal/cmd/nudge_tick.go`,
    `tools/learn/internal/cmd/nudge_tick_test.go`
  - depends: TASK-13, TASK-15
  - tests: TC-UC-06d, TC-UC-26, TC-UC-27a, TC-UC-27b, TC-UC-27c

- [x] **TASK-19**: `record-decision`, `apply-decision`, `validate-skill`,
  `audit-skills-prep`.
  - files: `tools/learn/internal/cmd/record_decision.go`,
    `tools/learn/internal/cmd/record_decision_test.go`,
    `tools/learn/internal/cmd/apply_decision.go`,
    `tools/learn/internal/cmd/apply_decision_test.go`,
    `tools/learn/internal/cmd/validate_skill.go`,
    `tools/learn/internal/cmd/validate_skill_test.go`,
    `tools/learn/internal/cmd/audit_skills_prep.go`,
    `tools/learn/internal/cmd/audit_skills_prep_test.go`
  - depends: TASK-15
  - tests: TC-D-08a–f, TC-UC-35, TC-UC-35a, TC-UC-35b, TC-UC-36, TC-UC-36a,
    TC-UC-36b, TC-UC-37

- [x] **TASK-20**: `stats` + logging package + `--help` em todos os subcomandos.
  - files: `tools/learn/internal/cmd/stats.go`,
    `tools/learn/internal/cmd/stats_test.go`,
    `tools/learn/internal/logging/logging.go`
  - depends: TASK-4
  - tests: TC-UC-28, TC-UC-29, TC-UC-30b, TC-UC-31a

### Hooks

- [x] **TASK-21**: Hook helper script (binary lookup + structured logging +
  db-path resolver).
  - files: `.claude/hooks/learn-hook-helpers.sh`

- [x] **TASK-22**: `stop-learn.sh` + fragment JSON pra settings.
  - files: `.claude/hooks/stop-learn.sh`,
    `.specs/wiring/learning-loop-harness/task-22.settings-json.fragment.md`
  - depends: TASK-13, TASK-21
  - tests: TC-SH-01, TC-SH-02, TC-SH-03, TC-SH-03a, TC-SH-03b, TC-SH-04,
    TC-SH-11, TC-SH-12

- [x] **TASK-23**: `user-prompt-submit-recall.sh` (com timeout 2s + track-use)
  + fragment.
  - files: `.claude/hooks/user-prompt-submit-recall.sh`,
    `.specs/wiring/learning-loop-harness/task-23.settings-json.fragment.md`
  - depends: TASK-17, TASK-21
  - tests: TC-SH-05, TC-SH-05a, TC-SH-06, TC-SH-06a, TC-SH-07, TC-SH-07a,
    TC-SH-07b, TC-SH-11, TC-SH-12

- [x] **TASK-24**: `reindex-learning.sh` + fragment.
  - files: `.claude/hooks/reindex-learning.sh`,
    `.specs/wiring/learning-loop-harness/task-24.settings-json.fragment.md`
  - depends: TASK-15, TASK-21
  - tests: TC-SH-08, TC-SH-09, TC-SH-10, TC-SH-11, TC-SH-12

- [x] **TASK-25**: Hook test fixtures + scripts bash de test.
  - files: `tools/learn/testdata/hooks/stop-input.json`,
    `tools/learn/testdata/hooks/user-prompt-input.json`,
    `tools/learn/testdata/hooks/post-tool-input.json`,
    `.claude/hooks/stop-learn_test.sh`,
    `.claude/hooks/user-prompt-submit-recall_test.sh`,
    `.claude/hooks/reindex-learning_test.sh`
  - depends: TASK-21, TASK-22, TASK-23, TASK-24

### Rule + skills agentic

- [x] **TASK-26**: Rubrica `.claude/rules/skill-quality.md` com 4 critérios +
  anchor examples.
  - files: `.claude/rules/skill-quality.md`

- [x] **TASK-27**: Template `.claude/skills/_template/SKILL.md`.
  - files: `.claude/skills/_template/SKILL.md`
  - depends: TASK-26

- [x] **TASK-28**: Skill `/learn-extract` (com citação literal rubrica).
  - files: `.claude/skills/learn-extract/SKILL.md`
  - depends: TASK-14, TASK-19, TASK-26
  - tests: TC-E2E-11

- [x] **TASK-29**: Skill `/learn-refine` (com citação rubrica + diff workflow).
  - files: `.claude/skills/learn-refine/SKILL.md`
  - depends: TASK-16, TASK-19, TASK-26
  - tests: TC-E2E-04, TC-E2E-11a

- [x] **TASK-30**: Skill `/learn-nudge` (com reset counter via binário).
  - files: `.claude/skills/learn-nudge/SKILL.md`
  - depends: TASK-18, TASK-26
  - tests: TC-E2E-05

- [x] **TASK-31**: Skill `/learn-recall` (manual com track-use).
  - files: `.claude/skills/learn-recall/SKILL.md`
  - depends: TASK-17

- [x] **TASK-32**: Skill `/learn-audit-skills` (com constraint não-destrutivo) +
  `.specs/reports/.gitkeep`.
  - files: `.claude/skills/learn-audit-skills/SKILL.md`,
    `.specs/reports/.gitkeep`
  - depends: TASK-19, TASK-26
  - tests: TC-E2E-11b, TC-E2E-12, TC-E2E-12a, TC-E2E-12b

### Fragments adicionais (pra accumulator)

- [ ] **TASK-33A**: Fragment Makefile (targets `learn-setup`, `learn-reindex`,
  `learn-smoke`, `learn-stats`, `learn-lint`, `learn-test`). Criação isolada;
  consumido por TASK-34 (MERGE).
  - files: `.specs/wiring/learning-loop-harness/task-32a.makefile.fragment.md`
  - depends: TASK-14, TASK-15, TASK-20

- [ ] **TASK-33B**: Fragment `docs/harness.md` (entries pra skills, hooks,
  tooling). Criação isolada; consumido por TASK-35.
  - files: `.specs/wiring/learning-loop-harness/task-33a.docs-harness.fragment.md`
  - depends: TASK-22, TASK-23, TASK-24, TASK-28, TASK-29, TASK-30, TASK-31,
    TASK-32

- [ ] **TASK-33C**: Fragment `CLAUDE.md` (entries em Skills table + Hooks). Criação
  isolada; consumido por TASK-36.
  - files: `.specs/wiring/learning-loop-harness/task-34a.claude-md.fragment.md`
  - depends: TASK-33B

### Merges (wiring)

- [ ] **TASK-34** (MERGE-SETTINGS): Aplicar fragments JSON em
  `.claude/settings.json` via `jq`. Validação final: `jq empty
  .claude/settings.json` retorna 0.
  - files: `.claude/settings.json`
  - depends: TASK-22, TASK-23, TASK-24

- [ ] **TASK-35** (MERGE-MAKEFILE): Aplicar fragments do Makefile.
  - files: `Makefile`
  - depends: TASK-1, TASK-33A

- [ ] **TASK-36** (MERGE-HARNESS-DOC).
  - files: `docs/harness.md`
  - depends: TASK-33B, TASK-34

- [ ] **TASK-37** (MERGE-CLAUDE-MD).
  - files: `CLAUDE.md`
  - depends: TASK-33C, TASK-36

- [ ] **TASK-38**: `.gitignore` update.
  - files: `.gitignore`
  - depends: TASK-4, TASK-20

- [ ] **TASK-39** (CI-1): Step adicional em `.github/workflows/ci.yml`
  cobrindo `cd tools/learn && go test ./...` e
  `cd tools/learn && golangci-lint run ./...`.
  - files: `.github/workflows/ci.yml`
  - depends: TASK-35
  - tests: TC-E2E-13

### Documentação

- [ ] **TASK-40**: Guia operacional `docs/guides/learning-loop.md` com 7 seções.
  - files: `docs/guides/learning-loop.md`
  - depends: TASK-36
  - tests: TC-E2E-10

### E2E + smoke

- [ ] **TASK-41**: E2E fixtures + roundtrip test + `learn smoke` subcommand.
  - files: `tools/learn/testdata/e2e/`,
    `tools/learn/e2e_test.go`,
    `tools/learn/internal/cmd/smoke.go`
  - depends: TASK-5, TASK-14, TASK-15, TASK-16, TASK-17, TASK-18, TASK-19,
    TASK-20
  - tests: TC-E2E-01, TC-E2E-02, TC-E2E-03, TC-E2E-07, TC-E2E-08, TC-E2E-09,
    TC-E2E-09a

- [ ] **TASK-SMOKE**: Executar `make learn-smoke` no repo de fato.
  - Após `make learn-build && make learn-setup`, simular sequência com a
    própria spec. Verificar outputs.
  - Se binário não build: `SMOKE: DEFERRED`.
  - files: (none — execução)
  - depends: TASK-35, TASK-41
  - tests: TC-E2E-06

## Parallel Batches

**Batch 1** — Bootstrap (1 task):
- TASK-1

**Batch 2** — Foundation (2 tasks paralelas):
- TASK-2 (config), TASK-3 (learnerr)

**Batch 3** — Store + utilitários (2 tasks paralelas):
- TASK-4 (store, depends TASK-3), TASK-6 (sanitize)

**Batch 4** — Init + remaining utils (3 tasks paralelas):
- TASK-5 (init), TASK-7 (ngram + schema), TASK-8 (similar utils)

**Batch 5** — Ingest sources (4 tasks paralelas, todas depend TASK-6 do Batch 3):
- TASK-9, TASK-10, TASK-11, TASK-12

**Batch 6** — CLI commands base (3 tasks paralelas):
- TASK-13 (complete-task), TASK-15 (reindex), TASK-20 (stats)

**Batch 7** — CLI derivados (3 tasks paralelas):
- TASK-14 (extract), TASK-16 (similar), TASK-17 (recall + track-use)

**Batch 8** — CLI restantes + helper (3 tasks paralelas):
- TASK-18 (nudge-tick), TASK-19 (record/apply/validate/audit-prep), TASK-21 (hook helpers)

**Batch 9** — Hooks (3 tasks paralelas, fragments distintos pra settings.json):
- TASK-22, TASK-23, TASK-24

**Batch 10** — Hook tests + rule (2 tasks paralelas):
- TASK-25, TASK-26

**Batch 11** — Template + audit skill (2 tasks paralelas):
- TASK-27, TASK-32

**Batch 12** — Skills agentic restantes (4 tasks paralelas):
- TASK-28, TASK-29, TASK-30, TASK-31

**Batch 13** — Fragments adicionais (3 tasks paralelas, arquivos distintos):
- TASK-33A (Makefile fragment), TASK-33B (harness-doc fragment), TASK-33C (CLAUDE.md fragment)

  *Observação:* TASK-33C depende de TASK-33B (precisa saber o que vai pro
  harness inventory pra refletir corretamente em CLAUDE.md). Mantemos no
  mesmo batch porque o batch só termina quando todas acabarem, e o
  ralph-loop respeita dependências dentro do batch.

  **Ajuste:** TASK-33C move pra Batch 14 pra evitar ambiguidade. Ver Batch 14.

  Batch 13 efetivo: [TASK-33A, TASK-33B].

**Batch 14** — TASK-33C + Merges paralelos sem overlap:
- TASK-33C (CLAUDE.md fragment, depends TASK-33B)
- TASK-34 (MERGE-SETTINGS, depends TASK-22, 23, 24)
- TASK-35 (MERGE-MAKEFILE, depends TASK-33A)
- TASK-38 (.gitignore, depends TASK-4, 20)

  Files distintos: `CLAUDE.md fragment`, `.claude/settings.json`, `Makefile`,
  `.gitignore` — sem overlap. Paralelo seguro.

**Batch 15** — MERGE-HARNESS-DOC:
- TASK-36 (depends TASK-33B + TASK-34)

**Batch 16** — MERGE-CLAUDE-MD:
- TASK-37 (depends TASK-33C + TASK-36)

**Batch 17** — Docs guide + E2E + CI step (3 tasks paralelas):
- TASK-39 (CI workflow, files: ci.yml — único)
- TASK-40 (docs/guides/learning-loop.md)
- TASK-41 (E2E + smoke subcommand, files: tools/learn/* + testdata)

**Batch 18** — Smoke real:
- TASK-SMOKE

### File classification

- **Exclusive**: cada arquivo em `tools/learn/internal/*`, `.claude/hooks/*.sh`,
  `.claude/skills/*/SKILL.md` é tocado por 1 task específica.
- **Shared-additive (accumulator)**:
  - `.claude/settings.json` (3 hook tasks → 3 fragments → MERGE em TASK-34 via `jq`)
  - `Makefile` (TASK-1 + TASK-33A → 2 fragments → MERGE em TASK-35)
  - `docs/harness.md` (fragment em TASK-33B → MERGE em TASK-36 — single fragment,
    formal accumulator overkill mas mantido pra audit trail)
  - `CLAUDE.md` (fragment em TASK-33C → MERGE em TASK-37)
- **Shared-mutative**: nenhum. `.gitignore` é additive (TASK-38 single-owner).

## Validation Criteria

- [ ] `make lint` passa (cobertura root via `golangci-lint run ./...`)
- [ ] `make learn-lint` passa (cobre `tools/learn/...` via
  `cd tools/learn && golangci-lint run ./...`)
- [ ] `make test` passa (root)
- [ ] `make learn-test` passa (`cd tools/learn && go test ./...`)
- [ ] `make learn-build` produz binário em `bin/learn`
- [ ] `make learn-setup` em dir limpo cria `.claude/learning/db.sqlite` com
  schema completo (incl. FTS5 virtual tables, triggers, PRAGMA WAL ativo)
- [ ] `make learn-smoke` exit 0 com fixtures (TC-E2E-06)
- [ ] TASK-SMOKE passa no repo real (ou `SMOKE: DEFERRED` com razão)
- [ ] `swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal`
  passa (spec não toca handlers HTTP)
- [ ] `jq empty .claude/settings.json` retorna 0 após TASK-34 (validação JSON)
- [ ] `make ci-local` passa, incluindo cobertura `tools/learn/`
- [ ] `go build ./...` (root) NÃO compila `modernc.org/sqlite` no build graph
  (TC-UC-33)
- [ ] Inspeção: `docs/harness.md` lista todas adições (TC-E2E-09, 09a)
- [ ] Inspeção: `CLAUDE.md` lista 5 skills novas
- [ ] Inspeção: `.gitignore` ignora `.claude/learning/db.sqlite` e `learn.log`
- [ ] Inspeção: rodar `/learn-audit-skills` produz relatório em
  `.specs/reports/skill-audit-<date>.md` com 14 skills avaliadas, sem mtime de
  nenhuma skill alterado (TC-E2E-12, 12a, 12b)
- [ ] Inspeção: `learn stats` JSON populado após TASK-SMOKE

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### TASK-1 (2026-05-14 18:42)

TDD: RED(3 compile-fail) → GREEN(3 passing) → REFACTOR(clean) — go.mod separado
em `tools/learn/` com cobra 1.10.2; `internal/cli/` com `Run` + `NewRootCmd`
testáveis; `cmd/learn/main.go` thin entry; root usa `RunE` explícito pra
rejeitar args desconhecidos (cobra com SilenceErrors+no-Run cai em help silencioso
mesmo com NoArgs). TC-UC-30, TC-UC-30a, TC-UC-30b passando. TC-UC-34 verificado
via `cd tools/learn && go build ./...` + `make build`. Fragment Makefile escrito.

### Batch 2 [TASK-2, TASK-3] (2026-05-14 18:50)

Parallel via worktrees.

- TASK-2 (config): TDD RED(12 funcs compile-fail) → GREEN(12 passing, 41 subtests).
  `internal/config/` com `Config`, `SecretPattern`, `DefaultConfig`,
  `DefaultSecretPatterns`, `Load(path)` aceitando defaults+YAML+env override.
  9 fields tipados + ranges validados (citando field+value em erros).
  yaml.v3 v3.0.1 adicionado ao go.mod. Defaults pra `SecretPatterns`
  cobrem AWS, OpenAI/Anthropic, GitHub PAT (clássico + fine-grained),
  Slack tokens, SSH paths, .env lines. Missing file → fallback silencioso
  pra defaults (errors.Is fs.ErrNotExist).
- TASK-3 (learnerr): TDD RED(11 funcs compile-fail) → GREEN(11 passing).
  `internal/learnerr/` com `UsageError`, `RuntimeError`, `Usagef`,
  `Runtimef`, `ExitCode`. `splitWrapped` helper extrai wrapped causa em
  fmt-style %w pra round-trip lossless. `internal/cli/cli.go` atualizado:
  `Run` agora retorna `learnerr.ExitCode(err)` e emite `Error: <msg>` pro
  stderr; root `RunE` emite `learnerr.Usagef` em vez de `fmt.Errorf`,
  preservando TC-UC-30 exit 1.
- Merge: worktree TASK-3 auto-mergeada (runtime); worktree TASK-2 copiada
  manual (internal/config + go.mod/go.sum). `go mod tidy` reconciliou.
  4 pacotes verdes: cmd/learn (build-only), cli, config, learnerr.
- Worktrees prunadas; estado limpo.

### Batch 3 [TASK-4, TASK-6] (2026-05-14 18:58)

Parallel via worktrees.

- TASK-4 (store): TDD RED(9 funcs compile-fail) → GREEN(9 passing).
  `internal/store/` com `schema.sql` embedado (10 base tables + 3 FTS5
  virtual tables + 9 sync triggers AI/AD/AU), `store.go` com
  `Open(path)` aplicando PRAGMA journal_mode=WAL (return verificado="wal"),
  busy_timeout=5000, foreign_keys=ON, `SetMaxOpenConns(1)` pra garantir
  PRAGMA aplicado em todas as conexões, `Close`/`DB`; `queries.go` com
  `InsertEvent`, `IncrementNudgeCounter` (UPDATE...RETURNING atômico,
  testado com 10 goroutines), `GetNudgeCounter`, `ResetNudgeCounter`,
  `UpsertSkillIndex`, `UpsertMemoryIndex`, `DeleteOrphan*`. Erros wrapped
  como `*learnerr.RuntimeError`. modernc.org/sqlite v1.50.1 adicionado.
  TCs cobertas: TC-UC-01 (parte), TC-UC-01a, TC-UC-02, TC-UC-31,
  concorrência counter, FTS5 trigger sync (skill+memory, insert+update),
  delete-orphan cascade.
- TASK-6 (sanitize): TDD RED(4 funcs compile-fail) → GREEN(17 subtests).
  `internal/sanitize/` com `Pattern`, `Sanitizer`, `New`,
  `NewFromConfig` (reusa `config.DefaultSecretPatterns` sem duplicar
  regex), `Sanitize`, `SanitizeBytes`. Auto-promotes line-anchored patterns
  a `(?m)` pra env_value matchar per-line. TCs: TC-D-04 (AWS),
  TC-D-05a–g (7 token classes), TC-D-06 (env multi-line), TC-D-07
  (false-positive policy), TC-D-07a (filesystem purity snapshot).
- Merge: TASK-4 escreveu direto no main tree (worktree não tinha
  `tools/learn/`); TASK-6 worktree copiada (`internal/sanitize/`).
  5 pacotes verdes total: cli, config, learnerr, sanitize, store.
- Worktrees prunadas.
