#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Consumer performance smoke: one full repository scan with the pinned
# analyzer against the catastrophic-regression limits calibrated for this
# codebase. Deliberately single-sample — this is not performance
# certification; the analyzer's multi-sample certification runs in the
# github.com/invowk/goplint repository against a pinned reference corpus.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
POLICY_FILE="${1:-${GOPLINT_BENCH_SMOKE_POLICY:-$ROOT_DIR/.goplint/consumer-smoke.github-ubuntu-x64-4cpu.toml}}"
if [[ "$POLICY_FILE" != /* ]]; then
  POLICY_FILE="$ROOT_DIR/$POLICY_FILE"
fi

(cd "$ROOT_DIR" && go tool benchmark-policy -manifest "$POLICY_FILE" -policy consumer-smoke)

toml_section_value() {
  local section="$1"
  local key="$2"
  awk -F '=' -v section="[$section]" -v key="$key" '
    $0 == section { active=1; next }
    active && /^\[/ { exit }
    active && $1 ~ "^[[:space:]]*" key "[[:space:]]*$" {
      value=$2
      gsub(/^[[:space:]"]+|[[:space:]"]+$/, "", value)
      print value
      exit
    }
  ' "$POLICY_FILE"
}

compare_upper() {
  local name="$1"
  local metric="$2"
  local observed="$3"
  local limit="$4"
  if awk -v observed="$observed" -v limit="$limit" 'BEGIN { exit !(observed > limit) }'; then
    echo "$name catastrophic smoke regression: $metric $observed > $limit" >&2
    return 1
  fi
  echo "$name smoke $metric: $observed (catastrophic limit $limit)"
}

if [[ ! -x /usr/bin/time ]]; then
  echo "/usr/bin/time is required for repository smoke measurement" >&2
  exit 1
fi
"${SCRIPT_DIR}/goplint.sh" build
time_file="$(mktemp)"
trap 'rm -f "$time_file"' EXIT
set +e
(cd "$ROOT_DIR" && /usr/bin/time -f 'goplint-smoke-time %e %M' -o "$time_file" \
  ./bin/goplint -test=false -check-all -check-enum-sync \
  -baseline=.goplint/baseline.toml \
  -config=.goplint/exceptions.toml \
  ./cmd/... ./internal/... ./pkg/... >/dev/null 2>&1)
scan_status=$?
set -e
if [[ $scan_status -ne 0 && $scan_status -ne 3 ]]; then
  echo "canonical repository smoke scan failed" >&2
  exit 1
fi
read -r elapsed_seconds peak_kib < <(awk '$1 == "goplint-smoke-time" { print $2, $3 }' "$time_file")
wall_ms="$(awk -v seconds="$elapsed_seconds" 'BEGIN { printf "%.0f", seconds * 1000 }')"
peak_bytes="$((peak_kib * 1024))"
compare_upper repository_full_scan wall_ms "$wall_ms" "$(toml_section_value repository_full_scan max_wall_ms)"
compare_upper repository_full_scan peak_bytes "$peak_bytes" "$(toml_section_value repository_full_scan max_peak_bytes)"
