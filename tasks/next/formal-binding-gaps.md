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
calibration). The remaining 419 distinct survivors are baselined as deferred
`binding-gap` entries: the binding named in the table does not constrain the
mutated behaviour. No survivor was shown to be a product defect, so no finding
id was allocated (F18 stays free).

## Rows whose binding does not reach the function

The table claimed a binding that never called the function, so every mutant of
it escaped. The binding reach gate (`make formal-reach`, harden-formal-instruments,
2026-11-14) found eight such rows: the seven this run listed, plus `Auth` /
`Server.ValidateToken`. Their status:

- `Invowkfile.GetEffectiveWorkDir` (`pkg/invowkfile/invowkfile.go`, 20 survivors): **rebound** to `TestModulePathContainment_ContainerWorkDirGolden`, which calls it through `getContainerWorkDir`.
- `CustomCheckScript.ResolveWithFSAndModule` (`pkg/invowkfile/dependency.go`, 15 survivors): **rebound**; `TestModulePathContainment_GoldenVectors` replays it on every `ScriptRead` instance with a recording reader, F13 included. Calibration note (decision 11): the seeded defect "drop the `validateScriptPathContainment` call" survives, because it is redundant at this entry point, fully masked by `ScriptFilePath.Validate` (no input class is rejected only by `validateScriptPathContainment`; `TestModulePathContainment_ContainmentLayerGolden` binds that layer directly). The calibration seeds "resolve against the module's parent" instead, which the golden catches. This is not a ledger entry.
- `LoadEnvFile` (`internal/runtime/dotenv.go`, 11 survivors): **rebound**; the golden calls it on every `EnvRead` instance of an admitted module that `ValidateEnvFilePath` accepts, F17 included.
- `newVirtualPathResolverForFilesystem` (`internal/runtime/virtual_policy.go`, 12 survivors): **rebound** to `TestVirtualPathHarness_RootConstruction`.
- `VirtualFilesystemConfig.EffectiveAccess` (`pkg/invowkfile/virtual_filesystem.go`, 1 survivor): **rebound** to `TestVirtualPathHarness_RootConstruction`.
- `validateDestinationPath` (`internal/app/moduleops/packaging.go`, 12 survivors): **unbound** (decision 5). The model's destination check, `unpackInside[destCheck]`, transcribes `normalizeZIPPath` followed by this function; alone it accepts an `AbsPath` name, and after `normalizeZIPPath` its rejection is unreachable on POSIX.
- `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go`, 22 survivors): **rebound** to `TestVirtualPathHarness_RootConstruction` with the narrow claim "anchors are explicit allowed roots" (decision 14, superseding decision 8's unbinding): the reported `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work` anchors are accepted and `@home/.ssh` is denied under restricted access. The anchor paths are inputs from the production resolver and are not verified; the model states no per-OS layout. The replay catches only an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, and an anchor covering `@home/.ssh` (decision 17). Two drops are masked (decision 31): `@work` is always the workdir root, so dropping it is an equivalent defect, and dropping `@tmp` is masked off Windows while `TMPDIR` lies under `/tmp`, by `defaultTempRoots()`, which is the case on the Linux reach lane; an `@tmp` probe with `TMPDIR` outside `/tmp` would close it. Its 22 baselined survivors are planned again, but bound is not covered: most mutate the per-OS layout and the environment overrides, which the replay does not assert. Rerun and retriage them; the ledger is unchanged (D9).
- `Server.ValidateToken` (`internal/sshserver/server_auth.go`, 8 survivors): **unbound** (decision 1). It has no production caller; the model's `Auth` is implemented by `Server.admitConn` (row `Auth/End atomicity`).

Retriage flags:
- The 81 survivors of the six rebound functions keep their now-stale reason ("never calls ..."); rerun and retriage them.
- The 20 survivors of the two unbound rows (`validateDestinationPath` 12, `ValidateToken` 8) name functions the plan no longer targets. They are listed in `[triage] orphaned_survivors` in `formal/manifest.toml`, which `triage --check --base` keeps from growing.
- The baseline carries one plan-inputs digest, and the change edited killer-test files many functions share, so by the digest convention the rerun evidence of all 419 survivors is stale. `triage --check` still passes because it compares the baseline digest with the rerun records, not with the current tree.

## Future work: survivor triage

One follow-up change owns all of this:
- triage the 419 survivors and refresh the baseline (new reruns at the new digest);
- re-point the `Auth` row to `Server.admitConn`;
- add a `normalizeZIPPath` row for `unpackInside`;
- model per-OS anchors in `VirtualPathHarness.als` (decision 8), then widen the `anchors` row past its narrow claim (decision 14);
- reconcile the Alloy-only "`validateDestinationPath` omitted" calibration claim in `module-path-containment-model` ("Seeded copy, discovery, hash, unpack, and harness calibration defects"), which no Go test detects;
- rename the 34 unresolved model elements, re-key their ledger survivors, and empty `[correspondence] unresolved_elements` (decision 7);
- empty `[triage] orphaned_survivors` (decision 9) and bind the 19 `[triage] binding_gaps` (decision 15).
- re-point the type-symbol rows whose claim a function implements (decision 18). They are by design today, so the `[triage]` lists do not carry them:
  - `Watch` `backend` (`watcherBackend`, `formal/tla/Watch.tla:22`): "closed channels modelled as fatal errors" is implemented by `Watcher.Run` (`internal/watch/watcher.go:432-433`, `:471-472`), which the `loop` row already binds;
  - `ScopeConstruction` `globalChildren` (`IsGlobalModule`, `formal/alloy/ScopeConstruction.als:25`): vendored children inherit the global flag in `Discovery.appendModulesWithVendored` (`internal/discovery/discovery_files.go:723-728`);
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

`internal/app/modulesync/resolver.go`, 12 survivors. TestSyncFreshCacheRejectsChangedContent covers one rejection; empty requirements, transitive diagnostics, lock-save errors and the mutex are unasserted.

### tidyToFixedPoint

`internal/app/modulesync/resolver_tidy.go`, 11 survivors. Tidy bindings compare the added key set; mutants that only change redundant rounds or the known-set bookkeeping survive.

### Base.TransitionToFailed

`internal/core/serverbase/base.go`, 5 survivors. The sequential model test compares states; lastErr, error delivery, cancel and the returned error are unasserted.

### Base.TransitionToStarting

`internal/core/serverbase/base.go`, 6 survivors. The sequential model test compares states; error values, the lock scope and the stored context are unasserted.

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

`internal/runtime/dotenv.go`, 11 survivors. The golden replay never calls LoadEnvFile (only the F17 characterisation test does), so the row's binding does not reach the function.

### newVirtualPathResolverForFilesystem

`internal/runtime/virtual_policy.go`, 12 survivors. The harness golden builds its validator over materialised roots directly and never calls the resolver constructor.

### normalizeExistingOrParent

`internal/runtime/virtual_policy.go`, 12 survivors. Golden paths are absolute and non-empty, so the empty-path, cwd and os.Getwd branches are unreached.

### pathWithin

`internal/runtime/virtual_policy.go`, 8 survivors. Some disjuncts never decide a golden instance (filepath.Rel never returns ""; a bare ".." is not generated).

### standardVirtualAnchorsForOS

`internal/runtime/virtual_policy.go`, 22 survivors. The harness golden never calls standardVirtualAnchorsForOS; only the host OS case can run on one platform. Since harden-formal-instruments (decision 14) the construction replay reaches it for the host GOOS and checks only that anchors are explicit allowed roots; the per-OS paths and environment overrides stay unasserted.

### virtualPathValidator.validate

`internal/runtime/virtual_policy.go`, 3 survivors. Error-message and early-return branches no golden instance reaches.

### Server.GenerateToken

`internal/sshserver/server_auth.go`, 14 survivors. The lifecycle test checks token acceptance; token encoding, crypto/rand errors and expiry fields are unasserted.

### Server.ValidateToken

`internal/sshserver/server_auth.go`, 8 survivors. The lifecycle test checks acceptance outcomes; expiry and bookkeeping branches are unasserted.

### Server.admitConn

`internal/sshserver/server_conns.go`, 3 survivors. The race test asserts only that an admitted connection is closed after revocation, so never admitting passes.

### Watcher.Run

`internal/watch/watcher.go`, 38 survivors. The skip-if-busy scenario checks delivered bursts; error, shutdown and filtering paths are unasserted.

### atomicWriteFile

`pkg/fspath/atomic.go`, 4 survivors. Failure injection covers each step's failure; mutants in cleanup and error wrapping survive.

### CustomCheckScript.ResolveWithFSAndModule

`pkg/invowkfile/dependency.go`, 15 survivors. The golden replay never calls CustomCheckScript.ResolveWithFSAndModule (only the F13 characterisation test does), so the row's binding does not reach the function. Since harden-formal-instruments the golden replays it; its `validateScriptPathContainment` call is redundant at this entry point (fully masked by `ScriptFilePath.Validate`), so a mutant that drops it is expected to survive here and is caught only by the containment-layer golden.

### Implementation.ResolveScriptWithFSAndModule

`pkg/invowkfile/implementation.go`, 3 survivors. Branches no golden instance reaches (inline scripts, read errors).

### validateScriptPathContainment

`pkg/invowkfile/implementation.go`, 7 survivors. The containment-layer golden compares the verdict; error wrapping and Rel-failure branches are unasserted.

### Invowkfile.GetEffectiveWorkDir

`pkg/invowkfile/invowkfile.go`, 20 survivors. The ModulePathContainment golden never calls GetEffectiveWorkDir; only the container-workdir golden reaches it and the row does not name that test.

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
