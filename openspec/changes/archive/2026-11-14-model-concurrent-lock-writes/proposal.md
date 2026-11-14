## Why

`pkg/fspath.AtomicWriteFile` keeps each write of `invowkmod.cue` and `invowkmod.lock.cue` whole (finding F3 is fixed), but Invowk has no inter-process locking around module-mutating commands. The only production `flock` serialises Podman runs (`internal/container/run_lock_linux.go`). The other one is a test-suite lock in `internal/testutil`. `Resolver.mu` protects a single `Resolver` value inside one process.

`module add`, `remove`, `sync`, `update`, and `vendor` each read the two files, fetch over the network, and write the files back. Two concurrent invocations can therefore lose each other's update, leave the lock and `requires` disagreeing, or race in the shared module cache (`~/.invowk/modules`, or `$INVOWK_MODULES_PATH`) and `invowk_modules/`. The rollback path of `module add` also rewrites both files with a plain `os.WriteFile`, which bypasses the atomic write. The `AtomicWrite` model says so explicitly: its correspondence row for `LockFile.Save` reads "concurrent multi-process writers not modelled". This change closes that gap with a model, not a fix.

## What Changes

- Add a TLA+ model, `formal/tla/ConcurrentModuleEdits.tla`, of two invowk processes running module operations step by step, as the code performs them. The processes run in one project or in two projects that share a module cache. The model covers:
  - `AddModuleDependency`: its pre-fetch snapshots, lock re-read and save, `invowkmod.cue` edit, and a rollback, modelled as a truncate step followed by a write step;
  - `RemoveModuleDependency`, `Resolver.Sync` (as called by `SyncModule` and `vendor --update`), and `UpdateModule`;
  - vendoring from the lock;
  - the shared Git source checkout and the cache copy;
  - Tidy, modelled as a reader.
- Check these properties, each with a declared verdict in `formal/manifest.toml`:
  - `NoLostUpdate`;
  - `LockMatchesRequires`;
  - `ReadersSeeWholeFiles`;
  - `LockHashMatchesCommit`;
  - `VendorMatchesFinalLock`;
  - `NoCircularWait`, for the fix configuration.

  Calibrate them with witnesses and seeded mutants. Every property that passes, including under the fix configurations, has a mutant that is not a finding.
- Record the counterexamples on today's code under ids fixed in advance:
  - **F8**: lost updates from concurrent module commands. Sync or update overwrites a concurrent add or remove, and an add's rollback erases another process's add.
  - **F9**: the shared Git checkout race. The lock records one commit together with the content hash of another.
  - **F10**: the non-atomic rollback write in `AddModuleDependency`. `os.WriteFile` truncates in place, so a concurrent reader can see an empty or partial file.

  A further distinct defect gets no id in this change. It is recorded as a proposed finding and escalated for id allocation. A finding that does not reproduce is not recorded, and its id stays unused with a note in the change.
- Add Go characterisation tests in `internal/app/modulesync` and `internal/app/moduleops`. Each test replays a counterexample against the real `Resolver` methods and `vendorDependenciesWithResolver`, using one `Resolver` per simulated process. Processes are interleaved deterministically through the existing `moduleFetcher` and `vendorDependencyResolver` seams, and the tests make no production code changes. Each test asserts today's behaviour and is inverted when a fix lands, as with F4 and F6. Counterexamples with no seam (F10's truncate-to-write window) are recorded as model-only, with the reason stated.
- Add fix configurations. An advisory lock is modelled as a plain mutex: one per project, and one per Git source for F9, taken in a fixed order. F10 has its own fix configuration, an atomic restore. Each fix passes its properties, and its mutants produce counterexamples.
- Build on the machinery from `formal-infra-refinements`: TLC configurations generated from the manifest, and a runner that rejects unknown manifest keys. The scenario is an ordinary constant, so no new manifest key is needed.
- Point the `AtomicWrite` correspondence row for `LockFile.Save` at the new model.

**Out of scope:** the product fix. Adding an inter-process lock, making the rollback atomic, or any other remedy changes the behaviour of every module-mutating command, touches Windows file-sharing semantics, and may add lock files to users' directories. It needs maintainer approval and its own OpenSpec change. This change adds no production code. Trace validation is also deferred, for the reason given in design.md.

## Capabilities

### New Capabilities

- `concurrent-module-edit-model`: a bounded TLA+ model of concurrent module commands over `invowkmod.cue`, `invowkmod.lock.cue`, the shared module cache, and `invowk_modules/`. Characterisation tests bind it to the real code. It includes findings F8–F10 and validated fix configurations for a follow-up change.

### Modified Capabilities

(none. `protocol-model-verification` still lives in the unarchived `adopt-formal-verification` change, so this change adds a separate capability instead of a delta against it.)

## Impact

- New: `formal/tla/ConcurrentModuleEdits.tla`, `internal/app/modulesync/concurrent_edits_formal_test.go`, and `internal/app/moduleops/vendor_concurrent_formal_test.go`.
- Edited:
  - `formal/manifest.toml` (model, commands, `distinct_states`, calibration);
  - `formal/README.md` (models table; findings rows for F8, F9, and F10);
  - the `AtomicWrite.tla` correspondence row;
  - `.agents/skills/formal-verification/SKILL.md` (the Verification line gains `./internal/app/moduleops/`; the scenario-constant and circular-wait patterns are added).
- Depends on `formal-infra-refinements` landing first.
- No production code, schema, CLI, or website changes. `make test` gains deterministic tests with no network access and no sleeps, and they pass under `-race`. `make formal` gains one model, and its total TLC wall time is recorded for `promote-formal-ci-gate`'s budget.
