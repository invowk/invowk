## Context

Module-mutating commands build a fresh `Resolver` per invocation. The use-case wrappers call `NewResolver`, or `newLockOnlyResolver`, with the real `GitFetcher` (`internal/app/modulesync/use_cases.go:113,154,316,338,358`). `Resolver.mu` (`resolver.go:42-43`) therefore serialises nothing across processes. Lock and `invowkmod.cue` writes go through `fspath.AtomicWriteFile`, so readers never see a torn file from those writes. The exception is the rollback, `fileSnapshot.restore` (`use_cases.go:245-263`), which uses `os.Remove` and `os.WriteFile`. Atomic replacement is still not isolation: every operation has a read-modify-write window, and several have a network fetch inside it.

The code paths below were traced for this proposal. Line numbers refer to `main` at 4831f23d.

| Operation | Reads | Window | Writes |
|---|---|---|---|
| `Resolver.AddModuleDependency` | Snapshots of the lock and `invowkmod.cue` (`use_cases.go:126,130`). The lock for tamper checks (`resolver.go:136`). | `resolveOne`: list versions, fetch, cache (`resolver.go:142`) | Re-reads and saves the lock (`resolver.go:149-158`). `AddRequirement` reads and writes `invowkmod.cue` (`invowkmod_edit.go:29,73`). On an edit error, **restores both pre-fetch snapshots** (`use_cases.go:140`) with `os.WriteFile` (`use_cases.go:259`). The method still returns a nil error, and the CLI exits 0 with "lock file changes were rolled back" (`cmd/invowk/module_deps.go:195`). |
| `Resolver.RemoveModuleDependency` | Snapshots (`use_cases.go:164,168`). The lock (`resolver.go:173`). | none | The lock (`resolver.go:200`). `RemoveRequirement` for each entry. Restore on error (`use_cases.go:181`). |
| `SyncModule` → `Resolver.Sync` | `requires` (`use_cases.go:308`). The lock (`resolver.go:283`). | `resolveAll` (`resolver.go:289`) | A new lock built only from the pre-fetch `requires` (`resolver.go:300-307`). |
| `UpdateModule` → `Resolver.Update` | The lock (`resolver.go:216`). | `resolveOne` for each key (`resolver.go:253`) | The pre-fetch lock, with updated entries (`resolver.go:264`). |
| `TidyModule` → `Resolver.Tidy` | `requires` (`use_cases.go:330`). The lock (`resolver_tidy.go:29`). | `tidyToFixedPoint` fetches | `AddRequirement` for each missing ref (`use_cases.go:346-349`). Never writes the lock. |
| `vendorDependenciesWithResolver` | The lock through `LoadDeclaredFromLock`, or `Sync` for `--update` (`vendor_dependencies.go:115-132`). | Between resolving and `VendorModules` | For each module, `RemoveAll` then copy, then verify (`vendor.go:107-137`). Then prune. |
| `Resolver.resolveOne` and the cache | The shared Git worktree, one per URL and not per version (`git.go:94-117`; path at `git.go:467`). | Checkout (`Force: true`, `git.go:410-413`), then copy from the worktree (`resolver_deps.go:130,170`) | `cacheModule`: stat, copy, hash, and `RemoveAll` on a mismatch (`cache.go:62-89`). |

Concrete paths visible in the code, which the model confirms or refutes:

1. **F8: sync overwrites a concurrent add.** P1 syncs from `requires = {a}` and fetches. Meanwhile P2 completes `module add b`. P1 then saves a lock containing only a. Final state: b is in `requires` but not in the lock, and both commands reported success.
2. **F8: update overwrites a concurrent add or remove.** Same shape, through `resolver.go:216` and `resolver.go:264`. A removed entry comes back into the lock.
3. **F8: a rollback erases another process's add.** P1 and P2 both add x, and both snapshot before fetching. P1 finishes. P2 saves the lock, then `AddRequirement` fails with `ErrModuleAlreadyExists`, and P2 restores its pre-fetch snapshots. x is now absent from both files, although P1 reported ok.
4. **F9: the shared worktree poisons the lock.** Two projects that share the cache need v1 and v2 of the same repository. P1 checks out v1. P2 checks out v2 in the same worktree. P1 copies the worktree into the v1 cache path. With no prior lock entry, P1 locks commit c1 together with the hash of v2's content. The comment at `cache.go:84` ("dstDir did not exist before this call") is false under concurrency.
5. **F10: the non-atomic rollback.** `os.WriteFile` truncates the target and then writes it. A concurrent reader, such as `LoadLockFile` (`lockfile.go:344`) inside another command, can see an empty or partial file between those two steps. It gets a parse error, or it treats the lock as empty and may save a lock without those entries.

`AtomicWrite.tla` assumes one writer, and `LockIntegrity.tla` models an attacker. Neither covers these paths.

## Goals / Non-Goals

**Goals:**
- A TLA+ model that decomposes every operation into the reads, fetch window, and writes listed above, at the granularity the code interleaves. That includes the rollback's truncate and write steps.
- Properties that state the user-visible guarantees, each calibrated by witnesses and mutants that are not findings.
- Findings F8, F9, and F10 recorded with deterministic Go replays where an existing seam allows one.
- Fix configurations that validate a lock remedy, modelled as a mutex, and an atomic restore, for a follow-up change.

**Non-Goals:**
- Implementing any remedy.
- Changing production code, including adding test seams.
- Crash durability. `AtomicWrite` owns it. Every `AtomicWriteFile` call is modelled as an atomic replace, and this model covers only the concurrency of the rollback write, not its behaviour after a crash.
- Attackers. `LockIntegrity` owns them, and every actor here is an honest invowk process.
- Tidy's `AddRequirement` loop. Tidy is modelled as a reader of `requires` and the lock, following the orchestrator's decision. Its writes use the same `invowkmod.cue` read-modify-write as Add's edit step, which the model does include.
- The concurrent first clone. Two processes that both fail `PlainOpen` both clone into `sources/<url>` (`git.go:99-105`). The loser gets an error from `PlainClone`, or it opens a half-written repository and fails its checkout. Both outcomes are errors, not silent inconsistency, and F9's source lock would serialise them anyway. This is noted, not modelled.
- Vendor versus discovery. The orchestrator's decision was to include it only if it is cheap. Discovery re-verifies vendored hashes against the lock (`LockIntegrity`), so a torn copy fails closed. It is left out.
- Trace validation. Each replay pins exactly one interleaving, chosen by the test's gates. The interesting steps (snapshot, re-read, save, edit) happen inside `Resolver` methods, which have no recording points, so a `tlatrace.Recorder` could only observe the gate boundaries and the final files. That projection would add little beyond the replay's own assertions. A follow-up change can add `ConcurrentModuleEditsTrace.tla` on `TraceBase` if recording points are ever added.

## Decisions

### D1. One model, with a scenario constant that selects each process's program

The model's constants:
- `Procs`: two model values, used in every command;
- `Projects`: one project by default, two in `fetch_v1_v2`;
- `Scenario`: a string;
- `AdvisoryLock`: `"none"`, `"project"`, or `"project_and_source"`;
- `AtomicRestore`: a boolean;
- `Mutant`.

A `CASE` on `Scenario` inside the spec gives each process its program and its project. TLC configurations cannot assign function values, and the scenario is an ordinary constant, so the unknown-key check of `formal-infra-refinements` needs no new manifest key. The scenarios:

| Scenario | P1 | P2 | Projects |
|---|---|---|---|
| `sync_add` | sync (`requires` = {a}) | add b | 1 |
| `update_add` | update all | add b | 1 |
| `update_remove` | update all | remove a | 1 |
| `add_add_same` | add x | add x | 1 |
| `rollback_read` | add x whose edit fails (x already in `requires`, not in the lock), so it rolls back | tidy (reader) | 1 |
| `fetch_v1_v2` | add r@v1 in project A | add r@v2 in project B | 2 |
| `vendor_sync` | vendor from the lock | update (the new version) | 1 |
| `vendor_vendor` | vendor | vendor | 1 (witness only) |
| `sync_update_same_source` | sync | update, both fetching one source | 1 (`NoCircularWait`) |

*Alternatives:* separate models for the project files and the cache, or nondeterministic programs. Two models were rejected because F9 matters only through what it writes into the lock. Nondeterministic programs were rejected because they blow up the state space, and a lost-update property must know each process's intent.

### D2. Abstract state

- `mod[proj]`: the set of keys in `requires`.
- `lock[proj]`: a function from key to an entry `[ver, commit, hash]`.
- `modVis[proj]` and `lockVis[proj]`: the file's visibility. The values:
  - `"whole"`;
  - `"absent"`: the file does not exist, a legitimate state, for example after `os.Remove` restores a snapshot that did not exist;
  - `"partial"`: between a restore's truncate and its write;
  - `"gap"`: transiently absent in the middle of a restore. Only the `restore_remove_then_write` mutant produces it.
- `worktree[url]`: the version currently checked out.
- `cache[url, ver]`: `"absent"`, `"partial"`, or `CommitContent[c]`.
- `vendor[proj][key]`: `"absent"`, `"partial"`, or content.
- Per process:
  - `pc` and `proj`;
  - its local copies: `snapLock`, `snapMod`, `readLock`, and `readMod`;
  - `result`, one of `"running"`, `"ok"`, `"rolled_back"`, or `"err"`;
  - `holds` and `waits`, under a lock configuration.

`result` maps onto the code as follows:
- `ok`: a nil error and `Declaration().Err() == nil`;
- `rolled_back`: a nil error with a declaration error (CLI exit 0);
- `err`: a non-nil error.

A read that finds a file `partial` sets `result = "err"`, as the parse error would. Content hashing is abstracted to content equality. A fetch has no effect on shared state; it only opens a window for interleaving.

### D3. Properties

"Quiescence" means that every process has `result /= "running"`.

- `NoLostUpdate`: at quiescence, take any process whose result is `ok` and whose program adds (or removes) key k in project j. Unless another process's program has the opposite intent on k in j (Add versus Remove), k SHALL be in both `mod[j]` and `DOMAIN lock[j]` (or in neither). Under `add_add_same`, both processes intend the same thing, so the exemption does not apply. A process with result `ok` requires `k ∈ mod ∧ k ∈ DOMAIN lock`.
- `LockMatchesRequires`: at quiescence, if every mutating process in project j has result `ok`, then `DOMAIN lock[j] = mod[j]`. Tidy is a reader and does not count.
- `ReadersSeeWholeFiles`: no process reads `invowkmod.cue` or the lock while its visibility is `partial` or `gap`.
- `LockHashMatchesCommit`: every lock entry's hash equals `CommitContent[entry.commit]`.
- `VendorMatchesFinalLock`: at quiescence, every vendored key holds `CommitContent` of the final lock's commit for that key, or the vendoring process returned `err`.
- `NoCircularWait`: no two processes each wait for a lock the other holds. This is stated as an invariant, because explicit stuttering hides TLC's deadlock check and the skill forbids `CHECK_DEADLOCK FALSE`.
- `TypeOK` holds on every command.

### D4. Commands, findings, dead actions, and cost

The base constants mirror today's code: `AdvisoryLock = "none"`, `AtomicRestore = FALSE`, `Mutant = "none"`. Only the pairs of scenario and property that matter get a command:

| Scenario | Property | Base (today) | Fix configuration | Non-finding mutant of the fix |
|---|---|---|---|---|
| `sync_add` | `LockMatchesRequires` | finding F8 | `AdvisoryLock = "project"` passes | `release_between_files` |
| `update_add` | `NoLostUpdate` | finding F8 | project passes | `lock_after_snapshot` |
| `update_remove` | `NoLostUpdate` | finding F8 | project passes | `lock_after_snapshot` |
| `add_add_same` | `NoLostUpdate` | finding F8 | project passes | `lock_after_snapshot` |
| `rollback_read` | `ReadersSeeWholeFiles` | finding F10 | `AtomicRestore = TRUE` passes | `restore_remove_then_write` (remove, then create) |
| `fetch_v1_v2` | `LockHashMatchesCommit` | finding F9 | `"project_and_source"` passes | `AdvisoryLock = "project"` (no source lock) |
| `vendor_sync` | `VendorMatchesFinalLock` | a counterexample becomes a proposed finding (escalated for an id); a pass is recorded as refuted | project passes | `vendor_outside_lock` |
| `sync_update_same_source` | `NoCircularWait` | – | `"project_and_source"` passes | `reverse_order` |

Each finding command has `finding = "F<n>"` and `mutant_of` set to the fix command on its row, following the F5 pattern. `AdvisoryLock = "project"` is therefore the fix for F8 and, at the same time, the non-finding mutant for F9, so no separate `no_source_lock` value is needed.

The witnesses, each an invariant `~W` expected to produce a counterexample:
- `WitnessNoFetchOverlap` (`sync_add`, base);
- `WitnessNoRollback` (`add_add_same`, base);
- `WitnessNoPartialFile` (`rollback_read`, base);
- `WitnessNoVendorOverlap` (`vendor_vendor`, base: both processes between `RemoveAll` and copy);
- `WitnessNotAllOk` under each fix configuration, which shows the fix neither blocks nor rolls back every behaviour.

**Dead actions.** Step actions are generic, one per kind of step, and dispatched on `pc`:
- `Snapshot`, `ReadMod`, `ReadLock`, `Fetch`, `Checkout`, `CopyToCache`, `WriteLock`, `WriteMod`;
- `RestoreTruncate`, `RestoreWrite`, `RestoreRemove`;
- `VendorRemove`, `VendorCopy`;
- `Acquire`, `Release`, `Return`.

Checkout and copy are separate steps only for a source-fetching fetch. That keeps coverage per step kind, and each pass command lists what its scenario cannot reach. The runner fails any command in which an action generates zero states and is not listed (`formal.py:454`).

| Scenario | Dead actions (plus `Acquire`/`Release` when `AdvisoryLock = "none"`) |
|---|---|
| `sync_add`, `update_add`, `update_remove` | `RestoreTruncate`, `RestoreWrite`, `RestoreRemove`, `VendorRemove`, `VendorCopy` |
| `add_add_same` | `VendorRemove`, `VendorCopy`, `RestoreRemove` |
| `rollback_read` | `VendorRemove`, `VendorCopy`, `WriteLock` only if the failing add saves nothing (confirm with TLC) |
| `fetch_v1_v2` | the three restore actions, `VendorRemove`, `VendorCopy` |
| `vendor_sync` | the three restore actions |

The table is confirmed against TLC's coverage output when the model is written. Any difference is corrected in the manifest, not hidden.

**Cost.** Two processes and short programs keep each command small. The runner has no cap on states for each command, so every command records `distinct_states`. The calibration entry reports the model's total TLC wall time (about 30 commands), which becomes an input to `promote-formal-ci-gate`'s `budget_seconds`.

### D5. The fix configurations

- **Advisory lock, modelled as a plain mutex.** `Acquire(p, L)` waits until no process holds L, then takes it. `Release` gives it up. A process's project mutex is held from before its first read of either file (before the snapshots) until after its last write or rollback. That applies to add, remove, sync, update, vendor, and the tidy reader. Under `"project_and_source"`, the source mutex for a URL is also held from checkout until the verified cache copy, and it is always taken after the project mutex. Where the lock files live, and whether they block with context cancellation, is left to the fix change, per the orchestrator's decision.
- **Atomic restore (F10).** `RestoreTruncate` and `RestoreWrite` become one atomic replace. A mutex alone also satisfies `ReadersSeeWholeFiles`, but only if every reader, including discovery and `module deps`, takes it. The atomic restore does not depend on readers, so it is the configuration the finding points to.
- **Notes for the follow-up change** (these are not requirements of this change):
  - The Unix primitive is `unix.Flock(fd, LOCK_EX)` on a file opened with `O_CREATE|O_RDWR`, as in `internal/container/run_lock_linux.go`.
  - On Windows, `windows.LockFileEx` from `golang.org/x/sys` (already a dependency at v0.47.0).
  - Both locks are released when the process exits.
  - Unlike the Podman lock, this lock must be real on macOS and Windows, because it protects host files.

*Alternatives:*
- Optimistic concurrency (re-read, compare, retry) was rejected. The lock write and the `invowkmod.cue` write stay two separate commits, and a rollback cannot tell its own changes from another process's.
- One global lock under `~/.invowk` would be accepted by the model: it is `"project_and_source"` with a single mutex. It is simpler, but it serialises unrelated projects during long fetches.

### D6. Binding to code without production changes

Each simulated process gets its own `Resolver` from `newResolverWithFetcher`, its own fetcher, and its own gate, and the processes synchronise only through channels. `fakeModuleFetcher` mutates its call counters without synchronisation (`resolver_test.go:31-39`), so it is never shared between processes. The tests pass under `go test -race`. Entry points per test:

| Test | Process 1 | Process 2 | Gate |
|---|---|---|---|
| `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` | `modulesync.LoadRequirements`, then `Resolver.Sync` | `Resolver.AddModuleDependency` | P1's `ListVersions` |
| `TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd` | `Resolver.Update` | `Resolver.AddModuleDependency` | P1's `ListVersions` |
| `TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove` | `Resolver.Update` | `Resolver.RemoveModuleDependency` | P1's `ListVersions` |
| `TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd` | `Resolver.AddModuleDependency` (x) | `Resolver.AddModuleDependency` (x) | P2's `ListVersions`, released after P1 returns |
| `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` | `Resolver.Add` (r@v1), working directory A | `Resolver.Add` (r@v2), working directory B, same `cacheDir` | The real `GitFetcher` for each, with P1's wrapped to block after `Fetch` returns |
| `TestConcurrentEdits_VendorFromStaleLock` (in `moduleops`) | `vendorDependenciesWithResolver`, with a real `Resolver` wrapped to block after `LoadDeclaredFromLock` returns | `Resolver.Update` to a new version | P1's `vendorDependencyResolver` wrapper |

The `SyncModule`, `UpdateModule`, and `TidyModule` wrappers are not replayed: they build their own resolver with the real `GitFetcher`. They are covered by correspondence rows instead. The sync test calls `LoadRequirements` and then `Resolver.Sync`, which is the wrapper's body. The correspondence row for `SyncModule` records that transcription.

F10 is model-only: `os.WriteFile` has no seam between its truncate and its write. The same holds for the narrow `invowkmod.cue` read-modify-write inside `AddRequirement`, and for the `RemoveAll`-to-copy window inside `VendorModules`. Vendor against a lock-changing update can be replayed through `vendorDependenciesWithResolver`.

F9 needs a module repository with two tags, where the module ID is the same at both tags and the content differs. `newTestGitRepo` (`git_test.go:23`) writes no `invowkmod.cue`, so a helper builds the repository with `writeIntegrityModule` from `resolver_integrity_test.go`.

Each test asserts today's behaviour and names its finding and manifest command in its doc comment. The follow-up fix change inverts it. `TestConcurrentEdits_VendorFromStaleLock` exists only if the model reports a counterexample; otherwise the case is recorded as refuted and the test is not added.

### D7. Correspondence

There is one symbol per row and one binding test per row. A row is repeated to bind a second test. `scripts/formal.py correspondence` resolves the leaf after the last `.`.

| Symbol | File | Binding |
|---|---|---|
| `Resolver.AddModuleDependency` | `use_cases.go` | `TestConcurrentEdits_FindingF8_DuplicateAddRollsBackOtherAdd` |
| `Resolver.AddModuleDependency` | `use_cases.go` | `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` |
| `Resolver.RemoveModuleDependency` | `use_cases.go` | `TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove` |
| `fileSnapshot.restore` | `use_cases.go` | `-` (F10 is model-only: no seam in `os.WriteFile`) |
| `SyncModule` | `use_cases.go` | `-` (wrapper; the `requires` read is transcribed in the sync replay) |
| `UpdateModule` | `use_cases.go` | `-` (wrapper) |
| `TidyModule` | `use_cases.go` | `-` (no fetcher seam; modelled as a reader) |
| `Resolver.Add` | `resolver.go` | `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` |
| `Resolver.Remove` | `resolver.go` | `TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove` |
| `Resolver.Sync` | `resolver.go` | `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` |
| `Resolver.Update` | `resolver.go` | `TestConcurrentEdits_FindingF8_UpdateOverwritesConcurrentAdd` |
| `Resolver.Update` | `resolver.go` | `TestConcurrentEdits_FindingF8_UpdateResurrectsConcurrentRemove` |
| `Resolver.resolveOne` | `resolver_deps.go` | `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` |
| `Resolver.cacheModule` | `cache.go` | `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` |
| `GitFetcher.checkout` | `git.go` | `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent` |
| `AddRequirement` | `pkg/invowkmod/invowkmod_edit.go` | `-` (narrow window with no seam) |
| `RemoveRequirement` | `pkg/invowkmod/invowkmod_edit.go` | `-` (narrow window with no seam) |
| `LockFile.Save` | `pkg/invowkmod/lockfile.go` | `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` |
| `vendorDependenciesWithResolver` | `internal/app/moduleops/vendor_dependencies.go` | `TestConcurrentEdits_VendorFromStaleLock`, or `-` if refuted |
| `VendorModules` | `internal/app/moduleops/vendor.go` | `-` (the `RemoveAll`-to-copy window has no seam) |

When `mutation-test-formal-bindings` lands, its completeness guard requires every test in the two new files to appear in some row. The `AtomicWrite.tla` row for `LockFile.Save` is updated to point at `ConcurrentModuleEdits`.

## Risks / Trade-offs

- [A transcription error in the model hides a race] → Each step carries a comment with its source line. Each reported counterexample is replayed where a seam exists, and otherwise marked model-only.
- [A test that asserts a bug looks like an endorsement] → Names follow `TestConcurrentEdits_FindingF<n>_…`, the doc comments point at the fix change, and the README lists the finding as open.
- [The maintainer judges a finding acceptable, e.g. "don't run two syncs at once"] → The README verdict column records that decision, and the fix configurations stay as validated design.
- [The model's mutex hides Windows `LockFileEx` and rename interaction] → The follow-up change must add Windows CI coverage.
- [The dead-action table drifts from TLC's coverage] → The runner already fails when an action is uncovered and unlisted, and `formal-infra-refinements` rejects unknown keys. Both are fixed in the manifest.

## Migration Plan

None. This change adds a model, manifest entries, tests, and documentation rows. Rollback is to delete them.

## Open Questions

- Should the lock also cover discovery reading `invowk_modules/` while `vendor` rewrites it? This is deferred as not cheap (see Non-Goals).

## Review edits not applied as written

- Review item 4 offered a choice between generic step actions and a dead-actions table. D4 does both: the generic actions minimise dead actions, and the table lists the rest. With one program per scenario, neither approach alone reaches zero dead actions.
- Review item 14 (trace validation) is answered with a deferral in Non-Goals rather than optional tasks, for the reason given there.
