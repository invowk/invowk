# Proposal

## Why

Since `lock-write-skip-unchanged` (PR #186), saving the module lock leaves an unchanged `invowkmod.lock.cue` byte-identical, but `invowk module sync`, `update`, `add` and `remove` still print that the lock file was updated (`cmd/invowk/module_deps.go:198`, `:237`, `:239`, `:284`, `:364`). The message is false on every repeated run, so a user cannot tell from the output whether the lock changed (tracker N22, maintainer decision 47: fix it before the next tag).

## What Changes

- The lock writer reports whether it wrote the file. A new, additive call on the lock (for example `SaveIfChanged`) returns "written" or "skipped" together with the error; the existing save call keeps its signature and delegates to it, so every current caller keeps compiling.
- The module resolver operations that save the lock (add, remove, update, sync) return that fact, and the module services the CLI calls carry it in their typed results. The CLI adapters only render it.
- `module sync` and `module update` print `Lock file unchanged: invowkmod.lock.cue` when the write was skipped and the existing `Lock file updated: invowkmod.lock.cue` when it happened. `module update` with no module to update also prints that line, because it still saves (and may create) the lock.
- `module add` prints `Module lock file unchanged` or the existing `Module lock file updated`, then its existing `invowkmod.cue` line. `module remove` reports the lock and `invowkmod.cue` separately, so a skipped lock write is no longer folded into "Lock file and invowkmod.cue updated".
- Docs that quote these lines (website module CLI page and its snippets, the pt-BR copy, the README lock-file paragraph) show the new wording.
- Tests: written/skipped for each resolver operation and for the new lock call, CLI rendering of every line, and a network-free testscript that runs a lock-saving command twice and sees "updated", then "unchanged", with a byte-identical lock.

Not breaking for users: the lock file, its format and the exit codes do not change; only the success line differs when nothing was written. Go callers of the public `pkg/invowkmod` API keep `Save` as it is. Internal signatures in `internal/app/modulesync` and `internal/app/moduleops` change, which is allowed for private packages.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `module-lock-persistence`: adds that saving the lock reports whether it wrote the file, and that the module commands that save it say "unchanged" or "updated" to match. The existing write and skip rules are unchanged.

## Impact

- Code: `pkg/invowkmod/lockfile.go` (`LockFile.Save` and the new reporting call); `internal/app/modulesync/resolver.go` (`Resolver.Add`, `Remove`, `Update`, `Sync`, minimal body change: return the flag); `internal/app/modulesync/use_cases.go` (typed results for add, remove, sync, update); `internal/app/moduleops/vendor_dependencies.go` (resolver interface follows the new `Sync` signature, flag ignored); `cmd/invowk/module_deps.go` (rendering).
- Tests: `pkg/invowkmod/lockfile_save_test.go`, `internal/app/modulesync/resolver_lock_generated_test.go` and the other callers of the changed resolver methods, `internal/app/moduleops/vendor_test.go` (fake resolver), a new `cmd/invowk` rendering test, and the `module_update_lock_unchanged.txtar` / `module_sync_update.txtar` testscripts.
- Formal: `Resolver.Add`, `Remove`, `Update`, `Sync`, `Resolver.AddModuleDependency`, `Resolver.RemoveModuleDependency`, `SyncModule`, `UpdateModule` and `LockFile.Save` are named by the correspondence tables of `formal/tla/ConcurrentModuleEdits.tla`, `formal/tla/LockIntegrity.tla` and `formal/tla/AtomicWrite.tla`. No model changes; rows and line references are updated. `Resolver.Sync`'s body changes, so the fix stage runs the formal-bindings mutation once and refreshes the ledger.
- Docs: `website/docs/modules/dependencies/cli-commands.mdx`, `website/src/components/Snippet/data/modules.ts`, `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx`, `README.md` (lock-file paragraph near line 2184), `website/docs/modules/dependencies/lock-file.mdx` and its pt-BR copy.
- No dependency, schema, flag or exit-code change.
