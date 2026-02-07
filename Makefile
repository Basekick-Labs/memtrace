.PHONY: build run test clean docker

BINARY=memtrace

build:
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/memtrace/

run: build
	./$(BINARY)

test:
	go test ./internal/... ./pkg/... -v

clean:
	rm -f $(BINARY)
	rm -rf data/

docker:
	docker build -t memtrace .

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet
	go build ./cmd/... ./internal/... ./pkg/...
