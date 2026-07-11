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

Use the current repository workflows as executable examples only after the
checks above; do not copy an old static snippet from this skill.
