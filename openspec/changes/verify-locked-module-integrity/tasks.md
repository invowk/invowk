## 1. Lock baseline

- [x] 1.1 Add `LockedModule.ExpectedContentHash` to `pkg/invowkmod` (design D1). Add unit tests covering same version, re-pointed commit, version change, and v1.0 entries without a hash
- [x] 1.2 Replace `knownHashes map[ModuleRefKey]ContentHash` with `map[ModuleRefKey]LockedModule` across `resolveAll`, `resolveOne`, `Sync`, `Add`, `Update`, and `tidyToFixedPoint`. Rename `loadExistingLockHashes` to `loadLockedModules` and keep its fail-closed errors. Carry the existing comments over to the new locations

## 2. Verification logic

- [x] 2.1 Add `ErrLockedCommitMismatch` and `LockedCommitMismatchError`. Add key and version context to `ContentHashMismatchError`. Follow the sentinel and wrapper conventions
- [x] 2.2 Implement the expectation decision in `resolveOne` right after the fetch (design D2), passing one `cacheExpectation` into `cacheModule`
- [x] 2.3 Make `cacheModule` verify the hash on both paths through one `verifyModuleHash` helper, and on a fresh-copy mismatch remove only the directory it created (design D3)
- [x] 2.4 Map both errors to a remediation hint in `cmd/invowk` (`lockIntegrityHint`), printed by `module add`, `sync`, `update`, and `tidy`, with a unit test

## 3. Tests

- [x] 3.1 Unit tests with `newResolverWithFetcher` and a fake `moduleFetcher`, covering:
  - fresh cache, matching content: success, and the hash is unchanged;
  - fresh cache, different content: mismatch error, cache directory removed, lock not written;
  - existing cache, different content: mismatch error;
  - same version with a different commit: commit error, cache untouched;
  - constraint change: no comparison, and the new values are recorded;
  - new version already cached: no false mismatch;
  - `Update` with no newer version: the checks apply;
  - v1.0 entry without a hash: the commit is checked and no hash is expected;
  - a new module: success, plus the warning.
- [x] 3.2 Add a tidy test showing that the baseline passes through all fixed-point rounds
- [x] 3.3 Add a CLI testscript for the fresh-cache mismatch if a local-git fixture exists in `tests/cli/testdata`. Otherwise record why the unit tests cover it. None exists: sync accepts only `https://`, `git@`, and `ssh://` URLs, so a testscript would need the network. The fake-fetcher tests drive the real `Sync` and `Update` end to end, and five seeded defects were each caught
- [x] 3.4 Confirm that the existing `resolver_cache_test.go` cases still pass, and adjust any that relied on the fresh-cache path skipping the comparison

## 4. Documentation

- [x] 4.1 Update `website/docs/modules/dependencies/lock-file.mdx`: the `git_commit` and `content_hash` rows, a usage note on sync-time verification, and the re-tag procedure. Mirror the changes in the `pt-BR` translation
- [x] 4.2 Update the README lock-file section and `website/docs/security/audit.mdx` wherever they describe sync-time tamper detection
- [x] 4.3 Update `adopt-formal-verification` design D9 to mark F5 as fixed by this change, and add a `LockIntegrity` fixed-configuration expectation

## 5. Verification

- [x] 5.1 Run `/simplify` on the changed code
- [x] 5.2 Run `make tidy`, `make license-check`, `make lint`, and `make check-file-length`
- [ ] 5.3 Run `make test`, then `make test-cli`. Locally, every package passes except five `internal/runtime` container integration tests and `TestContainerCLI`. All six fail identically on a clean `main` worktree with "Permission denied" on the `/workspace` mount (a local Docker engine issue). `make test-cli` ran without `-race`, because this machine has no gcc for cgo. CI must confirm
- [x] 5.4 Run `make check-baseline` and triage new goplint findings. Of 13 new findings, 8 were fixed in code: cache helpers became `Resolver` methods, the copy moved into its own helper so the casts stay conclusive, and the expectation struct became typed parameters. One CLI display helper got an exception, following `invowk.formatDuration`. Five nonzero fields on error structs were baselined, following the existing `ContentHashMismatchError` fields
- [x] 5.5 Run `cd website && npm run build`
- [ ] 5.6 Run `make sonar-local` and resolve issues. Blocked locally because `jq` is not installed, and the branch has no SonarCloud analysis until it is pushed
- [ ] 5.7 Run `/learn`
