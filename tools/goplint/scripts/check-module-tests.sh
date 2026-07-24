#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${MODULE_DIR}"
echo "Running the goplint module test suite..."
(
  # Evidence-emitting tests must not publish into this subgate's own report.
  unset GOPLINT_SOUNDNESS_EVIDENCE_DIR
  unset GOPLINT_SOUNDNESS_SUBGATE_REPORT_PATH
  GOCACHE="${GOCACHE:-/tmp/go-build}" "${SCRIPT_DIR}/soundness-go-test.sh" -count=1 ./...
)
go run ./cmd/subgate-report -observation module-test-suites=goplint-module-tests
