.PHONY: all build clean weaver shuttle test deps fmt lint docker-weaver docker-shuttle workspace-sync

all: build

build: weaver shuttle

weaver:
	mkdir -p bin
	go build -o bin/weaver ./weaver

shuttle:
	mkdir -p bin
	go build -o bin/shuttle ./shuttle

test:
	go test ./shuttle/...
	go test ./weaver/...

lint:
	golangci-lint run ./shuttle/... ./weaver/... --timeout 5m

fmt:
	go fmt ./shuttle/...
	go fmt ./weaver/...

clean:
	rm -rf bin/

deps:
	go work sync
	go mod tidy -C shuttle
	go mod tidy -C weaver

docker-weaver:
	DOCKER_BUILDKIT=1 docker build -t cf-weaver:dev -f build/weaver/Dockerfile .

docker-shuttle:
	DOCKER_BUILDKIT=1 docker build -t cf-shuttle:dev -f build/shuttle/Dockerfile .
