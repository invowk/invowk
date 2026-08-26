#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Consumer-side routed goplint gate: classify the staged diff through the
# two-class ownership manifest (.goplint/ownership.v1.json). A diff whose
# every path matches a documentation pattern skips analyzer execution;
# any other path — and any classification failure, fail-closed — runs the
# consumer tier (one canonical repository audit with baseline and exception
# governance against the pinned analyzer). Analyzer-semantics and harness
# assurance are governed by the github.com/invowk/goplint repository.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
MANIFEST="$ROOT_DIR/.goplint/ownership.v1.json"

run_consumer_tier() {
  echo "Selected goplint profile: consumer"
  audit_dir="$(mktemp -d)"
  trap 'rm -rf "$audit_dir"' EXIT
  # One canonical scan serves every consumer verdict below.
  export GOPLINT_REPOSITORY_AUDIT_PATH="$audit_dir/repository-audit.json"
  make -C "$ROOT_DIR" check-goplint-repository-audit
  make -C "$ROOT_DIR" check-baseline
  make -C "$ROOT_DIR" check-goplint-exceptions
  make -C "$ROOT_DIR" check-goplint-full-scan
  make -C "$ROOT_DIR" check-goplint-performance-smoke
}

if ! command -v jq >/dev/null 2>&1 || [[ ! -f "$MANIFEST" ]]; then
  # Fail closed: without classification machinery, run the consumer tier.
  run_consumer_tier
  exit 0
fi

changed_paths="$(cd "$ROOT_DIR" && git diff --cached --name-only)"
if [[ -z "$changed_paths" ]]; then
  changed_paths="$(cd "$ROOT_DIR" && git diff --name-only HEAD 2>/dev/null || true)"
fi
if [[ -z "$changed_paths" ]]; then
  # Empty or unknowable diff fails closed to the consumer tier.
  run_consumer_tier
  exit 0
fi

mapfile -t doc_patterns < <(jq -r '.documentation_patterns[]' "$MANIFEST")
all_documentation=1
while IFS= read -r path; do
  [[ -z "$path" ]] && continue
  matched=0
  for pattern in "${doc_patterns[@]}"; do
    if [[ "$path" =~ $pattern ]]; then
      matched=1
      break
    fi
  done
  if [[ "$matched" -eq 0 ]]; then
    all_documentation=0
    break
  fi
done <<<"$changed_paths"

if [[ "$all_documentation" -eq 1 ]]; then
  echo "Selected goplint profile: documentation (no analyzer execution)"
  exit 0
fi
run_consumer_tier
