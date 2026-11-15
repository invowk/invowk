#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Self-test for check-script-tests-worktree-safety.sh: the gate must fail
# (exit 3) when the suite reproduces the old test_mutation.sh behaviour
# (`git restore` of cmd/ internal/ pkg/ in the checkout it runs in) or any
# other destructive change, including changes to git config, refs and the
# stash, and must pass a benign suite. Every run guards a
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
SIBLING="$TMP/sibling"
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
# looks: two tracked text files under each of cmd/, internal/ and pkg/, a text
# file in website/ and docs/ and at the root, a tag, a branch, a stash entry, a
# sibling worktree, and uncommitted work (unstaged, staged, untracked) the gate
# must mirror and keep.
make_fixture() {
	local dir="$1"
	local sub
	local stash

	for sub in cmd/app internal/core pkg/lib; do
		mkdir -p "$dir/$sub"
		printf 'package %s\n' "${sub##*/}" >"$dir/$sub/a.go"
		printf '# %s\n' "$sub" >"$dir/$sub/README.md"
	done
	mkdir -p "$dir/website" "$dir/docs"
	printf '# site\n' >"$dir/website/README.md"
	printf '# guide\n' >"$dir/docs/guide.md"
	printf '# fixture\n' >"$dir/README.md"
	printf 'artifacts/\n' >"$dir/.gitignore"
	git -C "$dir" init -q
	git -C "$dir" config user.email test@example.com
	git -C "$dir" config user.name Test
	git -C "$dir" config commit.gpgsign false
	git -C "$dir" config core.hooksPath /dev/null
	git -C "$dir" add -A
	git -C "$dir" commit -qm init
	git -C "$dir" tag v1.0.0
	git -C "$dir" branch feature
	git -C "$dir" worktree add -q --detach "$SIBLING"
	printf 'sibling work\n' >>"$SIBLING/docs/guide.md"

	printf 'stashed work\n' >>"$dir/docs/guide.md"
	stash="$(git -C "$dir" stash create)"
	git -C "$dir" stash store -m "real stash" "$stash"
	git -C "$dir" checkout -q -- docs/guide.md

	printf 'real unstaged work\n' >>"$dir/cmd/app/README.md"
	printf 'real staged work\n' >>"$dir/pkg/lib/README.md"
	git -C "$dir" add pkg/lib/README.md
	printf 'real untracked work\n' >"$dir/internal/core/notes.txt"
}

# repo_state DIR prints everything of the fixture a suite could damage,
# including the shared git state: config, refs, stash and worktree list.
repo_state() {
	git -C "$1" diff --binary
	git -C "$1" diff --cached --binary
	git -C "$1" status --porcelain --untracked-files=all
	cat "$1/internal/core/notes.txt"
	git -C "$1" config --local --list
	git -C "$1" for-each-ref --format='%(objectname) %(refname)'
	git -C "$1" stash list --format='%H %gs'
	git -C "$1" worktree list --porcelain
	cat "$SIBLING/docs/guide.md"
}

# gate_status [--in-place] COMMAND... runs the gate over the fixture with
# COMMAND as the suite and prints its exit status.
gate_status() {
	local status
	local -a mode=()

	if [[ "$1" == "--in-place" ]]; then
		mode=(--in-place)
		shift
	fi
	set +e
	TMPDIR="$TMP" "$GATE" --repo "$FIXTURE" "${mode[@]}" -- "$@" >"$OUTPUT" 2>&1
	status=$?
	set -e
	printf '%s\n' "$status"
}

leftover_work_dirs() {
	find "$TMP" -mindepth 1 -maxdepth 1 -name 'invowk-script-tests.*' | wc -l | tr -d ' '
}

kept_before_dir() {
	sed -n 's/^  \(.*real-before\)$/\1/p' "$OUTPUT" | head -n 1
}

make_fixture "$FIXTURE"
before="$(repo_state "$FIXTURE")"

# --- benign suites -----------------------------------------------------------
assert_eq "a benign suite passes" "0" "$(gate_status true)"
assert_output_contains "a benign suite reports success" "left every checkout unchanged"
assert_eq "a suite may write ignored files" "0" "$(gate_status sh -c 'mkdir -p artifacts && echo x >artifacts/report.json')"
assert_eq "the suite sees the mirrored real work" "0" \
	"$(gate_status sh -c 'grep -q "real unstaged work" cmd/app/README.md && git diff --cached --name-only | grep -qx pkg/lib/README.md && test -f internal/core/notes.txt')"
assert_eq "the suite sees the real tags and branches" "0" \
	"$(gate_status sh -c 'git rev-parse -q --verify refs/tags/v1.0.0 >/dev/null && git rev-parse -q --verify refs/heads/feature >/dev/null')"
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "the suite runs in a repository of its own, with no remote and no real stash" "0" \
	"$(gate_status sh -c 'test -d .git && test -z "$(git remote)" && ! git stash list | grep -q "real stash"')"
assert_eq "a failing suite that changes nothing exits 1" "1" "$(gate_status false)"
assert_output_contains "a failing suite is reported as such" "no checkout was changed"
assert_eq "passing and failing runs leave no temporary directory behind" "0" "$(leftover_work_dirs)"

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

# Canaries sit in every top-level directory and at the root, not only in the
# mutation source paths.
assert_eq "a path-scoped checkout outside cmd/ internal/ pkg/ is caught" "3" "$(gate_status git checkout -q -- website)"
assert_output_contains "the website canary reports the checkout" "CANARY: website/"
assert_eq "a path-scoped clean outside cmd/ internal/ pkg/ is caught" "3" "$(gate_status git clean -fdq docs)"
assert_output_contains "the docs canary reports the clean" "CANARY: docs/"
assert_eq "a restore of a root file is caught" "3" "$(gate_status git restore -- README.md)"
assert_eq "deleting a root untracked file is caught" "3" "$(gate_status git clean -fq -- ':(glob).*canary*')"

# Shared git state: the throwaway repository has its own copies, and the gate
# compares them, so a suite that would damage them in a real checkout fails.
assert_eq "a config change is caught" "3" \
	"$(gate_status sh -c 'git config commit.gpgsign true && git config core.hooksPath /tmp/elsewhere')"
assert_output_contains "a config change is reported" "config changed"
assert_eq "git -C with an empty directory (the current one) is caught" "3" \
	"$(gate_status git -C "" config user.email other@example.com)"
assert_eq "clearing the stash is caught" "3" "$(gate_status git stash clear)"
assert_output_contains "clearing the stash is reported" "stash changed"
assert_eq "deleting a branch is caught" "3" "$(gate_status git branch -q -D feature)"
assert_output_contains "deleting a branch is reported" "refs changed"
assert_eq "deleting the canary branch is caught" "3" \
	"$(gate_status git branch -q -D invowk-script-tests-worktree-safety-canary-branch)"
assert_eq "a sibling worktree cannot be removed from the throwaway repository" "1" \
	"$(gate_status git worktree remove --force "$SIBLING")"
assert_eq "runs inside the throwaway repository leave the real one alone, shared state included" "$before" "$(repo_state "$FIXTURE")"
assert_eq "the throwaway repository is never registered as a worktree" "2" \
	"$(git -C "$FIXTURE" worktree list --porcelain | grep -c '^worktree ')"

# --- damage to the real checkout -----------------------------------------------
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "a change to the real checkout is caught" "3" \
	"$(gate_status sh -c 'printf "lost\n" >>"$1/cmd/app/a.go"' sh "$FIXTURE")"
assert_output_contains "real-checkout damage names the real checkout" "SAFETY VIOLATION: the script tests changed the real checkout"
assert_output_contains "real-checkout damage prints recovery commands" "apply --index"
kept="$(kept_before_dir)"
if [[ -n "$kept" && -s "$kept/staged.diff" && -s "$kept/unstaged.diff" && -f "$kept/untracked.tar" ]]; then
	record_pass
else
	record_fail "real-checkout damage keeps the before fingerprint" "kept: ${kept:-<none>}"
fi
assert_eq "nothing is restored automatically" "lost" "$(tail -n 1 "$FIXTURE/cmd/app/a.go")"

# A suite that breaks the throwaway repository so that its state cannot be read
# must still get the real checkout compared, and its recovery data kept.
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "a broken throwaway repository plus real damage is a violation" "3" \
	"$(gate_status sh -c 'rm -rf .git; printf "lost again\n" >>"$1/cmd/app/a.go"' sh "$FIXTURE")"
assert_output_contains "a broken throwaway repository is reported" "SAFETY VIOLATION: the script tests broke the throwaway checkout"
assert_output_contains "the real checkout is still compared" "SAFETY VIOLATION: the script tests changed the real checkout"
kept="$(kept_before_dir)"
if [[ -n "$kept" && -s "$kept/unstaged.diff" ]]; then
	record_pass
else
	record_fail "a broken throwaway repository keeps the real before fingerprint" "kept: ${kept:-<none>}"
fi
assert_eq "a broken throwaway repository alone is a violation" "3" "$(gate_status rm -rf .git)"

# An interrupt after the suite started still compares the real checkout, from
# the exit trap. The suite signals the gate itself (GATE_PID) and exits, so the
# gate handles the signal as soon as its suite returns.
interrupted_gate_status() {
	local status

	set +e
	TMPDIR="$TMP" bash -c 'export GATE_PID=$$; exec "$@"' gate "$GATE" --repo "$FIXTURE" -- "$@" >"$OUTPUT" 2>&1
	status=$?
	set -e
	printf '%s\n' "$status"
}
work_dirs_before="$(leftover_work_dirs)"
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "an interrupted benign run exits with the signal status" "143" \
	"$(interrupted_gate_status sh -c 'kill -TERM "$GATE_PID"')"
assert_eq "an interrupted benign run leaves no temporary directory behind" "$work_dirs_before" "$(leftover_work_dirs)"
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "an interrupted run that damaged the real checkout is a violation" "3" \
	"$(interrupted_gate_status sh -c 'printf "lost on interrupt\n" >>"$1/cmd/app/a.go"; kill -TERM "$GATE_PID"' sh "$FIXTURE")"
assert_output_contains "an interrupted run still compares the real checkout" "SAFETY VIOLATION: the script tests changed the real checkout"
kept="$(kept_before_dir)"
if [[ -n "$kept" && -s "$kept/unstaged.diff" ]]; then
	record_pass
else
	record_fail "an interrupted run keeps the real before fingerprint" "kept: ${kept:-<none>}"
fi

# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "a change to the real checkout's guarded config is caught" "3" \
	"$(gate_status sh -c 'git -C "$1" config core.hooksPath /tmp/elsewhere' sh "$FIXTURE")"
assert_output_contains "a real config change is reported" "config changed"
git -C "$FIXTURE" config core.hooksPath /dev/null
# shellcheck disable=SC2016 # expanded by the shell that runs the snippet
assert_eq "a sibling's branch tracking config is not a violation" "0" \
	"$(gate_status sh -c 'git -C "$1" config branch.feature.remote origin' sh "$FIXTURE")"
git -C "$FIXTURE" config --unset branch.feature.remote

# --- in-place (unguarded) runs are still fingerprinted ---------------------------
in_place_before="$(repo_state "$FIXTURE")"
assert_eq "an in-place benign suite passes" "0" "$(gate_status --in-place true)"
assert_output_contains "an in-place run says it is unguarded" "Running the script tests in place (unguarded)"
assert_eq "an in-place failing suite that changes nothing exits 1" "1" "$(gate_status --in-place false)"
assert_eq "an in-place config change is caught" "3" "$(gate_status --in-place git config commit.gpgsign true)"
git -C "$FIXTURE" config commit.gpgsign false
assert_eq "an in-place git restore is caught" "3" \
	"$(gate_status --in-place git restore --worktree -- cmd internal pkg)"
assert_output_contains "an in-place violation names the real checkout" "SAFETY VIOLATION: the script tests changed the real checkout"
assert_output_contains "an in-place violation prints recovery commands" "apply --index"
kept="$(kept_before_dir)"
if [[ -n "$kept" ]] && git -C "$FIXTURE" apply --check "$kept/unstaged.diff" 2>/dev/null; then
	git -C "$FIXTURE" apply "$kept/unstaged.diff"
	assert_eq "the printed recovery restores the lost work" "$in_place_before" "$(repo_state "$FIXTURE")"
else
	record_fail "an in-place violation keeps an applicable before-diff" "kept: ${kept:-<none>}"
fi

# --- the single test list covers every script test file ------------------------
list_root="$TMP/list-check"
mkdir -p "$list_root/scripts"
git -C "$list_root" init -q
cp "$SCRIPT_DIR/script-tests.sh" "$list_root/scripts/"
while IFS= read -r file; do
	mkdir -p "$list_root/$(dirname "$file")"
	: >"$list_root/$file"
done < <(git -C "$SCRIPT_DIR/.." ls-files --cached --others --exclude-standard |
	grep -E '(^|/)(test_[^/]+\.(sh|bash|py|mjs|js|ps1)|[^/]+_test\.(sh|bash|py|mjs|js))$' |
	grep -Ev '^tests/cli/|(^|/)testdata/|(^|/)node_modules/')
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
rm "$list_root/scripts/test_unlisted.sh"
mkdir -p "$list_root/.agents/skills/new/scripts" "$list_root/tools/new"
: >"$list_root/.agents/skills/new/scripts/test_new.py"
: >"$list_root/tools/new/helper_test.sh"
assert_eq "the list check fails on unlisted test files outside scripts/" "1" "$(list_check_status)"
assert_output_contains "the list check names a skill test" ".agents/skills/new/scripts/test_new.py is not in"
assert_output_contains "the list check names a *_test script" "tools/new/helper_test.sh is not in"
rm -r "$list_root/.agents/skills/new" "$list_root/tools/new"
mkdir -p "$list_root/tests/cli/testdata" "$list_root/pkg/x/testdata"
: >"$list_root/tests/cli/testdata/test_fixture.sh"
: >"$list_root/pkg/x/testdata/test_fixture.py"
assert_eq "the list check skips testscript and testdata fixtures" "0" "$(list_check_status)"
rm "$list_root/scripts/test_mutation.sh"
assert_eq "the list check fails on a listed file that is missing" "1" "$(list_check_status)"
assert_output_contains "the list check names the missing file" "lists scripts/test_mutation.sh, which does not exist"

# --- setup errors ---------------------------------------------------------------
set +e
"$GATE" --repo "$TMP/not-a-repo" -- true >"$OUTPUT" 2>&1
assert_eq "a missing checkout is a setup error" "2" "$?"
set -e

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
