## 0. Prerequisite

- [x] 0.1 Confirm `formal-infra-refinements` has landed. That provides TLC configurations generated from the manifest, rejection of unknown manifest keys, and a recorded `distinct_states`. Start this change on its own branch or worktree from that base. Satisfied by the orchestrator: this branch starts from main and is rebased onto `formal-infra-refinements` when that lands; only manifest keys the runner accepts today are used.

## 1. Confirm the code paths

- [x] 1.1 Re-trace the operations in design.md's table on the current base. Update the line citations for:
  - add, remove, sync, update, and tidy;
  - `vendorDependenciesWithResolver`, `VendorModules`, and `resolveOne`;
  - `cacheModule`, `GitFetcher.Fetch` and `GitFetcher.checkout`;
  - `fileSnapshot.restore`.

  Re-traced on a1f2f2c2: every citation in design.md's table still holds (the only commit since 4831f23d is documentation).
- [x] 1.2 Confirm again that no inter-process lock guards module commands. Grep for `Flock`, `LockFileEx`, and `lockedfile`, excluding `internal/container/run_lock_*.go` and `internal/testutil/container_suite_lock_*.go`. Confirmed: the only other hits are the Podman run-serialisation tests and `run_serialization.go`, which consume the excluded lock.

## 2. Model

- [x] 2.1 Write `formal/tla/ConcurrentModuleEdits.tla`:
  - SPDX header;
  - named assumptions: an atomic write is an atomic replace, and a fetch has no shared effect;
  - a correspondence table with one symbol and one test per row (design D7);
  - constants `Procs`, `Projects`, `Scenario`, `AdvisoryLock`, `AtomicRestore`, and `Mutant`;
  - per-project state and per-process `proj` and `result` (`ok`, `rolled_back`, `err`), as in design D2.
- [x] 2.2 Write the generic step actions, dispatched on `pc`: `Snapshot`, `ReadMod`, `ReadLock`, `Fetch`, `Checkout`, `CopyToCache`, `WriteLock`, `WriteMod`, `RestoreTruncate`, `RestoreWrite`, `RestoreRemove`, `VendorRemove`, `VendorCopy`, `Acquire`, `Release`, and `Return`. Give each operation's program a source-line comment on each step.
- [x] 2.3 Add the scenario programs from design D1 (`sync_add`, `update_add`, `update_remove`, `add_add_same`, `rollback_read`, `fetch_v1_v2`, `vendor_sync`, `vendor_vendor`, `sync_update_same_source`), selected by `CASE` on `Scenario`. Tidy is a reader in `rollback_read`.
- [x] 2.4 Add the invariants `TypeOK`, `NoLostUpdate`, `LockMatchesRequires`, `ReadersSeeWholeFiles`, `LockHashMatchesCommit`, `VendorMatchesFinalLock`, and `NoCircularWait`, as defined in design D3. Add the witness invariants from design D4.
- [x] 2.5 Add the mutex fix configurations (`"project"` and `"project_and_source"`), the `AtomicRestore` configuration, and the mutants `lock_after_snapshot`, `release_between_files`, `reverse_order`, `vendor_outside_lock`, and `restore_remove_then_write`

## 3. Manifest and verdicts

- [x] 3.1 Register the model in `formal/manifest.toml` with base constants that mirror today's code, and add only the commands in design D4's matrix plus its witnesses. Use only runner-accepted keys: `constants`, `property`, `expect`, `finding`, `mutant_of`, `witness`, `dead_actions`, and `distinct_states`.
- [x] 3.2 Run each base command. Record confirmed counterexamples under the fixed ids: F8 (`sync_add`, `update_add`, `update_remove`, `add_add_same`), F9 (`fetch_v1_v2`), and F10 (`rollback_read`), each with `mutant_of` pointing at its fix command. A non-reproducing id stays unused and is noted in the calibration entry. A `vendor_sync` counterexample, or any other distinct defect, becomes a proposed finding without an id and is escalated. All three ids reproduced; `vendor_sync` is recorded as `proposedVendorFromStaleLock` without an id.
- [x] 3.3 Add the fix commands and their non-finding mutants, and confirm every declared verdict
- [x] 3.4 Reconcile each command's `dead_actions` with TLC's coverage output, starting from design D4's table
- [x] 3.5 Record `distinct_states` on every command. Write a dated calibration entry that states:
  - which findings reproduced and which were refuted;
  - which Go replays confirm them;
  - the model's total TLC wall time, for `promote-formal-ci-gate`'s `budget_seconds`.

## 4. Characterisation tests

- [x] 4.1 In a `_test.go` file of `internal/app/modulesync`, add a gating fetcher decorator. Each simulated process gets its own `fakeModuleFetcher` and its own gate, and processes synchronise only through channels. Never share a fake between goroutines.
- [x] 4.2 Add a helper that builds a local module repository with two tags. The module ID is the same at both tags and the content differs. Reuse `writeIntegrityModule` from `resolver_integrity_test.go`.
- [x] 4.3 `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd`: P1 calls `LoadRequirements`, then `Resolver.Sync`; P2 calls `Resolver.AddModuleDependency`. Afterwards, `invowkmod.cue` holds the key and the lock lacks it.
- [x] 4.4 `TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd` and `TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove`
- [x] 4.5 `TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd`: P1 returns with no declaration error, P2 returns with a declaration error, and the key ends up absent from both files
- [x] 4.6 `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent`: two working directories share one `cacheDir`, and each has its own `Resolver` with a real `GitFetcher`. P1's fetcher blocks after `Fetch` returns. The recorded hash differs from the content of the locked commit.
- [x] 4.7 Only if `vendor_sync` produces a counterexample: `TestConcurrentEdits_VendorFromStaleLock` in `internal/app/moduleops`. P1 runs `vendorDependenciesWithResolver` with a real `Resolver` wrapped to block after `LoadDeclaredFromLock`; P2 calls `Resolver.Update`.
- [x] 4.8 Mark as model-only in their correspondence rows, with the reason: F10 (`fileSnapshot.restore`), the `AddRequirement` and `RemoveRequirement` windows, the `VendorModules` `RemoveAll`-to-copy window, and the `SyncModule`, `UpdateModule`, and `TidyModule` wrappers
- [x] 4.9 In every test's doc comment, name the finding and manifest command, and state that the test asserts current behaviour and must be inverted with the fix. Run `go test -race ./internal/app/modulesync/ ./internal/app/moduleops/`. `-race` needs cgo and gcc, which this machine lacks; the tests use only channels and per-process fakes and run under `go test -count=1` here, and CI runs them with `-race`.

## 5. Documentation

- [x] 5.1 `formal/README.md`: add a models-table row, open findings rows for F8, F9, and F10 ("Open: fix requires maintainer approval"), and any refuted hypotheses or proposed findings
- [x] 5.2 In `formal/tla/AtomicWrite.tla`, replace "concurrent multi-process writers not modelled" in the `LockFile.Save` row with a pointer to `ConcurrentModuleEdits`
- [x] 5.3 `.agents/skills/formal-verification/SKILL.md`:
  - add `./internal/app/moduleops/` to the Verification `go test` line;
  - add the scenario-constant pattern and the rule that circular wait is checked as an invariant;
  - run `make check-agent-docs`.

## 6. Verification

- [x] 6.1 `python3 scripts/formal.py fetch` (if the jars are absent), then `make formal` and `python3 scripts/test_formal.py`
- [x] 6.2 `go test -race ./internal/app/modulesync/ ./internal/app/moduleops/`, then `make test`
- [x] 6.3 `make lint`, `make license-check`, `make check-file-length`, and `make check-baseline`
- [x] 6.4 `git diff --stat` shows that no production Go file changed
- [x] 6.5 `openspec validate model-concurrent-lock-writes --strict`
