# Agent Workflows

The floor for every Workflow-script agent in this repository, whatever the template. The per-change template is `.agents/workflows/change-flow.js`; its procedure, modes and the probe facts behind this rule are in `.agents/runbooks/change-flow.md`.

- **Independent review.** Every change gets a reviewer that wrote none of its code, and the agent that applies review findings and runs the gates wrote none of it either.
- **Gates on the tree that merges.** Rebase on `origin/main` (or the given base) before the final gates, run them on that tree, and report the base SHA. The main session checks that it still equals `origin/main` before merging.
- **`code-review` from a workflow agent:** always with an explicit target and level (`high <branch>` or `high <pr>`), never with no target, never `--fix` or `--comment`, and only from an agent without a `schema` (with a schema the fork's output is lost). A "(none)" result with no explanation is a failed review, not zero findings.
- **Absolute paths.** Bash cwd resets to the session's primary directory between calls: use absolute paths, `git -C <worktree>`, and `cd <worktree> && ...` inside one command. Never rely on a bare cwd.
- **Main-checkout guard.** Every stage records `git status --porcelain`, the branch and `HEAD` of the primary checkout as its first and its last action. The script stops when they differ. A stage never edits, commits, stashes or checks out anything in the primary checkout.
- **Outward writes.** The only ones allowed are pushing the change's own branch (`--force-with-lease` only on that branch) and creating or updating its draft PR. Never: mark a PR ready or merge it, change SonarCloud issue status, change GitHub settings, rulesets, labels or comments, publish releases, or dispatch workflows. Put any such need in the result's `maintainer_asks`.
- **Heavy-command lock.** `make test`, `make lint`, `make test-scripts`, `make check-baseline`, `make formal`, mutation targets and `git commit` run under `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" <cmd>`. All worktrees share that lock.
- **Single-pass skills.** Workflow agents have `Skill` but not `Agent`, so `simplify` and `code-review high` run single-pass there. That is expected, not a failure.
