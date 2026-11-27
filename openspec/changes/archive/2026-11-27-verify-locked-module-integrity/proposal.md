## Why

The lock file's `content_hash` is documented as tamper detection, but `module sync`, `add`, `update`, and `tidy` only compare it when the module's cache directory already exists (`cacheModule` in `internal/app/modulesync/cache.go`). On a fresh cache, such as a new CI runner, a new machine, or a wiped `~/.invowk` cache, whatever the remote serves is copied, hashed, and written back as the new trusted hash. The locked `git_commit` is never compared, so a re-pointed upstream tag also goes unnoticed.

This is finding F5 of `adopt-formal-verification`. The maintainer chose to fix it ahead of the formal-verification phases because it is a supply-chain gap.

The same code has a second, latent defect. The expected hash is keyed by module source only (`ModuleRefKey`, which is URL plus path and carries no version), while the cache directory is versioned. After a version change, a previously cached copy of the new version is compared with the old version's hash, and sync fails with a false mismatch.

## What Changes

- Sync, add, update, and tidy derive a per-module **lock baseline** from the existing lock file: resolved version, git commit, and content hash.
- When the newly resolved version **equals** the baseline's resolved version:
  - a different git commit fails with a new actionable `LockedCommitMismatchError`;
  - the content hash is verified on **both** the cached and the fresh-cache paths;
  - a fresh-cache mismatch removes the just-populated cache directory before returning `ContentHashMismatchError`.
- When the resolved version **differs** from the baseline, the change is treated as intentional. This covers a changed constraint in `invowkmod.cue` and `module update` finding a newer version. No hash or commit expectation is applied, which fixes the false mismatch.
- A lock entry without a content hash (a v1.0 lock) still has its commit checked. Hash policy for v1.0 locks is finding F4 and stays out of scope.
- Error messages explain how to accept an intentional upstream re-tag: `invowk module remove <key>`, then add the module again. The generated lock file, marked "DO NOT EDIT MANUALLY", is never edited by hand.
- **BREAKING (behavioural):** a sync that previously passed silently now fails when the remote serves different content or a different commit for an unchanged locked version.

## Capabilities

### New Capabilities

- `locked-module-integrity`: Sync-time verification that re-resolving a locked version reproduces its locked commit and content hash, independent of local cache state. It also defines how version changes, v1.0 entries, and cache cleanup behave.

### Modified Capabilities

(none. `invowkmod-source-identity` requirements on lock keys and collisions are unchanged.)

## Impact

- **Code:** `internal/app/modulesync/`, specifically `resolver.go`, `resolver_deps.go`, `resolver_tidy.go`, `cache.go`, and error types. The `knownHashes map[ModuleRefKey]ContentHash` becomes a baseline map carrying version, commit, and hash. `pkg/invowkmod` gains a lock accessor that returns baseline entries.
- **CLI:** `cmd/invowk/module_deps.go` renders the new error as an actionable error.
- **Tests:** unit tests with the existing fake `moduleFetcher` (`newResolverWithFetcher`); a CLI testscript if a local-git fixture exists.
- **Docs:** `website/docs/modules/dependencies/lock-file.mdx` and its `pt-BR` translation, `README.md` (lock-file section), and `website/docs/security/audit.mdx` if it describes sync-time checks.
- **Formal verification:** `adopt-formal-verification` keeps F5 in `LockIntegrity.tla`. The model's current-code configuration records F5 as a counterexample, and a fixed configuration must pass. The F5 replay test becomes a regression test for this change.
