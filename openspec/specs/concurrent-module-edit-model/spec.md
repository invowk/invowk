# concurrent-module-edit-model Specification

## Purpose
This capability specifies a TLA+ model, `formal/tla/ConcurrentModuleEdits.tla`, of two invowk processes running module add, remove, sync, update, vendor and read-only consumer operations concurrently, in one project or in two projects sharing a module cache. It defines the concurrency properties those operations must keep under the adopted project/source advisory locks and atomic rollback restore, records deterministic Go tests that replay the fixed F8, F9 and F10 interleavings against the real code, and preserves the pre-fix behaviours as regression mutants.

## Requirements

### Requirement: Concurrent module edit model
Invowk SHALL maintain a TLA+ model, `formal/tla/ConcurrentModuleEdits.tla`, of two invowk processes running module operations concurrently. The processes SHALL run either in one project or in two projects that share one module cache. Its operations and file primitives SHALL be modelled as the requirements "Modelled module operations" and "Modelled file primitives" state.

#### Scenario: Header correspondence table
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** the model's header table SHALL have one Go symbol and one binding test (or `-` with a reason) per row, and the check SHALL pass

#### Scenario: Registered in the manifest
- **WHEN** `make formal` runs
- **THEN** every command of the model SHALL be listed in `formal/manifest.toml` using only keys the runner accepts, with a declared verdict and a recorded `distinct_states`, and each command SHALL check exactly one property plus `TypeOK`

### Requirement: Modelled module operations
The model SHALL decompose each of the following into the reads, network-fetch windows, and writes the code performs, in the code's order, together with the project and source mutex acquisitions and releases where the configuration takes them:
- `Resolver.AddModuleDependency`, including its pre-fetch snapshots and its rollback;
- `Resolver.RemoveModuleDependency`, `Resolver.SyncModule` (and `Resolver.Sync`), and `Resolver.UpdateModule` (and `Resolver.Update`);
- vendoring through `vendorDependenciesWithResolver` and `VendorModules`;
- the Git source checkout and the `cacheModule` copy inside `resolveOne`.

Each program a process can run SHALL be a named operator that `Program` calls: `AddProg`, `RemoveProg`, `SyncProg`, `UpdateProg`, `VendorProg`, and `ReadProg` for the read-only consumer.

#### Scenario: Operations follow the code's order
- **WHEN** the model is checked
- **THEN** each listed operation SHALL appear as its reads, network-fetch windows, and writes in the order the code performs them, with `Resolver.AddModuleDependency` including its pre-fetch snapshots and its rollback

#### Scenario: Program rows resolve
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** every `ConcurrentModuleEdits` row whose element names a program SHALL resolve to its operator, and `[correspondence] unresolved_elements` SHALL list no `ConcurrentModuleEdits` pair

#### Scenario: Naming the programs changes no verdict
- **WHEN** the programs are expressed as named operators instead of inline `CASE` arms, with the same bodies
- **THEN** every manifest command of the model SHALL keep its verdict and its `distinct_states`

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

### Requirement: Update invariants
The model SHALL state these TLC invariants:
- `NoLostUpdate`: at quiescence, the effect of every Add or Remove whose result is `ok` is present in both `invowkmod.cue` and the lock file. The only exception is when another process's program has the opposite intent on the same key.
- `LockMatchesRequires`: at quiescence, if every mutating process in a project has result `ok`, the lock file's keys equal the `requires` keys.

#### Scenario: Opposite intent is exempt
- **WHEN** at quiescence an `ok` Add or Remove's effect is missing because another process's program has the opposite intent on the same key
- **THEN** `NoLostUpdate` SHALL NOT be violated, while any other missing effect SHALL violate it

### Requirement: Read and content invariants
The model SHALL state these TLC invariants:
- `ReadersSeeWholeFiles`: no process reads either file while a restore has it truncated, partially written, or transiently removed.
- `LockHashMatchesCommit`: every lock entry's content hash equals the content of its locked commit.
- `VendorMatchesFinalLock`: at quiescence, every vendored module holds the content of the final lock's commit for its key, or the vendoring process returned `err`.

#### Scenario: Vendor error is exempt
- **WHEN** at quiescence a vendored module does not hold the final lock's commit content for its key
- **THEN** `VendorMatchesFinalLock` SHALL hold only if the vendoring process returned `err`

### Requirement: Deadlock freedom
The model SHALL state the TLC invariant `NoCircularWait`, and it SHALL be checked for every configuration that takes more than one lock.

#### Scenario: Multi-lock configuration
- **WHEN** a configuration takes more than one lock
- **THEN** `NoCircularWait` SHALL be checked for it

### Requirement: Finding ids
The finding ids SHALL be fixed:
- F8: lost updates from concurrent module commands;
- F9: the shared Git checkout race that records a lock hash disagreeing with its commit;
- F10: the non-atomic rollback write.

#### Scenario: Finding recorded under its fixed id
- **WHEN** a base command's counterexample is recorded as a finding
- **THEN** it SHALL use F8 for a lost update from concurrent module commands, F9 for the shared Git checkout race, or F10 for the non-atomic rollback write

### Requirement: Unconfirmed hypotheses and further defects
A hypothesis that TLC does not confirm SHALL NOT be recorded as a finding; `formal/README.md` SHALL record it as refuted, and its id SHALL stay unused. A further distinct defect SHALL NOT take an id in this change. It SHALL be recorded as a proposed finding and escalated for id allocation.

#### Scenario: TLC refutes a hypothesis
- **WHEN** TLC passes a command that was expected to confirm a finding hypothesis
- **THEN** `formal/README.md` SHALL record the hypothesis as refuted and its id SHALL stay unused

#### Scenario: A further defect is found
- **WHEN** a distinct defect beyond F8, F9 and F10 is found
- **THEN** it SHALL be recorded as a proposed finding without an id and escalated for id allocation

### Requirement: Characterisation tests replay counterexamples against the real code
Each fixed finding's counterexample that a seam can reproduce SHALL have a deterministic Go test that replays the same interleaving against the real code and asserts the fixed outcome. The test SHALL call the real use-case entry points or `Resolver` methods, or `vendorDependenciesWithResolver`, with one `Resolver` per simulated process and the real file lock on real files. A process whose real entry point cannot run offline in the test's package (the concurrent update of the vendor replay in `internal/app/moduleops`, which cannot inject a fetcher) MAY transcribe that entry point's file write, provided it takes the real project lock through `Resolver.LockProject` first and its doc comment says so. Each process SHALL have its own fetcher and gate, and the processes SHALL synchronise only through channels and the lock. Each test SHALL also meet "Characterisation test constraints". Because each test now asserts the fixed behaviour, it SHALL NOT match the characterisation name pattern of `scripts/formal.py` (`_FindingF<n>`, `_Characterisation`), and no correspondence row that tables it SHALL carry a note beginning `characterisation:`, so the reach run and `mutation-formal` use it as a killer.

#### Scenario: Inverted replays are killers
- **WHEN** the formal tooling (`is_characterisation_test` in `scripts/formal.py`) classifies the binding tests of the `ConcurrentModuleEdits` rows
- **THEN** none of `TestConcurrentEdits_F8SyncKeepsConcurrentAdd`, `TestConcurrentEdits_F8UpdateKeepsConcurrentAdd`, `TestConcurrentEdits_F8UpdateKeepsConcurrentRemove`, `TestConcurrentEdits_F8DuplicateAddKeepsOtherAdd`, `TestConcurrentEdits_F9SourceLockRecordsMatchingContent`, `TestConcurrentEdits_F10RestoreReplacesFile` and `TestConcurrentEdits_VendorFromStaleLock` SHALL be classified as a characterisation test

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

### Requirement: Atomic restore fix configuration
In the atomic restore fix configuration, the rollback SHALL replace each file atomically.

#### Scenario: Restore under AtomicRestore
- **WHEN** a rollback runs with `AtomicRestore = TRUE`
- **THEN** the model SHALL replace each restored file atomically

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
The base configuration SHALL mirror the fixed code: `AdvisoryLock = "project_and_source"` and `AtomicRestore = TRUE`. Each former finding command SHALL be kept as a `mutantPreFix…` command with its own fix's constant set back to the pre-fix value (as each scenario below states), with an expected counterexample, no `finding` field and `mutant_of` set to its fixed command. The F8, F9 and F10 rows of `formal/README.md` SHALL read Fixed and name those commands and their Go replays.

#### Scenario: Sync overlaps an add
- **WHEN** under `sync_add` one process syncs while another completes `module add` of a different key
- **THEN** `syncAddProjectLock`, run under the base constants, SHALL pass `LockMatchesRequires`, `mutantPreFixF8SyncOverwritesConcurrentAdd` (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_F8SyncKeepsConcurrentAdd` SHALL show the lock holding the added key

#### Scenario: Update overlaps an add or a remove
- **WHEN** under `update_add` or `update_remove` one process updates while another completes an add or a remove
- **THEN** `updateAddProjectLock` and `updateRemoveProjectLock`, run under the base constants, SHALL each pass `NoLostUpdate`, the matching `mutantPreFixF8Update…` command (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_F8UpdateKeepsConcurrentAdd` or `TestConcurrentEdits_F8UpdateKeepsConcurrentRemove` SHALL show the added key in the lock, or the removed key out of it

#### Scenario: Two adds of the same key
- **WHEN** under `add_add_same` two processes add the same key
- **THEN** `addAddSameProjectLock`, run under the base constants, SHALL pass `NoLostUpdate`, `mutantPreFixF8DuplicateAddRollsBackOtherAdd` (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_F8DuplicateAddKeepsOtherAdd` SHALL show the key in both files after the second add rolled back

#### Scenario: Reader during a rollback
- **WHEN** under `rollback_read` one process rolls back an add while the read-only consumer reads the lock file or `invowkmod.cue`
- **THEN** `rollbackReadAtomicRestore`, run under the base constants, SHALL pass `ReadersSeeWholeFiles`, `mutantPreFixF10NonAtomicRollback` (`AtomicRestore = FALSE`) SHALL be a counterexample, and the `Restore` correspondence row SHALL name `TestConcurrentEdits_F10RestoreReplacesFile`, a Go test showing that a restore replaces the file instead of rewriting it in place

#### Scenario: Shared source worktree
- **WHEN** under `fetch_v1_v2` two projects that share a cache add different versions of one repository
- **THEN** `fetchProjectAndSourceLock`, run under the base constants, SHALL pass `LockHashMatchesCommit`, `mutantPreFixF9SharedWorktreeRecordsWrongContent` (`AdvisoryLock = "none"`) and `mutantNoSourceLock` (`AdvisoryLock = "project"`) SHALL be counterexamples, and `TestConcurrentEdits_F9SourceLockRecordsMatchingContent` SHALL show the lock hash equal to the locked commit's content

#### Scenario: Vendor overlaps an update
- **WHEN** under `vendor_sync` one process vendors from the lock while another updates it
- **THEN** `vendorSyncProjectLock`, run under the base constants, SHALL pass `VendorMatchesFinalLock`, `mutantPreFixVendorFromStaleLock` (`AdvisoryLock = "none"`) SHALL be a counterexample, and `TestConcurrentEdits_VendorFromStaleLock` SHALL show that the update's lock write waits until the vendor has returned

### Requirement: Fixed scenarios pass under the base constants
Every scenario of a fixed finding SHALL have a passing command that runs under the base constants, with no `AdvisoryLock` or `AtomicRestore` override, so the model checks the configuration the code runs.

#### Scenario: Passing commands use the base
- **WHEN** the manifest's passing commands for `sync_add`, `update_add`, `update_remove`, `add_add_same`, `rollback_read`, `fetch_v1_v2` and `vendor_sync` are read
- **THEN** each SHALL set neither `AdvisoryLock` nor `AtomicRestore`

### Requirement: Lock elements are bound to the code
The model's `Acquire` and `Release` elements SHALL each have a correspondence row naming the Go symbols that take and release the project and source locks, and the `SyncProg` and `UpdateProg` rows SHALL name `Resolver.SyncModule` and `Resolver.UpdateModule`; each of these rows SHALL have a binding test among the inverted replays.

#### Scenario: Correspondence check
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** the `Acquire`, `Release`, `SyncProg` and `UpdateProg` rows SHALL name existing Go symbols and binding tests, and the check SHALL pass

### Requirement: Use-case wrappers and renamed replays are bound
Whenever a bound correspondence row names a `Resolver` method that shares its name with a package-level function of the same file, that package-level function SHALL have a correspondence row of its own, bound to a killer test that calls it. This applies to the package-level `AddModuleDependency`, `RemoveModuleDependency`, `SyncModule` and `UpdateModule` use cases. Every row that names a renamed replay SHALL name it by its new name.

#### Scenario: Wrapper rows
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** a row SHALL name each of the four package-level wrappers with an existing binding test, no row SHALL name a test that no longer exists, and the check SHALL pass

#### Scenario: Lock code is planned for mutation
- **WHEN** `scripts/formal_mutation.py plan` runs
- **THEN** `Resolver.LockProject`, `Resolver.lockSource` and `editLocks.release` SHALL be planned, both the package-level function and the `Resolver` method of `AddModuleDependency`, `RemoveModuleDependency`, `SyncModule` and `UpdateModule` SHALL be planned, and the plan SHALL report no match collision

### Requirement: Bound rows' killers reach their functions
Every binding test of a `ConcurrentModuleEdits` row that this change adds, re-points or re-binds SHALL execute that row's Go function. The `WriteLock` row for `LockFile.Save` SHALL therefore bind `TestConcurrentEdits_VendorFromStaleLock`, whose concurrent update writes through `LockFile.Save`, and not a resolver replay: the resolver calls only `LockFile.SaveIfChanged`.

#### Scenario: Killers reach their functions
- **WHEN** `make formal-reach` runs at any commit of this change
- **THEN** every binding test of every added, re-pointed or re-bound `ConcurrentModuleEdits` row SHALL cover at least one statement of the row's function, and the check SHALL pass
