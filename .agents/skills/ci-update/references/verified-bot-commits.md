# Verified Bot Commits

Load this reference when auditing or changing a workflow that creates commits.

## Contract

The `required_signatures` ruleset rejects unsigned commits. Do not use runner
`git commit` plus `git push` for CI-created commits. Use GitHub's GraphQL
`createCommitOnBranch` mutation so GitHub creates the commit, and retain
`expectedHeadOid` for optimistic concurrency.

Use `${{ github.token }}` when the commit must not trigger another workflow.
Use the repository's GitHub App token when a downstream workflow must run.
Derive this choice from the actual workflow chain.

## Validation

Build the current inventory instead of trusting historical job names:

```bash
rg -n 'createCommitOnBranch|git commit|git push|git tag' .github/workflows
```

For each commit-producing job, verify:

1. The mutation targets the intended repository and branch.
2. `expectedHeadOid` is the checked-out head.
3. Additions and deletions represent the complete intended change.
4. Large content is passed through files/`--rawfile`, not shell arguments.
5. The result is checked for GraphQL errors and a non-empty commit OID.
6. The chosen token has the required contents permission and desired workflow
   trigger behavior.

`gh api graphql --input -` accepts a JSON request body on standard input. Build
that body with `jq`, keep the GraphQL query and variables separate, and fail if
the response contains `.errors` or lacks `.data.createCommitOnBranch.commit.oid`.

Live examples, to use only after verifying them against the six checks above:
the `Commit PGO profile (verified)` step of the `PGO Sanity` job in
`.github/workflows/ci.yml` (`github.token`, no downstream trigger) and the
`Commit and push` step in `.github/workflows/version-docs.yml` (App token).
Neither meets check 5 today: both only print
`.data.createCommitOnBranch.commit` with `jq` and rely on `gh`'s exit status
(the ci.yml step under `set -euo pipefail`, the version-docs.yml step under the
default `bash -e` shell). Add the explicit `.errors`/empty-oid check rather than
copying that gap.
