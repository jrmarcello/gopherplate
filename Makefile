# ============================================
# CONFIGURAÇÃO DO PROJETO
# ============================================
# Customize estas variáveis para seu projeto

APP_NAME := go-boilerplate
IMAGE_NAME := $(APP_NAME)-api
DB_NAME := entities

# ============================================
# VARIÁVEIS INTERNAS
# ============================================

GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(shell go env GOPATH)/bin
endif

# Fallback defaults (match config.go defaults for local dev)
DB_DSN ?= postgres://user:password@localhost:5432/$(DB_NAME)?sslmode=disable
MIGRATIONS_DIR := internal/infrastructure/db/postgres/migration

# Docker env file (optional)
ENV_FILE := $(shell test -f .env && echo "--env-file .env" || echo "")

# Declara todos os targets que não são arquivos
.PHONY: help setup tools dev run build clean lint lint-full security \
        test test-unit test-e2e test-coverage \
        load-smoke load-test load-stress load-spike load-clean \
        docker-up docker-down docker-build \
        observability-up observability-down observability-logs \
        kind-up kind-down kind-deploy kind-logs kind-migrate \
        migrate-up migrate-down migrate-status migrate-reset migrate-redo migrate-create \
        swagger

# Target padrão
.DEFAULT_GOAL := help

# ============================================
# AJUDA
# ============================================

help: ## Exibe esta mensagem de ajuda
	@echo "$(APP_NAME) - Comandos disponíveis:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ============================================
# SETUP
# ============================================

setup: tools ## 🚀 Setup completo: tools + hooks + docker + migrations
	@echo ""
	@echo "🔧 Setting up git hooks..."
	@$(GOBIN)/lefthook install || lefthook install
	@echo ""
	@echo "🐳 Starting Docker containers..."
	@$(MAKE) docker-up
	@echo ""
	@echo "⏳ Waiting for database to be ready..."
	@sleep 5
	@echo ""
	@echo "📊 Running database migrations..."
	@$(MAKE) migrate-up
	@echo ""
	@echo "============================================"
	@echo "✅ Setup complete!"
	@echo "============================================"
	@echo ""
	@echo "Próximos passos:"
	@echo "  make dev      → Servidor com hot reload"
	@echo "  make test     → Roda todos os testes"
	@echo ""

tools: ## 📦 Instala ferramentas de desenvolvimento
	@echo "📦 Installing dev tools..."
	@go install github.com/air-verse/air@latest
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@go install github.com/evilmartians/lefthook@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "✅ Tools installed in $(GOBIN)"

# ============================================
# DESENVOLVIMENTO
# ============================================

dev: ## 🔥 Inicia servidor com hot reload (air)
	@$(GOBIN)/air || air

run: ## ▶️  Inicia servidor sem hot reload
	go run cmd/api/main.go

lint: ## 🔍 Roda linters básicos (vet + gofmt)
	@go vet ./...
	@gofmt -w .

lint-full: ## 🔍 Roda golangci-lint com todas as verificações
	@golangci-lint run ./...

security: ## 🔒 Roda análise de segurança (gosec)
	@golangci-lint run --enable-only gosec ./...

swagger: ## 📚 Regenera documentação Swagger
	@swag init -g cmd/api/main.go -o docs
	@echo "✅ Swagger docs generated in docs/"

# ============================================
# TESTES
# ============================================

test: ## 🧪 Roda todos os testes
	go test ./... -v

test-unit: ## 🧪 Roda apenas testes unitários
	go test ./internal/... -v

test-e2e: ## 🧪 Roda testes e2e (requer Docker)
	go test ./tests/e2e/... -v -count=1

test-coverage: ## 📊 Gera relatório de cobertura
	@mkdir -p tests/coverage
	go test ./... -coverprofile=tests/coverage/coverage.out
	go tool cover -html=tests/coverage/coverage.out -o tests/coverage/coverage.html
	@echo "✅ Coverage report: tests/coverage/coverage.html"

# ============================================
# BUILD
# ============================================

build: ## 🔨 Compila binário para bin/
	@mkdir -p bin
	go build -o bin/api ./cmd/api
	@echo "✅ Binary: bin/api"

clean: ## 🧹 Remove arquivos gerados
	rm -rf bin/ tests/coverage/ tmp/
	@echo "✅ Cleaned"

# ============================================
# DOCKER
# ============================================

docker-up: ## 🐳 Sobe containers Docker (Postgres + Redis)
	docker compose $(ENV_FILE) -f docker/docker-compose.yml up -d

docker-down: ## 🐳 Para containers Docker
	docker compose $(ENV_FILE) -f docker/docker-compose.yml down

docker-build: ## 🐳 Cria a imagem de produção
	docker build -f docker/Dockerfile -t $(IMAGE_NAME) .

# ============================================
# OBSERVABILIDADE (OpenTelemetry)
# ============================================

observability-up: ## 📈 Sobe stack de observabilidade
	docker compose -f docker/observability/docker-compose.yml up -d
	@echo "🔍 Aguarde ~30s para Elasticsearch iniciar..."
	@echo "📊 Kibana: http://localhost:5601"
	@echo "🔌 OTel Collector: localhost:4317 (gRPC)"

observability-down: ## 📈 Para stack de observabilidade
	docker compose -f docker/observability/docker-compose.yml down

observability-logs: ## 📈 Mostra logs do OTel Collector
	docker compose -f docker/observability/docker-compose.yml logs -f otel-collector

# ============================================
# KIND (Kubernetes Local)
# ============================================

KIND_CLUSTER := $(APP_NAME)-dev
KIND_NAMESPACE := $(APP_NAME)-dev
KIND_CONFIGMAP := deploy/overlays/develop/configmap.yaml
KIND_DB_PORT := 5433

kind-up: ## ☸️ Cria cluster Kind com NGINX Ingress
	@if ! kind get clusters | grep -q $(KIND_CLUSTER); then \
		echo "📦 Criando cluster kind..."; \
		kind create cluster --name $(KIND_CLUSTER) --config deploy/overlays/develop/kind-config.yaml; \
		echo "🌐 Instalando NGINX Ingress..."; \
		kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml; \
		kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s || true; \
	else \
		echo "✅ Cluster $(KIND_CLUSTER) já existe"; \
	fi
	@kubectl create namespace $(KIND_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@echo "🐘 Deploying PostgreSQL..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/kind-postgres.yaml

kind-down: ## ☸️ Remove cluster Kind
	kind delete cluster --name $(KIND_CLUSTER)

kind-deploy: docker-build ## ☸️ Build e deploy no Kind (simula ArgoCD PreSync)
	@echo "📤 Loading image into kind..."
	@docker tag $(IMAGE_NAME):latest $(APP_NAME):dev
	@kind load docker-image $(APP_NAME):dev --name $(KIND_CLUSTER)
	@echo ""
	@echo "⏳ Waiting for PostgreSQL..."
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=ready pod --selector=app=postgres --timeout=60s
	@echo ""
	@echo "🔄 Running migration Job (simulating ArgoCD PreSync)..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/serviceaccount.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/configmap.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/secret.yaml
	@kubectl delete job $(APP_NAME)-migrate -n $(KIND_NAMESPACE) --ignore-not-found
	@cat deploy/base/migration-job.yaml | sed 's|go-boilerplate:latest|go-boilerplate:dev|g' | kubectl apply -n $(KIND_NAMESPACE) -f -
	@echo "⏳ Waiting for migration Job to complete..."
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=complete job/$(APP_NAME)-migrate --timeout=120s
	@echo "✅ Migrations completed!"
	@echo ""
	@echo "☸️ Deploying application..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/deployment.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/service.yaml
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=ready pod --selector=app=$(APP_NAME) --timeout=120s || true
	@echo ""
	@echo "✅ Deploy completo!"
	@echo "📍 http://$(DB_NAME).localhost/health"

kind-migrate: ## ☸️ Roda migrations no PostgreSQL do Kind
	@echo "📊 Rodando migrations via port-forward..."
	@kubectl port-forward -n $(KIND_NAMESPACE) svc/postgres-service $(KIND_DB_PORT):5432 &
	@sleep 3
	@goose -dir $(MIGRATIONS_DIR) postgres "$$(grep 'DB_DSN:' $(KIND_CONFIGMAP) | sed 's/.*DB_DSN: *\"//;s/\".*//;s/postgres-service:5432/localhost:$(KIND_DB_PORT)/')" up || true
	@pkill -f "port-forward.*$(KIND_DB_PORT)" || true

kind-setup: kind-up kind-deploy ## ☸️ Setup completo: cluster + postgres + migrations + deploy
	@echo ""
	@echo "🎉 Kind setup completo!"
	@echo "📍 http://$(DB_NAME).localhost/health"
	@echo ""
	@echo "Comandos úteis:"
	@echo "  make kind-logs   → Ver logs da aplicação"
	@echo "  make kind-down   → Remover cluster"

kind-logs: ## ☸️ Mostra logs do serviço no Kind
	kubectl logs -n $(KIND_NAMESPACE) -l app=$(APP_NAME) -f

# ============================================
# MIGRAÇÕES
# ============================================

migrate-up: ## 📊 Roda migrações do banco
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

migrate-down: ## 📊 Reverte última migração
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down

migrate-status: ## 📊 Mostra status das migrações
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" status || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" status

migrate-reset: ## 📊 Reverte todas as migrações (CUIDADO!)
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" reset || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" reset

migrate-redo: ## 📊 Reverte e reaplica última migração
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" redo || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" redo

migrate-create: ## 📊 Cria nova migração (ex: make migrate-create NAME=add_users)
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) create $(NAME) sql || \
		goose -dir $(MIGRATIONS_DIR) create $(NAME) sql

# ============================================
# LOAD TESTING (k6)
# ============================================
# Requer k6: brew install k6

load-smoke: ## 🔥 Smoke test - validação básica
	k6 run --env SCENARIO=smoke tests/load/scenarios.js

load-test: ## 🔥 Load test - carga progressiva
	k6 run --env SCENARIO=load tests/load/scenarios.js

load-stress: ## 🔥 Stress test - encontrar limites
	k6 run --env SCENARIO=stress tests/load/scenarios.js

load-spike: ## 🔥 Spike test - pico súbito
	k6 run --env SCENARIO=spike tests/load/scenarios.js

load-clean: ## 🔥 Limpa dados de testes de carga
	@echo "🧹 Limpando dados de load test..."
	@docker exec $$(docker ps --format '{{.Names}}' | grep -E 'db|postgres' | head -1) psql -U user -d $(DB_NAME) -c "DELETE FROM entities WHERE name LIKE 'Load Test%';"
	@echo "✅ Dados de load test removidos"
