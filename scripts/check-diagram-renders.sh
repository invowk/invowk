#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Validate rendered D2 SVG manifests.
#
# This check intentionally does not render diagrams. It verifies that each D2
# source has a committed SVG, every committed SVG maps back to a source, and
# each SVG carries a source hash stamped by scripts/render-diagrams.sh.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

tmp_dir="$(mktemp -d)"
expected_svgs="$tmp_dir/expected-svgs.txt"
trap 'rm -rf "$tmp_dir"' EXIT
: > "$expected_svgs"

source_to_svg() {
  local d2file="$1"
  local rel dir name base

  rel="${d2file#docs/diagrams/}"
  dir="${rel%/*}"
  name="${d2file##*/}"
  base="${name%.d2}"
  printf 'docs/diagrams/rendered/%s/%s.svg\n' "$dir" "$base"
}

errors=0
total=0

echo "Checking rendered SVG manifests..."

while IFS= read -r -d '' d2file; do
  total=$((total + 1))
  svg="$(source_to_svg "$d2file")"
  printf '%s\n' "$svg" >> "$expected_svgs"

  printf '  %s -> %s ... ' "$d2file" "$svg"

  if [ ! -f "$svg" ]; then
    echo "MISSING"
    errors=$((errors + 1))
    continue
  fi

  source_hash="$(sha256sum "$d2file" | awk '{print $1}')"
  manifest="invowk-diagram-source: ${d2file} sha256:${source_hash}"

  if grep -Fq "$manifest" "$svg"; then
    echo "OK"
  else
    echo "STALE"
    echo "    Expected manifest containing: $manifest"
    errors=$((errors + 1))
  fi
done < <(find docs/diagrams \
  -path docs/diagrams/rendered -prune -o \
  -path docs/diagrams/experiments -prune -o \
  -name "*.d2" -type f -print0 2>/dev/null | sort -z)

while IFS= read -r -d '' svg; do
  if ! grep -Fxq "$svg" "$expected_svgs"; then
    echo "  ORPHAN: $svg"
    errors=$((errors + 1))
  fi
done < <(find docs/diagrams/rendered -name "*.svg" -type f -print0 2>/dev/null | sort -z)

echo ""
echo "Summary: $total D2 source file(s), $errors render manifest error(s)"

if [ "$total" -eq 0 ]; then
  echo "ERROR: no D2 files found in docs/diagrams/."
  exit 1
fi

if [ "$errors" -gt 0 ]; then
  echo ""
  echo "Run: make render-diagrams"
  echo "For local preview without TALA, run: ./scripts/render-diagrams.sh --allow-elk"
  exit 1
fi

echo "All rendered SVG manifests are current."
