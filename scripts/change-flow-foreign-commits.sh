#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# change-flow-foreign-commits.sh <worktree> <branch>
#
# Prints, oldest first, the commits on origin/<branch> that have no
# patch-equivalent commit on the local <branch>: commits someone else pushed
# (for example the CI PGO bot's), which the change-flow fix agent must
# cherry-pick before it force-pushes. A stopped run that rebased the branch
# locally leaves pre-rebase copies on origin/<branch>; those are
# patch-equivalent and are not printed, so they are never replayed.
#
# Ancestry is the wrong test here: after a rebase every local commit has a new
# SHA, so origin/<branch> is "not an ancestor" even when nobody else pushed.
#
# Exit 0 with no output: nothing to integrate (or the branch was never pushed).
# A commit whose patch changed while resolving a rebase conflict is also
# printed; the agent must then stop and list it for the maintainer instead of
# cherry-picking it (see .agents/runbooks/change-flow.md).
# Run `git -C <worktree> fetch origin` first.

set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "usage: $0 <worktree> <branch>" >&2
	exit 2
fi
worktree="$1"
branch="$2"

if ! git -C "$worktree" rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null; then
	exit 0
fi
git -C "$worktree" log --cherry-pick --right-only --no-merges --reverse --format='%H %an: %s' \
	"$branch...refs/remotes/origin/$branch"
