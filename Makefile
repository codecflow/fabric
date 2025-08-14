.PHONY: all build clean fabric worker gateway test deps fmt lint

all: build

build: fabric worker gateway

fabric:
	@mkdir -p bin
	go build -o bin/fabric ./cmd/fabric

worker:
	@mkdir -p bin
	go build -o bin/worker ./cmd/worker

gateway:
	@mkdir -p bin
	go build -o bin/gateway ./cmd/gateway

test:
	go test ./pkg/...

lint:
	golangci-lint run ./cmd/... ./pkg/... --timeout 5m

fmt:
	go fmt ./cmd/...
	go fmt ./pkg/...

clean:
	rm -rf bin/

deps:
	go mod tidy

.DEFAULT_GOAL := build
