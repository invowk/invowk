#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# The single list of repository script tests.
#
#   make test-scripts             runs this list through
#                                 scripts/check-script-tests-worktree-safety.sh,
#                                 inside a throwaway checkout guarded by canaries
#                                 (CI runs this target);
#   make test-scripts-unguarded   runs it directly in this checkout.
#
# Every scripts/test_* file must appear in SCRIPT_TESTS or, when it can only
# run elsewhere, in SCRIPT_TESTS_ELSEWHERE with the reason. Each run starts
# with that check (--check-list runs only the check), so a new test cannot be
# left out of `make test-scripts` and CI.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# "label|command", run from the repository root in this order. Arguments are
# split on whitespace, so commands must not need quoting.
SCRIPT_TESTS=(
	"POSIX install script|sh scripts/test_install.sh"
	"Release helper scripts|bash scripts/test_release.sh"
	"Bencher BMF script|node scripts/test_bench_bmf.mjs"
	"Bencher registry login script|bash scripts/test_bencher_registry_login.sh"
	"WinGet manifest enhancement|sh scripts/test_enhance_winget_manifest.sh"
	"golangci-lint wrapper|bash scripts/test_golangci_lint.sh"
	"govulncheck wrapper|bash scripts/test_govulncheck_all.sh"
	"Mutation wrapper|bash scripts/test_mutation.sh"
	"Formal-verification runner|python3 scripts/test_formal.py"
	"Formal promotion gate|python3 scripts/test_formal_promotion_gate.py"
	"Formal-bindings mutation planner|python3 scripts/test_formal_mutation.py"
	"Formal-bindings triage ledger|python3 scripts/formal_mutation.py triage --check"
	"Skill package checker|python3 scripts/test_check_skill_packages.py"
	"Release poetic-opening checker|python3 .agents/skills/publish-release/scripts/test_check_poetic_opening.py"
	"Script-test worktree-safety gate|bash scripts/test_check_script_tests_worktree_safety.sh"
)

# "path|reason" for test files this list cannot run.
SCRIPT_TESTS_ELSEWHERE=(
	"scripts/test_install.ps1|PowerShell only: ci.yml and release.yml run it under pwsh and Windows PowerShell 5.1 on windows-latest"
)

# check_list fails when a scripts/test_* file is in neither list, or when a
# list names a test file that does not exist.
check_list() {
	local entry
	local file
	local word
	local -a words=()
	local -A listed=()
	local errors=0

	for entry in "${SCRIPT_TESTS[@]}"; do
		read -r -a words <<<"${entry#*|}"
		for word in "${words[@]}"; do
			[[ "$word" == scripts/test_* ]] && listed["$word"]=1
		done
	done
	for entry in "${SCRIPT_TESTS_ELSEWHERE[@]}"; do
		listed["${entry%%|*}"]=1
	done

	for file in "$REPO_ROOT"/scripts/test_*; do
		[[ -e "$file" ]] || continue
		file="scripts/${file##*/}"
		if [[ -z "${listed[$file]:-}" ]]; then
			printf 'ERROR: %s is not in scripts/script-tests.sh: add it to SCRIPT_TESTS (or, if it cannot run there, to SCRIPT_TESTS_ELSEWHERE with the reason).\n' "$file" >&2
			errors=$((errors + 1))
		fi
	done
	for file in "${!listed[@]}"; do
		if [[ ! -f "$REPO_ROOT/$file" ]]; then
			printf 'ERROR: scripts/script-tests.sh lists %s, which does not exist.\n' "$file" >&2
			errors=$((errors + 1))
		fi
	done
	((errors == 0))
}

run_tests() {
	local entry
	local label
	local status
	local -a command=()
	local -a failed=()

	cd "$REPO_ROOT"
	for entry in "${SCRIPT_TESTS[@]}"; do
		label="${entry%%|*}"
		read -r -a command <<<"${entry#*|}"
		printf '\n==> %s: %s\n' "$label" "${command[*]}"
		status=0
		"${command[@]}" || status=$?
		if ((status != 0)); then
			printf '<== FAILED (exit %s): %s\n' "$status" "$label" >&2
			failed+=("$label")
		fi
	done

	printf '\n'
	for entry in "${SCRIPT_TESTS_ELSEWHERE[@]}"; do
		printf 'Not run here: %s (%s)\n' "${entry%%|*}" "${entry#*|}"
	done
	if ((${#failed[@]} > 0)); then
		printf 'Script tests FAILED: %s\n' "${failed[@]}" >&2
		return 1
	fi
	printf 'All %d script tests passed.\n' "${#SCRIPT_TESTS[@]}"
}

case "${1:-}" in
	--check-list)
		check_list
		printf 'scripts/script-tests.sh lists every scripts/test_* file.\n'
		;;
	"")
		check_list
		run_tests
		;;
	*)
		printf 'Usage: %s [--check-list]\n' "$0" >&2
		exit 2
		;;
esac
