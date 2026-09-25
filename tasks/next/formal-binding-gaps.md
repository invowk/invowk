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

The table claims a binding that never calls the function, so every mutant of
it escapes. Fix the row (name the test that does reach it, or add a replay of
the real function to the named binding) before trusting the model's claim:

- `validateDestinationPath` (`internal/app/moduleops/packaging.go`, 12 survivors): no test calls validateDestinationPath; the golden asserts unpackRejectsEscape only as a model fact, so the row's binding does not reach the function.
- `LoadEnvFile` (`internal/runtime/dotenv.go`, 11 survivors): the golden replay never calls LoadEnvFile (only the F17 characterisation test does), so the row's binding does not reach the function.
- `newVirtualPathResolverForFilesystem` (`internal/runtime/virtual_policy.go`, 12 survivors): the harness golden builds its validator over materialised roots directly and never calls the resolver constructor.
- `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go`, 22 survivors): the harness golden never calls standardVirtualAnchorsForOS; only the host OS case can run on one platform.
- `CustomCheckScript.ResolveWithFSAndModule` (`pkg/invowkfile/dependency.go`, 15 survivors): the golden replay never calls CustomCheckScript.ResolveWithFSAndModule (only the F13 characterisation test does), so the row's binding does not reach the function.
- `Invowkfile.GetEffectiveWorkDir` (`pkg/invowkfile/invowkfile.go`, 20 survivors): the ModulePathContainment golden never calls GetEffectiveWorkDir; only the container-workdir golden reaches it and the row does not name that test.
- `VirtualFilesystemConfig.EffectiveAccess` (`pkg/invowkfile/virtual_filesystem.go`, 1 survivor): the harness golden fixes the access mode per instance and never calls EffectiveAccess.

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

`internal/runtime/virtual_policy.go`, 22 survivors. The harness golden never calls standardVirtualAnchorsForOS; only the host OS case can run on one platform.

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

`pkg/invowkfile/dependency.go`, 15 survivors. The golden replay never calls CustomCheckScript.ResolveWithFSAndModule (only the F13 characterisation test does), so the row's binding does not reach the function.

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
