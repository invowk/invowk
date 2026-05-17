#!/bin/bash
# SPDX-License-Identifier: MPL-2.0
#
# Render all D2 diagrams to SVG.
#
# Usage:
#   ./scripts/render-diagrams.sh
#   ./scripts/render-diagrams.sh --allow-elk
#
# Production renders require D2 with the TALA layout engine. ELK rendering is
# available only as an explicit local preview path through --allow-elk or
# INVOWK_ALLOW_ELK_DIAGRAMS=1.
#
# Each SVG is stamped with a source path/hash comment so CI can detect stale or
# orphaned renders without requiring TALA.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

ALLOW_ELK="${INVOWK_ALLOW_ELK_DIAGRAMS:-0}"

usage() {
  cat <<'USAGE'
Usage: ./scripts/render-diagrams.sh [--allow-elk]

Options:
  --allow-elk   Render with ELK when TALA is not installed. This is for local
                preview only; production renders should use TALA.
  -h, --help    Show this help.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --allow-elk)
      ALLOW_ELK=1
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1"
      usage
      exit 1
      ;;
  esac
  shift
done

if ! command -v d2 >/dev/null 2>&1; then
  echo "ERROR: d2 is not installed."
  echo "Install it from: https://d2lang.com"
  exit 1
fi

tmp_dir="$(mktemp -d)"
expected_svgs="$tmp_dir/expected-svgs.txt"
tala_probe_err="$tmp_dir/tala-probe.err"
trap 'rm -rf "$tmp_dir"' EXIT
: > "$expected_svgs"

probe_tala() {
  local probe="$tmp_dir/tala-probe.d2"
  local output="$tmp_dir/tala-probe.svg"

  printf 'a -> b\n' > "$probe"
  d2 --layout=tala "$probe" "$output" > /dev/null 2> "$tala_probe_err"
}

tala_seed_supported() {
  d2 layout tala 2>&1 | grep -q -- '--tala-seeds'
}

layout="tala"
seed="none"
render_args=(--layout=tala)

if probe_tala; then
  echo "Using TALA layout engine (production)"
  if tala_seed_supported; then
    seed="100"
    render_args+=(--tala-seeds=100)
  fi
else
  if [ "$ALLOW_ELK" = "1" ]; then
    layout="elk"
    render_args=(--layout=elk)
    echo "WARNING: TALA is not available; using ELK because --allow-elk/INVOWK_ALLOW_ELK_DIAGRAMS was set."
    echo "WARNING: ELK renders are for local preview only."
  else
    echo "ERROR: TALA layout engine is required for production diagram rendering."
    echo ""
    echo "TALA probe failed:"
    sed 's/^/  /' "$tala_probe_err"
    echo ""
    echo "For local preview without TALA, run:"
    echo "  ./scripts/render-diagrams.sh --allow-elk"
    exit 1
  fi
fi

source_to_svg() {
  local d2file="$1"
  local rel dir name base

  rel="${d2file#docs/diagrams/}"
  dir="${rel%/*}"
  name="${d2file##*/}"
  base="${name%.d2}"
  printf 'docs/diagrams/rendered/%s/%s.svg\n' "$dir" "$base"
}

stamp_svg() {
  local source="$1"
  local svg="$2"
  local source_hash metadata tmp_svg

  source_hash="$(sha256sum "$source" | awk '{print $1}')"
  metadata="<!-- invowk-diagram-source: ${source} sha256:${source_hash} layout:${layout} seed:${seed} -->"
  tmp_svg="$tmp_dir/$(basename "$svg").stamped"

  awk -v metadata="$metadata" '
    NR == 1 {
      if (sub(/\?>/, "?>\n" metadata "\n")) {
        print
        next
      }
      print metadata
    }
    { print }
  ' "$svg" > "$tmp_svg"

  mv "$tmp_svg" "$svg"
}

render_one() {
  local d2file="$1"
  local out

  out="$(source_to_svg "$d2file")"
  printf '%s\n' "$out" >> "$expected_svgs"
  mkdir -p "$(dirname "$out")"

  printf 'Rendering: %s -> %s ... ' "$d2file" "$out"

  if ! d2 fmt "$d2file" >/dev/null; then
    echo "FAILED (fmt)"
    return 1
  fi

  if d2 "${render_args[@]}" "$d2file" "$out" >/dev/null; then
    stamp_svg "$d2file" "$out"
    echo "OK"
    return 0
  fi

  echo "FAILED (render)"
  return 1
}

total=0
success=0
failed=0

while IFS= read -r -d '' d2file; do
  total=$((total + 1))
  if render_one "$d2file"; then
    success=$((success + 1))
  else
    failed=$((failed + 1))
  fi
done < <(find docs/diagrams \
  -path docs/diagrams/rendered -prune -o \
  -path docs/diagrams/experiments -prune -o \
  -name "*.d2" -type f -print0 2>/dev/null | sort -z)

orphaned=0
while IFS= read -r -d '' svg; do
  if ! grep -Fxq "$svg" "$expected_svgs"; then
    echo "ERROR: orphaned rendered SVG without matching source: $svg"
    orphaned=$((orphaned + 1))
  fi
done < <(find docs/diagrams/rendered -name "*.svg" -type f -print0 2>/dev/null | sort -z)

echo ""
echo "=== Render Summary ==="
echo "Total:    $total"
echo "Success:  $success"
echo "Failed:   $failed"
echo "Orphaned: $orphaned"
echo "Layout:   $layout"
echo "Seed:     $seed"

if [ "$failed" -gt 0 ] || [ "$orphaned" -gt 0 ]; then
  echo ""
  echo "Some diagrams failed to render or orphaned SVGs were found."
  exit 1
fi

if [ "$total" -eq 0 ]; then
  echo ""
  echo "No D2 files found in docs/diagrams/"
  echo "Create .d2 files in docs/diagrams/{c4,sequences,flowcharts}/ to render them."
fi
