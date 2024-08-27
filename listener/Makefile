# Image URL to use all building/pushing image targets
APP_NAME = sample-listener
IMG_REPO := $(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)
IMG_NAME := $(IMG_REPO)/$(APP_NAME)
IMG := $(IMG_NAME):$(DOCKER_TAG)

## Tool Binaries
GOLANG_CI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
GOLANG_CI_LINT_VERSION ?= v1.60.3

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ Development

.PHONY: resolve
resolve: ## Run go mod tidy against code.
	go mod tidy

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

##@ Testing

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -v --race -coverprofile cover.out

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: lint
lint: ## Run golangci-lint against code.
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANG_CI_LINT_VERSION)
	$(LOCALBIN)/golangci-lint run -v -c .golangci.yaml

.PHONY: docker-build
docker-build: ## Build docker image for sample-listener.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push docker image for sample-listener.
	docker push $(IMG)
