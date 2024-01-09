APP_NAME = runtime-watcher-skr
IMG_REPO := $(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)
IMG_NAME := $(IMG_REPO)/$(APP_NAME)
IMG := $(IMG_NAME):$(DOCKER_TAG)
BUILD_VERSION := from_makefile
ENVTEST_K8S_VERSION = 1.24.1
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANG_CI_LINT = $(LOCALBIN)/golangci-lint
GOLANG_CI_LINT_VERSION ?= v1.54.2

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

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy against code.
	go mod tidy

.PHONY: test
test: fmt vet envtest ## Run unit and envtest.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test `go list ./... | grep -v /tests/` -coverprofile cover.out -coverpkg=./...

.PHONY: envtest
envtest: $(ENVTEST) ## Install envtest.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: lint
lint: ## Run golangci-lint against code.
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANG_CI_LINT_VERSION)
	$(LOCALBIN)/golangci-lint run -v -c .golangci.yaml

.PHONY: run
run: fmt vet ## Run runtime-watcher from your host.
	go run ./main.go

.PHONY: build
build: fmt vet ## Build runtime-watcher binary.
	go build -ldflags="-X 'main.buildVersion=${BUILD_VERSION}'" -o bin/webhook main.go

.PHONY: docker-build
docker-build: ## Build docker image for the webhook.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image for the webhook.
	docker push ${IMG}

.PHONY: clean
clean: ## Delete bin folder.
	rm -rf bin

.PHONY: release
release: ## Run the release script.
	./hack/release.sh

.PHONY: ci-release
ci-release: tidy build test release ## Run ci release.
