# Formal-binding gaps from the first formal-bindings mutation run

> **STATUS: OPEN** (2026-09-25). Follow-up of the `mutation-test-formal-bindings`
> change. Every entry of `tools/mutation/triage/formal-bindings.toml` points at a
> section here.

## Summary

The first `make mutation-formal` run mutated the 49 functions that bound rows of
the correspondence tables name (30 files): 809 mutants, 310 killed, 441
escaped, 58 skipped, 0 errored, 22 of the kills by timeout. Strengthening the
module-copy assertion in `TestModulePathContainment_GoldenVectors` closed 19
of the escapes (listed under `[[closed]]` and in `ModulePathContainment`'s
calibration). The remaining 419 distinct survivors were baselined as deferred
`binding-gap` entries: the binding named in the table does not constrain the
mutated behaviour. No survivor was shown to be a product defect, so no finding
id was allocated (F18 stays free). refresh-anchors-mutation-ledger (2026-11-19)
replaced the 22 stale `standardVirtualAnchorsForOS` survivors with the 5 of its
new body, which left 403 distinct ids in the ledger and in the baseline (no id
is listed twice any more). The partial refresh of
module-read-physical-containment (2026-11-19) removed 6 and added 35, so both
held 432. The partial refresh of virtual-path-physical-normalisation
(2026-11-24) removed 21 and added 1, so both held 412. The partial refresh of
virtual-path-no-follow-operands (2026-11-24) removed none and added 23 (4
`equivalent`, 19 deferred `binding-gap`), so both held 435 (439 on main
after subsequent refreshes). The partial refresh of
`module-concurrency-rollback` (2026-11-25) removed none and added 198
deferred `binding-gap` entries across the newly bound and rewritten
module-concurrency functions, so both held 637. The partial refresh of
`tui-servers-acp-followups` (2026-11-25, redone on top of `29735d3d`)
removed 5 and added 6 (deferred `binding-gap`); both now hold 638.

## Rows whose binding does not reach the function

The table claimed a binding that never called the function, so every mutant of
it escaped. The binding reach gate (`make formal-reach`, harden-formal-instruments,
2026-11-14) found eight such rows: the seven this run listed, plus `Auth` /
`Server.ValidateToken`. Their status:

- `Invowkfile.GetEffectiveWorkDir` (`pkg/invowkfile/invowkfile.go`, 20 survivors until #181, 21 since: refresh-anchors-mutation-ledger (#181) added one `equivalent` entry): **rebound** to `TestModulePathContainment_ContainerWorkDirGolden`, which calls it through `getContainerWorkDir`.
- `CustomCheckScript.ResolveWithFSAndModule` (`pkg/invowkfile/dependency.go`, 15 survivors until the 2026-11-19 refresh of module-read-physical-containment, 10 since): **rebound**; `TestModulePathContainment_GoldenVectors` replays it on every `ScriptRead` instance with a recording reader, F13 included. Calibration note (decision 11): the seeded defect "drop the `validateScriptPathContainment` call" survives, because it is redundant at this entry point, fully masked by `ScriptFilePath.Validate` (no input class is rejected only by `validateScriptPathContainment`; `TestModulePathContainment_ContainmentLayerGolden` binds that layer directly). The calibration seeds "resolve against the module's parent" instead, which the golden catches. This is not a ledger entry.
- `LoadEnvFile` (`internal/runtime/dotenv.go`, 11 survivors until the 2026-11-19 refresh of module-read-physical-containment, 32 since): **rebound**; the golden calls it on every `EnvRead` instance of an admitted module that `ValidateEnvFilePath` accepts, F17 included.
- `newVirtualPathResolverForFilesystem` (`internal/runtime/virtual_policy.go`, 12 survivors): **rebound** to `TestVirtualPathHarness_RootConstruction`.
- `VirtualFilesystemConfig.EffectiveAccess` (`pkg/invowkfile/virtual_filesystem.go`, 1 survivor): **rebound** to `TestVirtualPathHarness_RootConstruction`.
- `validateDestinationPath` (`internal/app/moduleops/packaging.go`, 12 survivors): **unbound** (decision 5). The model's destination check, `unpackInside[destCheck]`, transcribes `normalizeZIPPath` followed by this function; alone it accepts an `AbsPath` name, and after `normalizeZIPPath` its rejection is unreachable on POSIX.
- `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go`, 22 survivors until 2026-11-19, 5 since): **rebound** to `TestVirtualPathHarness_RootConstruction` with the narrow claim "anchors are explicit allowed roots" (decision 14, superseding decision 8's unbinding): the reported `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work` anchors are accepted and `@home/.ssh` is denied under restricted access. The anchor paths are inputs from the production resolver and are not verified; the model states no per-OS layout. The replay catches only an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, and an anchor covering `@home/.ssh` (decision 17). Two drops are masked (decision 31): `@work` is always the workdir root, so dropping it is an equivalent defect, and dropping `@tmp` is masked off Windows while `TMPDIR` lies under `/tmp`, by `defaultTempRoots()`, which is the case on the Linux reach lane; an `@tmp` probe with `TMPDIR` outside `/tmp` would close it. Its 22 baselined survivors are planned again, but bound is not covered: most mutate the per-OS layout and the environment overrides, which the replay does not assert. Rerun and retriage them; the ledger is unchanged (D9). **Retriaged by require-absolute-base-dirs (S4, 2026-11-15):** S4 moved the per-OS layout and the environment overrides into `internal/basedir` (`Dirs.ConfigHome`, `DataHome`, `CacheHome`, `StateHome`, which no correspondence row names) and rewrote this function to read them, so all 22 baselined survivors mutate a body that no longer exists (`getenv`, `firstNonEmpty`, the darwin case). A `make mutation-formal` run on the rebased S4 tree (report only, baseline untouched) generated 14 mutants in the new body: 9 killed (the three cleared literal fields, the four dropped default-case assignments, the default-case no-op, `return nil`) and 5 escaped, all in the `windows` case (the case no-op and its four dropped assignments), which the binding reaches only on a Windows host. They are binding gaps, not untested code: `TestStandardVirtualAnchorsMatchLegacy` (not a killer) passes `goos = "windows"` on every host and compares each anchor with the frozen pre-change oracle. The S4-changed lines of `internal/app/modulecache/cache.go` and `internal/runtime/container_provision.go` produced no mutant in a planned function. The ledger and the baseline were unchanged (D9). **Refreshed by refresh-anchors-mutation-ledger (2026-11-19, decision 35):** a fresh report-only `make mutation-formal` run on 567e7b00 generated the same 14 mutants (9 killed, 5 escaped, all 5 in the `case goosWindows` block, lines 165-169). Of the 22 old ids, 19 no longer exist and 3 (`virtualAnchorTmp`/`virtualAnchorWork` field-clear, `return anchors`) regenerate in the new body and are killed; all 22 left the ledger and the baseline. The 5 Windows-case survivors entered as `binding-gap`, with two reruns each and their own evidence `digest` (857457136e35…). `TestStandardVirtualAnchorsMatchLegacy` stays a unit test, not a killer.
- `Server.ValidateToken` (`internal/sshserver/server_auth.go`, 8 survivors): **unbound** (decision 1). It has no production caller; the model's `Auth` is implemented by `Server.admitConn` (row `Auth/End atomicity`).

Retriage flags:
- The six rebound functions hold 81 survivors in the ledger at fa70957b (`GetEffectiveWorkDir` 21, `ResolveWithFSAndModule` 10, `LoadEnvFile` 32, `newVirtualPathResolverForFilesystem` 12, `EffectiveAccess` 1, `standardVirtualAnchorsForOS` 5). 33 of them keep their now-stale reason ("never calls ..."): `GetEffectiveWorkDir` 20, `newVirtualPathResolverForFilesystem` 12 and `EffectiveAccess` 1; rerun and retriage them. The other 48 carry reasons written by the refreshes above (refresh-anchors-mutation-ledger in #181, module-read-physical-containment in #188).
- The 20 survivors of the two unbound rows (`validateDestinationPath` 12, `ValidateToken` 8) name functions the plan no longer targets. They are listed in `[triage] orphaned_survivors` in `formal/manifest.toml`, which `triage --check --base` keeps from growing.
- The baseline carries one plan-inputs digest, and the change edited killer-test files many functions share, so by the digest convention the rerun evidence of the 391 survivors still at the baseline's digest is stale (fa70957b: 432 baseline ids, of which 41 carry their own evidence digest from a partial refresh; 397 of 403 after #181, before #188 merged the module-read-physical-containment refresh). The tree's plan-inputs digest at fa70957b (c7bc8354…) matches none of the three recorded digests (baseline 7c0bfca91bee…, per-mutant 83fa6108536c… and 857457136e35…). That check compares the baseline digest with the rerun records, not with the tree; since refresh-anchors-mutation-ledger, a separate per-entry anchor check fails when an entry's mutated lines leave its function.
- The 2026-11-19 run on 567e7b00 regenerated 380 of the 419 then-baselined ids: 350 escaped and 30 killed (3 of them the anchors ids above). The 27 other now-killed survivors stay in the ledger until the full refresh: the anchor check finds dead mutants, not killed ones.

## Future work: survivor triage

One follow-up change owns all of this:
- triage the 391 survivors left at the old digest and refresh the baseline (new reruns at the new digest), which also drops the 27 survivors now killed and the per-mutant digests;
- empty `[triage] stale_survivors` (seeded empty on 2026-11-19; keep it empty: a change that rewrites a bound function refreshes its entries instead);
- re-point the `Auth` row to `Server.admitConn`;
- add a `normalizeZIPPath` row for `unpackInside`;
- model per-OS anchors in `VirtualPathHarness.als` (decision 8), then widen the `anchors` row past its narrow claim (decision 14);
- reconcile the Alloy-only "`validateDestinationPath` omitted" calibration claim in `module-path-containment-model` ("Seeded copy, discovery, hash, unpack, and harness calibration defects"), which no Go test detects;
- rename the 34 unresolved model elements, re-key their ledger survivors, and empty `[correspondence] unresolved_elements` (decision 7);
- empty `[triage] orphaned_survivors` (decision 9) and bind the 19 `[triage] binding_gaps` (decision 15).
- re-point the type-symbol rows whose claim a function implements (decision 18). They are by design today, so the `[triage]` lists do not carry them:
  - `Watch` `backend` (`watcherBackend`, `formal/tla/Watch.tla:22`): "closed channels modelled as fatal errors" is implemented by `Watcher.Run` (`internal/watch/watcher.go:432-433`, `:471-472`), which the `loop` row already binds;
  - `ScopeConstruction` `globalChildren` (`IsGlobalModule`, `formal/alloy/ScopeConstruction.als:25`): vendored children inherit the global flag in `Discovery.appendModulesWithVendored` (`internal/discovery/discovery_files.go:760-765`), and `Discovery.mergeDuplicateModule` (`internal/discovery/discovery_module_dedupe.go:93-98`) keeps the claim when a global duplicate of an appended module directory is merged into it: a re-pointed row binds both;
  - `ScopeConstruction` `RootSrc` (`SourceID`, `formal/alloy/ScopeConstruction.als:26`): the cwd invowkfile gets `SourceIDInvowkfile` and no `ModuleID` in `Discovery.DiscoverCommandSet` (`internal/discovery/discovery_commands.go:260-262`).

## Per-function gaps

Each section lists why the named binding misses the survivors. Resolve each
mutant per the feedback loop in `.agents/skills/formal-verification/SKILL.md`:
strengthen the binding (then list it under `[[closed]]` and in the model's
calibration), extend the model, state the abstraction in the row, or mark it
equivalent. Many survivors are error-propagation mutants on I/O or crypto
failures that no golden instance can trigger; those are candidates for an
explicit abstraction note (`I/O errors are not modelled`) on their rows.

### buildCommandScope

`internal/app/deps/deps.go`, 5 survivors. Scope goldens and the rapid intent test compare allow/deny per target; root-caller, ModuleID and module-path candidate fields are not asserted.

### commandScopeDecision

`internal/app/deps/deps.go`, 3 survivors. Scope goldens compare Allowed only; the decision's target and reason fields are not asserted.

### copyDir

`internal/app/modulecache/cache.go`, 13 survivors. I/O error propagation: no materialised golden tree makes a copy step fail.

### validateDestinationPath

`internal/app/moduleops/packaging.go`, 12 survivors. No test calls validateDestinationPath; the golden asserts unpackRejectsEscape only as a model fact, so the row's binding does not reach the function.

### VendorModules

`internal/app/moduleops/vendor.go`, 43 survivors. TestVendorHashMismatchLeavesCopiedContent covers one hash-mismatch path; I/O errors, conflicts, hashless entries, pruning and result fields are unasserted.

### Resolver.cacheModule

`internal/app/modulesync/cache.go`, 15 survivors. The cache tests cover tampered and fresh content; stat, copy and cleanup error paths and the unhashed branch are unasserted.

### Resolver.Sync

`internal/app/modulesync/resolver.go`, 13 survivors. TestSyncFreshCacheRejectsChangedContent covers one rejection; empty requirements, transitive diagnostics, lock-save errors, the mutex and the `lockWritten` result (reporting only; asserted by the unbound `TestResolverSyncWithoutRequirementsReportsNoWrite` and `TestResolverRepeatedSyncReportsWriteThenSkip`) are unasserted.

### tidyToFixedPoint

`internal/app/modulesync/resolver_tidy.go`, 11 survivors. Tidy bindings compare the added key set; mutants that only change redundant rounds or the known-set bookkeeping survive.

### vendorDependenciesWithResolver

`internal/app/moduleops/vendor_dependencies.go`, 17 deferred `binding-gap` survivors, added by `module-concurrency-rollback` (2026-11-25). `TestConcurrentEdits_VendorFromStaleLock` replays a successful concurrent vendor under the project lock; error returns, empty-requirements skip and result fields are covered outside the binding cell by `TestVendorDependenciesRequiresLocker`, `TestVendorDependenciesWithoutRequirementsCreatesOnlyProjectLock`, `TestVendorDependenciesHoldsProjectLock` and `vendor_test.go`.

### Resolver.LockProject and Resolver.lockSource

`internal/app/modulesync/edit_locks.go`, 20 deferred `binding-gap` survivors (`LockProject` 15, `lockSource` 5), added by `module-concurrency-rollback` (2026-11-25). The `Acquire project` and `Acquire source` killers are concurrency replays in which every lock is granted once per resolver; refusals, lock-order errors, `PhysicalDir`/path/open failures, wait callbacks and cancellations are checked outside the binding cell by `edit_locks_test.go`, `edit_locks_edge_test.go` and `source_locks_test.go`.

### GitFetcher.checkout

`internal/app/modulesync/git.go`, 11 deferred `binding-gap` survivors, added by `module-concurrency-rollback` (2026-11-25). `TestConcurrentEdits_F9SourceLockRecordsMatchingContent` exercises clean two-tag checkouts; bare-repo URL normalization and git failure branches are checked outside the binding cell by `git_test.go`.

### Resolver.Add, Resolver.Remove and Resolver.Update

`internal/app/modulesync/resolver.go`, 24 deferred `binding-gap` survivors (`Add` 12, `Remove` 1, `Update` 11), added by `module-concurrency-rollback` (2026-11-25). Concurrent replays exercise successful operations; missing-entry, resolve-error and lock-save error branches are checked outside the binding cell by `resolver_test.go` and `resolver_lock_test.go`.

### Resolver.resolveOne

`internal/app/modulesync/resolver_deps.go`, 46 deferred `binding-gap` survivors, added by `module-concurrency-rollback` (2026-11-25). The F9 replay resolves two versions under the source lock; version-selection, lock-acquisition failure and cache-verification error branches are checked outside the binding cell by `resolver_test.go` and `source_locks_test.go`.

### Module edit use cases and fileSnapshot.restore

`internal/app/modulesync/use_cases.go`, 80 deferred `binding-gap` survivors (`AddModuleDependency` 8, `Resolver.AddModuleDependency` 14, `RemoveModuleDependency` 3, `Resolver.RemoveModuleDependency` 17, `SyncModule` 3, `Resolver.SyncModule` 9, `UpdateModule` 3, `Resolver.UpdateModule` 4, `fileSnapshot.restore` 19), added by `module-concurrency-rollback` (2026-11-25). Concurrent replays exercise happy paths and F10's lock-file atomic replace; locker validation, lock-error, declaration-error, symlink/mode-preserving and rollback-failure branches are checked outside the binding cell by `use_cases_*_test.go`, `edit_locks_test.go` and `edit_locks_edge_test.go`.

### Base.TransitionToFailed

`internal/core/serverbase/base.go`, 5 survivors. The sequential model test compares states; lastErr, error delivery, cancel and the returned error are unasserted.

### Base.TransitionToStarting

`internal/core/serverbase/base.go`, 6 survivors until the partial refresh of tui-servers-acp-followups (2026-11-25), 1 since. The guard of decision 120 rewrote the function and the row now also binds the three direct `TransitionToStarting` tests, which assert the returned error, `LastError()` and the delivery on `Err()`: of the six, three were not generated again and two are killed. The one left is the lock scope (`defer b.stateMu.Unlock()` made immediate), which no sequential test observes.

### Base.TransitionToStopped

`internal/core/serverbase/base.go`, 2 survivors. The sequential model test compares states; the terminal-state guard and the return value are unasserted.

### Base.TransitionToStopping

`internal/core/serverbase/base.go`, 6 survivors. CAS-retry branches need contention the sequential test never creates; cancel calls are unasserted.

### Base.closeErrChannelLocked

`internal/core/serverbase/base.go`, 2 survivors. The concurrent invariants test never observes whether the error channel is closed.

### CopyDir

`internal/provision/helpers.go`, 12 survivors. I/O error propagation: no materialised golden tree makes a copy step fail.

### ContainerRuntime.getContainerWorkDir

`internal/runtime/container_provision.go`, 8 survivors. The container-workdir golden covers a subset of workdir shapes; the default and outside-root returns are unasserted.

### LoadEnvFile

`internal/runtime/dotenv.go`, 32 survivors (11 before 2026-11-19). Refreshed by module-read-physical-containment (2026-11-19): the golden replay calls LoadEnvFile with the module root on every admitted, validated `EnvRead` vector, so its verdict binds the contained read. Its remaining binding gaps are branches the golden never reaches: the absolute and lexical pre-checks (masked by `ValidateEnvFilePath`), the `filepath.Rel` failure, the `OpenRoot` error branches (the golden module roots always exist), the optional skip, and the read-error wrapping. `TestLoadEnvFile_ModuleContext`, `TestLoadEnvFile_ModuleRootMissing` and `TestLoadEnvFile_ModuleRootNotADirectory` kill most of them; binding them to the `readOp | LoadEnvFile` row would close the gaps (`TestLoadEnvFile_ModuleNoSearchPermission` would first need its skips moved to an fstree-style helper). The lexical pre-check mutants are not equivalent: os.Root's own escape rejection masks them only while the module root opens. With a missing module root, or one that is not a directory, a leaving path (`..`, `../x`) or a `filepath.Rel` failure no longer returns `ErrEnvFileOutsideModule`: an optional file is skipped and a required one fails with a read error (a plain read error for both when the root is not a directory). No content is read, so this is verdict drift, not a leak. They stay deferred binding gaps until bound tests cover those cases: lexical-escape cases (`..` and `../x`, optional and required) for a missing module root and for a module root that is not a directory, and a `filepath.Rel`-failure case (a relative `basePath` under an absolute module root). The read-error wrapping mutant is already killed by `TestLoadEnvFile_ModuleContext/required_missing_file`, which the `readOp | LoadEnvFile` row does not name.

### readModuleEnvFileWith

`internal/runtime/dotenv.go`, bound by module-read-physical-containment's review (2026-11-19). The ReadFile-error classification after a successful Stat is reached only for a directory (EISDIR, returned unchanged) or under a race; a bound test injecting a readFile that returns an escape error would close the gap.

### checkModuleFilePhysicalContainment

`pkg/invowkfile/module_file_containment.go`, bound by module-read-physical-containment. Gaps: the `filepath.Rel` failure (`TestCheckModuleFilePhysicalContainment_RelFailure` kills it; add it to the `physContains` binding), the OpenRoot error wrapping (`_RootIsFile` does not assert the wrapped OS error), the Windows-only os.Root panic branch (killed on windows-latest only), and the default OS-error case (`_NoSearchPermission`, which skips directly). The not-exist conjunct mutants are `equivalent`: Stat and Lstat report not-exist together.

### newVirtualPathResolverForFilesystem

`internal/runtime/virtual_policy.go`, 4 survivors (12 before virtual-path-physical-normalisation, 2026-11-24, whose refresh found 8 of them killed). The harness golden builds its validator over materialised roots directly and never calls the resolver constructor.

### normalizeExistingOrParent

`internal/runtime/virtual_policy.go`, no survivors. virtual-path-physical-normalisation (2026-11-24) moved the resolution into `walkVirtualPath` and its helpers, so the 12 survivors of the old body were no longer generated and left the ledger.

### walkVirtualPath

`internal/runtime/virtual_policy.go`, 1 survivor, added by virtual-path-physical-normalisation (2026-11-24). Returning the zero walk when the canonical step fails drops `unresolvedLink`. Strict callers deny on that error either way; only the full-access fallback form changes, and no killer makes the canonical step fail after a walk that ended inside a link target. A `TestVirtualPathHarness_WalkOverFakeFilesystem` case with a `canonErr` on a dangling link's target directory that asserts `unresolvedLink` would close it.

### virtualPathWalker.follow

`internal/runtime/virtual_policy.go`, 10 survivors, added by virtual-path-no-follow-operands (2026-11-24), when the walk's helpers got rows of their own. Every one sits in a Windows-only branch: a link target with a volume name, a rooted target without one, a bare volume, a drive-relative target (refused), or a missing drive in a target. Off Windows `filepath.VolumeName` is always empty, so the formal-bindings lane, which runs on Linux, cannot generate those targets. `TestVirtualPathHarness_WalkOverFakeFilesystem` has a Windows-only drive-relative case; closing these needs a Windows mutation lane, or fake-filesystem cases whose volume handling does not depend on the host `filepath`.

### virtualPathResolver.resolveNoFollow

`internal/runtime/virtual_path_nofollow.go`, 6 deferred survivors and 1 equivalent, added by virtual-path-no-follow-operands (2026-11-24). No killer gives a no-follow request whose anchor expansion fails, a request that names a directory by its form and whose full resolution fails, or a volume root (`/`) as the request. A `TestVirtualPathNoFollowValidatorDirectoryCheck` or `TestVirtualPathHarness_NoFollowOperands` case for each (an unknown `@anchor`, `@unknown/`, and `/` under both access modes, asserting the error or the returned path) would close them. The removal of the whole volume-root branch is equivalent (the general path gives the same result).

### virtualPathValidator.validateNoFollow

`internal/runtime/virtual_path_nofollow.go`, 3 deferred survivors and 1 equivalent, added by virtual-path-no-follow-operands (2026-11-24). No killer checks the returned path of a request whose last element is `.` or `..`, or of one whose resolved path already ends in a separator (a volume root). Cases asserting `validateNoFollow(work, "dirlink/.")` returns the target with no separator, and `/` returns `/`, would close them.

### pathWithin

`internal/runtime/virtual_policy.go`, 8 survivors. Some disjuncts never decide a golden instance (filepath.Rel never returns ""; a bare ".." is not generated).

### standardVirtualAnchorsForOS

`internal/runtime/virtual_policy.go`, 5 survivors (22 before 2026-11-19). The harness golden never calls standardVirtualAnchorsForOS; only the host OS case can run on one platform. Since harden-formal-instruments (decision 14) the construction replay reaches it for the host GOOS and checks only that anchors are explicit allowed roots; the per-OS paths and environment overrides stay unasserted. Since require-absolute-base-dirs the per-OS bases come from `internal/basedir`, so the 22 baselined survivors were stale (they mutated the removed body). refresh-anchors-mutation-ledger replaced them with the new body's 5 survivors, all in the `windows` case: the formal-bindings mutation lane runs only on Linux, where `TestVirtualPathHarness_RootConstruction` reaches this function only for the host GOOS. `TestStandardVirtualAnchorsMatchLegacy` covers the Windows layout as a unit test and is deliberately not a killer. Closing them needs per-OS anchors in `VirtualPathHarness.als` (Future work).

### virtualPathValidator.validate

`internal/runtime/virtual_policy.go`, 2 survivors (3 before virtual-path-physical-normalisation, 2026-11-24, whose refresh found one killed). Error-message and early-return branches no golden instance reaches.

### Server.GenerateToken

`internal/sshserver/server_auth.go`, 14 survivors. The lifecycle test checks token acceptance; token encoding, crypto/rand errors and expiry fields are unasserted.

### Server.ValidateToken

`internal/sshserver/server_auth.go`, 8 survivors. The lifecycle test checks acceptance outcomes; expiry and bookkeeping branches are unasserted.

### Server.admitConn

`internal/sshserver/server_conns.go`, 3 survivors. The race test asserts only that an admitted connection is closed after revocation, so never admitting passes.

### Server.closeAcceptedConns

`internal/sshserver/server_conns.go`, 2 survivors (bound by tui-servers-acp-followups, 2026-11-25, for the F11 fix). Both drop `acceptClosed = true`. The bindings close connections the library already handed to the ConnCallback; none delivers a connection to the ConnCallback after teardown began, which needs a hook between the library's Accept and the callback, so the refusal is never exercised.

### Server.Stop

`internal/sshserver/server_lifecycle.go`, 4 survivors (the row is measured since tui-servers-acp-followups, 2026-11-25, when it stopped being a characterisation of F11). `TestHostCallbackToken_StopClosesAuthenticatedConnection` stops a running server once, so the losing branch (`WaitForShutdown` and its early return), the `stopMu` scope and the test-only `stopEntered` hook are not observed. The lifecycle tests `TestServerDoubleStop` and `TestServerConcurrentStopsDuringStart` exercise them but are not bound to this row, whose claim is the closing of connections.

### Watcher.Run

`internal/watch/watcher.go`, 38 survivors. The skip-if-busy scenario checks delivered bursts; error, shutdown and filtering paths are unasserted.

### atomicWriteFile

`pkg/fspath/atomic.go`, 4 survivors. Failure injection covers each step's failure; mutants in cleanup and error wrapping survive.

### CustomCheckScript.ResolveWithFSAndModule

`pkg/invowkfile/dependency.go`, 10 survivors (15 before 2026-11-19). The golden replay never calls CustomCheckScript.ResolveWithFSAndModule (only the F13 characterisation test does), so the row's binding does not reach the function. Since harden-formal-instruments the golden replays it; its `validateScriptPathContainment` call is redundant at this entry point (fully masked by `ScriptFilePath.Validate`), so a mutant that drops it is expected to survive here and is caught only by the containment-layer golden.

### Implementation.ResolveScriptWithFSAndModule

`pkg/invowkfile/implementation.go`, 3 survivors. Branches no golden instance reaches (inline scripts, read errors).

### validateScriptPathContainment

`pkg/invowkfile/implementation.go`, 7 survivors. The containment-layer golden compares the verdict; error wrapping and Rel-failure branches are unasserted.

### Invowkfile.GetEffectiveWorkDir

`pkg/invowkfile/invowkfile.go`, 21 survivors. 20 keep the first run's reason: the ModulePathContainment golden never calls GetEffectiveWorkDir; only the container-workdir golden reaches it and the row does not name that test (stale since the rebind above). refresh-anchors-mutation-ledger (#181) added one `equivalent` entry: the removed `if workdir == ""` body is unreachable, since every resolve call site checks `!= ""` first.

### ScriptFilePath.Validate

`pkg/invowkfile/script_file_path.go`, 7 survivors. The golden compares validity only; error text and some rejection branches decide no instance.

### ValidateContainerfilePath

`pkg/invowkfile/validation_filesystem.go`, 6 survivors. The golden compares validity only; error wrapping and branches no instance reaches survive.

### ValidateEnvFilePath

`pkg/invowkfile/validation_filesystem.go`, 7 survivors. The golden compares validity only; error wrapping and branches no instance reaches survive.

### VirtualFilesystemConfig.EffectiveAccess

`pkg/invowkfile/virtual_filesystem.go`, 1 survivor. The harness golden fixes the access mode per instance and never calls EffectiveAccess.

### CommandScope.CanCallTarget

`pkg/invowkmod/command_scope.go`, 13 survivors. Scope goldens compare Allowed only; the deny reason and target fields are not asserted.

### computeModuleHash

`pkg/invowkmod/content_hash.go`, 17 survivors. The golden asserts only that hashing does not fail; the hash value itself is never compared.

### LockedModule.ExpectedContentHash

`pkg/invowkmod/lock_integrity.go`, 7 survivors. TestSyncRejectsRepointedTag checks one commit mismatch; the returned hash and other branches are unasserted.

### inspectModuleEntry

`pkg/invowkmod/operations_validate.go`, 3 survivors. Branches no golden tree reaches.

### CheckMissingVendoredTransitiveDeps

`pkg/invowkmod/transitive_policy.go`, 3 survivors. Vendored-metadata branches no golden instance reaches.

### IsDeclaredLockedCommandSource

`pkg/invowkmod/vendored_policy.go`, 6 survivors. Bindings compare admission outcomes; branches no golden or rapid instance reaches stay unasserted.

### EvaluateVendoredModuleHash

`pkg/invowkmod/verify.go`, 6 survivors. Content hashing is abstracted, so hash-computation errors and status details are unasserted.

### FindAmbiguousLockedModuleEntries

`pkg/invowkmod/verify.go`, 2 survivors. Ordering and duplicate-handling branches no golden instance distinguishes.

### VerifyLockedVendoredModuleHash

`pkg/invowkmod/verify.go`, 6 survivors. TestLockIntegrity_HashlessEntryIsRejected covers the hashless case; mismatch and error branches are unasserted.

## Flaky mutants (not baselined)

Two `Server.admitConn` mutants in `internal/sshserver/server_conns.go` change
outcome between runs under `TestRevocationRacesAuthenticationSafely`, whose
result depends on goroutine scheduling:

- `86bfda6c2abc7c8d726b6ec84f181a04` (`defer s.tokenMu.Unlock()` →
  `s.tokenMu.Unlock()`): escaped in the baseline run, killed in both focused
  reruns, so it was dropped from the baseline.
- `4f56f81e6eb939b7fb00014854d51bea` (`s.conns[tokenValue] == nil` →
  `!= nil`): escaped in the first full run, killed in the baseline run.

A deterministic binding for `admitConn` (a sequential admit of a valid token
before any revocation, plus the race) would make both stable.

## Deferred decisions

- Sharding the profile across workers (go-mutesting forces one worker with
  `--exec`); the first run took 28 minutes locally, within the 90-minute job.
- An opt-in `--include-unbound` to measure the 51 unbound rows.
