BINARY_NAME=DBMcp.exe
BUILD_DIR=bin

.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
