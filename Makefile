.PHONY: build build-mcp run test clean docker validate-api

BINARY=memtrace
MCP_BINARY=memtrace-mcp

build:
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/memtrace/

build-mcp:
	CGO_ENABLED=0 go build -o $(MCP_BINARY) ./cmd/mcp/

run: build
	./$(BINARY)

test:
	go test ./internal/... ./pkg/... -v

clean:
	rm -f $(BINARY) $(MCP_BINARY)
	rm -rf data/

docker:
	docker build -t memtrace .

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet
	go build ./cmd/... ./internal/... ./pkg/...

validate-api:
	npx @redocly/cli lint docs/openapi.yaml
