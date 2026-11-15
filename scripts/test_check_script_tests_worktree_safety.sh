#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Self-test for check-script-tests-worktree-safety.sh: the gate must fail
# (exit 3) when the suite reproduces the old test_mutation.sh behaviour
# (`git restore` of cmd/ internal/ pkg/ in the checkout it runs in) or any
# other destructive change, and must pass a benign suite. Every run guards a
# throwaway fixture repository and injects a small command as the suite; the
# real suite is never run from here, and nothing touches this checkout.

set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="$SCRIPT_DIR/check-script-tests-worktree-safety.sh"

PASS=0
FAIL=0
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
FIXTURE="$TMP/repo"
OUTPUT="$TMP/output"

record_pass() {
	PASS=$((PASS + 1))
}

record_fail() {
	FAIL=$((FAIL + 1))
	printf 'FAIL: %s\n  %s\n' "$1" "$2" >&2
}

assert_eq() {
	if [[ "$2" == "$3" ]]; then
		record_pass
	else
		record_fail "$1" "expected: $2; actual: $3"
	fi
}

assert_output_contains() {
	if grep -Fq -- "$2" "$OUTPUT"; then
		record_pass
	else
		record_fail "$1" "missing text: $2"
		sed 's/^/    | /' "$OUTPUT" >&2
	fi
}

# make_fixture DIR creates a repository shaped like this one where the gate
# looks: two tracked text files under each of cmd/, internal/ and pkg/, plus
# uncommitted work (unstaged, staged, untracked) the gate must mirror and keep.
make_fixture() {
	local dir="$1"
	local sub

	for sub in cmd/app internal/core pkg/lib; do
		mkdir -p "$dir/$sub"
		printf 'package %s\n' "${sub##*/}" >"$dir/$sub/a.go"
		printf '# %s\n' "$sub" >"$dir/$sub/README.md"
	done
	printf 'artifacts/\n' >"$dir/.gitignore"
	git -C "$dir" init -q
	git -C "$dir" config user.email test@example.com
	git -C "$dir" config user.name Test
	git -C "$dir" config commit.gpgsign false
	git -C "$dir" config core.hooksPath /dev/null
	git -C "$dir" add -A
	git -C "$dir" commit -qm init

	printf 'real unstaged work\n' >>"$dir/cmd/app/README.md"
	printf 'real staged work\n' >>"$dir/pkg/lib/README.md"
	git -C "$dir" add pkg/lib/README.md
	printf 'real untracked work\n' >"$dir/internal/core/notes.txt"
}

repo_state() {
	git -C "$1" diff --binary
	git -C "$1" diff --cached --binary
	git -C "$1" status --porcelain --untracked-files=all
	cat "$1/internal/core/notes.txt"
}

worktree_count() {
	git -C "$FIXTURE" worktree list --porcelain | grep -c '^worktree '
}

# gate_status COMMAND... runs the gate over the fixture with COMMAND as the
# suite and prints its exit status.
gate_status() {
	local status

	set +e
	TMPDIR="$TMP" "$GATE" --repo "$FIXTURE" -- "$@" >"$OUTPUT" 2>&1
	status=$?
	set -e
	printf '%s\n' "$status"
}

make_fixture "$FIXTURE"
before="$(repo_state "$FIXTURE")"

# --- benign suites -----------------------------------------------------------
assert_eq "a benign suite passes" "0" "$(gate_status true)"
assert_output_contains "a benign suite reports success" "left every checkout unchanged"
assert_eq "a suite may write ignored files" "0" "$(gate_status sh -c 'mkdir -p artifacts && echo x >artifacts/report.json')"
assert_eq "the suite sees the mirrored real work" "0" \
	"$(gate_status sh -c 'grep -q "real unstaged work" cmd/app/README.md && git diff --cached --name-only | grep -qx pkg/lib/README.md && test -f internal/core/notes.txt')"
assert_eq "a failing suite that changes nothing exits 1" "1" "$(gate_status false)"
assert_output_contains "a failing suite is reported as such" "no checkout was changed"

# --- the bug this gate exists for ---------------------------------------------
# The old test_tool_report_collection_before_untracked_cleanup ran
# restore_tracked_mutation_paths against the checkout the tests ran in.
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "the old git restore of cmd/ internal/ pkg/ is caught" "3" \
	"$(gate_status bash -c 'REPO_ROOT="$(pwd)"; git -C "$REPO_ROOT" restore --worktree -- cmd internal pkg >/dev/null 2>&1 || true')"
assert_output_contains "the restore is reported as a canary change" "CANARY: cmd/"
assert_output_contains "the restore is reported as a safety violation" "SAFETY VIOLATION: the script tests changed the throwaway checkout"

# --- other destructive behaviour ----------------------------------------------
assert_eq "an index reset is caught" "3" "$(gate_status git reset -q)"
assert_output_contains "an index reset changes the staged diff" "staged.diff changed"
assert_eq "deleting untracked files is caught" "3" "$(gate_status git clean -fdq)"
assert_eq "a stash is caught" "3" "$(gate_status git stash -q)"
assert_eq "a leftover untracked file is caught" "3" "$(gate_status touch internal/core/leftover.txt)"
assert_eq "a violating suite fails as a violation even when it exits 0" "3" \
	"$(gate_status sh -c 'git checkout -q -- cmd; exit 0')"
assert_eq "a violating suite fails as a violation even when it exits 1" "3" \
	"$(gate_status sh -c 'git checkout -q -- cmd; exit 1')"
assert_eq "the throwaway checkout is removed after every run" "1" "$(worktree_count)"
assert_eq "runs inside the throwaway checkout leave the real checkout alone" "$before" "$(repo_state "$FIXTURE")"

# --- damage to the real checkout -----------------------------------------------
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "a change to the real checkout is caught" "3" \
	"$(gate_status sh -c 'printf "lost\n" >>"$1/cmd/app/a.go"' sh "$FIXTURE")"
assert_output_contains "real-checkout damage names the real checkout" "SAFETY VIOLATION: the script tests changed the real checkout"
assert_output_contains "real-checkout damage prints recovery commands" "apply --index"
kept="$(sed -n 's/^  \(.*real-before\)$/\1/p' "$OUTPUT" | head -n 1)"
if [[ -n "$kept" && -s "$kept/staged.diff" && -s "$kept/unstaged.diff" && -f "$kept/untracked.tar" ]]; then
	record_pass
else
	record_fail "real-checkout damage keeps the before fingerprint" "kept: ${kept:-<none>}"
fi
assert_eq "nothing is restored automatically" "lost" "$(tail -n 1 "$FIXTURE/cmd/app/a.go")"
assert_eq "the throwaway checkout is removed after a violation" "1" "$(worktree_count)"

# --- the single test list covers every scripts/test_* file ---------------------
list_root="$TMP/list-check"
mkdir -p "$list_root/scripts"
cp "$SCRIPT_DIR/script-tests.sh" "$list_root/scripts/"
for file in "$SCRIPT_DIR"/test_*; do
	: >"$list_root/scripts/${file##*/}"
done
list_check_status() {
	set +e
	bash "$list_root/scripts/script-tests.sh" --check-list >"$OUTPUT" 2>&1
	printf '%s\n' "$?"
	set -e
}
assert_eq "the list check passes when every test file is listed" "0" "$(list_check_status)"
: >"$list_root/scripts/test_unlisted.sh"
assert_eq "the list check fails on an unlisted test file" "1" "$(list_check_status)"
assert_output_contains "the list check names the unlisted file" "scripts/test_unlisted.sh is not in scripts/script-tests.sh"
rm "$list_root/scripts/test_unlisted.sh" "$list_root/scripts/test_mutation.sh"
assert_eq "the list check fails on a listed file that is missing" "1" "$(list_check_status)"
assert_output_contains "the list check names the missing file" "lists scripts/test_mutation.sh, which does not exist"

# --- setup errors ---------------------------------------------------------------
set +e
"$GATE" --repo "$TMP/not-a-repo" -- true >"$OUTPUT" 2>&1
assert_eq "a missing checkout is a setup error" "2" "$?"
set -e

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
