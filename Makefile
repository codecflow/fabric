.PHONY: all build clean fabric worker gateway test deps fmt lint kind-up kind-down kind-status

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

# Kubernetes (kind) cluster management
kind-up:
	@echo "Creating kind cluster 'fabric'..."
	kind create cluster --name fabric --wait 5m
	@echo "Cluster ready!"
	kubectl cluster-info --context kind-fabric

kind-down:
	@echo "Deleting kind cluster 'fabric'..."
	kind delete cluster --name fabric

kind-status:
	@echo "Checking kind cluster status..."
	kubectl get nodes --context kind-fabric || echo "Cluster not running"

.DEFAULT_GOAL := build
