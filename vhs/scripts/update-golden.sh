#!/usr/bin/env bash
# update-golden.sh - Update golden files from current VHS output
#
# This script:
# 1. Runs each VHS tape file to generate output
# 2. Normalizes the output to remove variable content
# 3. Updates the golden files with the normalized output
#
# Usage: ./update-golden.sh [tape_pattern]
#   tape_pattern: Optional glob pattern to filter which tapes to update (default: *.tape)
#
# IMPORTANT: Review golden file changes before committing!

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VHS_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$VHS_DIR")"

TAPES_DIR="$VHS_DIR/tapes"
GOLDEN_DIR="$VHS_DIR/golden"
OUTPUT_DIR="$VHS_DIR/output"
NORMALIZE_CONFIG="$VHS_DIR/normalize.cue"

# Colors for output (if terminal supports it)
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    NC=''
fi

# Parse arguments
TAPE_PATTERN="${1:-*.tape}"

# Ensure directories exist
mkdir -p "$OUTPUT_DIR"
mkdir -p "$GOLDEN_DIR"

# Check for VHS
if ! command -v vhs &> /dev/null; then
    echo -e "${RED}Error: VHS is not installed${NC}" >&2
    echo "Install VHS from: https://github.com/charmbracelet/vhs" >&2
    exit 1
fi

# Check for invowk binary
if [[ ! -x "$ROOT_DIR/bin/invowk" ]]; then
    echo -e "${RED}Error: invowk binary not found at $ROOT_DIR/bin/invowk${NC}" >&2
    echo "Run 'make build' first" >&2
    exit 1
fi

# Find all tape files matching pattern
mapfile -t TAPES < <(find "$TAPES_DIR" -name "$TAPE_PATTERN" -type f | sort)

if [[ ${#TAPES[@]} -eq 0 ]]; then
    echo -e "${YELLOW}No tape files found matching pattern: $TAPE_PATTERN${NC}"
    exit 0
fi

echo "Updating VHS golden files..."
echo "  Tapes directory: $TAPES_DIR"
echo "  Golden directory: $GOLDEN_DIR"
echo "  Output directory: $OUTPUT_DIR"
echo ""

UPDATED=0
FAILED=0

for tape in "${TAPES[@]}"; do
    tape_name="$(basename "$tape" .tape)"
    golden_file="$GOLDEN_DIR/${tape_name}.golden"
    output_file="$OUTPUT_DIR/${tape_name}.txt"
    normalized_file="$OUTPUT_DIR/${tape_name}.normalized"

    printf "  %-30s " "$tape_name"

    # Run VHS tape
    if ! (cd "$ROOT_DIR" && vhs "$tape" 2>/dev/null); then
        echo -e "${RED}FAIL${NC} (vhs execution error)"
        FAILED=$((FAILED + 1))
        continue
    fi

    # Check if output file was created
    if [[ ! -f "$output_file" ]]; then
        echo -e "${RED}FAIL${NC} (no output file)"
        FAILED=$((FAILED + 1))
        continue
    fi

    # Normalize output using Go-based normalizer
    "$ROOT_DIR/bin/invowk" internal vhs normalize "$output_file" -c "$NORMALIZE_CONFIG" -o "$normalized_file"

    # Update golden file
    cp "$normalized_file" "$golden_file"
    echo -e "${GREEN}UPDATED${NC}"
    UPDATED=$((UPDATED + 1))
done

echo ""
echo "Results: ${GREEN}$UPDATED updated${NC}, ${RED}$FAILED failed${NC}"
echo ""
echo -e "${YELLOW}IMPORTANT: Review the golden file changes before committing!${NC}"
echo "  git diff $GOLDEN_DIR/"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

exit 0
