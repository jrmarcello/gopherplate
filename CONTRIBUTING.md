# Guia de Contribuicao

Obrigado pelo interesse em contribuir com o **Go Boilerplate**!

## Como comecar

1. Clone o projeto: `git clone https://bitbucket.org/appmax-space/go-boilerplate`
2. Crie uma branch: `git checkout -b feat/minha-feature`
3. Setup: `make setup` (instala tools, sobe Docker, roda migrations)

## Propor features e reportar bugs

Use o **Issue Tracker** do Bitbucket para sugerir melhorias ou reportar problemas:

1. Acesse [Issues](https://bitbucket.org/appmax-space/go-boilerplate/issues)
2. Verifique se ja existe uma issue similar
3. Crie uma nova issue com o tipo adequado:
   - **Bug**: algo nao funciona como esperado (inclua steps to reproduce)
   - **Enhancement**: nova funcionalidade ou melhoria (descreva o problema, nao so a solucao)
   - **Task**: melhoria tecnica, refactoring, docs

Se quiser implementar a feature, comente na issue antes de comecar para alinhar a abordagem.

## Desenvolvimento

Ferramentas necessarias: `Go 1.25`, `Docker`, `Make`.

```bash
make setup     # Setup completo (tools + Docker + migrations)
make dev       # Servidor com hot reload
make test      # Todos os testes
make lint      # golangci-lint + gofmt
```

Ferramentas opcionais (o Makefile mostra como instalar se faltarem):

- `k6` para load tests (`make load-smoke`)
- `kind` + `kubectl` para Kubernetes local (`make kind-setup`)

## Commits

Seguimos **Conventional Commits** (enforced por Lefthook):

```text
feat(scope): nova funcionalidade
fix(scope): correcao de bug
docs(scope): documentacao
refactor(scope): mudanca sem alterar comportamento
test(scope): testes
chore(scope): configuracao, dependencias
```

Exemplo: `feat(api): add pagination to list endpoint`

O scope e opcional mas recomendado: `api`, `config`, `cache`, `db`, `auth`, `ci`, `dx`, `docs`.

## Pull requests

Ao abrir um PR:

1. Descreva claramente o que foi feito e por que
2. Garanta que `make lint` e `make test` passam
3. Se mudou a API, regenere o Swagger: `make swagger`
4. Se adicionou features, atualize o `CHANGELOG.md`

O pipeline roda automaticamente: lint, vulncheck, unit tests e E2E tests em paralelo.

## Testes

Novas funcionalidades devem incluir:

- **Testes unitarios** para domain e usecases (hand-written mocks em `mock_test.go`)
- **Testes de repositorio** com go-sqlmock
- **Testes E2E** com TestContainers para mudancas criticas

## Arquitetura

Antes de criar ou modificar arquivos, consulte:

- `CLAUDE.md` — visao geral da arquitetura e padroes
- `AGENTS.md` — regras e convencoes detalhadas
- `docs/adr/` — decisoes arquiteturais (Clean Architecture, IDs, config, errors, auth, migrations, pkg/)
