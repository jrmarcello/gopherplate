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

# Carrega variáveis do .env (se existir)
-include .env
export

# Fallback defaults (match config.go defaults for local dev)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= user
DB_PASSWORD ?= password
DB_SSLMODE ?= disable
DB_DSN ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
MIGRATIONS_DIR := internal/infrastructure/db/postgres/migration

# Docker Compose
COMPOSE := docker compose -f docker/docker-compose.yml
ENV_FILE := $(shell test -f .env && echo "--env-file .env" || echo "")

# Declara todos os targets que não são arquivos
.PHONY: help setup tools go-tools-check docker-check k6-check kind-check \
        dev run run-stop build build-cli install-cli clean lint security vulncheck swagger \
        test test-unit test-e2e test-coverage \
        load-smoke load-test load-stress load-spike load-kind load-clean \
        docker-up docker-down docker-build \
        observability-up observability-down observability-logs \
        kind-up kind-down kind-deploy kind-logs kind-status kind-migrate kind-setup \
        migrate-up migrate-down migrate-status migrate-reset migrate-redo migrate-create \
        sandbox sandbox-claude sandbox-shell sandbox-stop sandbox-clean sandbox-build sandbox-rebuild \
        sandbox-firewall sandbox-ssh sandbox-status

# Target padrão
.DEFAULT_GOAL := help

# ============================================
# AJUDA
# ============================================

help: ## Exibe esta mensagem de ajuda
	@echo ""
	@echo "\033[1m$(APP_NAME)\033[0m"
	@echo ""
	@echo "\033[1;33m  Setup\033[0m"
	@grep -Eh '^(setup|tools):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Development\033[0m"
	@grep -Eh '^(dev|run|run-stop|build|clean|changelog|release):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  CLI\033[0m"
	@grep -Eh '^(build-cli|install-cli):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Code Quality\033[0m"
	@grep -Eh '^(lint|security|vulncheck|swagger):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Testing\033[0m"
	@grep -Eh '^(test|test-unit|test-e2e|test-coverage):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Docker\033[0m"
	@grep -Eh '^docker-(up|down|build):.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Database Migrations\033[0m"
	@grep -Eh '^migrate-.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Kubernetes (Kind)\033[0m"
	@grep -Eh '^kind-.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Observability (ELK + OTel)\033[0m"
	@grep -Eh '^observability-.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo ""
	@echo "\033[1;33m  Load Testing (k6)\033[0m"
	@grep -Eh '^load-.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "\033[1;33m  Sandbox (DevContainer)\033[0m"
	@grep -Eh '^sandbox.*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "    \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ============================================
# SETUP (ÚNICO COMANDO NECESSÁRIO)
# ============================================

setup: tools docker-up migrate-up ## Setup completo: tools + hooks + docker + migrations
	@echo ""
	@echo "Setting up git hooks..."
	@$(GOBIN)/lefthook install || lefthook install
	@echo ""
	@echo "============================================"
	@echo "Setup complete!"
	@echo "============================================"
	@echo ""
	@echo "Proximos passos:"
	@echo "  make dev      -> Servidor com hot reload"
	@echo "  make test     -> Roda todos os testes"
	@echo ""

# Prerequisite checks (used as dependencies by targets that need external tools)
go-tools-check:
	@command -v $(GOBIN)/air >/dev/null 2>&1 || command -v air >/dev/null 2>&1 || { echo "Dev tools not found. Run: make tools"; exit 1; }

docker-check:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found. Install: https://docs.docker.com/get-docker/"; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "Docker daemon not running. Start Docker Desktop and try again."; exit 1; }

k6-check:
	@command -v k6 >/dev/null 2>&1 || { echo "k6 not found. Install: brew install k6 (macOS) or https://grafana.com/docs/k6/latest/set-up/install-k6/"; exit 1; }

tools: ## Instala ferramentas de desenvolvimento
	@echo "Installing dev tools..."
	@go install github.com/air-verse/air@latest
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@go install github.com/evilmartians/lefthook@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installed in $(GOBIN)"

# ============================================
# DESENVOLVIMENTO
# ============================================

dev: go-tools-check docker-up migrate-up ## Inicia servidor local com hot reload (air)
	@$(GOBIN)/air || air

run: ## Sobe tudo em Docker (infra + migrations + API)
	$(COMPOSE) $(ENV_FILE) --profile api up -d --build

run-stop: ## Para todos os containers (infra + API)
	$(COMPOSE) $(ENV_FILE) --profile api down

build: ## Compila binarios para bin/
	@mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/boilerplate ./cmd/cli
	@echo "Binaries: bin/api, bin/migrate, bin/boilerplate"

build-cli: ## Compila o CLI de scaffold para bin/
	@mkdir -p bin
	go build -o bin/boilerplate ./cmd/cli
	@echo "Binary: bin/boilerplate"

install-cli: ## Instala o CLI de scaffold no GOBIN
	go install ./cmd/cli
	@echo "Installed: $(GOBIN)/boilerplate"

clean: ## Remove arquivos gerados
	rm -rf bin/ tests/coverage/ tests/load/results/
	@echo "Cleaned"

changelog: ## Gera sugestão de changelog a partir dos commits (somente visualização)
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not found. Install: brew install git-cliff"; exit 1; }
	@echo "Gerando changelog sugerido (não sobrescreve CHANGELOG.md)..."
	@git-cliff --output /dev/stdout
	@echo ""
	@echo "Para criar uma release, use: make release VERSION=x.y.z"

release: ## Cria release: tag + CHANGELOG.md automático (uso: make release VERSION=0.7.0)
	@command -v git-cliff >/dev/null 2>&1 || { echo "git-cliff not found. Install: brew install git-cliff"; exit 1; }
	@[ -n "$(VERSION)" ] || { echo "Erro: informe a versão. Uso: make release VERSION=0.7.0"; exit 1; }
	@[ -z "$$(git status --porcelain)" ] || { echo "Erro: working tree com mudanças não commitadas. Faça commit antes."; exit 1; }
	@echo "Criando release v$(VERSION)..."
	@git tag "v$(VERSION)"
	@git-cliff --output CHANGELOG.md
	@git add CHANGELOG.md
	@git commit -m "chore(release): v$(VERSION) [skip ci]"
	@git tag -f "v$(VERSION)"
	@echo ""
	@echo "Release v$(VERSION) criada. Para publicar:"
	@echo "  git push origin main --tags"

# ============================================
# QUALIDADE DE CÓDIGO
# ============================================

lint: go-tools-check ## Roda golangci-lint + gofmt
	@gofmt -w .
	@$(GOBIN)/golangci-lint run ./... || golangci-lint run ./...

security: go-tools-check ## Roda analise de seguranca (gosec via golangci-lint)
	@$(GOBIN)/golangci-lint run --enable-only gosec ./... || golangci-lint run --enable-only gosec ./...

vulncheck: go-tools-check ## Scan de vulnerabilidades em dependencias (govulncheck)
	@$(GOBIN)/govulncheck -show verbose ./... || govulncheck -show verbose ./...

swagger: go-tools-check ## Regenera documentacao Swagger
	@$(GOBIN)/swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal || \
		swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "Swagger docs generated in docs/"

# ============================================
# TESTES
# ============================================

test: ## Roda todos os testes
	go test ./... -v

test-unit: ## Roda apenas testes unitarios
	go test ./pkg/... ./config/... ./internal/... -v

test-e2e: ## Roda testes e2e (requer Docker)
	go test ./tests/e2e/... -v -count=1

test-coverage: ## Gera relatorio de cobertura (exclui bootstrap/wiring)
	@mkdir -p tests/coverage
	go test $$(go list ./internal/... ./pkg/... ./config/... | grep -v -E '(web/handler$$|web/router$$|telemetry$$|db/postgres$$)') -coverprofile=tests/coverage/coverage.out
	@go tool cover -func=tests/coverage/coverage.out | tail -1
	go tool cover -html=tests/coverage/coverage.out -o tests/coverage/coverage.html
	@echo "Coverage report: tests/coverage/coverage.html"

# ============================================
# DOCKER
# ============================================

docker-up: docker-check ## Sobe containers Docker (Postgres + Redis)
	$(COMPOSE) $(ENV_FILE) up -d

docker-down: docker-check ## Para containers Docker
	$(COMPOSE) $(ENV_FILE) down

docker-build: docker-check ## Cria a imagem de producao
	docker build -f docker/Dockerfile -t $(IMAGE_NAME) .

# ============================================
# OBSERVABILIDADE (ELK + OpenTelemetry)
# ============================================

observability-up: docker-up ## Sobe stack de observabilidade (Elasticsearch + Kibana + OTel)
	docker compose -f docker/observability/docker-compose.yml up -d
	@echo "Aguarde ~30s para Elasticsearch iniciar..."
	@echo "Kibana: http://localhost:5601"
	@echo "OTel Collector: localhost:4317 (gRPC)"

observability-down: ## Para stack de observabilidade
	docker compose -f docker/observability/docker-compose.yml down

observability-logs: ## Mostra logs do OTel Collector
	docker compose -f docker/observability/docker-compose.yml logs -f otel-collector

observability-setup: ## Importa dashboard + data views + alertas no Kibana
	@bash docker/observability/scripts/setup_kibana.sh

# ============================================
# KIND (Kubernetes Local)
# ============================================

KIND_CLUSTER := $(APP_NAME)-dev
KIND_NAMESPACE := $(APP_NAME)-dev
KIND_CONFIGMAP := deploy/overlays/develop/configmap.yaml
KIND_DB_PORT := 5433

kind-check:
	@command -v kind >/dev/null 2>&1 || { echo "kind not found. Install: brew install kind (macOS) or go install sigs.k8s.io/kind@latest"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "kubectl not found. Install: brew install kubectl (macOS) or https://kubernetes.io/docs/tasks/tools/"; exit 1; }

kind-up: kind-check ## Cria cluster Kind com NGINX Ingress
	@if ! kind get clusters | grep -q $(KIND_CLUSTER); then \
		echo "Criando cluster kind..."; \
		kind create cluster --name $(KIND_CLUSTER) --config deploy/overlays/develop/kind-config.yaml; \
		echo "Instalando NGINX Ingress..."; \
		kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml; \
		kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s || true; \
	else \
		echo "Cluster $(KIND_CLUSTER) ja existe"; \
	fi
	@kubectl create namespace $(KIND_NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@echo "Deploying PostgreSQL..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/kind-postgres.yaml

kind-down: kind-check ## Remove cluster Kind
	kind delete cluster --name $(KIND_CLUSTER)

kind-deploy: kind-check docker-build ## Build e deploy no Kind (simula ArgoCD PreSync)
	@echo "Loading image into kind..."
	@docker tag $(IMAGE_NAME):latest $(APP_NAME):dev
	@kind load docker-image $(APP_NAME):dev --name $(KIND_CLUSTER)
	@echo ""
	@echo "Waiting for PostgreSQL..."
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=ready pod --selector=app=postgres --timeout=60s
	@echo ""
	@echo "Running migration Job (simulating ArgoCD PreSync)..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/serviceaccount.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/configmap.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/overlays/develop/secret.yaml
	@kubectl delete job $(APP_NAME)-migrate -n $(KIND_NAMESPACE) --ignore-not-found
	@cat deploy/base/migration-job.yaml | sed 's|go-boilerplate:latest|go-boilerplate:dev|g' | kubectl apply -n $(KIND_NAMESPACE) -f -
	@echo "Waiting for migration Job to complete..."
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=complete job/$(APP_NAME)-migrate --timeout=120s
	@echo "Migrations completed!"
	@echo ""
	@echo "Deploying application..."
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/deployment.yaml
	@kubectl apply -n $(KIND_NAMESPACE) -f deploy/base/service.yaml
	@kubectl wait --namespace $(KIND_NAMESPACE) --for=condition=ready pod --selector=app=$(APP_NAME) --timeout=120s || true
	@echo ""
	@echo "Deploy completo!"
	@echo "http://$(DB_NAME).localhost/health"

kind-migrate: kind-check ## Roda migrations no PostgreSQL do Kind
	@echo "Rodando migrations via port-forward..."
	@kubectl port-forward -n $(KIND_NAMESPACE) svc/postgres-service $(KIND_DB_PORT):5432 &
	@sleep 3
	@goose -dir $(MIGRATIONS_DIR) postgres "$$(grep 'DB_DSN:' $(KIND_CONFIGMAP) | sed 's/.*DB_DSN: *\"//;s/\".*//;s/postgres-service:5432/localhost:$(KIND_DB_PORT)/')" up || true
	@pkill -f "port-forward.*$(KIND_DB_PORT)" || true

kind-setup: kind-up kind-deploy ## Setup completo: cluster + postgres + migrations + deploy
	@echo ""
	@echo "Kind setup completo!"
	@echo "http://$(DB_NAME).localhost/health"
	@echo ""
	@echo "Comandos uteis:"
	@echo "  make kind-logs   -> Ver logs da aplicacao"
	@echo "  make kind-status -> Status dos pods/services"
	@echo "  make kind-down   -> Remover cluster"

kind-logs: kind-check ## Mostra logs do servico no Kind
	kubectl logs -n $(KIND_NAMESPACE) -l app=$(APP_NAME) -f

kind-status: kind-check ## Mostra status dos pods/services no Kind
	@echo "Pods:"
	@kubectl get pods -n $(KIND_NAMESPACE) -o wide
	@echo ""
	@echo "Services:"
	@kubectl get svc -n $(KIND_NAMESPACE)
	@echo ""
	@echo "Ingress:"
	@kubectl get ingress -n $(KIND_NAMESPACE)
	@echo ""
	@echo "HPA:"
	@kubectl get hpa -n $(KIND_NAMESPACE)

# ============================================
# MIGRAÇÕES
# ============================================

migrate-up: ## Roda migracoes do banco
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" up

migrate-down: ## Reverte ultima migracao
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" down

migrate-status: ## Mostra status das migracoes
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" status || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" status

migrate-reset: ## Reverte todas as migracoes (CUIDADO!)
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" reset || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" reset

migrate-redo: ## Reverte e reaplica ultima migracao
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" redo || \
		goose -dir $(MIGRATIONS_DIR) postgres "$(DB_DSN)" redo

migrate-create: ## Cria nova migracao (ex: make migrate-create NAME=add_users)
	@$(GOBIN)/goose -dir $(MIGRATIONS_DIR) create $(NAME) sql || \
		goose -dir $(MIGRATIONS_DIR) create $(NAME) sql

# ============================================
# LOAD TESTING (k6)
# ============================================
load-setup: k6-check
	@mkdir -p tests/load/results

LOAD_URL ?= http://localhost:8080

load-smoke: load-setup ## Smoke test (validacao basica)
	k6 run --env SCENARIO=smoke --env BASE_URL=$(LOAD_URL) tests/load/scenarios.js 2>&1 | tee tests/load/results/smoke_$(shell date +%Y%m%d_%H%M%S).log

load-test: load-setup ## Load test (carga progressiva)
	k6 run --env SCENARIO=load --env BASE_URL=$(LOAD_URL) tests/load/scenarios.js 2>&1 | tee tests/load/results/load_$(shell date +%Y%m%d_%H%M%S).log

load-stress: load-setup ## Stress test (encontrar limites)
	k6 run --env SCENARIO=stress --env BASE_URL=$(LOAD_URL) tests/load/scenarios.js 2>&1 | tee tests/load/results/stress_$(shell date +%Y%m%d_%H%M%S).log

load-spike: load-setup ## Spike test (pico subito)
	k6 run --env SCENARIO=spike --env BASE_URL=$(LOAD_URL) tests/load/scenarios.js 2>&1 | tee tests/load/results/spike_$(shell date +%Y%m%d_%H%M%S).log

load-kind: ## Roda smoke test contra o cluster Kind
	@$(MAKE) load-smoke LOAD_URL=http://$(DB_NAME).localhost

load-clean: ## Limpa dados de testes de carga
	@echo "Limpando dados de load test..."
	@docker exec $$(docker ps --format '{{.Names}}' | grep -E 'db|postgres' | head -1) psql -U user -d $(DB_NAME) -c "DELETE FROM entities WHERE name LIKE 'Load Test%';"
	@echo "Dados de load test removidos"

# ============================================
# SANDBOX (Claude Code DevContainer)
# ============================================

SANDBOX_IMAGE     := $(APP_NAME)-sandbox
SANDBOX_CONTAINER := $(APP_NAME)-sandbox
SANDBOX_ROOT      := $(shell pwd)
SANDBOX_PORT      ?= 8081

# SSH agent detection
ifeq ($(shell uname),Darwin)
  SANDBOX_SSH := -v /run/host-services/ssh-auth.sock:/ssh-agent -e SSH_AUTH_SOCK=/ssh-agent
else ifdef SSH_AUTH_SOCK
  SANDBOX_SSH := -v $(SSH_AUTH_SOCK):/ssh-agent -e SSH_AUTH_SOCK=/ssh-agent
else
  SANDBOX_SSH :=
endif

# Git identity from host (passed to container for commits)
SANDBOX_GIT_EMAIL := $(shell git config user.email 2>/dev/null || echo "dev@boilerplate.local")
SANDBOX_GIT_NAME  := $(shell git config user.name 2>/dev/null || echo "Developer")

SANDBOX_RUN_ARGS := -it --rm \
	--name $(SANDBOX_CONTAINER) \
	--cap-add=NET_ADMIN \
	--cap-add=NET_RAW \
	-e NODE_OPTIONS="--max-old-space-size=4096" \
	-e CLAUDE_CONFIG_DIR="/home/node/.claude" \
	-e GOPATH="/home/node/go" \
	-e GIT_AUTHOR_EMAIL="$(SANDBOX_GIT_EMAIL)" \
	-e GIT_AUTHOR_NAME="$(SANDBOX_GIT_NAME)" \
	-e GIT_COMMITTER_EMAIL="$(SANDBOX_GIT_EMAIL)" \
	-e GIT_COMMITTER_NAME="$(SANDBOX_GIT_NAME)" \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v $(APP_NAME)-bashhistory:/commandhistory \
	-v $(APP_NAME)-claude-config:/home/node/.claude \
	-v $(APP_NAME)-gopath:/home/node/go \
	-v "$(SANDBOX_ROOT):/workspace" \
	-p $(SANDBOX_PORT):8080 \
	$(SANDBOX_SSH)

SANDBOX_INIT := sudo /usr/local/bin/init-firewall.sh && (sudo chmod 666 /ssh-agent 2>/dev/null || true) && (sudo chmod 666 /var/run/docker.sock 2>/dev/null || true) && ([ -n "$$GIT_AUTHOR_EMAIL" ] && git config --global user.email "$$GIT_AUTHOR_EMAIL" && git config --global user.name "$$GIT_AUTHOR_NAME" || true)

sandbox-build: ## Build sandbox image
	docker build -t $(SANDBOX_IMAGE) -f .devcontainer/Dockerfile .devcontainer

sandbox-rebuild: ## Rebuild sandbox image (no cache)
	docker build --no-cache -t $(SANDBOX_IMAGE) -f .devcontainer/Dockerfile .devcontainer

sandbox: sandbox-build ## Open sandbox shell (firewall enabled)
	-docker run $(SANDBOX_RUN_ARGS) $(SANDBOX_IMAGE) \
		bash -c "$(SANDBOX_INIT) && exec zsh"

# WARNING: --dangerously-skip-permissions should ONLY be used inside the sandboxed
# container with the firewall active (init-firewall.sh). Never use this flag on a
# host machine or in any environment without network-level isolation.
sandbox-claude: sandbox-build ## Launch Claude in sandbox directly
	-docker run $(SANDBOX_RUN_ARGS) $(SANDBOX_IMAGE) \
		bash -c "$(SANDBOX_INIT) && claude --dangerously-skip-permissions"

sandbox-shell: ## Attach shell to running sandbox
	docker exec -it $(SANDBOX_CONTAINER) zsh

sandbox-stop: ## Stop sandbox container
	docker stop $(SANDBOX_CONTAINER) 2>/dev/null || true

sandbox-clean: ## Remove sandbox container, image and all volumes
	docker stop $(SANDBOX_CONTAINER) 2>/dev/null || true
	docker rm $(SANDBOX_CONTAINER) 2>/dev/null || true
	docker rmi $(SANDBOX_IMAGE) 2>/dev/null || true
	docker volume rm $(APP_NAME)-bashhistory $(APP_NAME)-claude-config $(APP_NAME)-gopath 2>/dev/null || true
	@echo "Sandbox cleaned (container, image, volumes)"

sandbox-firewall: ## Test sandbox firewall rules
	@echo "\033[36m-- Blocked (example.com) --\033[0m"
	@docker exec $(SANDBOX_CONTAINER) curl --connect-timeout 3 https://example.com 2>&1 && \
		echo "\033[31mFAIL\033[0m" || echo "\033[32mPASS\033[0m"
	@echo "\033[36m-- Allowed (api.github.com) --\033[0m"
	@docker exec $(SANDBOX_CONTAINER) curl --connect-timeout 5 -s https://api.github.com/zen && \
		echo "\n\033[32mPASS\033[0m" || echo "\033[31mFAIL\033[0m"
	@echo "\033[36m-- Allowed (proxy.golang.org) --\033[0m"
	@docker exec $(SANDBOX_CONTAINER) curl --connect-timeout 5 -s -o /dev/null -w "%{http_code}" https://proxy.golang.org && \
		echo "\n\033[32mPASS\033[0m" || echo "\033[31mFAIL\033[0m"
	@echo "\033[36m-- Allowed (bitbucket.org) --\033[0m"
	@docker exec $(SANDBOX_CONTAINER) curl --connect-timeout 5 -s -o /dev/null -w "%{http_code}" https://bitbucket.org && \
		echo "\n\033[32mPASS\033[0m" || echo "\033[31mFAIL\033[0m"

sandbox-ssh: ## Verify SSH agent in sandbox
	@docker exec $(SANDBOX_CONTAINER) ssh-add -l 2>/dev/null && \
		echo "\033[32mSSH agent OK\033[0m" || \
		echo "\033[31mSSH agent not available -- run 'ssh-add' on host\033[0m"

sandbox-status: ## Show sandbox container and volumes
	@echo "\033[1m-- Container --\033[0m"
	@docker ps -a --filter name=$(SANDBOX_CONTAINER) --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" 2>/dev/null
	@echo "\033[1m-- Volumes --\033[0m"
	@docker volume ls --filter name=$(APP_NAME) --format "table {{.Name}}\t{{.Driver}}"
