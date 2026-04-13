# Template CLI

O **gopherplate CLI** é uma ferramenta de linha de comando que gera novos microsserviços a partir deste template. Em vez de clonar o repositório e fazer find-replace manual, um único comando cria um projeto configurado com a arquitetura correta, dependências selecionadas e código pronto para rodar.

---

## Sumário

- [Instalação](#instalação)
- [Quick Start](#quick-start)
- [Comandos](#comandos)
  - [`gopherplate new`](#gopherplate-new-service-name)
  - [`gopherplate add domain`](#gopherplate-add-domain-name)
  - [`gopherplate remove domain`](#gopherplate-remove-domain-name)
  - [`gopherplate add endpoint`](#gopherplate-add-endpoint-domain-name)
  - [`gopherplate remove endpoint`](#gopherplate-remove-endpoint-domain-name)
  - [`gopherplate wiring`](#gopherplate-wiring)
  - [`gopherplate doctor`](#gopherplate-doctor)
  - [`gopherplate version`](#gopherplate-version)
- [Flags (modo não-interativo)](#flags-modo-não-interativo)
- [O que cada opção faz](#o-que-cada-opção-faz)
- [Customização dos templates](#customização-dos-templates)
- [Em breve (Roadmap)](#em-breve-roadmap)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

---

## Instalação

### Pré-requisitos

- **Go 1.25+** instalado e configurado
- `$GOBIN` (ou `$GOPATH/bin`) presente no `$PATH`

### Instalando

```bash
go install github.com/jrmarcello/gopherplate/cmd/cli@latest
```

### Verificando

```bash
gopherplate version
# gopherplate dev
```

> A versão mostra `dev` quando compilado localmente. Releases futuras terão versionamento via ldflags.

---

## Quick Start

O fluxo mais comum: criar um novo serviço, responder aos prompts e começar a desenvolver.

```bash
gopherplate new payment-service
# Responda aos prompts interativos...

cd payment-service
make setup    # Instala ferramentas + sobe Docker + roda migrations
make dev      # Inicia o servidor com hot reload
```

Em poucos minutos você tem um microsserviço rodando com Clean Architecture, observabilidade e infraestrutura configurada.

---

## Comandos

### `gopherplate new [service-name]`

Cria um novo projeto completo a partir do template. O comando gera toda a estrutura de diretórios, configura dependências e deixa o projeto pronto para `make setup && make dev`.

#### Prompts interativos

Ao executar `gopherplate new`, o CLI guia você por uma série de perguntas:

| # | Prompt | Opções | Descrição |
|---|--------|--------|-----------|
| 1 | Nome do serviço | texto livre | Nome do diretório e referência interna (ex: `payment-service`) |
| 2 | Module path | texto livre | Go module path completo (ex: `github.com/yourorg/payment-service`) |
| 3 | Banco de dados | PostgreSQL / MySQL / SQLite3 / Outro | Driver de banco de dados que será configurado no projeto |
| 4 | Protocolo | HTTP/REST (Gin) / ~~gRPC~~ | Protocolo de comunicação da API (gRPC em breve) |
| 5 | Injeção de dependência | Manual / ~~Uber Fx~~ | Estratégia de DI (Uber Fx em breve) |
| 6 | Cache Redis? | sim / não | Habilita cache com Redis (pkg/cache) |
| 7 | Idempotência? | sim / não | Habilita middleware de idempotência (só aparece se Redis = sim) |
| 8 | Service Key Auth? | sim / não | Habilita autenticação service-to-service via headers |
| 9 | Manter domínios de exemplo? | sim / não | Mantém os domínios `user` e `role` como referência |

#### Exemplo completo

```bash
$ gopherplate new payment-service

  Nome do serviço []: payment-service
  Module path [github.com/appmax/payment-service]: github.com/yourorg/payment-service
  Banco de dados (postgres/mysql/sqlite3/other) [postgres]: postgres

  Protocolo: HTTP/REST (Gin) [gRPC: em breve]
  Injeção de dependência: Manual [Uber Fx: em breve]

  Incluir cache Redis? [Y/n]: y
  Incluir idempotência? [Y/n]: y
  Incluir Service Key Auth? [Y/n]: y
  Manter domínios de exemplo (user/role)? [Y/n]: n

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Resumo
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Serviço:      payment-service
  Module:       github.com/yourorg/payment-service
  Banco:        postgres
  Protocolo:    http
  DI:           manual
  Redis:        sim
  Idempotência: sim
  Auth:         sim
  Exemplos:     não
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Criando payment-service...

  Rewriting module path...
  Replacing service name...
  Removing disabled features...
  Cleaning up wiring...
  Initializing git...
  Running go mod tidy...

Projeto 'payment-service' criado com sucesso!

Próximos passos:
  cd payment-service
  make setup     # Instala tools + sobe Docker + roda migrations
  make dev       # Inicia servidor com hot reload
```

#### Estrutura gerada

```text
payment-service/
├── cmd/
│   ├── api/
│   │   ├── main.go              # Entrypoint da aplicação
│   │   ├── server.go            # DI manual e inicialização
│   │   └── doc.go               # Swagger metadata
│   └── migrate/
│       └── main.go              # CLI de migrations
├── config/
│   └── config.go                # Carregamento de configuração (env vars)
├── internal/
│   ├── domain/                  # Camada de domínio (zero dependências externas)
│   ├── usecases/                # Camada de aplicação (use cases + DTOs)
│   └── infrastructure/
│       ├── db/postgres/
│       │   ├── repository/      # Implementações de repositório
│       │   └── migration/       # Migrations SQL (Goose)
│       ├── web/
│       │   ├── handler/         # HTTP handlers (Gin)
│       │   ├── middleware/       # Logger, metrics, idempotency, auth
│       │   └── router/          # Registro de rotas
│       └── telemetry/           # Métricas de negócio (OpenTelemetry)
├── pkg/                         # Pacotes reutilizáveis
│   ├── apperror/                # Erros estruturados com HTTP status
│   ├── cache/                   # Interface de cache + implementação Redis
│   ├── database/                # Conexão DB com Writer/Reader cluster
│   ├── httputil/                # Helpers de resposta HTTP padronizada
│   ├── idempotency/             # Store de idempotência (Redis)
│   ├── logutil/                 # Logging estruturado com mascaramento PII
│   └── telemetry/               # Setup OpenTelemetry (traces + métricas)
├── tests/
│   └── e2e/                     # Testes E2E com TestContainers
├── docs/                        # Documentação
├── deploy/                      # Kustomize overlays (staging/production)
├── .env.example                 # Template de variáveis de ambiente
├── docker/
│   ├── docker-compose.yml       # Infraestrutura local (Postgres, Redis)
│   └── Dockerfile               # Build multi-stage
├── Makefile                     # Comandos de desenvolvimento
├── go.mod
└── go.sum
```

> **Nota:** Se você respondeu "não" para Redis, os diretórios `pkg/cache/`, `pkg/idempotency/` e o middleware de idempotência não são incluídos. O mesmo vale para Service Key Auth e o middleware correspondente.

---

### `gopherplate add domain [name]`

Adiciona um novo domínio a um projeto existente. Gera todas as camadas da Clean Architecture para o domínio especificado: entity, use cases, repository, handler, router e migration.

#### Uso

```bash
cd payment-service
gopherplate add domain order
```

#### Arquivos gerados

```text
internal/
├── domain/order/
│   ├── entity.go                # Aggregate Order com factory NewOrder()
│   ├── errors.go                # Erros de domínio (ErrNotFound, etc.)
│   └── filter.go                # Filtros de listagem
│
├── usecases/order/
│   ├── create.go                # CreateUseCase
│   ├── get.go                   # GetUseCase
│   ├── list.go                  # ListUseCase
│   ├── update.go                # UpdateUseCase
│   ├── delete.go                # DeleteUseCase
│   ├── dto/                     # Input/Output DTOs
│   │   ├── create.go
│   │   ├── get.go
│   │   ├── list.go
│   │   ├── update.go
│   │   └── delete.go
│   └── interfaces/
│       └── repository.go        # Interface do repositório
│
└── infrastructure/
    ├── db/postgres/
    │   ├── repository/
    │   │   └── order.go         # Implementação do repositório (sqlx)
    │   └── migration/
    │       └── 20260329120000_create_orders.sql
    ├── web/
    │   ├── handler/
    │   │   └── order.go         # HTTP handlers
    │   └── router/
    │       └── order.go         # Registro de rotas
```

#### Próximos passos após `add domain`

A forma mais rápida e segura é deixar o CLI regenerar o wiring para todos os domínios detectados:

```bash
gopherplate wiring   # auto-gera server.go + router.go + container.go + test_helpers.go
make migrate-up      # cria a tabela no banco
```

Se preferir wiring manual, o CLI imprime instruções com código copy-pasteable após `add domain`:

1. Registre as dependências em `cmd/api/server.go:buildDependencies()`
2. Registre as rotas em `internal/infrastructure/web/router/router.go`
3. Execute `make migrate-up`
4. Edite a entity, value objects e use cases conforme sua regra de negócio

---

### `gopherplate remove domain [name]`

Remove um domínio inteiro do projeto (inverso de `add domain`).

```bash
gopherplate remove domain order          # pede confirmação (default N)
gopherplate remove domain order --yes    # pula confirmação
```

#### O que faz

- Lista todos os arquivos a serem deletados (domain/, usecases/, repository, handler, router)
- Pede confirmação com lista completa — default é **N** (não deletar)
- Remove com `os.RemoveAll`
- **Preserva migrations** por padrão (risco de perda de dados) — apenas lista para revisão manual
- Imprime instruções de cleanup manual (remover wiring residual de server.go/router.go ou rodar `gopherplate wiring`)

#### Validações

- Domínio deve existir em `internal/domain/<name>/`, senão retorna erro

---

### `gopherplate add endpoint [domain] [name]`

Scaffolda um endpoint customizado (não-CRUD) dentro de um domínio existente.

```bash
gopherplate add endpoint order cancel
```

#### Gera

- `internal/usecases/order/cancel.go` — use case com `ClassifyError`, `toAppError`, `SpanFromContext`
- `internal/usecases/order/dto/cancel.go` — Input/Output DTOs
- `internal/usecases/order/cancel_test.go` — testes unitários com mock + assertions de `*apperror.AppError`

#### Próximos passos

1. Adicionar método handler em `internal/infrastructure/web/handler/<domain>.go`
2. Adicionar rota em `internal/infrastructure/web/router/<domain>.go`
3. Rodar `gopherplate wiring` para auto-wiring (ou wiring manual em `cmd/api/server.go`)

#### Validações

- Domínio deve existir
- Nome do endpoint não pode ser CRUD padrão (`create`, `get`, `update`, `delete`, `list`) — esses já existem por convenção
- Endpoint não pode já existir
- Nome deve ser snake_case e começar com letra

---

### `gopherplate remove endpoint [domain] [name]`

Remove um endpoint customizado (inverso de `add endpoint`).

```bash
gopherplate remove endpoint order cancel          # pede confirmação (default N)
gopherplate remove endpoint order cancel --yes    # pula confirmação
```

#### Remove

- `internal/usecases/<domain>/<name>.go`
- `internal/usecases/<domain>/dto/<name>.go`
- `internal/usecases/<domain>/<name>_test.go`

#### Proteção CRUD

Bloqueia remoção de `create`/`get`/`update`/`delete`/`list` — para remover esses, use `gopherplate remove domain` (remove o domínio inteiro).

---

### `gopherplate wiring`

Auto-regenera os 4 arquivos de wiring a partir dos domínios detectados em `internal/domain/`.

```bash
gopherplate wiring          # pede confirmação (default Y)
gopherplate wiring --yes    # pula confirmação
```

#### Regenera

- `cmd/api/server.go` — bootstrap.New() com configs
- `internal/infrastructure/web/router/router.go` — `Register<Domain>Routes()` para cada domínio
- `internal/bootstrap/container.go` — Repos, UseCases, Handlers structs com campos por domínio
- `internal/bootstrap/test_helpers.go` — `NewForTest`, `SetupTestRouter`, `SetupTestRouterWithAuth`

#### Detecção inteligente

Inspeciona cada domínio para detectar:

- Quais use cases existem (Create/Get/Update/Delete/List) — alguns domínios podem ter só CRUD parcial (ex: `role` tem só Create/List/Delete)
- Se o handler aceita `*telemetry.Metrics` — gera o construtor com ou sem o param

#### Quando usar

- Após `add domain` (em vez de wiring manual)
- Após `add endpoint` para registrar nova rota/handler
- Após `remove domain` ou `remove endpoint` para limpar wiring órfão

---

### `gopherplate doctor`

Diagnostica o ambiente de desenvolvimento (similar a `flutter doctor`).

```bash
gopherplate doctor
```

#### Verifica

- **Tools**: Go, Docker, golangci-lint, swag, goose, air, k6, kind, kubectl
- **Projeto**: `go.mod` existe (estamos em um projeto Go válido)
- **Infraestrutura**: containers postgres e redis rodando (via `docker ps`)

#### Output

```text
gopherplate doctor

  [OK] Go - go version go1.25.9 darwin/amd64
  [OK] Docker - running
  [OK] golangci-lint - golangci-lint has version 2.11.4 ...
  [!!] swag - not installed (run: go install github.com/swaggo/swag/cmd/swag@latest)
  ...

Project:
  [OK] go.mod found
  Docker containers:
    [OK] postgres running
    [--] redis not running
```

Para tools ausentes, exibe instrução de instalação.

---

### `gopherplate version`

Exibe a versão instalada do CLI.

```bash
gopherplate version
# gopherplate dev
```

---

## Flags (modo não-interativo)

Para uso em CI/CD ou scripts, todas as opções podem ser passadas como flags, eliminando os prompts interativos. Use `-y` para aceitar os defaults sem prompts.

### Referência de flags para `gopherplate new`

| Flag | Tipo | Default | Descrição |
|------|------|---------|-----------|
| `--module` | string | — | Go module path (ex: `github.com/org/svc`) |
| `--db` | string | `postgres` | Driver de banco: `postgres`, `mysql`, `sqlite3`, `other` |
| `--template` | string | `.` | Path do diretório raiz do template |
| `--no-redis` | bool | `false` | Desabilita cache Redis e pacotes relacionados |
| `--no-auth` | bool | `false` | Desabilita Service Key Auth |
| `--no-idempotency` | bool | `false` | Desabilita middleware de idempotência |
| `--no-examples` | bool | `false` | Remove os domínios de exemplo (`user` e `role`) |
| `--keep-examples` | bool | `false` | Mantém explicitamente os domínios de exemplo |
| `-y`, `--yes` | bool | `false` | Aceita todos os defaults (modo não-interativo) |

> **Nota sobre exemplos:** No modo interativo, o default é manter os domínios de exemplo (o prompt pergunta "Manter domínios de exemplo? [Y/n]"). No modo `-y`, os defaults também mantêm. Use `--no-examples` para removê-los explicitamente.

### Exemplo em CI/scripting

```bash
# Projeto minimal: sem Redis, sem auth, sem exemplos
gopherplate new my-svc \
  --module github.com/yourorg/my-svc \
  --db postgres \
  --no-redis \
  --no-auth \
  --no-examples \
  -y

# Projeto com todos os defaults (Redis, auth, idempotência, exemplos)
gopherplate new my-svc \
  --module github.com/yourorg/my-svc \
  -y
```

---

## O que cada opção faz

### Banco de dados

| Opção | Driver | Pacote Go | Descrição |
|-------|--------|-----------|-----------|
| **PostgreSQL** | `postgres` | `github.com/lib/pq` | Driver padrão. Migrations via Goose, repositórios com sqlx. |
| **MySQL** | `mysql` | `github.com/go-sql-driver/mysql` | Configurado com `pkg/database.DBCluster`. |
| **SQLite3** | `sqlite` | `modernc.org/sqlite` | Pure Go, sem CGO. Ideal para testes e prototipagem. |
| **Outro** | — | — | Gera o projeto com `pkg/database` configurado mas sem driver específico. Adicione o driver desejado manualmente. |

> **Todos os drivers** usam a abstração `database/sql` via `pkg/database.DBCluster`, que suporta split Writer/Reader. Consulte o guia [Multi-Database](multi-database.md) para detalhes.

### Cache Redis

| Estado | O que inclui | Quando usar |
|--------|-------------|-------------|
| **Habilitado** (padrão) | `pkg/cache/` com implementação Redis, configuração de pool, TTL, health check. Use cases gerados com `.WithCache()` builder. | Serviços com leitura frequente e tolerância a dados levemente desatualizados. |
| **Desabilitado** (`--no-redis`) | Remove `pkg/cache/`, `pkg/idempotency/`, middleware de idempotência e todas as referências ao Redis no `docker-compose.yml` e configuração. | Serviços simples, batch jobs, ou quando o cache é gerenciado externamente. |

### Idempotência

| Estado | O que inclui | Quando usar |
|--------|-------------|-------------|
| **Habilitada** (padrão, requer Redis) | `pkg/idempotency/` com Store Redis, middleware que intercepta requests com `X-Idempotency-Key`. Usa SHA-256 fingerprint + lock/unlock. | Endpoints de escrita (POST, PUT) onde retry seguro é necessário. |
| **Desabilitada** (`--no-idempotency`) | Remove `pkg/idempotency/` e o middleware de idempotência. Redis continua disponível para cache. | Quando idempotência é tratada pelo API Gateway ou não é necessária. |

### Service Key Auth

| Estado | O que inclui | Quando usar |
|--------|-------------|-------------|
| **Habilitada** (padrão) | Middleware que valida `X-Service-Name` + `X-Service-Key` headers. Configuração via env vars `SERVICE_KEYS`. | Comunicação service-to-service em ambientes sem API Gateway com auth. |
| **Desabilitada** (`--no-auth`) | Remove o middleware de Service Key e as configurações relacionadas. | Quando a autenticação é feita pelo API Gateway, ou em serviços internos sem exposição externa. |

### Domínios de exemplo

| Estado | O que inclui | Quando usar |
|--------|-------------|-------------|
| **Mantidos** (padrão) | Domínios `user` (CRUD completo com cache, singleflight, idempotência) e `role` (exemplo simples de multi-domain DI). Incluem testes unitários e E2E. | Primeiro contato com o template. Use como referência para entender os padrões. |
| **Removidos** (`--no-examples`) | Remove `internal/domain/user/`, `internal/domain/role/`, use cases, handlers, routers, repositories e migrations dos domínios de exemplo. | Projetos reais. Crie seus próprios domínios com `gopherplate add domain`. |

---

## Customização dos templates

Os templates usados pelo CLI estão embarcados no binário via Go `embed.FS`. Isso significa que o CLI funciona como um único executável, sem dependências externas de arquivos.

### Estrutura dos templates

```text
cmd/cli/
├── main.go                      # Entrypoint do CLI
├── commands/                    # Cobra commands (new, add domain, version)
├── scaffold/                    # Engine de scaffold (config, helpers, renderer, rewriter, remover, wiring)
└── templates/
    ├── boilerplate/             # Lógica de copy + transform para `gopherplate new`
    │   ├── copy.go              # Copia o projeto excluindo paths irrelevantes
    │   ├── snapshot.go          # Lista de exclusões (ExcludePaths)
    │   ├── servicename.go       # Substituição do nome do serviço em configs
    │   └── dbdriver.go          # Troca de driver de banco nos imports
    └── domain/                  # Templates .tmpl para `gopherplate add domain`
        ├── entity.go.tmpl
        ├── errors.go.tmpl
        ├── create_usecase.go.tmpl
        ├── repository_postgres.go.tmpl
        ├── handler.go.tmpl
        ├── migration.sql.tmpl
        └── ...                  # (18 templates no total)
```

> **Nota sobre `gopherplate new`:** O comando não usa templates `.tmpl` para o projeto inteiro. Ele copia a árvore real do template, depois aplica transformações: reescrita de module path, substituição do nome do serviço, troca de driver DB, remoção de features desabilitadas, e regeneração do wiring (`server.go`/`router.go`). Isso garante que o projeto gerado sempre reflete a versão mais atual do template.

### Como customizar

1. **Fork** o repositório do boilerplate
2. **Edite** os templates em `cmd/cli/templates/`
3. **Rebuild** o CLI:

```bash
go build -o boilerplate ./cmd/cli/
```

1. **Instale** localmente:

```bash
go install ./cmd/cli/
```

### Engine de scaffold

A lógica de geração de código está em `cmd/cli/scaffold/`. Para customizações avançadas -- como adicionar novos prompts, alterar a lógica de remoção condicional de código, ou integrar novos protocolos -- este é o ponto de extensão.

---

## Em breve (Roadmap)

Duas opções aparecem nos prompts como desabilitadas, sinalizando o roadmap do template:

### gRPC

Atualmente o único protocolo disponível é **HTTP/REST (Gin)**. O suporte a gRPC adicionará:

- Definição de `.proto` files com protobuf
- Servidor gRPC com interceptors (logging, metrics, tracing)
- Handlers gRPC como alternativa aos HTTP handlers
- Opção de rodar ambos os protocolos simultaneamente (gRPC + HTTP gateway)

### Uber Fx

Atualmente a única estratégia de DI é **Manual** (wiring em `server.go:buildDependencies()`). O suporte a Uber Fx substituirá o wiring manual por:

- `fx.Module` para cada domínio (providers agrupados)
- `fx.Lifecycle` para gerenciamento de ciclo de vida (graceful shutdown)
- Autowiring automático via tipos de interface
- Redução significativa de código boilerplate em `server.go`

> Para entender como Uber Fx funciona com este projeto, consulte o guia [Uber Fx para Injeção de Dependência](fx-dependency-injection.md).

---

## Troubleshooting

### `command not found: boilerplate`

O binário do Go não está no `$PATH`. Verifique:

```bash
# Onde o Go instala binários
go env GOBIN
go env GOPATH

# Adicione ao seu ~/.zshrc ou ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

### `go mod tidy` falha após gerar o projeto

- Verifique se o module path é válido e acessível
- Confirme que você tem acesso à rede (para baixar dependências)
- Para módulos privados (GitHub), configure `GOPRIVATE`:

```bash
export GOPRIVATE=github.com/yourorg/*
```

### `permission denied` ao criar o projeto

O CLI precisa de permissão de escrita no diretório atual:

```bash
ls -la .
# Verifique se o usuário tem permissão de escrita
```

### `domain already exists` ao usar `add domain`

O CLI não sobrescreve domínios existentes para evitar perda de código. Se você precisa recriá-lo:

1. Remova manualmente os diretórios do domínio (`domain/`, `usecases/`, `infrastructure/` do domínio)
2. Execute `gopherplate add domain` novamente

---

## FAQ

### Posso usar em projetos existentes?

O comando `gopherplate add domain` funciona em projetos existentes que seguem a estrutura deste template. Já o comando `gopherplate new` cria um projeto do zero -- não é indicado para projetos já iniciados.

### Como atualizo o CLI?

```bash
go install github.com/jrmarcello/gopherplate/cmd/cli@latest
```

### Funciona no Windows?

Sim. O CLI é escrito em Go, que compila nativamente para Windows, macOS e Linux. Os templates gerados também são compatíveis com todos os sistemas operacionais.

### Posso adicionar meus próprios templates?

Sim, via fork. Faça fork do repositório, edite os templates em `cmd/cli/templates/`, e rebuilde o binário. Veja a seção [Customização dos templates](#customização-dos-templates).

### O CLI precisa de conexão com internet?

Não para gerar o projeto. Os templates estão embarcados no binário. Porém, após a geração, `go mod tidy` e `make setup` precisam de internet para baixar dependências.

### Posso gerar um projeto sem nenhuma feature opcional?

Sim. O modo mais enxuto possível:

```bash
gopherplate new minimal-svc \
  --module github.com/yourorg/minimal-svc \
  --db postgres \
  --no-redis \
  --no-auth \
  --no-examples \
  -y
```

Isso gera um projeto apenas com Clean Architecture, PostgreSQL e OpenTelemetry -- sem cache, idempotência, autenticação ou domínios de exemplo.

---

## Referências

- [Clean Architecture - Guia de Arquitetura](architecture.md)
- [Cache Strategy - Guia de Cache](cache.md)
- [Multi-Database - Guia de Banco de Dados](multi-database.md)
- [Uber Fx - Guia de DI](fx-dependency-injection.md)
- [Go embed package](https://pkg.go.dev/embed)
