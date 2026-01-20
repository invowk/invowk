#!/usr/bin/env bash
# run-tests.sh - Run VHS integration tests and compare against golden files
#
# This script:
# 1. Runs each VHS tape file to generate output
# 2. Normalizes the output to remove variable content
# 3. Compares normalized output against golden files
# 4. Reports pass/fail for each test
#
# Usage: ./run-tests.sh [tape_pattern]
#   tape_pattern: Optional glob pattern to filter which tapes to run (default: *.tape)
#
# Exit codes:
#   0 - All tests passed
#   1 - One or more tests failed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VHS_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$VHS_DIR")"

TAPES_DIR="$VHS_DIR/tapes"
GOLDEN_DIR="$VHS_DIR/golden"
OUTPUT_DIR="$VHS_DIR/output"
NORMALIZE_SCRIPT="$SCRIPT_DIR/normalize.sh"

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

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

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

echo "Running VHS integration tests..."
echo "  Tapes directory: $TAPES_DIR"
echo "  Golden directory: $GOLDEN_DIR"
echo "  Output directory: $OUTPUT_DIR"
echo ""

PASSED=0
FAILED=0
SKIPPED=0

for tape in "${TAPES[@]}"; do
    tape_name="$(basename "$tape" .tape)"
    golden_file="$GOLDEN_DIR/${tape_name}.golden"
    output_file="$OUTPUT_DIR/${tape_name}.txt"
    normalized_file="$OUTPUT_DIR/${tape_name}.normalized"

    printf "  %-30s " "$tape_name"

    # Check if golden file exists
    if [[ ! -f "$golden_file" ]]; then
        echo -e "${YELLOW}SKIP${NC} (no golden file)"
        ((SKIPPED++))
        continue
    fi

    # Run VHS tape
    # VHS tapes should output to files in output/ directory
    # We run from the root directory so paths work correctly
    if ! (cd "$ROOT_DIR" && vhs "$tape" 2>/dev/null); then
        echo -e "${RED}FAIL${NC} (vhs execution error)"
        ((FAILED++))
        continue
    fi

    # Check if output file was created
    if [[ ! -f "$output_file" ]]; then
        echo -e "${RED}FAIL${NC} (no output file)"
        ((FAILED++))
        continue
    fi

    # Normalize output
    "$NORMALIZE_SCRIPT" "$output_file" > "$normalized_file"

    # Compare with golden file
    if diff -q "$golden_file" "$normalized_file" > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++))
    else
        echo -e "${RED}FAIL${NC}"
        ((FAILED++))
        echo ""
        echo "    Differences:"
        diff -u "$golden_file" "$normalized_file" | head -30 | sed 's/^/    /'
        echo ""
    fi
done

echo ""
echo "Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC}, ${YELLOW}$SKIPPED skipped${NC}"

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

exit 0
