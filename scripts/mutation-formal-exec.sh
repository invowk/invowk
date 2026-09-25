#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# go-mutesting --exec command for the formal-bindings target set.
#
# For one mutant it finds the planned function that contains the first changed
# line, applies the mutant through `go test -overlay` (the tracked file is never
# rewritten), and runs only that function's killer tests across the packages
# that own them: -count=1, no -short (golden vectors replay in full), no -race,
# no trace validation or TLC.
#
# Exit codes follow go-mutesting's --exec contract:
#   0 killed (a test failed, or the test binary timed out: logged as timeout-kill)
#   1 escaped (every killer test passed)
#   2 skipped (the mutant does not build, or test setup failed)
#   3 errored (missing determinism settings, unknown file/function, other failures)
#
# With MUTATE_CHANGED equal to MUTATE_ORIGINAL (the clean-code pre-flight) it
# runs every planned function of the file with `go test -json`, and copies the
# JSON stream to MUTATION_FORMAL_PREFLIGHT_JSON.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_CMD="${GO_CMD:-go}"
PYTHON="${PYTHON:-python3}"

readonly EXIT_KILLED=0
readonly EXIT_ESCAPED=1
readonly EXIT_SKIPPED=2
readonly EXIT_ERRORED=3

log() {
	printf 'formal-mutation: %s\n' "$*" >&2
}

require_env() {
	local missing=()

	[[ -n "${MUTATION_FORMAL_PLAN:-}" ]] || missing+=(MUTATION_FORMAL_PLAN)
	[[ -n "${RAPID_SEED:-}" ]] || missing+=(RAPID_SEED)
	[[ "${RAPID_NOFAILFILE:-}" == "1" ]] || missing+=("RAPID_NOFAILFILE=1")
	[[ -n "${RAPID_SHRINKTIME:-}" ]] || missing+=(RAPID_SHRINKTIME)
	[[ -n "${MUTATE_ORIGINAL:-}" ]] || missing+=(MUTATE_ORIGINAL)
	[[ -n "${MUTATE_CHANGED:-}" ]] || missing+=(MUTATE_CHANGED)
	if ((${#missing[@]} > 0)); then
		log "refusing to run without: ${missing[*]}"
		return 1
	fi
}

# classify_output STATUS OUTPUT_FILE FILE FUNCTION prints nothing and returns
# the go-mutesting exit code for a finished `go test`.
classify_output() {
	local status="$1"
	local output="$2"
	local file="$3"
	local function="$4"

	if ((status == 0)); then
		return "$EXIT_ESCAPED"
	fi
	if grep -Fq 'panic: test timed out after' "$output"; then
		log "timeout-kill file=$file function=$function"
		return "$EXIT_KILLED"
	fi
	if grep -Fq -- '--- FAIL:' "$output"; then
		return "$EXIT_KILLED"
	fi
	if grep -Eq '\[(build|setup) failed\]' "$output"; then
		return "$EXIT_SKIPPED"
	fi
	# A package-level failure outside any test (a panic in init or TestMain, an
	# os.Exit from mutated code) still fails the killer binary.
	if grep -Eq '^FAIL[[:space:]]|"Action":"fail"' "$output"; then
		return "$EXIT_KILLED"
	fi
	log "unclassified go test failure (exit $status) for $file ($function)"
	return "$EXIT_ERRORED"
}

main() {
	local tmp
	local lookup
	local key
	local value
	local file=""
	local function=""
	local timeout=""
	local run=""
	local packages=""
	local status
	local -a go_args=()
	local -a package_list=()

	require_env || return "$EXIT_ERRORED"

	tmp="$(mktemp -d)"
	# shellcheck disable=SC2064 # expand now: tmp is local to main
	trap "rm -rf '$tmp'" EXIT

	if ! lookup="$("$PYTHON" "$SCRIPT_DIR/formal_mutation.py" exec-args \
		--plan "$MUTATION_FORMAL_PLAN" \
		--original "$MUTATE_ORIGINAL" \
		--changed "$MUTATE_CHANGED" \
		--overlay-out "$tmp/overlay.json")"; then
		log "no planned function for $MUTATE_ORIGINAL"
		return "$EXIT_ERRORED"
	fi
	while IFS='=' read -r key value; do
		case "$key" in
			file) file="$value" ;;
			function) function="$value" ;;
			timeout) timeout="$value" ;;
			run) run="$value" ;;
			packages) packages="$value" ;;
		esac
	done <<<"$lookup"
	read -r -a package_list <<<"$packages"
	if [[ -z "$file" || -z "$timeout" || -z "$run" || ${#package_list[@]} -eq 0 ]]; then
		log "incomplete plan lookup for $MUTATE_ORIGINAL"
		return "$EXIT_ERRORED"
	fi

	go_args=(test -count=1 -vet=off -timeout "${timeout}s" -run "$run")
	if [[ -f "$tmp/overlay.json" ]]; then
		go_args+=("-overlay=$tmp/overlay.json")
	fi
	if [[ -n "${MUTATION_FORMAL_PREFLIGHT_JSON:-}" ]]; then
		go_args+=(-json)
	fi

	set +e
	(cd "$REPO_ROOT" && "$GO_CMD" "${go_args[@]}" "${package_list[@]}") >"$tmp/output" 2>&1
	status=$?
	set -e

	if [[ -n "${MUTATION_FORMAL_PREFLIGHT_JSON:-}" ]]; then
		cp "$tmp/output" "$MUTATION_FORMAL_PREFLIGHT_JSON"
	fi
	if [[ "${MUTATE_DEBUG:-false}" == "true" ]]; then
		cat "$tmp/output" >&2
	fi

	classify_output "$status" "$tmp/output" "$file" "$function"
}

if [[ "${INVOWK_MUTATION_FORMAL_EXEC_TESTING:-0}" != "1" ]]; then
	set +e
	main "$@"
	exit_code=$?
	set -e
	exit "$exit_code"
fi
