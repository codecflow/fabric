.PHONY: all build clean weaver shuttle gauge test proto

# Build all components
all: build

build: proto weaver shuttle gauge

# Generate protobuf files
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/weaver/weaver.proto

# Build individual components
weaver:
	go build -o bin/weaver cmd/weaver/main.go

shuttle:
	go build -o bin/shuttle cmd/shuttle/main.go

gauge:
	go build -o bin/gauge cmd/gauge/main.go

# Test all packages
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f proto/weaver/*.pb.go
	rm -f weaver shuttle gauge

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run weaver in development mode
dev-weaver:
	go run cmd/weaver/main.go

# Run shuttle in development mode
dev-shuttle:
	go run cmd/shuttle/main.go

# Run gauge in development mode
dev-gauge:
	go run cmd/gauge/main.go
