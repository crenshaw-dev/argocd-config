# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTROLLER_GEN version — pin for reproducible codegen
CONTROLLER_TOOLS_VERSION ?= v0.19.0

# Build metadata (overridden by goreleaser / CI)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/crenshaw-dev/argocd-config/cmd/argocd-config/commands.Version=$(VERSION) \
	-X github.com/crenshaw-dev/argocd-config/cmd/argocd-config/commands.Commit=$(COMMIT) \
	-X github.com/crenshaw-dev/argocd-config/cmd/argocd-config/commands.Date=$(DATE)

.PHONY: all
all: generate manifests build test

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy methods.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests.
	$(CONTROLLER_GEN) crd paths="./api/v1alpha1/..." output:crd:artifacts:config=config/crd/bases

.PHONY: build
build: ## Build the argocd-config CLI.
	go build -ldflags "$(LDFLAGS)" -o bin/argocd-config ./cmd/argocd-config

.PHONY: test
test: ## Run unit tests.
	go test -race -cover ./...

.PHONY: cover
cover: ## Run tests with coverage for pkg/ and cmd/
	go test -race -coverprofile=coverage.out -covermode=atomic -coverpkg=./pkg/...,./cmd/... ./pkg/... ./cmd/...
	@echo "=== Package coverage (handwritten) ==="
	@go tool cover -func=coverage.out | grep -E '^(total:|github.com/.*/(pkg|cmd)/)' | grep -v zz_generated || true
	@go tool cover -func=coverage.out | awk '/^total:/ {print}'

.PHONY: cover-html
cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Wrote coverage.html"

.PHONY: fmt
fmt: ## Format Go sources.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

##@ Tooling

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
LOCALBIN ?= $(shell pwd)/bin

.PHONY: localbin
localbin:
	@mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: localbin ## Download controller-gen locally if necessary.
	@test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
