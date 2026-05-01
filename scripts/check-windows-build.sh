#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Cross-compiles the codebase for Windows on a non-Windows host. Catches build-time
# regressions (Linux-only imports, build-tag mistakes, missing platform shims)
# before they reach Windows CI. Does NOT catch runtime path bugs — that is the
# cross-platform-paths goplint analyzer's job.
#
# Runs on both Go modules: the root module and tools/goplint/.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "Cross-compiling for GOOS=windows GOARCH=amd64..."

# Disable cgo for portable cross-compile (no Windows toolchain needed locally).
# CGO_ENABLED=0 is what release builds use for Windows anyway.
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

# Use a discardable cache scope to avoid polluting the developer's normal
# native build cache with windows-amd64 objects.
WIN_BUILD_CACHE="${WIN_BUILD_CACHE:-$ROOT_DIR/.cache/go-build-windows}"
mkdir -p "$WIN_BUILD_CACHE"
export GOCACHE="$WIN_BUILD_CACHE"

errors=0

run() {
    local label=$1
    shift
    if ! "$@"; then
        echo "FAIL: $label"
        errors=$((errors + 1))
    fi
}

# Root module
echo "  [1/4] go build  (root module)..."
run "root build" go build ./...
echo "  [2/4] go vet    (root module)..."
run "root vet" go vet ./...

# Tools module (goplint)
echo "  [3/4] go build  (tools/goplint)..."
(cd tools/goplint && run "tools/goplint build" go build ./...)
echo "  [4/4] go vet    (tools/goplint)..."
(cd tools/goplint && run "tools/goplint vet" go vet ./...)

if [[ "$errors" -gt 0 ]]; then
    echo ""
    echo "Windows cross-compile failed with $errors error(s)."
    echo "Common causes:"
    echo "  - Linux-only import (e.g. golang.org/x/sys/unix without build tag)"
    echo "  - Wrong build tag (//go:build linux instead of //go:build !windows)"
    echo "  - Missing platform-specific shim file for windows"
    echo "See .agents/rules/windows.md for cross-platform build patterns."
    exit 1
fi

echo "Windows cross-compile OK."
