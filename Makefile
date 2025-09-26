# DB MCP Server Makefile

# Variables
BINARY_NAME=DBMcp.exe
BUILD_DIR=bin
GO_FILES=$(shell find . -name '*.go' -type f)
DB_URL=postgresql://postgres:postgres@localhost:5432/woco360?sslmode=disable

# Default target
.PHONY: all
all: build

# Build the server
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

# Run the server with stdio transport
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME) with stdio transport..."
	./$(BUILD_DIR)/$(BINARY_NAME) stdio --conn-string $(DB_URL)

# Run with connection string
.PHONY: run-db
run-db: build
	@echo "Running $(BINARY_NAME) with database connection..."
	./$(BUILD_DIR)/$(BINARY_NAME) stdio --conn-string "$(DB_CONN_STRING)"

# Run in read-only mode
.PHONY: run-readonly
run-readonly: build
	@echo "Running $(BINARY_NAME) in read-only mode..."
	./$(BUILD_DIR)/$(BINARY_NAME) stdio --conn-string "$(DB_CONN_STRING)" --read-only

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Linting code..."
	golangci-lint run

# Development setup
.PHONY: dev-setup
dev-setup: deps
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the server binary"
	@echo "  run           - Build and run the server with stdio transport"
	@echo "  run-db        - Build and run with DB_CONN_STRING environment variable"
	@echo "  run-readonly  - Build and run in read-only mode"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install and tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Lint code (requires golangci-lint)"
	@echo "  dev-setup     - Set up development environment"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  DB_CONN_STRING - Database connection string (required for run-db and run-readonly)"
