# concurrent-module-edit-model Specification

## Purpose
This capability specifies a TLA+ model, `formal/tla/ConcurrentModuleEdits.tla`, of two invowk processes running module add, remove, sync, update, tidy and vendor operations concurrently, in one project or in two projects sharing a module cache. It defines the concurrency properties those operations must keep, records today's violations as findings F8, F9 and F10 with deterministic Go tests that replay them against the real code, and checks candidate remedies (an advisory lock and an atomic restore) in fix configurations. The model checks only the configurations it encodes; it does not change production code, and adopting a remedy needs a separate change.

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
The model SHALL decompose each of the following into the reads, network-fetch windows, and writes the code performs, in the code's order:
- `Resolver.AddModuleDependency`, including its pre-fetch snapshots and its rollback;
- `Resolver.RemoveModuleDependency`, `Resolver.Sync`, and `Resolver.Update`;
- vendoring through `vendorDependenciesWithResolver` and `VendorModules`;
- the Git source checkout and the `cacheModule` copy inside `resolveOne`.

#### Scenario: Operations follow the code's order
- **WHEN** the model is checked
- **THEN** each listed operation SHALL appear as its reads, network-fetch windows, and writes in the order the code performs them, with `Resolver.AddModuleDependency` including its pre-fetch snapshots and its rollback

### Requirement: Modelled file primitives
Tidy SHALL be modelled as a reader of `requires` and the lock file. Every `fspath.AtomicWriteFile` call SHALL be modelled as an atomic replace. The rollback's `os.WriteFile` SHALL be modelled as a truncate step followed by a write step, and its `os.Remove` as a remove step.

#### Scenario: Rollback is not atomic in the model
- **WHEN** the model performs a rollback
- **THEN** each `os.WriteFile` SHALL be a truncate step followed by a write step and each `os.Remove` a remove step, while every `fspath.AtomicWriteFile` call SHALL be one atomic replace and tidy SHALL only read `requires` and the lock file

### Requirement: Concurrency properties
The model SHALL define process results `ok`, `rolled_back`, and `err`, matching three outcomes in the code: a nil error with no declaration error; a nil error with a declaration error; and a non-nil error. The model SHALL state the TLC invariants listed in the requirements "Update invariants", "Read and content invariants", and "Deadlock freedom".

#### Scenario: Properties are not vacuous
- **WHEN** the model is checked
- **THEN** witness commands SHALL show each of the following situations is reachable:
  - two operations overlap inside a fetch window;
  - a rollback is reached;
  - a file is partial during a restore;
  - two vendors overlap between remove and copy;
  - every fix configuration can end with every process `ok`

#### Scenario: Every passing property has a mutant
- **WHEN** a property passes in any configuration
- **THEN** at least one seeded mutant that is not a finding SHALL produce a counterexample through the same property

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

### Requirement: Current-code verdicts are recorded as findings F8, F9, and F10
The base configuration SHALL mirror today's code, with no inter-process lock and a non-atomic restore. The finding ids SHALL be fixed as the requirement "Finding ids" states. Each finding SHALL be a manifest command with its `finding` id and with `mutant_of` set to a passing fix command, and it SHALL have an open row in the findings table of `formal/README.md`. Unconfirmed hypotheses and further defects SHALL follow "Unconfirmed hypotheses and further defects".

#### Scenario: Sync overlaps an add
- **WHEN** under `sync_add`, one process syncs from a `requires` set read before its fetch while another completes `module add` of a different key
- **THEN** the base command SHALL be a counterexample of `LockMatchesRequires` recorded as F8, the `AdvisoryLock = "project"` command SHALL pass, and `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` SHALL reproduce the lock without the added key; if TLC instead passes, `formal/README.md` SHALL record the hypothesis as refuted

#### Scenario: Update overlaps an add or a remove
- **WHEN** under `update_add` or `update_remove`, one process updates from a lock read before its fetch while another completes an add or a remove
- **THEN** each base command SHALL be a counterexample of `NoLostUpdate` recorded as F8, the project-lock command SHALL pass, and the matching `TestConcurrentEdits_FindingF8_Update…` test SHALL reproduce it

#### Scenario: Two adds of the same key
- **WHEN** under `add_add_same`, two processes add the same key, and the later one's `invowkmod.cue` edit fails with a duplicate error and restores its pre-fetch snapshots
- **THEN** the base command SHALL be a counterexample of `NoLostUpdate` recorded as F8, in which the other process has result `ok` and the key is absent from both files; the project-lock command SHALL pass; and `TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd` SHALL reproduce it

#### Scenario: Reader during a rollback
- **WHEN** under `rollback_read`, one process rolls back an add while another reads the lock file or `invowkmod.cue`
- **THEN** the base command SHALL be a counterexample of `ReadersSeeWholeFiles` recorded as F10, the `AtomicRestore = TRUE` command SHALL pass, and the correspondence row for `fileSnapshot.restore` SHALL mark F10 as model-only because `os.WriteFile` has no seam between truncate and write

#### Scenario: Shared source worktree
- **WHEN** under `fetch_v1_v2`, two projects that share a cache add different versions of the same repository, and neither has a prior lock entry
- **THEN** the base command SHALL be a counterexample of `LockHashMatchesCommit` recorded as F9; the `"project_and_source"` command SHALL pass and the `"project"` command SHALL fail; and `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` SHALL reproduce a lock hash that differs from the locked commit's content

#### Scenario: Vendor overlaps an update
- **WHEN** under `vendor_sync`, one process vendors from the lock while another updates it to a new version
- **THEN** a counterexample of `VendorMatchesFinalLock` SHALL be recorded as a proposed finding without an id and replayed through `vendorDependenciesWithResolver`, or a pass SHALL be recorded as a refuted hypothesis

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
Each finding's counterexample that an existing seam can reproduce SHALL have a deterministic Go test. The test SHALL call the real `Resolver` methods, or `vendorDependenciesWithResolver`, with one `Resolver` per simulated process. Each process SHALL have its own fetcher and gate, and the processes SHALL synchronise only through channels. Each test SHALL also meet "Characterisation test constraints".

#### Scenario: Test runs in the ordinary suite
- **WHEN** `make test` runs
- **THEN** every characterisation test SHALL pass without Java, network access, or container engines

#### Scenario: Test detects a fix
- **WHEN** a follow-up change serialises the replayed operations
- **THEN** the characterisation test SHALL fail until it is inverted, so the finding cannot close silently

### Requirement: Characterisation test constraints
The characterisation tests SHALL NOT change production code, access the network, or use sleeps, and they SHALL pass under `go test -race`. Each test SHALL assert today's behaviour and name its finding and manifest command. A counterexample that no existing seam can reproduce SHALL be marked model-only, with the reason stated in its correspondence row.

#### Scenario: Race detector run
- **WHEN** the characterisation tests run under `go test -race`
- **THEN** they SHALL pass, assert today's behaviour, and name their finding and manifest command

#### Scenario: No seam can reproduce a counterexample
- **WHEN** no existing seam can reproduce a finding's counterexample
- **THEN** it SHALL be marked model-only, with the reason stated in its correspondence row

### Requirement: Fix configurations for a follow-up change
The model SHALL include fix configurations that validate candidate remedies without implementing them: the advisory lock and the atomic restore stated in the requirements "Advisory lock fix configuration" and "Atomic restore fix configuration".

#### Scenario: Fixes pass their properties
- **WHEN** TLC checks each pair of scenario and property from the design's command matrix under its fix configuration
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

#### Scenario: Product fix stays out of scope
- **WHEN** this change is complete
- **THEN** no production Go code SHALL have changed, and adopting any remedy SHALL require a separate, maintainer-approved OpenSpec change

### Requirement: Advisory lock fix configuration
The advisory lock fix configuration SHALL be a plain mutex. Each project's mutex is held by add, remove, sync, update, vendor, and the tidy reader, from before their first read of either file until after their last write or rollback. Under `"project_and_source"`, each Git source's mutex is also held from checkout until the verified cache copy, and it is always acquired after the project mutex.
#### Scenario: Source lock ordering
- **WHEN** a process under `"project_and_source"` checks out a Git source
- **THEN** it SHALL hold that source's mutex from checkout until the verified cache copy, acquired after its project mutex

### Requirement: Atomic restore fix configuration
In the atomic restore fix configuration, the rollback SHALL replace each file atomically.

#### Scenario: Restore under AtomicRestore
- **WHEN** a rollback runs with `AtomicRestore = TRUE`
- **THEN** the model SHALL replace each restored file atomically
