# Tasks

## 1. Characterise on the base tree

- [ ] 1.1 Write `tests/cli/testdata/virtual_module_workdir_roots.txtar` (design D5, CLI bullet) and run it against the unchanged tree. Expect the module-outside reads and writes to succeed there (that is F15), and record the result in a comment at the top of the file. Add its exact entry to `tests/cli/runtime_mirror_exemptions.json` ("restricted path denials exist only in the virtual runtimes"). Verify: `go test -run 'TestCLI/virtual_module_workdir_roots' ./tests/cli/` fails only on the module-outside denial steps before the fix.

## 2. Effective workdir with its source (`pkg/invowkfile`)

- [ ] 2.1 Add `WorkDirSource` (with `Validate()` and `DeclaredByInvowkfile()`), `EffectiveWorkDir` (with `Validate()`) and `(*Invowkfile).ResolveEffectiveWorkDir` in a new file `pkg/invowkfile/workdir_source.go` (design D1). Leave the body of `GetEffectiveWorkDir` unchanged. Comment on `effectiveWorkDirSource` that it sits beside the formal-bound `GetEffectiveWorkDir` on purpose. Verify: `git diff origin/main -- pkg/invowkfile/invowkfile.go` shows no change inside `GetEffectiveWorkDir`.
- [ ] 2.2 Tests in `pkg/invowkfile/workdir_source_test.go`: the 16-combination drift test (the level named by `Source` reproduces `Path` on its own), the invalid-path fallback case, and `WorkDirSource.Validate` for every value and for an invalid value. Verify: `go test -run 'TestResolveEffectiveWorkDir|TestWorkDirSource' ./pkg/invowkfile/` passes.

## 3. The workdir-root decision (`internal/runtime`)

- [ ] 3.1 Add `virtualWorkdir`, `virtualWorkdirFor` and `(virtualWorkdir).isReadRoot` in a new file `internal/runtime/virtual_workdir.go` (design D2; `virtual_policy.go` is at 907 lines). Change `newVirtualPathResolverForFilesystem` to take a `virtualWorkdir` and to add the workdir and `@work` to the roots only when `isReadRoot()` holds. Make `newVirtualPathResolver` use `virtualWorkdirFor`, and update every test caller (`runtime_env_test.go`, `virtual_path_resolution_test.go`, `virtual_path_harness_formal_test.go`). Verify: `go build ./...` and `go test ./internal/runtime/ -run 'TestVirtualPath'` pass, except the F15 and construction replays, which tasks 5.3 and 5.4 change.
- [ ] 3.2 Unit tests in `internal/runtime/virtual_workdir_test.go` (design D5): CLI override, plain invowkfile, module default, module inside, module outside by `..`, by an absolute path and through a link; a link cycle; a missing workdir below the module; on darwin `/var` against `/private/var`; on windows another volume and a junction inside the module to an outside directory (`fstree.MustJunction`). Each checks both `isReadRoot()` and the resolver's verdict on a read below the workdir under restricted and full access. Verify: `go test -run 'TestVirtualWorkdir' ./internal/runtime/` passes, with none of the Linux cases skipped.

## 4. Interactive subprocesses

- [ ] 4.1 Add `WorkDirModuleRoot` to `ShInteractiveCommandSpec`, `LuaInteractiveCommandSpec`, `ShScriptOptions` and `LuaScriptOptions`. Set it in `virtual_interactive.go` from the parent's `virtualWorkdir`. Pass the hidden `--workdir-module-root` flag in `internal/app/commandadapters/{sh,lua}_interactive.go` only when it is set, and parse it in `cmd/invowk/internal_exec_{sh,lua}.go`. Make `newVirtualPathResolverForInteractiveConfig` build the same `virtualWorkdir` (design D3). The commands stay hidden. Verify: `go test ./internal/runtime/ -run 'TestInteractive|Interactive'` and `go test ./internal/app/commandadapters/ ./cmd/invowk/` pass.
- [ ] 4.2 Tests: the launcher spec carries `WorkDirModuleRoot` for a module-outside workdir and nil otherwise (`internal/runtime/interactive_test.go`). The adapter adds the flag only when it is set, and `RunShScript` and the Lua runner deny the module-outside read when the option is set. Verify: the new tests pass under `go test ./internal/runtime/ ./internal/app/commandadapters/`.

## 5. Formal model and replays

- [ ] 5.1 Model (`formal/alloy/VirtualPathHarness.als`, design D4): `WorkdirKind` and its fact, `WorkdirRule`, `workdirIsRoot`, the rule parameter on `locIsAllowedRoot`, `accepts` and `harnessContained`, the `allowedTrustedWorkdir` policy, `trustedWorkdirReadable`, `witModuleOutsideWorkdir`, the rewritten `workdirWidening` and the golden with `H.workdir` free. Update the header's F15 paragraph to "fixed" and the correspondence table: the `mutantPreFixF15WorkdirRoot` row replaces `findingF15`, a new `workdirIsRoot` row, and updated cells for `allowedRoots`, `anchors`, `contained` and `allowedWorkdirCwd`. Verify: `make formal-alloy` passes.
- [ ] 5.2 Manifest (`formal/manifest.toml`): rename `findingF15WorkdirWidening` to `mutantPreFixF15WorkdirRoot` (`harnessContained[FixedEval, AlwaysRoot]`, no `finding`). Move the other harness commands to `TrustedOrInside`. Add `trustedWorkdirReadableFixed`, `mutantStrictWorkdirNeverRoot` and the new witness. Remove `"VirtualPathHarness: findingF15"` from `unresolved_elements`. Rewrite the F15 sentences of the calibration text. Run `make formal-golden`. Verify: `make formal-alloy` shows `harnessContainedFixed` and `trustedWorkdirReadableFixed` passing, and the four `mutantPreFix…` commands plus `mutantStrictWorkdirNeverRoot` each reporting a counterexample.
- [ ] 5.3 Invert and rename the replay to `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot` (spec "The F15 replay asserts the denial"). Verify: `go test -run 'TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot' ./internal/runtime/` passes, and fails when the root rule is seeded back to always-admit (`go test -overlay`; record it in the calibration text).
- [ ] 5.4 Construction replay `TestVirtualPathHarness_RootConstruction` with the workdir kinds and the `@work` class per kind (design D5). Calibrate: seed "always add the workdir and `@work`" and "never add the workdir" into `newVirtualPathResolverForFilesystem` and `isReadRoot`, one at a time, and revert. Record the failing bindings in the calibration text. Verify: `go test -run 'TestVirtualPathHarness_RootConstruction' ./internal/runtime/` passes, and each seed fails it.
- [ ] 5.5 Golden replay: build `virtualWorkdir` per instance kind and add the `work` root exactly when `isReadRoot()` holds. Disagreements name the workdir kind. Verify: `go test -run 'TestVirtualPathHarness_GoldenVectors' ./internal/runtime/` passes with zero disagreements, and the new instance count and replay time are in the calibration text.
- [ ] 5.6 Rapid property: workdir source and placement generators, with the oracle in both directions (spec "Harness rapid property covers workdir placement"). Verify: `go test -run 'TestVirtualPathHarness_Property' ./internal/runtime/` and `make formal-rapid-deep` pass.
- [ ] 5.7 `formal/README.md`: the F15 row (Fixed, the pre-fix mutant, the new replay, and the base replay `TestVirtualPathHarness_F15WorkdirWidening` at 3bb45325 as the pre-fix reproduction) and the VirtualPathHarness summary row. Verify: `python3 scripts/formal.py correspondence` passes, and `grep -n 'F15' formal/README.md` shows "Fixed".

## 6. CLI test and docs

- [ ] 6.1 Finish `virtual_module_workdir_roots.txtar` for the fixed behaviour. Under restricted access the module-outside virtual-sh read and write and the virtual-lua `io.open` fail with `virtual path denied`, and nothing is written. They succeed with `virtual.filesystem.paths: {WORK: "@work"}`, with `--ivk-workdir` naming the same directory, and under `full` access. A plain invowkfile with `workdir: "../shared"` succeeds. The module command's cwd is the outside directory (`pwd`). Verify: `go test -run 'TestCLI/virtual_module_workdir_roots' ./tests/cli/` and `go test -run 'TestShRuntimeMirrorCoverage|TestVirtualNativeCommandPathAlignment' ./tests/cli/` pass.
- [ ] 6.2 Docs (design D7): `website/docs/runtime-modes/virtual.mdx` (the root list), `virtual-lua.mdx` (the implicit roots sentence), `website/docs/advanced/workdir.mdx` (the virtual-runtime note with the `virtual.filesystem.paths` escape hatch and `--ivk-workdir`), their pt-BR copies, and README.md if it lists the roots or the internal command flags. Never say "sandbox" or "isolated", and give no exploit steps. Verify: `cd website && npm run build` passes.
- [ ] 6.3 Agent docs where they describe the harness roots (`.agents/skills/virtual-lua/SKILL.md`, `.agents/skills/formal-verification/SKILL.md`, `.agents/skills/module-security/` if it mentions workdirs). Verify: `make check-agent-docs` passes.

## 7. Commit, formal lane and platform evidence

- [ ] 7.1 Commit, signed and under the heavy lock: `fix(runtime): keep module workdirs outside the module out of virtual roots (F15)`, with the release note of the proposal in the body and the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Verify: `git log -1 --show-signature` shows a good signature.
- [ ] 7.2 Formal lane: `python3 scripts/formal.py affected --event pull_request --base origin/main --head HEAD`. When it prints `run=true`, run `python3 scripts/formal.py correspondence`, `make formal-reach`, `make formal` and `make test-scripts`. Run `python3 scripts/formal_mutation.py symbols` and note in the PR body which bound symbols need a refresh (expected: `newVirtualPathResolverForFilesystem`, plus the new bound symbols; not `GetEffectiveWorkDir`). Verify: each passes and no killer is reported as skipped.
- [ ] 7.3 Push `fix/module-workdir-roots` and open the draft PR. Verify: `gh pr view` shows a draft.
- [ ] 7.4 From the PR's `Test (windows)` and `Test (macos)` jobs, quote the run id, the job id and the test names that ran and passed: `TestVirtualWorkdir*` (the other-volume and junction cases on Windows, `/private` on macOS), `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot`, `TestVirtualPathHarness_RootConstruction`, `TestVirtualPathHarness_GoldenVectors`, `TestVirtualPathHarness_Property` and `TestCLI/virtual_module_workdir_roots`. Verify: none of the quoted tests is reported as skipped; a green job alone is not evidence.

## 8. Completion gates (`.agents/rules/checklist.md`)

- [ ] 8.1 `make tidy`, and `make license-check` for the new Go files. Verify: no diff, no missing header.
- [ ] 8.2 `make lint`. Verify: `rc=0`.
- [ ] 8.3 `make test` (full suite). Verify: `rc=0`.
- [ ] 8.4 `make check-baseline`, and triage of any new goplint finding (the new value types `WorkDirSource` and `EffectiveWorkDir` included). Verify: `rc=0`.
- [ ] 8.5 `make check-file-length`. Verify: every Go file stays under 1000 lines.
- [ ] 8.6 `make sonar-local`. Verify: no new issue that code can fix (go:S8209, go:S1192, go:S107, go:S8193, go:S8239).
- [ ] 8.7 Conditional checks: `make test-scripts`, `python3 scripts/formal.py correspondence`, `make formal`, the schema sync tests only if a CUE-backed struct changed (none expected), `cd website && npm run build` and `make check-agent-docs`. Verify: each passes.
- [ ] 8.8 Mutation: the partial refresh of the touched ledger entries (design D6) and `python3 scripts/formal_mutation.py triage --check`. On a rebase over the concurrent lock-unchanged-message change, keep both sides' entries in `tools/mutation/triage/formal-bindings.toml`. Verify: `triage --check` passes.

## Workflow follow-up

- The spec review, simplify, code review and fix stages run per `.agents/runbooks/change-flow.md`. The mutation refresh in 8.8 belongs to the fix stage after its rebase.
- Archive the change after merge, and sync the three delta specs into `openspec/specs/`.
