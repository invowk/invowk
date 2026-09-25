## ADDED Requirements

### Requirement: Hashless entries never vouch for vendored content
Discovery and `module vendor` SHALL reject a vendored module whose lock entry has no content hash, and the error SHALL tell the user to run `invowk module sync` to upgrade the lock file to v2.0.

#### Scenario: Discovery with a v1.0 lock
- **WHEN** a vendored module's declared lock entry has no content hash
- **THEN** discovery SHALL fail with `ErrLockEntryWithoutContentHash`

#### Scenario: Vendoring with a v1.0 lock
- **WHEN** `module vendor` copies a module whose resolved entry has no content hash
- **THEN** it SHALL fail with `ErrLockEntryWithoutContentHash` instead of skipping verification

### Requirement: Sync does not trust a cache through a hashless entry
When a requirement resolves to a locked version whose entry has no content hash, sync SHALL replace any cached copy with the fetch of the verified commit and record that content's hash.

#### Scenario: Tampered cache under a v1.0 lock
- **WHEN** the cache holds modified content and the locked entry has a commit but no hash
- **THEN** sync SHALL record the hash of the fetched content, not the cached content

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `LockIntegrity.f4RejectUnhashed` SHALL pass and `LockIntegrity.mutantPreFixF4TrustHashless` SHALL produce a counterexample
