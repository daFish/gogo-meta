VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
EXTRA_LDFLAGS := -X github.com/daFish/gogo-meta/internal/version.Version=$(VERSION) -X github.com/daFish/gogo-meta/internal/version.BuildDate=$(BUILD_DATE) -X github.com/daFish/gogo-meta/internal/version.GitCommit=$(GIT_COMMIT)

.PHONY: help build docker fmt lint test test-coverage clean all

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the gogo binary
	CGO_ENABLED=0 go build -ldflags '-s -w $(EXTRA_LDFLAGS)' -trimpath -o dist/gogo ./cmd/gogo

docker: ## Build the gogo container image locally
	docker buildx build --pull --progress=plain \
		-f Dockerfile.local \
		--build-arg VERSION="$(VERSION)" \
		--build-arg BUILD_DATE="$(BUILD_DATE)" \
		--build-arg GIT_COMMIT="$(GIT_COMMIT)" \
		-t ghcr.io/dafish/gogo-meta:latest .

fmt: ## Run go fmt
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run

test: ## Run all tests
	go test -race -v ./...

test-coverage: ## Run tests and generate a coverage report
	mkdir -p coverage
	go test -race -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

clean: ## Remove build artifacts and caches
	rm -rf dist/ coverage/
	golangci-lint cache clean

all: clean lint test-coverage build ## Clean, lint, run tests with coverage, and build
