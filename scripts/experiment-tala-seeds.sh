#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Experiment with different TALA seed values for D2 diagrams.
#
# Usage: ./scripts/experiment-tala-seeds.sh
#
# This script renders a representative set of D2 diagrams with multiple
# seed values to determine the optimal default for deterministic layouts.
#
# Output: docs/diagrams/experiments/seed-{N}/
#
# Prerequisites:
#   - d2 (https://d2lang.com) must be installed
#   - TALA layout engine must be licensed

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

# Check if TALA is available
if ! d2 --layout=tala /dev/null /dev/null 2>/dev/null; then
    echo "ERROR: TALA layout engine is not available."
    echo "This experiment requires TALA for meaningful results."
    echo "Install TALA: https://d2lang.com/tour/tala"
    exit 1
fi

echo "TALA Seeds Experiment"
echo "====================="
echo ""

# Test diagrams (representative sample)
DIAGRAMS=(
    "c4/container.d2"
    "c4/context.d2"
    "flowcharts/runtime-decision.d2"
    "flowcharts/discovery-flow.d2"
    "sequences/execution-main.d2"
)

# Seed values to test
SEEDS=(0 1 7 13 23 42 100 123 256 500 1000 9999)

# Output directory
OUTPUT_DIR="docs/diagrams/experiments"

# Track statistics
total=0
success=0
failed=0

# Render each diagram with each seed
for seed in "${SEEDS[@]}"; do
    seed_dir="$OUTPUT_DIR/seed-$seed"
    mkdir -p "$seed_dir"

    echo "Rendering with seed $seed..."

    for diagram in "${DIAGRAMS[@]}"; do
        total=$((total + 1))

        # Input path
        input="docs/diagrams/$diagram"

        # Output path: convert path separators to dashes
        # e.g., c4/container.d2 -> c4-container.svg
        base="${diagram%.d2}"
        output_name="${base//\//-}.svg"
        output="$seed_dir/$output_name"

        echo -n "  $diagram -> seed-$seed/$output_name ... "

        if d2 --layout=tala --tala-seeds="$seed" "$input" "$output" 2>/dev/null; then
            echo "OK"
            success=$((success + 1))
        else
            echo "FAILED"
            failed=$((failed + 1))
        fi
    done

    echo ""
done

# Summary
echo "=== Experiment Summary ==="
echo "Total renders:  $total"
echo "Successful:     $success"
echo "Failed:         $failed"
echo "Seeds tested:   ${SEEDS[*]}"
echo "Diagrams:       ${#DIAGRAMS[@]}"
echo ""
echo "Output: $OUTPUT_DIR/seed-*/"
echo ""
echo "Next steps:"
echo "  1. Open SVGs in docs/diagrams/experiments/seed-*/  for visual comparison"
echo "  2. Score each diagram according to evaluation criteria"
echo "  3. Document findings in docs/diagrams/experiments/README.md"

if [ $failed -gt 0 ]; then
    echo ""
    echo "WARNING: Some renders failed. Check d2 output for details."
    exit 1
fi
