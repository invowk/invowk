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

if [[ -n "${BENCH_HISTORY_JSON:-}" && ! -s "$BENCH_HISTORY_JSON" ]]; then
	echo "Error: BENCH_HISTORY_JSON is set but missing or empty: $BENCH_HISTORY_JSON" >&2
	exit 1
fi

TAG="$TAG" BENCH_HISTORY_JSON="${BENCH_HISTORY_JSON:-}" make bench-report

mapfile -t json_reports < <(find "$BENCH_REPORT_OUT_DIR" -maxdepth 1 -type f -name '*.json' | sort)
count="${#json_reports[@]}"
if [[ "$count" -ne 1 ]]; then
	echo "Error: expected exactly 1 benchmark JSON report in '$BENCH_REPORT_OUT_DIR', found $count." >&2
	if [[ "$count" -gt 0 ]]; then
		printf 'Found files:\n%s\n' "${json_reports[@]}" >&2
	fi
	exit 1
fi

json_report="${json_reports[0]}"
stem="${json_report%.json}"
markdown_report="${stem}.md"
svg_report="${stem}_summary.svg"
raw_report="${stem}_raw.txt"

for expected in "$markdown_report" "$json_report" "$svg_report" "$raw_report"; do
	if [[ ! -s "$expected" ]]; then
		echo "Error: expected benchmark asset is missing or empty: $expected" >&2
		exit 1
	fi
done

version="${TAG#v}"
md_asset_name="invowk_${version}_bench-report.md"
json_asset_name="invowk_${version}_bench-report.json"
svg_asset_name="invowk_${version}_bench-summary.svg"
raw_asset_name="invowk_${version}_bench-raw.txt"

rm -rf "$RELEASE_ASSETS_DIR"
mkdir -p "$RELEASE_ASSETS_DIR"
cp "$markdown_report" "${RELEASE_ASSETS_DIR}/${md_asset_name}"
cp "$json_report" "${RELEASE_ASSETS_DIR}/${json_asset_name}"
cp "$svg_report" "${RELEASE_ASSETS_DIR}/${svg_asset_name}"
cp "$raw_report" "${RELEASE_ASSETS_DIR}/${raw_asset_name}"

node scripts/benchmark-report.mjs validate-assets --dir "$RELEASE_ASSETS_DIR" --layout release --tag "$TAG"

if [[ -n "${BENCH_HISTORY_JSON:-}" ]]; then
	if ! grep -Fq "## Performance Evolution" "${RELEASE_ASSETS_DIR}/${md_asset_name}"; then
		echo "Error: history-aware benchmark report is missing the Performance Evolution section." >&2
		exit 1
	fi
fi

echo "Benchmark report staged: ${RELEASE_ASSETS_DIR}/${md_asset_name}"
echo "Benchmark JSON staged: ${RELEASE_ASSETS_DIR}/${json_asset_name}"
echo "Benchmark SVG staged: ${RELEASE_ASSETS_DIR}/${svg_asset_name}"
echo "Benchmark raw output staged: ${RELEASE_ASSETS_DIR}/${raw_asset_name}"
echo "Source report stem: $stem"
