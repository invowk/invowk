# Squash Merge

Merging into `main` is squash-only by project policy (`.agents/rules/git.md`), and only through a pull request: the "Safety" ruleset has a pull-request rule (0 approvals, every review option off) besides signatures, linear history and three required checks (`goplint consumer gates`, `Models, golden vectors, and correspondence` and `script tests (worktree-safety gate)`). A local `git merge --squash` pushed to `main` is rejected. The repository settings still enable merge-commit and rebase merging, so pick squash explicitly, in the UI or with `gh`:

```bash
gh pr merge <number> --squash \
  --match-head-commit <reviewed-head-sha> \
  --subject 'feat(scope): summary (#<number>)' \
  --body-file squash-message.md
```

- `--match-head-commit` makes GitHub refuse the merge when the branch head is no longer the commit that was reviewed and tested.
- The body file holds the 3-6 bullets the commit rules ask for, followed by the trailers (`Co-authored-by: Name <email>`).
- GitHub creates and signs the squash commit, which satisfies the signature rule.
- Only the `invowk-docs-bot` App (`version-docs.yml`) bypasses the ruleset.

After the merge, delete the local branch (`git branch -D feature-branch`: a squash merge leaves it unmerged in git's view) and the remote one (`gh pr merge --delete-branch`, or `git push origin --delete feature-branch`). Delete only branches the current session created.

Release tags (`refs/tags/v*`) have their own ruleset, "Release tags": creation, update and deletion are blocked for everyone but the repository admin role. `make release` from the maintainer's machine creates them. Workflow tokens and App tokens cannot; an admin's account token (a PAT) carries the admin bypass.

Why squash: each feature lands as one signed, atomic commit with a cohesive message, which keeps `main` linear and makes reverts easy.
