#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

TAG="${TAG:-}"
BENCH_REPORT_OUT_DIR="${BENCH_REPORT_OUT_DIR:-}"
RELEASE_ASSETS_DIR="${RELEASE_ASSETS_DIR:-release-assets}"

if [[ -z "$TAG" ]]; then
	echo "Error: TAG must be set to the release tag (for example, v1.2.3)." >&2
	exit 1
fi

if [[ -z "$BENCH_REPORT_OUT_DIR" ]]; then
	echo "Error: BENCH_REPORT_OUT_DIR must be set." >&2
	exit 1
fi

rm -rf "$BENCH_REPORT_OUT_DIR"
mkdir -p "$BENCH_REPORT_OUT_DIR"
make bench-report

mapfile -t reports < <(find "$BENCH_REPORT_OUT_DIR" -maxdepth 1 -type f -name '*.md' | sort)
count="${#reports[@]}"
if [[ "$count" -ne 1 ]]; then
	echo "Error: expected exactly 1 benchmark report in '$BENCH_REPORT_OUT_DIR', found $count." >&2
	if [[ "$count" -gt 0 ]]; then
		printf 'Found files:\n%s\n' "${reports[@]}" >&2
	fi
	exit 1
fi

report="${reports[0]}"
version="${TAG#v}"
asset_name="invowk_${version}_bench-report.md"
asset_path="${RELEASE_ASSETS_DIR}/${asset_name}"

rm -rf "$RELEASE_ASSETS_DIR"
mkdir -p "$RELEASE_ASSETS_DIR"
cp "$report" "$asset_path"

echo "Benchmark report staged: $asset_path"
echo "Source report: $report"
