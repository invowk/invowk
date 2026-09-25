## Context

`resolveOne` (`internal/app/modulesync/resolver_deps.go`) runs these steps:

1. Resolves the version constraint.
2. Fetches `(repoPath, commit)`.
3. Looks up `expectedHash = knownHashes[req.Key()]`.
4. Calls `cacheModule(src, versionedCachePath, expectedHash)`.

`cacheModule` compares the expected hash only when the cache directory already exists. On a fresh cache it copies the module and returns the computed hash without comparing, even when it was handed an expected hash. `knownHashes` comes from `LockFile.ContentHashes()`, keyed by `ModuleRefKey` (URL + path, no version). The returned `commit` is stored and never compared with the lock.

Two defects follow:

- **Fresh-cache trust (F5).** An attacker who controls the remote, or re-points a tag, gets new content accepted on any machine without the cache.
- **Stale cross-version expectation.** After a version change, a cache hit on the new version is compared with the old version's hash, which reports a false mismatch.

`Sync`, `Add` (`resolver.go:136`), `Update` (`resolver.go:250`), and tidy (`resolver_tidy.go`) all pass the same map.

## Goals / Non-Goals

**Goals:**
- Make lock verification independent of local cache state.
- Detect re-pointed tags for unchanged locked versions.
- Stop applying an old version's hash to a new version.
- Keep a poisoned fresh-cache copy from persisting.

**Non-Goals:**
- v1.0 lock files without hashes (F4). Discovery-time policy for hashless entries is a separate decision.
- Inter-process locking of the cache or the lock file.
- A CLI flag to accept re-tags. The documented procedure is `invowk module remove` followed by `invowk module add`, which respects the lock file's "DO NOT EDIT MANUALLY" header.
- Vendored-copy verification (F6).

## Decisions

### D1. Pass the lock entries; put the rule on `LockedModule`

The resolver receives the existing lock file's `map[ModuleRefKey]LockedModule` in place of the version-less `map[ModuleRefKey]ContentHash`. `loadExistingLockHashes` becomes `loadLockedModules` and keeps its fail-closed behaviour on stat and parse errors. `Update` passes its live `lock.Modules`: each key is visited once and `AddModule` rewrites only that key, so no later lookup sees an updated entry.

The decision lives on the type that records the invariant: `LockedModule.ExpectedContentHash(key, version, commit)`. It returns:
- no expectation when the resolved version differs from the entry's;
- a `LockedCommitMismatchError` for the same version at a different commit;
- otherwise the locked hash, which is empty for v1.0 entries.

`LockFile.ContentHashes()` stays, because audit and vendored verification still use it.

*Alternative considered (and first implemented):* a separate `LockBaseline` projection with three fields. The `/simplify` review dropped it. It duplicated `LockedModule` fields, needed its own validation and aliases, and was rebuilt on every `Update` iteration.

### D2. Decide expectations right after the fetch

In `resolveOne`, the expectation is derived as soon as `Fetch` returns the commit. That happens before the module source is parsed or cached, so a re-pointed tag never reaches the cache and wastes no CUE parse. The expectation travels as one `cacheExpectation{key, version, hash}` value into `cacheModule`, which builds a complete mismatch error. The error is never patched after it is returned.

### D3. `cacheModule` verifies on both paths and cleans up

Both paths share one `verifyModuleHash` helper. After copying into a fresh directory, `cacheModule` computes the hash. When `expectedHash` is non-empty and differs, it removes the directory it just created and returns `ContentHashMismatchError`. The existing-cache path keeps its current comparison.

Cleanup is limited to the directory created in this call. A pre-existing directory is never removed. It is reported, and the user clears the cache explicitly.

*Alternative considered:* verify in a temporary directory, then rename. That is more robust against a crash mid-copy, but it depends on the cache layout's rename atomicity. Deferred, because F3 and the formal model's durability configurations will inform it.

### D4. Typed errors in the domain; remediation in the CLI

- `LockedCommitMismatchError{ModuleKey, Version, Locked, Fetched}` wraps the new sentinel `ErrLockedCommitMismatch`.
- `ContentHashMismatchError` gains a `Version` field, and the mismatch is wrapped with the cache directory.
- Both `Error()` texts stay factual. Following the repository rule that domain packages return typed errors and adapters render, `cmd/invowk` maps both errors to a hint through `lockIntegrityHint`. The hint is printed by `module add`, `sync`, `update`, and `tidy`, and names the concrete `invowk module remove <key>` to run.

### D5. The first-sync warning stays

The `slog.Warn` for caching without a baseline is kept. It now fires only when there is no applicable baseline: a new module, a version change, or a v1.0 entry without a hash.

## Risks / Trade-offs

- **Legitimate re-tags now fail sync.** This is intended. The error names the procedure, and the lock-file docs document it.
- **CI jobs with hand-edited lock files may start failing.** The failure surfaces real drift. It is called out as BREAKING in the proposal.
- **Cleanup races with a concurrent sync of the same module on the same machine.** No inter-process lock exists today. Cleanup removes only a directory this call created, so the worst case is a re-fetch.
- **The mismatch check is bypassed when the lock entry has no hash.** This is covered by F4 and stays out of scope. The commit check still applies.

## Migration Plan

No lock format change. Existing lock files work unchanged. Rollback is a code revert.

## Open Questions

- Should a `--accept-retag <key>` flag be added later, instead of manual entry removal? Deferred until a user asks for it.
