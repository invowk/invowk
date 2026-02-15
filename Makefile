# Makefile for invowk
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
LDFLAGS += -X 'github.com/invowk/invowk/cmd/invowk.Version=$(VERSION)'
LDFLAGS += -X 'github.com/invowk/invowk/cmd/invowk.Commit=$(COMMIT)'
LDFLAGS += -X 'github.com/invowk/invowk/cmd/invowk.BuildDate=$(BUILD_DATE)'

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

# Detect gotestsum for enhanced test output and rerun-fails support
GOTESTSUM := $(shell command -v gotestsum 2>/dev/null)

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

# Run tests (uses gotestsum with rerun-fails when available, falls back to go test)
.PHONY: test
test:
	@echo "Running tests..."
ifdef GOTESTSUM
	@echo "  (using gotestsum)"
	gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 5 --packages ./... -- -v
else
	@echo "  (gotestsum not found, using go test)"
	$(GOTEST) -v ./...
endif

# Run tests (short mode, skip integration tests)
.PHONY: test-short
test-short:
	@echo "Running tests (short mode)..."
ifdef GOTESTSUM
	@echo "  (using gotestsum)"
	gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 3 --packages ./... -- -v -short
else
	@echo "  (gotestsum not found, using go test)"
	$(GOTEST) -v -short ./...
endif

# Run integration tests only
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
ifdef GOTESTSUM
	@echo "  (using gotestsum)"
	gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 3 --packages ./... -- -v -run Integration
else
	@echo "  (gotestsum not found, using go test)"
	$(GOTEST) -v -run Integration ./...
endif

# Run CLI integration tests (testscript-based)
.PHONY: test-cli
test-cli:
	@echo "Running CLI integration tests..."
ifdef GOTESTSUM
	@echo "  (using gotestsum)"
	gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 3 --packages ./tests/cli/... -- -v -race -timeout 5m
else
	@echo "  (gotestsum not found, using go test)"
	$(GOTEST) -v -race -timeout 5m ./tests/cli/...
endif

# Generate PGO profile from benchmarks (includes container tests)
# This produces a CPU profile that Go 1.20+ uses for Profile-Guided Optimization.
# The profile is stored as default.pgo which Go automatically detects.
.PHONY: pgo-profile
pgo-profile:
	@echo "Generating PGO profile from benchmarks..."
	@echo "This may take several minutes..."
	$(GOTEST) -run=^$$ -bench=. -benchtime=10s -cpuprofile=cpu.prof ./internal/benchmark/
	@mv cpu.prof default.pgo
	@echo ""
	@echo "PGO profile generated: default.pgo"
	@ls -lh default.pgo | awk '{print "Profile size:", $$5}'
	@echo ""
	@echo "To verify PGO is active during builds:"
	@echo "  GODEBUG=pgoinstall=1 make build 2>&1 | grep -i pgo"

# Generate PGO profile (short mode - no container benchmarks)
# Faster but may result in less comprehensive optimization.
.PHONY: pgo-profile-short
pgo-profile-short:
	@echo "Generating PGO profile (short mode)..."
	$(GOTEST) -run=^$$ -bench=. -benchtime=10s -short -cpuprofile=cpu.prof ./internal/benchmark/
	@mv cpu.prof default.pgo
	@echo ""
	@echo "PGO profile generated: default.pgo"
	@ls -lh default.pgo | awk '{print "Profile size:", $$5}'

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
		if ! head -1 "$$file" | grep -q "SPDX-License-Identifier: MPL-2.0"; then \
			echo "Missing SPDX header: $$file"; \
			missing=$$((missing + 1)); \
		fi; \
	done; \
	if [ $$missing -gt 0 ]; then \
		echo ""; \
		echo "ERROR: $$missing file(s) missing SPDX-License-Identifier: MPL-2.0 header"; \
		echo "All Go source files must start with: // SPDX-License-Identifier: MPL-2.0"; \
		exit 1; \
	else \
		echo "All Go files have proper SPDX license headers."; \
	fi

# Run golangci-lint
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

# Lint shell scripts with shellcheck (optional tool, like gotestsum)
SHELLCHECK := $(shell command -v shellcheck 2>/dev/null)

.PHONY: lint-scripts
lint-scripts:
	@echo "Linting shell scripts..."
ifdef SHELLCHECK
	@echo "  (using shellcheck)"
	shellcheck scripts/install.sh scripts/release.sh scripts/version-docs.sh scripts/render-diagrams.sh
else
	@echo "  (shellcheck not found, skipping shell script linting)"
endif

# Run install script unit tests (POSIX only; PowerShell tests run on Windows CI)
.PHONY: test-scripts
test-scripts:
	@echo "Running shell script tests..."
	sh scripts/test_install.sh
	@echo ""
	@echo "Note: PowerShell tests (scripts/test_install.ps1) run on Windows CI only."

# Install pre-commit hooks
.PHONY: install-hooks
install-hooks:
	@echo "Installing pre-commit hooks..."
	@command -v pre-commit >/dev/null 2>&1 || { echo "Error: pre-commit not installed. Run: pip install pre-commit"; exit 1; }
	pre-commit install
	@echo "Pre-commit hooks installed successfully."

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

# Render D2 diagrams to SVG (requires D2 installed locally)
# Uses TALA layout engine if available, falls back to ELK
.PHONY: render-diagrams
render-diagrams:
	@echo "Rendering D2 diagrams..."
	./scripts/render-diagrams.sh

# VHS Demo Generation (not used for CI testing - see test-cli for that)
.PHONY: vhs-demos vhs-validate

# Generate VHS demo recordings
vhs-demos: build
	@echo "Generating VHS demo recordings..."
	@command -v vhs >/dev/null 2>&1 || { echo "Error: VHS is not installed. See vhs/README.md"; exit 1; }
	@command -v ffmpeg >/dev/null 2>&1 || { echo "Error: ffmpeg is not installed. See vhs/README.md"; exit 1; }
	@command -v ttyd >/dev/null 2>&1 || { echo "Error: ttyd is not installed. See vhs/README.md"; exit 1; }
	@cd vhs/demos && for tape in *.tape; do echo "Recording $$tape..."; vhs "$$tape"; done

# Validate VHS tape syntax (only needs vhs, no recording)
vhs-validate:
	@echo "Validating VHS tape syntax..."
	@command -v vhs >/dev/null 2>&1 || { echo "Error: VHS is not installed. See vhs/README.md"; exit 1; }
	@vhs validate vhs/demos/*.tape

# Version docs: snapshot documentation for a release
# Usage: make version-docs VERSION=1.0.0
.PHONY: version-docs
version-docs:
ifndef VERSION
	$(error VERSION is required (without v prefix). Usage: make version-docs VERSION=1.0.0)
endif
	@./scripts/version-docs.sh "$(VERSION)"

# Release: create and push a signed version tag
# Usage: make release VERSION=v0.1.0-alpha.1 [YES=1] [DRY_RUN=1]
.PHONY: release
release:
ifeq ($(filter command line,$(origin VERSION)),)
	$(error VERSION is required. Usage: make release VERSION=v0.1.0-alpha.1)
endif
	@./scripts/release.sh tag "$(VERSION)" "$(YES)" "$(DRY_RUN)"

# Release bump: compute next version and create signed tag
# Usage: make release-bump TYPE=minor [PRERELEASE=alpha] [PROMOTE=1] [YES=1] [DRY_RUN=1]
.PHONY: release-bump
release-bump:
ifndef TYPE
	$(error TYPE is required (major, minor, or patch). Usage: make release-bump TYPE=minor)
endif
	@./scripts/release.sh bump "$(TYPE)" "$(PRERELEASE)" "$(PROMOTE)" "$(YES)" "$(DRY_RUN)"

# Help
.PHONY: help
help:
	@echo "invowk Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build stripped binary (default)"
	@echo "  build-upx        Build UPX-compressed binary (requires UPX)"
	@echo "  build-all        Build both stripped and UPX variants"
	@echo "  build-dev        Build with debug symbols (for development)"
	@echo "  build-cross      Cross-compile for Linux, macOS, Windows"
	@echo "  test             Run all tests"
	@echo "  test-short       Run tests in short mode (skip integration)"
	@echo "  test-integration Run integration tests only"
	@echo "  test-cli         Run CLI integration tests (testscript)"
	@echo "  pgo-profile      Generate PGO profile from benchmarks (full)"
	@echo "  pgo-profile-short Generate PGO profile (short, no container benchmarks)"
	@echo "  vhs-demos        Generate VHS demo recordings (requires VHS)"
	@echo "  vhs-validate     Validate VHS tape syntax"
	@echo "  render-diagrams  Render D2 diagrams to SVG (requires D2)"
	@echo "  version-docs     Snapshot docs for a release version"
	@echo "  release          Create and push a signed version tag"
	@echo "  release-bump     Compute next version and create signed tag"
	@echo "  clean            Remove build artifacts"
	@echo "  install          Install to GOPATH/bin"
	@echo "  tidy             Tidy go.mod dependencies"
	@echo "  license-check    Verify SPDX headers in all Go files"
	@echo "  lint             Run golangci-lint on all packages"
	@echo "  lint-scripts     Lint shell scripts (requires shellcheck)"
	@echo "  test-scripts     Run install script tests (POSIX; PS1 on Windows CI)"
	@echo "  install-hooks    Install pre-commit hooks (requires pre-commit)"
	@echo "  size             Compare binary sizes (debug vs stripped vs UPX)"
	@echo "  help             Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION        Override version string (default: git describe)"
	@echo "  GOAMD64        x86-64 microarchitecture level (default: v3)"
	@echo "                 v1 = baseline x86-64 (maximum compatibility)"
	@echo "                 v2 = Nehalem+ (2008+): CMPXCHG16B, LAHF, SAHF, POPCNT, SSE3, SSE4.1, SSE4.2, SSSE3"
	@echo "                 v3 = Haswell+ (2013+): AVX, AVX2, BMI1, BMI2, F16C, FMA, LZCNT, MOVBE"
	@echo "                 v4 = Skylake-X+ (2017+): AVX512F, AVX512BW, AVX512CD, AVX512DQ, AVX512VL"
	@echo "  TYPE           Bump type for release-bump: major, minor, or patch"
	@echo "  PRERELEASE     Pre-release label: alpha, beta, or rc (optional)"
	@echo "  PROMOTE        Set to 1 to allow promoting a prerelease stream to stable"
	@echo "  YES            Set to 1 to skip confirmation prompts"
	@echo "  DRY_RUN        Set to 1 to show actions without executing them"
	@echo ""
	@echo "Examples:"
	@echo "  make build                                  # Build for x86-64-v3 (default)"
	@echo "  make build GOAMD64=v1                       # Build for baseline x86-64 (max compat)"
	@echo "  make build-cross GOAMD64=v2                 # Cross-compile with x86-64-v2"
	@echo "  make release VERSION=v1.0.0                 # Tag and push v1.0.0"
	@echo "  make release-bump TYPE=minor                # Bump minor version (e.g., v1.0.0 -> v1.1.0)"
	@echo "  make release-bump TYPE=minor PRERELEASE=alpha  # Start/continue alpha stream"
	@echo "  make release-bump TYPE=minor PROMOTE=1      # Promote prerelease to stable"
	@echo "  make release-bump TYPE=patch DRY_RUN=1      # Preview next patch version"
