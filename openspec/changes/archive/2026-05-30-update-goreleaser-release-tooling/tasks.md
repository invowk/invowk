## 1. Version Track Synchronization

- [x] 1.1 Update every `goreleaser/goreleaser-action` `version:` input in `.github/workflows/ci.yml` from `~> v2.15` to `~> v2.16`.
- [x] 1.2 Update every `goreleaser/goreleaser-action` `version:` input in `.github/workflows/release.yml` from `~> v2.15` to `~> v2.16`.
- [x] 1.3 Update `.agents/rules/version-pinning.md` so the GoReleaser current track documents `~> v2.16`.
- [x] 1.4 Search workflow and agent-rule files for `~> v2.15` and confirm no stale GoReleaser track references remain.

## 2. Homebrew Cask Completion Metadata

- [x] 2.1 Add `generate_completions_from_executable` to the `homebrew_casks` entry in `.goreleaser.yaml`.
- [x] 2.2 Configure completion generation to run the installed `invowk` executable with the existing `completion` subcommand.
- [x] 2.3 Configure Cobra shell parameter handling and include bash, zsh, fish, and PowerShell unless validation rejects one of those shells.
- [x] 2.4 Preserve the existing Homebrew cask repository, stable-release `skip_upload: auto`, binary install, quarantine hook, and commit message behavior.

## 3. Release Contract Preservation

- [x] 3.1 Confirm the release workflow still passes `--release-notes=${{ steps.release-notes.outputs.path }}` to real and manual dry-run GoReleaser release paths.
- [x] 3.2 Confirm `.goreleaser.yaml` still signs `checksums.txt` with the existing Cosign keyless signing configuration.
- [x] 3.3 Confirm the WinGet enhancement workflow step still runs after real non-dry-run GoReleaser publishing.
- [x] 3.4 Avoid archive format, Docker image publishing, Node builder, Source RPM, Flatpak, Nix, Cosign, UPX, and install-script changes unless validation proves they are required.

## 4. Validation

- [x] 4.1 Run `go run github.com/goreleaser/goreleaser/v2@v2.16.0 check` and resolve any schema or deprecation findings.
- [x] 4.2 Run a GoReleaser snapshot release dry run with the v2.16 track and dry-run Homebrew/WinGet tokens; inspect generated Homebrew cask output for completion generation.
- [x] 4.3 Run `sh scripts/test_enhance_winget_manifest.sh` to verify the post-GoReleaser WinGet enhancement contract remains intact.
- [x] 4.4 Run `make check-agent-docs` because `.agents/rules/version-pinning.md` changes.
- [x] 4.5 Run `openspec validate update-goreleaser-release-tooling --strict`.
- [x] 4.6 Run `git diff --check` before finalizing the implementation.
