.PHONY: all build clean weaver shuttle gauge test proto deps dev-weaver dev-shuttle dev-gauge

# Build all components
all: build

build: proto weaver shuttle gauge

# Generate protobuf files
proto:
	protoc --go_out=weaver/internal/grpc --go_opt=paths=source_relative \
		--go-grpc_out=weaver/internal/grpc --go-grpc_opt=paths=source_relative \
		weaver/internal/grpc/weaver.proto

# Build individual components
weaver:
	mkdir -p bin
	go build -o bin/weaver ./weaver

shuttle:
	mkdir -p bin
	go build -o bin/shuttle ./shuttle

gauge:
	mkdir -p bin
	go build -o bin/gauge ./gauge

# Test all packages
test:
	go test weaver/...
	go test shuttle/...
	go test gauge/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f weaver/internal/grpc/*.pb.go

# Install dependencies for all modules
deps:
	go work sync

# Run components in development mode
dev-weaver:
	go run weaver

dev-shuttle:
	go run shuttle

dev-gauge:
	go run gauge

# Development helpers
fmt:
	go fmt weaver/...
	go fmt shuttle/...
	go fmt gauge/...

# Workspace operations
workspace-sync:
	go work sync
