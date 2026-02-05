#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Render all D2 diagrams to SVG.
#
# Usage: ./scripts/render-diagrams.sh
#
# This script renders all D2 files in docs/diagrams/ to SVG files in docs/diagrams/rendered/.
# It automatically detects TALA availability and falls back to ELK if TALA is not licensed.
#
# Prerequisites:
#   - d2 (https://d2lang.com) must be installed
#   - TALA layout engine (optional, for production-quality layouts)
#
# For external contributors without TALA:
#   1. Edit .d2 source files
#   2. Run `d2 validate` to check syntax
#   3. Submit PR with only .d2 changes
#   4. Maintainer renders SVGs before merge

set -euo pipefail

# Change to repository root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# Check if d2 is installed
if ! command -v d2 &>/dev/null; then
    echo "ERROR: d2 is not installed."
    echo "Install it from: https://d2lang.com"
    exit 1
fi

# Detect layout engine: TALA if licensed, otherwise ELK
# TALA is detected by checking if `d2 --layout=tala` works without error
LAYOUT="elk"
if d2 --layout=tala /dev/null /dev/null 2>/dev/null; then
    LAYOUT="tala"
    echo "Using TALA layout engine (production quality)"
else
    echo "Using ELK layout engine (TALA not available)"
    echo "  Tip: For production diagrams, install TALA: https://d2lang.com/tour/tala"
fi

# Track statistics
total=0
success=0
failed=0

# Find and render all D2 files
while IFS= read -r -d '' d2file; do
    total=$((total + 1))

    # Compute paths
    # Input:  docs/diagrams/c4/context.d2
    # Output: docs/diagrams/rendered/c4/context.svg
    rel="${d2file#docs/diagrams/}"       # c4/context.d2
    dir="${rel%/*}"                       # c4
    name="${d2file##*/}"                  # context.d2
    base="${name%.d2}"                    # context
    out="docs/diagrams/rendered/${dir}/${base}.svg"

    # Create output directory if needed
    mkdir -p "$(dirname "$out")"

    echo -n "Rendering: $d2file -> $out ... "

    # Format the D2 file first (ensures canonical formatting)
    if ! d2 fmt "$d2file" 2>/dev/null; then
        echo "FAILED (fmt)"
        failed=$((failed + 1))
        continue
    fi

    # Render to SVG
    # When using TALA, pass --tala-seeds=100 for deterministic layout output
    # Seed 100 provides optimal compactness for flowcharts while maintaining
    # quality C4 layouts. See docs/diagrams/experiments/README.md for analysis.
    render_args=(--layout="$LAYOUT")
    if [ "$LAYOUT" = "tala" ]; then
        render_args+=(--tala-seeds=100)
    fi

    if d2 "${render_args[@]}" "$d2file" "$out" 2>/dev/null; then
        echo "OK"
        success=$((success + 1))
    else
        echo "FAILED (render)"
        failed=$((failed + 1))
    fi
done < <(find docs/diagrams -name "*.d2" -type f -print0 2>/dev/null | sort -z)

# Summary
echo ""
echo "=== Render Summary ==="
echo "Total:   $total"
echo "Success: $success"
echo "Failed:  $failed"
echo "Layout:  $LAYOUT"

if [ $failed -gt 0 ]; then
    echo ""
    echo "Some diagrams failed to render. Run 'd2 validate <file>' for details."
    exit 1
fi

if [ $total -eq 0 ]; then
    echo ""
    echo "No D2 files found in docs/diagrams/"
    echo "Create .d2 files in docs/diagrams/{c4,sequences,flowcharts}/ to render them."
fi
