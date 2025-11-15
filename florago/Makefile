# Binary name
BINARY_NAME=florago

# Version information
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X florago/cmd.version=$(VERSION) -X florago/cmd.commit=$(COMMIT) -X florago/cmd.date=$(BUILD_DATE)"

# Build directory
BUILD_DIR=bin

.PHONY: all build clean test run help build-amd64 build-arm64 build-all debug-build debug-server debug-remote

all: build

## build: Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

## build-amd64: Build universal AMD64 binary (Linux/macOS Intel/Windows)
build-amd64:
	@echo "Building $(BINARY_NAME) for AMD64 (universal)..."
	@echo "  - Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-amd64 .
	@echo "✓ Built: $(BUILD_DIR)/$(BINARY_NAME)-amd64 (for Linux/macOS Intel)"

## build-arm64: Build for Apple Silicon (macOS ARM64)
build-arm64:
	@echo "Building $(BINARY_NAME) for Apple Silicon..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 .
	@echo "✓ Built: $(BUILD_DIR)/$(BINARY_NAME)-arm64 (for macOS Apple Silicon)"

## build-all: Build both universal binaries
build-all: build-amd64 build-arm64
	@echo ""
	@echo "Built universal binaries:"
	@echo "  $(BINARY_NAME)-amd64  - For AMD64 (Linux, macOS Intel, Windows via WSL)"
	@echo "  $(BINARY_NAME)-arm64  - For Apple Silicon (macOS M1/M2/M3)"
	@ls -lh $(BUILD_DIR)

## install: Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) .

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean

## test: Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

## run: Build and run the application
run: build
	@$(BUILD_DIR)/$(BINARY_NAME)

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

## debug-build: Build with debug symbols for remote debugging
debug-build:
	@echo "Building with debug symbols..."
	@go build -gcflags="all=-N -l" $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-debug .

## debug-server: Start debug server on localhost:2345
debug-server: debug-build
	@echo "Starting debug server on localhost:2345..."
	@dlv exec $(BUILD_DIR)/$(BINARY_NAME)-debug --headless --listen=:2345 --api-version=2 --accept-multiclient

## debug-remote: Show instructions for remote debugging
debug-remote:
	@echo "Remote Debugging Setup:"
	@echo ""
	@echo "1. On remote machine:"
	@echo "   make debug-build"
	@echo "   dlv exec ./bin/$(BINARY_NAME)-debug --headless --listen=:2345 --api-version=2"
	@echo ""
	@echo "2. On local machine (in another terminal):"
	@echo "   ssh -L 2345:localhost:2345 user@remote-host"
	@echo ""
	@echo "3. Connect debugger:"
	@echo "   dlv connect localhost:2345"
	@echo "   OR use VS Code/GoLand remote debugging (see DEBUGGING.md)"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
