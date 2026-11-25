# Proposal

## Why

PR #209 made the module commands say truthfully whether they wrote `invowkmod.lock.cue`, but three lines around it still misreport what happened (maintainer decisions 105 and 106, follow-up of the archived change `2026-11-24-lock-unchanged-message`, open questions Q1 and Q2 and the rejected `os.IsNotExist` item):

- `invowk module add` with no `invowkmod.cue` prints "Could not auto-edit invowkmod.cue; lock file changes were rolled back: …" instead of its dedicated "invowkmod.cue not found — dependency was not persisted" line, because the CLI tests the wrapped error with `os.IsNotExist`, which does not unwrap `%w` or `errors.Join`. That line has been unreachable since the use case started joining the rollback errors.
- `invowk module remove`, after a failed `invowkmod.cue` edit, prints "Lock file updated; invowkmod.cue needs manual review", although the use case has just restored the lock file.
- `invowk module remove` prints "invowkmod.cue updated" even when `invowkmod.cue` had no matching `requires` entry (or did not exist) and was left untouched.

## What Changes

- `module add` recognises a missing `invowkmod.cue` through the error chain (`errors.Is(err, fs.ErrNotExist)`) and prints its existing not-found line again. No message text changes.
- `module remove`'s summary after a failed `invowkmod.cue` edit says the lock file changes were rolled back, using the same words as `module add`'s rollback line: `Lock file changes were rolled back; invowkmod.cue needs manual review`.
- Removing a `requires` entry from `invowkmod.cue` reports whether it wrote the file. A new, additive call in `pkg/invowkmod` returns that flag with the error, in the same shape as `LockFile.SaveIfChanged`; the existing `RemoveRequirement` keeps its signature and delegates to it. The remove use case records the flag in its declaration results, and the CLI prints `invowkmod.cue unchanged` when no edit wrote the file: `Lock file updated; invowkmod.cue unchanged` or `Lock file and invowkmod.cue unchanged`, next to the two existing success lines.
- Docs that quote these lines (README "Lock File" paragraph, `website/docs/modules/dependencies/cli-commands.mdx` and its pt-BR copy) describe the new remove lines, the rollback line and the add not-found line.
- Tests: unit tests for the not-found detection on the error the use case really returns, for the new remove call's written flag, for the remove use case's declaration results, and for every remove summary row; a network-free testscript for the remove lines.

Not breaking: no flag, file format or exit code changes. Only success and rollback lines differ in the cases above. Go callers of `pkg/invowkmod.RemoveRequirement` keep its signature.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `module-lock-persistence`: the requirement "Add and remove report each file truthfully" now also covers `invowkmod.cue` on remove (updated only when an edit wrote it) and replaces "keep the messages it prints today" for a failed `invowkmod.cue` edit with defined lines: add's not-found and rollback lines, and remove's rollback summary. A new requirement says removing a `requires` entry reports whether it wrote `invowkmod.cue`.

## Impact

- Code: `cmd/invowk/module_deps.go` (`runModuleAdd`, `runModuleRemove`, render helpers); `internal/app/modulesync/use_cases.go` (`Resolver.RemoveModuleDependency` passes the written flag into `NewDeclarationEditResult`); `pkg/invowkmod/invowkmod_edit.go` (`RemoveRequirement` and the new reporting call).
- Tests: `cmd/invowk/module_deps_lock_test.go`, `internal/app/modulesync/use_cases_dependency_test.go` and `use_cases_lock_written_test.go`, `pkg/invowkmod/invowkmod_edit_test.go`, a testscript under `tests/cli/testdata/`.
- Formal: `RemoveRequirement` (`ConcurrentModuleEdits` `WriteMod remove`, no binding) and `Resolver.RemoveModuleDependency` (`RemoveProg`, characterisation-only) are named by correspondence rows; `formal_mutation.py symbols` labels both `unplanned` (`-> skip`), so no mutation run. Row notes and step-comment line references in `formal/tla/ConcurrentModuleEdits.tla` are updated; no model logic changes.
- Docs: `README.md` (Lock File paragraph), `website/docs/modules/dependencies/cli-commands.mdx`, `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx`.
- No dependency, schema, flag or exit-code change.
