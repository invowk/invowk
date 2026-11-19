#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Tests scripts/change-flow-foreign-commits.sh in throwaway repositories:
# - a bot commit only, a stopped-run rebase only, and both;
# - two bot commits, which must come out oldest first;
# - a merge of main pushed to the branch (GitHub "Update branch"): exit 3, and
#   main's commits are never listed as foreign;
# - a deleted remote branch after `fetch --prune`, and a branch never pushed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="$SCRIPT_DIR/change-flow-foreign-commits.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Isolate from the user's git config (signing, hooks, templates).
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_CONFIG_NOSYSTEM=1

fail=0
check() {
	local name="$1" want="$2" got="$3"
	if [[ "$got" == "$want" ]]; then
		echo "ok - $name"
	else
		printf 'not ok - %s\n  want: %q\n  got:  %q\n' "$name" "$want" "$got"
		fail=1
	fi
}

ident() { git -C "$1" config user.email "$2@example.invalid" && git -C "$1" config user.name "$2"; }
commit() { echo "$3" >"$1/$2" && git -C "$1" add -A && git -C "$1" commit -q -m "$2"; }
# run <local>: the helper's output without SHAs, then its exit status.
run() {
	local out rc=0
	out="$("$HELPER" "$1" feat origin/main)" || rc=$?
	printf '%s|rc=%s' "$(printf '%s' "$out" | sed -E 's/^(merge )?[0-9a-f]{40} /\1/')" "$rc"
}

# setup <dir>: a remote with main and feat (two commits), a local clone on feat,
# and a bot clone.
setup() {
	local d="$1"
	mkdir -p "$d"
	git init -q -b main "$d/seed" && ident "$d/seed" dev
	commit "$d/seed" base 0
	git -C "$d/seed" checkout -q -b feat
	commit "$d/seed" a 1
	commit "$d/seed" b 2
	git clone -q --bare "$d/seed" "$d/remote.git"
	git -C "$d/remote.git" symbolic-ref HEAD refs/heads/main
	git clone -q -b feat "$d/remote.git" "$d/local" && ident "$d/local" dev
	git clone -q -b feat "$d/remote.git" "$d/bot" && ident "$d/bot" bot
}
bot_push() { commit "$1/bot" "$2" p && git -C "$1/bot" push -q origin feat; }
main_moves() {
	git -C "$1/bot" checkout -q main && commit "$1/bot" m 3 && git -C "$1/bot" push -q origin main
	git -C "$1/bot" checkout -q feat
}
local_rebases() { git -C "$1/local" fetch -q origin && git -C "$1/local" rebase -q origin/main; }

d="$TMP/bot"
setup "$d"
bot_push "$d" pgo
git -C "$d/local" fetch -q --prune origin
check "bot commit only" "bot: pgo|rc=0" "$(run "$d/local")"

d="$TMP/order"
setup "$d"
bot_push "$d" pgo1
bot_push "$d" pgo2
git -C "$d/local" fetch -q --prune origin
check "two bot commits, oldest first" "bot: pgo1
bot: pgo2|rc=0" "$(run "$d/local")"

d="$TMP/stopped"
setup "$d"
main_moves "$d"
local_rebases "$d"
check "stopped-run rebase only" "|rc=0" "$(run "$d/local")"

d="$TMP/both"
setup "$d"
bot_push "$d" pgo
main_moves "$d"
local_rebases "$d"
check "bot commit and stopped-run rebase" "bot: pgo|rc=0" "$(run "$d/local")"

d="$TMP/merge"
setup "$d"
main_moves "$d"
git -C "$d/bot" pull -q --no-rebase --no-edit origin main && git -C "$d/bot" push -q origin feat
bot_push "$d" pgo
git -C "$d/local" fetch -q --prune origin
check "merge of main pushed to the branch" "merge bot: Merge branch 'main' of $d/remote into feat|rc=3" "$(run "$d/local")"

d="$TMP/remote-rebased"
setup "$d"
main_moves "$d"
git -C "$d/bot" rebase -q origin/main && git -C "$d/bot" push -q -f origin feat
git -C "$d/local" fetch -q --prune origin
check "branch rebased onto newer main by someone else" "|rc=0" "$(run "$d/local")"

d="$TMP/deleted"
setup "$d"
git -C "$d/local" fetch -q origin
git -C "$d/bot" push -q origin --delete feat
git -C "$d/local" fetch -q --prune origin
check "deleted remote branch after fetch --prune" "|rc=0" "$(run "$d/local")"

d="$TMP/never"
mkdir -p "$d"
git init -q -b main "$d/local" && ident "$d/local" dev
commit "$d/local" base 0
git -C "$d/local" update-ref refs/remotes/origin/main HEAD
git -C "$d/local" checkout -q -b feat
check "never pushed" "|rc=0" "$(run "$d/local")"

rc=0
"$HELPER" only-one-arg >/dev/null 2>&1 || rc=$?
check "usage error" "2" "$rc"

exit "$fail"
