## ADDED Requirements

### Requirement: Locked versions reproduce their locked content
When `module sync`, `module add`, `module update`, or `module tidy` resolves a requirement to the same resolved version recorded in the existing lock file, Invowk SHALL verify that the fetched module has the locked content hash. The check SHALL apply whether or not the module's cache directory already exists.

#### Scenario: Fresh cache with matching content
- **WHEN** the cache directory for a locked module version does not exist, and the fetched content hashes to the locked `content_hash`
- **THEN** sync SHALL succeed and SHALL keep the locked `content_hash` unchanged

#### Scenario: Fresh cache with different content
- **WHEN** the cache directory for a locked module version does not exist, and the fetched content hashes to a value different from the locked `content_hash`
- **THEN** sync SHALL fail with a content-hash mismatch error that names the module key, version, expected hash, and actual hash
- **THEN** the newly populated cache directory SHALL be removed before the error is returned
- **THEN** the lock file SHALL NOT be written

#### Scenario: Existing cache with different content
- **WHEN** the cache directory for a locked module version exists, and its content hashes to a value different from the locked `content_hash`
- **THEN** sync SHALL fail with the content-hash mismatch error, and the lock file SHALL NOT be written

### Requirement: Locked versions reproduce their locked commit
When a requirement resolves to the same resolved version recorded in the existing lock file, and that entry has a non-empty `git_commit`, Invowk SHALL fail if the fetched commit differs from the locked commit.

#### Scenario: Re-pointed tag
- **WHEN** the remote serves the locked resolved version at a commit different from the locked `git_commit`
- **THEN** sync SHALL fail with a locked-commit mismatch error that names the module key, version, locked commit, and fetched commit
- **THEN** the error SHALL explain that an intentional re-tag is accepted by running `invowk module remove <key>` and adding the module again, so the generated lock file is never edited by hand
- **THEN** the lock file SHALL NOT be written

#### Scenario: v1.0 entry without a hash
- **WHEN** the locked entry has a `git_commit` but no `content_hash`
- **THEN** the commit check SHALL still apply, and no hash expectation SHALL be applied

### Requirement: Version changes do not carry stale expectations
When a requirement resolves to a version different from the resolved version recorded in the existing lock file, Invowk SHALL NOT apply the old entry's commit or content hash to the new version. It SHALL record the new version's commit and hash.

#### Scenario: Constraint changed in invowkmod.cue
- **WHEN** the requirement's version constraint changes and sync resolves a new version
- **THEN** sync SHALL succeed without a commit or hash comparison against the old entry, and the lock file SHALL record the new resolved version, commit, and hash

#### Scenario: New version already cached
- **WHEN** the new resolved version already exists in the module cache, for example from another project
- **THEN** its cached content SHALL NOT be compared with the old version's hash, and no false mismatch SHALL be reported

#### Scenario: Update with no newer version
- **WHEN** `module update` resolves the same version that is already locked
- **THEN** the commit and content-hash checks SHALL apply as for sync

### Requirement: New modules are recorded with a warning
When no existing lock entry matches a requirement's key, Invowk SHALL record the fetched commit and content hash as the new baseline and SHALL keep emitting the first-sync integrity warning.

#### Scenario: First sync of a module
- **WHEN** a requirement has no lock entry
- **THEN** sync SHALL succeed, SHALL write the commit and hash to the lock file, and SHALL log that the module was cached without an integrity baseline

### Requirement: Documentation describes sync-time integrity
The lock-file documentation SHALL state that re-syncing a locked version verifies both its commit and its content hash independent of cache state. It SHALL describe how version changes are handled and how to accept an intentional upstream re-tag.

#### Scenario: Lock-file page
- **WHEN** a reader opens the lock-file page in English or pt-BR
- **THEN** the `git_commit` and `content_hash` rows and the usage notes SHALL describe the sync-time checks and the re-tag procedure
