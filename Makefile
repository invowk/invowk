# Makefile for invowk-cli
#
# Build targets:
#   make build       - Build stripped binary (default)
#   make build-upx   - Build UPX-compressed binary (smallest size)
#   make build-all   - Build both variants
#   make clean       - Remove build artifacts
#   make test        - Run all tests
#   make install     - Install to GOPATH/bin

# Binary name
BINARY_NAME := invowk
BINARY_UPX := $(BINARY_NAME)-upx

# Build directory
BUILD_DIR := bin

# Go parameters (override GOCMD if go is not in PATH)
GOCMD ?= go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Version info (can be overridden)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags for stripping and version info
LDFLAGS := -s -w
LDFLAGS += -X 'invowk-cli/cmd/invowk.Version=$(VERSION)'
LDFLAGS += -X 'invowk-cli/cmd/invowk.Commit=$(COMMIT)'
LDFLAGS += -X 'invowk-cli/cmd/invowk.BuildDate=$(BUILD_DATE)'

# Build flags
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

# UPX parameters (--best for maximum compression, --lzma for better ratio)
UPX := upx
UPX_FLAGS := --best --lzma

# Default target
.DEFAULT_GOAL := build

# Ensure build directory exists
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build stripped binary (no UPX)
.PHONY: build
build: $(BUILD_DIR)
	@echo "Building $(BINARY_NAME) (stripped)..."
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) | awk '{print "Size:", $$5}'

# Build UPX-compressed binary
.PHONY: build-upx
build-upx: $(BUILD_DIR)
	@echo "Building $(BINARY_UPX) (stripped + UPX compressed)..."
	@command -v $(UPX) >/dev/null 2>&1 || { echo "Error: UPX is not installed. Install it with your package manager."; exit 1; }
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_UPX) .
	@echo "Compressing with UPX..."
	$(UPX) $(UPX_FLAGS) $(BUILD_DIR)/$(BINARY_UPX)
	@echo "Built: $(BUILD_DIR)/$(BINARY_UPX)"
	@ls -lh $(BUILD_DIR)/$(BINARY_UPX) | awk '{print "Size:", $$5}'

# Build both variants
.PHONY: build-all
build-all: build build-upx
	@echo ""
	@echo "Build complete. Artifacts:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_UPX) 2>/dev/null | awk '{print $$9 ":", $$5}'

# Build for development (with debug symbols, faster)
.PHONY: build-dev
build-dev: $(BUILD_DIR)
	@echo "Building $(BINARY_NAME) (development)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) | awk '{print "Size:", $$5}'

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests (short mode, skip integration tests)
.PHONY: test-short
test-short:
	@echo "Running tests (short mode)..."
	$(GOTEST) -short ./...

# Run integration tests only
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -run Integration ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME) $(BINARY_UPX)
	@echo "Clean complete."

# Install to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed: $(GOPATH)/bin/$(BINARY_NAME)"

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Show binary sizes comparison
.PHONY: size
size: $(BUILD_DIR)
	@echo "Building size comparison..."
	@echo ""
	@echo "=== Debug build (with symbols) ==="
	@$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-debug . && ls -lh $(BUILD_DIR)/$(BINARY_NAME)-debug | awk '{print "Size:", $$5}'
	@echo ""
	@echo "=== Stripped build ==="
	@$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-stripped . && ls -lh $(BUILD_DIR)/$(BINARY_NAME)-stripped | awk '{print "Size:", $$5}'
	@echo ""
	@if command -v $(UPX) >/dev/null 2>&1; then \
		echo "=== UPX compressed ==="; \
		cp $(BUILD_DIR)/$(BINARY_NAME)-stripped $(BUILD_DIR)/$(BINARY_NAME)-upx-test; \
		$(UPX) $(UPX_FLAGS) -q $(BUILD_DIR)/$(BINARY_NAME)-upx-test 2>/dev/null; \
		ls -lh $(BUILD_DIR)/$(BINARY_NAME)-upx-test | awk '{print "Size:", $$5}'; \
		rm -f $(BUILD_DIR)/$(BINARY_NAME)-upx-test; \
	else \
		echo "=== UPX compressed (skipped - UPX not installed) ==="; \
	fi
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)-debug $(BUILD_DIR)/$(BINARY_NAME)-stripped

# Cross-compile for multiple platforms
.PHONY: build-cross
build-cross: $(BUILD_DIR)
	@echo "Cross-compiling for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo ""
	@echo "Cross-compilation complete:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-* | awk '{print $$9 ":", $$5}'

# Help
.PHONY: help
help:
	@echo "invowk-cli Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build stripped binary (default)"
	@echo "  build-upx      Build UPX-compressed binary (requires UPX)"
	@echo "  build-all      Build both stripped and UPX variants"
	@echo "  build-dev      Build with debug symbols (for development)"
	@echo "  build-cross    Cross-compile for Linux, macOS, Windows"
	@echo "  test           Run all tests"
	@echo "  test-short     Run tests in short mode (skip integration)"
	@echo "  test-integration Run integration tests only"
	@echo "  clean          Remove build artifacts"
	@echo "  install        Install to GOPATH/bin"
	@echo "  tidy           Tidy go.mod dependencies"
	@echo "  size           Compare binary sizes (debug vs stripped vs UPX)"
	@echo "  help           Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION        Override version string (default: git describe)"
