# Design

## Context

See proposal.md (Why). Current state:

- `LockFile.Save` (`pkg/invowkmod/lockfile.go:371-390`) validates, re-reads the file with `InspectLockFile`, and returns nil without writing when the on-disk lock has equal content (`:376-380`); otherwise it stamps `Generated` and writes atomically (`:381-389`). Its caller cannot tell the two outcomes apart.
- The resolver saves the lock in four places, each as `if err := lock.Save(lockPath); err != nil` (`internal/app/modulesync/resolver.go:158`, `:200`, `:264`, `:307`), inside `Resolver.Add` (`:126`), `Remove` (`:167`), `Update` (`:210`) and `Sync` (`:272`). `Sync` returns early without saving when there are no requirements (`:276-278`); `Update` with no lock entries still saves (`:230-264`), which creates an empty lock when none exists.
- Services: `AddModuleDependency` / `Resolver.AddModuleDependency` (`internal/app/modulesync/use_cases.go:109`, `:122`) and `RemoveModuleDependency` (`:153`, `:163`) already return typed results (`AddModuleDependencyResult`, `RemoveModuleDependencyResult`, `:33-45`); `SyncModule` returns a tuple (`:307`) and `UpdateModule` a slice (`:357`). On an `invowkmod.cue` edit failure both add and remove restore the lock snapshot (`:139-141`, `:179-180`).
- CLI: add prints `Module lock file updated` on a successful declaration edit (`cmd/invowk/module_deps.go:189-200`); remove prints `Lock file and invowkmod.cue updated` or `Lock file updated; invowkmod.cue needs manual review` (`:236-240`); sync and update print `Lock file updated: invowkmod.lock.cue` (`:284`, `:364`); update with no module prints only `No modules to update` and returns (`:351-354`).
- `module vendor` calls `Resolver.Sync` through the `vendorDependencyResolver` interface (`internal/app/moduleops/vendor_dependencies.go:30-31`, `:117`, `:131`) with a fake in `internal/app/moduleops/vendor_test.go:27`. Its output names no lock write (`cmd/invowk/module_package.go:200-207`).
- The resolver tests already drive every save path network-free through a fake fetcher: `TestResolverContentChangeAdvancesGenerated` and `TestResolverUnchangedContentKeepsLockBytes` (`internal/app/modulesync/resolver_lock_generated_test.go:19`, `:87`). The writer's own cases are in `pkg/invowkmod/lockfile_save_test.go:65-222`.
- A CLI `module sync` with requirements always contacts the remote: `GitFetcher.ListVersions` lists remote refs (`internal/app/modulesync/git.go:110-125`), and only `https://`, `git@` and `ssh://` URLs are accepted (`internal/app/modulesync/resolver_deps.go:17-18`). No testscript can run it offline (the only offline sync testscript has no requirements: `tests/cli/testdata/module_sync_happy.txtar`). `module update` with an empty lock is offline (`tests/cli/testdata/module_update_lock_unchanged.txtar`, `module_sync_update.txtar` Test 2).
- Formal: rows naming `LockFile.Save` are `formal/tla/AtomicWrite.tla:27` (element `caller`, no binding, so a `[triage] binding_gaps` entry at `formal/manifest.toml:200`) and `formal/tla/ConcurrentModuleEdits.tla:60` (`WriteLock`). The resolver and service rows are `ConcurrentModuleEdits.tla:43-54` (with line references such as `resolver.go:136`, `149-158`, `173`, `200`, `300-307`, `216, 264` and the step comments at `:162-189`) and `LockIntegrity.tla:32`. `binding_gaps` only shrinks: `triage --check` fails on any entry the base does not list (`scripts/formal_mutation.py:925-926`, `:1026-1028`).
- goplint: `LockFile.Save`'s string path is excepted by pattern `invowkmod.LockFile.Save.*` (`.goplint/exceptions.toml:1748-1750`).

## Goals / Non-Goals

**Goals:**
- One source of truth for "written or skipped": the lock writer. Services carry it; the CLI renders it.
- Bound function bodies change only to return the flag (brief decision 6).

**Non-Goals:**
- No change to when the lock is written (spec "Unchanged lock is not rewritten" and its siblings stay as they are).
- No change to `module vendor`, `module tidy` or their output; vendor ignores the flag.
- No change to the add/remove failure-branch messages, or to how `invowkmod.cue` edits are reported (see Open questions Q1, Q2).
- No model change: a skipped write already equals an atomic replace with identical content in the models.

## Decisions

### D1. `SaveIfChanged` holds the body; `Save` wraps it

`func (l *LockFile) SaveIfChanged(path string) (written bool, err error)` takes over `Save`'s current body: validation error, mkdir error or `AtomicWriteFile` error return `false` with the error; the equal-content skip returns `false, nil` (still setting `l.Generated` from the file); a completed write returns `true, nil`. `Save` becomes `_, err := l.SaveIfChanged(path); return err`, keeping its signature and doc comment (pointing at `SaveIfChanged`).

Alternatives: changing `Save`'s signature (breaks the public `pkg/invowkmod` API, brief decision 1 rules it out); a "last save wrote" field on `LockFile` (hidden mutable state); comparing timestamps or re-reading the file in the CLI (ruled out by the brief). The parameter stays `string` to mirror `Save`; if goplint does not treat `invowkmod.LockFile.Save.*` as covering `SaveIfChanged`, add a sibling exception with the same reason ("filepath boundary") rather than diverge from `Save`.

### D2. Resolver methods return the flag as an extra result

`Resolver.Add`, `Remove`, `Update` and `Sync` gain a `bool` result before the error, named `lockWritten` in each signature, taken from `SaveIfChanged`. Every early return passes `false`. `Sync`'s no-requirements return is `nil, false, nil`. No other statement in these bodies changes: no reordering, no new branches.

Alternatives: a typed result struct per resolver method (more churn in four bound bodies and in the formal replay tests, no extra information); a resolver field recording the last write (mutable state shared across calls on a mutex-guarded resolver). The `vendorDependencyResolver` interface and its fake follow the new `Sync` signature; `resolveVendorDependencies` discards the flag.

### D3. Typed service results carry it

- `AddModuleDependencyResult` and `RemoveModuleDependencyResult` gain a `lockWritten bool` field, a `LockWritten() bool` accessor, and a `lockWritten` parameter in `NewAddModuleDependencyResult` / `NewRemoveModuleDependencyResult` (placed after the resolved/removed argument). `LockWritten` reports what the resolver's save did, even when a later declaration edit failed and the lock was rolled back; the CLI reads it only on the success branches (D4).
- `SyncModule` returns a new `SyncModuleResult` (`Requirements()`, `Resolved()`, `LockWritten()`) and `UpdateModule` a new `UpdateModuleResult` (`Updated()`, `LockWritten()`), each with a `New…` constructor and a `Validate()` that validates the resolved modules, following `AddModuleDependencyResult` (`use_cases.go:58-64`, `:275-286`). `SyncModule` with no requirements returns `LockWritten() == false`.

Alternative: adding a fourth tuple element to `SyncModule`. Rejected: the services-return-typed-results rule (`.agents/rules/general-rules.md`, Code) and a tuple of three slices and a bool is hard to read at the call site.

### D4. CLI wording and rendering

Pure helpers in `cmd/invowk/module_deps.go` return the text (unit-testable without stdout capture); the run functions print it:

| Command | Written | Skipped |
|---|---|---|
| sync, update | `✓ Lock file updated: invowkmod.lock.cue` | `✓ Lock file unchanged: invowkmod.lock.cue` |
| add (declaration edit succeeded) | `✓ Module lock file updated` | `✓ Module lock file unchanged` |
| remove (all declaration edits succeeded) | `✓ Lock file and invowkmod.cue updated` | `✓ Lock file unchanged; invowkmod.cue updated` |

The written lines are today's text. Both outcomes use the success icon: a skip is a successful command. `module update` with no module to update prints `No modules to update` and then the lock line, since it still saved (and may have created) the lock. Failure branches of add and remove print what they print today. `invowkmod.lock.cue` comes from `invowkmod.LockFileName`, as now.

Alternatives: the info icon for "unchanged" (reads like a warning next to the module list); leaving the no-module update silent (it would keep hiding the creation of an empty lock and leave the "unchanged" line with no offline CLI test).

### D5. Tests

- `pkg/invowkmod/lockfile_save_test.go`: `SaveIfChanged` reports `false` for the existing skip cases (`:65`, `:102`), `true` for the write cases (`:118-222`), and `false` with the error on a failed save (for example the invalid-lock validation path); `Save` still returns nil on the skip.
- `internal/app/modulesync/resolver_lock_generated_test.go`: the two table tests also assert `lockWritten` (`true` in the content-change table for update, add, remove, sync; `false` in the unchanged table for add, update all, update one, sync). A new test syncs twice from an empty directory and asserts `true`, then `false`, with byte-identical lock bytes. Remove's `false` is reachable only when the lock on disk already lacks the entries at `SaveIfChanged`'s re-read (a concurrent edit); it is covered at the `SaveIfChanged` level, not through `Remove`.
- `internal/app/modulesync` service tests: `Resolver.AddModuleDependency` with a lock that already holds the module but an `invowkmod.cue` without the `requires` entry returns `LockWritten() == false` and `Declaration().Updated() == true`; an add of a new module returns `true`. Remove returns `true`.
- `cmd/invowk`: a table test of the D4 helpers covering every row and the no-module update.
- Testscripts (offline): `module_update_lock_unchanged.txtar` also asserts `Lock file unchanged: invowkmod.lock.cue` and no `Lock file updated`; `module_sync_update.txtar` Test 2 runs `module update` with no lock twice, asserting `Lock file updated` and then `Lock file unchanged` with a `cmp` of the lock between the runs. `module_remove_happy.txtar` keeps asserting the updated line. The brief asked for a CLI testscript running `module sync` twice; that cannot run offline (Context), so the sync-twice behavior is covered by the resolver test above and the CLI rendering by the helper test (maintainer decision M1 below records the substitution).
- All callers of the changed signatures are updated, including the formal replay and trace tests in `internal/app/modulesync` (`lock_trace_ops_test.go`, `concurrent_edits_formal_test.go`, `resolver_integrity_test.go`) and the benchmark in `resolver_bench_test.go`; their assertions do not change.

### D6. Formal correspondence

- Keep both `LockFile.Save` rows (`AtomicWrite.tla:27`, `ConcurrentModuleEdits.tla:60`) on `LockFile.Save`, and extend their notes: `Save` delegates to `SaveIfChanged`, which holds the skip-or-replace body and which the resolver calls; the written flag is reporting only. Renaming the `AtomicWrite` `caller` row to `LockFile.SaveIfChanged` would turn its listed binding gap into a new, unlisted one, which `triage --check` refuses (`scripts/formal_mutation.py:925-926`, `:1026-1028`); binding it instead would add planned mutants and a model-level binding decision outside this change (Open question Q3).
- Update the line references in the `ConcurrentModuleEdits.tla` rows and step comments (`:43-54`, `:162-189`) and in `LockIntegrity.tla:32` if the bodies move; the symbols keep their names. No model logic, constants or goldens change.
- `Resolver.Add`, `Remove`, `Update` and `Sync` are rewritten bound bodies (their returns change). The fix stage runs `formal_mutation.py symbols` with them plus `LockFile.Save`, expects `-> refresh` (`LockFile.Save` labelled `unplanned`), runs `make mutation-formal` once, report-only, in a throwaway worktree, and refreshes those symbols' ledger entries, per `.agents/runbooks/change-flow.md` (Formal lane).

## Risks / Trade-offs

- [Renamed body, unrenamed rows] The `LockFile.Save` rows name the wrapper, not the function that now calls `AtomicWriteFile`. → The row notes say so explicitly; Q3 asks whether to rename and bind later.
- [goplint on the new exported method] A new finding for `SaveIfChanged(path string)` would fail `make check-baseline`. → D1: add a sibling exception with the existing reason; no inline ignore.
- [Signature churn in bound tests] Changing four resolver signatures touches formal replay tests. → Only call sites change (`x, _, err :=`), never assertions; `python3 scripts/formal.py correspondence` and `make formal-reach` prove the rows still resolve and reach.
- [Mutation run cost] Any rewritten bound body plans every formal-bindings mutant (about 35 minutes locally). → Accepted by the brief; run once, report-only, in the background with an `rc=` log.
- [Shared ledger with the F22 run] The sibling virtual-runtime run in `invowk-f22` is also formal-bound and may refresh the same ledger and baseline files. → Whichever merges second rebases and re-runs `triage --check`; reported to the maintainer.
- [Output change for scripts] A script grepping `Lock file updated` after a no-op run sees `Lock file unchanged`. → Intended: the old line was false. Exit codes are unchanged.

## Migration Plan

None. No file format, flag or exit code changes. Rollback is a revert of the PR.

## Maintainer decisions

- 2026-11-24 (brief n22, maintainer decision 47; binding): additive reporting API with `Save` wrapping it; resolver operations return the fact in typed results; the CLI prints "unchanged" on a skip and the existing "updated" line on a write for sync, update, add and remove, reporting each file truthfully; spec delta against `module-lock-persistence`; unit tests for written and skipped per operation; minimal change to bound bodies, accept the mutation run.
- M1, 2026-11-24 (this stage, default pending spec review): the brief's "testscript that runs `module sync` twice" is replaced by an offline `module update` twice testscript plus the resolver-level sync-twice test, because a `module sync` with requirements needs a reachable Git remote (Context). If the maintainer wants a CLI sync-twice test, it needs a test seam for the fetcher, which is a separate change.

## Implementation notes (implement stage, 2026-11-24)

- D2: the resolver methods name every result, with names that no body statement declares (`added`, `removed`, `synced`, `updatedModules`, `addErr`, `removeErr`, `syncErr`, `updateErr`, plus `lockWritten`), and keep explicit returns. gocritic's `unnamedResult` requires names for the three results, and reusing `resolved`, `updated` or `err` as result names would turn the bodies' `:=` statements into redeclarations or shadows. The save assigns the named `lockWritten` with a new `saveErr` (`lockWritten, saveErr := lock.SaveIfChanged(lockPath)`), so `Update`'s loop-local `err` shadows nothing used after the loop.
- D3: `SyncModule` now returns `SyncModuleResult{}` with the resolver's error, instead of the requirements with a nil module list; the CLI never read the requirements on that path.
- D6: `LockIntegrity.tla:32` names `Resolver.Sync` without line references, so it needed no edit. The `vendor_concurrent_formal_test.go` comment that cites `Resolver.Update`'s lines was updated with the `ConcurrentModuleEdits.tla` references.
- The vendor resolver interface carries no comment for the new flag, so the `vendor_dependencies.go` line references in `ConcurrentModuleEdits.tla` stay valid.

## Open questions

- Q1. `module remove` prints `Lock file updated; invowkmod.cue needs manual review` after an `invowkmod.cue` edit failure, although the use case has restored the lock snapshot (`use_cases.go:179-180`). This change keeps that line (Non-Goals). Fixing it is a separate wording change.
- Q2. `module remove` reports `invowkmod.cue updated` even when `RemoveRequirement` matched no entry and wrote nothing (`pkg/invowkmod/invowkmod_edit.go:76-79`, `:110-112`). Reporting it truthfully needs `RemoveRequirement` to say whether it wrote: a follow-up with the same shape as D1.
- Q3. Whether to rename the two `LockFile.Save` rows to `LockFile.SaveIfChanged` and bind the `AtomicWrite` `caller` row with a killer test, closing its `binding_gaps` entry. That is a binding decision with its own mutation triage; it does not change this change's behavior or tasks.
- Q4. `module update` with no lock file creates an empty lock (`resolver.go:230-264`, Context). This change only makes that visible ("Lock file updated"); whether update should create it at all is a separate question.

## Fix-stage notes (2026-11-24)

- Applied: a `SaveIfChanged` test that reaches the `AtomicWriteFile` error branch (a directory at the lock path), with the MkdirAll test renamed to say what it covers; README's lock-file sentence names each command's line; the `module remove` note (English and pt-BR) says the unchanged lock happens only after a concurrent removal; the TLA prose (ConcurrentModuleEdits A1 and the WriteLock action comment, the AtomicWrite header) names `LockFile.SaveIfChanged`, which `Save` wraps. Comment-only; no model logic.
- Rejected: the claim that `make formal-reach` fails for the `WriteLock` row (`ConcurrentModuleEdits` `WriteLock` is `characterisation-only`, never a killer; `make formal-reach` passed); post-save `Validate()` in `NewSyncModuleResult` / `NewUpdateModuleResult` (unreachable: the resolver validates every requirement and builds every resolved module; same pattern as `NewAddModuleDependencyResult`); `LockWritten()` after rollback (D3, documented, read only on success branches); the remove failure-branch wording and `os.IsNotExist` on a wrapped error in `module add` (pre-existing, outside the brief: Q1 and a new follow-up).
