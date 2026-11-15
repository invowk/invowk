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
#   1. It fingerprints the real checkout: staged and unstaged binary diffs,
#      untracked non-ignored files with their content, HEAD, the branch and
#      the safety-relevant local git config (hooks, signing, identity).
#   2. It creates a throwaway repository that is NOT linked to the real one:
#      `git init` plus an alternates file that borrows the real object store
#      read-only, a copy of the refs (except the stash) and no remote. Config,
#      refs, stash and worktree list are therefore private to it. It checks out
#      HEAD detached, mirrors the real checkout's staged, unstaged and untracked
#      work into it, and plants canaries: an unstaged edit to a tracked text
#      file and an untracked file in every top-level tracked directory and at
#      the root, one staged edit, one stash entry and one branch.
#   3. It runs the suite (default: bash scripts/script-tests.sh) INSIDE the
#      throwaway checkout, then compares the canary bytes, both diffs, the
#      untracked set, the throwaway's config, refs and stash list against step
#      2, and the real checkout against step 1. Any difference fails the gate.
#
# It never restores anything automatically. When the real checkout changed, or
# its state could no longer be read, it keeps the step-1 fingerprint
# (including copies of the untracked files) and prints the commands that
# recover it. The throwaway repository is always removed.
#
# The real checkout's stash, refs and worktree list are not compared: sibling
# worktrees change them concurrently. The throwaway repository's private copies
# of them are compared instead, which catches a suite that touches them.
#
# --in-place (make test-scripts-unguarded) skips steps 2-3: it runs the suite
# in the real checkout and only compares the step-1 fingerprint afterwards, so
# damage is still detected and recoverable, though no longer prevented (and a
# change to the shared stash, refs or worktree list goes unnoticed).
#
# Usage: check-script-tests-worktree-safety.sh [--repo DIR] [--in-place] [-- COMMAND...]
#   --repo DIR   checkout to guard (default: this script's repository)
#   --in-place   run the suite in the checkout itself, not a throwaway one
#   COMMAND      suite to run (default: bash scripts/script-tests.sh)
#
# Exit status: 0 suite passed and nothing changed; 1 suite failed and nothing
# changed; 2 usage or setup error; 3 a checkout changed or could no longer be
# read (whatever the suite's status).

set -euo pipefail

# Never let an inherited git environment (for example a git hook's) redirect
# the commands below to another repository or index.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

readonly EXIT_SUITE_FAILED=1
readonly EXIT_SETUP=2
readonly EXIT_VIOLATION=3
readonly CANARY_TAG="invowk-script-tests-worktree-safety-canary"
readonly CANARY_BRANCH="$CANARY_TAG-branch"
# Local config keys whose change would weaken every later git operation in the
# real checkout. Only these are compared: sibling worktrees legitimately write
# others (`git push -u` and `gh` add branch.* entries).
readonly GUARDED_CONFIG_KEYS='^(core\.(hookspath|bare|worktree|sshcommand)|commit\.gpgsign|tag\.gpgsign|gpg\..*|user\..*|include\..*|includeif\..*)$'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$SCRIPT_DIR/.."
SUITE=(bash scripts/script-tests.sh)

WORK=""
CHECKOUT=""
KEEP_REAL_BEFORE=0
IN_PLACE=0
SUITE_STARTED=0
REAL_CHECKED=0

setup_error() {
	printf 'ERROR: %s\n' "$*" >&2
	exit "$EXIT_SETUP"
}

usage() {
	sed -n '/^# Usage:/,/^# read (whatever/s/^# \{0,1\}//p' "${BASH_SOURCE[0]}"
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

# hash_untracked DIR LIST prints a sorted "hash  path" line for every path in
# the NUL-separated LIST.
hash_untracked() {
	local dir="$1"
	local list="$2"
	local path

	while IFS= read -r -d '' path; do
		if [[ -L "$dir/$path" ]]; then
			printf 'symlink %s -> %s\n' "$path" "$(readlink "$dir/$path")"
		elif [[ -f "$dir/$path" ]]; then
			printf '%s  %s\n' "$(sha256_file "$dir/$path")" "$path"
		else
			printf 'other %s\n' "$path"
		fi
	done <"$list" | LC_ALL=C sort
}

# guarded_config DIR prints the GUARDED_CONFIG_KEYS entries of DIR's local
# config. `git config` exits 1 when none is set, which is not an error.
guarded_config() {
	local status=0

	git -C "$1" config --local --null --get-regexp "$GUARDED_CONFIG_KEYS" | tr '\0' '\n' || status=$?
	((status <= 1))
}

# fingerprint DIR OUT KIND records everything a destructive test could change
# in the checkout DIR. KIND "real" also records the guarded config keys and
# archives the untracked files so they can be recovered; KIND "throwaway" also
# records the private config file, every ref and the stash list. It returns
# non-zero when any part cannot be read, so callers decide what that means
# (set -e is off inside `if !` and `||` contexts, hence the explicit checks).
fingerprint() {
	local dir="$1"
	local out="$2"
	local kind="$3"
	local git_dir

	mkdir -p "$out" || return 1
	git -C "$dir" rev-parse HEAD >"$out/head" || return 1
	{ git -C "$dir" symbolic-ref -q HEAD || true; } >"$out/branch"
	git -C "$dir" diff --binary >"$out/unstaged.diff" || return 1
	git -C "$dir" diff --cached --binary >"$out/staged.diff" || return 1
	git -C "$dir" ls-files --others --exclude-standard -z >"$out/untracked.z" || return 1
	hash_untracked "$dir" "$out/untracked.z" >"$out/untracked" || return 1
	case "$kind" in
		real)
			guarded_config "$dir" >"$out/config" || return 1
			if [[ -s "$out/untracked.z" ]]; then
				tar -C "$dir" --null -T "$out/untracked.z" -cf "$out/untracked.tar" || return 1
			fi
			;;
		throwaway)
			git_dir="$(git -C "$dir" rev-parse --absolute-git-dir)" || return 1
			cp -- "$git_dir/config" "$out/config" || return 1
			git -C "$dir" for-each-ref --format='%(objectname) %(refname)' >"$out/refs" || return 1
			git -C "$dir" stash list --format='%H %gs' >"$out/stash" || return 1
			;;
	esac
}

fingerprint_parts() {
	case "$1" in
		real) printf '%s\n' head branch staged.diff unstaged.diff untracked config ;;
		throwaway) printf '%s\n' head branch staged.diff unstaged.diff untracked config refs stash ;;
	esac
}

# compare_fingerprints LABEL KIND BEFORE AFTER prints a report and returns 1
# when any component differs.
compare_fingerprints() {
	local label="$1"
	local kind="$2"
	local before="$3"
	local after="$4"
	local part
	local changed=0

	while IFS= read -r part; do
		if ! cmp -s "$before/$part" "$after/$part"; then
			if ((changed == 0)); then
				printf '\nSAFETY VIOLATION: the script tests changed the %s.\n' "$label" >&2
			fi
			changed=1
			printf -- '--- %s changed (first 40 lines of the difference):\n' "$part" >&2
			diff -u "$before/$part" "$after/$part" 2>&1 | sed -n '3,42p' >&2 || true
		fi
	done < <(fingerprint_parts "$kind")
	((changed == 0))
}

# canary_comment FILE LABEL prints the comment line appended to FILE, or fails
# when its type has no comment syntax the gate knows to be harmless.
canary_comment() {
	local text="$CANARY_TAG $2"

	case "$1" in
		*.md) printf '\n<!-- %s -->\n' "$text" ;;
		*.txt) printf '\n%s\n' "$text" ;;
		*.go | *.cue | *.js | *.mjs | *.ts | *.tsx) printf '\n// %s\n' "$text" ;;
		*.sh | *.bash | *.py | *.yml | *.yaml | *.toml | *.tape | *.d2 | *.properties) printf '\n# %s\n' "$text" ;;
		*) return 1 ;;
	esac
}

# canary_files DIR prints up to two tracked regular text files directly in the
# root (DIR ".") or anywhere under DIR, Markdown and plain text first, then
# other commented types, Go last. Fixtures (testdata/), Go tests and MDX are
# never edited.
canary_files() {
	local dir="$1"
	local -a pathspec=(-- "$dir")

	[[ "$dir" == "." ]] && pathspec=(-- ':(glob)*')
	git -C "$CHECKOUT" ls-files -s "${pathspec[@]}" |
		awk '$1 == "100644" { sub(/^[^\t]*\t/, ""); print }' |
		grep -Ev '(^|/)testdata/|_test\.go$|\.mdx$' |
		awk '
			/\.(md|txt)$/ { print 0 "\t" $0; next }
			/\.go$/ { print 2 "\t" $0; next }
			/\.(cue|js|mjs|ts|tsx|sh|bash|py|yml|yaml|toml|tape|d2|properties)$/ { print 1 "\t" $0 }
		' | LC_ALL=C sort -s -k1,1 | cut -f2 | head -n 2 || true
}

canary_dirs() {
	git -C "$CHECKOUT" ls-files | awk -F/ 'NF > 1 { print $1 }' | LC_ALL=C sort -u
}

plant_canaries() {
	local token="$1"
	local dir
	local file
	local staged=""
	local stash
	local -a files=()

	: >"$WORK/canaries"
	while IFS= read -r dir; do
		mapfile -t files < <(canary_files "$dir")
		if ((${#files[@]} > 0)); then
			canary_comment "${files[0]}" "unstaged $token" >>"$CHECKOUT/${files[0]}"
			printf '%s\n' "${files[0]}" >>"$WORK/canaries"
			if [[ -z "$staged" && ${#files[@]} -gt 1 && "$dir" =~ ^(cmd|internal|pkg)$ ]]; then
				staged="${files[1]}"
			fi
		elif [[ "$dir" =~ ^(cmd|internal|pkg)$ ]]; then
			setup_error "no tracked text file under $dir/ to plant a canary in"
		fi

		if [[ "$dir" == "." ]]; then
			file=".$CANARY_TAG-untracked"
		else
			file="$dir/.$CANARY_TAG-untracked"
		fi
		[[ -L "$CHECKOUT/$dir" ]] && continue
		printf '%s untracked %s\n' "$CANARY_TAG" "$token" >"$CHECKOUT/$file"
		if git -C "$CHECKOUT" check-ignore -q -- "$file"; then
			rm -f -- "$CHECKOUT/$file"
			[[ ! "$dir" =~ ^(cmd|internal|pkg)$ ]] || setup_error "the untracked canary $file is ignored; pick a non-ignored name"
			continue
		fi
		printf '%s\n' "$file" >>"$WORK/canaries"
	done < <(printf '.\n'; canary_dirs)

	[[ -n "$staged" ]] || setup_error "no second tracked text file under cmd/, internal/ or pkg/ for the staged canary"
	canary_comment "$staged" "staged $token" >>"$CHECKOUT/$staged"
	git -C "$CHECKOUT" add -- "$staged"
	printf '%s\n' "$staged" >>"$WORK/canaries"

	# A stash entry and a branch that only the suite could remove. `stash
	# create` records the canaries without touching the checkout.
	stash="$(git -C "$CHECKOUT" stash create "$CANARY_TAG stash")"
	[[ -n "$stash" ]] || setup_error "could not create the canary stash entry"
	git -C "$CHECKOUT" stash store -m "$CANARY_TAG stash" "$stash"
	git -C "$CHECKOUT" branch -- "$CANARY_BRANCH" HEAD

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

# create_throwaway makes CHECKOUT an independent repository at the real HEAD.
# The alternates file lets it read the real object store without copying it;
# git never writes through alternates, and the throwaway's own objects, refs,
# config, stash and hooks live in its own .git. No remote is configured, so a
# stray push fails instead of reaching the real repository or GitHub.
create_throwaway() {
	local common
	local head

	common="$(git -C "$REPO" rev-parse --path-format=absolute --git-common-dir)" ||
		setup_error "could not locate the git directory of $REPO"
	head="$(git -C "$REPO" rev-parse HEAD)" || setup_error "could not read HEAD of $REPO"
	# An empty template: no sample or user-template hooks.
	git init -q --template= -- "$CHECKOUT" || setup_error "could not create the throwaway repository"
	mkdir -p "$CHECKOUT/.git/objects/info" "$CHECKOUT/.git/info"
	printf '%s\n' "$common/objects" >"$CHECKOUT/.git/objects/info/alternates"
	if [[ -f "$common/shallow" ]]; then
		cp -- "$common/shallow" "$CHECKOUT/.git/shallow"
	fi
	if [[ -f "$common/info/exclude" ]]; then
		cp -- "$common/info/exclude" "$CHECKOUT/.git/info/exclude"
	fi
	# Tags and branches matter to the release tests; the real stash does not.
	git -C "$REPO" for-each-ref --format='create %(refname) %(objectname)' |
		awk '$2 != "refs/stash"' |
		git -C "$CHECKOUT" update-ref --stdin ||
		setup_error "could not copy the refs into the throwaway repository"
	git -C "$CHECKOUT" checkout -q --detach "$head" ||
		setup_error "could not check out $head in the throwaway repository"
}

print_recovery() {
	local before="$WORK/real-before"

	cat >&2 <<EOF

The real checkout ($REPO) changed while the script tests ran, or its state could
not be read afterwards. Nothing was restored automatically. Its state from
before the run is kept in:
  $before
Compare it with the checkout, then re-apply only what was lost, for example:
  git -C '$REPO' status
  git -C '$REPO' apply --check --index '$before/staged.diff' && git -C '$REPO' apply --index '$before/staged.diff'
  git -C '$REPO' apply --check '$before/unstaged.diff' && git -C '$REPO' apply '$before/unstaged.diff'
  cat '$before/config'   # guarded local config; compare with: git -C '$REPO' config --local --list
EOF
	if [[ -f "$before/untracked.tar" ]]; then
		printf "  tar -tf '%s'   # untracked files; extract lost ones with: tar -C '%s' -xf '%s' PATH...\n" \
			"$before/untracked.tar" "$REPO" "$before/untracked.tar" >&2
	fi
	printf 'Delete %s when you are done.\n' "$WORK" >&2
}

# check_real compares the real checkout with its step-1 fingerprint, at most
# once. On any difference, or when the state cannot be read, it keeps the
# step-1 fingerprint, prints the recovery commands and returns 1.
check_real() {
	((REAL_CHECKED == 0)) || return 0
	REAL_CHECKED=1
	if ! fingerprint "$REPO" "$WORK/real-after" real; then
		printf '\nSAFETY VIOLATION: the state of the real checkout %s could not be read after the script tests.\n' "$REPO" >&2
	elif compare_fingerprints "real checkout $REPO" real "$WORK/real-before" "$WORK/real-after"; then
		return 0
	fi
	KEEP_REAL_BEFORE=1
	print_recovery
	return 1
}

cleanup() {
	local status=$?

	# An interrupt or an unexpected error after the suite started must not skip
	# the real-checkout comparison, nor delete the data that recovers it.
	if ((SUITE_STARTED == 1 && REAL_CHECKED == 0)); then
		check_real || status="$EXIT_VIOLATION"
	fi
	if [[ -n "$CHECKOUT" ]]; then
		rm -rf -- "$CHECKOUT"
	fi
	if [[ -n "$WORK" ]]; then
		if ((KEEP_REAL_BEFORE == 1)); then
			find "$WORK" -mindepth 1 -maxdepth 1 ! -name real-before -exec rm -rf -- {} +
		else
			rm -rf -- "$WORK"
		fi
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

WORK="$(mktemp -d "${TMPDIR:-/tmp}/invowk-script-tests.XXXXXX")"
WORK="$(cd "$WORK" && pwd -P)"

fingerprint "$REPO" "$WORK/real-before" real || setup_error "could not fingerprint the real checkout $REPO"
suite_status=0
violation=0

if ((IN_PLACE == 1)); then
	printf 'Running the script tests in place (unguarded): %s\n' "$REPO"
	SUITE_STARTED=1
	(cd "$REPO" && "${SUITE[@]}") || suite_status=$?
else
	CHECKOUT="$WORK/checkout"
	create_throwaway

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
	fingerprint "$CHECKOUT" "$WORK/checkout-before" throwaway || setup_error "could not fingerprint the throwaway checkout"

	printf 'Running the script tests in a throwaway checkout: %s\n' "$CHECKOUT"
	printf '  (%d canaries planted, plus a stash entry and the branch %s)\n' "$(wc -l <"$WORK/canaries")" "$CANARY_BRANCH"
	SUITE_STARTED=1
	(cd "$CHECKOUT" && "${SUITE[@]}") || suite_status=$?

	if [[ ! -d "$CHECKOUT" ]]; then
		printf '\nSAFETY VIOLATION: the script tests deleted the throwaway checkout %s.\n' "$CHECKOUT" >&2
		violation=1
	elif ! fingerprint "$CHECKOUT" "$WORK/checkout-after" throwaway; then
		printf '\nSAFETY VIOLATION: the script tests broke the throwaway checkout %s (its state could not be read).\n' "$CHECKOUT" >&2
		violation=1
	else
		check_canaries || violation=1
		compare_fingerprints "throwaway checkout (its canaries stand in for uncommitted work)" throwaway \
			"$WORK/checkout-before" "$WORK/checkout-after" || violation=1
	fi
fi

check_real || violation=1

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
