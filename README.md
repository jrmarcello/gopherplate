# Go Microservice Boilerplate

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Architecture](https://img.shields.io/badge/Architecture-Clean-blueviolet)](docs/adr/001-clean-architecture.md)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](docker/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?logo=kubernetes)](deploy/)

**Padronização e Developer Experience como padrão.** Template production-ready para microsserviços Go — de zero a produção em minutos, não semanas.

Clone, renomeie `entity_example` para seu domínio, e comece a desenvolver. A infraestrutura já está pronta: PostgreSQL com Writer/Reader split, Redis cache com singleflight, OpenTelemetry, idempotência, autenticação service-to-service, 223 testes (89% de cobertura), CI/CD com notificações Slack, Kubernetes com Kustomize, e observabilidade completa com dashboard e alertas.

DX pensado para produtividade: 40+ comandos make com verificação automática de pré-requisitos, hot reload, Lefthook com 3 camadas de verificação de qualidade (pre-commit, commit-msg, pre-push), e integração nativa com Claude Code — skills, hooks, agents especializados e rules auto-aplicadas que atuam como um code reviewer contínuo enquanto você desenvolve.

O template é **pouco opinativo e fortemente extensível**: serve como base para vários tipos de projeto, e o desenvolvedor tem liberdade total para adicionar as bibliotecas e frameworks que desejar.

```bash
git clone https://bitbucket.org/appmax-space/go-boilerplate my-service
cd my-service && make setup && make dev
```

---

## O que vem incluso

### Código pronto para produção

| Feature | O que faz | Por que importa |
| ------- | --------- | --------------- |
| **CRUD completo** | Create, Get, List, Update, Delete | Endpoint funcional de exemplo para copiar |
| **PostgreSQL** | Writer/Reader split, pool tunado | Escala com read replicas sem mudar código |
| **Redis Cache** | Cache-aside + singleflight + pool config | Performance com proteção contra cache stampede |
| **Idempotência** | Redis-backed, SHA-256 fingerprint, fail-open | Requests duplicados não causam efeitos colaterais |
| **UUID v7** | IDs ordenados por tempo, tipo nativo no Postgres | Performance de índice + unicidade global |
| **OpenTelemetry** | Traces + metrics + logs integrados | Observabilidade completa desde o dia 1 |
| **Service Key Auth** | Autenticação entre serviços via headers | Segurança entre microsserviços |
| **Logging estruturado** | Contexto propagado + mascaramento de dados pessoais (email, CPF, telefone) | Logs rastreáveis e em conformidade com LGPD — dados sensíveis nunca aparecem em plaintext no ELK/Kibana |
| **Health checks** | `/health` + `/ready` com verificação de dependências | Kubernetes liveness/readiness probes prontos |

### Qualidade automatizada

| Feature | O que faz | Quando roda |
| ------- | --------- | ----------- |
| **223 testes** | Unit + sqlmock + E2E com TestContainers | `make test` |
| **89% de cobertura** | Domain, usecases, middleware, pkg — tudo coberto | CI exige 60% mínimo |
| **golangci-lint** | 50+ linters incluindo gosec | Pre-commit + CI |
| **govulncheck** | Varredura de vulnerabilidades em dependências | Pre-push + CI |
| **Lefthook** | 3 camadas: pre-commit (formatação), commit-msg (convenção), pre-push (lint+testes+vuln) | Automático |

### DevOps pronto

| Feature | O que faz | Comando |
| ------- | --------- | ------- |
| **Docker Compose** | DB + Redis + API tudo em Docker | `make run` |
| **Hot Reload** | Air com rebuild automático | `make dev` |
| **Kubernetes** | Kustomize overlays (dev, hml, prd) | `make kind-setup` |
| **CI/CD** | 4 verificações paralelas + notificações Slack | Bitbucket Pipelines |
| **Observabilidade** | ELK 8.13 + OTel + dashboard 20 painéis + 6 alertas | `make observability-up` |
| **Load Tests** | k6 com 4 cenários (smoke, load, stress, spike) | `make load-smoke` |
| **Migrations** | Goose SQL com ArgoCD PreSync | `make migrate-up` |

### DX (Developer Experience)

| Feature | O que faz |
| ------- | --------- |
| **40+ comandos make** | Tudo via Makefile com help categorizado |
| **Verificação de pré-requisitos** | Falta Docker? k6? kind? O Makefile avisa como instalar |
| **Claude Code** | Skills, hooks, agents, rules — IA integrada ao fluxo de trabalho |
| **DevContainer** | Sandbox seguro com firewall default-deny |
| **Conventional Commits** | Formato de commits padronizado e verificado automaticamente |

---

## Números

| Métrica | Valor |
| ------- | ----- |
| Testes passando | **223** |
| Cobertura (código com lógica) | **89%** |
| Arquivos Go | **80** |
| Comandos make | **40+** |
| ADRs documentados | **8** |
| Guias | **6** |
| Regras de alerta | **6** |
| Painéis no dashboard | **20** |
| Vulnerabilidades conhecidas | **0** |

---

## Estrutura do projeto

O código é organizado em **camadas com responsabilidades claras**. O domínio fica no centro, protegido de detalhes de infraestrutura — exatamente o padrão de dependência da Clean Architecture.

```text
               ┌─────────────────────────────┐
               │      Infrastructure         │
               │  (Banco, Cache, HTTP, OTel) │
               │                             │
               │   ┌─────────────────────┐   │
               │   │     Use Cases       │   │
               │   │ (Operações de       │   │
               │   │  negócio, 1 por     │   │
               │   │  arquivo)           │   │
               │   │                     │   │
               │   │   ┌─────────────┐   │   │
               │   │   │   Domain    │   │   │
               │   │   │ (Entidades, │   │   │
               │   │   │  VOs, Erros)│   │   │
               │   │   └─────────────┘   │   │
               │   └─────────────────────┘   │
               └─────────────────────────────┘

Dependências apontam para dentro: Infrastructure → Use Cases → Domain
Domain não conhece nada das camadas externas.
```

### Na prática, no código

```text
├── cmd/
│   ├── api/              # Entrypoint HTTP server
│   └── migrate/          # Binário de migrations (K8s Job)
├── config/               # Configuração (godotenv + env vars)
├── internal/
│   ├── domain/           # Entidades, Value Objects, erros (zero deps externas)
│   ├── usecases/         # Casos de uso + interfaces (1 arquivo por operação)
│   └── infrastructure/   # Banco, cache, HTTP handlers, telemetria
├── pkg/                  # Pacotes reutilizáveis entre serviços
│   ├── apperror/         # Erros estruturados
│   ├── cache/            # Redis + singleflight
│   ├── database/         # PostgreSQL Writer/Reader
│   ├── httputil/         # Respostas padronizadas
│   ├── idempotency/      # Idempotência distribuída
│   ├── logutil/          # Logging + mascaramento de dados pessoais
│   └── telemetry/        # OpenTelemetry setup
├── deploy/               # Kubernetes (Kustomize overlays)
├── docker/               # Dockerfile + docker-compose + observabilidade
├── docs/                 # ADRs + guias
└── tests/                # E2E (TestContainers) + load (k6)
```

**Por que isso importa na prática?**

- **Testabilidade**: use cases testados com mocks simples, sem precisar de banco rodando
- **Onboarding**: dev novo sabe exatamente onde colocar cada tipo de código
- **Extensibilidade**: trocar Postgres por DynamoDB? Só muda a infra, use cases não mudam. Quer adicionar gRPC? Só mais um adapter na infrastructure
- **Trabalho em paralelo**: 5 devs podem trabalhar em features diferentes sem conflito

> **Nota**: a arquitetura em camadas é uma sugestão, não uma imposição. Para serviços mais simples, você pode colapsar as camadas. O valor real está na **padronização entre serviços** e na **liberdade de estender** sem quebrar o que já funciona.

---

## Comparativo: sem template vs com template

| Tarefa | Sem template | Com Go Boilerplate |
| ------ | ------------ | ------------------ |
| Setup do projeto | 1-2 dias | `make setup` (2 min) |
| Primeiro endpoint | 1 dia | Já vem pronto (CRUD completo) |
| CI/CD | 1 semana | Já configurado (Bitbucket Pipelines) |
| Kubernetes | 1-2 semanas | `make kind-setup` (5 min) |
| Observabilidade | "a gente vê depois" | `make observability-setup` (1 min) |
| Testes | "a gente escreve depois" | 223 testes de exemplo |
| **Padronização** | **Serviços diferentes** | **Mesmo DX e padrão de qualidade em todos** |

---

## Quick Start

### 1. Clone e renomeie

```bash
git clone https://bitbucket.org/appmax-space/go-boilerplate my-service
cd my-service
rm -rf .git && git init

# Renomear entity_example para seu domínio (find+replace em todo o projeto)
```

### 2. Configure

```bash
cp .env.example .env
# Editar .env com suas configs
make setup
```

### 3. Desenvolva

```bash
make dev          # Hot reload local (Go + DB + Redis)
make test         # Testes
make lint         # Linters
make run          # Tudo em Docker (sem Go local)
```

### 4. Deploy

```bash
make kind-setup   # Testar localmente no Kubernetes
# Push para develop → CI roda → deploy automático via ArgoCD
```

---

## Comandos

```bash
make help              # Lista todos os comandos com descrições

# Desenvolvimento
make setup             # Setup completo (tools + Docker + migrations)
make dev               # Servidor local com hot reload
make run               # Tudo em Docker (infra + migrations + API)
make run-stop          # Para todos os containers

# Qualidade
make lint              # golangci-lint + gofmt
make vulncheck         # Varredura de vulnerabilidades (govulncheck)
make swagger           # Regenera documentação Swagger

# Testes
make test              # Todos (unit + E2E)
make test-unit         # Apenas unit tests
make test-e2e          # E2E com TestContainers
make test-coverage     # Relatório HTML de cobertura

# Infraestrutura
make docker-up         # Sobe PostgreSQL + Redis
make docker-down       # Para containers
make observability-up  # ELK + OTel Collector
make observability-setup # Dashboard + alertas no Kibana
make kind-setup        # Kubernetes local completo

# Load Tests
make load-smoke        # Validação básica (5 VUs)
make load-test         # Carga progressiva (até 50 VUs)
make load-stress       # Encontrar limites (até 200 VUs)
```

---

## Configuração

Hierarquia (maior prioridade primeiro):

1. **Variáveis de Ambiente** — Kubernetes, CI/CD
2. **Arquivo `.env`** — Desenvolvimento local
3. **Defaults no código** — Fallback seguro

```bash
# Servidor
SERVER_PORT=8080

# Postgres (Writer)
DB_HOST=localhost
DB_PORT=5432
DB_USER=user
DB_PASSWORD=password
DB_NAME=entities

# Redis
REDIS_ENABLED=true
REDIS_URL=redis://localhost:6379

# Swagger (desabilitado por padrão — habilite para desenvolvimento)
SWAGGER_ENABLED=true

# Service Key Auth (vazio = modo desenvolvimento)
SERVICE_KEYS=myservice:sk_myservice_abc123
```

Ver `.env.example` para a lista completa e [ADR-003](docs/adr/003-config-strategy.md) para detalhes.

---

## Sandbox (DevContainer)

O projeto inclui um **DevContainer** pré-configurado que cria um ambiente de desenvolvimento isolado com todas as ferramentas instaladas. Ideal para:

- **Rodar o Claude Code com permissões irrestritas** sem risco para sua máquina — o container tem um firewall que bloqueia todo tráfego de rede exceto os domínios necessários (GitHub, Go modules, Bitbucket, Anthropic)
- **Onboarding instantâneo** — qualquer dev abre o projeto no VS Code e tem Go, linters, CLI tools e extensões prontos, sem instalar nada localmente
- **Ambiente reproduzível** — todos desenvolvem com as mesmas versões de Go, golangci-lint, swag, goose, etc.

### Via VS Code

Abra o projeto no VS Code com a extensão **Dev Containers** instalada. Ele detecta o `.devcontainer/devcontainer.json` automaticamente e oferece "Reopen in Container".

### Via Makefile (sem VS Code)

```bash
make sandbox          # Abre um shell no container com firewall ativo
make sandbox-claude   # Abre o Claude Code direto no container
make sandbox-shell    # Conecta num container já rodando
make sandbox-stop     # Para o container
make sandbox-firewall # Testa se o firewall está funcionando
make sandbox-status   # Mostra status do container e volumes
```

### O que vem instalado no container

- Go 1.25 + todas as dev tools (air, goose, lefthook, golangci-lint, swag, gopls, goimports)
- Node.js 20 + Claude Code
- Docker-in-Docker (para rodar `docker compose` dentro do container)
- zsh com Powerline10k
- git-delta para diffs aprimorados

### Firewall (default-deny)

O container roda com `--cap-add=NET_ADMIN` e um script de firewall (`init-firewall.sh`) que:

1. Bloqueia **todo** tráfego de saída por padrão
2. Permite apenas domínios necessários: Anthropic (Claude), GitHub, Go modules, Bitbucket, Docker Hub, Kibana
3. Permite tráfego local (host network, Docker network)

Isso garante que o Claude Code com `--dangerously-skip-permissions` não consiga acessar serviços externos não autorizados.

---

## Autenticação

Rotas protegidas requerem headers `X-Service-Name` e `X-Service-Key`:

```bash
curl -X GET http://localhost:8080/entities \
  -H "X-Service-Name: myservice" \
  -H "X-Service-Key: sk_myservice_abc123"
```

| Rota | Proteção |
| ---- | -------- |
| `/health`, `/ready` | Pública |
| `/swagger/*` | Pública |
| `/entities/*` | Protegida |

**Comportamento por ambiente:**

| `SERVICE_KEYS_ENABLED` | `SERVICE_KEYS` | Resultado |
| ---------------------- | -------------- | --------- |
| `false` (padrão) | qualquer | Tudo permitido (modo desenvolvimento) |
| `true` | configurado | Valida normalmente |
| `true` | **vazio** | **503 Service Unavailable** (fail-closed — protege HML/PRD) |

---

## Pacotes reutilizáveis (pkg/)

Estes pacotes podem ser importados por **qualquer serviço Go** — não só quem usa o template:

| Pacote | O que faz |
| ------ | --------- |
| `pkg/apperror` | Erros estruturados com código, mensagem e status HTTP |
| `pkg/httputil` | Respostas JSON padronizadas (`SendSuccess`, `SendError`) |
| `pkg/cache` | Interface de cache + Redis + singleflight (proteção contra stampede) |
| `pkg/database` | Conexão PostgreSQL com Writer/Reader cluster |
| `pkg/idempotency` | Idempotência distribuída via Redis (lock/unlock, fingerprint SHA-256) |
| `pkg/logutil` | Logging estruturado com propagação de contexto e mascaramento de dados pessoais (LGPD) |
| `pkg/telemetry` | Setup OTel (traces + HTTP metrics + DB pool metrics) |
| `pkg/health` | Health checker com verificação de dependências e timeouts |

---

## Documentação

### Decisões Arquiteturais (ADRs)

Cada decisão técnica tem um documento explicando o **por quê**:

| ADR | Decisão |
| --- | ------- |
| [001](docs/adr/001-clean-architecture.md) | Organização em camadas com DI manual |
| [002](docs/adr/002-ids.md) | UUID v7 para IDs (não auto-increment, não UUID v4) |
| [003](docs/adr/003-config-strategy.md) | Configuração via env vars (não YAML, não Viper) |
| [004](docs/adr/004-error-handling.md) | Erros de domínio puros (sem HTTP no domain) |
| [005](docs/adr/005-service-key-auth.md) | Service Key para autenticação entre serviços |
| [006](docs/adr/006-migration-strategy.md) | Migrations via ArgoCD PreSync Job |
| [007](docs/adr/007-pkg-reusable-packages.md) | Pacotes reutilizáveis em pkg/ |
| [008](docs/adr/008-api-response-format.md) | Formato padrão de resposta da API |

### Guias

| Guia | Sobre |
| ---- | ----- |
| [architecture.md](docs/guides/architecture.md) | Diagramas e visão geral |
| [cache.md](docs/guides/cache.md) | Cache com Redis, singleflight e pool config |
| [kubernetes.md](docs/guides/kubernetes.md) | Deploy, Kind e operação |
| [fx-dependency-injection.md](docs/guides/fx-dependency-injection.md) | Uber Fx como alternativa ao DI manual |
| [multi-database.md](docs/guides/multi-database.md) | Estratégia para serviços com múltiplos bancos |

### Para Agentes de IA

Ver [AGENTS.md](AGENTS.md) para diretrizes de código e arquitetura, e [CLAUDE.md](CLAUDE.md) para orientação específica do Claude Code.

---

## Roadmap

O template está em evolução contínua. Próximos passos planejados:

- [ ] Outbox pattern para eventos assíncronos (SQS/SNS)
- [ ] Suporte a gRPC como alternativa ao REST
- [ ] Feature flags com LaunchDarkly/Unleash
- [ ] Uber Fx como opção de DI (guia já documentado)
- [ ] CLI para scaffold automático (`boilerplate new my-service`)

Sugestões são bem-vindas via [Issues](https://bitbucket.org/appmax-space/go-boilerplate/issues).

---

## FAQ

**"Qual a filosofia por trás do projeto?"**

Entregar **padronização e DX completa como padrão**. Todo novo microsserviço parte do mesmo ponto: mesma estrutura, mesma qualidade, mesma experiência de desenvolvimento. O objetivo não é impor uma arquitetura, mas garantir que boas práticas venham de fábrica — e que o dev tenha liberdade para evoluir a partir delas.

**"Preciso mesmo de tudo isso para um serviço simples?"**

Não. O template é uma **base extensível**, não um monolito obrigatório. Use o que fizer sentido para seu caso. Cache é opcional (`.WithCache()`), idempotência é opcional (nil-safe), observabilidade é opcional. Comece simples e adicione conforme a necessidade.

**"A organização em camadas não é over-engineering pra Go?"**

O template usa camadas simples com DI manual — sem frameworks, sem reflection, sem mágica. São 3 diretórios (domain, usecases, infrastructure) com regras claras de dependência. O valor real não é a arquitetura em si, mas a **padronização entre serviços**: todo mundo sabe onde encontrar o quê, independente do projeto.

Se for muito pro seu caso, colapse as camadas. A Clean Architecture brilha aqui por um motivo prático: **isola o domínio e as lógicas de negócio** das escolhas de infraestrutura, dando liberdade para o dev trocar qualquer peça sem reescrever a lógica.

**"Por que não usar framework X ou ORM Y?"**

O template é intencionalmente **pouco opinativo nas ferramentas**. Usa Gin (HTTP), sqlx (DB), go-redis (cache) — bibliotecas maduras e amplamente adotadas. Não usa ORMs, DI frameworks, ou geradores de código. Quanto menos mágica, mais fácil de entender e debugar.

Mas essa é uma escolha do template como ponto de partida. Você tem **liberdade total para adicionar** qualquer biblioteca ou framework que desejar — Uber Fx para DI, GORM para ORM, gRPC para comunicação. A arquitetura em camadas facilita isso: adicione um adapter na infraestrutura e o domínio nem percebe.

**"Posso usar só partes do template?"**

Sim. Os pacotes em `pkg/` são independentes. Pode importar `pkg/cache` ou `pkg/apperror` em qualquer projeto Go sem usar o template inteiro.

**"Como atualizo meu serviço quando o template evolui?"**

O template é um ponto de partida, não um fork contínuo. Acompanhe o [CHANGELOG](CHANGELOG.md) e adote as melhorias que fizerem sentido. Para mudanças estruturais, consulte os ADRs para entender o raciocínio por trás de cada decisão.

**"E a performance? Camadas não adicionam overhead?"**

Não. As camadas são uma separação lógica de código, não uma separação runtime. Não há serialização, rede, ou cópia de dados entre camadas — são chamadas de função Go normais. O overhead é zero.

---

## Arquitetura de infraestrutura

```text
                    ┌─────────────────┐
                    │    Ingress      │
                    │   (NGINX)       │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   API Service   │
                    │   (Go + Gin)    │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼────────┐ ┌───▼───┐ ┌───────▼───────┐
     │   PostgreSQL    │ │ Redis │ │ OTel Collector│
     │   (Dados)       │ │(Cache)│ │ (Telemetria)  │
     └─────────────────┘ └───────┘ └───────────────┘
```

---

> **TL;DR**: Clone, renomeie, `make setup`, desenvolva features. A infraestrutura já está resolvida — foque no que importa: seu domínio de negócio.
