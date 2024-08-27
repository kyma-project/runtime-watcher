# Mark all the targets in this Makefile as PHONY ones.
# Effectively, all the targets passed to the make during the invocation are
# passed through the MAKECMDGOALS special variable. So, the instruction below
# defines the targets as phony ones.
.PHONY: $(MAKECMDGOALS)

APP_NAME = runtime-watcher-skr
IMG_REPO := $(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)
IMG_NAME := $(IMG_REPO)/$(APP_NAME)
IMG := $(IMG_NAME):$(DOCKER_TAG)
BUILD_VERSION := from_makefile
ENVTEST_K8S_VERSION = 1.24.1
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANG_CI_LINT = $(LOCALBIN)/golangci-lint
GOLANG_CI_LINT_VERSION ?= v1.60.3

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

all: build

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

tidy: ## Run go mod tidy against code.
	go mod tidy

test: fmt vet envtest ## Run unit and envtest.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test `go list ./... | grep -v /tests/` -coverprofile cover.out -coverpkg=./...

envtest: $(ENVTEST) ## Install envtest.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

lint: ## Run golangci-lint against code.
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANG_CI_LINT_VERSION)
	$(LOCALBIN)/golangci-lint run -v -c .golangci.yaml

run: fmt vet ## Run runtime-watcher from your host.
	go run ./main.go

build: fmt vet ## Build runtime-watcher binary.
	go build -ldflags="-X 'main.buildVersion=${BUILD_VERSION}'" -o bin/webhook main.go

docker-build: ## Build docker image for the webhook.
	docker build -t ${IMG} .

docker-push: ## Push docker image for the webhook.
	docker push ${IMG}

clean: ## Delete bin folder.
	rm -rf bin

release: ## Run the release script.
	./hack/release.sh

ci-release: tidy build test release ## Run ci release.
