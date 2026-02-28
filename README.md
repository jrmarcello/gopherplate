# Go Microservice Boilerplate

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Architecture](https://img.shields.io/badge/Architecture-Clean-blueviolet)](docs/adr/001-clean-architecture.md)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](docker/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?logo=kubernetes)](deploy/)

Template production-ready para microserviços Go com Clean Architecture, cache Redis, observabilidade e deploy Kubernetes.

---

## ✨ O que está incluído

| Feature | Tecnologia | Descrição |
| ------- | ---------- | --------- |
| **Arquitetura** | Clean Architecture | Separação de camadas, DI, testabilidade |
| **API** | Gin + Swagger | REST API documentada, respostas padronizadas via `pkg/httputil` |
| **Banco** | PostgreSQL + sqlx | Migrations com Goose, DB Cluster (Writer/Reader split) |
| **Cache** | Redis (opcional) | Cache transparente com builder pattern (`.WithCache()`) |
| **Observabilidade** | OpenTelemetry | Traces, HTTP metrics com Apdex, DB pool metrics |
| **Erros** | `pkg/apperror` | Erros estruturados com código, mensagem e HTTP status |
| **Pacotes** | `pkg/` | Pacotes reutilizáveis entre serviços |
| **Testes** | TestContainers + k6 | E2E com banco real, load testing |
| **Deploy** | Docker + Kubernetes | Kustomize overlays por ambiente |
| **DX** | Makefile + Air | Hot reload, git hooks, linters |

---

## 🚀 Começando um Novo Projeto

### 1. Clone o template

```bash
git clone <repo> meu-novo-servico
cd meu-novo-servico
rm -rf .git && git init
```

### 2. Renomeie o módulo

```bash
# Substitua em todos os arquivos
find . -type f -name "*.go" -exec sed -i '' 's|bitbucket.org/appmax-space/go-boilerplate|github.com/sua-org/meu-novo-servico|g' {} +
sed -i '' 's|bitbucket.org/appmax-space/go-boilerplate|github.com/sua-org/meu-novo-servico|g' go.mod
```

### 3. Customize o domínio

O template vem com um domínio genérico `entity_example`. Substitua por seu domínio:

```text
internal/domain/entity_example/     → internal/domain/user/
internal/usecases/entity_example/   → internal/usecases/user/
```

### 4. Setup e run

```bash
make setup    # Instala tools + sobe Docker + roda migrations
make dev      # Hot reload
```

---

## 📁 Estrutura

```text
├── cmd/
│   ├── api/              # Entrypoint HTTP server
│   └── migrate/          # Binário para migrations (K8s Job)
├── config/               # Configuração (godotenv)
├── deploy/               # Kubernetes manifests (Kustomize)
│   ├── base/
│   └── overlays/
├── docker/               # Dockerfile + docker-compose
├── docs/
│   ├── adr/              # Decisões arquiteturais
│   └── guides/           # Guias e diagramas
├── internal/
│   ├── domain/           # 🟢 Entidades, VOs, Erros (puro)
│   ├── usecases/         # 🟡 Casos de uso + interfaces
│   └── infrastructure/   # 🔴 DB, Cache, HTTP, Telemetry
├── pkg/                  # 📦 Pacotes reutilizáveis entre serviços
│   ├── apperror/         # Erros estruturados
│   ├── httputil/         # Respostas HTTP padronizadas
│   ├── ctxkeys/          # Chaves tipadas para context
│   ├── logutil/          # Logging estruturado
│   ├── telemetry/        # OpenTelemetry setup + metrics
│   ├── cache/            # Interface de cache + Redis
│   └── database/         # Conexão PostgreSQL (Writer/Reader)
└── tests/
    ├── e2e/              # TestContainers
    └── load/             # k6
```

**Regra de dependência**: `domain` ← `usecases` ← `infrastructure`

---

## ⚙️ Configuração

Hierarquia (maior prioridade primeiro):

1. **Variáveis de Ambiente** - Kubernetes, CI/CD
2. **Arquivo `.env`** - Desenvolvimento local
3. **Defaults no código** - Fallback seguro

```bash
# .env (exemplo)
SERVER_PORT=8080
DB_DSN=postgres://user:password@localhost:5432/mydb?sslmode=disable
DB_READER_DSN=                                # opcional, Reader replica
REDIS_ENABLED=true
SWAGGER_ENABLED=true                          # toggle Swagger UI
SERVICE_KEYS=myservice:sk_myservice_abc123    # opcional, vazio = dev mode
OTEL_ENABLED=false
OTEL_COLLECTOR_URL=localhost:4317
```

Ver: [docs/adr/003-config-strategy.md](docs/adr/003-config-strategy.md)

---

## 🔐 Autenticação

Rotas protegidas requerem headers `X-Service-Name` e `X-Service-Key`:

```bash
curl -X GET http://localhost:8080/entities \
  -H "X-Service-Name: myservice" \
  -H "X-Service-Key: sk_myservice_abc123"
```

**Dev Mode**: Se `SERVICE_KEYS` estiver vazio, todas as requisições são permitidas.

| Rota | Proteção |
| ------ | ---------- |
| `/health`, `/ready` | Pública |
| `/swagger/*` | Pública |
| `/entities/*` | Protegida |

Ver: [docs/adr/005-service-key-auth.md](docs/adr/005-service-key-auth.md)

---

## 🛠️ Comandos

```bash
make help           # Lista todos os comandos

# Desenvolvimento
make setup          # Setup completo
make dev            # Hot reload
make lint           # go vet + gofmt
make lint-full      # golangci-lint (igual CI)
make security       # gosec

# Testes
make test           # Todos (unit + e2e)
make test-unit      # Apenas unit tests (internal/ + pkg/)
make test-e2e       # E2E com TestContainers
make test-coverage  # Relatório HTML

# Deploy
make docker-up      # Sobe infra local
make kind-setup     # Setup completo Kind (cluster + db + migrate + deploy)
make kind-logs      # Ver logs no Kind
```

---

## 📚 Documentação

### Decisões Arquiteturais (ADRs)

| ADR | Sobre |
| --- | ----- |
| [ADR-001: Clean Architecture](docs/adr/001-clean-architecture.md) | Estrutura de camadas e DI |
| [ADR-002: ULID](docs/adr/002-ulid.md) | Por que ULID ao invés de UUID |
| [ADR-003: Config Strategy](docs/adr/003-config-strategy.md) | godotenv + .env + Kubernetes |
| [ADR-004: Error Handling](docs/adr/004-error-handling.md) | Erros em camadas |
| [ADR-005: Service Key Auth](docs/adr/005-service-key-auth.md) | Autenticação via Service Key |
| [ADR-006: Migration Strategy](docs/adr/006-migration-strategy.md) | ArgoCD PreSync + binário separado |
| [ADR-007: Reusable Packages](docs/adr/007-pkg-reusable-packages.md) | Pacotes reutilizáveis em `pkg/` |
| [ADR-008: API Response Format](docs/adr/008-api-response-format.md) | Formato padronizado de resposta HTTP |

### Guias

| Guia | Sobre |
| ---- | ----- |
| [architecture.md](docs/guides/architecture.md) | Diagramas e visão geral |
| [cache.md](docs/guides/cache.md) | Cache com Redis e builder pattern |
| [kubernetes.md](docs/guides/kubernetes.md) | Deploy e operação |

### Para Agentes de IA

Ver [AGENTS.md](AGENTS.md) para diretrizes de código e arquitetura, e [CLAUDE.md](CLAUDE.md) para orientação específica do Claude Code.

---

## 🔧 Customização

### Adicionar novo domínio

1. Crie a entidade em `internal/domain/<nome>/` (entidade, VOs, erros)
2. Crie os use cases em `internal/usecases/<nome>/` (um arquivo por use case)
3. Defina interfaces em `internal/usecases/<nome>/interfaces/`
4. Crie o handler em `internal/infrastructure/web/handler/`
5. Registre as rotas no router
6. Crie o repositório em `internal/infrastructure/db/postgres/repository/`
7. Crie a migration em `internal/infrastructure/db/postgres/migration/`
8. Wire tudo em `cmd/api/server.go:buildDependencies()`

### Adicionar novo ambiente K8s

1. Copie `deploy/overlays/develop/` para novo overlay
2. Ajuste `configmap.yaml` e `secret.yaml` com as variáveis do ambiente
3. Ajuste `kustomization.yaml` se necessário

---

## 📊 Arquitetura

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
