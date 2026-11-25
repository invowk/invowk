# Design

## Context

See proposal.md (Why). Current state on base `e2011627`:

- `module add` (`cmd/invowk/module_deps.go:156-203`): after a successful resolve it reads `result.Declaration()` (`:188`). On a declaration error it tests `os.IsNotExist(declaration.Err())` (`:190`) to choose between `invowkmod.cue not found — dependency was not persisted` (`:192`) and `Could not auto-edit invowkmod.cue; lock file changes were rolled back: %v` (`:195`). On success it prints `moduleAddLockLine` (`:198`, `:261-266`) and `Updated invowkmod.cue with new requires entry` (`:199`).
- That error comes from `Resolver.AddModuleDependency` (`internal/app/modulesync/use_cases.go:187-212`): `invowkmod.AddRequirement` wraps the read error with `%w` (`pkg/invowkmod/invowkmod_edit.go:29-32`), and the use case joins it with both restore errors (`use_cases.go:203-206`). `errors.Join` keeps a wrapper even when the restore errors are nil, and `os.IsNotExist` only looks through `*PathError`, `*LinkError` and `*SyscallError`, so the not-found branch is never taken. `errors.Is(err, os.ErrNotExist)` already holds for the bare `AddRequirement` error (`pkg/invowkmod/invowkmod_edit_mutation_test.go:20-23`). With no `invowkmod.cue` the declaration snapshot records `exists=false` (`use_cases.go:292-297`), so the rollback removes nothing and restores the lock (`:310-327`): "dependency was not persisted" is the truth.
- `module remove` (`cmd/invowk/module_deps.go:205-243`): prints `Could not auto-edit invowkmod.cue: %v` per failed declaration (`:225-229`), `Removed <namespace>` per removed lock entry (`:231-233`), then `moduleRemoveSummaryLine(result.LockWritten())` when every edit was clean (`:236-237`, `:273-278`, `moduleDeclarationEditsClean` at `:288-295`), else `Lock file updated; invowkmod.cue needs manual review` (`:239`).
- `Resolver.RemoveModuleDependency` (`use_cases.go:228-262`): after `m.Remove` saved the lock (`:237`), it calls `invowkmod.RemoveRequirement` per removed entry (`:244`). On an error it joins the restore of both snapshots and returns at once (`:245-252`), so the lock is restored. On success it records `NewDeclarationEditResult(true, nil)` (`:254`) whatever the edit did.
- `RemoveRequirement` (`pkg/invowkmod/invowkmod_edit.go:76-138`) returns nil without writing when the file is missing (`:84-86`), has no `requires` block (`:93-95`) or has no matching entry (`:110-112`); it writes only at `:137`. A read error other than "not exist" is returned (`:87`). Its only production caller is `use_cases.go:244`; tests call it in `invowkmod_edit_test.go:200`, `:406` and `invowkmod_edit_mutation_test.go`.
- `DeclarationEditResult.Updated()` is documented as "whether invowkmod.cue was edited" (`use_cases.go:115-116`); for add it is `editErr == nil` (`:207`), which is exact because `AddRequirement` either writes or fails (`invowkmod_edit.go:24-74`).
- The written-or-skipped precedent is `LockFile.SaveIfChanged` with `Save` as a delegate (archived change `2026-11-24-lock-unchanged-message`, design D1).
- Tests: `TestAddModuleDependencyRollsBackLockOnDeclarationFailure` already runs add with no `invowkmod.cue` and a fake fetcher (`internal/app/modulesync/use_cases_dependency_test.go:164-205`), and `TestRemoveModuleDependencyRollsBackLockOnDeclarationFailure` makes the edit fail with a directory at the `invowkmod.cue` path (`:207-260`). `TestRemoveModuleDependencyReportsLockWritten` covers the clean remove (`use_cases_lock_written_test.go:83-112`). The CLI lines are covered by the table in `cmd/invowk/module_deps_lock_test.go:13-38`. The only offline remove testscript is `tests/cli/testdata/module_remove_happy.txtar`; every `module add` testscript is argument validation only (`module_add_remove.txtar`), since add always resolves over the network (archived design, Context).
- Formal: `formal/tla/ConcurrentModuleEdits.tla:46` (`RemoveProg`, `Resolver.RemoveModuleDependency`, characterisation-only), `:60` (`WriteMod remove`, `RemoveRequirement`, no binding, listed in `[triage] binding_gaps` at `formal/manifest.toml:207`) and the step comment `:176` (`invowkmod_edit.go:82,137`). `python3 scripts/formal_mutation.py symbols` on this base labels `internal/app/modulesync/use_cases.go:Resolver.RemoveModuleDependency` and `pkg/invowkmod/invowkmod_edit.go:RemoveRequirement` `unplanned` and prints `-> skip` (run by this stage; the bare names are `unknown` because neither has a ledger entry). `binding_gaps` only shrinks, so a row renamed to a new symbol would be an unlisted gap (archived design D6).

## Goals / Non-Goals

**Goals:**
- Each of the three lines tells the truth, with the decision taken in one place: the edit helper knows whether it wrote, the use case carries it, the CLI renders it.
- Bound or row-named bodies change only as far as the flag requires.

**Non-Goals:**
- No change to when files are written, to the rollback itself, or to `module add`'s success path, `module sync`, `update`, `tidy` or `vendor`.
- `module update` with no lock still creates an empty lock (tracker Q4; brief "Not in scope").
- The `LockFile.Save` formal rows (archived Q3) belong to the formal follow-ups change.
- The `Removed <namespace>` lines printed before a rollback summary, and rollback lines when a restore itself failed, keep today's behaviour (Open questions OQ1, OQ2).

## Decisions

### D1. `module add` tests the error chain

A pure render helper in `cmd/invowk/module_deps.go`, `moduleAddDeclarationFailureLine(err error) string` (annotated `//plint:render` like its neighbours), returns the not-found line when `errors.Is(err, fs.ErrNotExist)` and the rollback line with `%v` otherwise; `runModuleAdd` prints the blank line and the helper's text. The texts are today's. The `os` import goes if nothing else in the file uses it.

Alternatives: keep the check inline (the fix then has no unit test without capturing stdout); classify "missing" in the use case with a new `DeclarationEditResult` field (more surface, and the brief names `errors.Is`); `os.IsNotExist(errors.Unwrap(...))` (fails on `errors.Join`, which has no single `Unwrap`).

### D2. `module remove`'s rollback summary

The summary after a failed declaration edit becomes `ℹ Lock file changes were rolled back; invowkmod.cue needs manual review`. "Lock file changes were rolled back" is `module add`'s rollback wording; "invowkmod.cue needs manual review" stays, since the dependency is still declared and the cause (in the `Could not auto-edit invowkmod.cue: <error>` line above it) needs the user. The info icon stays. The per-declaration error line and the `Removed` lines do not change.

One helper chooses every remove summary: `moduleRemoveSummaryLine(declarations []modulesync.DeclarationEditResult, lockWritten bool) string` returns the rollback line when any declaration has an error, else one of the four success rows of D4. `moduleDeclarationEditsClean` folds into it or stays as its private predicate.

Alternatives: print add's whole line (`Could not auto-edit invowkmod.cue; lock file changes were rolled back: <error>`) in place of both remove lines (drops the per-entry error line and changes two lines instead of one); "nothing was removed" (contradicts the `Removed` lines, OQ1).

### D3. `RemoveRequirementIfPresent` holds the body; `RemoveRequirement` wraps it

`func RemoveRequirementIfPresent(invowkmodPath types.FilesystemPath, gitURL GitURL, subPath SubdirectoryPath) (written bool, err error)` takes over `RemoveRequirement`'s body: the missing-file, no-block and no-match returns give `false, nil`; a read error `false, err`; the final `fspath.AtomicWriteFile` gives `true, nil` on success and `false, err` on failure. `RemoveRequirement` becomes `_, err := RemoveRequirementIfPresent(...); return err`, with its doc comment pointing at the new function. No statement of the moved body changes beyond its returns.

Alternatives: change `RemoveRequirement`'s signature (breaks the public `pkg/invowkmod` API; the brief says follow `SaveIfChanged`); compare `invowkmod.cue` bytes before and after in the use case (a second read, and a race window the model does not have); a written-flag field (hidden state). Name: `IfPresent` reads like `SaveIfChanged`'s `IfChanged`, naming the condition under which it writes.

### D4. The use case records the flag; the CLI renders four success rows

`Resolver.RemoveModuleDependency` calls `RemoveRequirementIfPresent` and records `NewDeclarationEditResult(written, nil)` on success; the error branch is unchanged (it still records `false`). `Updated()` then means "this edit wrote invowkmod.cue" for both add and remove, as its comment already says. `RemoveModuleDependencyResult` gains nothing: the CLI derives "invowkmod.cue written" as "any declaration `Updated()`".

| Lock | invowkmod.cue | Line |
|---|---|---|
| written | written | `✓ Lock file and invowkmod.cue updated` (today) |
| skipped | written | `✓ Lock file unchanged; invowkmod.cue updated` (today) |
| written | not written | `✓ Lock file updated; invowkmod.cue unchanged` |
| skipped | not written | `✓ Lock file and invowkmod.cue unchanged` |
| any | an edit failed | `ℹ Lock file changes were rolled back; invowkmod.cue needs manual review` (D2) |

All success rows keep the success icon (a no-op remove of the declaration is still a successful command, as in the archived D4). With several removed entries, "written" means at least one edit wrote.

Alternative: a separate `InvowkmodWritten()` accessor on the remove result (duplicates what the declarations already say).

### D5. Tests

- `pkg/invowkmod/invowkmod_edit_test.go`: `RemoveRequirementIfPresent` reports `true` for the removal cases of `TestRemoveRequirement`, `false` with no error and byte-identical files for a missing file, a file with no `requires` block and a file with no matching entry, and `false` with the error for a directory path. Existing `RemoveRequirement` tests stay.
- `internal/app/modulesync`: `TestAddModuleDependencyRollsBackLockOnDeclarationFailure` also asserts `errors.Is(result.Declaration().Err(), fs.ErrNotExist)`. A new remove test with an `invowkmod.cue` lacking the entry asserts `LockWritten()`, `Declarations()[0].Updated() == false`, no error, and byte-identical `invowkmod.cue`; the same with no `invowkmod.cue`. The rollback test keeps `Updated() == false` and gains a byte comparison of the restored lock.
- `cmd/invowk/module_deps_lock_test.go`: the remove rows become a table over the five D4 rows (declarations built with `modulesync.NewDeclarationEditResult`); a new table for `moduleAddDeclarationFailureLine` with (a) the error the use case really returns for a missing file, built as `errors.Join(invowkmod.AddRequirement(<missing path>, ref), nil, nil)`, (b) the same joined with a non-nil restore error, both expecting the not-found line, and (c) a duplicate-requirement error expecting the rollback line with the error text. Case (a) fails against the base's `os.IsNotExist` check, which is the regression the brief asks for.
- Testscript `tests/cli/testdata/module_remove_messages.txtar`, offline with a seeded lock as in `module_remove_happy.txtar`: (1) `invowkmod.cue` without the matching entry prints `Lock file updated; invowkmod.cue unchanged`, not `invowkmod.cue updated`, and `cmp` shows `invowkmod.cue` unchanged; (2) a directory at the `invowkmod.cue` path (created with `mkdir`) prints `Could not auto-edit invowkmod.cue`, `Lock file changes were rolled back; invowkmod.cue needs manual review` and no `Lock file updated`, and `cmp` shows the lock byte-identical to a copy taken before. If a platform rejects case (2) for a reason unrelated to this change, the implement stage guards that case with a platform condition and records why; the unit tests above still cover it. A `module add` testscript is not cheap: add always resolves over the network, so item 1 is covered at unit level only.

### D6. Formal correspondence

- `ConcurrentModuleEdits.tla:60` keeps the symbol `RemoveRequirement` (renaming it to the new function would turn its listed binding gap into an unlisted one, which `triage --check` refuses) and its note becomes "delegates to RemoveRequirementIfPresent, which holds the narrow read (N) to write (M) window with no seam", with the new line numbers. The step comment at `:176` names the new lines. If `Resolver.RemoveModuleDependency`'s lines move, `:46` and `:172-173` follow. No model logic, constants or goldens change.
- `formal_mutation.py symbols` labels both row-named functions `unplanned` (`-> skip`, Context), and `RemoveRequirementIfPresent` is named by no row, so no `mutation-formal` run and no ledger refresh. The fix stage re-runs `symbols` on the final tree, then `python3 scripts/formal_mutation.py triage --check`, `python3 scripts/formal.py correspondence` and `make formal-reach`.

## Risks / Trade-offs

- [Restore failure behind a not-found line] If `invowkmod.cue` is missing and the lock restore also fails, `errors.Is` still finds `fs.ErrNotExist` and add prints "dependency was not persisted" without the restore error; a restore failing with a not-exist error of its own would do the same with `invowkmod.cue` present. → Rare (the missing-file rollback removes nothing; the lock restore writes a file in a directory the command just wrote to). Case (b) of D5 pins the chosen behaviour; OQ2 covers reporting restore failures.
- [Rollback claimed when a restore failed] The add and remove rollback lines say the changes were rolled back even when a restore failed; the restore error is part of the error printed with them (`use_cases.go:205`, `:246`). → Same as add today; OQ2.
- [New public function] `RemoveRequirementIfPresent` is new `pkg/invowkmod` API; goplint may flag it. → Its parameters are already typed; fix any finding in code, with no baseline entry or inline ignore.
- [Output change for scripts] A script grepping `invowkmod.cue updated` or `Lock file updated; invowkmod.cue needs manual review` sees the new lines. → Intended: the old lines were false. Exit codes are unchanged.
- [Shared formal files with the F15 run] F15 (`invowk-f15`) may edit `formal/manifest.toml` or `tools/mutation/triage/formal-bindings.toml`. → This change edits only `ConcurrentModuleEdits.tla` comments; on rebase keep both sides' entries.

## Migration Plan

None. No file format, flag or exit code changes. Rollback is a revert of the PR.

## Maintainer decisions

- 2026-11-25 (decisions 105, 106, 107; brief `program-notes/briefs/module-cli-messages-brief.md`; binding): scope is the three items only: the `os.IsNotExist` bug with `errors.Is(err, fs.ErrNotExist)` and a test that fails on the base (testscript if cheap); remove's rollback line saying the lock was restored, matching add's; `RemoveRequirement` reporting whether it wrote, following `LockFile.SaveIfChanged`, with an accurate CLI line. Q3 and Q4 stay out. Docs: README "Lock File" and module sections, `cli-commands.mdx` in en and pt-BR. Check `formal_mutation.py symbols` on every changed function.
- S1, 2026-11-25 (this stage, default pending spec review): D2's wording, D3's name `RemoveRequirementIfPresent`, and the two new success rows of D4.

## Open questions

- OQ1. After a failed `invowkmod.cue` edit, `module remove` still prints `✓ Removed <namespace>` for every removed lock entry (`cmd/invowk/module_deps.go:231-233`), although the lock has been restored and nothing was removed. Moving those lines to the success branch is a separate wording change; this change only fixes the summary line (brief item 2).
- OQ2. The add and remove rollback lines do not distinguish a failed restore from a successful one (Risks). A typed rollback outcome in `DeclarationEditResult` would make them exact; it is a separate change touching both commands.
- OQ3. The `RemoveProg` row note "RemoveRequirement never fails, so no rollback path" (`ConcurrentModuleEdits.tla:46`) is not what the code does: remove has a rollback path (`use_cases.go:245-252`), tested by `TestRemoveModuleDependencyRollsBackLockOnDeclarationFailure`. Whether the model should include it, and whether the `WriteMod remove` row should move to `RemoveRequirementIfPresent` (as Q3 asks for `LockFile.Save`), belongs to the formal follow-ups change.

## Implementation notes (implement stage, 2026-11-25)

- The testscript's directory case (D5 case 2) needs no platform guard: it reuses the directory-at-`invowkmod.cue` trick that `TestRemoveModuleDependencyRollsBackLockOnDeclarationFailure` already runs on every CI platform.
- `TestRemoveModuleDependencyReportsLockWritten` seeded a one-line `requires` entry (`{git_url: "…", version: "…"},`). The requires editor matches `git_url` only on its own line, so that edit never wrote `invowkmod.cue` and the old `Updated() == true` hid it; with the written flag the test failed. The fixture now uses the multi-line layout `module add` writes, and the test also checks the entry is gone. That `RemoveRequirement` leaves one-line entries in place is pre-existing and out of scope (OQ4).
- The module commands' info icon is `•` (`cmd/invowk/module.go`, `moduleInfoIcon`), not `ℹ` as D2 and the existing `cli-commands.mdx` output block show. The new doc lines quote the info-line texts without an icon; the existing `ℹ` in that block is a pre-existing doc mismatch (OQ5).
- `formal_mutation.py symbols` on this tree (`Resolver.RemoveModuleDependency`, `RemoveRequirement`, `RemoveRequirementIfPresent` and the four changed `cmd/invowk/module_deps.go` functions): 0 planned, 7 unplanned, `-> skip`. `triage --check`, `formal.py correspondence` and `make formal-reach` pass.

## Open questions (implement stage)

- OQ4. `RemoveRequirement` does not match a `requires` entry written on one line (`{git_url: "…", version: "…"},`), which CUE allows; `module remove` then leaves it in `invowkmod.cue` and now says `invowkmod.cue unchanged`. Matching one-line entries is a separate editor change.
- OQ5. `website/docs/modules/dependencies/cli-commands.mdx` (en and pt-BR) shows the module info icon as `ℹ` in its output blocks; the CLI prints `•`.
