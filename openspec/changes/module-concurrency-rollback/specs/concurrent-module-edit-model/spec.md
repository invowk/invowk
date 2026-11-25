# Spec Delta

## MODIFIED Requirements

### Requirement: Modelled file primitives
A read-only consumer (discovery, a command run, `validate`, `audit`) SHALL be modelled as a reader of `requires` and the lock file that takes no lock in any configuration. Every `fspath.AtomicWriteFile` call SHALL be modelled as an atomic replace. In the base configuration the rollback SHALL be modelled as an atomic replace of each file and its `os.Remove` as a remove step; the pre-fix rollback, an `os.WriteFile` modelled as a truncate step followed by a write step, SHALL remain selectable as a configuration.

#### Scenario: Rollback is atomic in the base configuration
- **WHEN** the model performs a rollback with the base constants
- **THEN** each restored file SHALL be one atomic replace and each removal one remove step, every `fspath.AtomicWriteFile` call SHALL be one atomic replace, and the read-only consumer SHALL only read `requires` and the lock file

#### Scenario: Rollback is not atomic in the model
- **WHEN** a command sets `AtomicRestore = FALSE`
- **THEN** each restored file SHALL be a truncate step followed by a write step

### Requirement: Concurrency properties
The model SHALL define process results `ok`, `rolled_back`, and `err`, matching three outcomes in the code: a nil error with no declaration error; a nil error with a declaration error; and a non-nil error. The model SHALL state the TLC invariants listed in the requirements "Update invariants", "Read and content invariants", and "Deadlock freedom".

#### Scenario: Properties are not vacuous
- **WHEN** the model is checked
- **THEN** witness commands SHALL show each of the following situations is reachable, each under the configuration its command names:
  - two operations overlap inside a fetch window;
  - a rollback is reached;
  - a file is partial during a pre-fix restore;
  - two vendors overlap between remove and copy without the project lock;
  - every fix configuration can end with every process `ok`

#### Scenario: Every passing property has a mutant
- **WHEN** a property passes in any configuration
- **THEN** at least one seeded mutant that is not a pre-fix configuration SHALL produce a counterexample through the same property

### Requirement: Characterisation tests replay counterexamples against the real code
Each fixed finding's counterexample that a seam can reproduce SHALL have a deterministic Go test that replays the same interleaving against the real code and asserts the fixed outcome. The test SHALL call the real use-case entry points or `Resolver` methods, or `vendorDependenciesWithResolver`, with one `Resolver` per simulated process and the real file lock on real files. Each process SHALL have its own fetcher and gate, and the processes SHALL synchronise only through channels and the lock. Each test SHALL also meet "Characterisation test constraints".

#### Scenario: Test runs in the ordinary suite
- **WHEN** `make test` runs
- **THEN** every characterisation test SHALL pass without Java, network access, or container engines, on Linux, macOS and Windows

#### Scenario: Test detects a fix
- **WHEN** a later change removes or weakens the project lock, the source lock or the atomic restore in the code a test replays
- **THEN** that characterisation test SHALL fail, so the finding cannot reopen silently

### Requirement: Characterisation test constraints
The characterisation tests SHALL NOT access the network or use sleeps to order steps, and they SHALL pass under `go test -race`. Each test SHALL assert the fixed behaviour and name its finding and the manifest commands of its fixed and pre-fix configurations. A finding that no seam can replay as an interleaving SHALL be marked model-only in its correspondence row, and SHALL have a Go test of the fixed primitive instead.

#### Scenario: Race detector run
- **WHEN** the characterisation tests run under `go test -race`
- **THEN** they SHALL pass, assert the fixed behaviour, and name their finding and manifest commands

#### Scenario: No seam can reproduce a counterexample
- **WHEN** no seam can replay a finding's interleaving
- **THEN** it SHALL be marked model-only, with the reason stated in its correspondence row, and a Go test SHALL check the fixed primitive

### Requirement: Advisory lock fix configuration
The advisory lock fix configuration SHALL be a plain mutex. Each project's mutex is held by add, remove, sync, update and vendor, from before their first read of either file until after their last write or rollback; the read-only consumer holds none. Under `"project_and_source"`, each Git source's mutex is also held from checkout until the verified cache copy, and it is always acquired after the project mutex.

#### Scenario: Source lock ordering
- **WHEN** a process under `"project_and_source"` checks out a Git source
- **THEN** it SHALL hold that source's mutex from checkout until the verified cache copy, acquired after its project mutex

#### Scenario: Reader takes no lock
- **WHEN** the read-only consumer reads either file under any `AdvisoryLock` value
- **THEN** it SHALL hold no mutex and SHALL NOT wait for one

## ADDED Requirements

### Requirement: Fix configurations are the base configuration
The model SHALL keep the advisory lock and atomic restore configurations stated in "Advisory lock fix configuration" and "Atomic restore fix configuration". Since the code adopted both, the base configuration SHALL be the fix configuration, and the configurations without them SHALL be kept as pre-fix regression mutants.

#### Scenario: Fixes pass their properties
- **WHEN** TLC checks each pair of scenario and property from the design's command matrix under the base configuration
- **THEN** each command SHALL pass, and `NoCircularWait` SHALL pass under `"project_and_source"`

#### Scenario: Fix mutants are rejected
- **WHEN** the fix is weakened in any of these ways:
  - the lock is taken after the snapshot;
  - the lock is released between the two file writes;
  - the lock order is reversed;
  - the source lock is omitted;
  - the vendor runs outside the lock;
  - the restore removes the file before writing it
- **THEN** each mutant SHALL produce a counterexample for the property it targets

### Requirement: Findings F8, F9 and F10 are fixed
The base configuration SHALL mirror the fixed code: `AdvisoryLock = "project_and_source"` and `AtomicRestore = TRUE`. Each former finding command SHALL be kept as a `mutantPreFix…` command with the pre-fix constants, an expected counterexample, no `finding` field and `mutant_of` set to its fixed command. The F8, F9 and F10 rows of `formal/README.md` SHALL read Fixed and name those commands and their Go replays.

#### Scenario: Sync overlaps an add
- **WHEN** under `sync_add` one process syncs while another completes `module add` of a different key
- **THEN** `syncAddProjectLock`, run under the base constants, SHALL pass `LockMatchesRequires`, `mutantPreFixF8SyncOverwritesConcurrentAdd` (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` SHALL show the lock holding the added key

#### Scenario: Update overlaps an add or a remove
- **WHEN** under `update_add` or `update_remove` one process updates while another completes an add or a remove
- **THEN** `updateAddProjectLock` and `updateRemoveProjectLock`, run under the base constants, SHALL each pass `NoLostUpdate`, the matching `mutantPreFixF8Update…` command SHALL be a counterexample, and the matching `TestConcurrentEdits_FindingF8_Update…` test SHALL show the added key in the lock, or the removed key out of it

#### Scenario: Two adds of the same key
- **WHEN** under `add_add_same` two processes add the same key
- **THEN** `addAddSameProjectLock`, run under the base constants, SHALL pass `NoLostUpdate`, `mutantPreFixF8DuplicateAddRollsBackOtherAdd` SHALL be a counterexample, and `TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd` SHALL show the key in both files after the second add rolled back

#### Scenario: Reader during a rollback
- **WHEN** under `rollback_read` one process rolls back an add while the read-only consumer reads the lock file or `invowkmod.cue`
- **THEN** `rollbackReadAtomicRestore`, run under the base constants, SHALL pass `ReadersSeeWholeFiles`, `mutantPreFixF10NonAtomicRollback` (`AtomicRestore = FALSE`) SHALL be a counterexample, and the `Restore` correspondence row SHALL name a Go test showing that a restore replaces the file instead of rewriting it in place

#### Scenario: Shared source worktree
- **WHEN** under `fetch_v1_v2` two projects that share a cache add different versions of one repository
- **THEN** `fetchProjectAndSourceLock`, run under the base constants, SHALL pass `LockHashMatchesCommit`, `mutantPreFixF9SharedWorktreeRecordsWrongContent` (`AdvisoryLock = "none"`) and `mutantNoSourceLock` (`AdvisoryLock = "project"`) SHALL be counterexamples, and `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` SHALL show the lock hash equal to the locked commit's content

#### Scenario: Vendor overlaps an update
- **WHEN** under `vendor_sync` one process vendors from the lock while another updates it
- **THEN** `vendorSyncProjectLock`, run under the base constants, SHALL pass `VendorMatchesFinalLock`, `mutantPreFixVendorFromStaleLock` (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_VendorFromStaleLock` SHALL show that the update's lock write waits until the vendor has returned

### Requirement: Fixed scenarios pass under the base constants
Every scenario of a fixed finding SHALL have a passing command that runs under the base constants, with no `AdvisoryLock` or `AtomicRestore` override, so the model checks the configuration the code runs.

#### Scenario: Passing commands use the base
- **WHEN** the manifest's passing commands for `sync_add`, `update_add`, `update_remove`, `add_add_same`, `rollback_read`, `fetch_v1_v2` and `vendor_sync` are read
- **THEN** each SHALL set neither `AdvisoryLock` nor `AtomicRestore`

### Requirement: Lock elements are bound to the code
The model's `Acquire` and `Release` elements SHALL each have a correspondence row naming the Go symbols that take and release the project and source locks, with a binding test among the inverted replays.

#### Scenario: Correspondence check
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** the `Acquire` and `Release` rows SHALL name existing Go symbols and binding tests, and the check SHALL pass

## REMOVED Requirements

### Requirement: Current-code verdicts are recorded as findings F8, F9, and F10
**Reason**: The code now holds the project and source locks and restores atomically, so F8, F9 and F10 are no longer behaviours of today's code.
**Migration**: See "Findings F8, F9 and F10 are fixed": the base configuration mirrors the fixed code, and each former finding command is kept as a `mutantPreFix…` regression mutant.

### Requirement: Fix configurations for a follow-up change
**Reason**: The product code adopted the advisory lock and the atomic restore, so the fix configurations no longer wait for a follow-up change, and its scenario "Product fix stays out of scope" no longer holds.
**Migration**: See "Fix configurations are the base configuration".
