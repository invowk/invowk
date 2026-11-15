#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Print the growth base of `formal_mutation.py triage --check` for a GitHub
# Actions run, as a full commit SHA on stdout (harden-formal-instruments
# decisions 19 and 22). Callers export it as INVOWK_TRIAGE_BASE in the step
# that runs the check, so a failed fetch fails that step:
#
#   base="$(scripts/ci-triage-base.sh)"
#   export INVOWK_TRIAGE_BASE="$base"
#
#   pull_request   fetch the base branch tip (GITHUB_BASE_REF) at depth 1 and
#                  print it; the shrink-only lists may not outgrow it.
#   other events   print the commit under test (GITHUB_SHA, else HEAD), with a
#                  notice: the check then covers only the lists' consistency.
#
# Notices go to stderr. Exit status: 0 printed a base; 1 the fetch or the
# resolution failed (the check must not run against a guessed base).

set -euo pipefail

notice() {
	printf '::notice title=Triage base::%s\n' "$*" >&2
}

fail() {
	printf '::error title=Triage base::%s\n' "$*" >&2
	exit 1
}

event="${GITHUB_EVENT_NAME:-}"
if [[ "$event" == "pull_request" ]]; then
	ref="${GITHUB_BASE_REF:-}"
	[[ -n "$ref" ]] || fail "pull_request event without GITHUB_BASE_REF"
	git fetch -q --no-tags --depth=1 origin "refs/heads/$ref" ||
		fail "could not fetch the base branch $ref from origin"
	base="$(git rev-parse --verify -q 'FETCH_HEAD^{commit}')" ||
		fail "the fetch of $ref left no FETCH_HEAD commit"
	notice "triage base: $ref tip $base"
else
	head="${GITHUB_SHA:-}"
	[[ -n "$head" ]] || head="$(git rev-parse HEAD)" || fail "cannot read HEAD"
	base="$(git rev-parse --verify -q "$head^{commit}")" || fail "the commit under test $head is not in this checkout"
	notice "triage base == HEAD ($base) on a ${event:-local} run: the shrink-only lists are checked for consistency only"
fi
printf '%s\n' "$base"
