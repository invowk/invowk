#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Tests for scripts/ci-website-build.sh: exit-status passthrough, the memory
# defaults, the watchdog's process-group kill, signal forwarding, the sampler
# and its warnings.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_SCRIPT="$SCRIPT_DIR/ci-website-build.sh"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

fail() {
	printf 'FAIL: %s\n' "$1" >&2
	if [[ -n "${2:-}" ]]; then
		printf '  output:\n%s\n' "$2" >&2
	fi
	exit 1
}

assert_eq() {
	if [[ "$2" != "$3" ]]; then
		fail "$1 (expected '$2', got '$3')" "${4:-}"
	fi
}

assert_contains() {
	if [[ "$2" != *"$3"* ]]; then
		fail "$1 (missing '$3')" "$2"
	fi
}

# run_build sets OUT and RC from one run of the script with fast settings.
run_build() {
	RC=0
	OUT="$(env WEBSITE_BUILD_DIR="$TMP_DIR" WEBSITE_BUILD_SAMPLE_SECS=1 "$@" 2>&1)" || RC=$?
}

test_exit_status_passthrough() {
	run_build bash "$BUILD_SCRIPT" true
	assert_eq "success exits 0" 0 "$RC" "$OUT"
	assert_contains "prints a memory sample" "$OUT" "[website-build mem +0s] available="
	assert_contains "reports the exit" "$OUT" "[website-build] exit=0 after"

	run_build bash "$BUILD_SCRIPT" bash -c 'exit 3'
	assert_eq "failure status is kept" 3 "$RC" "$OUT"
	if [[ "$OUT" == *"::error"* ]]; then
		fail "an ordinary failure raises no annotation" "$OUT"
	fi
}

test_runs_in_website_dir() {
	run_build bash "$BUILD_SCRIPT" pwd
	assert_eq "pwd exits 0" 0 "$RC" "$OUT"
	assert_contains "runs in WEBSITE_BUILD_DIR" "$OUT" "$TMP_DIR"
}

test_memory_defaults() {
	# shellcheck disable=SC2016 # expanded by the command under test
	local probe='echo "probe=$DOCUSAURUS_NO_PERSISTENT_CACHE:$MIMALLOC_PURGE_DELAY"'

	run_build env -u DOCUSAURUS_NO_PERSISTENT_CACHE -u MIMALLOC_PURGE_DELAY bash "$BUILD_SCRIPT" bash -c "$probe"
	assert_contains "sets the memory defaults" "$OUT" "probe=true:0"

	run_build MIMALLOC_PURGE_DELAY=25 bash "$BUILD_SCRIPT" bash -c "$probe"
	assert_contains "keeps a preset value" "$OUT" "probe=true:25"
}

test_watchdog_kills_process_group() {
	local pid_file="$TMP_DIR/grandchild.pid"
	local grandchild

	# shellcheck disable=SC2016 # expanded by the command under test
	run_build WEBSITE_BUILD_TIMEOUT_SECS=1 WEBSITE_BUILD_KILL_AFTER_SECS=2 \
		bash "$BUILD_SCRIPT" bash -c 'sleep 60 & echo $! > "$0"; wait' "$pid_file"
	assert_eq "watchdog exit status" 124 "$RC" "$OUT"
	assert_contains "watchdog annotation" "$OUT" "::error title=Website build watchdog::"
	grandchild="$(cat "$pid_file")"
	if kill -0 "$grandchild" 2>/dev/null; then
		kill "$grandchild" 2>/dev/null || true
		fail "watchdog left grandchild $grandchild running" "$OUT"
	fi

	# A command that ignores SIGTERM (inherited by its children) needs SIGKILL.
	run_build WEBSITE_BUILD_TIMEOUT_SECS=1 WEBSITE_BUILD_KILL_AFTER_SECS=1 \
		bash "$BUILD_SCRIPT" bash -c 'trap "" TERM; sleep 60'
	assert_eq "kill-after exit status" 137 "$RC" "$OUT"
}

test_low_memory_warnings_are_capped() {
	local count

	run_build WEBSITE_BUILD_LOW_MEM_MIB=999999999 bash "$BUILD_SCRIPT" sleep 5
	assert_eq "sleep exits 0" 0 "$RC" "$OUT"
	count="$(grep -c '^::warning title=Website build memory::' <<<"$OUT" || true)"
	assert_eq "at most three low-memory warnings" 3 "$count" "$OUT"
}

test_output_closes_promptly() {
	local started=$SECONDS

	# With a 10 s interval, an orphaned sampler `sleep` would hold the output
	# pipe open (and a runner step with it) for up to 10 s after the build.
	RC=0
	OUT="$(env WEBSITE_BUILD_DIR="$TMP_DIR" WEBSITE_BUILD_SAMPLE_SECS=10 bash "$BUILD_SCRIPT" true 2>&1)" || RC=$?
	assert_eq "exits 0" 0 "$RC" "$OUT"
	if ((SECONDS - started > 5)); then
		fail "output stayed open for $((SECONDS - started)) s after the build" "$OUT"
	fi
}

test_signal_stops_build_group() {
	local sig want pid_file grandchild wrapper rc

	for sig in TERM INT; do
		want=143
		[[ "$sig" == INT ]] && want=130
		pid_file="$TMP_DIR/signal-$sig.pid"
		rm -f "$pid_file"
		# env --default-signal undoes the SIGINT ignore that a non-interactive
		# shell gives background jobs; the runner signals a foreground step.
		# shellcheck disable=SC2016 # expanded by the command under test
		env --default-signal=INT WEBSITE_BUILD_DIR="$TMP_DIR" WEBSITE_BUILD_SAMPLE_SECS=1 \
			WEBSITE_BUILD_KILL_AFTER_SECS=2 \
			bash "$BUILD_SCRIPT" bash -c 'sleep 60 & echo $! > "$0"; wait' "$pid_file" \
			>"$TMP_DIR/signal-$sig.out" 2>&1 &
		wrapper=$!
		for _ in $(seq 50); do
			[[ -s "$pid_file" ]] && break
			sleep 0.1
		done
		[[ -s "$pid_file" ]] || fail "$sig: the build never started" "$(cat "$TMP_DIR/signal-$sig.out")"
		kill "-$sig" "$wrapper"
		rc=0
		wait "$wrapper" || rc=$?
		assert_eq "$sig exit status" "$want" "$rc" "$(cat "$TMP_DIR/signal-$sig.out")"
		grandchild="$(cat "$pid_file")"
		if kill -0 "$grandchild" 2>/dev/null; then
			kill "$grandchild" 2>/dev/null || true
			fail "$sig left build process $grandchild running after the wrapper exited" "$(cat "$TMP_DIR/signal-$sig.out")"
		fi
	done
}

test_exit_status_passthrough
test_runs_in_website_dir
test_memory_defaults
test_watchdog_kills_process_group
test_signal_stops_build_group
test_low_memory_warnings_are_capped
test_output_closes_promptly

echo "ci-website-build tests passed"
