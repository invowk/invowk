#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# CI entry point for the Docusaurus website build (test-website.yml,
# deploy-website.yml, version-docs.yml). Usage, from any directory:
#
#   scripts/ci-website-build.sh [command...]      # default: npm run build
#
# The command runs in website/ with three additions:
#
# 1. Bounded memory. A two-locale build on a 4-CPU host peaked at 13.1-15.3 GiB
#    of resident memory (maxrss up to 15.8 GiB), against the 16 GB of an
#    ubuntu-latest runner; that is how CI runs died mid-step with no log.
#    Almost all of it is Rspack's native heap (mimalloc), which keeps the first
#    locale's pages while the second locale bundles, plus its persistent cache.
#    MIMALLOC_PURGE_DELAY=0 makes mimalloc return freed pages at once and
#    DOCUSAURUS_NO_PERSISTENT_CACHE skips the cache, which CI never restores.
#    Together they brought the peak to 9.7-10.0 GiB in three of four runs
#    (13.2 GiB in the fourth) at the same build time. Each locale building in
#    its own process would bound it further, but `--locale X` also drops the
#    /X/ baseUrl, so that needs explicit i18n.localeConfigs baseUrls first.
#    This lowers the risk; it does not remove it. Values already set in the
#    environment win; Docusaurus treats any non-empty
#    DOCUSAURUS_NO_PERSISTENT_CACHE (even "false" or "0") as set, so only an
#    empty value turns the cache back on.
# 2. A watchdog. The command runs under `timeout` in its own process group,
#    which is killed after WEBSITE_BUILD_TIMEOUT_SECS (default 600, under the
#    15-minute job limit; release builds took 178-272 s at v0.14-v0.17 and
#    grow with every docs version), so a hang fails the step and its log is
#    kept.
# 3. A memory sampler. Every WEBSITE_BUILD_SAMPLE_SECS (default 10) it prints
#    available memory, swap and the largest processes, and it raises a
#    `::warning::` annotation (at most three) when available memory drops
#    below WEBSITE_BUILD_LOW_MEM_MIB (default 1536). Annotations travel with
#    the step status, so they may survive a runner that dies before its log
#    is uploaded (not verified).
#
# The exit status is the command's own; 124 or 137 after a watchdog kill.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEBSITE_DIR="${WEBSITE_BUILD_DIR:-$SCRIPT_DIR/../website}"

timeout_secs="${WEBSITE_BUILD_TIMEOUT_SECS:-600}"
kill_after_secs="${WEBSITE_BUILD_KILL_AFTER_SECS:-30}"
sample_secs="${WEBSITE_BUILD_SAMPLE_SECS:-10}"
low_mem_mib="${WEBSITE_BUILD_LOW_MEM_MIB:-1536}"

export DOCUSAURUS_NO_PERSISTENT_CACHE="${DOCUSAURUS_NO_PERSISTENT_CACHE-true}"
export MIMALLOC_PURGE_DELAY="${MIMALLOC_PURGE_DELAY-0}"

if (($# == 0)); then
	set -- npm run build
fi

# meminfo_mib KEY prints the /proc/meminfo field KEY in MiB (0 when absent).
meminfo_mib() {
	awk -v key="$1:" '$1 == key { printf "%d\n", $2 / 1024; found = 1 } END { if (!found) print 0 }' /proc/meminfo
}

# top_processes prints the three largest processes by RSS, "comm=MiB" each.
top_processes() {
	ps -eo rss=,comm= --sort=-rss 2>/dev/null | head -n 3 |
		awk '{ printf "%s%s=%dMiB", sep, $2, $1 / 1024; sep = " " }'
}

# memory_line ELAPSED prints one sample and returns 1 when available memory is
# below the warning threshold.
memory_line() {
	local avail swap_used top

	avail="$(meminfo_mib MemAvailable)"
	swap_used=$(($(meminfo_mib SwapTotal) - $(meminfo_mib SwapFree)))
	top="$(top_processes || true)"
	printf '[website-build mem +%ds] available=%sMiB swap_used=%sMiB top: %s\n' \
		"$1" "$avail" "$swap_used" "$top"
	if ((avail < low_mem_mib)); then
		LOW_MEM_DETAIL="$avail MiB available at +$1 s (swap used $swap_used MiB); largest: $top"
		return 1
	fi
}

# sample_memory loops forever, one sample per interval. It runs in the
# background and is killed when the command ends.
sample_memory() {
	# A failed sample (ps hit by SIGPIPE, say) must not end the sampler.
	set +e +o pipefail
	local warnings=0

	# The sleep runs as a background job that the TERM trap kills, so no
	# sleep outlives the sampler and holds the step's stdout open.
	local sleep_pid=""
	trap 'kill "$sleep_pid" 2>/dev/null; exit 0' TERM
	while :; do
		sleep "$sample_secs" &
		sleep_pid=$!
		wait "$sleep_pid"
		if ! memory_line $((SECONDS - started)) && ((warnings < 3)); then
			warnings=$((warnings + 1))
			printf '::warning title=Website build memory::%s\n' "$LOW_MEM_DETAIL"
		fi
	done
}

sampler_pid=""
build_pid=""

cleanup() {
	local children

	if [[ -n "$sampler_pid" ]]; then
		# Also stop the sampler's `sleep`: an orphan still holds the step's
		# stdout, and the runner waits for it to close.
		children="$(pgrep -P "$sampler_pid" 2>/dev/null || true)"
		kill "$sampler_pid" 2>/dev/null || true
		if [[ -n "$children" ]]; then
			# shellcheck disable=SC2086 # one PID per word
			kill $children 2>/dev/null || true
		fi
		wait "$sampler_pid" 2>/dev/null || true
		sampler_pid=""
	fi
}

# forward_signal SIG stops the command's whole process group, waits for it
# (bounded: timeout arms its --kill-after alarm on an external TERM too), then
# exits with the signal's conventional status.
# shellcheck disable=SC2329 # invoked by the INT/TERM traps
forward_signal() {
	local status=143
	[[ "$1" == INT ]] && status=130
	if [[ -n "$build_pid" ]]; then
		kill -TERM -- "-$build_pid" 2>/dev/null || kill -TERM "$build_pid" 2>/dev/null || true
		wait "$build_pid" 2>/dev/null || true
	fi
	cleanup
	exit "$status"
}

trap cleanup EXIT
trap 'forward_signal INT' INT
trap 'forward_signal TERM' TERM

cd "$WEBSITE_DIR"

printf '[website-build] cpus=%s mem_total=%sMiB swap_total=%sMiB timeout=%ss\n' \
	"$(nproc)" "$(meminfo_mib MemTotal)" "$(meminfo_mib SwapTotal)" "$timeout_secs"
printf '[website-build] DOCUSAURUS_NO_PERSISTENT_CACHE=%s MIMALLOC_PURGE_DELAY=%s\n' \
	"$DOCUSAURUS_NO_PERSISTENT_CACHE" "$MIMALLOC_PURGE_DELAY"
printf '[website-build] running: %s\n' "$*"

started=$SECONDS
LOW_MEM_DETAIL=""
memory_line 0 || true
sample_memory &
sampler_pid=$!

# Without --foreground, timeout makes itself a process-group leader and sends
# its signals to the whole group, so worker processes cannot outlive it once
# it fires.
timeout --kill-after="${kill_after_secs}s" "${timeout_secs}s" "$@" &
build_pid=$!
rc=0
wait "$build_pid" || rc=$?
build_pid=""

cleanup

elapsed=$((SECONDS - started))
if ((rc == 124 || (rc == 137 && elapsed >= timeout_secs))); then
	printf '::error title=Website build watchdog::the build did not finish within %ss and was killed (exit %s); see the memory samples above\n' \
		"$timeout_secs" "$rc"
elif ((rc > 128)); then
	printf '::error title=Website build killed::the build died from signal %s (exit %s) after %ss; see the memory samples above\n' \
		$((rc - 128)) "$rc" "$elapsed"
	if sudo -n true 2>/dev/null; then
		sudo -n dmesg 2>/dev/null | grep -iE 'out of memory|oom-kill|killed process' | tail -n 5 || true
	fi
fi

memory_line "$elapsed" || true
printf '[website-build] exit=%s after %ss\n' "$rc" "$elapsed"
exit "$rc"
