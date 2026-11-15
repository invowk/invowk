#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Worktree-safety gate for the script tests (`make test-scripts`).
#
# Script tests exercise tools that revert sources and delete files (the
# mutation wrapper among them). A test that points such a tool at the real
# checkout silently discards the uncommitted work of whoever runs it. This
# gate makes that mechanically detectable:
#
#   1. It fingerprints the real checkout (staged and unstaged binary diffs,
#      untracked non-ignored files with their content, HEAD and branch).
#   2. It creates a throwaway checkout (`git worktree add --detach` at HEAD),
#      mirrors the real checkout's staged, unstaged and untracked work into it,
#      and plants canaries: an unstaged edit to a tracked file under each of
#      cmd/, internal/ and pkg/, one staged edit, and one untracked file.
#   3. It runs the suite (default: bash scripts/script-tests.sh) INSIDE the
#      throwaway checkout, then compares the canary bytes, the full staged and
#      unstaged diffs and the untracked set against step 2, and the real
#      checkout against step 1. Any difference fails the gate.
#
# It never restores anything automatically. When the real checkout changed, it
# keeps the step-1 fingerprint (including copies of the untracked files) and
# prints the commands that recover it. The throwaway checkout is always
# removed.
#
# --in-place (make test-scripts-unguarded) skips steps 2-3: it runs the suite
# in the real checkout and only compares the step-1 fingerprint afterwards, so
# damage is still detected and recoverable, though no longer prevented.
#
# Usage: check-script-tests-worktree-safety.sh [--repo DIR] [--in-place] [-- COMMAND...]
#   --repo DIR   checkout to guard (default: this script's repository)
#   --in-place   run the suite in the checkout itself, not a throwaway one
#   COMMAND      suite to run (default: bash scripts/script-tests.sh)
#
# Exit status: 0 suite passed and nothing changed; 1 suite failed and nothing
# changed; 2 usage or setup error; 3 a checkout changed (whatever the suite's
# status).

set -euo pipefail

# Never let an inherited git environment (for example a git hook's) redirect
# the commands below to another repository or index.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

readonly EXIT_SUITE_FAILED=1
readonly EXIT_SETUP=2
readonly EXIT_VIOLATION=3
readonly CANARY_TAG="invowk-script-tests-worktree-safety-canary"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$SCRIPT_DIR/.."
SUITE=(bash scripts/script-tests.sh)

WORK=""
CHECKOUT=""
CHECKOUT_ADMIN=""
KEEP_REAL_BEFORE=0
IN_PLACE=0

setup_error() {
	printf 'ERROR: %s\n' "$*" >&2
	exit "$EXIT_SETUP"
}

usage() {
	sed -n '/^# Usage:/,/^# status)/s/^# \{0,1\}//p' "${BASH_SOURCE[0]}"
}

while (($# > 0)); do
	case "$1" in
		--repo)
			(($# > 1)) || setup_error "--repo requires a directory"
			REPO="$2"
			shift 2
			;;
		--in-place)
			IN_PLACE=1
			shift
			;;
		--help | -h)
			usage
			exit 0
			;;
		--)
			shift
			(($# > 0)) || setup_error "-- requires a command"
			SUITE=("$@")
			break
			;;
		*)
			setup_error "unknown argument: $1 (see --help)"
			;;
	esac
done

REPO="$(git -C "$REPO" rev-parse --show-toplevel 2>/dev/null)" || setup_error "not a git checkout: $REPO"
REPO="$(cd "$REPO" && pwd -P)"

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum <"$1" | cut -d' ' -f1
	else
		shasum -a 256 <"$1" | cut -d' ' -f1
	fi
}

# fingerprint DIR OUT records everything a destructive test could change in
# the checkout DIR. With a third argument, it also archives the untracked
# files so they can be recovered.
fingerprint() {
	local dir="$1"
	local out="$2"
	local archive="${3:-}"
	local path

	mkdir -p "$out"
	git -C "$dir" rev-parse HEAD >"$out/head"
	git -C "$dir" symbolic-ref -q HEAD >"$out/branch" || true
	git -C "$dir" diff --binary >"$out/unstaged.diff"
	git -C "$dir" diff --cached --binary >"$out/staged.diff"
	git -C "$dir" ls-files --others --exclude-standard -z >"$out/untracked.z"
	while IFS= read -r -d '' path; do
		if [[ -L "$dir/$path" ]]; then
			printf 'symlink %s -> %s\n' "$path" "$(readlink "$dir/$path")"
		elif [[ -f "$dir/$path" ]]; then
			printf '%s  %s\n' "$(sha256_file "$dir/$path")" "$path"
		else
			printf 'other %s\n' "$path"
		fi
	done <"$out/untracked.z" | LC_ALL=C sort >"$out/untracked"
	if [[ -n "$archive" && -s "$out/untracked.z" ]]; then
		tar -C "$dir" --null -T "$out/untracked.z" -cf "$out/untracked.tar"
	fi
}

# compare_fingerprints LABEL BEFORE AFTER prints a report and returns 1 when
# any component differs.
compare_fingerprints() {
	local label="$1"
	local before="$2"
	local after="$3"
	local part
	local changed=0

	# Not the stash list: it is shared by every worktree of the repository, so
	# a sibling worktree's stash would be a false alarm. A stash made by the
	# suite still shows up as changed canaries and diffs.
	for part in head branch staged.diff unstaged.diff untracked; do
		if ! cmp -s "$before/$part" "$after/$part"; then
			if ((changed == 0)); then
				printf '\nSAFETY VIOLATION: the script tests changed the %s.\n' "$label" >&2
			fi
			changed=1
			printf -- '--- %s changed (first 40 lines of the difference):\n' "$part" >&2
			diff -u "$before/$part" "$after/$part" 2>&1 | sed -n '3,42p' >&2 || true
		fi
	done
	((changed == 0))
}

# canary_files DIR prints up to two tracked regular files under DIR in the
# throwaway checkout, preferring non-Go text files. Go files that are not
# tests get a trailing comment, which keeps them valid Go.
canary_files() {
	local dir="$1"

	git -C "$CHECKOUT" ls-files -s -- "$dir" |
		awk '$1 == "100644" { sub(/^[^\t]*\t/, ""); print }' |
		grep -Ev '(^|/)testdata/|_test\.go$' |
		grep -E '\.(md|cue|go)$' |
		awk '{ print ($0 ~ /\.go$/ ? 1 : 0) "\t" $0 }' | LC_ALL=C sort -s -k1,1 | cut -f2 | head -n 2 || true
}

plant_canaries() {
	local token="$1"
	local dir
	local file
	local staged=""
	local -a files=()

	: >"$WORK/canaries"
	for dir in cmd internal pkg; do
		mapfile -t files < <(canary_files "$dir")
		((${#files[@]} > 0)) || setup_error "no tracked text file under $dir/ to plant a canary in"
		printf '\n// %s unstaged %s\n' "$CANARY_TAG" "$token" >>"$CHECKOUT/${files[0]}"
		printf '%s\n' "${files[0]}" >>"$WORK/canaries"
		if [[ -z "$staged" && ${#files[@]} -gt 1 ]]; then
			staged="${files[1]}"
		fi
	done
	[[ -n "$staged" ]] || setup_error "no second tracked text file under cmd/, internal/ or pkg/ for the staged canary"
	printf '\n// %s staged %s\n' "$CANARY_TAG" "$token" >>"$CHECKOUT/$staged"
	git -C "$CHECKOUT" add -- "$staged"
	printf '%s\n' "$staged" >>"$WORK/canaries"

	file="cmd/.$CANARY_TAG-untracked"
	printf '%s untracked %s\n' "$CANARY_TAG" "$token" >"$CHECKOUT/$file"
	if git -C "$CHECKOUT" check-ignore -q -- "$file"; then
		setup_error "the untracked canary $file is ignored; pick a non-ignored name"
	fi
	printf '%s\n' "$file" >>"$WORK/canaries"

	mkdir -p "$WORK/canary-bytes"
	while IFS= read -r file; do
		mkdir -p "$WORK/canary-bytes/$(dirname "$file")"
		cp -p -- "$CHECKOUT/$file" "$WORK/canary-bytes/$file"
	done <"$WORK/canaries"
}

check_canaries() {
	local file
	local changed=0

	while IFS= read -r file; do
		if [[ ! -f "$CHECKOUT/$file" ]]; then
			printf 'CANARY: %s was deleted.\n' "$file" >&2
			changed=1
		elif ! cmp -s "$WORK/canary-bytes/$file" "$CHECKOUT/$file"; then
			printf 'CANARY: %s changed.\n' "$file" >&2
			changed=1
		fi
	done <"$WORK/canaries"
	((changed == 0))
}

print_recovery() {
	local before="$WORK/real-before"

	cat >&2 <<EOF

The real checkout ($REPO) changed while the script tests ran.
Nothing was restored automatically. Its state from before the run is kept in:
  $before
Compare it with the checkout, then re-apply only what was lost, for example:
  git -C '$REPO' status
  git -C '$REPO' apply --check --index '$before/staged.diff' && git -C '$REPO' apply --index '$before/staged.diff'
  git -C '$REPO' apply --check '$before/unstaged.diff' && git -C '$REPO' apply '$before/unstaged.diff'
EOF
	if [[ -f "$before/untracked.tar" ]]; then
		printf "  tar -tf '%s'   # untracked files; extract lost ones with: tar -C '%s' -xf '%s' PATH...\n" \
			"$before/untracked.tar" "$REPO" "$before/untracked.tar" >&2
	fi
	printf 'Delete %s when you are done.\n' "$WORK" >&2
}

remove_checkout() {
	[[ -n "$CHECKOUT" ]] || return 0
	if [[ -d "$CHECKOUT" ]] && git -C "$REPO" worktree remove --force -- "$CHECKOUT" >/dev/null 2>&1; then
		return 0
	fi
	# The suite deleted or broke the checkout: remove it and only its own
	# worktree entry (never `git worktree prune`, which touches every entry).
	rm -rf -- "$CHECKOUT"
	if [[ -n "$CHECKOUT_ADMIN" && "$CHECKOUT_ADMIN" == */worktrees/* && -d "$CHECKOUT_ADMIN" ]]; then
		rm -rf -- "$CHECKOUT_ADMIN"
	fi
}

cleanup() {
	local status=$?

	remove_checkout
	if [[ -n "$WORK" ]]; then
		if ((KEEP_REAL_BEFORE == 1)); then
			find "$WORK" -mindepth 1 -maxdepth 1 ! -name real-before -exec rm -rf -- {} +
		else
			rm -rf -- "$WORK"
		fi
	fi
	return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

WORK="$(mktemp -d "${TMPDIR:-/tmp}/invowk-script-tests.XXXXXX")"
WORK="$(cd "$WORK" && pwd -P)"
CHECKOUT="$WORK/checkout"

fingerprint "$REPO" "$WORK/real-before" archive
suite_status=0
violation=0

if ((IN_PLACE == 1)); then
	printf 'Running the script tests in place (unguarded): %s\n' "$REPO"
	(cd "$REPO" && "${SUITE[@]}") || suite_status=$?
else
	# No hooks: a post-checkout hook must not run against the throwaway checkout.
	git -C "$REPO" -c core.hooksPath=/dev/null worktree add --quiet --detach -- "$CHECKOUT" HEAD ||
		setup_error "could not create the throwaway checkout"
	CHECKOUT_ADMIN="$(git -C "$CHECKOUT" rev-parse --absolute-git-dir)"

	# Mirror the real checkout's work, so the suite tests what is on disk, not only
	# HEAD: staged changes into the index, unstaged ones on top, untracked files.
	if [[ -s "$WORK/real-before/staged.diff" ]]; then
		git -C "$CHECKOUT" apply --index --whitespace=nowarn "$WORK/real-before/staged.diff" ||
			setup_error "could not mirror the staged changes into the throwaway checkout"
	fi
	if [[ -s "$WORK/real-before/unstaged.diff" ]]; then
		git -C "$CHECKOUT" apply --whitespace=nowarn "$WORK/real-before/unstaged.diff" ||
			setup_error "could not mirror the unstaged changes into the throwaway checkout"
	fi
	if [[ -f "$WORK/real-before/untracked.tar" ]]; then
		tar -C "$CHECKOUT" -xf "$WORK/real-before/untracked.tar"
	fi

	plant_canaries "$$-$RANDOM"
	fingerprint "$CHECKOUT" "$WORK/checkout-before"

	printf 'Running the script tests in a throwaway checkout: %s\n' "$CHECKOUT"
	printf '  (canaries planted in: %s)\n' "$(paste -sd' ' "$WORK/canaries")"
	(cd "$CHECKOUT" && "${SUITE[@]}") || suite_status=$?

	if [[ -d "$CHECKOUT" ]]; then
		fingerprint "$CHECKOUT" "$WORK/checkout-after"
		check_canaries || violation=1
		compare_fingerprints "throwaway checkout (its canaries stand in for uncommitted work)" \
			"$WORK/checkout-before" "$WORK/checkout-after" || violation=1
	else
		printf '\nSAFETY VIOLATION: the script tests deleted the throwaway checkout %s.\n' "$CHECKOUT" >&2
		violation=1
	fi
fi

fingerprint "$REPO" "$WORK/real-after"
if ! compare_fingerprints "real checkout $REPO" "$WORK/real-before" "$WORK/real-after"; then
	violation=1
	KEEP_REAL_BEFORE=1
	print_recovery
fi

if ((violation == 1)); then
	printf '\nFAILED: a script test discarded or created work in a checkout (suite exit status %s).\n' "$suite_status" >&2
	printf 'Run destructive helpers only against a temporary repository (see make_fixture_repo in scripts/test_mutation.sh).\n' >&2
	exit "$EXIT_VIOLATION"
fi
if ((suite_status != 0)); then
	printf '\nScript tests failed (exit status %s); no checkout was changed.\n' "$suite_status" >&2
	exit "$EXIT_SUITE_FAILED"
fi
printf '\nScript tests passed and left every checkout unchanged.\n'
