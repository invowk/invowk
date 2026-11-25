# Design

## Context

See proposal.md (Why). Current state on base `origin/main` 88f44a6e:

- **No inter-process lock.** `Resolver` has only an in-process `sync.Mutex` (`internal/app/modulesync/resolver.go:41-42`), taken by `Add` (`:129-130`), `Remove` (`:173-174`), `Update` (`:219-220`), `Sync` (`:285-286`) and `Tidy` (`resolver_tidy.go:19-21`). Each process builds its own `Resolver` (`use_cases.go:178`, `:219`, `:415`, `:437`, `:457`; `internal/app/moduleops/vendor_dependencies.go:83`), so nothing orders two processes.
- **Read-to-write windows.** `SyncModule` reads `requires` (`use_cases.go:407`) before `Resolver.Sync` reads the lock (`resolver.go:296`), fetches, and saves a lock built from the requires it read (`:313-320`). `Update` saves the lock it read before fetching (`:225`, `:273`). `AddModuleDependency` snapshots both files (`use_cases.go:191-198`) before `m.Add` fetches (`:199`), and on an edit error restores those snapshots (`:203-206`), erasing whatever another process wrote meanwhile. `TidyModule` reads requires (`:429`) and the lock (`resolver_tidy.go:29`), fetches, then writes `invowkmod.cue` (`use_cases.go:445-449`). `VendorDependencies` reads requires (`vendor_dependencies.go:75`) and the lock (`:122-127`), then removes and copies each module (`VendorModules`, `vendor.go:107-115`). These are F8 and the vendor case (`formal/README.md:262`, and the proposed-finding paragraph `:341-347`).
- **Shared worktree.** `GitFetcher.Fetch` keeps one checkout per URL-derived path (`git.go:148-175`, path from `getRepoCachePath` at `:563-574`, which maps `https://`, `git@` and `ssh://` forms of one repository to one directory). `resolveOne` fetches (`resolver_deps.go:130`), selects the module in that checkout (`:151`), and copies it to the versioned cache directory (`:170`, `cache.go:61-89`); a fresh copy trusts whatever the checkout holds at copy time. That is F9 (`formal/README.md:263`).
- **Rollback.** `readFileSnapshot` uses `os.Stat` (`use_cases.go:293`), so a dangling `invowkmod.cue` link reads as missing (`:295-296`) and `restore` then removes the link (`:315-319`). A link to a regular file is snapshot through the link (`:303`) and restored with `os.WriteFile` on the path (`:324`), which by then is the regular file `AddRequirement` or `LockFile.SaveIfChanged` renamed over the link (`fspath.AtomicWriteFile`, `pkg/fspath/atomic.go:52-99`, renames onto the path, replacing a link). `os.WriteFile` truncates before writing, so a concurrent reader can see an empty file (F10, `formal/README.md:264`).
- **Rollback reporting.** Both use cases return `errors.Join(editErr, declarationRestoreErr, lockRestoreErr)` (`use_cases.go:205`, `:246`). The CLI infers a failed restore from a top-level join with more than one part (`moduleDeclarationRestoreFailed`, `cmd/invowk/module_deps.go:315-318`), and `moduleAddDeclarationFailureLine` calls it twice (`:268-277`). After a rollback both commands return nil (`:197`, `:233`), so they exit 0. `runModuleRemove` prints `Removed <namespace>` for every removed lock entry (`:226-228`) before the rollback summary (`:231`).
- **One-line entries.** The requires editor works line by line: `findEntryBounds` records entry line ranges (`pkg/invowkmod/invowkmod_edit.go:191-213`) and `parseRequiresEntryFields` matches `git_url:` and `path:` only at the start of a trimmed line (`:218-227`), so `{git_url: "…", version: "…"},` never matches; `AddRequirement`'s duplicate check (`:36-49`) and `RemoveRequirementIfPresent` (`:116-125`) both miss it. `findRequiresBlock` and `findEntryBounds` count brackets and braces inside strings and comments too (`:162-189`, `:191-213`).
- **Lock-only remove.** `RemoveModuleDependency` builds a resolver that does not create the cache directory (`use_cases.go:219`, `resolver.go:59-61`, `:97-101`), pinned by `TestRemoveModuleDependencyDoesNotCreateModuleCache` (`use_cases_dependency_test.go:102-141`; from commit 13504c1a, "avoid cache creation for lock-only module operations"). No spec requires it.
- **Model.** `ConcurrentModuleEdits` (`formal/tla/ConcurrentModuleEdits.tla`) has base constants `AdvisoryLock = "none"`, `AtomicRestore = FALSE` (`formal/manifest.toml:1888-1894`); the fixes are `"project_and_source"` and `TRUE`, already calibrated (`:1872-1886`). The rollback reader of scenario `rollback_read` is `TidyProg` (`.tla:117-118`), which takes the project mutex whenever `AdvisoryLock /= "none"` (`.tla:137-145`, `:190-193`). The Go replays assert the open findings (`internal/app/modulesync/concurrent_edits_formal_test.go:168-368`, `internal/app/moduleops/vendor_concurrent_formal_test.go:79-140`); `raceFetchWindow` runs P2 to completion while P1 is parked (`concurrent_edits_formal_test.go:157-166`), which deadlocks once P1 holds a lock P2 needs.
- **Platform primitives.** `golang.org/x/sys` v0.50.0 is a direct dependency (`go.mod:33`) and already used for `unix.Flock` (`internal/container/run_lock_linux.go:54`); its `windows` package has `LockFileEx`, `LOCKFILE_EXCLUSIVE_LOCK` and `LOCKFILE_FAIL_IMMEDIATELY` (checked in the module cache). Releases target linux, darwin and windows only (`.goreleaser.yaml:27-39`).

## Goals / Non-Goals

**Goals:**
- One mechanism, one place per concern: modulesync decides what is locked and for how long; an OS adapter only takes and releases an exclusive file lock; the CLI only renders.
- Each formal fix lands with its model flip, its inverted replay and its `mutantPreFix…` command in one commit, so `make formal` and the replays agree at every commit.

**Non-Goals:**
- `module update` with no lock still creates an empty lock (tracker Q4; brief "Not in scope").
- Discovery items (F12, module-subdirectory hardening, discovery performance, `agent --verify`) belong to W1b.
- The `RemoveProg` note and the `WriteMod` and `LockFile.Save` rows belong to the W3 formal follow-ups; this change leaves them as they are.
- No lock for read-only consumers, no config option for the timeout, no `go.mod` change.
- `AddRequirement` keeps inserting before the line that holds the block's closing `]` (`invowkmod_edit.go:55-60`); see Open questions OQ-A.

## Decisions

### D1. A port in modulesync, a file-lock adapter in `internal/modulelock`

modulesync defines the port it needs (names indicative):

```go
// EditLocker takes exclusive OS advisory locks on lock files.
type EditLocker interface {
    // Lock tries path without blocking; while it is busy it calls onBusy once,
    // retries about every 100 ms, and gives up when ctx ends or 5 minutes pass.
    Lock(ctx context.Context, path types.FilesystemPath, onBusy func()) (unlock func() error, err error)
    // PhysicalDir returns dir's physical absolute path (links and junctions resolved).
    PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error)
}
```

`internal/modulelock` implements it with `flock(LOCK_EX|LOCK_NB)` on linux, darwin, freebsd, netbsd, openbsd and dragonfly, and `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)` on windows, through `golang.org/x/sys`. Its signatures use only the standard library and `pkg/types`, and it imports neither modulesync nor moduleops, so their internal tests can use the real adapter without an import cycle. The lock file is opened `O_RDWR|O_CREATE` with mode 0600 and never truncated or deleted; unlock releases the lock and closes the descriptor. A second `Lock` of a path the same `FileLocker` value already holds fails at once (`ErrLockHeld`) instead of waiting for itself. The poll interval (100 ms) and the timeout (5 minutes) are fields set by the constructor; the adapter's own tests shorten them through `export_test.go`. On timeout it returns `*modulelock.TimeoutError` whose message is `timed out after 5m0s waiting for lock file <path>: another invowk process holds it; let it finish or stop it, then retry`. On cancellation it returns `waiting for lock file <path>: <ctx error>`. Any other failure (open, permission, unexpected lock error) fails at once and names the path. Other platforms return `ErrLockUnsupported` from `Lock` (fail closed: no release target is affected).

`PhysicalDir` is `filepath.EvalSymlinks(filepath.Abs(dir))` on Unix. On Windows it opens the directory with `FILE_FLAG_BACKUP_SEMANTICS` and reads `GetFinalPathNameByHandle(FILE_NAME_NORMALIZED|VOLUME_NAME_DOS)`, stripping `\\?\` (and turning `\\?\UNC\` into `\\`), because `EvalSymlinks` does not follow junctions on Go 1.23+ (`formal/README.md` F18 row); this also yields the on-disk case and long names.

The composition root is `cmd/invowk`: it builds one `modulelock.FileLocker` per command run and passes it, with the wait renderer, to the use cases. Alternatives: the adapter computing lock names and printing (puts naming policy and rendering in an OS package and makes order checks untestable without real files); `modulesync` importing the adapter directly (app depends on infrastructure, no fake for tests).

### D2. What the project lock covers, command by command

modulesync owns a small per-operation helper, `editLocks` (held by the `Resolver`), exposed as the exported `(*Resolver).LockProject(ctx) (unlock func() error, err error)`, which moduleops' vendor path needs (it joins `vendorDependencyResolver`, `vendor_dependencies.go:30-33`), and the unexported `(*Resolver).lockSource(ctx, gitURL)`. `LockProject` fails with `ErrEditLockerRequired` on a resolver without a locker. The lock-taking use cases receive the locker and the wait callback as one explicit dependency value (`modulesync.EditLocks{Locker EditLocker; OnWait func(LockWait)}`) and fail with `ErrEditLockerRequired` when the locker is nil: production never runs unlocked by accident. `NewResolver` gains a variadic option (`WithEditLocks`) so existing callers compile; the test constructor `newResolverWithFetcher` gets the same option.

| Command | Lock taken | Released after |
|---|---|---|
| `module add` (`(*Resolver).AddModuleDependency`) | before `readLockSnapshot` | the declaration edit or the last restore |
| `module remove` (`(*Resolver).RemoveModuleDependency`) | before `readLockSnapshot` | the last edit or the last restore |
| `module sync` (new `(*Resolver).SyncModule`; `SyncModule` wraps it) | before `LoadRequirements` | the lock save (or the empty-requires return) |
| `module update` (new `(*Resolver).UpdateModule`) | before `m.Update` reads the lock | the lock save |
| `module tidy` (new `(*Resolver).TidyModule`) | before `LoadRequirements` | the last `AddRequirement` |
| `module vendor` (`vendorDependenciesWithResolver`) | before `LoadRequirements` (moved into it from `VendorDependencies`) | `VendorModules` returns |

So `SyncModule`, `TidyModule` and `VendorDependencies` create their resolver before reading `invowkmod.cue` (they read it first today, `use_cases.go:407-418`, `:429-440`, `vendor_dependencies.go:75-86`). The inner methods (`Add`, `Remove`, `Sync`, `Update`, `Tidy`) do not lock: they run under the caller's project lock, and vendor calls `Sync` while holding it. This matches the model's `Early`/`Final` placement (`.tla:142-145`, `:164-198`).

### D3. The source lock covers checkout to the verified, read copy

`resolveOne` takes the source lock right before `m.fetcher.Fetch` (`resolver_deps.go:130`) and releases it when `resolveOne` returns, which covers the clone or fetch and checkout, `selectSourceModule`, the hashless-entry `RemoveAll`, `cacheModule` with its verification, and `loadTransitiveDeps`' read of the cache copy (`:130-181`). Holding it to the end of `resolveOne` (rather than exactly to the verified copy) also keeps a concurrent hashless refetch from removing the copy this process is still reading; nothing inside takes another lock. `ListVersions` stays outside: it uses in-memory storage and no checkout (`git.go:110-144`). When the resolver has no locker (unit tests that build a `Resolver` directly), `resolveOne` takes no source lock; every production construction sets one (D2).

The key is the checkout path's cache-relative part: `getRepoCachePath`'s derivation moves into one function (`sourceCheckoutKey(gitURL) string`) that both `getRepoCachePath` and the source lock use, so every URL spelling that shares a checkout shares the lock.

### D4. Lock order is enforced, not only documented

`editLocks` records which locks it holds. `LockProject` fails with `errLockOrder` (no wait) while a source lock is held, and `lockSource` fails while another source lock is held. A resolver used directly with no project lock (the F9 replay, unit tests) may take a source lock: the project-first rule binds the use cases, which always lock the project first. A unit test with a recording fake locker checks the order project, then source, then source release, then project release for a sync that fetches, and a second test calls `LockProject` while a source lock is held and expects `errLockOrder`; that test fails against an implementation that takes the locks in reverse or does not check. The model's `noCircularWait` and `mutantReverseOrder` (`manifest.toml:2077-2091`) cover the protocol.

### D5. Lock file names and the `locks` directory

Per the binding decision: `<cache root>/locks/project-<hex SHA-256 of PhysicalDir(project dir)>.lock` and `<cache root>/locks/source-<hex SHA-256 of sourceCheckoutKey(url)>.lock`, where the cache root is the resolver's `cacheDir` (`resolver.go:79-95`). Before the first lock, modulesync runs `os.MkdirAll(cacheRoot, 0o755)` (the mode `NewResolver` uses, `resolver.go:97-99`) and then `os.Mkdir(locks, 0o700)`, tolerating an existing directory; it does not change the mode of an existing one. Lock files are never deleted by invowk.

Consequence: `module remove` now creates the cache root and `locks/` when they are missing. Skipping the lock when the cache root is missing is not safe: a concurrent `module update` creates the root and takes the lock, and the remove would run unlocked, which is F8's `update_remove` case. `TestRemoveModuleDependencyDoesNotCreateModuleCache` changes to assert that remove creates nothing in the cache except the `locks` directory and its project lock file (no `sources/`, no module copy). `module deps` takes no lock and keeps creating nothing (`TestListModuleDependenciesDoesNotCreateModuleCache` unchanged). See Maintainer asks MA1.

### D6. Waiting is reported once and rendered by the CLI

`editLocks` passes the adapter an `onBusy` that calls `OnWait(LockWait{Kind, LockPath, GitURL})` once. `cmd/invowk` renders it with a `//plint:render` helper to stderr: `Waiting for another invowk module command on this project (lock: <path>)` or `Waiting for another invowk module command on module source <url> (lock: <path>)`. No line is printed when the lock is free. Ctrl+C cancels the command context (`fang.WithNotifySignal(os.Interrupt)`, `cmd/invowk/root.go:101-107`), the wait returns the cancellation error before any file read, and the command exits through the normal error path. The timeout error goes through the command's existing failure line (`Failed to … module(s): <error>`) and exits 1 (`exitCodeForError`, `cmd/invowk/usage_error.go:175-186`).

### D7. Read-only consumers, and the model's reader

Discovery, command runs, `validate`, `audit` and `module deps` take no lock (binding decision 2). They read `invowkmod.cue` and the lock with plain reads and rely on every write being an atomic replace: `LockFile.SaveIfChanged` (A1, `.tla:12-19`), `AddRequirement` and `RemoveRequirementIfPresent` already are, and the restore becomes one (D8).

The check the brief asks for: the model's `rollback_read` reader is `TidyProg` (`.tla:117-118`, `:190-193`). It is not a read-only consumer: under any lock configuration it takes the project mutex (`UseProj`, `.tla:139`), and in the code `module tidy` writes `invowkmod.cue` and takes the project lock (D2). With the flipped base it would be serialised behind the rolling-back add, so `rollback_read` could no longer witness F10 and `mutantRestoreRemoveThenWrite` would stop being detected. The model therefore gets a read-only consumer: a program kind `read` (`ReadProg`: `ReadMod`, then `ReadLock`, never `Acquire`, in every configuration), which replaces `TidyProg` as `rollback_read`'s second process. `TidyProg` has no other scenario, so its program branch and correspondence row are deleted, and its `[triage] binding_gaps` entry (`manifest.toml:202`) goes with it. Tidy's lock window is covered by a Go use-case test (D2 table) instead. `ReadProg`'s row has no Go symbol (`-`, unbound by design), with the reason "read-only consumers (discovery, command runs, validate, audit) take no mutex and rely on atomic replaces".

### D8. Snapshot without following links; restore by atomic replace (item 4, F10)

`readFileSnapshot` uses `os.Lstat`. A snapshot is one of: missing; regular (bytes and permission bits); symbolic link (its `os.Readlink` target); other (directory, device: restored as today, by doing nothing). `restore`:

- missing: `os.Remove`, ignoring "not exist" (as today, `use_cases.go:315-319`);
- regular: `fspath.AtomicWriteFile(path, data, perm)`;
- link: if the path is still a link with the same target, nothing; otherwise create a link with that target under a random name in the same directory and rename it over the path (one atomic replace). A failure is a restore error (for example Windows without symlink privilege);
- other: nothing.

The restore's writer is a `Resolver` field defaulting to `fspath.AtomicWriteFile` (the seam the F10 test and the lock-restore failure test use). The F10 Go test (`TestConcurrentEdits_F10RestoreReplacesFile`) records `os.Stat` of the lock before a rollback and checks after it that the content is the snapshot's and that `os.SameFile` is false: a rename produced a new file, while an in-place `os.WriteFile` keeps the same file and fails the test on Linux, macOS and Windows. A second check reads the target through the writer seam at the moment the restore calls it and requires the pre-restore content to be whole (nothing truncated first). This is the `Restore` row's binding; the row stays model-only for the interleaving itself, because the rename has no observable intermediate state to replay.

The dangling-link test (`TestAddModuleDependencyRollbackKeepsDanglingInvowkmodLink`) fails on the base: `os.Stat` reports not-exist and the restore deletes the link. Link tests skip where `os.Symlink` is refused (Windows without developer mode), with the reason logged.

### D9. Typed rollback outcome (item 1)

`DeclarationEditResult` gains `rollback DeclarationRollback`, a string enum with `Validate()`: `none`, `complete`, `incomplete`. The use cases compute `restoreErr := errors.Join(declarationSnapshot.restore(), lockSnapshot.restore())`, set `incomplete` when it is non-nil and `complete` otherwise, and keep `Err()` as `errors.Join(editErr, restoreErr)` for display. `Validate` rejects `none` with a non-nil error and a rollback value with a nil error. `NewDeclarationEditResult` takes the outcome. The two call sites (add and remove) carry reciprocal comments naming each other, since they must build the outcome the same way. `moduleDeclarationRestoreFailed` is deleted; `moduleAddDeclarationFailureLine` and `moduleRemoveSummaryLine` switch on `Rollback()`, which also removes the double call. A use-case test forces a real lock-restore failure through the restore-writer seam (D8) and checks `incomplete` plus the restore error in `Err()`; another checks `complete` after a clean rollback (replacing `assertRestoresSucceeded`'s error-shape check).

### D10. CLI after a rollback (items 2 and 5)

`runModuleRemove` prints `Removed <namespace>` lines only when no declaration has a rollback. After a rollback (`complete` or `incomplete`) `runModuleAdd` and `runModuleRemove` print the lines they print today and return `&ExitError{Code: 1, Err: <declaration error>}`, so the process exits 1 and stderr carries the error, like the commands' other failure paths. Exit status 1 is the existing "invowk failed" class (`openspec/specs/cli-exit-codes/spec.md`, "Four exit-code classes"), so the exit-code spec needs no change.

### D11. Requires entries by byte span (item 3)

The editor stops working on lines for matching. A small scanner walks the `requires` block from the `requires:` field to its matching `]`, skipping quoted strings (with escapes) and `//` comments, and returns each top-level `{…}` entry as a byte span. Fields are read from the span: a `git_url:` or `path:` label at depth 1, outside strings and comments, followed by a string literal (parsed with the existing `strconv.QuotedPrefix` logic). Add's duplicate check and remove's match both use it.

Removal: when the entry's span starts and ends on lines that hold nothing else but whitespace and a trailing comma, the lines are removed exactly as today (so the multi-line layout `module add` writes produces byte-identical output; existing tests pin it). Otherwise the span, one following comma and the spaces up to the next token are cut from the text. The last entry still removes the whole `requires` field as today (`invowkmod_edit.go:130-148`), whether the block spans lines or one line. `findRequiresBlock` gets the same string- and comment-awareness.

### D12. Formal: three commits, base flipped step by step

| Commit | Base constants | Model edits | Manifest edits | Go replays |
|---|---|---|---|---|
| F10 | `AtomicRestore = TRUE` | `ReadProg` replaces `TidyProg` in `rollback_read` (D7); header "Findings on today's code" rewritten | `findingF10NonAtomicRollback` → `mutantPreFixF10NonAtomicRollback` (`AtomicRestore = FALSE`, `mutant_of = rollbackReadAtomicRestore`, no `finding`); `witnessPartialFile` gets `AtomicRestore = FALSE` | `TestConcurrentEdits_F10RestoreReplacesFile` |
| F8 | `AdvisoryLock = "project"` | none | the four `findingF8*` → `mutantPreFixF8*` (`AdvisoryLock = "none"`); `proposedVendorFromStaleLock` → `mutantPreFixVendorFromStaleLock` (`AdvisoryLock = "none"`, `mutant_of = vendorSyncProjectLock`); witnesses whose situation the lock makes unreachable (at least `witnessVendorOverlap`) get `AdvisoryLock = "none"` | the four F8 replays and `TestConcurrentEdits_VendorFromStaleLock` inverted |
| F9 | `AdvisoryLock = "project_and_source"` | none | `findingF9SharedWorktreeRecordsWrongContent` → `mutantPreFixF9SharedWorktreeRecordsWrongContent` (`AdvisoryLock = "none"`); `mutantNoSourceLock` stays | the F9 replay inverted |

In every commit TLC re-measures `distinct_states` and `dead_actions` of every command (for example `Acquire`/`Release` leave `rollbackReadAtomicRestore`'s dead list once the add locks, and `RestoreTruncate` joins the dead lists of commands under `AtomicRestore = TRUE`); a witness that stops being violated gets the pre-fix constants it was written for, and the calibration text gets a dated entry. The non-finding mutants (`lock_after_snapshot`, `release_between_files`, `reverse_order`, `vendor_outside_lock`, `restore_remove_then_write`) must stay counterexamples; a command whose override equals the new base drops the redundant override only if the name still reads right.

Correspondence rows (`.tla:43-64`) after the change:
- `Restore`: binding `TestConcurrentEdits_F10RestoreReplacesFile`, abstraction "atomic replace (fspath.AtomicWriteFile or link rename); the interleaving is model-only: a rename has no intermediate state"; its `binding_gaps` entry (`manifest.toml:200`) is removed.
- `TidyProg`: deleted with its gap entry (D7); `ReadProg`: new, no symbol.
- `SyncProg` → `Resolver.SyncModule`, `UpdateProg` → `Resolver.UpdateModule`, each bound to its inverted replay, which now calls the method; their gap entries (`manifest.toml:201`, `:203`) are removed.
- New `Acquire project` → `Resolver.LockProject`, `Acquire source` → `Resolver.lockSource`, `Release` → `heldLock.unlock` (`internal/modulelock`; the `unlock` closure `Lock` returns calls it), bound to the inverted sync replay, the F9 replay, and both.
- `AddProg (add_add_same)`, `VendorProg`, `Checkout, CopyToCache`: line numbers and notes updated to the locked code.
- Untouched: `RemoveProg`, `WriteMod add`, `WriteMod remove`, `WriteLock` (W3).
No new `binding_gaps` or `unresolved_elements` entry: both lists only shrink.

`formal/README.md`: F8, F9 and F10 rows read "Fixed (module-concurrency-rollback): …; the pre-fix code is kept as `ConcurrentModuleEdits.mutantPreFix…`" with their replays; the stale sentence "F8–F12 are owned by sibling changes…" (`:275`) and the proposed-finding paragraph (`:341-347`) are rewritten to say the vendor case is fixed by the project lock. The `ConcurrentModuleEdits` models-table row (`:176`) says "fix configurations are the base".

The fix stage runs `formal_mutation.py symbols` on every changed bound or row-named function, ONE report-only `mutation-formal` run and one targeted refresh (brief "Formal").

### D13. Inverted replays: the waiting process runs in its own goroutine

`raceFetchWindow` becomes: start P1 (parked inside its fetch window, holding its lock), start P2 in a goroutine, wait until P2's `OnWait` fires (it is blocked on the real lock file), check that P2 has not written either file, open P1's gate, and join both. Every process gets its own `Resolver`, fetcher and `modulelock.FileLocker` on one shared temporary cache root, so the exclusion is the OS lock on a real file. Assertions: `sync_add` → requires and lock both `{a, b}`; `update_add` → lock `{a, b}`; `update_remove` → requires `{}` and lock `{}`; `add_add_same` → both `{a, b}` and the second add's outcome `complete` with `ErrModuleAlreadyExists`; `fetch_v1_v2` → P1's entry hash equals the 1.0.0 reference; vendor → P2's lock write (taken through `LockProject` of a second resolver, then the transcribed write, `vendor_concurrent_formal_test.go:86-90`) happens only after the vendor returned, and the vendored copy matches the lock the vendor read. No sleeps: ordering comes from the gate, the wait callback and channels. A further adapter test re-executes the test binary as a helper process that holds a lock, checks that the parent waits, and kills the helper to show the lock is released on process death (Linux, macOS, Windows).

### D14. Commits

One commit per item, in this order so each builds on the last: Lstat snapshot (item 4); atomic restore with the F10 model flip (item 6); typed outcome (item 1); `Removed` lines (item 2); exit status (item 5); one-line entries (item 3); project lock with the F8 flip (item 7); source lock with the F9 flip (item 8). Docs go with the item that changes the documented behaviour. The spec artifacts are this stage's commit.

## Risks / Trade-offs

- [Different cache roots] Two processes with different `INVOWK_MODULES_PATH` use different `locks` directories and do not exclude each other on a shared project. → Documented; the module cache is per user, and pointing two runs at different caches already gives them different module copies.
- [Case-insensitive volumes on macOS] `EvalSymlinks` keeps the case it is given, and `os.Getwd` can return `$PWD`'s spelling, so `cd ~/Proj` and `cd ~/proj` give two project lock names on APFS. → Windows is normalised by `GetFinalPathNameByHandle`; macOS residual recorded as MA2.
- [Network file systems] `flock` on NFS may be local-only or emulated. → Documented as an advisory lock between invowk processes on one machine.
- [Remove creates the cache root] See D5 and MA1.
- [Longer runs] A command now waits for another on the same project, up to 5 minutes. → The waiting line says why; Ctrl+C ends it.
- [Link restore on Windows] Recreating a link can fail without symlink privilege. → Reported as an incomplete rollback with the error (exit 1), never silently skipped.
- [Shared formal files with W2] W2 also edits `formal/README.md` rows and refreshes the formal-bindings ledger. → W1a merges first; W2 rebases and redoes its refresh (brief "Overlap").
- [Exit-status change] Scripts that ignored a rolled-back add now fail. → Called out as BREAKING with migration notes; the run did not change the project.

## Migration Plan

No data migration. The `locks` directory appears on first use. Rolling back this change leaves empty lock files in the cache, which nothing else reads.

## Maintainer decisions

- 2026-11-25 (decisions 110, 111, 113, 125, 127; brief `program-notes/briefs/w1a-module-concurrency-brief.md`, binding): scope is items 1-8 of the brief; `module add` and `module remove` exit non-zero after a rollback; F8, F9 and F10 are fixed here, the project mutex for add, remove, sync, update, tidy and vendor, the source mutex after it, lock files under `<module cache>/locks/` with the names in D5, flock or `LockFileEx` through `golang.org/x/sys` with no `go.mod` change, waiting lines, 100 ms retry and a 5-minute limit with no config option, read-only consumers unlocked, three formal commits with inverted replays and `mutantPreFix…` commands, one report-only `mutation-formal` run and one targeted refresh. Q4 and the discovery items stay out; W3 owns the `RemoveProg` note and the `WriteMod`/`LockFile.Save` rows.
- S1, 2026-11-25 (this stage, default pending spec review): the model's reader becomes `ReadProg` and `TidyProg` is deleted (D7); `SyncProg` and `UpdateProg` are re-pointed to the new methods and bound (D12); the source lock is held to the end of `resolveOne` (D3); unsupported platforms fail closed (D1); `remove` creates `locks/` (D5, MA1).

## Maintainer asks

- MA1. `module remove` now creates the module cache root and `locks/` when they are missing (D5), which narrows the intent of commit 13504c1a ("avoid cache creation for lock-only module operations"). Recommended: accept; the alternative leaves remove unlocked against a concurrent update. Non-blocking.
- MA2. On macOS case-insensitive volumes, two spellings of one project directory give two project locks (Risks). Recommended: accept now; a follow-up can key the project lock on the directory's file identity (device and inode, or volume serial and file index), which would replace the binding decision's path hash. Non-blocking.

## Open questions

- OQ-A. `AddRequirement` inserts the new entry before the line holding the block's closing `]` (`invowkmod_edit.go:55-60`); for a `requires` block written on one line that line is also the `requires:` line, so the entry lands before the field and the file stops being valid CUE. It is pre-existing and outside item 3 (matching); with D11's spans the fix is small (insert at the `]` offset). For the tracker.
- OQ-B. `AddRequirement` and `RemoveRequirementIfPresent` rename over a symbolic `invowkmod.cue`, replacing the link with a regular file on every successful edit (`pkg/fspath/atomic.go:86`). The rollback now restores links (D8), but a successful edit still breaks them. For the tracker.
