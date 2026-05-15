# tools/learn — module-local development Makefile.
#
# Targets are designed to be invoked from this directory (or via
# `make -C tools/learn <target>` from the project root). The root Makefile
# exposes `learn-*` aliases that delegate here.

LEARN_BIN ?= bin/learn

.PHONY: build test lint tidy clean help smoke

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compile the learn binary (output: $LEARN_BIN, default bin/learn)
	@mkdir -p $(dir $(LEARN_BIN))
	go build -o $(LEARN_BIN) ./cmd/learn

test: ## Run all tests in this module
	go test ./... -count=1

lint: ## Run golangci-lint on this module
	golangci-lint run ./...

tidy: ## Sync go.mod and go.sum
	go mod tidy

clean: ## Remove build artifacts
	rm -f $(LEARN_BIN)

smoke: build ## Run the end-to-end fixture smoke (TC-E2E-06)
	$(LEARN_BIN) smoke
