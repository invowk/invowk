# Release Recovery

Load this reference before deleting tags, rerunning failed releases, closing package PRs, or telling users a release is safe.

## First Triage

Capture facts before acting:

```bash
tag=v1.2.3
git fetch --tags origin
git rev-parse "$tag" || true
gh run list --workflow Release --limit 10
gh run watch <run-id>
gh release view "$tag" --json tagName,isDraft,isPrerelease,url,assets,body || true
gh release view --json tagName --jq '.tagName' || true
gh run view <run-id> --log > /tmp/release.log
rg -n "::error|ERROR|failed|GoReleaser|Bencher|WinGet|Homebrew|Cosign|release-notes|tag" /tmp/release.log
```

Classify the release state:

- No tag exists.
- Local tag exists only.
- Remote tag exists, no GitHub Release/assets.
- GitHub Release/assets exist.
- Homebrew tap was updated.
- WinGet PR was opened or merged.
- Docs versioning committed or failed.

## Rollback Rules

- If no tag exists: fix inputs and rerun.
- If only a local tag exists: `git tag -d <tag>` is usually enough.
- If a remote tag exists but no public release/assets/package updates exist: deleting the remote tag can be acceptable after maintainer confirmation.
- If GitHub Release assets or package-channel updates exist: do not move or reuse the tag. Publish a corrective release, yank visibly, or repair package channels in place.
- Never silently edit release notes to hide a known issue. Add a clear note with status and remediation.

Remote tag deletion, only when safe and approved:

```bash
git push origin ":refs/tags/<tag>"
git tag -d <tag>
```

## Common Failures

### Release notes missing or invalid

The helper fails before tag creation when `RELEASE_NOTES_FILE` is missing, empty, non-markdown, or not a regular file. Fix the file and rerun. If a manual workflow dispatch failed validation, rerun with markdown notes input.

### Local release helper fails after creating a signed tag

If push failed and the tag is local only, delete the local tag, fix the cause, and rerun:

```bash
git tag -d <tag>
```

If the remote tag was pushed, inspect whether the release workflow published anything before deciding whether deletion is safe.

### Tests fail after a tag push

No GoReleaser publish occurs. Inspect test artifacts and logs. If no GitHub Release/assets exist, either rerun after fixing CI or delete/recreate the tag with maintainer confirmation. If the tag came from a workstation, remember the signed annotated tag already exists remotely.

### Bencher blocks release

Release benchmarking runs before GoReleaser. If `bencher run` fails or alerts, no release assets should be published yet. Use the `bencher` skill, inspect the report URL, and decide whether the regression is real. Do not weaken thresholds to force a release.

### GitHub App token or docs secrets fail

The release workflow needs `DOCS_APP_ID` and `DOCS_APP_PRIVATE_KEY` before GoReleaser so the release event can trigger docs versioning. Fix secrets and rerun the workflow. Avoid switching to plain `GITHUB_TOKEN` unless the user explicitly accepts that docs automation may not trigger.

### Cosign fails

Check `id-token: write`, `sigstore/cosign-installer`, and the `signs` step. If GoReleaser did not publish, fix and rerun. If partial assets exist, inspect the GitHub Release before replacing or deleting anything.

### GoReleaser publishes GitHub Release but Homebrew fails

Check `HOMEBREW_TAP_TOKEN` and the `invowk/homebrew-tap` state. Since GoReleaser uses `release.mode: replace`, a workflow rerun may repair GitHub Release assets and retry the tap. If the tap has a bad commit, revert or correct it in the tap repo and link the fix from release notes if users were affected.

### WinGet PR is missing

Prereleases intentionally skip WinGet. For stable releases, check `WINGET_TOKEN`, GoReleaser logs, and branch `invowk-<version-without-v>` in `invowk/winget-pkgs`. If GoReleaser created no PR but GitHub Release is good, fix token/config and rerun or manually submit the manifest.

### WinGet enhancement fails

The release may still be usable from GitHub Release/Homebrew. Rerun:

```bash
GH_TOKEN=<token> bash scripts/enhance-winget-manifest.sh <version-without-v>
```

If the PR already exists, update it. If it was merged with bad metadata, submit a corrective PR to `microsoft/winget-pkgs`.

### Version docs fail

The release can be published while docs versioning fails afterward. Fix docs generation/parity/build issues and rerun `version-docs.yml` with the tag. If needed locally:

```bash
make version-docs VERSION=<version-without-v>
```

Then validate docs parity and website build.

## User Communication

When a release is delayed or repaired:

- State which channels are healthy and which are pending.
- Give a direct workaround, such as install script with `INVOWK_VERSION=<tag>` or downloading GitHub Release assets.
- Avoid vague "package managers may lag" language when you know the specific channel.
- Update the GitHub Release body if users may already have seen the release.
