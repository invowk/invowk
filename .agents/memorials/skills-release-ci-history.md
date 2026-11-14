# Release, CI and dependency skills history

Historical notes moved out of the release, CI and dependency skills on 2026-11-14. None of this is normative; the current skills win.

- **Changelog and GitHub-native notes.** The changelog skill used to say that Invowk releases publish GoReleaser's GitHub-native release notes (`changelog.use: github-native` in `.goreleaser.yaml`, categories in `.github/release.yml`) and that its own output was a draft to compare against them. Real releases now pass the reviewed, tag-bound notes to GoReleaser with `--release-notes` (`release.yml`), so the GitHub-native generator and its categories are not used; the changelog skill now feeds the publish-release notes instead. Older tag-format docs: the "Release docs" bullet in `tooling-and-ci-history.md`.
- **Pre-`make vulncheck` fallback.** The dep-audit skill once told agents to fall back to a manual per-module `govulncheck` loop when `make vulncheck` was missing in an older checkout. The target (`scripts/govulncheck-all.sh`) exists, so the loop survives only as a per-module debugging aid.
