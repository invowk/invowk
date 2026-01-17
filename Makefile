# Makefile for invowk-cli
#
# Build targets:
#   make build       - Build stripped binary (default)
#   make build-upx   - Build UPX-compressed binary (smallest size)
#   make build-all   - Build both variants
#   make clean       - Remove build artifacts
#   make test        - Run all tests
#   make install     - Install to GOPATH/bin

# Root dir
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

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

# x86-64 microarchitecture level for amd64 builds
# v3 = Haswell/Excavator+ (2013+): AVX, AVX2, BMI1, BMI2, F16C, FMA, LZCNT, MOVBE
# This provides better performance on modern CPUs while maintaining broad compatibility
# Override with: make build GOAMD64=v2 (or v1 for maximum compatibility)
GOAMD64 ?= v3

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

# Detect host architecture for applying GOAMD64
# GOAMD64 only applies when GOARCH=amd64
HOST_ARCH := $(shell $(GOCMD) env GOARCH)
ifeq ($(HOST_ARCH),amd64)
    AMD64_ENV := GOAMD64=$(GOAMD64)
else
    AMD64_ENV :=
endif

# Default target
.DEFAULT_GOAL := build

# Ensure build directory exists
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build stripped binary (no UPX)
# On amd64, targets x86-64-v3 microarchitecture by default
.PHONY: build
build: $(BUILD_DIR)
	@echo "Building $(BINARY_NAME) (stripped)..."
ifeq ($(HOST_ARCH),amd64)
	@echo "  Target: x86-64-$(GOAMD64)"
endif
	$(AMD64_ENV) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME) | awk '{print "Size:", $$5}'

# Build UPX-compressed binary
.PHONY: build-upx
build-upx: $(BUILD_DIR)
	@echo "Building $(BINARY_UPX) (stripped + UPX compressed)..."
ifeq ($(HOST_ARCH),amd64)
	@echo "  Target: x86-64-$(GOAMD64)"
endif
	@command -v $(UPX) >/dev/null 2>&1 || { echo "Error: UPX is not installed. Install it with your package manager."; exit 1; }
	$(AMD64_ENV) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_UPX) .
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
ifeq ($(HOST_ARCH),amd64)
	@echo "  Target: x86-64-$(GOAMD64)"
endif
	$(AMD64_ENV) $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) .
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
	$(GOTEST) -v -short ./...

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

# Check SPDX license headers in all Go files
.PHONY: license-check
license-check:
	@echo "Checking SPDX license headers..."
	@missing=0; \
	for file in $$(find . -name "*.go" -type f); do \
		if ! head -1 "$$file" | grep -q "SPDX-License-Identifier: EPL-2.0"; then \
			echo "Missing SPDX header: $$file"; \
			missing=$$((missing + 1)); \
		fi; \
	done; \
	if [ $$missing -gt 0 ]; then \
		echo ""; \
		echo "ERROR: $$missing file(s) missing SPDX-License-Identifier: EPL-2.0 header"; \
		echo "All Go source files must start with: // SPDX-License-Identifier: EPL-2.0"; \
		exit 1; \
	else \
		echo "All Go files have proper SPDX license headers."; \
	fi

# Run golangci-lint
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Show binary sizes comparison
.PHONY: size
size: $(BUILD_DIR)
	@echo "Building size comparison..."
ifeq ($(HOST_ARCH),amd64)
	@echo "  Target: x86-64-$(GOAMD64)"
endif
	@echo ""
	@echo "=== Debug build (with symbols) ==="
	@$(AMD64_ENV) $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-debug . && ls -lh $(BUILD_DIR)/$(BINARY_NAME)-debug | awk '{print "Size:", $$5}'
	@echo ""
	@echo "=== Stripped build ==="
	@$(AMD64_ENV) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-stripped . && ls -lh $(BUILD_DIR)/$(BINARY_NAME)-stripped | awk '{print "Size:", $$5}'
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
# amd64 targets use x86-64-v3 microarchitecture by default
.PHONY: build-cross
build-cross: $(BUILD_DIR)
	@echo "Cross-compiling for multiple platforms..."
	@echo "  amd64 targets: x86-64-$(GOAMD64)"
	GOOS=linux GOARCH=amd64 GOAMD64=$(GOAMD64) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 GOAMD64=$(GOAMD64) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 GOAMD64=$(GOAMD64) $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
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
	@echo "  license-check  Verify SPDX headers in all Go files"
	@echo "  lint           Run golangci-lint on all packages"
	@echo "  size           Compare binary sizes (debug vs stripped vs UPX)"
	@echo "  help           Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION        Override version string (default: git describe)"
	@echo "  GOAMD64        x86-64 microarchitecture level (default: v3)"
	@echo "                 v1 = baseline x86-64 (maximum compatibility)"
	@echo "                 v2 = Nehalem+ (2008+): CMPXCHG16B, LAHF, SAHF, POPCNT, SSE3, SSE4.1, SSE4.2, SSSE3"
	@echo "                 v3 = Haswell+ (2013+): AVX, AVX2, BMI1, BMI2, F16C, FMA, LZCNT, MOVBE"
	@echo "                 v4 = Skylake-X+ (2017+): AVX512F, AVX512BW, AVX512CD, AVX512DQ, AVX512VL"
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build for x86-64-v3 (default)"
	@echo "  make build GOAMD64=v1         # Build for baseline x86-64 (max compat)"
	@echo "  make build-cross GOAMD64=v2   # Cross-compile with x86-64-v2"
