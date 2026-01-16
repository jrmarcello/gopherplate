# Go Microservice Boilerplate

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Architecture](https://img.shields.io/badge/Architecture-Clean-blueviolet)](docs/adr/clean-architecture.md)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](docker/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?logo=kubernetes)](deploy/)

Template production-ready para microserviços Go com Clean Architecture, cache Redis, observabilidade e deploy Kubernetes.

---

## ✨ O que está incluído

| Feature | Tecnologia | Descrição |
| ------- | ---------- | --------- |
| **Arquitetura** | Clean Architecture | Separação de camadas, DI, testabilidade |
| **API** | Gin + Swagger | REST API documentada |
| **Banco** | PostgreSQL + sqlx | Migrations com Goose |
| **Cache** | Redis (opcional) | Cache transparente com invalidação |
| **Observabilidade** | OpenTelemetry | Traces, métricas, logs estruturados |
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

O template vem com um domínio genérico `entity`. Substitua por seu domínio:

```text
internal/domain/entity/     → internal/domain/user/
internal/usecases/entity/   → internal/usecases/user/
```

### 4. Setup e run

```bash
make setup    # Instala tools + sobe Docker + roda migrations
make dev      # Hot reload
```

---

## 📁 Estrutura

```text
├── cmd/api/              # Entrypoint
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
REDIS_ENABLED=true
SERVICE_KEYS=myservice:sk_myservice_abc123  # opcional, vazio = dev mode
```

Ver: [docs/adr/config-strategy.md](docs/adr/config-strategy.md)

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
|------|----------|
| `/health`, `/ready` | Pública |
| `/swagger/*` | Pública |
| `/entities/*` | Protegida |

Ver: [docs/adr/service-key-auth.md](docs/adr/service-key-auth.md)

---

## 🛠️ Comandos

```bash
make help           # Lista todos os comandos

# Desenvolvimento
make setup          # Setup completo
make dev            # Hot reload
make lint           # Linters

# Testes
make test           # Todos
make test-e2e       # E2E com TestContainers
make test-coverage  # Relatório HTML

# Deploy
make docker-up      # Sobe infra local
make kind-deploy    # Kubernetes local (Kind)
```

---

## 📚 Documentação

### Decisões Arquiteturais (ADRs)

| ADR | Sobre |
| --- | ----- |
| [clean-architecture.md](docs/adr/clean-architecture.md) | Estrutura de camadas e DI |
| [config-strategy.md](docs/adr/config-strategy.md) | godotenv + .env + Kubernetes |
| [error-handling.md](docs/adr/error-handling.md) | Erros em camadas |
| [ulid.md](docs/adr/ulid.md) | Por que ULID ao invés de UUID |

### Guias

| Guia | Sobre |
| ---- | ----- |
| [architecture.md](docs/guides/architecture.md) | Diagramas e visão geral |
| [kubernetes.md](docs/guides/kubernetes.md) | Deploy e operação |

### Para Agentes de IA

Ver [AGENTS.md](AGENTS.md) para diretrizes de código e arquitetura.

---

## 🔧 Customização

### Adicionar novo domínio

1. Crie a entidade em `internal/domain/<nome>/`
2. Crie os use cases em `internal/usecases/<nome>/`
3. Crie o handler em `internal/infrastructure/web/handler/`
4. Registre as rotas no router
5. Crie a migration em `internal/infrastructure/db/postgres/migration/`

### Adicionar novo ambiente K8s

1. Copie `deploy/overlays/dev-local/` para novo overlay
2. Ajuste `configmap.yaml` com as variáveis do ambiente
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
