## ADDED Requirements

### Requirement: Atomic writes survive power loss
`AtomicWriteFile` SHALL fsync the temp file before renaming it, so that after power loss the target holds either its complete previous content or the complete new content.

#### Scenario: Power loss after the rename
- **WHEN** power is lost after the rename but before any later write
- **THEN** the target SHALL hold the complete old or new content, never an empty or torn file

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `AtomicWrite.durableAtomic` SHALL pass and `AtomicWrite.mutantPreFixNoSyncAtomic` SHALL produce a counterexample

### Requirement: A returned success is durable
When `AtomicWriteFile` returns nil, the new content SHALL survive power loss. It SHALL fsync the parent directory after the rename, except on Windows, where directories cannot be fsynced.

#### Scenario: Directory fsync fails
- **WHEN** the directory fsync fails after a successful rename
- **THEN** `AtomicWriteFile` SHALL return an error stating the new content is in place with unknown durability, and SHALL NOT try to remove the consumed temp name

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `AtomicWrite.durableCommit` SHALL pass and `AtomicWrite.durableCommitFileSyncOnly` SHALL produce a counterexample
