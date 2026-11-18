#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Tests scripts/change-flow-foreign-commits.sh in throwaway repositories: a bot
# commit only, a stopped-run rebase only, both, and a never-pushed branch.

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
subjects() { "$HELPER" "$1" feat | sed 's/^[0-9a-f]* //'; }

# setup <dir>: a remote with main and feat (two commits), a local clone on feat,
# and a bot clone; prints nothing.
setup() {
	local d="$1"
	mkdir -p "$d"
	git init -q -b main "$d/seed" && ident "$d/seed" dev
	commit "$d/seed" base 0
	git -C "$d/seed" checkout -q -b feat
	commit "$d/seed" a 1
	commit "$d/seed" b 2
	git clone -q --bare "$d/seed" "$d/remote.git"
	git clone -q -b feat "$d/remote.git" "$d/local" && ident "$d/local" dev
	git clone -q -b feat "$d/remote.git" "$d/bot" && ident "$d/bot" bot
}
bot_push() { commit "$1/bot" pgo p && git -C "$1/bot" push -q origin feat; }
main_moves_and_local_rebases() {
	git -C "$1/seed" checkout -q main && commit "$1/seed" m 3
	git -C "$1/local" fetch -q "$1/seed" main:refs/remotes/seed/main
	git -C "$1/local" rebase -q refs/remotes/seed/main
}

d="$TMP/bot"
setup "$d"
bot_push "$d"
git -C "$d/local" fetch -q origin
check "bot commit only" "bot: pgo" "$(subjects "$d/local")"

d="$TMP/stopped"
setup "$d"
main_moves_and_local_rebases "$d"
git -C "$d/local" fetch -q origin
check "stopped-run rebase only" "" "$(subjects "$d/local")"

d="$TMP/both"
setup "$d"
bot_push "$d"
main_moves_and_local_rebases "$d"
git -C "$d/local" fetch -q origin
check "bot commit and stopped-run rebase" "bot: pgo" "$(subjects "$d/local")"

d="$TMP/never"
mkdir -p "$d"
git init -q -b main "$d/local" && ident "$d/local" dev
commit "$d/local" base 0
git -C "$d/local" checkout -q -b feat
check "never pushed" "" "$("$HELPER" "$d/local" feat)"

if "$HELPER" only-one-arg >/dev/null 2>&1; then
	check "usage error" "exit 2" "exit 0"
else
	check "usage error" "exit 2" "exit $?"
fi

exit "$fail"
