# Make target to run both lint targets from ./listener and ./runtime-watcher
.PHONY: lint-all
lint-all: lint-runtime-watcher lint-listener

.PHONY: lint-runtime-watcher
lint-runtime-watcher: ## Run golangci-lint against runtime-watcher code.
	$(MAKE) -C runtime-watcher lint

.PHONY: lint-listener
lint-listener: ## Run golangci-lint against listener code.
	$(MAKE) -C listener lint

.PHONY: bump-go-version
bump-go-version: ## Bump Go version. Usage: make bump-go-version GO_VERSION=1.26.3
	./scripts/bump-go-version.sh $(GO_VERSION)
