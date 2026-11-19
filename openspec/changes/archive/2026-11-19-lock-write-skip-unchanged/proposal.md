# Proposal

## Why

`invowk module sync` rewrites `invowkmod.lock.cue` even when no module changed: the lock is restamped with the current time and written in RFC 3339 at second resolution, so any sync that crosses a second boundary dirties the file in Git. The same defect makes `TestSyncFreshCacheReproducesLockedHash` fail whenever its two syncs straddle a second (tracker N18, reproduced 3 of 3 with a 1.1 s sleep between the syncs on 2026-11-19).

## What Changes

- Saving the lock file compares the new lock with the lock already on disk by meaning (format version plus every module entry), not by bytes, and writes nothing when they are equal. The file stays byte-identical, `generated:` included, even when its existing formatting is not what the writer would produce.
- When the meaning differs, or no valid lock exists, the writer stamps `generated:` with the current time and writes the file as today (atomic replace).
- The fix lives in the lock writer, so every path that persists the lock gets it: `module sync`, `module add`, `module remove` and `module update`. `module tidy` persists the lock only through a later sync, and the rollback of a failed add/remove restores exact prior bytes; neither changes.
- User-visible change: today `module add`, `module remove` and `module update` keep the `generated:` value they loaded from the existing lock (`pkg/invowkmod/lockfile.go:271-277` stamps only a new lock; `internal/app/modulesync/resolver.go:147-158`, `:172-200`, `:215-264` load, mutate and save). After this change, `generated:` advances whenever any of these commands changes the lock's content, and never when the content is unchanged. Only `module sync` restamped before, and it did so on every run.
- Tests: the fresh-cache integrity test becomes deterministic (the on-disk `generated:` is backdated between syncs, no sleeps), a new test proves `generated:` advances on a content change, and a network-free CLI testscript proves `invowk module update` leaves an equivalent lock untouched.

Not breaking: the lock file format, its schema and its parser do not change.

## Capabilities

### New Capabilities
- `module-lock-persistence`: when and how Invowk writes `invowkmod.lock.cue`: no write when the module set and content are unchanged, and a fresh `generated:` timestamp only when the content changes.

### Modified Capabilities
None. No main spec under `openspec/specs/` covers lock file persistence. The in-flight change `verify-locked-module-integrity` (capability `locked-module-integrity`, not yet archived) states when the lock SHALL NOT be written after an integrity failure; this change does not alter those requirements.

## Impact

- Code: `pkg/invowkmod/lockfile.go` (`LockFile.Save`, plus a semantic-equality helper). The four `Save` callers in `internal/app/modulesync/resolver.go` keep their call; `Resolver.Sync`'s body is not changed.
- Tests: `internal/app/modulesync/resolver_integrity_test.go`, `pkg/invowkmod/lockfile_test.go` (or a sibling test file), and a new `tests/cli/testdata/module_update_lock_unchanged.txtar`.
- Formal: `LockFile.Save` is named in the correspondence tables of `formal/tla/AtomicWrite.tla:27` and `formal/tla/ConcurrentModuleEdits.tla:57` and is a `binding_gaps` entry in `formal/manifest.toml:206`. The tables' notes need to state that a skipped write is equivalent to an atomic replace with identical content; no model changes.
- No dependency, schema or CLI flag changes. The CLI's "Lock file updated" message after sync and update (`cmd/invowk/module_deps.go:284`, `:364`) is left as is (see design, Open questions).
