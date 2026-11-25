# Tasks

## 1. Remove reports whether it wrote invowkmod.cue (`pkg/invowkmod`)

- [x] 1.1 In `pkg/invowkmod/invowkmod_edit.go`, add `RemoveRequirementIfPresent(invowkmodPath types.FilesystemPath, gitURL GitURL, subPath SubdirectoryPath) (written bool, err error)` holding `RemoveRequirement`'s current body, with `false, nil` on the missing-file, no-block and no-match returns, `false, err` on a read or write error, and `true, nil` after a successful `fspath.AtomicWriteFile`; make `RemoveRequirement` a delegate that keeps its signature and points its doc comment at the new function (design D3). Verify `go build ./...` passes.
- [x] 1.2 In `pkg/invowkmod/invowkmod_edit_test.go`, cover `RemoveRequirementIfPresent`: `true` for the removal cases of `TestRemoveRequirement`; `false`, no error and byte-identical file for a missing file, no `requires` block and no matching entry; `false` with the error for a directory path (spec "Removing a requires entry reports whether it wrote"). Verify `go test -run 'TestRemoveRequirement' ./pkg/invowkmod/` passes.

## 2. Use case records the flag (`internal/app/modulesync`)

- [x] 2.1 In `Resolver.RemoveModuleDependency` (`internal/app/modulesync/use_cases.go`), call `invowkmod.RemoveRequirementIfPresent` and record `NewDeclarationEditResult(written, nil)` on success; leave the error branch and the rest of the body as they are (design D4). Verify `go build ./...` passes.
- [x] 2.2 Tests: in `TestAddModuleDependencyRollsBackLockOnDeclarationFailure` also assert `errors.Is(result.Declaration().Err(), fs.ErrNotExist)`; add a remove test with an `invowkmod.cue` lacking the removed entry and one with no `invowkmod.cue`, each asserting `LockWritten()`, `Declarations()[0].Updated() == false`, no declaration error and an unchanged (or still absent) `invowkmod.cue`; in `TestRemoveModuleDependencyRollsBackLockOnDeclarationFailure` compare the restored lock bytes with the bytes before the call (design D5). Verify `go test -run 'Test(Add|Remove)ModuleDependency' ./internal/app/modulesync/` passes.

## 3. CLI lines (`cmd/invowk`)

- [x] 3.1 In `cmd/invowk/module_deps.go`, add the render helper `moduleAddDeclarationFailureLine(err error) string` using `errors.Is(err, fs.ErrNotExist)` and print it from `runModuleAdd`'s failure branch with today's texts; drop the `os` import if unused (design D1). Change `moduleRemoveSummaryLine` to take the declarations and `lockWritten` and return the five rows of design D4 (rollback row: `ℹ Lock file changes were rolled back; invowkmod.cue needs manual review`, design D2), and print it from `runModuleRemove`; keep the per-declaration error line and the `Removed` lines. Verify `go build ./...` passes.
- [x] 3.2 In `cmd/invowk/module_deps_lock_test.go`, replace the two remove rows with a table over the five D4 rows (declarations from `modulesync.NewDeclarationEditResult`, including several declarations where only one wrote), and add a table for `moduleAddDeclarationFailureLine` with the three cases of design D5 (the missing-file error built as `errors.Join(invowkmod.AddRequirement(<missing path>, ref), nil, nil)`, the same joined with a non-nil restore error, and a duplicate-requirement error). Verify `go test -run 'TestModule(Lock|AddDeclaration|RemoveSummary)' ./cmd/invowk/` passes (use the tests' actual names).
- [x] 3.3 Add `tests/cli/testdata/module_remove_messages.txtar` (offline, seeded lock as in `module_remove_happy.txtar`) with the two cases of design D5: no matching `requires` entry (`Lock file updated; invowkmod.cue unchanged`, `! stdout 'invowkmod.cue updated'`, `cmp` of `invowkmod.cue`) and a directory at the `invowkmod.cue` path (`Could not auto-edit invowkmod.cue`, `Lock file changes were rolled back; invowkmod.cue needs manual review`, `! stdout 'Lock file updated'`, `cmp` of the lock with a copy taken before). Guard the second case by platform only if a platform rejects it for an unrelated reason, and record why in design.md. Verify `go test -run 'TestCLI/module_remove_(messages|happy)' ./tests/cli/` passes.
- [x] 3.4 Docs, for every changed line: in `website/docs/modules/dependencies/cli-commands.mdx` and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx`, the add note says what the command prints when `invowkmod.cue` does not exist (the not-found line; the lock is rolled back) instead of only "(if the file exists)", and the remove note gives the two `invowkmod.cue unchanged` lines and the rollback summary; in `README.md`'s Lock File paragraph (near line 2184) name remove's new lines and the rollback line. Leave the Snippet data and versioned docs alone unless they quote a changed line. Verify `cd website && npm run build` passes.

## 4. Formal correspondence

- [x] 4.1 In `formal/tla/ConcurrentModuleEdits.tla`, keep the `WriteMod remove` row's symbol `RemoveRequirement` and change its note to say it delegates to `RemoveRequirementIfPresent`, with the new read and write line numbers; update the step comment (`invowkmod_edit.go:82,137`, near `:176`) and, if `Resolver.RemoveModuleDependency`'s lines moved, the `RemoveProg` row and the `use_cases.go` step comments near `:172-173`. Touch neither model logic nor `formal/manifest.toml` (design D6). Verify `python3 scripts/formal.py correspondence` and `make formal-reach` pass.
- [x] 4.2 Fix stage, on the final tree: `python3 scripts/formal_mutation.py plan --out <scratch>`, then `symbols --plan <scratch>/formal-plan.json internal/app/modulesync/use_cases.go:Resolver.RemoveModuleDependency pkg/invowkmod/invowkmod_edit.go:RemoveRequirement` plus any other row-named function whose body changed; expect `-> skip` (both `unplanned`), so no mutation run; on `-> refresh`, follow `.agents/runbooks/change-flow.md` (Formal lane). Verify `python3 scripts/formal_mutation.py triage --check` passes. Because `formal/` changed, run `make test-scripts` under the heavy-command lock (backgrounded with an `rc=` log) and verify it passes; run `make formal` when Java is available, otherwise say so in the result.

## 5. Completion gates (`.agents/rules/checklist.md`)

- [x] 5.1 `make tidy`; `make license-check` for any new Go file.
- [x] 5.2 `make lint` passes with no issues (under the heavy-command lock).
- [x] 5.3 `make test` passes (full suite, under the heavy-command lock, backgrounded with an `rc=` log).
- [x] 5.4 `make check-baseline` passes; triage any new goplint finding (for `RemoveRequirementIfPresent`, fix it in code; no baseline entry, exception or inline ignore without a maintainer decision).
- [x] 5.5 `make check-file-length` passes.
- [x] 5.6 `make sonar-local`: no new fixable issues (go:S1192 outside tests, go:S8193, go:S8209, go:S107, go:S1871; go:S3776 is accepted).
- [x] 5.7 Conditional checks: `python3 scripts/formal.py correspondence` (formal comments changed), `cd website && npm run build` (website changed), `make check-agent-docs` only if `.agents/` changed.
- [x] 5.8 `/learn` (`.agents/skills/learn/`) last; it does not edit the change-flow tooling inside this PR.

## Workflow follow-up

- Archive the change (`openspec-archive-change`) in the fix stage, after the review.
- Report OQ1-OQ3 from design.md in the result's `maintainer_asks`; do not edit `.claude/issues/cst-adoption-gaps.md`.

## Fix-stage record (2026-11-25)

- Gates on `4a85f732` (base `origin/main` e2011627): `make tidy`, `make lint`, `make check-baseline`, `make test`, `make test-scripts`, `make formal` (Java present), `make check-file-length`, `make sonar-local`, `make formal-reach`, `python3 scripts/formal.py correspondence`, `python3 scripts/formal_mutation.py triage --check`, `cd website && npm run build` and `go run . validate` on every sample module all passed. No new Go file, so no `license-check`.
- `formal_mutation.py symbols` on `Resolver.RemoveModuleDependency`, `RemoveRequirement`, `RemoveRequirementIfPresent`, `runModuleAdd`, `runModuleRemove`, `moduleAddDeclarationFailureLine`, `moduleRemoveSummaryLine` and `moduleDeclarationRestoreFailed`: 0 planned, 0 removal-only, 8 unplanned, `-> skip`; no `mutation-formal` run and no ledger refresh. The deleted `moduleDeclarationEditsClean` has no ledger entry and no planned row (`unknown`), so it needs no removal-only refresh.
