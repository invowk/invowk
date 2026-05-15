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
		--threshold-measure file-size
		--threshold-test percentage
		--threshold-min-sample-size 2
		--threshold-max-sample-size 16
		--threshold-lower-boundary _
		--threshold-upper-boundary 0.10
		--thresholds-reset
	)
}
