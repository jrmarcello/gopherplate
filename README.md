# MS Boilerplate Go

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://go.dev/)

[![Tests](https://img.shields.io/badge/Tests-Passing-success)](tests/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](docker/Dockerfile)

Boilerplate genérico para microserviços Go com arquitetura Clean, cache Redis e deploy Kubernetes.

## 🚀 Quick Start

```bash
# Clone e setup
git clone <repo> && cd ms-boilerplate-go
make setup

# Desenvolvimento local (Docker)
make docker-up
make dev

# Desenvolvimento local (Kubernetes/Kind)
make kind-up
make kind-deploy
curl http://entities.localhost/health
```

## 📋 Pré-requisitos

- Go 1.24+
- Docker
- Kind (opcional, para Kubernetes local)
- K6 (opcional, para testes de carga)

## 🛠️ Comandos

```bash
make help              # Lista todos os comandos

# Desenvolvimento
make setup             # Setup completo (tools + docker + migrations)
make dev               # Servidor com hot reload
make build             # Compila binário

# Testes
make test              # Todos os testes
make test-unit         # Testes unitários
make test-e2e          # Testes e2e com Postgres + Redis
make test-coverage     # Gera relatório HTML

# Docker
make docker-up         # Sobe Postgres + Redis
make docker-down       # Para containers
make docker-build      # Build da imagem

# Kubernetes (Kind)
make kind-up           # Cria cluster local + Ingress + Postgres + Redis
make kind-deploy       # Build + deploy + migrations
make kind-logs         # Ver logs do serviço
make kind-down         # Remove cluster

# Banco de dados
make migrate-up        # Roda migrations
make migrate-down      # Reverte última migration
make migrate-status    # Status das migrations
```

## 📁 Estrutura

```text
ms-boilerplate-go/
├── cmd/api/                          # Entrypoint (main.go, server.go)
├── config/                           # Configuração (env vars)
├── deploy/                           # Kubernetes manifests
│   ├── base/                         # Manifests base (Kustomize)
│   └── overlays/
│       ├── dev-local/                # Kind (local)
├── docker/                           # Dockerfile, docker-compose
├── docs/                             # Swagger, documentação
├── internal/
│   ├── domain/entity/                # Entidades, Value Objects, Erros
│   ├── infrastructure/
│   │   ├── cache/                    # Redis client
│   │   ├── db/postgres/              # Conexão, migrations, repository
│   │   ├── telemetry/                # OpenTelemetry
│   │   └── web/                      # HTTP Handlers, Middlewares, Router
│   ├── pkg/apperror/                 # Erros de aplicação
│   └── usecases/entity/              # Casos de uso (Create, Get, List, Update, Delete)
└── tests/
    ├── e2e/                          # Testes e2e (TestContainers)
    └── load/                         # Testes de carga (k6)
```

## ⚙️ Configuração

### Docker Compose (`.env`)

Para desenvolvimento local com Docker, crie um arquivo `.env`:

```bash
SERVER_PORT=8080
POSTGRES_USER=user
POSTGRES_PASSWORD=password
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=entities
REDIS_URL=redis://localhost:6379
REDIS_TTL=5m
REDIS_ENABLED=true
OTEL_SERVICE_NAME=entity-service
OTEL_COLLECTOR_URL=
```

### Kubernetes (ConfigMap)

Para Kubernetes (Kind/EKS), as variáveis ficam em:

- **dev-local**: `deploy/overlays/dev-local/configmap.yaml`
- **homologação**: `deploy/overlays/homologacao/configmap.yaml`

## 🔌 API

```http
### Health Check
GET /health

### Readiness (verifica DB)
GET /ready

### Criar Entity
POST /entities
Content-Type: application/json

{
  "name": "Nome da Entity",
  "email": "entity@example.com"
}

### Listar Entities (paginado)
GET /entities?page=1&limit=20

### Buscar por ID
GET /entities/:id

### Atualizar Entity
PUT /entities/:id

### Deletar Entity (soft delete)
DELETE /entities/:id
```

📚 Swagger: `http://localhost:8080/swagger/index.html`

Veja [api.http](api.http) para mais exemplos.

## 🧪 Testes

| Tipo | Comando | Descrição |
|---|---|---|
| Unit | `make test-unit` | Domínio, UseCases |
| E2E | `make test-e2e` | API + Postgres + Redis (TestContainers) |
| Coverage | `make test-coverage` | Gera relatório HTML |

## 🐳 Deploy

### Docker Compose

```bash
docker compose -f docker/docker-compose.yml up -d
```

### Kubernetes (Kind - local)

```bash
# Setup inicial (uma vez)
make kind-up

# Deploy (repetir a cada mudança)
make kind-deploy

# Acessar
curl http://entities.localhost/health
```

### Kubernetes (EKS - produção)

Os manifests estão em `deploy/overlays/homologacao/`. O deploy é feito via Bitbucket Pipelines + ArgoCD/Kustomize.

## 📊 Arquitetura

```text
                    ┌─────────────────┐
                    │    Ingress      │
                    │   (NGINX)       │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   API Service   │
                    │   (Go 1.24)     │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼────────┐ ┌───▼───┐ ┌───────▼───────┐
     │   PostgreSQL    │ │ Redis │ │ OTel Collector│
     │   (Dados)       │ │(Cache)│ │ (Telemetria)  │
     └─────────────────┘ └───────┘ └───────────────┘
```
