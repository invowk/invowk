# Tasks

## 1. Lock writer reports the write (`pkg/invowkmod`)

- [x] 1.1 In `pkg/invowkmod/lockfile.go`, add `func (l *LockFile) SaveIfChanged(path string) (written bool, err error)` with `Save`'s current body (design D1): `false, nil` on the equal-content skip (still taking `Generated` from the file), `true, nil` after the atomic write, `false, err` on any error. Make `Save` delegate to it and keep its doc comment, pointing at `SaveIfChanged`. Verify `go build ./...` passes and no other caller changes.
- [x] 1.2 In `pkg/invowkmod/lockfile_save_test.go`, assert `SaveIfChanged`'s flag in every existing case (`false` for the two skip tests, `true` for changed content, version change, missing, unparseable, unknown-version and oversized files) and add a failed-save case (invalid lock) that returns the error with `written == false` (spec: Saving the lock reports whether it wrote). Verify `go test -run 'TestLockFileSave' ./pkg/invowkmod/` passes.

## 2. Resolver and services carry the flag (`internal/app/modulesync`, `internal/app/moduleops`)

- [x] 2.1 In `internal/app/modulesync/resolver.go`, change `Resolver.Add`, `Remove`, `Update` and `Sync` to return `lockWritten bool` before the error, taken from `lock.SaveIfChanged(lockPath)`; every early return passes `false` (design D2). Change nothing else in these bodies. Verify `go vet ./internal/app/modulesync/` passes after 2.2-2.4.
- [x] 2.2 In `internal/app/modulesync/use_cases.go`, add `lockWritten` to `AddModuleDependencyResult` and `RemoveModuleDependencyResult` (field, `LockWritten()` accessor, constructor parameter), and replace the returns of `SyncModule` and `UpdateModule` with new `SyncModuleResult` and `UpdateModuleResult` types (constructor, accessors, `Validate()`), following `AddModuleDependencyResult` (design D3). Verify `go build ./...` passes.
- [x] 2.3 Update `vendorDependencyResolver` in `internal/app/moduleops/vendor_dependencies.go` and its fake in `internal/app/moduleops/vendor_test.go` to the new `Sync` signature; `resolveVendorDependencies` discards the flag (design D2). Verify `go test ./internal/app/moduleops/` passes.
- [x] 2.4 Update every other caller of the changed signatures, call sites only, assertions unchanged: `lock_trace_ops_test.go`, `concurrent_edits_formal_test.go`, `resolver_integrity_test.go`, `resolver_test.go`, `resolver_lock_test.go`, `resolver_bench_test.go` and any other file `go vet ./...` names. Verify `go vet ./...` passes.
- [x] 2.5 In `internal/app/modulesync/resolver_lock_generated_test.go`, assert `lockWritten == true` in each row of `TestResolverContentChangeAdvancesGenerated` and `false` in each row of `TestResolverUnchangedContentKeepsLockBytes`; add a test that syncs twice from an empty working directory and asserts `true`, then `false`, with byte-identical lock bytes (design D5; spec: Repeated sync). Verify `go test -run 'TestResolver' ./internal/app/modulesync/` passes.
- [x] 2.6 Add service tests in `internal/app/modulesync`: `Resolver.AddModuleDependency` for a module the lock already holds, with an `invowkmod.cue` that lacks its `requires` entry, returns `LockWritten() == false` and `Declaration().Updated() == true`; adding a new module returns `true`; `Resolver.RemoveModuleDependency` returns `true`; `SyncModule` with no requirements returns `false` (spec: Add and remove report each file truthfully). Verify `go test -run 'ModuleDependency|SyncModule' ./internal/app/modulesync/` passes.

## 3. CLI rendering (`cmd/invowk`)

- [x] 3.1 In `cmd/invowk/module_deps.go`, add pure helpers that return the D4 lines for sync/update, add and remove from the result's `LockWritten()`, and use them in `runModuleAdd`, `runModuleRemove`, `runModuleSync` and `runModuleUpdate`; `runModuleUpdate` prints the lock line after `No modules to update` too. Leave the add and remove failure branches as they are (design D4). Verify `go build ./...` passes.
- [x] 3.2 Add a table test for the helpers in `cmd/invowk` (every D4 row, both outcomes, and the no-module update), new file with the SPDX header. Verify `go test -run 'TestModuleLock' ./cmd/invowk/` passes (use the test's actual name).
- [x] 3.3 Testscripts: in `tests/cli/testdata/module_update_lock_unchanged.txtar` add `stdout 'Lock file unchanged: invowkmod.lock.cue'` and `! stdout 'Lock file updated'`; in `tests/cli/testdata/module_sync_update.txtar` Test 2, assert `Lock file updated: invowkmod.lock.cue` on the first `module update` with no lock, copy the created lock, run `module update` again, assert `Lock file unchanged: invowkmod.lock.cue` and `cmp` the lock with the copy (design D5, M1; spec: Update creates a missing lock). Verify `go test -run 'TestCLI/module_(update_lock_unchanged|sync_update|remove_happy)' ./tests/cli/` passes.

## 4. Formal correspondence

- [x] 4.1 Extend the notes of the `LockFile.Save` rows (`formal/tla/AtomicWrite.tla:27`, `formal/tla/ConcurrentModuleEdits.tla:60`) to say that `Save` delegates to `SaveIfChanged`, which holds the skip-or-replace body and which the resolver calls, and that the written flag is reporting only; keep the symbol `LockFile.Save` and do not touch `formal/manifest.toml` `[triage]` (design D6). Update the line references in the `ConcurrentModuleEdits.tla` rows (`:43-54`) and step comments (`:162-189`) and in `LockIntegrity.tla:32` to the new line numbers. No model logic changes. Verify `python3 scripts/formal.py correspondence` and `make formal-reach` pass.
- [x] 4.2 Run `make formal` when Java is available (under the heavy-command lock, backgrounded with an `rc=` log); otherwise say so in the result. Because `formal/` changed, run `make test-scripts` the same way and verify it passes.
- [x] 4.3 Fix stage, after the review fixes and the tidy and lint fixes are committed: run `python3 scripts/formal_mutation.py plan --out <scratch>` and `symbols --plan <scratch>/formal-plan.json internal/app/modulesync/resolver.go:Resolver.Add internal/app/modulesync/resolver.go:Resolver.Remove internal/app/modulesync/resolver.go:Resolver.Update internal/app/modulesync/resolver.go:Resolver.Sync pkg/invowkmod/lockfile.go:LockFile.Save` (plus any other bound symbol whose body changed), expect `-> refresh`, then run `make mutation-formal` once, report-only, in a throwaway worktree, refresh the planned symbols, triage the added entries, run the named reruns, and commit baseline, ledger and reruns (`.agents/runbooks/change-flow.md`, Formal lane). Verify `python3 scripts/formal_mutation.py triage --check` passes.

## 5. Docs

- [x] 5.1 Update the quoted output: `website/src/components/Snippet/data/modules.ts` (`add-output`, `remove-output`, `sync-output`, `update-output`, and `update-empty`, which gains the lock line), the add and remove notes in `website/docs/modules/dependencies/cli-commands.mdx` and in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx`; add one sentence that a command prints `Lock file unchanged` when it skipped the write to the lock-file paragraph of `README.md` (near line 2184), `website/docs/modules/dependencies/lock-file.mdx` and its pt-BR copy. Do not touch versioned docs or README's virtual-runtime sections. Verify `cd website && npm run build` passes.

## 6. Completion gates (`.agents/rules/checklist.md`)

- [x] 6.1 `make tidy`; `make license-check` for the new Go test file.
- [x] 6.2 `make lint` passes with no issues (under the heavy-command lock).
- [x] 6.3 `make test` passes (full suite, under the heavy-command lock, backgrounded with an `rc=` log).
- [x] 6.4 `make check-baseline` passes; triage any new goplint finding (for `SaveIfChanged`, a sibling exception with the reason "filepath boundary", design D1).
- [x] 6.5 `make check-file-length` passes.
- [x] 6.6 `make sonar-local`: no new real issues (no go:S8209, go:S1192 outside tests, go:S107 or go:S8239 in new code).
- [x] 6.7 Conditional checks: `python3 scripts/formal.py correspondence`, `make test-scripts` (formal changed), `cd website && npm run build` (website changed), `make check-agent-docs` only if `.agents/` changed.
- [x] 6.8 `/learn` (`.agents/skills/learn/`) last; it does not edit the change-flow tooling inside this PR.

## Workflow follow-up

- Archive the change (`openspec-archive-change`) in the fix stage, after the review.
- Report Q1-Q4 from design.md and the M1 test substitution in the result's `maintainer_asks`.
