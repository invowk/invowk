#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

MODE="short"
OUT_DIR="docs/benchmarks"
BINARY="./bin/invowk"
GO_CMD="${GOCMD:-go}"
STARTUP_SAMPLES="${STARTUP_SAMPLES:-40}"
BENCH_COUNT="${BENCH_COUNT:-5}"
SHORT_BENCH_REGEX='^Benchmark(CUEParsing|CUEParsingComplex|InvowkmodParsing|Discovery.*|ModuleValidation|FullPipeline)$'

usage() {
	cat <<'EOF'
Usage: scripts/bench-report.sh [--mode short|full] [--out-dir <dir>] [--binary <path>]

Generates a benchmark report with:
  1) CLI startup timing scenarios
  2) Go benchmarks from ./internal/benchmark

Options:
  --mode      short (default) or full
  --out-dir   output directory for markdown reports (default: docs/benchmarks)
  --binary    invowk binary path (default: ./bin/invowk)
  -h, --help  show this help text

Environment:
  STARTUP_SAMPLES  Startup timing samples per scenario (default: 40)
  BENCH_COUNT      Go benchmark run count (default: 5)
  GOCMD            Go command override (default: go)
  TAG              Release tag for asset metadata (optional)
  BENCH_HISTORY_JSON Existing aggregate history JSON for comparisons (optional)
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--mode)
			if [[ $# -lt 2 ]]; then
				echo "Error: --mode requires a value (short|full)." >&2
				exit 1
			fi
			MODE="$2"
			shift 2
			;;
		--out-dir)
			if [[ $# -lt 2 ]]; then
				echo "Error: --out-dir requires a value." >&2
				exit 1
			fi
			OUT_DIR="$2"
			shift 2
			;;
		--binary)
			if [[ $# -lt 2 ]]; then
				echo "Error: --binary requires a value." >&2
				exit 1
			fi
			BINARY="$2"
			shift 2
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "Error: unknown argument: $1" >&2
			usage
			exit 1
			;;
	esac
done

if [[ "$MODE" != "short" && "$MODE" != "full" ]]; then
	echo "Error: invalid --mode '$MODE'. Expected 'short' or 'full'." >&2
	exit 1
fi

if ! [[ "$STARTUP_SAMPLES" =~ ^[1-9][0-9]*$ ]]; then
	echo "Error: STARTUP_SAMPLES must be a positive integer. Got: $STARTUP_SAMPLES" >&2
	exit 1
fi

if ! [[ "$BENCH_COUNT" =~ ^[1-9][0-9]*$ ]]; then
	echo "Error: BENCH_COUNT must be a positive integer. Got: $BENCH_COUNT" >&2
	exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "$BINARY" = /* ]]; then
	BINARY_PATH="$BINARY"
else
	BINARY_PATH="$ROOT_DIR/$BINARY"
fi

if [[ ! -x "$BINARY_PATH" ]]; then
	echo "Error: invowk binary not found or not executable at: $BINARY_PATH" >&2
	echo "Hint: run 'make build' first." >&2
	exit 1
fi

if ! command -v "$GO_CMD" >/dev/null 2>&1; then
	echo "Error: Go command not found: $GO_CMD" >&2
	exit 1
fi

date_probe="$(date +%s%N)"
if ! [[ "$date_probe" =~ ^[0-9]+$ ]]; then
	echo "Error: date +%s%N is required for startup timing measurements." >&2
	exit 1
fi

mkdir -p "$OUT_DIR"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

startup_rows_tsv="$tmp_dir/startup_rows.tsv"
go_bench_raw="$tmp_dir/go_bench_raw.txt"
go_bench_rows_tsv="$tmp_dir/go_bench_rows.tsv"
go_bench_summary_tsv="$tmp_dir/go_bench_summary.tsv"

: >"$startup_rows_tsv"
: >"$go_bench_rows_tsv"
: >"$go_bench_summary_tsv"

run_startup_scenario() {
	local scenario_id="$1"
	local label="$2"
	shift 2

	local ns_file="$tmp_dir/startup_${scenario_id}.ns"
	: >"$ns_file"

	# One warm-up run reduces one-time initialization noise.
	"$BINARY_PATH" "$@" >/dev/null 2>&1

	local i start_ns end_ns elapsed_ns
	for ((i = 1; i <= STARTUP_SAMPLES; i++)); do
		start_ns="$(date +%s%N)"
		"$BINARY_PATH" "$@" >/dev/null 2>&1
		end_ns="$(date +%s%N)"
		elapsed_ns="$((end_ns - start_ns))"
		printf '%s\n' "$elapsed_ns" >>"$ns_file"
	done

	local stats mean_ms min_ms max_ms
	stats="$(
		awk '
			{
				sum += $1
				if (NR == 1 || $1 < min) min = $1
				if ($1 > max) max = $1
			}
			END {
				printf "%.2f\t%.2f\t%.2f", (sum / NR) / 1000000, min / 1000000, max / 1000000
			}
		' "$ns_file"
	)"

	IFS=$'\t' read -r mean_ms min_ms max_ms <<<"$stats"

	printf "%-28s %12.2f %12.2f %12.2f %10d\n" "$label" "$mean_ms" "$min_ms" "$max_ms" "$STARTUP_SAMPLES"
	printf "%s\t%s\t%s\t%s\t%s\n" "$label" "$mean_ms" "$min_ms" "$max_ms" "$STARTUP_SAMPLES" >>"$startup_rows_tsv"
}

echo "Running startup timing scenarios ($STARTUP_SAMPLES samples each)..."
printf "%-28s %12s %12s %12s %10s\n" "Scenario" "Mean (ms)" "Min (ms)" "Max (ms)" "Samples"
printf "%-28s %12s %12s %12s %10s\n" "----------------------------" "----------" "---------" "---------" "-------"

run_startup_scenario "version" "Version (--version)" --version
run_startup_scenario "help" "Help (--help)" --help
run_startup_scenario "cmd_help" "Cmd Help (cmd --help)" cmd --help
run_startup_scenario "cmd_list" "Cmd List (cmd)" cmd

echo ""
echo "Running Go benchmarks from ./internal/benchmark (mode: $MODE, count: $BENCH_COUNT)..."

go_bench_cmd=("$GO_CMD" "test" "-run=^$" "-benchmem" "-count=$BENCH_COUNT")
if [[ "$MODE" == "short" ]]; then
	go_bench_cmd+=("-bench=$SHORT_BENCH_REGEX")
else
	go_bench_cmd+=("-bench=.")
fi
go_bench_cmd+=("./internal/benchmark/")

set +e
"${go_bench_cmd[@]}" >"$go_bench_raw" 2>&1
go_bench_exit=$?
set -e

awk '
	/^Benchmark/ {
		name = $1
		iterations = $2
		ns = "-"
		bytes = "-"
		allocs = "-"

		for (i = 3; i <= NF; i++) {
			if ($i == "ns/op" && i > 1) {
				ns = $(i - 1)
			} else if ($i == "B/op" && i > 1) {
				bytes = $(i - 1)
			} else if ($i == "allocs/op" && i > 1) {
				allocs = $(i - 1)
			}
		}

		if (ns != "-") {
			printf "%s\t%s\t%s\t%s\t%s\n", name, iterations, ns, bytes, allocs
		}
	}
' "$go_bench_raw" >"$go_bench_rows_tsv"

if [[ ! -s "$go_bench_rows_tsv" ]]; then
	cat "$go_bench_raw"
	echo ""
	echo "Error: no benchmark rows were parsed from Go benchmark output." >&2
	exit 1
fi

go_bench_status="ok"
if [[ "$go_bench_exit" -ne 0 ]]; then
	go_bench_status="partial (command failed with exit code $go_bench_exit)"
	echo "Warning: Go benchmark command exited with code $go_bench_exit."
	echo "         Reporting parsed benchmark rows that completed before the failure."
	echo ""
fi

awk '
	BEGIN {
		FS = "\t"
		OFS = "\t"
	}
	function is_number(v) {
		return v ~ /^-?[0-9]+([.][0-9]+)?$/
	}
	{
		name = $1
		iterations = $2
		ns = $3
		bytes = $4
		allocs = $5

		if (!(name in seen)) {
			seen[name] = 1
			order[++order_count] = name
		}

		sample_count[name]++

		if (is_number(ns)) {
			ns_value = ns + 0
			valid_ns_count[name]++
			sum_ns[name] += ns_value
			if (!(name in min_ns) || ns_value < min_ns[name]) {
				min_ns[name] = ns_value
			}
			if (!(name in max_ns) || ns_value > max_ns[name]) {
				max_ns[name] = ns_value
			}
		}

		if (iterations ~ /^[0-9]+$/ && is_number(ns)) {
			run_ns = (iterations + 0) * (ns + 0)
			sum_run_ns[name] += run_ns
			sum_iterations[name] += (iterations + 0)
			valid_timing_count[name]++
		}

		if (bytes != "-" && is_number(bytes)) {
			sum_bytes[name] += bytes + 0
			bytes_count[name]++
		}
		if (allocs != "-" && is_number(allocs)) {
			sum_allocs[name] += allocs + 0
			allocs_count[name]++
		}
	}
	END {
		for (i = 1; i <= order_count; i++) {
			name = order[i]
			mean_ns = "-"
			min_ns_out = "-"
			max_ns_out = "-"
			mean_ms_per_op = "-"
			mean_iters_per_run = "-"
			est_run_ms = "-"
			est_total_s = "-"
			mean_bytes = "-"
			mean_allocs = "-"

			if (valid_ns_count[name] > 0) {
				mean_ns_value = sum_ns[name] / valid_ns_count[name]
				mean_ns = sprintf("%.2f", mean_ns_value)
				min_ns_out = sprintf("%.2f", min_ns[name])
				max_ns_out = sprintf("%.2f", max_ns[name])
				mean_ms_per_op = sprintf("%.6f", mean_ns_value / 1000000)
			}
			if (valid_timing_count[name] > 0) {
				mean_iters_per_run = sprintf("%.2f", sum_iterations[name] / valid_timing_count[name])
				est_run_ms = sprintf("%.3f", (sum_run_ns[name] / valid_timing_count[name]) / 1000000)
				est_total_s = sprintf("%.6f", sum_run_ns[name] / 1000000000)
			}
			if (bytes_count[name] > 0) {
				mean_bytes = sprintf("%.2f", sum_bytes[name] / bytes_count[name])
			}
			if (allocs_count[name] > 0) {
				mean_allocs = sprintf("%.2f", sum_allocs[name] / allocs_count[name])
			}
			printf "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", \
				name, sample_count[name], mean_ns, min_ns_out, max_ns_out, mean_ms_per_op, mean_iters_per_run, est_run_ms, est_total_s, mean_bytes, mean_allocs
		}
	}
' "$go_bench_rows_tsv" >"$go_bench_summary_tsv"

go_bench_timing_unavailable_count="$(awk -F '\t' '$9 == "-" {count++} END {print count + 0}' "$go_bench_summary_tsv")"
go_bench_est_total_s="$(awk -F '\t' '$9 ~ /^-?[0-9]+([.][0-9]+)?$/ {sum += $9; count++} END {if (count > 0) printf "%.6f", sum}' "$go_bench_summary_tsv")"
if [[ -z "$go_bench_est_total_s" ]]; then
	go_bench_est_total_s="unknown"
fi

go_bench_wall_s="$(awk '
	/^(ok|FAIL)[[:space:]]/ {
		value = $NF
		sub(/s$/, "", value)
		if (value ~ /^[0-9]+([.][0-9]+)?$/) {
			wall = value
		}
	}
	END {
		if (wall != "") {
			print wall
		}
	}
' "$go_bench_raw")"
if [[ -z "$go_bench_wall_s" ]]; then
	go_bench_wall_s="unknown"
fi

go_bench_overhead_s="unknown"
if [[ "$go_bench_wall_s" =~ ^[0-9]+([.][0-9]+)?$ && "$go_bench_est_total_s" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
	go_bench_overhead_s="$(
		awk -v wall="$go_bench_wall_s" -v est="$go_bench_est_total_s" '
			BEGIN {
				delta = wall - est
				if (delta < 0) {
					print "unknown"
				} else {
					printf "%.6f", delta
				}
			}
		'
	)"
fi

go_bench_timing_scope="all parsed benchmark rows"
if [[ "$go_bench_exit" -ne 0 ]]; then
	go_bench_timing_scope="parsed rows only (benchmark command exited early)"
fi

printf "%-42s %7s %12s %12s %12s %12s %15s %13s %13s %12s %14s\n" \
	"Benchmark" "Samples" "Mean ns/op" "Min ns/op" "Max ns/op" "Mean ms/op" "Mean iters/run" "Est run (ms)" "Est total (s)" "Mean B/op" "Mean allocs/op"
printf "%-42s %7s %12s %12s %12s %12s %15s %13s %13s %12s %14s\n" \
	"------------------------------------------" "-------" "----------" "---------" "---------" "----------" "--------------" "------------" "------------" "---------" "--------------"
while IFS=$'\t' read -r name samples mean_ns min_ns max_ns mean_ms mean_iters est_run_ms est_total_s mean_bytes mean_allocs; do
	printf "%-42s %7s %12s %12s %12s %12s %15s %13s %13s %12s %14s\n" \
		"$name" "$samples" "$mean_ns" "$min_ns" "$max_ns" "$mean_ms" "$mean_iters" "$est_run_ms" "$est_total_s" "$mean_bytes" "$mean_allocs"
done <"$go_bench_summary_tsv"

echo ""
echo "Go benchmark timing summary:"
echo "  Wall time (s): $go_bench_wall_s"
echo "  Estimated loop time (s): $go_bench_est_total_s"
echo "  Harness/overhead (s): $go_bench_overhead_s"
echo "  Timing scope: $go_bench_timing_scope"
if [[ "$go_bench_timing_unavailable_count" -gt 0 ]]; then
	echo "  Note: $go_bench_timing_unavailable_count benchmark row(s) had incomplete timing data."
fi

generated_at_utc="$(date -u +"%Y-%m-%d %H:%M:%S UTC")"
report_stamp="$(date -u +"%Y-%m-%d_%H-%M-%S")"
git_branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")"
git_commit="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
go_version="$($GO_CMD version 2>/dev/null || echo "unknown")"
platform="$(uname -srm)"
go_bench_command="${go_bench_cmd[*]}"

render_args=(
	"scripts/benchmark-report.mjs" "render"
	"--out-dir" "$OUT_DIR"
	"--stamp" "$report_stamp"
	"--mode" "$MODE"
	"--startup-samples" "$STARTUP_SAMPLES"
	"--go-bench-count" "$BENCH_COUNT"
	"--go-bench-status" "$go_bench_status"
	"--go-bench-wall" "$go_bench_wall_s"
	"--go-bench-est-total" "$go_bench_est_total_s"
	"--go-bench-overhead" "$go_bench_overhead_s"
	"--go-bench-timing-scope" "$go_bench_timing_scope"
	"--startup-tsv" "$startup_rows_tsv"
	"--go-summary-tsv" "$go_bench_summary_tsv"
	"--go-raw" "$go_bench_raw"
	"--branch" "$git_branch"
	"--commit" "$git_commit"
	"--platform" "$platform"
	"--go-version" "$go_version"
	"--binary" "$BINARY_PATH"
	"--go-bench-command" "$go_bench_command"
	"--generated-at" "$generated_at_utc"
)

if [[ -n "${TAG:-}" ]]; then
	render_args+=("--tag" "$TAG")
fi

if [[ -n "${BENCH_HISTORY_JSON:-}" ]]; then
	render_args+=("--history" "$BENCH_HISTORY_JSON")
fi

echo ""
node "${render_args[@]}"
