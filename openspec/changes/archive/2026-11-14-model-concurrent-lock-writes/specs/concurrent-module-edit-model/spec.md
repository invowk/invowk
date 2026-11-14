## ADDED Requirements

### Requirement: Concurrent module edit model
Invowk SHALL maintain a TLA+ model, `formal/tla/ConcurrentModuleEdits.tla`, of two invowk processes running module operations concurrently. The processes SHALL run either in one project or in two projects that share one module cache. The model SHALL decompose the following into the reads, network-fetch windows, and writes the code performs, in the code's order:
- `Resolver.AddModuleDependency`, including its pre-fetch snapshots and its rollback;
- `Resolver.RemoveModuleDependency`, `Resolver.Sync`, and `Resolver.Update`;
- vendoring through `vendorDependenciesWithResolver` and `VendorModules`;
- the Git source checkout and the `cacheModule` copy inside `resolveOne`.

Tidy SHALL be modelled as a reader of `requires` and the lock file. Every `fspath.AtomicWriteFile` call SHALL be modelled as an atomic replace. The rollback's `os.WriteFile` SHALL be modelled as a truncate step followed by a write step, and its `os.Remove` as a remove step.

#### Scenario: Header correspondence table
- **WHEN** `scripts/formal.py correspondence` runs
- **THEN** the model's header table SHALL have one Go symbol and one binding test (or `-` with a reason) per row, and the check SHALL pass

#### Scenario: Registered in the manifest
- **WHEN** `make formal` runs
- **THEN** every command of the model SHALL be listed in `formal/manifest.toml` using only keys the runner accepts, with a declared verdict and a recorded `distinct_states`, and each command SHALL check exactly one property plus `TypeOK`

### Requirement: Concurrency properties
The model SHALL define process results `ok`, `rolled_back`, and `err`, matching three outcomes in the code: a nil error with no declaration error; a nil error with a declaration error; and a non-nil error. The model SHALL state these TLC invariants:
- `NoLostUpdate`: at quiescence, the effect of every Add or Remove whose result is `ok` is present in both `invowkmod.cue` and the lock file. The only exception is when another process's program has the opposite intent on the same key.
- `LockMatchesRequires`: at quiescence, if every mutating process in a project has result `ok`, the lock file's keys equal the `requires` keys.
- `ReadersSeeWholeFiles`: no process reads either file while a restore has it truncated, partially written, or transiently removed.
- `LockHashMatchesCommit`: every lock entry's content hash equals the content of its locked commit.
- `VendorMatchesFinalLock`: at quiescence, every vendored module holds the content of the final lock's commit for its key, or the vendoring process returned `err`.
- `NoCircularWait`: this SHALL be checked for every configuration that takes more than one lock.

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

### Requirement: Current-code verdicts are recorded as findings F8, F9, and F10
The base configuration SHALL mirror today's code, with no inter-process lock and a non-atomic restore. The finding ids are fixed:
- F8: lost updates from concurrent module commands;
- F9: the shared Git checkout race that records a lock hash disagreeing with its commit;
- F10: the non-atomic rollback write.

Each finding SHALL be a manifest command with its `finding` id and with `mutant_of` set to a passing fix command, and it SHALL have an open row in the findings table of `formal/README.md`. A hypothesis that TLC does not confirm SHALL NOT be recorded as a finding; `formal/README.md` SHALL record it as refuted, and its id SHALL stay unused. A further distinct defect SHALL NOT take an id in this change. It SHALL be recorded as a proposed finding and escalated for id allocation.

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

### Requirement: Characterisation tests replay counterexamples against the real code
Each finding's counterexample that an existing seam can reproduce SHALL have a deterministic Go test. The test SHALL call the real `Resolver` methods, or `vendorDependenciesWithResolver`, with one `Resolver` per simulated process. Each process SHALL have its own fetcher and gate, and the processes SHALL synchronise only through channels. The tests SHALL NOT change production code, access the network, or use sleeps, and they SHALL pass under `go test -race`. Each test SHALL assert today's behaviour and name its finding and manifest command. A counterexample that no existing seam can reproduce SHALL be marked model-only, with the reason stated in its correspondence row.

#### Scenario: Test runs in the ordinary suite
- **WHEN** `make test` runs
- **THEN** every characterisation test SHALL pass without Java, network access, or container engines

#### Scenario: Test detects a fix
- **WHEN** a follow-up change serialises the replayed operations
- **THEN** the characterisation test SHALL fail until it is inverted, so the finding cannot close silently

### Requirement: Fix configurations for a follow-up change
The model SHALL include fix configurations that validate candidate remedies without implementing them:
- **Advisory lock (a plain mutex).** Each project's mutex is held by add, remove, sync, update, vendor, and the tidy reader, from before their first read of either file until after their last write or rollback. Under `"project_and_source"`, each Git source's mutex is also held from checkout until the verified cache copy, and it is always acquired after the project mutex.
- **Atomic restore.** The rollback replaces each file atomically.

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
