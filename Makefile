.PHONY: build clean weaver shuttle

BUILD_DIR := build

build: weaver shuttle

weaver:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/weaver ./cmd/weaver

shuttle:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/shuttle ./cmd/shuttle

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -v ./...
