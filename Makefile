# Make target to run both lint targets from ./listener and ./runtime-watcher
FIPS140_MODULE_VERSION := v1.0.0
GO := GOFIPS140=$(FIPS140_MODULE_VERSION) go

.PHONY: lint-all
lint-all: lint-runtime-watcher lint-listener

.PHONY: lint-runtime-watcher
lint-runtime-watcher: ## Run golangci-lint against runtime-watcher code.
	$(MAKE) -C runtime-watcher lint

.PHONY: lint-listener
lint-listener: ## Run golangci-lint against listener code.
	$(MAKE) -C listener lint

.PHONY: bump-go-version
bump-go-version: ## Bump Go version. Usage: make bump-go-version GO_VERSION=1.27.0
	curl -fsSL https://raw.githubusercontent.com/kyma-project/lifecycle-manager/refs/heads/main/scripts/bump-go-version.sh | bash -s $(GO_VERSION)

.PHONY: build-for-codeql
build-for-codeql: ## Compile all production modules for CodeQL analysis (no codegen, no output).
	cd listener && $(GO) build -o /dev/null ./...
	cd runtime-watcher && $(GO) build -o /dev/null ./...
