# Design

## Context

See proposal.md (Why). Current behavior:

- `LockFile.Save` validates, renders and always writes: `pkg/invowkmod/lockfile.go:360-371` (`fspath.AtomicWriteFile` at :370). The renderer writes `generated:` as `l.Generated.Format(time.RFC3339)` (`pkg/invowkmod/lockfile.go:432`), so the value has second resolution. Module keys are already sorted for deterministic output (`pkg/invowkmod/lockfile.go:439-448`).
- `NewLockFile` stamps `Generated: time.Now()` (`pkg/invowkmod/lockfile.go:271-277`); `LoadLockFile` returns the parsed `Generated` for an existing file and `NewLockFile()` for a missing one (`pkg/invowkmod/lockfile.go:344-355`). The parser reads `generated` with `time.Parse(time.RFC3339, ...)` (`pkg/invowkmod/lockfile_parser.go:117`).
- Callers of `Save`, all in `internal/app/modulesync/resolver.go`:

  | Caller | Lock it saves | `generated:` today |
  |--------|---------------|--------------------|
  | `Resolver.Add` (:126-163, save :158) | loaded lock plus the new entry | kept from the loaded file |
  | `Resolver.Remove` (:167-206, save :200) | loaded lock minus matched keys | kept from the loaded file |
  | `Resolver.Update` (:210-268, save :264) | loaded lock with re-resolved entries; saves even with zero keys | kept from the loaded file |
  | `Resolver.Sync` (:272-312, `NewLockFile` :300, save :307) | fresh lock from resolution | `time.Now()` on every run (the defect) |

- Other lock writers, unchanged by this design: `Resolver.Tidy` does not write the lock (`internal/app/modulesync/resolver_tidy.go:13-19`; a later sync does), and the add/remove rollback `fileSnapshot.restore` writes the exact prior bytes with `os.WriteFile` (`internal/app/modulesync/use_cases.go:245-262`, called at :140 and :181).
- `TestSyncFreshCacheReproducesLockedHash` (`internal/app/modulesync/resolver_integrity_test.go:108-124`) asserts byte equality across two syncs and fails whenever they straddle a second.
- Formal: `LockFile.Save` is the `caller` row of `formal/tla/AtomicWrite.tla:27` and the `WriteLock` row of `formal/tla/ConcurrentModuleEdits.tla:57` (action at :336-338, assumption A1 at :12), and a `binding_gaps` entry (`formal/manifest.toml:206`), so it has no mutation-ledger survivors. `Resolver.Sync` has formal-bindings ledger entries (`tools/mutation/triage/formal-bindings.toml:1453-1645`).

## Goals / Non-Goals

**Goals:**
- One fix in the writer that every `Save` caller inherits.
- Deterministic tests with no sleeps and no injected clock in the resolver.

**Non-Goals:**
- Changing the lock format, schema, parser or the `generated:` resolution.
- Changing the rollback writer, Tidy, or vendoring.
- Changing CLI messages (see Open questions).
- Whether `module update` with no lock file should create an empty one (it does today, `internal/app/modulesync/resolver.go:215-219`, `:264`; covered by `tests/cli/testdata/module_sync_update.txtar` test 2). Out of scope.

## Decisions

### D1. Skip decision in `LockFile.Save`, by semantic comparison

`Save` (after `validateForSave`) inspects the existing file with the same bounded reader the loaders use (`InspectLockFile`, size limit and CUE parse). If the file is present and parsed, and its `Version` and `Modules` equal the lock being saved, `Save` returns nil without writing and sets `l.Generated` to the on-disk value, so the in-memory lock matches the file. Otherwise it sets `l.Generated` to the current time and writes as today.

Equality: same `Version`, same key set, and for each key an equal `LockedModule` (every field is a string-backed value type, so struct `==` per entry, or `maps.Equal`, suffices; a test pins that a change to any single field counts as different).

Alternatives:
- Byte comparison of the rendered lock (with the on-disk `generated:`) against the file. Rejected: an equivalent but non-canonical file (hand formatting, `+00:00` instead of `Z`, an older header comment) would be rewritten, which contradicts "byte-identical when the module set and content are unchanged".
- Fix in `Resolver.Sync` only (reuse the loaded `generated:` when entries match). Rejected: Add/Remove/Update would still rewrite on canonicalization and would never advance `generated:`; and it rewrites a body bound by 12 formal-bindings ledger entries.
- Compare in each caller. Rejected: four copies of the same logic.

### D2. `Save` owns the timestamp

Stamping moves from "whoever built the lock" to the write itself: a written lock always carries the write time. This is what makes `generated:` advance on Add/Remove/Update content changes (proposal, What Changes). `NewLockFile` keeps stamping `time.Now()` so an unsaved lock still has a sensible value; tests that assert it (`pkg/invowkmod/lockfile_test.go:37`) stay valid.

Clock: `Save` reads `time.Now()` directly; no clock is threaded through `Resolver`. Deterministic tests do not need one (D3). Tests that pin an exact `generated:` in the written file (`pkg/invowkmod/lockfile_test.go:349-400`, `lockfile_cue_test.go`) must be checked: any test that sets `Generated` on a new lock and expects that exact value in the file now sees the write time. Those tests are adjusted to assert "later than the old value" or to call the renderer directly; the apply stage lists each one it touches.

Alternative: an unexported `saveAt(path, now)` seam used by in-package tests. Allowed if a `pkg/invowkmod` test needs an exact written time; it adds no public API.

### D3. Deterministic tests by backdating the on-disk lock

- Pre-fix calibration for `TestSyncFreshCacheReproducesLockedHash`: after the first sync, rewrite the lock file with `generated:` set to a fixed past instant (for example `2001-02-03T04:05:06Z`), keeping everything else, and compare the second sync's output against those backdated bytes. Pre-fix, Sync restamps with now and the bytes differ every time; post-fix, the file is untouched. No sleep.
- New test, content change advances the time: backdate the lock, then change content (for example `Update` to a newly listed version via the fake fetcher, or `Add` a second module), and assert the written `generated:` parses later than the backdated instant.
- `pkg/invowkmod` unit tests for `Save`: equivalent non-canonical file stays byte-identical; each single-field change in one entry is written; version change is written; missing file is created; unparseable file is replaced.
- CLI testscript `tests/cli/testdata/module_update_lock_unchanged.txtar`: seed an `invowkmod.cue` and a valid but non-canonical empty lock (`modules: {}`, an old `generated:` written with `+00:00`, different spacing), keep a copy, run `invowk module update`, then `cmp` the lock with the copy. `Update` reaches `Save` with zero keys (`internal/app/modulesync/resolver.go:225-264`) without network access. Pre-fix, `Save` rewrites the file in canonical form, so `cmp` fails; post-fix it passes. It is a built-in command test, so it needs no virtual/native mirror.

### D4. Formal correspondence

Read-compare-skip refines assumption A1 ("every `fspath.AtomicWriteFile` call is an atomic replace"): when the on-disk lock equals the lock being saved, skipping the write leaves the same abstract state as atomically replacing it with identical content at the instant of the read. When they differ, `Save` performs the same atomic replace as today. The models track lock keys and entries, not bytes or timestamps, so the extra read adds no observable step. The F8 replay `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` still reproduces: Sync's lock lacks the concurrently added key, so it differs from the file and is written. The apply stage adds one-line notes to the two table rows, runs `python3 scripts/formal.py correspondence` and `make formal-reach`, and runs `make test-scripts` because `formal/` changes. `LockFile.Save` is a `binding_gaps` entry, so no mutation-ledger refresh is needed for it; `Resolver.Sync`'s body is not changed, so its ledger entries stay valid.

## Risks / Trade-offs

- [The extra read on every save costs one bounded stat, read and CUE parse.] → The lock is small (size-capped by `LockFileSizeLimit`); module commands already parse it.
- [Read-compare-write is not atomic against a concurrent writer.] → Same exposure as today's unlocked writes (F8 in `formal/README.md`); a skip is equivalent to an earlier identical replace (D4), so no new interleaving outcome exists.
- [A hand-edited but equivalent lock is never canonicalized.] → Intended: the spec forbids rewriting unchanged content. The next real content change writes the canonical form.
- [Tests that expect a pinned `generated:` in a freshly written file break.] → D2: adjust them in the same group and list them.

## Migration Plan

None. Existing lock files are read unchanged; the first save that changes content restamps them.

## Open questions

- `cmd/invowk/module_deps.go:284` (sync) and `:364` (update) print "Lock file updated" unconditionally, which is untrue after a skipped write. Changing it needs `Save` to report whether it wrote, a public API change. Deferred to the maintainer; it does not change the specs or the tasks.
