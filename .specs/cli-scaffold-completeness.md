# Spec: CLI Scaffold — Completeness Refactor

## Status: DONE

## Context

O CLI scaffold (`gopherplate new my-service`) atualmente exclui varios arquivos/diretorios que deveriam estar presentes no novo projeto. O resultado e que projetos criados pelo scaffold ficam sem:

1. **`.claude/`** — toda a infraestrutura de Claude Code (agents, hooks, rules, skills) e excluida. Novos projetos perdem 14 skills, 7 hooks, 4 rules e 3 agents especializados.
2. **`.devcontainer/`** — configuracao de DevContainer excluida. Novos projetos nao tem sandbox pronto.
3. **`.specs/`** — template de specs excluido. Novos projetos nao tem estrutura para SDD workflow.
4. **`CLAUDE.md`**, **`AGENTS.md`**, **`CONTRIBUTING.md`** — excluidos. Novos projetos perdem documentacao essencial.
5. **`.markdownlint.json`** — excluido desnecessariamente.

Ao mesmo tempo, arquivos template-specific sao copiados indevidamente:

6. **`roadmap.md`** — roadmap do template, irrelevante para novos projetos.
7. **`docs/guides/template-cli.md`** — guia do CLI scaffold, nao faz sentido no projeto gerado.

**Solucao:** Refatorar `ExcludePaths`, adicionar pos-processamento para adaptar conteudo template-specific, e atualizar `serviceNameFiles` para os novos arquivos incluidos.

## Requirements

- [ ] REQ-1: **`.claude/` incluido com filtragem seletiva**
  - GIVEN o scaffold copia o projeto
  - WHEN `.claude/` e processado
  - THEN copia tudo EXCETO `.claude/worktrees/` e `.claude/settings.local.json`
  - AND hooks, rules, skills, agents e settings.json estao presentes no novo projeto

- [ ] REQ-2: **`.devcontainer/` incluido com adaptacao**
  - GIVEN o scaffold copia `.devcontainer/`
  - WHEN o service name e substituido
  - THEN `devcontainer.json` tem nomes de volumes adaptados (`<service>-bashhistory-*`, `<service>-claude-config-*`, `<service>-gopath-*`)
  - AND `init-firewall.sh` tem referencia ao novo service name
  - AND `Dockerfile` do devcontainer e copiado intacto

- [ ] REQ-3: **`.specs/` incluido com estrutura vazia**
  - GIVEN o scaffold copia `.specs/`
  - WHEN o novo projeto e criado
  - THEN `.specs/TEMPLATE.md` e `.specs/.gitkeep` e `.specs/.gitignore` estao presentes
  - AND specs do template (dx-sdd-tdd-parallelism.md, error-handling-refactor.md, etc.) NAO sao copiadas

- [ ] REQ-4: **`CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md` incluidos com adaptacao**
  - GIVEN o scaffold copia estes arquivos
  - WHEN o service name e substituido
  - THEN referencias a `jrmarcello/gopherplate` sao substituidas pelo module path do novo projeto
  - AND referencias ao CLI `gopherplate new/add` sao removidas ou genericizadas
  - AND conteudo generico (arquitetura, padroes, convencoes) permanece intacto

- [ ] REQ-5: **`.markdownlint.json` incluido**
  - GIVEN o scaffold copia arquivos da raiz
  - WHEN `.markdownlint.json` e processado
  - THEN o arquivo e copiado para o novo projeto

- [ ] REQ-6: **`roadmap.md` excluido**
  - GIVEN o scaffold copia arquivos da raiz
  - WHEN `roadmap.md` e processado
  - THEN o arquivo NAO e copiado para o novo projeto

- [ ] REQ-7: **`docs/guides/template-cli.md` excluido**
  - GIVEN o scaffold copia docs
  - WHEN `docs/guides/template-cli.md` e processado
  - THEN o arquivo NAO e copiado para o novo projeto

- [ ] REQ-8: **Specs do template excluidas**
  - GIVEN o scaffold copia `.specs/`
  - WHEN specs existentes do template sao processadas
  - THEN arquivos como `dx-sdd-tdd-parallelism.md`, `error-handling-refactor.md`, `bootstrap-di-container.md`, `load-tests-modular.md` e `*.active.md` NAO sao copiados
  - AND apenas `TEMPLATE.md`, `.gitkeep` e `.gitignore` sao mantidos

- [ ] REQ-9: **Testes atualizados refletindo novo comportamento**
  - GIVEN os testes de copy/shouldExclude existem
  - WHEN executados
  - THEN refletem a nova lista de exclusoes e inclusoes
  - AND `go test ./cmd/cli/...` passa

- [ ] REQ-10: **`serviceNameFiles` atualizado para novos arquivos**
  - GIVEN `.devcontainer/devcontainer.json` e `.devcontainer/init-firewall.sh` sao agora copiados
  - WHEN `ReplaceServiceName()` executa
  - THEN substitui `gopherplate` pelo novo service name nestes arquivos

## Test Plan

### Unit Tests

| TC | REQ | Category | Description | Expected |
|----|-----|----------|-------------|----------|
| TC-U-01 | REQ-1 | happy | CopyProject includes .claude/settings.json | file exists in destination |
| TC-U-02 | REQ-1 | happy | CopyProject includes .claude/hooks/guard-bash.sh | file exists |
| TC-U-03 | REQ-1 | happy | CopyProject includes .claude/rules/go-conventions.md | file exists |
| TC-U-04 | REQ-1 | happy | CopyProject includes .claude/agents/code-reviewer.md | file exists |
| TC-U-05 | REQ-1 | happy | CopyProject includes .claude/skills/validate/SKILL.md | file exists |
| TC-U-06 | REQ-1 | edge | CopyProject excludes .claude/worktrees/ | dir not copied |
| TC-U-07 | REQ-1 | edge | CopyProject excludes .claude/settings.local.json | file not copied |
| TC-U-08 | REQ-2 | happy | CopyProject includes .devcontainer/devcontainer.json | file exists |
| TC-U-09 | REQ-2 | happy | CopyProject includes .devcontainer/init-firewall.sh | file exists |
| TC-U-10 | REQ-3 | happy | CopyProject includes .specs/TEMPLATE.md | file exists |
| TC-U-11 | REQ-3 | happy | CopyProject includes .specs/.gitkeep | file exists |
| TC-U-12 | REQ-8 | edge | CopyProject excludes .specs/dx-sdd-tdd-parallelism.md | file not copied |
| TC-U-13 | REQ-8 | edge | CopyProject excludes .specs/*.active.md | file not copied |
| TC-U-14 | REQ-4 | happy | CopyProject includes CLAUDE.md | file exists |
| TC-U-15 | REQ-4 | happy | CopyProject includes AGENTS.md | file exists |
| TC-U-16 | REQ-4 | happy | CopyProject includes CONTRIBUTING.md | file exists |
| TC-U-17 | REQ-5 | happy | CopyProject includes .markdownlint.json | file exists |
| TC-U-18 | REQ-6 | edge | CopyProject excludes roadmap.md | file not copied |
| TC-U-19 | REQ-7 | edge | CopyProject excludes docs/guides/template-cli.md | file not copied |
| TC-U-20 | REQ-9 | happy | shouldExclude returns correct results for new rules | all assertions pass |
| TC-U-21 | REQ-10 | happy | ReplaceServiceName updates devcontainer.json volumes | gopherplate replaced |
| TC-U-22 | REQ-10 | happy | ReplaceServiceName updates init-firewall.sh | gopherplate replaced |
| TC-U-23 | REQ-8 | edge | shouldExclude matches .specs/*.md but not TEMPLATE.md | TEMPLATE.md passes, others excluded |

## Design

### Architecture Decisions

**Filtragem seletiva de `.claude/`:**
Em vez de excluir `.claude/` inteiro e depois copiar de volta, usamos exclusoes granulares:
- `.claude/worktrees/` — runtime artifacts, sempre excluir
- `.claude/settings.local.json` — local/personal settings, sempre excluir
- Todo o resto (hooks, rules, skills, agents, settings.json) e copiado normalmente

**Filtragem seletiva de `.specs/`:**
- `.specs/TEMPLATE.md`, `.specs/.gitkeep`, `.specs/.gitignore` — incluir (estrutura vazia)
- Qualquer outro `.specs/*.md` — excluir (specs do template)
- `.specs/*.active.md` — excluir (runtime state)

Isso requer mudar `shouldExclude` para suportar um novo tipo de regra: "exclude files matching a pattern within a directory, except specific ones". A abordagem mais simples: trocar a exclusao de `.specs/` por exclusoes especificas dos arquivos de spec conhecidos via uma funcao `isSpecTemplateFile()`.

Alternativa mais robusta: excluir `.specs/` do ExcludePaths e criar um `SpecsAllowList` com os arquivos que devem ser mantidos. No pos-processamento (step 9 do new.go), deletar tudo em `.specs/` que nao esta no allowlist.

**Decisao: usar pos-processamento (step 9)** para limpar `.specs/` — e mais simples e nao complica a logica de `shouldExclude`.

**Adaptacao de conteudo (CLAUDE.md, AGENTS.md, CONTRIBUTING.md):**
O `ReplaceServiceName()` ja faz find-replace de `gopherplate` -> novo nome em arquivos listados em `serviceNameFiles`. Basta adicionar estes arquivos a lista. O module path (`github.com/jrmarcello/gopherplate`) ja e reescrito pelo `RewriteModulePath()` que processa todos os `.go` e `.md` files.

**Adaptacao do `.devcontainer/devcontainer.json`:**
Os volumes usam `boilerplate-*` como prefixo. O `ReplaceServiceName()` ja substitui `gopherplate` -> novo nome. Adicionar `devcontainer.json` e `init-firewall.sh` a `serviceNameFiles`.

### Files to Modify

- `cmd/cli/templates/boilerplate/snapshot.go` — refatorar ExcludePaths
- `cmd/cli/templates/boilerplate/servicename.go` — adicionar novos arquivos a serviceNameFiles
- `cmd/cli/templates/boilerplate/copy_test.go` — atualizar testes
- `cmd/cli/commands/new.go` — adicionar step de cleanup .specs/ e template-specific files

### Dependencies

- Nenhuma dependencia externa nova

## Tasks

- [x] TASK-1: Refatorar ExcludePaths em snapshot.go
  - Remover da lista: `.claude/`, `CLAUDE.md`, `AGENTS.md`, `.specs/`, `.devcontainer/`, `CONTRIBUTING.md`, `.markdownlint.json`
  - Adicionar exclusoes granulares: `.claude/worktrees/`, `.claude/settings.local.json`
  - Adicionar: `roadmap.md`, `docs/guides/template-cli.md`
  - Manter existentes: `cmd/cli/`, `.git/`, `bin/`, `tests/coverage/`, `tests/load/results/`, `.github/`, `cliff.toml`, `CHANGELOG.md`, `docs/modules/`, `.env`
  - files: `cmd/cli/templates/boilerplate/snapshot.go`

- [x] TASK-2: Adicionar novos arquivos a serviceNameFiles em servicename.go
  - Adicionar: `.devcontainer/devcontainer.json`, `.devcontainer/init-firewall.sh`, `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`
  - Nota: `gopherplate` aparece nesses arquivos em nomes de volumes, descricoes, URLs do GitHub — tudo substituido pelo novo service name
  - files: `cmd/cli/templates/boilerplate/servicename.go`
  - depends: TASK-1

- [x] TASK-3: Adicionar cleanup de .specs/ e template-specific files no new.go
  - Apos step 8 (CleanupWiring), adicionar step 9:
    - Limpar `.specs/`: manter apenas `TEMPLATE.md`, `.gitkeep`, `.gitignore` — deletar todos os outros `.md` e `.active.md`
    - Deletar `docs/swagger.json`, `docs/swagger.yaml` (ja existente)
    - Se !KeepExamples: deletar `docs/docs.go` (ja existente)
  - files: `cmd/cli/commands/new.go`
  - depends: TASK-1

- [x] TASK-4: Atualizar testes de copy e shouldExclude
  - Atualizar `TestCopyProject`:
    - Adicionar a files source: `.claude/settings.json`, `.claude/hooks/guard.sh`, `.claude/worktrees/abc/file`, `.claude/settings.local.json`, `.devcontainer/devcontainer.json`, `.specs/TEMPLATE.md`, `.specs/my-feature.md`, `.specs/my-feature.active.md`, `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, `.markdownlint.json`, `roadmap.md`, `docs/guides/template-cli.md`
    - Mover para shouldExist: `.claude/settings.json`, `.claude/hooks/guard.sh`, `.devcontainer/devcontainer.json`, `.specs/TEMPLATE.md`, `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, `.markdownlint.json`
    - Mover para shouldNotExist: `.claude/worktrees/abc/file`, `.claude/settings.local.json`, `roadmap.md`, `docs/guides/template-cli.md`
    - Nota: `.specs/my-feature.md` e `.specs/my-feature.active.md` serao copiados pelo CopyProject (que nao filtra por conteudo) mas deletados pelo cleanup step no new.go. O teste de CopyProject so valida a copia, nao o cleanup.
  - Atualizar `TestShouldExclude`:
    - Remover cases que agora passam: `.claude/rules/go.md` (false), `CLAUDE.md` (false), `AGENTS.md` (false), `.specs/template-cli.md` (false)
    - Adicionar cases: `.claude/worktrees/abc` (true), `.claude/settings.local.json` (true), `roadmap.md` (true), `docs/guides/template-cli.md` (true), `.devcontainer/devcontainer.json` (false), `.markdownlint.json` (false), `CONTRIBUTING.md` (false)
  - files: `cmd/cli/templates/boilerplate/copy_test.go`
  - tests: TC-U-01 a TC-U-23
  - depends: TASK-1

## Parallel Batches

```
Batch 1: [TASK-1]              — foundation (ExcludePaths change)
Batch 2: [TASK-2, TASK-3]      — parallel (servicename.go vs new.go, exclusive files)
Batch 3: [TASK-4]              — tests (depends on all changes being in place)
```

File overlap analysis:
- `cmd/cli/templates/boilerplate/snapshot.go`: TASK-1 only -> exclusive
- `cmd/cli/templates/boilerplate/servicename.go`: TASK-2 only -> exclusive
- `cmd/cli/commands/new.go`: TASK-3 only -> exclusive
- `cmd/cli/templates/boilerplate/copy_test.go`: TASK-4 only -> exclusive

## Validation Criteria

- [ ] `go build ./...` passa
- [ ] `make lint` passa (0 issues)
- [ ] `go test ./cmd/cli/...` passa (todos os testes)
- [ ] `make test-unit` passa (zero regressoes em outros pacotes)
- [ ] ExcludePaths nao contem `.claude/`, `CLAUDE.md`, `AGENTS.md`, `.specs/`, `.devcontainer/`, `CONTRIBUTING.md`, `.markdownlint.json`
- [ ] ExcludePaths contem: `.claude/worktrees/`, `.claude/settings.local.json`, `roadmap.md`, `docs/guides/template-cli.md`
- [ ] `serviceNameFiles` contem: `.devcontainer/devcontainer.json`, `.devcontainer/init-firewall.sh`, `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`
- [ ] Teste manual: `gopherplate new test-svc --yes` cria projeto com `.claude/`, `.devcontainer/`, `.specs/TEMPLATE.md`, `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, `.markdownlint.json`
- [ ] Teste manual: projeto criado NAO contem `roadmap.md`, `docs/guides/template-cli.md`, `.claude/worktrees/`, `.claude/settings.local.json`, specs do template

## Execution Log

<!-- Ralph Loop appends here automatically — do not edit manually -->

### Iteration 1 — TASK-1 (2026-04-12 17:00)

Rewrote `ExcludePaths` in `snapshot.go`. Removed 7 entries (.claude/, CLAUDE.md, AGENTS.md, .specs/, .devcontainer/, CONTRIBUTING.md, .markdownlint.json). Added 4 entries (.claude/worktrees/, .claude/settings.local.json, roadmap.md, docs/guides/template-cli.md). Net result: new projects get .claude/ infra, .devcontainer/, .specs/, CLAUDE.md, AGENTS.md, CONTRIBUTING.md, .markdownlint.json.

### Iteration 2 — Batch 2: TASK-2, TASK-3 (2026-04-12 17:05)

Executed in parallel via worktree agents. TASK-2: added 5 entries to `serviceNameFiles` in servicename.go (CLAUDE.md, AGENTS.md, CONTRIBUTING.md, devcontainer.json, init-firewall.sh). TASK-3: added .specs/ cleanup step in new.go — uses `os.ReadDir` with allowlist (TEMPLATE.md, .gitkeep, .gitignore) to delete template specs after copy.

### Iteration 3 — Batch 3: TASK-4 (2026-04-12 17:42)

Rewrote `copy_test.go` with 31 source files, 19 shouldExist, 14 shouldNotExist covering TC-U-01 to TC-U-23. TestShouldExclude expanded to 34 cases covering all new rules. Fixed `defaultServiceName` in servicename.go (worktree agent reverted to "go-boilerplate"). All CLI tests pass (0 failures), build green, lint 0 issues.
TDD: RED(servicename_test failing) -> GREEN(34/34 shouldExclude + copy tests pass) -> REFACTOR(clean).
