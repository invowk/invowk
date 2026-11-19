#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# change-flow-foreign-commits.sh <worktree> <branch> <base>
#
# Prints, oldest first, the commits on origin/<branch> that have no
# patch-equivalent commit on the local <branch> and are not on <base>. These are
# commits someone else pushed (for example the CI PGO bot's), and the change-flow
# fix agent must cherry-pick them before it force-pushes.
#
# A stopped run that rebased the branch locally leaves pre-rebase copies on
# origin/<branch>. Those copies are patch-equivalent, so they are not printed and
# never replayed. Ancestry is the wrong test here: after a rebase every local
# commit has a new SHA, so origin/<branch> is "not an ancestor" even when nobody
# else pushed.
#
# Exit status:
#   0  the commits to cherry-pick, if any (no output means nothing to integrate,
#      or the branch was never pushed)
#   3  origin/<branch> has a merge commit the local branch lacks, for example
#      GitHub's "Update branch" or a web conflict resolution. The merges are
#      printed as "merge <sha> ..." lines. A force push would drop the merge and
#      any edits made in it, so the agent stops and lists them for the
#      maintainer.
#   2  usage error
#
# A commit whose patch changed while a rebase conflict was resolved also appears
# in the exit-0 list. The agent must then stop and list it for the maintainer
# instead of cherry-picking it (see .agents/runbooks/change-flow.md).
#
# Run `git -C <worktree> fetch --prune origin` first, so that a deleted remote
# branch leaves no stale remote-tracking ref.

set -euo pipefail

if [[ $# -ne 3 ]]; then
	echo "usage: $0 <worktree> <branch> <base>" >&2
	exit 2
fi
worktree="$1"
branch="$2"
base="$3"
remote="refs/remotes/origin/$branch"

if ! git -C "$worktree" rev-parse --verify --quiet "$remote" >/dev/null; then
	exit 0
fi
merges="$(git -C "$worktree" log --merges --format='merge %H %an: %s' "$branch..$remote" "^$base")"
if [[ -n "$merges" ]]; then
	printf '%s\n' "$merges"
	exit 3
fi
git -C "$worktree" log --cherry-pick --right-only --no-merges --reverse --format='%H %an: %s' \
	"$branch...$remote" "^$base"
