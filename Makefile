.PHONY: help build build-mcp build-all run test clean docker fmt vet lint validate-api release

BINARY=memtrace
MCP_BINARY=memtrace-mcp
VERSION ?= dev
LDFLAGS=-s -w -X main.version=$(VERSION)

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the server binary (CGO, version injected via VERSION=...)
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/memtrace/

build-mcp: ## Build the MCP server binary (no CGO, easy to cross-compile)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(MCP_BINARY) ./cmd/mcp/

build-all: build build-mcp ## Build both binaries

run: build ## Run the server (after building)
	./$(BINARY)

test: ## Run all tests
	go test ./internal/... ./pkg/... -v

clean: ## Remove build artifacts and the local data directory
	rm -f $(BINARY) $(MCP_BINARY)
	rm -rf data/

docker: ## Build Docker image (use VERSION=... to embed a specific version)
	docker build --build-arg VERSION=$(VERSION) -t memtrace:$(VERSION) -t memtrace:latest .

fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Format, vet, and verify the project still builds
	go build ./cmd/... ./internal/... ./pkg/...

validate-api: ## Lint the OpenAPI spec
	npx @redocly/cli lint docs/openapi.yaml

# ----- Release -----
# Usage:  make release VERSION=0.2.0
# Creates and pushes a `release/v<VERSION>` branch which triggers the
# release-build GitHub Actions workflow. The workflow builds Docker images,
# native binaries, .deb and .rpm packages for Linux + macOS, and creates a
# draft GitHub release for review.
release: ## Cut a release: create+push release/v$(VERSION) branch (triggers CI)
	@if [ "$(VERSION)" = "dev" ]; then \
		echo "ERROR: pass VERSION=x.y.z (e.g. make release VERSION=0.2.0)"; exit 1; \
	fi
	@if ! echo "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$$'; then \
		echo "ERROR: VERSION '$(VERSION)' is not semver (X.Y.Z or X.Y.Z-prerelease)"; exit 1; \
	fi
	@echo "==> Cutting release v$(VERSION)"
	@git diff --quiet || (echo "ERROR: working tree has uncommitted changes; commit or stash first" && exit 1)
	git checkout -b release/v$(VERSION)
	git push -u origin release/v$(VERSION)
	@echo
	@echo "Release branch pushed. Watch the workflow at:"
	@echo "  https://github.com/Basekick-Labs/memtrace/actions"
	@echo
	@echo "When the workflow finishes, review and publish the draft release at:"
	@echo "  https://github.com/Basekick-Labs/memtrace/releases"
