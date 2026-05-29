## ADDED Requirements

### Requirement: Release notes file is mandatory for real releases
The release command interface SHALL require `RELEASE_NOTES_FILE` for every real release tag created through the supported make release targets.

#### Scenario: Missing release notes file argument
- **WHEN** a maintainer runs a real release through `make release` or `make release-bump` without `RELEASE_NOTES_FILE`
- **THEN** the command MUST fail before computing or creating a release tag
- **AND** the error MUST explain that `RELEASE_NOTES_FILE=<path>` is required

#### Scenario: Dry run validates required release notes file
- **WHEN** a maintainer runs `make release` or `make release-bump` with `DRY_RUN=1`
- **THEN** the command MUST still require and validate `RELEASE_NOTES_FILE`
- **AND** the command MUST avoid creating or pushing a tag

### Requirement: Release notes file input is validated before tagging
The release helper SHALL validate the release notes file before creating or pushing any release tag.

#### Scenario: Release notes path does not exist
- **WHEN** `RELEASE_NOTES_FILE` points to a missing path
- **THEN** the release command MUST fail before tag creation
- **AND** the error MUST identify the missing path

#### Scenario: Release notes path is not a regular markdown file
- **WHEN** `RELEASE_NOTES_FILE` points to a directory, special file, or file without a markdown extension
- **THEN** the release command MUST fail before tag creation
- **AND** the error MUST describe the expected markdown file input

#### Scenario: Release notes file is empty
- **WHEN** `RELEASE_NOTES_FILE` points to an empty file
- **THEN** the release command MUST fail before tag creation
- **AND** the error MUST explain that release notes cannot be empty

### Requirement: Release notes content is bound to the release tag
The release process SHALL carry the validated markdown release notes content through a durable tag-bound handoff that is available to CI after the tag is pushed.

#### Scenario: Real release tag is created
- **WHEN** a maintainer confirms a real release
- **THEN** the signed annotated release tag MUST include the validated release notes content
- **AND** CI MUST be able to recover that content from the checked-out tag without reading a local maintainer path

#### Scenario: Release notes include multiline markdown
- **WHEN** the release notes file contains headings, lists, links, fenced code blocks, or blank lines
- **THEN** the tag-bound handoff MUST preserve the markdown content without shell interpolation loss

### Requirement: GitHub Release page uses provided release notes
The GitHub release publishing workflow SHALL publish the provided markdown release notes content as the body shown on the GitHub Releases page.

#### Scenario: GoReleaser publishes a release
- **WHEN** the release workflow invokes GoReleaser for a real release
- **THEN** the workflow MUST pass the recovered release notes markdown through GoReleaser's release-notes file input
- **AND** the GitHub Release body MUST use that provided markdown content instead of silently using a generated changelog

#### Scenario: Manual workflow dispatch attempts real publishing
- **WHEN** the release workflow is manually dispatched for a non-dry-run publication path
- **THEN** the workflow MUST require an equivalent release-notes input or fail before publishing
- **AND** the workflow MUST NOT publish generated release notes as a fallback

### Requirement: Release help documents release notes input
The release command documentation in the repository SHALL show `RELEASE_NOTES_FILE` as a required input for release make targets.

#### Scenario: Maintainer reads make help
- **WHEN** a maintainer runs `make help`
- **THEN** the release and release-bump examples MUST show `RELEASE_NOTES_FILE=<path>`
- **AND** the environment variable list MUST describe the required release notes file input
