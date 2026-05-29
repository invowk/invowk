## 1. Local Release Interface

- [x] 1.1 Update `Makefile` release and release-bump usage comments to require `RELEASE_NOTES_FILE=<path>`.
- [x] 1.2 Pass `RELEASE_NOTES_FILE` from both make targets into `scripts/release.sh`.
- [x] 1.3 Update `make help` environment-variable descriptions and examples to show the required release notes input.

## 2. Release Script Validation and Tagging

- [x] 2.1 Extend `scripts/release.sh` tag and bump argument parsing to receive the release notes file path without breaking existing `YES` and `DRY_RUN` semantics.
- [x] 2.2 Add release notes validation for missing argument, missing path, non-regular file, non-markdown extension, and empty file before prerequisite checks create or push tags.
- [x] 2.3 Include the validated release notes path and computed version in dry-run output.
- [x] 2.4 Create signed annotated release tags with the validated markdown release notes content as the durable tag-bound handoff.

## 3. GitHub Actions Publication

- [x] 3.1 Add a release workflow step that extracts the release tag annotation into a temporary markdown file without including tag signatures.
- [x] 3.2 Pass the extracted markdown file to GoReleaser publish with `--release-notes=<file>`.
- [x] 3.3 Pass the extracted markdown file to the GoReleaser dry-run path when manual dry runs execute the release job.
- [x] 3.4 Ensure non-dry-run manual workflow dispatch cannot publish without an equivalent release-notes source.

## 4. Release Configuration

- [x] 4.1 Verify `.goreleaser.yaml` release header/footer behavior composes correctly with `--release-notes`.
- [x] 4.2 Adjust GoReleaser changelog or release configuration only if needed so provided markdown is the authoritative GitHub Release body.

## 5. Tests and Verification

- [x] 5.1 Add shell test coverage for missing, missing-path, non-regular, non-markdown, empty, dry-run, and successful release notes validation paths.
- [x] 5.2 Add coverage for preserving multiline markdown content in the signed tag handoff or extraction helper.
- [x] 5.3 Run targeted shell tests for the release helper.
- [x] 5.4 Run `make lint-scripts` or the repository's shell lint gate covering `scripts/release.sh`.
- [x] 5.5 Run `openspec validate require-release-notes-file --strict`.
