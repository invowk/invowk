#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

bencher_threshold_args() {
	local -n args_ref="$1"
	args_ref=(
		--threshold-measure latency
		--threshold-test percentage
		--threshold-min-sample-size 5
		--threshold-max-sample-size 64
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.10
		--threshold-measure memory
		--threshold-test percentage
		--threshold-min-sample-size 5
		--threshold-max-sample-size 64
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.10
		--threshold-measure allocations
		--threshold-test percentage
		--threshold-min-sample-size 5
		--threshold-max-sample-size 64
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.10
		--threshold-measure build-time
		--threshold-test percentage
		--threshold-min-sample-size 5
		--threshold-max-sample-size 32
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.20
		--threshold-measure file-size
		--threshold-test percentage
		--threshold-min-sample-size 2
		--threshold-max-sample-size 16
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.10
		--thresholds-reset
	)
}

bencher_error_on_alert_args() {
	local -n args_ref="$1"
	args_ref=(--error-on-alert)

	if [[ -z "${BENCHER_START_POINT:-}" ]]; then
		return 0
	fi

	local api_url="${BENCHER_API_URL:-https://api.bencher.dev/v0}"
	local reports_url="$api_url/projects/$BENCHER_PROJECT/reports?branch=$BENCHER_START_POINT&testbed=$BENCHER_TESTBED&sort=date_time&direction=desc&per_page=1"
	local latest_result_sets
	if ! latest_result_sets="$(curl -fsS "$reports_url" | jq -r 'if length == 0 then 0 else (.[0].results | length) end')"; then
		echo "::warning::Unable to inspect Bencher start-point reports; keeping --error-on-alert." >&2
		return 0
	fi

	if [[ "$latest_result_sets" == "0" ]]; then
		echo "::warning::Bencher start point '$BENCHER_START_POINT' has no latest benchmark results; uploading this report without --error-on-alert to avoid comparing against stale thresholds." >&2
		args_ref=()
	fi
}
