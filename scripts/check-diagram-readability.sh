#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Validate readability guardrails for flowchart diagrams.
#
# Rules (strict):
# 1. `direction:` must be explicitly set to up/down/right/left.
# 2. A top-level `Start:` node must exist.
# 3. `Start` must be defined as an oval shape.
# 4. At least one outbound `Start -> ...` edge must exist.
#
# Usage: ./scripts/check-diagram-readability.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

FLOWCHART_DIR="docs/diagrams/flowcharts"

if [ ! -d "$FLOWCHART_DIR" ]; then
  echo "ERROR: ${FLOWCHART_DIR} does not exist."
  exit 1
fi

files=()
while IFS= read -r -d '' file; do
  files+=("$file")
done < <(find "$FLOWCHART_DIR" -name "*.d2" -type f -print0 | sort -z)

if [ "${#files[@]}" -eq 0 ]; then
  echo "ERROR: no flowchart diagrams found in ${FLOWCHART_DIR}."
  exit 1
fi

errors=0

for file in "${files[@]}"; do
  file_errors=0
  echo "Checking: $file"

  if ! grep -Eq '^[[:space:]]*direction:[[:space:]]*(up|down|right|left)[[:space:]]*$' "$file"; then
    echo "  ERROR: missing explicit direction (up/down/right/left)."
    file_errors=$((file_errors + 1))
  fi

  if ! grep -Eq '^[[:space:]]*Start:[[:space:]]' "$file"; then
    echo "  ERROR: missing Start node."
    file_errors=$((file_errors + 1))
  else
    start_line="$(grep -nE '^[[:space:]]*Start:[[:space:]].*\{' "$file" | head -n1 | cut -d: -f1 || true)"

    if [ -z "$start_line" ]; then
      echo "  ERROR: Start must use block syntax so shape can be validated."
      file_errors=$((file_errors + 1))
    else
      if ! awk -v start="$start_line" '
        NR < start { next }
        NR == start { in_block = 1; next }
        in_block {
          if ($0 ~ /shape:[[:space:]]*oval/) {
            has_oval = 1
          }
          if ($0 ~ /^[[:space:]]*}/) {
            if (has_oval == 1) {
              exit 0
            }
            exit 1
          }
        }
        END {
          if (in_block != 1 || has_oval != 1) {
            exit 1
          }
        }
      ' "$file"; then
        echo "  ERROR: Start node must declare shape: oval."
        file_errors=$((file_errors + 1))
      fi
    fi
  fi

  if ! grep -Eq 'Start[[:space:]]*->[[:space:]]*[A-Za-z0-9_.]+' "$file"; then
    echo "  ERROR: missing outbound Start edge (Start -> ...)."
    file_errors=$((file_errors + 1))
  fi

  if [ "$file_errors" -eq 0 ]; then
    echo "  OK"
  else
    errors=$((errors + file_errors))
  fi
done

echo ""
if [ "$errors" -gt 0 ]; then
  echo "Readability check failed with ${errors} error(s)."
  exit 1
fi

echo "All flowchart readability checks passed."
